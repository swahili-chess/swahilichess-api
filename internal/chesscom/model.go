package chesscom

import "time"

// Country is the inclusion rule: accounts that list Tanzania on Chess.com.
const Country = "TZ"

var Categories = []string{"rapid", "blitz", "bullet"}

type Rating struct {
	Value int `json:"value"`
}

// Player IDs are lowercase usernames, matching Chess.com's API paths.
// PlayerID is Chess.com's stable numeric ID, used to detect renamed duplicates.
type Player struct {
	ID       string             `json:"id"`
	Name     string             `json:"name"`
	PlayerID int64              `json:"playerId"`
	Title    string             `json:"title,omitempty"`
	Ratings  map[string]*Rating `json:"ratings"`
}

// Source counts describe one collection. Excluded accounts are counted, never stored.
type Source struct {
	Country string `json:"country"`
	Listed  int    `json:"listed"`
	Known   int    `json:"known"`
	Checked int    `json:"checked"`
	Closed  int    `json:"closed"`
	Moved   int    `json:"moved"`
	Missing int    `json:"missing"`
	Players int    `json:"players"`
	// Unranked accounts are saved for rechecking but meet no category's rules.
	Unranked int `json:"unranked"`
}

type Snapshot struct {
	Provider  string    `json:"provider"`
	Country   string    `json:"country"`
	FetchedAt time.Time `json:"fetchedAt"`
	Players   []Player  `json:"players"`
	Source    Source    `json:"source"`
}

type Response struct {
	Snapshot
	Stale bool `json:"stale"`
}

const (
	refreshInterval = 24 * time.Hour
	retryInterval   = 6 * time.Hour
	// Two missed daily refreshes mark the saved ratings as stale.
	staleAfter = 48 * time.Hour
)
