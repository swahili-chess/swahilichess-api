package events

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"api.swahilichess.com/config"
)

const defaultBaseURL = "https://lichess.org"

var tournamentID = regexp.MustCompile(`^[A-Za-z0-9]{8}$`)

// Official Lichess tournaments kept for the calendar: standard chess or Chess960
// from 1+0 to 15+10, plus one-off events such as the Titled Arena and its warm-ups.
var officialVariants = map[string]bool{"standard": true, "chess960": true}

const (
	shortestClock = 60          // 1+0
	longestClock  = 900 + 40*10 // 15+10, as an estimated duration
)

func keepOfficial(e Event) bool {
	if !officialVariants[e.Variant] {
		return false
	}
	if e.Schedule == "unique" {
		return true
	}
	d := e.Clock.Limit + 40*e.Clock.Increment
	return d >= shortestClock && d <= longestClock
}

type Client struct {
	HTTP    *http.Client
	BaseURL string
	Clubs   []config.Club
}

func NewClient(clubs []config.Club) *Client {
	return &Client{HTTP: &http.Client{Timeout: time.Minute}, BaseURL: defaultBaseURL, Clubs: clubs}
}

func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "chess.tz-events/1.0")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Lichess returned HTTP %d for %s", res.StatusCode, path)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, (8<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 8<<20 {
		return nil, fmt.Errorf("Lichess response exceeds size limit")
	}
	return data, nil
}

// Arena and Swiss records differ: Arena uses fullName, millisecond startsAt,
// numeric status and a variant object; Swiss uses name, ISO startsAt, string
// status and a variant string.
type record struct {
	ID         string          `json:"id"`
	FullName   string          `json:"fullName"`
	Name       string          `json:"name"`
	CreatedBy  string          `json:"createdBy"`
	StartsAt   json.RawMessage `json:"startsAt"`
	FinishesAt int64           `json:"finishesAt"`
	Minutes    int             `json:"minutes"`
	Status     json.RawMessage `json:"status"`
	Variant    json.RawMessage `json:"variant"`
	Clock      Clock           `json:"clock"`
	Rated      bool            `json:"rated"`
	NbRounds   int             `json:"nbRounds"`
	Schedule   *struct {
		Freq string `json:"freq"`
	} `json:"schedule"`
}

func (r record) toEvent(system, origin string) (Event, error) {
	e := Event{Provider: "lichess", System: system, SourceID: r.ID, Origin: origin, Rated: r.Rated, Clock: r.Clock, Rounds: r.NbRounds}
	if !tournamentID.MatchString(r.ID) {
		return e, fmt.Errorf("invalid tournament ID %q", r.ID)
	}
	e.ID = "lichess-" + system + "-" + r.ID
	e.Name = strings.TrimSpace(r.FullName)
	if e.Name == "" {
		e.Name = strings.TrimSpace(r.Name)
	}
	if e.Name == "" || len(e.Name) > 200 {
		return e, fmt.Errorf("invalid name for %s", r.ID)
	}
	var millis int64
	var iso string
	if json.Unmarshal(r.StartsAt, &millis) == nil && millis > 0 {
		e.StartsAt = time.UnixMilli(millis).UTC()
	} else if json.Unmarshal(r.StartsAt, &iso) == nil {
		t, err := time.Parse(time.RFC3339, iso)
		if err != nil {
			return e, fmt.Errorf("invalid start for %s", r.ID)
		}
		e.StartsAt = t.UTC()
	} else {
		return e, fmt.Errorf("missing start for %s", r.ID)
	}
	if r.FinishesAt > 0 {
		t := time.UnixMilli(r.FinishesAt).UTC()
		e.FinishesAt = &t
	} else if r.Minutes > 0 {
		t := e.StartsAt.Add(time.Duration(r.Minutes) * time.Minute)
		e.FinishesAt = &t
	}
	var code int
	var status string
	if json.Unmarshal(r.Status, &code) == nil {
		status = map[int]string{10: "created", 20: "started", 30: "finished"}[code]
	} else {
		_ = json.Unmarshal(r.Status, &status)
	}
	e.Status = map[string]string{"created": "upcoming", "started": "ongoing", "finished": "finished"}[status]
	if e.Status == "" {
		return e, fmt.Errorf("unknown status for %s", r.ID)
	}
	var variant struct {
		Key string `json:"key"`
	}
	if json.Unmarshal(r.Variant, &variant) != nil || variant.Key == "" {
		if json.Unmarshal(r.Variant, &variant.Key) != nil || variant.Key == "" {
			return e, fmt.Errorf("unknown variant for %s", r.ID)
		}
	}
	e.Variant = variant.Key
	if r.Clock.Limit < 0 || r.Clock.Increment < 0 || r.Clock.Limit+r.Clock.Increment == 0 {
		return e, fmt.Errorf("invalid clock for %s", r.ID)
	}
	e.Speed = speed(r.Clock)
	e.URL = "https://lichess.org/" + map[string]string{"arena": "tournament", "swiss": "swiss"}[system] + "/" + r.ID
	if r.Schedule != nil {
		e.Schedule = r.Schedule.Freq
	}
	return e, nil
}

