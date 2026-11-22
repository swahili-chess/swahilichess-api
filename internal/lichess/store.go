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

// BotRoster reads the Lichess IDs the chessbot keeps in its lichess table.
// The table belongs to the bot; this only reads it.
func BotRoster(db *sql.DB) Roster {
	return func(ctx context.Context) ([]string, error) {
		rows, err := db.QueryContext(ctx, `SELECT lichess_id FROM lichess`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		ids := []string{}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		return ids, rows.Err()
	}
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
