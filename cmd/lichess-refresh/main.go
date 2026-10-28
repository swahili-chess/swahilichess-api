// lichess-refresh is a separate worker. Run once from a scheduler or use -watch.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api.swahilichess.com/config"
	"api.swahilichess.com/internal/lichess"
)

func main() {
	var cfg config.Config
	flag.StringVar(&cfg.DB.DSN, "db-dsn", os.Getenv("SW_DB_DSN"), "PostgreSQL DSN")
	watch := flag.Bool("watch", false, "check the persisted schedule hourly until stopped")
	force := flag.Bool("force", false, "refresh once even if the next refresh is not due")
	flag.Parse()
	if *watch && *force {
		slog.Error("use -force only for a single refresh")
		os.Exit(1)
	}
	// Read the token from the environment only, so it never appears in process listings.
	token := os.Getenv("LICHESS_TOKEN")
	if token == "" {
		slog.Warn("LICHESS_TOKEN is not set; a private team roster cannot be read")
	}
	cfg.DB.MaxOpenConns, cfg.DB.MaxIdleConns, cfg.DB.MaxIdleTime = 2, 2, "5m"
	conn, err := config.OpenDB(cfg)
	if err != nil {
		slog.Error("database unavailable", "error", err)
		os.Exit(1)
	}
	defer conn.Close()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	store, client := &lichess.Store{DB: conn}, lichess.NewClient(token)
	for {
		attempt, cancel := context.WithTimeout(ctx, 5*time.Minute)
		updated, err := store.Refresh(attempt, time.Now().UTC(), *force, client.Collect)
		cancel()
		if err != nil {
			slog.Error("Lichess refresh failed; previous snapshot retained", "error", err)
		} else {
			slog.Info("Lichess schedule checked", "updated", updated)
		}
		if !*watch {
			if err != nil {
				os.Exit(1)
			}
			return
		}
		timer := time.NewTimer(time.Hour)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
