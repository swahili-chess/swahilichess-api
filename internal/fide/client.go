package fide

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

const downloadPage = "https://ratings.fide.com/download_lists.phtml"

var publication = regexp.MustCompile(`Download\s+([A-Za-z]+\s+[0-9]{4})\s+FRL`)

type Client struct{ HTTP *http.Client }

func NewClient() *Client { return &Client{HTTP: &http.Client{Timeout: 4 * time.Minute}} }

func (c *Client) get(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "chess.tz-monthly-ratings/1.0")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FIDE returned HTTP %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || int64(len(data)) > limit {
		return nil, fmt.Errorf("FIDE response empty or exceeds size limit")
	}
	return data, nil
}

func (c *Client) Collect(ctx context.Context, now time.Time) (Snapshot, error) {
	s := Snapshot{Eligibility: "active-standard-rated", Provider: "fide", Federation: "TAN", Sources: map[string]Source{}, Players: []Player{}}
	page, err := c.get(ctx, downloadPage, 2<<20)
	if err != nil {
		return s, err
	}
	match := publication.FindSubmatch(page)
	if len(match) != 2 {
		return s, fmt.Errorf("FIDE publication month missing")
	}
	period, err := time.Parse("January 2006", string(match[1]))
	if err != nil || period.After(now.UTC().AddDate(0, 1, 0)) {
		return s, fmt.Errorf("invalid publication month")
	}
	// A late publication must never be relabelled as the current month.
	if month(period) < month(now) {
		return s, fmt.Errorf("current month's FIDE list is not published yet")
	}
	s.RatingMonth = month(period)
	merged := map[string]Player{}
	eligible := map[string]bool{}
	for _, category := range Categories {
		name := category + "_" + strings.ToLower(period.Format("Jan06")) + "frl_xml"
		url := "https://ratings.fide.com/download/" + name + ".zip"
		data, err := c.get(ctx, url, 64<<20)
		if err != nil {
			return s, fmt.Errorf("%s: %w", category, err)
		}
		archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return s, err
		}
		if len(archive.File) != 1 || archive.File[0].Name != name+".xml" || archive.File[0].UncompressedSize64 > 512<<20 {
			return s, fmt.Errorf("unexpected FIDE archive contents")
		}
		r, err := archive.File[0].Open()
		if err != nil {
			return s, err
		}
		players, stats, parseErr := Parse(io.LimitReader(r, (512<<20)+1), category)
		closeErr := r.Close()
		if parseErr != nil {
			return s, parseErr
		}
		if closeErr != nil {
			return s, closeErr
		}
		stats.URL, stats.SHA256 = url, fmt.Sprintf("%x", sha256.Sum256(data))
		// FIDE's "Exclude Inactive (FIDE rated only)" search uses active
		// Standard-rated players for every category. A separate Rapid/Blitz
		// inactivity flag must not exclude an otherwise eligible player.
		if category == "standard" {
			for _, player := range players {
				if !player.Inactive {
					eligible[player.ID] = true
				}
			}
		}
		stats.Rated = 0
		for _, player := range players {
			if !eligible[player.ID] {
				stats.Excluded++
				continue
			}
			stats.Rated++
			if existing, ok := merged[player.ID]; ok {
				if existing.Name != player.Name {
					return s, fmt.Errorf("conflicting player identity %s", player.ID)
				}
				existing.Ratings[category] = player.Ratings[category]
				merged[player.ID] = existing
			} else {
				merged[player.ID] = player
			}
		}
		s.Sources[category] = stats
	}
	for _, player := range merged {
		s.Players = append(s.Players, player)
	}
	sort.Slice(s.Players, func(i, j int) bool { return s.Players[i].ID < s.Players[j].ID })
	if len(s.Players) == 0 {
		return s, fmt.Errorf("no active rated Tanzania players")
	}
	s.FetchedAt = time.Now().UTC()
	return s, nil
}
