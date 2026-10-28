package chesscom

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestSnapshotPersistence(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to an isolated local PostgreSQL database")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("chesscom_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec("DROP SCHEMA " + schema + " CASCADE")
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migration, err := os.ReadFile("../../db/migrations/000004_chesscom_snapshot.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(migration)); err != nil {
		t.Fatal(err)
	}
	store := NewStore(db)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	if _, err = store.Read(ctx, now); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("empty: %v", err)
	}
	good := Snapshot{Provider: "chesscom", Country: Country, FetchedAt: now, Players: []Player{{ID: "alice", Name: "Alice", PlayerID: 1, Ratings: map[string]*Rating{"blitz": {Value: 2000}}}}}
	var knownOnSecondRun []string
	collect := func(_ context.Context, _ time.Time, known []string) (Snapshot, error) {
		knownOnSecondRun = known
		return good, nil
	}
	if updated, err := store.Refresh(ctx, now, true, collect); err != nil || !updated {
		t.Fatalf("save: %v", err)
	}
	first, err := store.Read(ctx, now)
	if err != nil || first.Stale || first.Players[0].Ratings["blitz"].Value != 2000 {
		t.Fatalf("read: %+v %v", first, err)
	}
	called := false
	if updated, err := store.Refresh(ctx, now.Add(time.Minute), false, func(context.Context, time.Time, []string) (Snapshot, error) { called = true; return good, nil }); err != nil || updated || called {
		t.Fatal("fetched before the next daily refresh was due")
	}
	if _, err = store.Refresh(ctx, now, true, func(context.Context, time.Time, []string) (Snapshot, error) {
		return Snapshot{}, errors.New("HTTP 500")
	}); err == nil {
		t.Fatal("expected failure")
	}
	saved, err := store.Read(ctx, now)
	if err != nil || !saved.Stale || saved.Players[0].Ratings["blitz"].Value != 2000 || !saved.FetchedAt.Equal(first.FetchedAt) {
		t.Fatalf("lost last good snapshot: %+v %v", saved, err)
	}
	called = false
	store.Refresh(ctx, now.Add(5*time.Hour), false, func(context.Context, time.Time, []string) (Snapshot, error) { called = true; return good, nil })
	if called {
		t.Fatal("did not respect retry backoff")
	}
	if updated, err := store.Refresh(ctx, now.Add(6*time.Hour), false, collect); err != nil || !updated {
		t.Fatalf("retry after six hours: %v", err)
	}
	if len(knownOnSecondRun) != 1 || knownOnSecondRun[0] != "alice" {
		t.Fatalf("previous accounts not passed to the collector: %v", knownOnSecondRun)
	}
	if saved, err = store.Read(ctx, now); err != nil || saved.Stale {
		t.Fatal("recovery did not clear failure")
	}
	if saved, err = store.Read(ctx, now.Add(49*time.Hour)); err != nil || !saved.Stale {
		t.Fatal("a snapshot older than two days should be stale")
	}
}
