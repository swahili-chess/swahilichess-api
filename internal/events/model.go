package events

import "time"

// Event is one online tournament. IDs combine provider, system and the
// provider's ID, so the same tournament found through two clubs is merged.
type Event struct {
	ID         string     `json:"id"`
	Provider   string     `json:"provider"`
	System     string     `json:"system"`
	SourceID   string     `json:"sourceId"`
	Origin     string     `json:"origin"`
	Clubs      []string   `json:"clubs,omitempty"`
	Name       string     `json:"name"`
	URL        string     `json:"url"`
	StartsAt   time.Time  `json:"startsAt"`
	FinishesAt *time.Time `json:"finishesAt,omitempty"`
	Status     string     `json:"status"`
	Variant    string     `json:"variant"`
	Speed      string     `json:"speed"`
	Clock      Clock      `json:"clock"`
	Rated      bool       `json:"rated"`
	Rounds     int        `json:"rounds,omitempty"`
	Schedule   string     `json:"schedule,omitempty"`
}

// Clock limit and increment are in seconds, as Lichess reports them.
type Clock struct {
	Limit     int `json:"limit"`
	Increment int `json:"increment"`
}

type Club struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Snapshot struct {
	FetchedAt time.Time `json:"fetchedAt"`
	Clubs     []Club    `json:"clubs"`
	Events    []Event   `json:"events"`
}

type Response struct {
	Snapshot
	Stale bool `json:"stale"`
}

const (
	OriginClub     = "club"
	OriginOfficial = "official"

	// Clubs create tournaments shortly before play, sometimes an hour ahead.
	refreshInterval = 15 * time.Minute
	staleAfter      = time.Hour
	// Finished club tournaments stay visible in the calendar's past view.
	keepFinished = 365 * 24 * time.Hour
)
