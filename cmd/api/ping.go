package main

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

func (app *application) pingHandler(c echo.Context) error {

	ping := map[string]string{
		"status":      "available",
		"environment": app.config.ENV,
		"version":     version,
		"current_time":        time.Now().Format(time.RFC3339),
	}

	return c.JSON(http.StatusOK, ping)

}
