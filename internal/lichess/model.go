package lichess

import "time"

// Team membership is the inclusion rule: Lichess has no country leaderboard.
const Team = "nyumbani-mates"

var Categories = []string{"rapid", "blitz", "bullet"}

type Rating struct {
	Value       int  `json:"value"`
	Provisional bool `json:"provisional,omitempty"`
}

type Player struct {
	ID      string             `json:"id"`
	Name    string             `json:"name"`
	Title   string             `json:"title,omitempty"`
	Ratings map[string]*Rating `json:"ratings"`
}

// Source counts describe one collection. Excluded accounts are counted, never stored.
type Source struct {
	Team     string `json:"team"`
	Members  int    `json:"members"`
	Closed   int    `json:"closed"`
	Violated int    `json:"violated"`
	Bots     int    `json:"bots"`
	Missing  int    `json:"missing"`
	Players  int    `json:"players"`
}

type Snapshot struct {
	Provider  string    `json:"provider"`
	Team      string    `json:"team"`
	FetchedAt time.Time `json:"fetchedAt"`
	Players   []Player  `json:"players"`
	Source    Source    `json:"source"`
}

type Response struct {
	Snapshot
	Stale bool `json:"stale"`
}

const (
	refreshInterval = time.Hour
	// Three missed hourly refreshes mark the saved ratings as stale.
	staleAfter = 3 * time.Hour
)
