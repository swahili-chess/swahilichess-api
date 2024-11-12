package main

import (
	"crypto/subtle"
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

	// for chessbot
	b := e.Group("/bot")
	b.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
		if subtle.ConstantTimeCompare([]byte(username), []byte(app.config.BasicAuth.USERNAME)) == 1 &&
			subtle.ConstantTimeCompare([]byte(password), []byte(app.config.BasicAuth.PASSWORD)) == 1 {
			return true, nil
		}
		return false, nil
	}))

	b.GET("/lichess/members", app.getLichessTeamMemberHandler)
	b.POST("/lichess/members", app.insertLichessTeamMemberHandler)
	b.POST("/telegram/bot/users", app.insertTgUserHandler)
	b.PUT("/telegram/bot/users", app.updateTgUserHandler)
	b.GET("/telegram/bot/users/active", app.getActiveTgUserHandler)

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
