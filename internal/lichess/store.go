package lichess

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
	return &Store{snapshot.Store{DB: db, Table: "lichess_snapshot", LockID: 73102602, Success: refreshInterval, Failure: refreshInterval}}
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

type Collector func(context.Context, time.Time) (Snapshot, error)

func (s *Store) Refresh(ctx context.Context, now time.Time, force bool, collect Collector) (bool, error) {
	return s.s.Refresh(ctx, now, force, func(ctx context.Context, now time.Time, _ []byte) ([]byte, error) {
		result, err := collect(ctx, now)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	})
}
