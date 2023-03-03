package main

import (
	"fmt"
	"net/http"
)

func (app *application) showProfile(w http.ResponseWriter, r *http.Request) {

	fmt.Fprintln(w, "This is my profile ")

}
