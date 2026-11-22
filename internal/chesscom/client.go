package chesscom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultBaseURL = "https://api.chess.com"
	maxAccounts    = 20000
	attempts       = 5
)

var (
	username    = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,49}$`)
	errNotFound = errors.New("not found")
)

type profile struct {
	PlayerID int64  `json:"player_id"`
	Username string `json:"username"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Country  string `json:"country"`
	Joined   int64  `json:"joined"`
	URL      string `json:"url"`
}

// displayName keeps the capitalization players chose. The API's username is
// lowercase; the profile URL is not.
func (p profile) displayName() string {
	name := p.URL[strings.LastIndex(p.URL, "/")+1:]
	if strings.EqualFold(name, p.Username) {
		return name
	}
	return p.Username
}

// Chess.com's own leaderboard rules: the account is at least 7 days old, the
// category has at least 20 games, and the account played a rated game in the
// last 90 days. Comparison with Chess.com's Tanzania pages showed the 90 days
// apply to any category, including daily, not only the ranked one.
const (
	minAccountAge = 7 * 24 * time.Hour
	minGames      = 20
	activeWithin  = 90 * 24 * time.Hour
)

type stat struct {
	Last struct {
		Rating int   `json:"rating"`
		Date   int64 `json:"date"`
	} `json:"last"`
	Record struct {
		Win  int `json:"win"`
		Loss int `json:"loss"`
		Draw int `json:"draw"`
	} `json:"record"`
}

type Client struct {
	HTTP    *http.Client
	BaseURL string
	// Chess.com allows unlimited serial requests and may answer parallel ones
	// with HTTP 429, which get retried with backoff.
	Workers int
	Backoff time.Duration
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 30 * time.Second}, BaseURL: defaultBaseURL, Workers: 4, Backoff: 2 * time.Second}
}

// get retries rate limits, server errors and network failures. A 404 is final.
func (c *Client) get(ctx context.Context, path string, v any) error {
	var last error
	for attempt := range attempts {
		if attempt > 0 {
			timer := time.NewTimer(c.Backoff << (attempt - 1))
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "chess.tz-ratings/1.0")
		req.Header.Set("Accept", "application/json")
		res, err := c.HTTP.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			last = err
			continue
		}
		data, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
		res.Body.Close()
		switch {
		case res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusGone:
			return errNotFound
		case res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500:
			last = fmt.Errorf("Chess.com returned HTTP %d for %s", res.StatusCode, path)
			continue
		case res.StatusCode != http.StatusOK:
			return fmt.Errorf("Chess.com returned HTTP %d for %s", res.StatusCode, path)
		case err != nil:
			last = err
			continue
		}
		if err = json.Unmarshal(data, v); err != nil {
			return fmt.Errorf("invalid response for %s: %w", path, err)
		}
		return nil
	}
	return last
}

func (c *Client) countryPlayers(ctx context.Context) ([]string, error) {
	var list struct {
		Players []string `json:"players"`
	}
	if err := c.get(ctx, "/pub/country/"+Country+"/players", &list); err != nil {
		return nil, err
	}
	if len(list.Players) == 0 || len(list.Players) > maxAccounts {
		return nil, fmt.Errorf("unexpected country list size %d", len(list.Players))
	}
	return list.Players, nil
}

type outcome int

const (
	included outcome = iota
	closed
	moved
	missing
)

