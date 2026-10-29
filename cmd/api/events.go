package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// eventsHandler only reads the saved snapshot of club and official online tournaments.
func (app *application) eventsHandler(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*time.Second)
	defer cancel()
	snapshot, err := app.eventsStore.Read(ctx, time.Now().UTC())
	if err != nil {
		slog.Warn("events snapshot unavailable", "error", err)
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "Events are unavailable"})
	}
	c.Response().Header().Set("Cache-Control", "public, max-age=60")
	return c.JSON(http.StatusOK, snapshot)
}
