package events

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
	schema := fmt.Sprintf("events_test_%d", time.Now().UnixNano())
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
	migration, err := os.ReadFile("../../db/migrations/000005_events_snapshot.up.sql")
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
	good := Snapshot{FetchedAt: now, Clubs: []Club{}, Events: []Event{{ID: "lichess-arena-Abcdefgh", Name: "Club Arena", Status: "upcoming", StartsAt: now}}}
	collect := func(context.Context, time.Time) (Snapshot, error) { return good, nil }
	if updated, err := store.Refresh(ctx, now, true, collect); err != nil || !updated {
		t.Fatalf("save: %v", err)
	}
	first, err := store.Read(ctx, now)
	if err != nil || first.Stale || first.Events[0].Name != "Club Arena" {
		t.Fatalf("read: %+v %v", first, err)
	}
	called := false
	if updated, err := store.Refresh(ctx, now.Add(time.Minute), false, func(context.Context, time.Time) (Snapshot, error) { called = true; return good, nil }); err != nil || updated || called {
		t.Fatal("fetched before the next refresh was due")
	}
	if _, err = store.Refresh(ctx, now, true, func(context.Context, time.Time) (Snapshot, error) { return Snapshot{}, errors.New("HTTP 429") }); err == nil {
		t.Fatal("expected failure")
	}
	saved, err := store.Read(ctx, now)
	if err != nil || !saved.Stale || saved.Events[0].Name != "Club Arena" || !saved.FetchedAt.Equal(first.FetchedAt) {
		t.Fatalf("lost last good snapshot: %+v %v", saved, err)
	}
	called = false
	store.Refresh(ctx, now.Add(10*time.Minute), false, func(context.Context, time.Time) (Snapshot, error) { called = true; return good, nil })
	if called {
		t.Fatal("did not respect retry backoff")
	}
	if updated, err := store.Refresh(ctx, now.Add(15*time.Minute), false, collect); err != nil || !updated {
		t.Fatalf("retry after 15 minutes: %v", err)
	}
	if saved, err = store.Read(ctx, now); err != nil || saved.Stale {
		t.Fatal("recovery did not clear failure")
	}
	if saved, err = store.Read(ctx, now.Add(2*time.Hour)); err != nil || !saved.Stale {
		t.Fatal("a snapshot older than an hour should be stale")
	}
}
