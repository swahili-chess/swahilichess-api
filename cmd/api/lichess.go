package main

import (
	"log/slog"
	"net/http"
	"github.com/labstack/echo/v4"
	db "api.swahilichess.com/internal/db/sqlc"
)

func (app *application) getLichessTeamMemberHandler(c echo.Context) error {

	members, err := app.store.GetLichessTeamMembers(c.Request().Context())

	if err == nil {
		slog.Error("failed to get lichess member on db", "error", err)
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusOK, members)

}

func (app *application) insertLichessTeamMemberHandler(c echo.Context) error {

	var input struct {
		LichessID string `json:"lichess_id"`
        Username  string `json:"username"`
	}

	if err := c.Bind(&input); err != nil {
		c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	args := db.InsertLichessTeamMemberParams {
          LichessID: input.LichessID,
		  Username: input.Username,
	} 

	err := app.store.InsertLichessTeamMember(c.Request().Context(), args)

	if err != nil {
		slog.Error("failed to insert lichess member on db", "error", err)
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusOK, nil)

}
