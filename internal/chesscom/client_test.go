package chesscom

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func respond(status int, body string) (*http.Response, error) {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
}

func fakeClient(routes map[string]string) *Client {
	return &Client{BaseURL: "https://chess.test", Workers: 3, Backoff: time.Millisecond, HTTP: &http.Client{Transport: transport(func(req *http.Request) (*http.Response, error) {
		body, ok := routes[req.URL.Path]
		if !ok {
			return respond(404, `{"message":"not found"}`)
		}
		return respond(200, body)
	})}}
}

const tz = `"country":"https://api.chess.com/pub/country/TZ"`

func TestCollectAppliesChessComLeaderboardRules(t *testing.T) {
	now := time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)
	joined := fmt.Sprintf(`,"joined":%d`, now.AddDate(-1, 0, 0).Unix())
	stat := func(rating, games int, last time.Time) string {
		return fmt.Sprintf(`{"last":{"rating":%d,"date":%d},"record":{"win":%d,"loss":0,"draw":0}}`, rating, last.Unix(), games)
	}
	recent, lapsed := now.AddDate(0, 0, -10), now.AddDate(0, 0, -91)
	c := fakeClient(map[string]string{
		"/pub/country/TZ/players": `{"players":["Alice","bob","closedone","movedaway","deleted","newname","newbie","lapsed"]}`,
		"/pub/player/alice":       `{"player_id":1,"username":"alice","url":"https://www.chess.com/member/Alice","title":"IM","status":"premium",` + tz + joined + `}`,
		// Rapid qualifies; Blitz has no games. Bullet's last game is 91 days old but
		// counts because Alice played Rapid recently.
		"/pub/player/alice/stats": `{"chess_rapid":` + stat(2100, 25, recent) + `,"chess_blitz":` + stat(1200, 0, recent) + `,"chess_bullet":` + stat(2500, 300, lapsed) + `,"fide":0,"tactics":{"highest":{"rating":2272}}}`,
		"/pub/player/bob":         `{"player_id":2,"username":"bob","status":"basic",` + tz + joined + `}`,
		"/pub/player/bob/stats":   `{}`,
		"/pub/player/closedone":   `{"player_id":3,"username":"closedone","status":"closed:fair_play_violations",` + tz + joined + `}`,
		"/pub/player/movedaway":   `{"player_id":4,"username":"movedaway","status":"basic","country":"https://api.chess.com/pub/country/KE"` + joined + `}`,
		"/pub/player/newname":     `{"player_id":5,"username":"NewName","status":"basic",` + tz + joined + `}`,
		// Exactly 20 Blitz games qualify; 19 Bullet games do not.
		"/pub/player/newname/stats": `{"chess_blitz":` + stat(1800, 20, recent) + `,"chess_bullet":` + stat(1900, 19, recent) + `}`,
		"/pub/player/newbie":        fmt.Sprintf(`{"player_id":7,"username":"newbie","status":"basic",%s,"joined":%d}`, tz, now.AddDate(0, 0, -3).Unix()),
		"/pub/player/newbie/stats":  `{"chess_rapid":` + stat(1700, 40, recent) + `}`,
		// Lapsed has 300 Blitz games, none in the last 91 days, and no other games.
		"/pub/player/lapsed":          `{"player_id":8,"username":"lapsed","status":"basic",` + tz + joined + `}`,
		"/pub/player/lapsed/stats":    `{"chess_blitz":` + stat(2500, 300, lapsed) + `,"fide":2100}`,
		"/pub/player/oldfriend":       `{"player_id":6,"username":"oldfriend","status":"basic",` + tz + joined + `}`,
		"/pub/player/oldfriend/stats": `{"chess_blitz":` + stat(1500, 21, recent) + `}`,
	})
	// oldfriend is no longer listed but was saved last time, so it is rechecked.
	s, err := c.Collect(context.Background(), now, []string{"oldfriend", "alice"})
	if err != nil {
		t.Fatal(err)
	}
	players := map[string]Player{}
	ids := []string{}
	for _, p := range s.Players {
		ids = append(ids, p.ID)
		players[p.ID] = p
	}
	if strings.Join(ids, ",") != "alice,bob,lapsed,newbie,newname,oldfriend" {
		t.Fatalf("players %v", ids)
	}
	alice := players["alice"]
	if alice.Name != "Alice" || alice.Title != "IM" || alice.PlayerID != 1 || alice.Ratings["rapid"].Value != 2100 || alice.Ratings["blitz"] != nil || alice.Ratings["bullet"].Value != 2500 {
		t.Fatalf("alice %+v", alice)
	}
	if players["newname"].Ratings["blitz"] == nil || players["newname"].Ratings["bullet"] != nil {
		t.Fatalf("20-game rule: %+v", players["newname"].Ratings)
	}
	if len(players["newbie"].Ratings) != 0 || len(players["bob"].Ratings) != 0 || len(players["lapsed"].Ratings) != 0 {
		t.Fatal("new, gameless or inactive accounts must not be ranked")
	}
	want := Source{Country: "TZ", Listed: 8, Known: 2, Checked: 9, Closed: 1, Moved: 1, Missing: 1, Players: 6, Unranked: 3}
	if s.Source != want {
		t.Fatalf("source %+v, want %+v", s.Source, want)
	}
}

