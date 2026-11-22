package fide

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func record(id, country, rating, flag string) string {
	return fmt.Sprintf(`<player><fideid>%s</fideid><name>Player %s</name><country>%s</country><rating>%s</rating><flag>%s</flag><games>0</games></player>`, id, id, country, rating, flag)
}

func TestParseActivityAndMissingRatings(t *testing.T) {
	x := `<playerslist>` + record("1", "TAN", "2200", "") + record("2", "TAN", "2100", "wi") + record("3", "TAN", "0", "") + record("4", "TAN", "1800", "w") + record("5", "KEN", "2400", "") + `</playerslist>`
	players, stats, err := Parse(strings.NewReader(x), "rapid")
	if err != nil {
		t.Fatal(err)
	}
	if len(players) != 3 || players[0].Ratings["rapid"].Value != 2200 || stats.Records != 5 || stats.Tanzania != 4 || stats.Inactive != 1 || stats.Rated != 3 {
		t.Fatalf("unexpected result: %+v %+v", players, stats)
	}
	if !players[1].Inactive || players[0].Inactive {
		t.Fatal("archive flags must be retained for collector eligibility")
	}
	if players[0].Ratings["standard"] != nil {
		t.Fatal("invented standard rating")
	}
}

func TestParseRejectsInvalidSnapshots(t *testing.T) {
	valid := record("1", "TAN", "2200", "")
	cases := []string{
		"", "<html>error</html>", "<playerslist></playerslist>", "<playerslist>" + valid,
		"<playerslist>" + valid + valid + "</playerslist>",
		"<playerslist>" + strings.Replace(valid, "<flag></flag>", "", 1) + "</playerslist>",
		"<playerslist>" + record("1", "TAN", "oops", "") + "</playerslist>",
		"<playerslist>" + record("1", "TAN", "3501", "") + "</playerslist>",
		"<playerslist>" + record("1", "TAN", "2200", "unknown") + "</playerslist>",
		"<playerslist>" + record("x", "TAN", "2200", "") + "</playerslist>",
		"<playerslist>" + valid + "</playerslist><playerslist/>",
	}
	for i, x := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if _, _, err := Parse(strings.NewReader(x), "standard"); err == nil {
				t.Fatal("accepted malformed data")
			}
		})
	}
}

func TestMonthlySchedule(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	if got := nextPublication("2026-10", now); !got.Equal(time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC)) {
		t.Fatal(got)
	}
	if got := nextPublication("2026-09", now); !got.Equal(now.Add(24 * time.Hour)) {
		t.Fatal(got)
	}
	if got := nextPublication("2026-12", now); got.Year() != 2027 || got.Month() != 1 {
		t.Fatal(got)
	}
}

func TestParseBirthYearAndSex(t *testing.T) {
	person := func(id, birthday, sex string) string {
		return strings.Replace(record(id, "TAN", "2000", ""), "<games>", "<birthday>"+birthday+"</birthday><sex>"+sex+"</sex><games>", 1)
	}
	x := `<playerslist>` + person("1", "1990", "M") + person("2", " 2004 ", "f") + person("3", "0000", "X") + person("4", "", "") + record("5", "TAN", "1900", "") + `</playerslist>`
	players, _, err := Parse(strings.NewReader(x), "standard")
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		year int
		sex  string
	}{{1990, "M"}, {2004, "F"}, {0, ""}, {0, ""}, {0, ""}}
	for i, w := range want {
		if players[i].BirthYear != w.year || players[i].Sex != w.sex {
			t.Fatalf("player %d: got %d %q, want %d %q", i+1, players[i].BirthYear, players[i].Sex, w.year, w.sex)
		}
	}
}

func TestPublicOmitsBirthYearAndSex(t *testing.T) {
	r := Response{Snapshot: Snapshot{Players: []Player{{ID: "1", Name: "A", BirthYear: 1990, Sex: "M", Ratings: map[string]*Rating{}}}}}
	public := r.Public()
	if public.Players[0].BirthYear != 0 || public.Players[0].Sex != "" {
		t.Fatal("public response exposed birth year or sex")
	}
	if r.Players[0].BirthYear != 1990 || r.Players[0].Sex != "M" {
		t.Fatal("Public changed the saved snapshot")
	}
}
