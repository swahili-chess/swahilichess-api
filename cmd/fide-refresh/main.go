// fide-refresh is a separate worker. Run once from a scheduler or use -watch.
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
	"api.swahilichess.com/internal/fide"
)

func main() {
	var cfg config.Config
	flag.StringVar(&cfg.DB.DSN, "db-dsn", os.Getenv("SW_DB_DSN"), "PostgreSQL DSN")
	watch := flag.Bool("watch", false, "check the persisted schedule hourly until stopped")
	force := flag.Bool("force", false, "refresh once even if this month is already stored")
	flag.Parse()
	if *watch && *force {
		slog.Error("use -force only for a single refresh")
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
	store, client := &fide.Store{DB: conn}, fide.NewClient()
	for {
		attempt, cancel := context.WithTimeout(ctx, 15*time.Minute)
		updated, err := store.Refresh(attempt, time.Now().UTC(), *force, client.Collect)
		cancel()
		if err != nil {
			slog.Error("FIDE refresh failed; previous snapshot retained", "error", err)
		} else {
			slog.Info("FIDE schedule checked", "updated", updated)
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
