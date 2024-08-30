package main

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	db "backend.chesswahili.com/internal/db/sqlc"
	"backend.chesswahili.com/internal/token"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

func (app *application) createAuthTokenHandler(c echo.Context) error {

	var input struct {
		PhoneNumber string `json:"phone_number" `
		Username    string `json:"username" `
		Password    string `json:"password"`
	}

	if err := c.Bind(&input); err != nil {
		c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := app.validator.Struct(input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	args := db.GetUserByUsernameOrPhoneParams{
		PhoneNumber: input.PhoneNumber,
		Username:    input.Username,
	}

	user, err := app.store.GetUserByUsernameOrPhone(c.Request().Context(), args)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid phonenumber or username"})

		default:
			slog.Error("failed to get username or phone number", "error", err.Error())
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
	}

	err = bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(input.Password))

	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid password"})

		default:
			slog.Error("failed comparing hash ", "Error", err.Error())
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
	}

	token, expiry, err := token.New(user.ID, app.store, token.ScopeAuthentication)
	if err != nil {
		slog.Error("failed to create token", "error", err.Error())
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	res := struct {
		Token  string `json:"token"`
		Expiry int64  `json:"expiry"`
	}{
		Token:  token,
		Expiry: expiry.Unix(),
	}

	return c.JSON(200, res)

}
