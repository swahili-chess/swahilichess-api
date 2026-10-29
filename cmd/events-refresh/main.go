// events-refresh is a separate worker. Run once from a scheduler or use -watch.
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
	"api.swahilichess.com/internal/events"
)

func main() {
	var cfg config.Config
	flag.StringVar(&cfg.DB.DSN, "db-dsn", os.Getenv("SW_DB_DSN"), "PostgreSQL DSN")
	watch := flag.Bool("watch", false, "check the persisted schedule every few minutes until stopped")
	force := flag.Bool("force", false, "refresh once even if the next refresh is not due")
	flag.Parse()
	if *watch && *force {
		slog.Error("use -force only for a single refresh")
		os.Exit(1)
	}
	clubs, err := config.Clubs()
	if err != nil {
		slog.Error("invalid club list", "error", err)
		os.Exit(1)
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
	store, client := events.NewStore(conn), events.NewClient(clubs)
	for {
		attempt, cancel := context.WithTimeout(ctx, 5*time.Minute)
		updated, err := store.Refresh(attempt, time.Now().UTC(), *force, client.Collect)
		cancel()
		if err != nil {
			slog.Error("events refresh failed; previous snapshot retained", "error", err)
		} else {
			slog.Info("events schedule checked", "updated", updated)
		}
		if !*watch {
			if err != nil {
				os.Exit(1)
			}
			return
		}
		// Clubs often create tournaments shortly before play; the store refreshes every 15 minutes.
		timer := time.NewTimer(5 * time.Minute)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
