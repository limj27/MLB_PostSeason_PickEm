package main

import (
	"log"
	"net/http"
	"os"
)

func bootstrapAdmin() {
	adminUser := os.Getenv("ADMIN_USERNAME")
	adminPass := os.Getenv("ADMIN_PASSWORD")
	adminDisplayName := os.Getenv("ADMIN_DISPLAY_NAME")
	if adminDisplayName == "" {
		adminDisplayName = "Commissioner"
	}
	if adminUser == "" || adminPass == "" {
		log.Println("ADMIN_USERNAME/ADMIN_PASSWORD not set - skipping admin bootstrap")
		return
	}
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM users WHERE username = ?`, adminUser).Scan(&count)
	if count > 0 {
		// keep is_admin and display name in sync with .env on every boot
		db.Exec(`UPDATE users SET is_admin = TRUE, display_name = ? WHERE username = ?`, adminDisplayName, adminUser)
		return
	}
	hash, err := hashPassword(adminPass)
	if err != nil {
		log.Printf("could not hash admin password: %v", err)
		return
	}
	_, err = db.Exec(`INSERT INTO users (username, password_hash, display_name, is_admin) VALUES (?, ?, ?, TRUE)`,
		adminUser, hash, adminDisplayName)
	if err != nil {
		log.Printf("could not create admin user: %v", err)
		return
	}
	log.Printf("created admin user %q", adminUser)
}

func main() {
	connectDB()
	bootstrapAdmin()

	mux := http.NewServeMux()

	// Auth
	mux.HandleFunc("POST /api/register", handleRegister)
	mux.HandleFunc("POST /api/login", handleLogin)
	mux.HandleFunc("POST /api/logout", handleLogout)
	mux.HandleFunc("GET /api/me", handleMe)

	// Player-facing
	mux.HandleFunc("GET /api/rounds", requireAuth(handleRounds))
	mux.HandleFunc("POST /api/picks", requireAuth(handleSubmitPick))
	mux.HandleFunc("GET /api/leaderboard", handleLeaderboard)
	mux.HandleFunc("GET /api/all-picks", requireAuth(handleAllPicks))

	// Admin
	mux.HandleFunc("GET /api/admin/teams", requireAdmin(handleAdminTeams))
	mux.HandleFunc("POST /api/admin/rounds", requireAdmin(handleAdminUpsertRound))
	mux.HandleFunc("POST /api/admin/sync-lock", requireAdmin(handleAdminSyncLock))
	mux.HandleFunc("POST /api/admin/set-lock", requireAdmin(handleAdminSetLock))
	mux.HandleFunc("POST /api/admin/sync-result", requireAdmin(handleAdminSyncResult))
	mux.HandleFunc("POST /api/admin/set-result", requireAdmin(handleAdminSetResult))

	// Static frontend
	fs := http.FileServer(http.Dir("./frontend"))
	mux.Handle("/", fs)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("pickem server listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
