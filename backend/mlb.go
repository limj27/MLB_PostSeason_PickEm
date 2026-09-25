package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const mlbAPIBase = "https://statsapi.mlb.com/api/v1"

type mlbTeamsResponse struct {
	Teams []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"teams"`
}

// fetchMLBTeams returns all current MLB team names + IDs, used to populate
// the admin dropdowns when setting up a round's matchup.
func fetchMLBTeams() ([]struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}, error) {
	url := mlbAPIBase + "/teams?sportId=1&activeStatus=Y"
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var parsed mlbTeamsResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	return parsed.Teams, nil
}

type mlbScheduleResponse struct {
	Dates []struct {
		Games []struct {
			GamePk   int    `json:"gamePk"`
			GameDate string `json:"gameDate"`
			Status   struct {
				DetailedState string `json:"detailedState"`
			} `json:"status"`
			Teams struct {
				Away struct {
					Team struct {
						ID int `json:"id"`
					} `json:"team"`
					IsWinner bool `json:"isWinner"`
				} `json:"away"`
				Home struct {
					Team struct {
						ID int `json:"id"`
					} `json:"team"`
					IsWinner bool `json:"isWinner"`
				} `json:"home"`
			} `json:"teams"`
		} `json:"games"`
	} `json:"dates"`
}

type seriesGame struct {
	GamePk      int
	GameTime    time.Time
	Final       bool
	AWinnerTeam bool // true if teamA (the queried team) won this game
	BWinnerTeam bool
}

// fetchSeriesGames pulls postseason games involving teamAID within the given
// date window, then keeps only the ones that are also against teamBID -
// i.e. the actual series between the two clubs. gameTypes is a comma list
// such as "D" (Division Series), "L" (Championship Series), "W" (World Series).
func fetchSeriesGames(teamAID, teamBID int, gameTypes, startDate, endDate string) ([]seriesGame, error) {
	url := fmt.Sprintf("%s/schedule?sportId=1&teamId=%d&gameTypes=%s&startDate=%s&endDate=%s",
		mlbAPIBase, teamAID, gameTypes, startDate, endDate)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var parsed mlbScheduleResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}

	var games []seriesGame
	for _, d := range parsed.Dates {
		for _, g := range d.Games {
			homeID := g.Teams.Home.Team.ID
			awayID := g.Teams.Away.Team.ID
			// keep only games that are exactly teamA vs teamB
			if (homeID == teamAID && awayID == teamBID) || (homeID == teamBID && awayID == teamAID) {
				t, _ := time.Parse(time.RFC3339, g.GameDate)
				sg := seriesGame{
					GamePk:   g.GamePk,
					GameTime: t,
					Final:    g.Status.DetailedState == "Final" || g.Status.DetailedState == "Game Over",
				}
				if homeID == teamAID {
					sg.AWinnerTeam = g.Teams.Home.IsWinner
					sg.BWinnerTeam = g.Teams.Away.IsWinner
				} else {
					sg.AWinnerTeam = g.Teams.Away.IsWinner
					sg.BWinnerTeam = g.Teams.Home.IsWinner
				}
				games = append(games, sg)
			}
		}
	}
	return games, nil
}

// syncRoundLockTime finds the earliest game between the round's two teams
// and sets lock_time to that first pitch.
func syncRoundLockTime(round Round, startDate, endDate, gameTypes string) (*time.Time, error) {
	if round.TeamAMlbID == nil || round.TeamBMlbID == nil {
		return nil, fmt.Errorf("round is missing team MLB ids")
	}
	games, err := fetchSeriesGames(*round.TeamAMlbID, *round.TeamBMlbID, gameTypes, startDate, endDate)
	if err != nil {
		return nil, err
	}
	if len(games) == 0 {
		return nil, fmt.Errorf("no games found for this matchup in that date range")
	}
	earliest := games[0].GameTime
	for _, g := range games[1:] {
		if g.GameTime.Before(earliest) {
			earliest = g.GameTime
		}
	}
	return &earliest, nil
}

// syncRoundResult tallies completed games between the two teams and, once
// one side has clinched (best_of/2 + 1 wins), returns the winner name and
// series length. MVP is not available reliably from this endpoint right
// after a series ends, so it stays a manual admin entry.
func syncRoundResult(round Round, startDate, endDate, gameTypes string) (winnerIsA bool, seriesLength int, clinched bool, err error) {
	if round.TeamAMlbID == nil || round.TeamBMlbID == nil {
		return false, 0, false, fmt.Errorf("round is missing team MLB ids")
	}
	games, err := fetchSeriesGames(*round.TeamAMlbID, *round.TeamBMlbID, gameTypes, startDate, endDate)
	if err != nil {
		return false, 0, false, err
	}
	winsNeeded := round.BestOf/2 + 1
	aWins, bWins, played := 0, 0, 0
	for _, g := range games {
		if !g.Final {
			continue
		}
		played++
		if g.AWinnerTeam {
			aWins++
		} else if g.BWinnerTeam {
			bWins++
		}
	}
	if aWins >= winsNeeded {
		return true, played, true, nil
	}
	if bWins >= winsNeeded {
		return false, played, true, nil
	}
	return false, played, false, nil
}
