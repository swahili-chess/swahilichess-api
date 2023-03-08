package main

import (
	"expvar"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func (app *application) routes() http.Handler {
	router := httprouter.New()

	router.NotFound = http.HandlerFunc(app.notFoundResponse)
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)
	router.HandlerFunc(http.MethodGet, "/", app.home)
	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.sitecheckHandler)



	// User Routes

	router.HandlerFunc(http.MethodPost, "/v1/users", app.registerUserHandler)
	router.HandlerFunc(http.MethodPut, "/v1/users/activated", app.activateUserHandler)
	router.HandlerFunc(http.MethodPost, "/v1/tokens/authentication", app.createAuthenticationTokenHandler)

    // User profile
	router.HandlerFunc(http.MethodGet, "/v1/user/profile", app.requireActivatedUser(app.showProfile))
	router.HandlerFunc(http.MethodPost, "/v1/user/create_profile", app.requireActivatedUser(app.createProfile))
	router.HandlerFunc(http.MethodPut, "/v1/user/update_profile", app.requireActivatedUser(app.updateProfile))

	// Resend user token route
	router.HandlerFunc(http.MethodPost, "/v1/tokens/activation", app.createActivationTokenHandler)


	// Metrics Routes
	router.Handler(http.MethodGet, "/v1/metrics", expvar.Handler())


	//password management

	router.HandlerFunc(http.MethodPost, "/v1/tokens/passwordreset",app.createPasswordResetTokenHandler)
	router.HandlerFunc(http.MethodPut, "/v1/users/password", app.updateUserPasswordHandler)

	return app.metrics(app.recoverPanic(app.enableCORS(app.rateLimit(app.authenticate(router)))))

}
