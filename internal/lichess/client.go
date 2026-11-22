package lichess

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://lichess.org"
	// The batch user endpoint accepts at most 300 IDs per request.
	batchSize  = 300
	maxMembers = 5000
)

var userID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,29}$`)

type perf struct {
	Games  int  `json:"games"`
	Rating int  `json:"rating"`
	Prov   bool `json:"prov"`
}

type user struct {
	ID           string          `json:"id"`
	Username     string          `json:"username"`
	Title        string          `json:"title"`
	Disabled     bool            `json:"disabled"`
	TosViolation bool            `json:"tosViolation"`
	Perfs        map[string]perf `json:"perfs"`
}

// Roster returns the Lichess IDs to rank. Production reads the chessbot's lichess table,
// so no Lichess token is needed: the batch users endpoint is public.
type Roster func(context.Context) ([]string, error)

type Client struct {
	HTTP    *http.Client
	BaseURL string
	Roster  Roster
}

func NewClient(roster Roster) *Client {
	return &Client{HTTP: &http.Client{Timeout: time.Minute}, BaseURL: defaultBaseURL, Roster: roster}
}

func (c *Client) do(req *http.Request, limit int64) ([]byte, error) {
	req.Header.Set("User-Agent", "chess.tz-ratings/1.0")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Lichess returned HTTP %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("Lichess response exceeds size limit")
	}
	return data, nil
}

// members reads and normalizes the roster. The bot writes these rows, so malformed IDs are
// skipped and counted rather than blocking every refresh; duplicates collapse to one.
func (c *Client) members(ctx context.Context) (ids []string, skipped int, err error) {
	if c.Roster == nil {
		return nil, 0, fmt.Errorf("no Lichess roster configured")
	}
	raw, err := c.Roster(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("read Lichess roster: %w", err)
	}
	seen := map[string]bool{}
	for _, id := range raw {
		id = strings.ToLower(strings.TrimSpace(id))
		if !userID.MatchString(id) {
			skipped++
			continue
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 || len(ids) > maxMembers {
		return nil, skipped, fmt.Errorf("unexpected roster size %d", len(ids))
	}
	return ids, skipped, nil
}

func (c *Client) users(ctx context.Context, ids []string) ([]user, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/users", strings.NewReader(strings.Join(ids, ",")))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/plain")
	data, err := c.do(req, 16<<20)
	if err != nil {
		return nil, err
	}
	var users []user
	if err = json.Unmarshal(data, &users); err != nil {
		return nil, fmt.Errorf("invalid users response: %w", err)
	}
	return users, nil
}

// Collect reads the roster, then current ratings in serialized batches.
// Any failed request fails the whole collection so a partial roster is never saved.
func (c *Client) Collect(ctx context.Context, now time.Time) (Snapshot, error) {
	s := Snapshot{Provider: "lichess", Team: Team, Players: []Player{}, Source: Source{Team: Team}}
	ids, skipped, err := c.members(ctx)
	if err != nil {
		return s, err
	}
	s.Source.Members = len(ids) + skipped
	requested := map[string]bool{}
	for _, id := range ids {
		requested[id] = true
	}
	found := map[string]bool{}
	for start := 0; start < len(ids); start += batchSize {
		batch, err := c.users(ctx, ids[start:min(start+batchSize, len(ids))])
		if err != nil {
			return s, err
		}
		for _, u := range batch {
			if !requested[u.ID] || found[u.ID] || !strings.EqualFold(u.Username, u.ID) {
				return s, fmt.Errorf("unexpected user in response")
			}
			found[u.ID] = true
			switch {
			case u.Disabled:
				s.Source.Closed++
			case u.TosViolation:
				s.Source.Violated++
			case u.Title == "BOT":
				s.Source.Bots++
			default:
				s.Players = append(s.Players, toPlayer(u))
			}
		}
	}
	s.Source.Missing = len(ids) - len(found) + skipped
	s.Source.Players = len(s.Players)
	if len(s.Players) == 0 {
		return s, fmt.Errorf("no eligible roster members")
	}
	sort.Slice(s.Players, func(i, j int) bool { return s.Players[i].ID < s.Players[j].ID })
	s.FetchedAt = now.UTC()
	return s, nil
}

// A category without games carries Lichess's 1500 placeholder, so it stays null.
func toPlayer(u user) Player {
	p := Player{ID: u.ID, Name: u.Username, Title: u.Title, Ratings: map[string]*Rating{}}
	for _, category := range Categories {
		perf, ok := u.Perfs[category]
		if !ok || perf.Games <= 0 || perf.Rating <= 0 {
			continue
		}
		p.Ratings[category] = &Rating{Value: perf.Rating, Provisional: perf.Prov}
	}
	return p
}
