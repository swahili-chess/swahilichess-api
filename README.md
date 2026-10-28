# API for chess.tz sites and bots

Go, Echo and PostgreSQL. Website v2 work lives on `website-v2`. Existing auth, bot and `/lichess/leaderboard` routes are preserved.

## Local FIDE review

Requires Go 1.22.4 or newer and Docker with Compose. Tested with Go 1.27.3. Run from this repository:

```sh
docker compose -p chess-v2 -f compose.ratings.yml up -d --wait
export SW_DB_DSN='postgres://chess:local_review_only@127.0.0.1:55432/chess_ratings?sslmode=disable'
go run ./cmd/fide-refresh
go run ./cmd/api -host 127.0.0.1 -port 8081
```

The local-only credentials above belong to the isolated review database. Its first startup applies both migrations. For an existing development database, apply `db/migrations/000002_fide_snapshot.up.sql` once with your migration tool. No migration runs implicitly in the API.

The first import downloads three official monthly ZIP archives, about 33 MB compressed in October 2026. It may take a few minutes. Subsequent runs skip source requests until the next scheduled attempt. The API works without a collector process, but needs a saved snapshot to return FIDE ratings.

In the website repository, copy `.env.example` to `.env.local` if you do not already have local configuration, then run `pnpm dev`. `RATINGS_API_URL=http://127.0.0.1:8081` is server-only. Open <http://127.0.0.1:5174/leaderboards?provider=fide&category=standard>.

## Monthly worker

Run a separate worker alongside the API:

```sh
go run ./cmd/fide-refresh -watch
```

Alternatively, invoke the command without `-watch` from an existing scheduler. It checks a persisted due time before contacting FIDE. A successful import schedules the next check for 06:00 UTC on the first of the following rating month. The watch process checks that schedule hourly and exits on SIGINT/SIGTERM. A delayed publication or failed import retries after 24 hours. An advisory lock prevents overlapping collectors across processes.

To fetch a same-month correction manually:

```sh
go run ./cmd/fide-refresh -force
```

`-force` cannot be combined with `-watch`. It does not bypass parser validation. No scheduler or production deployment is configured by this local checkpoint.

## Snapshot contract

`GET /fide/leaderboard` only reads PostgreSQL. It returns `provider`, `federation`, `eligibility`, `ratingMonth` in `YYYY-MM`, `fetchedAt` in UTC, `stale`, `players`, and per-category `sources`. Sources record archive URLs, SHA-256 checksums, total records, Tanzania records, archive inactivity counts, excluded counts, and eligible rated counts. These counts describe each category, not distinct people across categories.

Players contain `id`, `name`, optional `title`, and `ratings` with `{ "value": 1971 }` entries. A missing category means no eligible rating. The collector matches FIDE search with "Exclude Inactive (FIDE rated only)" enabled. It first builds the eligible player IDs from active Standard-rated TAN records, then attaches their available Rapid and Blitz ratings. It does not apply the separate Rapid/Blitz archive inactivity flags as additional exclusions. This also excludes Standard-unrated players, matching the search. `eligibility` is `active-standard-rated`; source `inactive` records the raw archive flags, while `excluded` records actual eligibility exclusions. It does not match identities across platforms or store birthdays/sex.

The official [download page](https://ratings.fide.com/download_lists.phtml) identifies the publication month. Dated archives prevent month rollover from mixing lists. The parser validates the complete XML document and ZIP checksum, rejects duplicate Tanzania IDs and invalid fields, and reads all records. All three categories must succeed before one database update replaces the snapshot. Failed imports update attempt status without changing ratings or their fetch timestamp.

`stale` means the stored month precedes the current UTC month or the last import failed. With no saved snapshot, or a database failure, the endpoint returns HTTP 503 with a generic message. There is no public refresh/write endpoint. The website's Refresh button only reloads the saved data.

## Lichess ratings

Lichess has no country leaderboard, so the ranking includes members of the Nyumbani Mates team. Its member list is private. Create a Lichess personal access token with the `team:read` scope from an account in the team, and provide it only through the environment:

```sh
export LICHESS_TOKEN='<personal token>'
go run ./cmd/lichess-refresh          # once, when due
go run ./cmd/lichess-refresh -watch   # hourly until stopped
go run ./cmd/lichess-refresh -force   # once, immediately
```

Apply `db/migrations/000003_lichess_snapshot.up.sql` once to an existing database. The collector reads the team roster, then current ratings through the batch users API in serialized batches of up to 300. It excludes closed, terms-violation and bot accounts, stores unplayed categories as missing rather than Lichess's 1500 placeholder, and keeps provisional flags. Any failed request keeps the previous snapshot. Successful and failed runs both wait an hour before the next attempt.

`GET /lichess/ratings` only reads PostgreSQL. It returns `provider`, `team`, `fetchedAt`, `stale`, `players` with `id`, `name`, optional `title` and `ratings` such as `{ "value": 2304, "provisional": true }`, and `source` counts for members, closed, violated, bots, missing and ranked players. `stale` is true after a failed run or when the snapshot is more than three hours old. With no snapshot it returns HTTP 503. The original `/lichess/leaderboard` route and the bot's `lichess` table are unchanged.

## Chess.com ratings

Chess.com's official Published-Data API needs no login or token:

```sh
go run ./cmd/chesscom-refresh          # once, when due
go run ./cmd/chesscom-refresh -watch   # checks hourly, refreshes daily
go run ./cmd/chesscom-refresh -force   # once, immediately
```

Apply `db/migrations/000004_chesscom_snapshot.up.sql` once to an existing database. Each run reads `/pub/country/TZ/players`, adds every account saved by the previous snapshot, then reads each account's profile and stats. A category is ranked only when the account is at least 7 days old, has 20 games in that category, and played a rated game in any category in the last 90 days, matching Chess.com's leaderboard rules. Other accounts stay saved, unranked, for the next check. The country list only covers recently active accounts, so carrying saved accounts forward keeps players who have not logged in for a while. Each profile is rechecked: closed accounts, including fair-play closures, and accounts whose country is no longer Tanzania are dropped. Deleted accounts count as missing. Chess.com's stable `player_id` removes duplicates after a rename.

Four requests run at a time. HTTP 429, server errors and network failures retry up to five times with exponential backoff. Any request that still fails cancels the run and keeps the previous snapshot. A successful run waits 24 hours; a failed run retries after six hours. A full run of about 2,000 accounts takes several minutes.

`GET /chesscom/ratings` only reads PostgreSQL. It returns `provider`, `country`, `fetchedAt`, `stale`, `players` with `id` (lowercase username), `name`, `playerId`, optional `title` and `ratings` such as `{ "value": 1206 }`, and `source` counts for listed, previously known, checked, closed, moved, missing, saved and unranked accounts. A category without games is omitted. `stale` is true after a failed run or when the snapshot is more than two days old. With no snapshot it returns HTTP 503.

## Checks

```sh
go test ./...
go vet ./...
TEST_DATABASE_URL="$SW_DB_DSN" go test -race ./...
```

Database tests create and remove their own temporary schema. Always use the isolated local DSN. Without `TEST_DATABASE_URL`, database tests explicitly skip. Tests cover inactive and missing ratings, malformed/truncated XML, duplicate IDs, upstream failure, month selection, monthly scheduling, saved-snapshot retention, retry backoff, recovery, and freshness rollover.

Stop local services with Ctrl+C and `docker compose -p chess-v2 -f compose.ratings.yml stop`. The named volume retains the snapshot for the next review.
