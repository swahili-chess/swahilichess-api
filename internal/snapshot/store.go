// Package snapshot persists one provider's last good ratings payload in a
// single-row table with a refresh schedule. Readers never trigger collection.
package snapshot

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"time"
)

var ErrUnavailable = errors.New("no saved snapshot")

type Store struct {
	DB *sql.DB
	// Table is a trusted constant from the provider package, never user input.
	Table  string
	LockID int64
	// Success and Failure set the delay before the next attempt.
	Success, Failure time.Duration
}

func (s *Store) Read(ctx context.Context) (payload []byte, failed bool, err error) {
	err = s.DB.QueryRowContext(ctx, `SELECT payload, last_attempt_failed FROM `+s.Table+` WHERE id = true`).Scan(&payload, &failed)
	if err == nil && len(payload) == 0 {
		err = ErrUnavailable
	}
	return payload, failed, err
}

// Collector receives the previous payload, which may be empty, and returns the
// complete replacement.
type Collector func(ctx context.Context, now time.Time, previous []byte) ([]byte, error)

// Refresh serializes collectors across processes and replaces the payload in one
// statement only after collection succeeds.
func (s *Store) Refresh(ctx context.Context, now time.Time, force bool, collect Collector) (bool, error) {
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	var locked bool
	if err = conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, s.LockID).Scan(&locked); err != nil {
		return false, err
	}
	if !locked {
		return false, nil
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := conn.ExecContext(cleanup, `SELECT pg_advisory_unlock($1)`, s.LockID); err != nil {
			// Discard a connection whose session lock could not be released.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
	}()
	var next time.Time
	var previous []byte
	if err = conn.QueryRowContext(ctx, `SELECT next_attempt_at, payload FROM `+s.Table+` WHERE id = true`).Scan(&next, &previous); err != nil {
		return false, err
	}
	if !force && now.Before(next) {
		return false, nil
	}
	payload, collectErr := collect(ctx, now, previous)
	if collectErr != nil {
		// Even cancellation must record the failure without replacing the last good data.
		recordCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err = conn.ExecContext(recordCtx, `UPDATE `+s.Table+` SET last_attempt_at=$1, next_attempt_at=$2, last_attempt_failed=true WHERE id=true`, now, now.Add(s.Failure))
		return false, errors.Join(collectErr, err)
	}
	_, err = conn.ExecContext(ctx, `UPDATE `+s.Table+` SET payload=$1, last_attempt_at=$2, next_attempt_at=$3, last_attempt_failed=false WHERE id=true`, payload, now, now.Add(s.Success))
	return err == nil, err
}
