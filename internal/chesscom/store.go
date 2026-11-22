package chesscom

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"api.swahilichess.com/internal/snapshot"
)

var ErrUnavailable = snapshot.ErrUnavailable

type Store struct{ s snapshot.Store }

func NewStore(db *sql.DB) *Store {
	return &Store{snapshot.Store{DB: db, Table: "chesscom_snapshot", LockID: 73102603, Success: refreshInterval, Failure: retryInterval}}
}

func (s *Store) Read(ctx context.Context, now time.Time) (Response, error) {
	payload, failed, err := s.s.Read(ctx)
	if err != nil {
		return Response{}, err
	}
	var result Response
	if err = json.Unmarshal(payload, &result.Snapshot); err != nil {
		return result, err
	}
	result.Stale = failed || now.Sub(result.FetchedAt) > staleAfter
	return result, nil
}

// Collector receives the usernames saved by the previous snapshot so accounts
// that drop off Chess.com's country list are still rechecked.
type Collector func(ctx context.Context, now time.Time, known []string) (Snapshot, error)

func (s *Store) Refresh(ctx context.Context, now time.Time, force bool, collect Collector) (bool, error) {
	return s.s.Refresh(ctx, now, force, func(ctx context.Context, now time.Time, previous []byte) ([]byte, error) {
		known := []string{}
		if len(previous) > 0 {
			var old Snapshot
			if err := json.Unmarshal(previous, &old); err != nil {
				return nil, err
			}
			for _, player := range old.Players {
				known = append(known, player.ID)
			}
		}
		result, err := collect(ctx, now, known)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	})
}
