package lichess

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func respond(status int, body string) (*http.Response, error) {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
}

func fakeClient(members []string, users func([]string) string) (*Client, *[]int) {
	batches := []int{}
	c := &Client{BaseURL: "https://lichess.test", Token: "test-token", HTTP: &http.Client{Transport: transport(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Authorization") != "Bearer test-token" {
			return respond(401, "")
		}
		if req.Method == http.MethodGet && req.URL.Path == "/api/team/"+Team+"/users" {
			lines := []string{}
			for _, id := range members {
				lines = append(lines, fmt.Sprintf(`{"id":%q,"name":%q}`, id, id))
			}
			return respond(200, strings.Join(lines, "\n")+"\n")
		}
		if req.Method == http.MethodPost && req.URL.Path == "/api/users" {
			body, _ := io.ReadAll(req.Body)
			ids := strings.Split(string(body), ",")
			batches = append(batches, len(ids))
			return respond(200, users(ids))
		}
		return respond(404, "")
	})}}
	return c, &batches
}

func TestCollectAppliesTeamEligibilityAndNullRatings(t *testing.T) {
	members := []string{"alice", "bob", "closed1", "cheater", "somebot", "gone"}
	c, _ := fakeClient(members, func([]string) string {
		return `[
			{"id":"alice","username":"Alice","title":"FM","perfs":{"rapid":{"games":40,"rating":2100},"blitz":{"games":5,"rating":1900,"prov":true},"bullet":{"games":0,"rating":1500,"prov":true}}},
			{"id":"bob","username":"bob","perfs":{"rapid":{"games":3,"rating":1700}}},
			{"id":"closed1","username":"closed1","disabled":true},
			{"id":"cheater","username":"Cheater","tosViolation":true,"perfs":{"rapid":{"games":90,"rating":2500}}},
			{"id":"somebot","username":"SomeBot","title":"BOT","perfs":{"rapid":{"games":90,"rating":2600}}}
		]`
	})
	s, err := c.Collect(context.Background(), time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Players) != 2 || s.Players[0].ID != "alice" || s.Players[1].ID != "bob" {
		t.Fatalf("wrong players: %+v", s.Players)
	}
	alice := s.Players[0]
	if alice.Name != "Alice" || alice.Title != "FM" || alice.Ratings["rapid"].Value != 2100 || !alice.Ratings["blitz"].Provisional {
		t.Fatalf("wrong ratings: %+v", alice)
	}
	if alice.Ratings["bullet"] != nil || s.Players[1].Ratings["blitz"] != nil {
		t.Fatal("unplayed category must stay null, not the 1500 placeholder")
	}
	want := Source{Team: Team, Members: 6, Closed: 1, Violated: 1, Bots: 1, Missing: 1, Players: 2}
	if s.Source != want {
		t.Fatalf("source %+v, want %+v", s.Source, want)
	}
}

func TestCollectBatchesAtMost300(t *testing.T) {
	members := []string{}
	for i := range 650 {
		members = append(members, fmt.Sprintf("player%d", i))
	}
	c, batches := fakeClient(members, func(ids []string) string {
		users := []string{}
		for _, id := range ids {
			users = append(users, fmt.Sprintf(`{"id":%q,"username":%q,"perfs":{"blitz":{"games":1,"rating":1600}}}`, id, id))
		}
		return "[" + strings.Join(users, ",") + "]"
	})
	s, err := c.Collect(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(*batches) != "[300 300 50]" || len(s.Players) != 650 {
		t.Fatalf("batches %v, players %d", *batches, len(s.Players))
	}
}

func TestCollectRejectsInvalidResponses(t *testing.T) {
	cases := map[string]*Client{}
	cases["unexpected user"], _ = fakeClient([]string{"alice"}, func([]string) string {
		return `[{"id":"mallory","username":"mallory"}]`
	})
	cases["duplicate user"], _ = fakeClient([]string{"alice"}, func([]string) string {
		return `[{"id":"alice","username":"alice"},{"id":"alice","username":"alice"}]`
	})
	cases["truncated JSON"], _ = fakeClient([]string{"alice"}, func([]string) string { return `[{"id":"alice"` })
	cases["no eligible players"], _ = fakeClient([]string{"alice"}, func([]string) string {
		return `[{"id":"alice","username":"alice","disabled":true}]`
	})
	cases["empty team"], _ = fakeClient(nil, func([]string) string { return `[]` })
	cases["invalid member ID"], _ = fakeClient([]string{"../admin"}, func([]string) string { return `[]` })
	missingToken, _ := fakeClient([]string{"alice"}, func([]string) string { return `[]` })
	missingToken.Token = ""
	cases["missing token"] = missingToken
	for name, c := range cases {
		if _, err := c.Collect(context.Background(), time.Now()); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	for _, status := range []int{429, 500} {
		c := &Client{BaseURL: "https://lichess.test", HTTP: &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) {
			return respond(status, "failure")
		})}}
		if _, err := c.Collect(context.Background(), time.Now()); err == nil {
			t.Fatalf("accepted HTTP %d", status)
		}
	}
}