func TestCollectRetriesRateLimits(t *testing.T) {
	var calls atomic.Int32
	c := &Client{BaseURL: "https://chess.test", Workers: 2, Backoff: time.Millisecond, HTTP: &http.Client{Transport: transport(func(req *http.Request) (*http.Response, error) {
		if calls.Add(1)%2 == 1 {
			return respond(429, "")
		}
		switch req.URL.Path {
		case "/pub/country/TZ/players":
			return respond(200, `{"players":["alice"]}`)
		case "/pub/player/alice":
			return respond(200, `{"player_id":1,"username":"alice","status":"basic",`+tz+`}`)
		}
		return respond(200, `{}`)
	})}}
	s, err := c.Collect(context.Background(), time.Now(), nil)
	if err != nil || len(s.Players) != 1 {
		t.Fatalf("%+v %v", s, err)
	}
}

func TestCollectFailsWholeRunOnPersistentErrors(t *testing.T) {
	cases := map[string]*Client{
		"empty list":       fakeClient(map[string]string{"/pub/country/TZ/players": `{"players":[]}`}),
		"invalid username": fakeClient(map[string]string{"/pub/country/TZ/players": `{"players":["../admin"]}`}),
		"wrong profile": fakeClient(map[string]string{
			"/pub/country/TZ/players": `{"players":["alice"]}`,
			"/pub/player/alice":       `{"player_id":1,"username":"mallory","status":"basic",` + tz + `}`,
		}),
		"truncated stats": fakeClient(map[string]string{
			"/pub/country/TZ/players": `{"players":["alice"]}`,
			"/pub/player/alice":       `{"player_id":1,"username":"alice","status":"basic",` + tz + `}`,
			"/pub/player/alice/stats": `{"chess_rapid":`,
		}),
		"no eligible accounts": fakeClient(map[string]string{
			"/pub/country/TZ/players": `{"players":["alice"]}`,
			"/pub/player/alice":       `{"player_id":1,"username":"alice","status":"closed",` + tz + `}`,
		}),
	}
	for _, status := range []int{429, 500, 403} {
		cases["HTTP "+http.StatusText(status)] = &Client{BaseURL: "https://chess.test", Workers: 1, Backoff: time.Millisecond, HTTP: &http.Client{Transport: transport(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path == "/pub/country/TZ/players" {
				return respond(200, `{"players":["alice","bob"]}`)
			}
			return respond(status, "")
		})}}
	}
	for name, c := range cases {
		if _, err := c.Collect(context.Background(), time.Now(), nil); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
