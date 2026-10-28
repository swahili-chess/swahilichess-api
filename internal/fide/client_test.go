package fide

import (
	"archive/zip"
	"bytes"
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

func TestCollectMatchesFIDEActiveStandardEligibility(t *testing.T) {
	client := &Client{HTTP: &http.Client{Transport: transport(func(req *http.Request) (*http.Response, error) {
		body := []byte("<h1>Download October 2026 FRL</h1>")
		if strings.HasSuffix(req.URL.Path, ".zip") {
			name := strings.TrimPrefix(req.URL.Path, "/download/")
			// Player 1 is active Standard but inactive Rapid: include both ratings.
			// Player 2 is inactive Standard but active Rapid: exclude everywhere.
			// Player 3 is Standard-unrated but active Rapid: exclude everywhere.
			// Player 4 has only an active Standard rating.
			rows := record("1", "TAN", "2200", "") + record("2", "TAN", "1800", "i") + record("4", "TAN", "1600", "")
			if !strings.HasPrefix(name, "standard") {
				rows = record("1", "TAN", "2100", "i") + record("2", "TAN", "1700", "") + record("3", "TAN", "1900", "")
			}
			var b bytes.Buffer
			z := zip.NewWriter(&b)
			f, _ := z.Create(strings.TrimSuffix(name, ".zip") + ".xml")
			fmt.Fprint(f, "<playerslist>"+rows+"</playerslist>")
			z.Close()
			body = b.Bytes()
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body))}, nil
	})}}
	s, err := client.Collect(context.Background(), time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if s.RatingMonth != "2026-10" || len(s.Players) != 2 || s.Players[0].Ratings["rapid"] == nil || s.Players[0].Ratings["blitz"] == nil {
		t.Fatalf("%+v", s)
	}
	if s.Players[0].ID != "1" || s.Players[1].ID != "4" || s.Players[1].Ratings["rapid"] != nil || s.Sources["rapid"].Rated != 1 || s.Sources["rapid"].Excluded != 2 {
		t.Fatalf("wrong eligibility: %+v", s)
	}
	if _, err = client.Collect(context.Background(), time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("relabelled old list as new month")
	}
}

func TestCollectRejectsUpstreamError(t *testing.T) {
	for _, status := range []int{429, 500} {
		c := &Client{HTTP: &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("failure"))}, nil
		})}}
		if _, err := c.Collect(context.Background(), time.Now()); err == nil {
			t.Fatal("accepted upstream error")
		}
	}
}
