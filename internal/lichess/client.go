package lichess

import (
	"bufio"
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

type Client struct {
	HTTP    *http.Client
	BaseURL string
	// Token needs the team:read scope while the team's member list is private.
	Token string
}

func NewClient(token string) *Client {
	return &Client{HTTP: &http.Client{Timeout: time.Minute}, BaseURL: defaultBaseURL, Token: token}
}

func (c *Client) do(req *http.Request, limit int64) ([]byte, error) {
	req.Header.Set("User-Agent", "chess.tz-ratings/1.0")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("Lichess returned HTTP %d; set LICHESS_TOKEN with the team:read scope", res.StatusCode)
	case res.StatusCode != http.StatusOK:
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

// members reads the complete team roster. Lichess streams it as ndjson.
func (c *Client) members(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/team/"+Team+"/users", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/x-ndjson")
	data, err := c.do(req, 8<<20)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	seen := map[string]bool{}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var member struct {
			ID string `json:"id"`
		}
		if err = json.Unmarshal([]byte(line), &member); err != nil {
			return nil, fmt.Errorf("invalid team member record: %w", err)
		}
		if !userID.MatchString(member.ID) || seen[member.ID] {
			return nil, fmt.Errorf("invalid or duplicate team member ID")
		}
		seen[member.ID] = true
		ids = append(ids, member.ID)
	}
	if err = scanner.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 || len(ids) > maxMembers {
		return nil, fmt.Errorf("unexpected team size %d", len(ids))
	}
	return ids, nil
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

// Collect reads the team roster, then current ratings in serialized batches.
// Any failed request fails the whole collection so a partial roster is never saved.
func (c *Client) Collect(ctx context.Context, now time.Time) (Snapshot, error) {
	s := Snapshot{Provider: "lichess", Team: Team, Players: []Player{}, Source: Source{Team: Team}}
	ids, err := c.members(ctx)
	if err != nil {
		return s, err
	}
	s.Source.Members = len(ids)
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
	s.Source.Missing = len(ids) - len(found)
	s.Source.Players = len(s.Players)
	if len(s.Players) == 0 {
		return s, fmt.Errorf("no eligible team members")
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
