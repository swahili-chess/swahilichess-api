package main

import (
	"log/slog"
	"net/http"
	db "api.swahilichess.com/internal/db/sqlc"
	"github.com/labstack/echo/v4"
)

func (app *application) getActiveTgUserHandler(c echo.Context) error {

	tgActiveUsers, err := app.store.GetActiveTgBotUsers(c.Request().Context())
	if err != nil {
		slog.Error("failed to get active tg users on db", "error", err)
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusOK, tgActiveUsers)

}

func (app *application) insertTgUserHandler(c echo.Context) error {

	var input struct {
		ID       int64 `json:"id"`
        Isactive bool  `json:"isactive"`
	}

	if err := c.Bind(&input); err != nil {
		c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	args := db.InsertTgBotUsersParams{
		ID: input.ID,
		Isactive: input.Isactive,
	}
    
	err := app.store.InsertTgBotUsers(c.Request().Context(), args)

	if err != nil {
		slog.Error("failed to insert tg users on db", "error", err)
		c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()}) // real error is needed
	}

	return c.JSON(http.StatusOK, nil)


}

func (app *application) updateTgUserHandler(c echo.Context) error {

		var input struct {
			ID       int64 `json:"id"`
	        Isactive bool  `json:"isactive"`
        }

	if err := c.Bind(&input); err != nil {
		c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	args := db.UpdateTgBotUsersParams{
		ID: input.ID,
		Isactive: input.Isactive,
	}
    
	err := app.store.UpdateTgBotUsers(c.Request().Context(), args)

	if err != nil {
		slog.Error("failed to update tg users on db", "error", err)
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusOK, nil)

}
