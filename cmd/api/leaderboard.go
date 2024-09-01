package main

import (
	"encoding/json"
	"fmt"
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

type Data struct {
	username string
	rating   int
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

	var members []Member
	err = json.NewDecoder(resp.Body).Decode(&members)
	if err != nil {
		slog.Error("error while reading bod (users)", "err", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	  rapid := []Data{}
	  blitz :=  []Data{}

	for _, user := range members {
		if !user.Disabled {
			rapid = append(rapid, Data{username: user.Username, rating: user.Perfs["rapid"].Rating})
			blitz = append(blitz, Data{username: user.Username, rating: user.Perfs["blitz"].Rating})
		}

	}

     sort.Slice(rapid, func(i, j int) bool {
        return rapid[i].rating > rapid[j].rating
    })

	 sort.Slice(blitz, func(i, j int) bool {
        return blitz[i].rating > blitz[j].rating
    })

	summary := make(map[string][]Data)
	summary["rapid"] = rapid
	summary["blitz"] = blitz

	return c.JSON(http.StatusOK, summary)

}
