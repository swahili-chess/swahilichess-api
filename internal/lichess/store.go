package lichess

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

var ErrUnavailable = errors.New("no saved Lichess snapshot")

type Store struct{ DB *sql.DB }

func (s *Store) Read(ctx context.Context, now time.Time) (Response, error) {
	var payload []byte
	var failed bool
	err := s.DB.QueryRowContext(ctx, `SELECT payload, last_attempt_failed FROM lichess_snapshot WHERE id = true`).Scan(&payload, &failed)
	if err != nil {
		return Response{}, err
	}
	if len(payload) == 0 {
		return Response{}, ErrUnavailable
	}
	var result Response
	if err = json.Unmarshal(payload, &result.Snapshot); err != nil {
		return result, err
	}
	result.Stale = failed || now.Sub(result.FetchedAt) > staleAfter
	return result, nil
}

type Collector func(context.Context, time.Time) (Snapshot, error)

// Refresh serializes collectors across processes and replaces the snapshot in one
// statement only after the whole roster is collected. Readers never fetch.
func (s *Store) Refresh(ctx context.Context, now time.Time, force bool, collect Collector) (bool, error) {
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	var locked bool
	if err = conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(73102602)`).Scan(&locked); err != nil {
		return false, err
	}
	if !locked {
		return false, nil
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := conn.ExecContext(cleanup, `SELECT pg_advisory_unlock(73102602)`); err != nil {
			// Discard a connection whose session lock could not be released.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
	}()
	var next time.Time
	if err = conn.QueryRowContext(ctx, `SELECT next_attempt_at FROM lichess_snapshot WHERE id = true`).Scan(&next); err != nil {
		return false, err
	}
	if !force && now.Before(next) {
		return false, nil
	}
	snapshot, collectErr := collect(ctx, now)
	if collectErr != nil {
		// Even cancellation must record the failure without replacing the last good data.
		recordCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err = conn.ExecContext(recordCtx, `UPDATE lichess_snapshot SET last_attempt_at=$1, next_attempt_at=$2, last_attempt_failed=true WHERE id=true`, now, now.Add(refreshInterval))
		return false, errors.Join(collectErr, err)
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return false, err
	}
	_, err = conn.ExecContext(ctx, `UPDATE lichess_snapshot SET payload=$1, last_attempt_at=$2, next_attempt_at=$3, last_attempt_failed=false WHERE id=true`, payload, now, now.Add(refreshInterval))
	return err == nil, err
}
