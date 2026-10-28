package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// lichessRatingsHandler only reads the saved snapshot. The original
// /lichess/leaderboard route stays unchanged for existing consumers.
func (app *application) lichessRatingsHandler(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*time.Second)
	defer cancel()
	snapshot, err := app.lichessStore.Read(ctx, time.Now().UTC())
	if err != nil {
		slog.Warn("Lichess snapshot unavailable", "error", err)
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "Ratings are unavailable"})
	}
	c.Response().Header().Set("Cache-Control", "public, max-age=60")
	return c.JSON(http.StatusOK, snapshot)
}
