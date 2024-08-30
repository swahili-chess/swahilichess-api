package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (app *application) pingHandler(c echo.Context) error {

	ping := map[string]string{
		"status":      "available",
		"environment": app.config.ENV,
		"version":     version,
	}

	return c.JSON(http.StatusOK, ping)

}
