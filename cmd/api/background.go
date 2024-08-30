package main

import (
	"log/slog"
)

func (app *application) background(fn func()) {

	app.wg.Add(1)

	go func() {
		defer app.wg.Done()
		defer func() {
			if err := recover(); err != nil {
				slog.Error("error from background task", "error", err)
			}
		}()
		fn()
	}()
}
