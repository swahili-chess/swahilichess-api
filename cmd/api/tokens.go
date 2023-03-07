package main

import (
	"errors"
	"net/http"
	"time"

	"backend.chesswahili.com/internal/data"
	"backend.chesswahili.com/internal/validator"
)

func (app *application) createAuthenticationTokenHandler(w http.ResponseWriter, r *http.Request) {

	var input struct {
		UsernameOrEmail string `json:"username_email"`
		Password        string `json:"password"`
	}
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()

	res := data.ValidateEmailOrUsername(v, input.UsernameOrEmail)

	data.ValidatePasswordPlaintext(v, input.Password)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	var user *data.User
	var errGet error

	if email, ok := res["email"]; ok {

		user, errGet = app.models.Users.GetByEmail(email)
	} else {
		user, errGet  = app.models.Users.GetByUsername(res["username"])
	}

	if errGet != nil {
		switch {
		case errors.Is(errGet, data.ErrRecordNotFound):
			app.invalidCredentialsResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	match, err := user.Password.Matches(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if !match {
		app.invalidCredentialsResponse(w, r)
		return
	}

	token, err := app.models.Tokens.New(user.UUID, 24*time.Hour, data.ScopeAuthentication)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusCreated, envelope{"authentication_token": token}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
