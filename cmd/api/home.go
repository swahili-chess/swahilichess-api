package main

import (
	"fmt"
	"net/http"
)

func (app *application) home(w http.ResponseWriter, r *http.Request) {

	fmt.Fprintln(w, "Home sweet home")

}
