package main

import (
	"errors"
	"net/http"

	"backend.chesswahili.com/internal/data"
	"backend.chesswahili.com/internal/validator"
)

func (app *application) showProfile(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)
	user_id := user.UUID

	user_account, err := app.models.Accounts.Get(user_id)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"account": user_account}, nil)

	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

func (app *application) createProfile(w http.ResponseWriter, r *http.Request) {

	user := app.contextGetUser(r)
	user_id := user.UUID

	var input struct {
		Firstname        string `json:"firstname"`
		Lastname         string `json:"lastname"`
		LichessUsername  string `json:"lichess_username"`
		ChesscomUsername string `json:"chesscom_username"`
		PhoneNumber      string `json:"phone_number"`
		Photo            byte   `json:"photo"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	account := &data.Account{

		Firstname:        input.Firstname,
		Lastname:         input.Lastname,
		LichessUsername:  input.LichessUsername,
		ChesscomUsername: input.ChesscomUsername,
		PhoneNumber:      input.PhoneNumber,
		Photo:            input.Photo,
	}

	v := validator.New()

	if data.ValidateAccount(v, account); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	account.UserID = user_id
	err = app.models.Accounts.Insert(account)
	if err != nil {
		switch {

		case errors.Is(err, data.ErrDuplicateLichessUsername):
			v.AddError("lichess_username", "a user with this lichess username  already exists")
			app.failedValidationResponse(w, r, v.Errors)
		case errors.Is(err, data.ErrDuplicateChesscomUsername):
			v.AddError("chesscome_username", "a user with this chess.com username already exists")
			app.failedValidationResponse(w, r, v.Errors)

		case errors.Is(err, data.ErrDuplicatePhonenumber):
			v.AddError("phone_number", "a user with this phone number already exists")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusAccepted, envelope{"user_account": account}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) updateProfile(w http.ResponseWriter, r *http.Request) {

	user := app.contextGetUser(r)
	user_id := user.UUID

	var input struct {
		Firstname        *string `json:"firstname"`
		Lastname         *string `json:"lastname"`
		LichessUsername  *string `json:"lichess_username"`
		ChesscomUsername *string `json:"chesscom_username"`
		PhoneNumber      *string `json:"phone_number"`
		Photo            *byte   `json:"photo"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	account, err := app.models.Accounts.Get(user_id)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// account := &data.Account{

	// 	Firstname:        input.Firstname,
	// 	Lastname:         input.Lastname,
	// 	LichessUsername:  input.LichessUsername,
	// 	ChesscomUsername: input.ChesscomUsername,
	// 	PhoneNumber:      input.PhoneNumber,
	// 	Photo:            input.Photo,
	// }

	if input.Firstname != nil {
		account.Firstname = *input.Firstname
	}

	if input.Lastname != nil {
		account.Lastname = *input.Lastname

	}

	if input.LichessUsername != nil {
		account.LichessUsername = *input.LichessUsername
	}

	if input.ChesscomUsername != nil {
		account.ChesscomUsername = *input.ChesscomUsername

	}

	if input.PhoneNumber != nil {
		account.PhoneNumber = *input.PhoneNumber
	}

	if input.Photo != nil {
		account.Photo = *input.Photo
	}

	v := validator.New()

	if data.ValidateAccount(v, account); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	
	err = app.models.Accounts.Update(account)
	if err != nil {
		switch {

		case errors.Is(err, data.ErrDuplicateLichessUsername):
			v.AddError("lichess_username", "a user with this lichess username  already exists")
			app.failedValidationResponse(w, r, v.Errors)
		case errors.Is(err, data.ErrDuplicateChesscomUsername):
			v.AddError("chesscome_username", "a user with this chess.com username already exists")
			app.failedValidationResponse(w, r, v.Errors)

		case errors.Is(err, data.ErrDuplicatePhonenumber):
			v.AddError("phone_number", "a user with this phone number already exists")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusAccepted, envelope{"user_account": account}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
