package main

import "time"

type User struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	DisplayName  string `json:"displayName"`
	IsAdmin      bool   `json:"isAdmin"`
}

type Round struct {
	ID           int        `json:"id"`
	RoundKey     string     `json:"roundKey"`
	RoundName    string     `json:"roundName"`
	BestOf       int        `json:"bestOf"`
	HasMVP       bool       `json:"hasMvp"`
	TeamA        *string    `json:"teamA"`
	TeamAMlbID   *int       `json:"teamAMlbId"`
	TeamB        *string    `json:"teamB"`
	TeamBMlbID   *int       `json:"teamBMlbId"`
	LockTime     *time.Time `json:"lockTime"`
	Status       string     `json:"status"`
	ActualWinner *string    `json:"actualWinner"`
	ActualLength *int       `json:"actualLength"`
	ActualMVP    *string    `json:"actualMvp"`
	SortOrder    int        `json:"sortOrder"`

	// Populated per-request, not stored columns:
	Locked     bool  `json:"locked"`
	MyPick     *Pick `json:"myPick,omitempty"`
	PickCount  int   `json:"pickCount,omitempty"`
}

type Pick struct {
	ID           int     `json:"id"`
	UserID       int     `json:"userId"`
	RoundID      int     `json:"roundId"`
	PickedTeam   string  `json:"pickedTeam"`
	PickedLength int     `json:"pickedLength"`
	PickedMVP    *string `json:"pickedMvp"`
}

type LeaderboardRow struct {
	UserID      int     `json:"userId"`
	DisplayName string  `json:"displayName"`
	Points      float64 `json:"points"`
	Perfect     int     `json:"perfect"`
	Partial     int     `json:"partial"`
	Incorrect   int     `json:"incorrect"`
	MVPHits     int     `json:"mvpHits"`
}
