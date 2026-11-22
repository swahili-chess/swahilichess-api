package events

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"api.swahilichess.com/config"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func fakeClient(routes map[string]string) *Client {
	return &Client{
		BaseURL: "https://lichess.test",
		Clubs:   []config.Club{{Platform: "lichess", ID: "club-a", Name: "Club A"}, {Platform: "lichess", ID: "club-b", Name: "Club B"}},
		HTTP: &http.Client{Transport: transport(func(req *http.Request) (*http.Response, error) {
			body, ok := routes[req.URL.Path]
			if !ok {
				return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader(""))}, nil
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})},
	}
}

var now = time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)

func TestCollectNormalizesClubAndOfficialTournaments(t *testing.T) {
	shared := `{"id":"Shared01","fullName":"Shared Arena","createdBy":"a","startsAt":1793551200000,"finishesAt":1793556000000,"status":10,"variant":{"key":"standard"},"clock":{"limit":600,"increment":5},"rated":true}`
	c := fakeClient(map[string]string{
		"/api/team/club-a/arena": shared + "\n" +
			`{"id":"OldArena","fullName":"Old Arena","startsAt":1700000000000,"status":30,"variant":{"key":"standard"},"clock":{"limit":180,"increment":0}}` + "\n",
		"/api/team/club-a/swiss": `{"id":"SwissOne","name":"Club Swiss","startsAt":"2026-10-20T17:00:00Z","status":"finished","variant":"standard","clock":{"limit":180,"increment":1},"nbRounds":7,"rated":true}` + "\n",
		"/api/team/club-b/arena": shared + "\n",
		"/api/team/club-b/swiss": "",
		"/api/tournament": `{"created":[
			{"id":"Hourly01","fullName":"Hourly Bullet Arena","createdBy":"lichess","startsAt":1793220000000,"minutes":27,"status":10,"variant":{"key":"standard"},"clock":{"limit":60,"increment":0},"schedule":{"freq":"hourly"},"rated":true},
			{"id":"Hyper001","fullName":"Hourly HyperBullet Arena","createdBy":"lichess","startsAt":1793220000000,"status":10,"variant":{"key":"standard"},"clock":{"limit":30,"increment":0},"schedule":{"freq":"hourly"}},
			{"id":"Classic1","fullName":"Weekly Classical Arena","createdBy":"lichess","startsAt":1793220000000,"status":10,"variant":{"key":"standard"},"clock":{"limit":1200,"increment":10},"schedule":{"freq":"weekly"}},
			{"id":"Rapid151","fullName":"15+10 Rapid Arena","createdBy":"lichess","startsAt":1793220000000,"status":10,"variant":{"key":"standard"},"clock":{"limit":900,"increment":10},"schedule":{"freq":"hourly"}},
			{"id":"TitledW1","fullName":"Titled Arena Warm-up","createdBy":"lichess","startsAt":1793220000000,"status":10,"variant":{"key":"standard"},"clock":{"limit":30,"increment":0},"schedule":{"freq":"unique"}},
			{"id":"Atomic01","fullName":"Hourly Atomic Arena","createdBy":"lichess","startsAt":1793220000000,"status":10,"variant":{"key":"atomic"},"clock":{"limit":180,"increment":0}},
			{"id":"UserMade","fullName":"Someone's Arena","createdBy":"someone","startsAt":1793220000000,"status":10,"variant":{"key":"standard"},"clock":{"limit":180,"increment":0}}],
			"started":[{"id":"Fischer1","fullName":"Daily Chess960 Arena","createdBy":"lichess","startsAt":1793210000000,"status":20,"variant":{"key":"chess960"},"clock":{"limit":180,"increment":2},"schedule":{"freq":"daily"}}],
			"finished":[{"id":"Finish01","fullName":"Done","createdBy":"lichess","startsAt":1793200000000,"status":30,"variant":{"key":"standard"},"clock":{"limit":60,"increment":0}}]}`,
	})
	s, err := c.Collect(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Event{}
	for _, e := range s.Events {
		byID[e.ID] = e
	}
	if len(s.Events) != 6 || len(s.Clubs) != 2 {
		t.Fatalf("events %v", s.Events)
	}
	arena := byID["lichess-arena-Shared01"]
	if strings.Join(arena.Clubs, ",") != "club-a,club-b" || arena.Status != "upcoming" || arena.Speed != "rapid" || arena.URL != "https://lichess.org/tournament/Shared01" || arena.FinishesAt == nil {
		t.Fatalf("shared arena %+v", arena)
	}
	swiss := byID["lichess-swiss-SwissOne"]
	if swiss.Status != "finished" || swiss.Rounds != 7 || swiss.Speed != "blitz" || swiss.URL != "https://lichess.org/swiss/SwissOne" || swiss.Origin != OriginClub {
		t.Fatalf("swiss %+v", swiss)
	}
	if _, ok := byID["lichess-arena-OldArena"]; ok {
		t.Fatal("kept a club tournament older than a year")
	}
	for _, id := range []string{"Hyper001", "Classic1"} {
		if _, ok := byID["lichess-arena-"+id]; ok {
			t.Fatalf("kept %s outside 1+0 to 15+10", id)
		}
	}
	if _, ok := byID["lichess-arena-Rapid151"]; !ok {
		t.Fatal("dropped 15+10")
	}
	if _, ok := byID["lichess-arena-TitledW1"]; !ok {
		t.Fatal("dropped a one-off event")
	}
	hourly, fischer := byID["lichess-arena-Hourly01"], byID["lichess-arena-Fischer1"]
	if hourly.Origin != OriginOfficial || hourly.Schedule != "hourly" || hourly.Speed != "bullet" || hourly.FinishesAt == nil || fischer.Status != "ongoing" || fischer.Variant != "chess960" {
		t.Fatalf("official %+v %+v", hourly, fischer)
	}
	for i := 1; i < len(s.Events); i++ {
		if s.Events[i].StartsAt.Before(s.Events[i-1].StartsAt) {
			t.Fatal("events not sorted by start")
		}
	}
}

func TestCollectFailsWholeRunOnBadData(t *testing.T) {
	ok := map[string]string{"/api/team/club-a/arena": "", "/api/team/club-a/swiss": "", "/api/team/club-b/arena": "", "/api/team/club-b/swiss": "", "/api/tournament": `{"created":[]}`}
	cases := map[string][2]string{
		"bad ID":         {"/api/team/club-a/arena", `{"id":"../x","fullName":"X","startsAt":1,"status":10,"variant":{"key":"standard"},"clock":{"limit":60,"increment":0}}`},
		"unknown status": {"/api/team/club-a/arena", `{"id":"Abcdefgh","fullName":"X","startsAt":1,"status":99,"variant":{"key":"standard"},"clock":{"limit":60,"increment":0}}`},
		"truncated":      {"/api/team/club-b/swiss", `{"id":"Abcdefgh"`},
		"bad official":   {"/api/tournament", `{"created":[{"id":"Abcdefgh","createdBy":"lichess","startsAt":"never"}]}`},
	}
	for name, change := range cases {
		routes := map[string]string{}
		for k, v := range ok {
			routes[k] = v
		}
		routes[change[0]] = change[1]
		if _, err := fakeClient(routes).Collect(context.Background(), now); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	missing := map[string]string{}
	for k, v := range ok {
		missing[k] = v
	}
	delete(missing, "/api/team/club-b/arena")
	if _, err := fakeClient(missing).Collect(context.Background(), now); err == nil {
		t.Error("accepted a failed club request")
	}
}

func TestConfiguredClubsAreValid(t *testing.T) {
	clubs, err := config.Clubs()
	if err != nil || len(clubs) == 0 {
		t.Fatalf("%v %v", clubs, err)
	}
}
