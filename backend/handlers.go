package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// ---------- Auth ----------

func handleRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, "bad request")
		return
	}
	body.Username = strings.TrimSpace(strings.ToLower(body.Username))
	body.DisplayName = strings.TrimSpace(body.DisplayName)
	if body.Username == "" || len(body.Password) < 6 || body.DisplayName == "" {
		writeErr(w, 400, "username, display name, and a password (6+ chars) are required")
		return
	}
	hash, err := hashPassword(body.Password)
	if err != nil {
		writeErr(w, 500, "could not create account")
		return
	}
	res, err := db.Exec(`INSERT INTO users (username, password_hash, display_name) VALUES (?, ?, ?)`,
		body.Username, hash, body.DisplayName)
	if err != nil {
		writeErr(w, 409, "that username is already taken")
		return
	}
	id, _ := res.LastInsertId()
	if err := createSession(w, int(id)); err != nil {
		writeErr(w, 500, "account created, but login failed - try logging in")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, "bad request")
		return
	}
	username := strings.TrimSpace(strings.ToLower(body.Username))
	row := db.QueryRow(`SELECT id, password_hash FROM users WHERE username = ?`, username)
	var id int
	var hash string
	if err := row.Scan(&id, &hash); err != nil {
		writeErr(w, 401, "invalid username or password")
		return
	}
	if !checkPassword(hash, body.Password) {
		writeErr(w, 401, "invalid username or password")
		return
	}
	if err := createSession(w, id); err != nil {
		writeErr(w, 500, "login failed")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	clearSession(w, r)
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func handleMe(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeErr(w, 401, "not logged in")
		return
	}
	writeJSON(w, 200, u)
}

// ---------- Rounds & picks ----------