// speed follows Lichess's estimated game duration: limit + 40 × increment.
func speed(c Clock) string {
	switch d := c.Limit + 40*c.Increment; {
	case d < 30:
		return "ultraBullet"
	case d < 180:
		return "bullet"
	case d < 480:
		return "blitz"
	case d < 1500:
		return "rapid"
	default:
		return "classical"
	}
}

func parseNDJSON(data []byte, system, origin string) ([]Event, error) {
	events := []Event{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var r record
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, fmt.Errorf("invalid %s record: %w", system, err)
		}
		e, err := r.toEvent(system, origin)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, scanner.Err()
}

// Collect reads each recognized club's Arena and Swiss feeds, then Lichess's own
// current tournaments, one request at a time. Any failure fails the whole run.
func (c *Client) Collect(ctx context.Context, now time.Time) (Snapshot, error) {
	s := Snapshot{Clubs: []Club{}, Events: []Event{}}
	byID := map[string]*Event{}
	add := func(e Event, club string) {
		if existing, ok := byID[e.ID]; ok {
			if club != "" {
				existing.Clubs = append(existing.Clubs, club)
			}
			return
		}
		if club != "" {
			e.Clubs = []string{club}
		}
		byID[e.ID] = &e
	}
	for _, club := range c.Clubs {
		s.Clubs = append(s.Clubs, Club{ID: club.ID, Name: club.Name, URL: "https://lichess.org/team/" + club.ID})
		for _, system := range []string{"arena", "swiss"} {
			data, err := c.get(ctx, "/api/team/"+club.ID+"/"+system+"?max=100")
			if err != nil {
				return s, err
			}
			events, err := parseNDJSON(data, system, OriginClub)
			if err != nil {
				return s, fmt.Errorf("%s: %w", club.ID, err)
			}
			for _, e := range events {
				if e.Status == "finished" && now.Sub(e.StartsAt) > keepFinished {
					continue
				}
				add(e, club.ID)
			}
		}
	}
	data, err := c.get(ctx, "/api/tournament")
	if err != nil {
		return s, err
	}
	var official map[string][]json.RawMessage
	if err = json.Unmarshal(data, &official); err != nil {
		return s, fmt.Errorf("invalid official tournaments: %w", err)
	}
	// Finished official tournaments are dropped; hourly events would flood the past view.
	for _, group := range []string{"created", "started"} {
		for _, raw := range official[group] {
			var r record
			if err = json.Unmarshal(raw, &r); err != nil {
				return s, fmt.Errorf("invalid official tournament: %w", err)
			}
			if r.CreatedBy != "lichess" {
				continue
			}
			e, err := r.toEvent("arena", OriginOfficial)
			if err != nil {
				return s, err
			}
			if keepOfficial(e) {
				add(e, "")
			}
		}
	}
	for _, e := range byID {
		s.Events = append(s.Events, *e)
	}
	sort.Slice(s.Events, func(i, j int) bool {
		if !s.Events[i].StartsAt.Equal(s.Events[j].StartsAt) {
			return s.Events[i].StartsAt.Before(s.Events[j].StartsAt)
		}
		return s.Events[i].ID < s.Events[j].ID
	})
	s.FetchedAt = now.UTC()
	return s, nil
}
