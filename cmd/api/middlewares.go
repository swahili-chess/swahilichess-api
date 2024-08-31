package main

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	db "api.swahilichess.com/internal/db/sqlc"
	"api.swahilichess.com/internal/token"
	"github.com/labstack/echo/v4"
)

func (app *application) authenticate(next echo.HandlerFunc) echo.HandlerFunc {

	return func(c echo.Context) error {

		authorizationHeader := c.Request().Header.Get("Authorization")
		if authorizationHeader == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}

		headerParts := strings.Split(authorizationHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid auth token"})

		}

		tokenString := headerParts[1]
		tokenHash := sha256.Sum256([]byte(tokenString))

		params := db.GetUserByTokenParams{
			Hash:   tokenHash[:],
			Scope:  token.ScopeAuthentication,
			Expiry: time.Now(),
		}

		user, err := app.store.GetUserByToken(c.Request().Context(), params)

		if err != nil {
			switch {
			case errors.Is(err, sql.ErrNoRows):
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid or expired auth token"})
			default:
				slog.Error("Error on getting user associated with token ", "error", err.Error())
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			}
		}

		c.Set("user", user)

		return next(c)

	}

}

// func (app *application) metrics(next http.Handler) http.Handler {
// 	totalRequestsReceived := expvar.NewInt("total_requests_received")
// 	totalResponsesSent := expvar.NewInt("total_responses_sent")
// 	totalProcessingTimeMicroseconds := expvar.NewInt("total_processing_time_μs")

// 	totalResponsesSentByStatus := expvar.NewMap("total_responses_sent_by_status")
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

// 		totalRequestsReceived.Add(1)

// 		metrics := httpsnoop.CaptureMetrics(next, w, r)

// 		totalResponsesSent.Add(1)

// 		totalProcessingTimeMicroseconds.Add(metrics.Duration.Microseconds())

// 		totalResponsesSentByStatus.Add(strconv.Itoa(metrics.Code), 1)
// 	})
// }