func fetchRounds() ([]Round, error) {
	rows, err := db.Query(`SELECT id, round_key, round_name, best_of, has_mvp,
		team_a, team_a_mlb_id, team_b, team_b_mlb_id, lock_time, status,
		actual_winner, actual_length, actual_mvp, sort_order
		FROM rounds ORDER BY sort_order ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Round
	for rows.Next() {
		var rnd Round
		if err := rows.Scan(&rnd.ID, &rnd.RoundKey, &rnd.RoundName, &rnd.BestOf, &rnd.HasMVP,
			&rnd.TeamA, &rnd.TeamAMlbID, &rnd.TeamB, &rnd.TeamBMlbID, &rnd.LockTime, &rnd.Status,
			&rnd.ActualWinner, &rnd.ActualLength, &rnd.ActualMVP, &rnd.SortOrder); err != nil {
			return nil, err
		}
		rnd.Locked = rnd.LockTime != nil && time.Now().After(*rnd.LockTime)
		out = append(out, rnd)
	}
	return out, nil
}

// handleRounds returns all rounds plus the requesting user's picks.
func handleRounds(w http.ResponseWriter, r *http.Request, u *User) {
	rounds, err := fetchRounds()
	if err != nil {
		writeErr(w, 500, "could not load rounds")
		return
	}
	prows, err := db.Query(`SELECT id, user_id, round_id, picked_team, picked_length, picked_mvp
		FROM picks WHERE user_id = ?`, u.ID)
	if err != nil {
		writeErr(w, 500, "could not load picks")
		return
	}
	defer prows.Close()
	byRound := map[int]Pick{}
	for prows.Next() {
		var p Pick
		if err := prows.Scan(&p.ID, &p.UserID, &p.RoundID, &p.PickedTeam, &p.PickedLength, &p.PickedMVP); err != nil {
			writeErr(w, 500, "could not load picks")
			return
		}
		byRound[p.RoundID] = p
	}
	for i := range rounds {
		if p, ok := byRound[rounds[i].ID]; ok {
			pCopy := p
			rounds[i].MyPick = &pCopy
		}
	}
	writeJSON(w, 200, rounds)
}

// handleSubmitPick creates or updates the caller's pick for a round,
// rejecting the write once the round has locked.
func handleSubmitPick(w http.ResponseWriter, r *http.Request, u *User) {
	var body struct {
		RoundID      int     `json:"roundId"`
		PickedTeam   string  `json:"pickedTeam"`
		PickedLength int     `json:"pickedLength"`
		PickedMVP    *string `json:"pickedMvp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, "bad request")
		return
	}

	row := db.QueryRow(`SELECT id, round_key, round_name, best_of, has_mvp,
		team_a, team_a_mlb_id, team_b, team_b_mlb_id, lock_time, status,
		actual_winner, actual_length, actual_mvp, sort_order FROM rounds WHERE id = ?`, body.RoundID)
	var rnd Round
	if err := row.Scan(&rnd.ID, &rnd.RoundKey, &rnd.RoundName, &rnd.BestOf, &rnd.HasMVP,
		&rnd.TeamA, &rnd.TeamAMlbID, &rnd.TeamB, &rnd.TeamBMlbID, &rnd.LockTime, &rnd.Status,
		&rnd.ActualWinner, &rnd.ActualLength, &rnd.ActualMVP, &rnd.SortOrder); err != nil {
		writeErr(w, 404, "round not found")
		return
	}
	if rnd.LockTime != nil && time.Now().After(*rnd.LockTime) {
		writeErr(w, 403, "picks for this round are locked")
		return
	}
	if rnd.TeamA == nil || rnd.TeamB == nil {
		writeErr(w, 400, "matchup for this round isn't set yet")
		return
	}
	if body.PickedTeam != *rnd.TeamA && body.PickedTeam != *rnd.TeamB {
		writeErr(w, 400, "picked team must be one of the two teams in this round")
		return
	}
	maxLen := rnd.BestOf
	minLen := rnd.BestOf/2 + 1
	if body.PickedLength < minLen || body.PickedLength > maxLen {
		writeErr(w, 400, "series length pick is out of range for this round")
		return
	}
	if rnd.HasMVP && (body.PickedMVP == nil || strings.TrimSpace(*body.PickedMVP) == "") {
		writeErr(w, 400, "an MVP pick is required for this round")
		return
	}

	_, err := db.Exec(`INSERT INTO picks (user_id, round_id, picked_team, picked_length, picked_mvp)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE picked_team = VALUES(picked_team),
			picked_length = VALUES(picked_length), picked_mvp = VALUES(picked_mvp)`,
		u.ID, body.RoundID, body.PickedTeam, body.PickedLength, body.PickedMVP)
	if err != nil {
		writeErr(w, 500, "could not save pick")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	board, err := buildLeaderboard()
	if err != nil {
		writeErr(w, 500, "could not build leaderboard")
		return
	}
	writeJSON(w, 200, board)
}

// ---------- Admin ----------

func handleAdminTeams(w http.ResponseWriter, r *http.Request, u *User) {
	teams, err := fetchMLBTeams()
	if err != nil {
		writeErr(w, 502, "could not reach MLB Stats API")
		return
	}
	writeJSON(w, 200, teams)
}

// handleAdminUpsertRound creates or edits a round's static info (name, best-of,
// mvp flag, matchup). Use PATCH-style partial body.
func handleAdminUpsertRound(w http.ResponseWriter, r *http.Request, u *User) {
	var body struct {
		ID         int     `json:"id"`
		RoundKey   string  `json:"roundKey"`
		RoundName  string  `json:"roundName"`
		BestOf     int     `json:"bestOf"`
		HasMVP     bool    `json:"hasMvp"`
		TeamA      *string `json:"teamA"`
		TeamAMlbID *int    `json:"teamAMlbId"`
		TeamB      *string `json:"teamB"`
		TeamBMlbID *int    `json:"teamBMlbId"`
		SortOrder  int     `json:"sortOrder"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, "bad request")
		return
	}
	if body.ID == 0 {
		_, err := db.Exec(`INSERT INTO rounds (round_key, round_name, best_of, has_mvp, team_a, team_a_mlb_id, team_b, team_b_mlb_id, sort_order)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			body.RoundKey, body.RoundName, body.BestOf, body.HasMVP, body.TeamA, body.TeamAMlbID, body.TeamB, body.TeamBMlbID, body.SortOrder)
		if err != nil {
			writeErr(w, 500, "could not create round")
			return
		}
	} else {
		_, err := db.Exec(`UPDATE rounds SET round_name=?, best_of=?, has_mvp=?, team_a=?, team_a_mlb_id=?, team_b=?, team_b_mlb_id=?, sort_order=?
			WHERE id=?`,
			body.RoundName, body.BestOf, body.HasMVP, body.TeamA, body.TeamAMlbID, body.TeamB, body.TeamBMlbID, body.SortOrder, body.ID)
		if err != nil {
			writeErr(w, 500, "could not update round")
			return
		}
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// handleAdminSyncLock pulls Game 1's first-pitch time from the MLB API and
// sets it as the round's lock time.
func handleAdminSyncLock(w http.ResponseWriter, r *http.Request, u *User) {
	var body struct {
		RoundID   int    `json:"roundId"`
		GameTypes string `json:"gameTypes"` // e.g. "D", "L", "W"
		StartDate string `json:"startDate"` // YYYY-MM-DD
		EndDate   string `json:"endDate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, "bad request")
		return
	}
	rnd, err := getRoundByID(body.RoundID)
	if err != nil {
		writeErr(w, 404, "round not found")
		return
	}
	lockTime, err := syncRoundLockTime(*rnd, body.StartDate, body.EndDate, body.GameTypes)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	_, err = db.Exec(`UPDATE rounds SET lock_time=?, status='open' WHERE id=?`, lockTime, body.RoundID)
	if err != nil {
		writeErr(w, 500, "could not save lock time")
		return
	}
	writeJSON(w, 200, map[string]interface{}{"status": "ok", "lockTime": lockTime})
}

// handleAdminSyncResult tallies the series from the MLB API and, if it's
// over, marks the round final (winner + series length). MVP still needs to
// be set separately via handleAdminSetResult.
func handleAdminSyncResult(w http.ResponseWriter, r *http.Request, u *User) {
	var body struct {
		RoundID   int    `json:"roundId"`
		GameTypes string `json:"gameTypes"`
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, "bad request")
		return
	}
	rnd, err := getRoundByID(body.RoundID)
	if err != nil {
		writeErr(w, 404, "round not found")
		return
	}
	winnerIsA, length, clinched, err := syncRoundResult(*rnd, body.StartDate, body.EndDate, body.GameTypes)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	if !clinched {
		writeJSON(w, 200, map[string]interface{}{"status": "not final yet", "gamesPlayed": length})
		return
	}
	winner := *rnd.TeamB
	if winnerIsA {
		winner = *rnd.TeamA
	}
	_, err = db.Exec(`UPDATE rounds SET status='final', actual_winner=?, actual_length=? WHERE id=?`,
		winner, length, body.RoundID)
	if err != nil {
		writeErr(w, 500, "could not save result")
		return
	}
	writeJSON(w, 200, map[string]interface{}{"status": "ok", "winner": winner, "length": length})
}

// handleAdminSetResult is a manual override / MVP-entry endpoint - MVP
// winners aren't reliably available right away from the schedule API.
func handleAdminSetResult(w http.ResponseWriter, r *http.Request, u *User) {
	var body struct {
		RoundID      int     `json:"roundId"`
		ActualWinner *string `json:"actualWinner"`
		ActualLength *int    `json:"actualLength"`
		ActualMVP    *string `json:"actualMvp"`
		Status       string  `json:"status"` // "final" to lock it in, or "" to leave as-is
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, "bad request")
		return
	}
	status := body.Status
	if status == "" {
		status = "final"
	}
	_, err := db.Exec(`UPDATE rounds SET actual_winner=?, actual_length=?, actual_mvp=?, status=? WHERE id=?`,
		body.ActualWinner, body.ActualLength, body.ActualMVP, status, body.RoundID)
	if err != nil {
		writeErr(w, 500, "could not save result")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func getRoundByID(id int) (*Round, error) {
	row := db.QueryRow(`SELECT id, round_key, round_name, best_of, has_mvp,
		team_a, team_a_mlb_id, team_b, team_b_mlb_id, lock_time, status,
		actual_winner, actual_length, actual_mvp, sort_order FROM rounds WHERE id=?`, id)
	var rnd Round
	if err := row.Scan(&rnd.ID, &rnd.RoundKey, &rnd.RoundName, &rnd.BestOf, &rnd.HasMVP,
		&rnd.TeamA, &rnd.TeamAMlbID, &rnd.TeamB, &rnd.TeamBMlbID, &rnd.LockTime, &rnd.Status,
		&rnd.ActualWinner, &rnd.ActualLength, &rnd.ActualMVP, &rnd.SortOrder); err != nil {
		return nil, err
	}
	return &rnd, nil
}
