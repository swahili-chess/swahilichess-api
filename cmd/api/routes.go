package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func (app *application) routes() *echo.Echo {

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	DefaultCORSConfig := middleware.CORSConfig{
		Skipper:      middleware.DefaultSkipper,
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodPost, http.MethodDelete},
	}

	e.Use(middleware.CORSWithConfig(DefaultCORSConfig))

	//router.Handler(http.MethodGet, "/v1/metrics", expvar.Handler())

	e.GET("/ping", app.pingHandler)
	e.POST("/login", app.createAuthTokenHandler)
	g := e.Group("/auth")
	g.Use(app.authenticate)
	g.PUT("/users/:id", app.updateUserHandler)
	g.POST("/users/:id", app.registerUserHandler)
	g.POST("/users/activate", app.activateUserHandler)

	return e

}
