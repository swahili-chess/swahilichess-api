package main

import (
	"flag"
	"log/slog"
	"os"
	"sync"
	"time"

	"api.swahilichess.com/config"
	"api.swahilichess.com/internal/chesscom"
	db "api.swahilichess.com/internal/db/sqlc"
	"api.swahilichess.com/internal/events"
	"api.swahilichess.com/internal/fide"
	"api.swahilichess.com/internal/lichess"
	"api.swahilichess.com/internal/nextsms"
	"github.com/go-playground/validator/v10"
	_ "github.com/lib/pq"
)

const version = "1.0.0"

type leaderboardCache struct {
	data      *Leaderboard
	expiresAt time.Time
	mu        sync.RWMutex
}

type application struct {
	config           config.Config
	fideStore        *fide.Store
	lichessStore     *lichess.Store
	chesscomStore    *chesscom.Store
	eventsStore      *events.Store
	store            db.Store
	wg               sync.WaitGroup
	validator        *validator.Validate
	nextsms          nextsms.NextSmS
	leaderboardCache leaderboardCache
}

func init() {

	var programLevel = new(slog.LevelVar)

	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: programLevel})
	slog.SetDefault(slog.New(h))

}

func main() {
	var cfg config.Config

	flag.StringVar(&cfg.HOST, "host", os.Getenv("HOST"), "API bind host; empty listens on all interfaces")
	flag.StringVar(&cfg.PORT, "port", os.Getenv("PORT"), "API server port")
	flag.StringVar(&cfg.ENV, "env", os.Getenv("ENV_STAGE"), "Environment (development|Staging|production")
	flag.StringVar(&cfg.DB.DSN, "db-dsn", os.Getenv("SW_DB_DSN"), "PostgreSQL DSN")

	flag.StringVar(&cfg.BasicAuth.USERNAME, "basicauth-username", os.Getenv("BASICAUTH_USERNAME"), "basicauth-username")
	flag.StringVar(&cfg.BasicAuth.PASSWORD, "basicauth-password", os.Getenv("BASICAUTH_PASSWORD"), "basicauth-password")

	flag.StringVar(&cfg.NextSmS.Username, "nextsms-username", os.Getenv("NEXTSMS_USERNAME"), "nextsms-username")
	flag.StringVar(&cfg.NextSmS.Password, "nextsms-password", os.Getenv("NEXTSMS_PASSWORD"), "nextsms-password")

	flag.IntVar(&cfg.DB.MaxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.DB.MaxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max ilde connections")
	flag.StringVar(&cfg.DB.MaxIdleTime, "db-max-idle-time", "15m", "PostgreSQL max connection  connections")

	flag.Parse()

	conn, err := config.OpenDB(cfg)
	if err != nil {
		slog.Error("failed to establish connection to db", "error", err)
		return
	}
	defer conn.Close()
	slog.Info("database connection pool established")

	app := &application{
		config:        cfg,
		fideStore:     &fide.Store{DB: conn},
		lichessStore:  lichess.NewStore(conn),
		chesscomStore: chesscom.NewStore(conn),
		eventsStore:   events.NewStore(conn),
		store:         db.NewStore(conn),
		validator:     validator.New(),
		nextsms:       nextsms.New(cfg.NextSmS.Username, cfg.NextSmS.Password),
	}

	err = app.serve()
	if err != nil {
		slog.Error("failed to start or shutdown server", "error", err)
		return
	}

}
