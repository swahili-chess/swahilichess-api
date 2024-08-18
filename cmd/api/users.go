package main

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"backend.chesswahili.com/internal/data"
	"backend.chesswahili.com/internal/validator"
)

func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {

	var input struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user := &data.User{
		Username:  input.Username,
		Email:     input.Email,
		Activated: false,
	}

	err = user.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	v := validator.New()

	if data.ValidateUser(v, user); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.Users.Insert(user)
	if err != nil {
		switch {

		case errors.Is(err, data.ErrDuplicateEmail):
			v.AddError("email", "a user with this email address already exists")
			app.failedValidationResponse(w, r, v.Errors)
		case errors.Is(err, data.ErrDuplicateUsername):
			v.AddError("username", "a user with this username already exists")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	token, err := app.models.Tokens.New(user.UUID, 3*24*time.Hour, data.ScopeActivation)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.background(func() {
		username := user.Username
		data := map[string]interface{}{
			"activationToken": token.Plaintext,
			"userID":          user.UUID,
			"username":        username,
		}

		err = app.mailer.Send(user.Email, "user_welcome.tmpl", data)
		if err != nil {
			slog.Error("error sending email", "error", err)
		}
	})

	err = app.writeJSON(w, http.StatusAccepted, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) activateUserHandler(w http.ResponseWriter, r *http.Request) {

	var input struct {
		TokenPlaintext string `json:"token"`
	}
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	if data.ValidateTokenPlaintext(v, input.TokenPlaintext); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	user, err := app.models.Users.GetForToken(data.ScopeActivation, input.TokenPlaintext)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			v.AddError("token", "invalid or expired activation token")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	user.Activated = true

	err = app.models.Users.Update(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.models.Tokens.DeleteAllForUser(data.ScopeActivation, user.UUID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}


func (app *application) updateUserPasswordHandler(w http.ResponseWriter, r *http.Request) {

	var input struct {
		Password       string `json:"password"`
		TokenPlaintext string `json:"token"`
	}
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	v := validator.New()
	data.ValidatePasswordPlaintext(v, input.Password)
	data.ValidateTokenPlaintext(v, input.TokenPlaintext)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	user, err := app.models.Users.GetForToken(data.ScopePasswordReset, input.TokenPlaintext)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			v.AddError("token", "invalid or expired password reset token")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = user.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.models.Users.Update(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.models.Tokens.DeleteAllForUser(data.ScopePasswordReset, user.UUID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	env := envelope{"message": "your password was successfully reset"}
	err = app.writeJSON(w, http.StatusOK, env, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
