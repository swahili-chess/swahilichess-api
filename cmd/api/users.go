package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	db "backend.chesswahili.com/internal/db/sqlc"
	"backend.chesswahili.com/internal/token"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

const image_upload_path = "/var/www/lugano/images"
const base_image_url = "https://images.swahilichess.com"
const default_image = "https://images.swahilichess.com/pawn.png"

type input struct {
	Username         string `json:"username" validate:"required,min=3"`
	Password         string `json:"password" validate:"required,min=6"`
	Fullname         string `json:"fullname" validate:"required,min=3"`
	LichessUsername  string `json:"lichess_username"`
	ChesscomUsername string `json:"chesscom_username"`
	PhoneNumber      string `json:"phone_number"`
	Photo            string `json:"photo"`
}

func (app *application) registerUserHandler(c echo.Context) error {

	inp := new(input)

	inp.Username = c.FormValue("username")
	inp.Password = c.FormValue("password")
	inp.Fullname = c.FormValue("fullname")
	inp.LichessUsername = c.FormValue("lichess_username")
	inp.ChesscomUsername = c.FormValue("chesscom_username")
	inp.PhoneNumber = c.FormValue("phone_number")

	if err := c.Bind(&inp); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := app.validator.Struct(inp); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	is_file_uploaded := true
	image_url := default_image

	file, err := c.FormFile("photo")
	if err != nil {
		if err == http.ErrMissingFile {
			is_file_uploaded = false
		}

		if strings.Contains(strings.ToLower(err.Error()), "too large") {
			return c.JSON(http.StatusRequestEntityTooLarge, "File too large")
		}

		slog.Error("failed processing file upload", "error", err.Error())
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	if is_file_uploaded {
		src, err := file.Open()
		if err != nil {
			return err
		}
		defer src.Close()

		uniqueID := uuid.New()
		imageExt := filepath.Ext(file.Filename)
		image := fmt.Sprintf("%s%s", strings.Replace(uniqueID.String(), "-", "", -1), imageExt)
		image_url = fmt.Sprintf("%s/%s", base_image_url, image)

		dst, err := os.Create(fmt.Sprintf("%s/%s", image_upload_path, image))
		if err != nil {
			slog.Error("failed to create path for upload", "error", err.Error())
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		defer dst.Close()

		if _, err = io.Copy(dst, src); err != nil {
			slog.Error("failed to copy to path for upload", "error", err.Error())
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
	}

	password_hash, err := bcrypt.GenerateFromPassword([]byte(inp.Password), 6)
	if err != nil {
		slog.Error("Error hashing password ", "Error", err.Error())
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	passcode := rand.Intn(90000) + 10000

	args := db.CreateUserParams{
		Username:         inp.Username,
		FullName:         inp.Fullname,
		LichessUsername:  inp.LichessUsername,
		ChesscomUsername: inp.ChesscomUsername,
		PhoneNumber:      inp.PhoneNumber,
		Photo:            image_url,
		Passcode:         int32(passcode),
		PasswordHash:     password_hash,
		Activated:        false,
		Enabled:          false,
	}

	user, err := app.store.CreateUser(c.Request().Context(), args)
	if err != nil {
		slog.Error("failed to create user ", "error", err.Error())
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	msg := fmt.Sprintf("Code: %d \nUse it to activate your swahilichess account.", passcode)
	app.background(func() {
		err = app.nextsms.SendSmS(msg, user.PhoneNumber)
		if err != nil {
			slog.Error("error sending email", "error", err)
		}
	})

	return c.JSON(http.StatusCreated, map[string]string{"success": "user created successful"})

}

func (app *application) activateUserHandler(c echo.Context) error {

	var input struct {
		Passcode int32 `json:"passcode"`
	}

	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := app.validator.Struct(input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	user, err := app.store.GetUserByPasscode(context.Background(), input.Passcode)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "passcode doesn't exist or user arleady activated"})
		default:
			slog.Error("failed to get user by passcode ", "error", err.Error())
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
	}

	args := db.UpdateUserByIdParams{
		Username:         user.Username,
		FullName:         user.FullName,
		LichessUsername:  user.LichessUsername,
		ChesscomUsername: user.ChesscomUsername,
		PhoneNumber:      user.PhoneNumber,
		Photo:            user.Photo,
		Passcode:         0,
		PasswordHash:     user.PasswordHash,
		Activated:        true,
		Enabled:          true,
		ID:               user.ID,
	}

	err = app.store.UpdateUserById(context.Background(), args)
	if err != nil {
		slog.Error("failed to update user on activate", "error", err.Error())
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
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

func (app *application) updateUserHandler(c echo.Context) error {

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid uuid"})
	}
	password := c.FormValue("password")
	fullname := c.FormValue("fullname")
	lichessUsername := c.FormValue("lichess_username")
	chesscomUsername := c.FormValue("chesscom_username")

	user, err := app.store.GetUserById(context.Background(), id)
	if err != nil {
		slog.Error("failed to get user by id", "error", err.Error())
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	if fullname != "" {
		user.FullName = fullname
	}
	if lichessUsername != "" {
		user.LichessUsername = lichessUsername
	}
	if chesscomUsername != "" {
		user.ChesscomUsername = chesscomUsername
	}
	is_file_uploaded := true

	file, err := c.FormFile("photo")
	if err != nil {
		if err == http.ErrMissingFile {
			is_file_uploaded = false
		}

		if strings.Contains(strings.ToLower(err.Error()), "too large") {
			return c.JSON(http.StatusRequestEntityTooLarge, "File too large")
		}

		slog.Error("failed processing file upload", "error", err.Error())
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	image_url := ""

	if is_file_uploaded {
		src, err := file.Open()
		if err != nil {
			return err
		}
		defer src.Close()

		uniqueID := uuid.New()
		imageExt := filepath.Ext(file.Filename)
		image := fmt.Sprintf("%s%s", strings.Replace(uniqueID.String(), "-", "", -1), imageExt)
		image_url = fmt.Sprintf("%s/%s", base_image_url, image)

		dst, err := os.Create(fmt.Sprintf("%s/%s", image_upload_path, image))
		if err != nil {
			slog.Error("failed to create path for upload", "error", err.Error())
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		defer dst.Close()

		if _, err = io.Copy(dst, src); err != nil {
			slog.Error("failed to copy to path for upload", "error", err.Error())
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}

		user.Photo = image_url
	}

	if password != "" {
		if len(password) < 6 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "password short (less than 6)"})
		}
		password_hash, err := bcrypt.GenerateFromPassword([]byte(password), 6)
		if err != nil {
			slog.Error("Error hashing password ", "Error", err.Error())
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		user.PasswordHash = password_hash

	}

	args := db.UpdateUserByIdParams{
		Username:         user.Username,
		FullName:         user.FullName,
		LichessUsername:  user.LichessUsername,
		ChesscomUsername: user.ChesscomUsername,
		PhoneNumber:      user.PhoneNumber,
		Photo:            user.Photo,
		Passcode:         user.Passcode,
		PasswordHash:     user.PasswordHash,
		Activated:        user.Activated,
		Enabled:          user.Enabled,
		ID:               user.ID,
	}

	err = app.store.UpdateUserById(context.Background(), args)
	if err != nil {
		slog.Error("failed to update user details", "error", err.Error())
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusOK, map[string]string{"success": "user updated successfuly"})

}