func (c *Client) check(ctx context.Context, id string, now time.Time) (Player, outcome, error) {
	var p profile
	if err := c.get(ctx, "/pub/player/"+id, &p); errors.Is(err, errNotFound) {
		return Player{}, missing, nil
	} else if err != nil {
		return Player{}, 0, err
	}
	if p.PlayerID <= 0 || strings.ToLower(p.Username) != id {
		return Player{}, 0, fmt.Errorf("unexpected profile for %s", id)
	}
	if strings.HasPrefix(p.Status, "closed") {
		return Player{}, closed, nil
	}
	if !strings.HasSuffix(p.Country, "/country/"+Country) {
		return Player{}, moved, nil
	}
	// Stats also carry unrelated fields such as "fide": 0, so decode categories separately.
	var stats map[string]json.RawMessage
	if err := c.get(ctx, "/pub/player/"+id+"/stats", &stats); err != nil && !errors.Is(err, errNotFound) {
		return Player{}, 0, err
	}
	player := Player{ID: id, Name: p.displayName(), PlayerID: p.PlayerID, Title: p.Title, Ratings: map[string]*Rating{}}
	// A new account stays saved so it can qualify on a later run.
	if p.Joined <= 0 || now.Sub(time.Unix(p.Joined, 0)) < minAccountAge {
		return player, included, nil
	}
	categories := map[string]stat{}
	var lastGame int64
	for key, raw := range stats {
		if !strings.HasPrefix(key, "chess") {
			continue
		}
		var s stat
		if err := json.Unmarshal(raw, &s); err != nil {
			return Player{}, 0, fmt.Errorf("invalid %s stats for %s: %w", key, id, err)
		}
		categories[key] = s
		lastGame = max(lastGame, s.Last.Date)
	}
	if now.Sub(time.Unix(lastGame, 0)) > activeWithin {
		return player, included, nil
	}
	for _, category := range Categories {
		s, ok := categories["chess_"+category]
		if !ok {
			continue
		}
		games := s.Record.Win + s.Record.Loss + s.Record.Draw
		if s.Last.Rating <= 0 || games < minGames {
			continue
		}
		player.Ratings[category] = &Rating{Value: s.Last.Rating}
	}
	return player, included, nil
}

// Collect discovers accounts from the country list, keeps previously known
// accounts that Chess.com no longer lists, and rechecks every account's status
// and country. Any unrecoverable request error fails the whole collection.
func (c *Client) Collect(ctx context.Context, now time.Time, known []string) (Snapshot, error) {
	s := Snapshot{Provider: "chesscom", Country: Country, Players: []Player{}, Source: Source{Country: Country}}
	listed, err := c.countryPlayers(ctx)
	if err != nil {
		return s, err
	}
	s.Source.Listed, s.Source.Known = len(listed), len(known)
	seen := map[string]bool{}
	ids := []string{}
	for _, name := range append(listed, known...) {
		id := strings.ToLower(name)
		if !username.MatchString(id) {
			return s, fmt.Errorf("invalid username in country list")
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) > maxAccounts {
		return s, fmt.Errorf("too many accounts")
	}
	s.Source.Checked = len(ids)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan string)
	var mu sync.Mutex
	var firstErr error
	byPlayerID := map[int64]Player{}
	var wg sync.WaitGroup
	for range max(1, c.Workers) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				player, result, err := c.check(ctx, id, now)
				mu.Lock()
				switch {
				case err != nil:
					if firstErr == nil {
						firstErr = err
						cancel()
					}
				case result == closed:
					s.Source.Closed++
				case result == moved:
					s.Source.Moved++
				case result == missing:
					s.Source.Missing++
				default:
					// A renamed account can appear under its old and new names. The
					// profile's own username wins.
					byPlayerID[player.PlayerID] = player
				}
				mu.Unlock()
			}
		}()
	}
	for _, id := range ids {
		select {
		case jobs <- id:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(jobs)
	wg.Wait()
	if firstErr != nil {
		return s, firstErr
	}
	if err = ctx.Err(); err != nil {
		return s, err
	}
	for _, player := range byPlayerID {
		s.Players = append(s.Players, player)
	}
	s.Source.Players = len(s.Players)
	for _, player := range s.Players {
		if len(player.Ratings) == 0 {
			s.Source.Unranked++
		}
	}
	if len(s.Players) == 0 {
		return s, fmt.Errorf("no eligible accounts")
	}
	sort.Slice(s.Players, func(i, j int) bool { return s.Players[i].ID < s.Players[j].ID })
	s.FetchedAt = now.UTC()
	return s, nil
}
