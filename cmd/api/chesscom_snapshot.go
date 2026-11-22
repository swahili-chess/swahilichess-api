package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// chesscomRatingsHandler only reads the saved snapshot.
func (app *application) chesscomRatingsHandler(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*time.Second)
	defer cancel()
	snapshot, err := app.chesscomStore.Read(ctx, time.Now().UTC())
	if err != nil {
		slog.Warn("Chess.com snapshot unavailable", "error", err)
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "Ratings are unavailable"})
	}
	c.Response().Header().Set("Cache-Control", "public, max-age=60")
	return c.JSON(http.StatusOK, snapshot)
}
