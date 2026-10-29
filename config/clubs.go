package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
)

// Recognized clubs whose tournaments appear on the website calendar. Edit
// clubs.json and redeploy to add or remove a club.
//
//go:embed clubs.json
var clubsJSON []byte

type Club struct {
	Platform string `json:"platform"`
	ID       string `json:"id"`
	Name     string `json:"name"`
}

var lichessTeamID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,79}$`)

func Clubs() ([]Club, error) {
	var clubs []Club
	if err := json.Unmarshal(clubsJSON, &clubs); err != nil {
		return nil, fmt.Errorf("clubs.json: %w", err)
	}
	seen := map[string]bool{}
	for _, club := range clubs {
		if club.Platform != "lichess" || !lichessTeamID.MatchString(club.ID) || club.Name == "" || seen[club.ID] {
			return nil, fmt.Errorf("clubs.json: invalid or duplicate club %q", club.ID)
		}
		seen[club.ID] = true
	}
	return clubs, nil
}
