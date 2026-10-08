package main

import (
	"sort"
	"strings"
)

// mvpMatches compares two MVP picks loosely, since it's a free-text field
// people fill in from memory: case and whitespace shouldn't matter, and
// neither should typing just a last name ("Ohtani") against a full name
// ("Shohei Ohtani"). It matches when every word in the shorter pick
// appears somewhere in the longer one.
func mvpMatches(a, b *string) bool {
	if a == nil || b == nil {
		return false
	}
	aWords := strings.Fields(strings.ToLower(strings.TrimSpace(*a)))
	bWords := strings.Fields(strings.ToLower(strings.TrimSpace(*b)))
	if len(aWords) == 0 || len(bWords) == 0 {
		return false
	}

	shorter, longer := aWords, bWords
	if len(longer) < len(shorter) {
		shorter, longer = longer, shorter
	}
	longerSet := make(map[string]bool, len(longer))
	for _, w := range longer {
		longerSet[w] = true
	}
	for _, w := range shorter {
		if !longerSet[w] {
			return false
		}
	}
	return true
}

// scorePick returns points for one pick against a finalized round.
// Perfect (correct winner + correct series length): 1
// Partial (correct winner, wrong length):          0.5
// Incorrect (wrong winner):                        0
// MVP bonus (LCS/WS only, correct MVP):            +2
func scorePick(round Round, pick Pick) float64 {
	if round.Status != "final" || round.ActualWinner == nil {
		return 0
	}
	points := 0.0
	if pick.PickedTeam == *round.ActualWinner {
		if round.ActualLength != nil && pick.PickedLength == *round.ActualLength {
			points += 1.0
		} else {
			points += 0.5
		}
	}
	if round.HasMVP && mvpMatches(pick.PickedMVP, round.ActualMVP) {
		points += 2.0
	}
	return points
}

func buildLeaderboard() ([]LeaderboardRow, error) {
	rows, err := db.Query(`SELECT id, display_name FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type userRef struct {
		id   int
		name string
	}
	var users []userRef
	for rows.Next() {
		var u userRef
		if err := rows.Scan(&u.id, &u.name); err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	rounds, err := fetchRounds()
	if err != nil {
		return nil, err
	}

	// Find the World Series round, if it exists, for the WSCorrect tiebreaker.
	var wsRound *Round
	for i := range rounds {
		if strings.EqualFold(rounds[i].RoundKey, "WS") {
			wsRound = &rounds[i]
			break
		}
	}

	var board []LeaderboardRow
	for _, u := range users {
		lb := LeaderboardRow{UserID: u.id, DisplayName: u.name}
		prows, err := db.Query(`SELECT id, user_id, round_id, picked_team, picked_length, picked_mvp
			FROM picks WHERE user_id = ?`, u.id)
		if err != nil {
			return nil, err
		}
		picksByRound := map[int]Pick{}
		for prows.Next() {
			var p Pick
			if err := prows.Scan(&p.ID, &p.UserID, &p.RoundID, &p.PickedTeam, &p.PickedLength, &p.PickedMVP); err != nil {
				prows.Close()
				return nil, err
			}
			picksByRound[p.RoundID] = p
		}
		prows.Close()

		for _, rnd := range rounds {
			if rnd.Status != "final" {
				continue
			}
			p, ok := picksByRound[rnd.ID]
			if !ok {
				lb.Incorrect++
				continue
			}
			pts := scorePick(rnd, p)
			lb.Points += pts

			teamCorrect := p.PickedTeam == derefStr(rnd.ActualWinner)
			lengthCorrect := rnd.ActualLength != nil && p.PickedLength == *rnd.ActualLength
			switch {
			case teamCorrect && lengthCorrect:
				lb.Perfect++
			case teamCorrect:
				lb.Partial++
			default:
				lb.Incorrect++
			}
			if rnd.HasMVP && mvpMatches(p.PickedMVP, rnd.ActualMVP) {
				lb.MVPHits++
			}
		}

		// Tiebreaker: did they pick the actual World Series winner?
		if wsRound != nil && wsRound.Status == "final" && wsRound.ActualWinner != nil {
			if p, ok := picksByRound[wsRound.ID]; ok && p.PickedTeam == *wsRound.ActualWinner {
				lb.WSCorrect = true
			}
		}

		board = append(board, lb)
	}

	sort.SliceStable(board, func(i, j int) bool {
		a, b := board[i], board[j]
		if a.Points != b.Points {
			return a.Points > b.Points
		}
		if a.Perfect != b.Perfect {
			return a.Perfect > b.Perfect
		}
		if a.WSCorrect != b.WSCorrect {
			return a.WSCorrect // true sorts before false
		}
		return a.MVPHits > b.MVPHits
	})

	return board, nil
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
