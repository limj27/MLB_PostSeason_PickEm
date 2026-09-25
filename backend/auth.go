package main

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const sessionCookieName = "pickem_session"
const sessionDuration = 30 * 24 * time.Hour // 30 days

func hashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

func checkPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func createSession(w http.ResponseWriter, userID int) error {
	token, err := newSessionToken()
	if err != nil {
		return err
	}
	expires := time.Now().Add(sessionDuration)
	_, err = db.Exec(`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`,
		token, userID, expires)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Secure should be true once served over HTTPS (it will be, behind
		// your Nginx/Let's Encrypt setup) - flip this on in production.
		Secure: os.Getenv("COOKIE_SECURE") == "true",
	})
	return nil
}

func clearSession(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookieName); err == nil {
		db.Exec(`DELETE FROM sessions WHERE token = ?`, c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
	})
}

// currentUser returns the logged-in user for a request, or nil.
func currentUser(r *http.Request) *User {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil
	}
	row := db.QueryRow(`
		SELECT u.id, u.username, u.password_hash, u.display_name, u.is_admin
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token = ? AND s.expires_at > NOW()`, c.Value)
	var u User
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.IsAdmin); err != nil {
		return nil
	}
	return &u
}

func requireAuth(next func(w http.ResponseWriter, r *http.Request, u *User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := currentUser(r)
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r, u)
	}
}

func requireAdmin(next func(w http.ResponseWriter, r *http.Request, u *User)) http.HandlerFunc {
	return requireAuth(func(w http.ResponseWriter, r *http.Request, u *User) {
		if !u.IsAdmin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r, u)
	})
}
