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

	e.GET("/ping", app.pingHandler)
	e.POST("/login", app.createAuthTokenHandler)
	e.GET("/lichess/leaderboard", app.leaderboardHandler)

	// user management
	e.POST("/users", app.registerUserHandler)
	e.POST("/users/activate", app.activateUserHandler)
	e.POST("/users/resend/activation", app.resendactivationHandler)
	e.POST("/users/forgot-password", app.forgotPasswordUserHandler)
	e.POST("/users/change-password", app.changePasswordUserHandler)

	//TODO add ability to change phone number

	g := e.Group("/auth")
	g.Use(app.authenticate)

	g.PUT("/users/:id", app.updateUserHandler)

	return e

}
