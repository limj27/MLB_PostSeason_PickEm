package main

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
	if round.HasMVP && round.ActualMVP != nil && pick.PickedMVP != nil &&
		*pick.PickedMVP == *round.ActualMVP {
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
			if rnd.HasMVP && rnd.ActualMVP != nil && p.PickedMVP != nil && *p.PickedMVP == *rnd.ActualMVP {
				lb.MVPHits++
			}
		}
		board = append(board, lb)
	}
	return board, nil
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
