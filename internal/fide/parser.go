package fide

import (
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

var numericID = regexp.MustCompile(`^[1-9][0-9]{0,11}$`)

type entry struct {
	ID         string  `xml:"fideid"`
	Name       string  `xml:"name"`
	Country    string  `xml:"country"`
	Title      string  `xml:"title"`
	WomanTitle string  `xml:"w_title"`
	Rating     *string `xml:"rating"`
	Flag       *string `xml:"flag"`
}

// Parse reads the complete archive, including the closing root and ZIP checksum.
// Retain each archive flag. Collect applies FIDE search eligibility from the Standard list.
func Parse(r io.Reader, category string) ([]Player, Source, error) {
	d := xml.NewDecoder(r)
	players := []Player{}
	stats := Source{}
	seen := map[string]bool{}
	root, closed := false, false
	for {
		token, err := d.Token()
		if err == io.EOF {
			if !closed || stats.Records == 0 || stats.Tanzania == 0 {
				return nil, stats, fmt.Errorf("incomplete or empty FIDE list")
			}
			return players, stats, nil
		}
		if err != nil {
			return nil, stats, fmt.Errorf("decode FIDE XML: %w", err)
		}
		switch t := token.(type) {
		case xml.StartElement:
			if !root && t.Name.Local == "playerslist" {
				root = true
				continue
			}
			if !root || closed || t.Name.Local != "player" {
				return nil, stats, fmt.Errorf("unexpected XML element %s", t.Name.Local)
			}
			var p entry
			if err := d.DecodeElement(&p, &t); err != nil {
				return nil, stats, err
			}
			stats.Records++
			if strings.TrimSpace(p.Country) != "TAN" {
				continue
			}
			stats.Tanzania++
			p.ID, p.Name = strings.TrimSpace(p.ID), strings.TrimSpace(p.Name)
			if !numericID.MatchString(p.ID) || p.Name == "" || p.Rating == nil || p.Flag == nil || seen[p.ID] {
				return nil, stats, fmt.Errorf("invalid or duplicate Tanzania record %q", p.ID)
			}
			seen[p.ID] = true
			flag := strings.ToLower(strings.TrimSpace(*p.Flag))
			if flag != "" && flag != "w" && flag != "i" && flag != "wi" && flag != "iw" {
				return nil, stats, fmt.Errorf("unknown activity flag %q", flag)
			}
			raw := strings.TrimSpace(*p.Rating)
			value := 0
			if raw != "" {
				value, err = strconv.Atoi(raw)
				if err != nil || value < 0 || value > 3500 {
					return nil, stats, fmt.Errorf("invalid rating for %s", p.ID)
				}
			}
			if strings.Contains(flag, "i") {
				stats.Inactive++
			}
			if value == 0 {
				continue
			}
			title := strings.ToUpper(strings.TrimSpace(p.Title))
			if title == "" {
				title = strings.ToUpper(strings.TrimSpace(p.WomanTitle))
			}
			players = append(players, Player{Inactive: strings.Contains(flag, "i"), ID: p.ID, Name: p.Name, Title: title, Ratings: map[string]*Rating{category: {Value: value}}})
			stats.Rated++
		case xml.EndElement:
			if t.Name.Local != "playerslist" || closed {
				return nil, stats, fmt.Errorf("unexpected closing element")
			}
			closed = true
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" {
				return nil, stats, fmt.Errorf("unexpected text outside player")
			}
		}
	}
}
