package fide

import "time"

var Categories = []string{"standard", "rapid", "blitz"}

type Rating struct {
	Value int `json:"value"`
}

type Player struct {
	// Archive activity is retained for eligibility decisions, not exposed as a player-wide API flag.
	Inactive bool               `json:"-"`
	ID       string             `json:"id"`
	Name     string             `json:"name"`
	Title    string             `json:"title,omitempty"`
	Ratings  map[string]*Rating `json:"ratings"`
}

type Source struct {
	URL      string `json:"url"`
	SHA256   string `json:"sha256"`
	Records  int    `json:"records"`
	Tanzania int    `json:"tanzania"`
	Inactive int    `json:"inactive"`
	Excluded int    `json:"excluded"`
	Rated    int    `json:"rated"`
}

type Snapshot struct {
	Eligibility string            `json:"eligibility"`
	Provider    string            `json:"provider"`
	Federation  string            `json:"federation"`
	RatingMonth string            `json:"ratingMonth"`
	FetchedAt   time.Time         `json:"fetchedAt"`
	Players     []Player          `json:"players"`
	Sources     map[string]Source `json:"sources"`
}

type Response struct {
	Snapshot
	Stale bool `json:"stale"`
}

func month(t time.Time) string { return t.UTC().Format("2006-01") }

func nextPublication(period string, now time.Time) time.Time {
	p, _ := time.Parse("2006-01", period)
	next := p.AddDate(0, 1, 0).Add(6 * time.Hour)
	if !next.After(now) {
		return now.Add(24 * time.Hour)
	}
	return next
}
