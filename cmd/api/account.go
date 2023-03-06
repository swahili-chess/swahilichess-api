package main

import (
	"net/http"
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
