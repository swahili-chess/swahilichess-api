package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

type Performance struct {
	Games  int  `json:"games"`
	Rating int  `json:"rating"`
	Rd     int  `json:"rd"`
	Prog   int  `json:"prog"`
	Prov   bool `json:"prov"`
}

type Member struct {
	Username string                 `json:"username"`
	Perfs    map[string]Performance `json:"perfs"`
	Disabled bool                   `json:"disabled"`
}

type KeyValue struct {
	Key   string
	Value int
}

const user_url = "https://lichess.org/api/users"


func (app *application) leaderboardHandler(c echo.Context) error {

	members_ids, err := app.store.GetLichessTeamMembers(c.Request().Context())
	if err != nil {
		slog.Error("failed to get lichess team member ids", "error", err)
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})

	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("POST", user_url, strings.NewReader(strings.Join(members_ids, ",")))
	if err != nil {
		slog.Error(fmt.Sprintf("failed to create request %s", user_url), "error", err)
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})

	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := client.Do(req)
	if err != nil {
		slog.Error("failed to fetch team members data", "err", err)
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	defer resp.Body.Close()

	res := json.NewDecoder(resp.Body)

	var members []Member

	for {

		var member Member
		err := res.Decode(&member)
		if err != nil {
			if err != io.EOF {
				slog.Error("we got an error while reading body", "err", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			}

			break

		}

		members = append(members, member)

	}

	summary := make(map[string]map[string]int)
	summary["rapid"] = make(map[string]int)
	summary["blitz"] = make(map[string]int)

	for _, user := range members {
		if !user.Disabled {
			summary["rapid"][user.Username] = user.Perfs["rapid"].Rating
			summary["blitz"][user.Username] = user.Perfs["blitz"].Rating
		}

	}

	sort_replace := func(gameTypes ...string) {

		for _, gameType := range gameTypes {
			var kvs []KeyValue
			for k, v := range summary[gameType] {
				kvs = append(kvs, KeyValue{Key: k, Value: v})
			}

			sort.Slice(kvs, func(i, j int) bool {
				return kvs[i].Value > kvs[j].Value
			})

			summary[gameType] = make(map[string]int)
			for _, kv := range kvs {
				summary[gameType][kv.Key] = kv.Value
			}
		}
	}

	sort_replace("rapid", "blitz")

	return c.JSON(http.StatusOK, summary)

}
