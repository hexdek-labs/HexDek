// snapshot-backfill bulk-populates the deck_freya_profile table (added
// in PR #517) for every deck in showmatch_elo, so the snapshot DB
// carries the synergy_pct / archetype / power_tier_counts scalars
// needed by the bracket-vs-ELO correlation analysis (docs/bracket-elo-
// distribution-r60.md "Freya synergy cross-reference" section).
//
// Default flow per row in showmatch_elo:
//
//  1. If a deck_freya_profile row already exists and --force is NOT
//     set, skip (idempotent — re-runs are cheap).
//  2. If a .profile.json sidecar exists under
//     <decks-dir>/<owner>/freya/<id>.profile.json, read + upsert
//     directly (no Freya invocation). This is the cheap path.
//  3. Otherwise, if the deck file (.txt / .json) exists, invoke the
//     hexdek-freya CLI to generate the sidecar, then upsert.
//  4. If neither sidecar nor deck file is present, count as
//     "missing" and move on.
//
// Usage:
//
//	snapshot-backfill --db data/hexdek-snapshot.db
//	snapshot-backfill --db data/hexdek-snapshot.db --owner moxfield --limit 50
//	snapshot-backfill --db data/hexdek-snapshot.db --force --concurrency 4
//	snapshot-backfill --db data/hexdek-snapshot.db --dry-run
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"

	"github.com/hexdek/hexdek/internal/db"
)

// freyaInvoker is the interface that runs Freya against a single deck
// file. Production wires it to the hexdek-freya CLI; tests stub it
// to write a canned .profile.json beside the input. Keeping this
// extractable from main() is what makes the runner testable without
// shelling out to a binary.
type freyaInvoker interface {
	Invoke(ctx context.Context, deckPath string) error
}

// execFreya is the production invoker. Shells out to `hexdek-freya
// --deck <path> --format json`. The CLI's saveFreyaData writes the
// .profile.json sidecar next to the deck file as a side effect, so
// the runner doesn't need to capture stdout.
type execFreya struct {
	bin string
}

func (e execFreya) Invoke(ctx context.Context, deckPath string) error {
	cmd := exec.CommandContext(ctx, e.bin, "--deck", deckPath, "--format", "json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("freya %s: %w\n%s", deckPath, err, truncate(string(out), 400))
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// runStats accumulates the per-deck outcomes for the end-of-run
// summary line. Atomic counters so the concurrent worker pool doesn't
// need a mutex around the bookkeeping.
type runStats struct {
	considered     atomic.Int64
	skippedExists  atomic.Int64
	upsertedSidecar atomic.Int64
	upsertedFreya  atomic.Int64
	missingDeck    atomic.Int64
	freyaFailed    atomic.Int64
	parseFailed    atomic.Int64
	upsertFailed   atomic.Int64
	skippedFilter  atomic.Int64
}

func (s *runStats) String() string {
	return fmt.Sprintf(
		"considered=%d upserted_from_sidecar=%d upserted_via_freya=%d "+
			"skipped_already_populated=%d skipped_filter=%d "+
			"missing_deck_file=%d freya_failed=%d parse_failed=%d upsert_failed=%d",
		s.considered.Load(), s.upsertedSidecar.Load(), s.upsertedFreya.Load(),
		s.skippedExists.Load(), s.skippedFilter.Load(),
		s.missingDeck.Load(), s.freyaFailed.Load(),
		s.parseFailed.Load(), s.upsertFailed.Load())
}

// backfillConfig is the resolved CLI flag set, extracted so the
// runner can be invoked from tests with explicit values.
type backfillConfig struct {
	DecksDir    string
	OwnerFilter string
	Limit       int
	Concurrency int
	Force       bool
	DryRun      bool
	Verbose     bool
}

// deckRow is one row pulled from showmatch_elo with just the fields
// the runner needs. We intentionally don't pull rating / games here
// — those are showmatch's job; this CLI is single-purpose.
type deckRow struct {
	DeckKey   string
	Commander string
	Owner     string
}

// parseDeckKey splits "<owner>/<id>" into its parts. deck_key is
// constructed by the deck import path as `sanitizedOwner + "/" +
// sanitizedID`, neither of which can contain a slash, so a single
// split at the first slash is sufficient. Returns empty strings if
// the key is malformed; the caller treats that as "skip".
func parseDeckKey(key string) (owner, id string) {
	i := strings.Index(key, "/")
	if i < 0 {
		return "", ""
	}
	return key[:i], key[i+1:]
}

// resolveDeckPath returns the first existing .txt / .json deck file
// under decksDir/owner/id. Mirrors findDeckFile in hexapi/handler.go
// so this CLI agrees with the server's view of "what's on disk."
// Returns "" when neither extension hits.
func resolveDeckPath(decksDir, owner, id string) string {
	dir := filepath.Join(decksDir, owner)
	for _, ext := range []string{".txt", ".json"} {
		p := filepath.Join(dir, id+ext)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// sidecarPath is where saveFreyaData writes the .profile.json
// (cmd/hexdek-freya/main.go saveFreyaData → freyaDir/base.profile.json).
// Keep this in lock-step with the writer; the test suite pins the
// shape via Test_SaveProfileJSON_EmitsExpectedScalars in PR #517.
func sidecarPath(decksDir, owner, id string) string {
	return filepath.Join(decksDir, owner, "freya", id+".profile.json")
}

// loadShowmatchRows pulls the deck-identity columns from showmatch_elo
// honoring the optional --owner filter + --limit. Returns rows in
// stable deck_key order so concurrent re-runs produce deterministic
// logs.
func loadShowmatchRows(ctx context.Context, sqlDB *sql.DB, ownerFilter string, limit int) ([]deckRow, error) {
	q := `SELECT deck_key, commander, owner FROM showmatch_elo`
	args := []any{}
	if ownerFilter != "" {
		q += ` WHERE owner = ?`
		args = append(args, ownerFilter)
	}
	q += ` ORDER BY deck_key`
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := sqlDB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query showmatch_elo: %w", err)
	}
	defer rows.Close()
	var out []deckRow
	for rows.Next() {
		var r deckRow
		if err := rows.Scan(&r.DeckKey, &r.Commander, &r.Owner); err != nil {
			return nil, fmt.Errorf("scan showmatch_elo: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// hasFreyaProfile is a cheap existence probe used to honor the
// "skip if already populated" contract without --force. Returns
// (false, nil) for sql.ErrNoRows so callers don't have to thread
// errors.Is checks everywhere.
func hasFreyaProfile(ctx context.Context, sqlDB *sql.DB, deckKey string) (bool, error) {
	var k string
	err := sqlDB.QueryRowContext(ctx,
		`SELECT deck_key FROM deck_freya_profile WHERE deck_key = ?`,
		deckKey).Scan(&k)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// processRow handles one deck: idempotent-skip check, sidecar-or-
// invoke decision, parse, upsert. Returns nil even on most failure
// modes (the run continues, the stats counters carry the signal) so
// a single bad deck doesn't abort the whole 1300-row backfill.
// Catastrophic errors (DB I/O on the upsert path) DO return so the
// caller can decide whether to bail.
func processRow(
	ctx context.Context,
	sqlDB *sql.DB,
	cfg backfillConfig,
	row deckRow,
	invoker freyaInvoker,
	stats *runStats,
) error {
	stats.considered.Add(1)

	owner, id := parseDeckKey(row.DeckKey)
	if owner == "" || id == "" {
		if cfg.Verbose {
			log.Printf("  skip malformed deck_key %q", row.DeckKey)
		}
		stats.skippedFilter.Add(1)
		return nil
	}

	if !cfg.Force {
		exists, err := hasFreyaProfile(ctx, sqlDB, row.DeckKey)
		if err != nil {
			return fmt.Errorf("probe deck_freya_profile: %w", err)
		}
		if exists {
			stats.skippedExists.Add(1)
			return nil
		}
	}

	sidecar := sidecarPath(cfg.DecksDir, owner, id)
	freshFromFreya := false
	if _, err := os.Stat(sidecar); err != nil {
		// No sidecar — try to generate one if the deck file exists.
		deckPath := resolveDeckPath(cfg.DecksDir, owner, id)
		if deckPath == "" {
			if cfg.Verbose {
				log.Printf("  skip %s: no deck file under %s/%s", row.DeckKey, cfg.DecksDir, owner)
			}
			stats.missingDeck.Add(1)
			return nil
		}
		if invoker == nil {
			// Caller wants sidecar-only mode (e.g. dry-run, or no
			// freya binary). Mirror the missing-deck branch.
			if cfg.Verbose {
				log.Printf("  skip %s: no sidecar + no freya invoker", row.DeckKey)
			}
			stats.missingDeck.Add(1)
			return nil
		}
		if cfg.DryRun {
			if cfg.Verbose {
				log.Printf("  would invoke freya on %s", deckPath)
			}
			return nil
		}
		if err := invoker.Invoke(ctx, deckPath); err != nil {
			log.Printf("  freya invoke failed for %s: %v", row.DeckKey, err)
			stats.freyaFailed.Add(1)
			return nil
		}
		// Freya should have written the sidecar; re-stat.
		if _, err := os.Stat(sidecar); err != nil {
			log.Printf("  freya succeeded but no sidecar at %s: %v", sidecar, err)
			stats.freyaFailed.Add(1)
			return nil
		}
		freshFromFreya = true
	}

	blob, err := os.ReadFile(sidecar)
	if err != nil {
		log.Printf("  read sidecar %s: %v", sidecar, err)
		stats.parseFailed.Add(1)
		return nil
	}
	profile, err := db.FreyaProfileFromJSON(blob, row.DeckKey, owner, row.Commander)
	if err != nil {
		log.Printf("  parse sidecar %s: %v", sidecar, err)
		stats.parseFailed.Add(1)
		return nil
	}
	if cfg.DryRun {
		if cfg.Verbose {
			log.Printf("  would upsert %s: archetype=%q bracket=%d synergy=%.1f%%",
				row.DeckKey, profile.PrimaryArchetype, profile.Bracket, profile.SynergyPct)
		}
		return nil
	}

	if err := db.UpsertFreyaProfile(ctx, sqlDB, profile); err != nil {
		log.Printf("  upsert %s: %v", row.DeckKey, err)
		stats.upsertFailed.Add(1)
		return nil
	}
	// Distinguish "we paid for a Freya run" from "we reused an
	// existing sidecar" so the summary line shows whether the slow
	// path fired. Tracked through the local freshFromFreya flag
	// rather than sidecar mtime, since tests routinely create
	// sidecars seconds before the runner starts and a mtime
	// comparison would misclassify them.
	if freshFromFreya {
		stats.upsertedFreya.Add(1)
	} else {
		stats.upsertedSidecar.Add(1)
	}
	return nil
}

// runBackfill orchestrates the worker pool. Extracted from main() so
// the integration tests drive it directly with a stub invoker and an
// in-memory DB.
func runBackfill(ctx context.Context, sqlDB *sql.DB, cfg backfillConfig, invoker freyaInvoker) (*runStats, error) {
	stats := &runStats{}
	rows, err := loadShowmatchRows(ctx, sqlDB, cfg.OwnerFilter, cfg.Limit)
	if err != nil {
		return stats, err
	}
	if len(rows) == 0 {
		return stats, nil
	}

	conc := cfg.Concurrency
	if conc < 1 {
		conc = 1
	}
	// Concurrency above worker-vs-row ratio buys nothing.
	if conc > len(rows) {
		conc = len(rows)
	}

	work := make(chan deckRow, conc)
	var wg sync.WaitGroup
	var firstFatal atomic.Value // stores error

	for i := 0; i < conc; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for row := range work {
				if err := processRow(ctx, sqlDB, cfg, row, invoker, stats); err != nil {
					firstFatal.CompareAndSwap(nil, err)
					return
				}
			}
		}()
	}
	for _, r := range rows {
		if v := firstFatal.Load(); v != nil {
			break
		}
		work <- r
	}
	close(work)
	wg.Wait()

	if v := firstFatal.Load(); v != nil {
		return stats, v.(error)
	}
	return stats, nil
}

func main() {
	var (
		dbPath      = flag.String("db", "", "(required) SQLite DB path containing showmatch_elo and deck_freya_profile tables")
		decksDir    = flag.String("decks-dir", "data/decks", "root directory holding <owner>/<id>.txt deck files")
		freyaBin    = flag.String("freya-bin", "", "path to hexdek-freya binary (default: look up on PATH, fall back to ./hexdek-freya)")
		ownerFilter = flag.String("owner", "", "only process decks under this owner (e.g. 'moxfield')")
		limit       = flag.Int("limit", 0, "maximum number of decks to process (0 = no limit)")
		concurrency = flag.Int("concurrency", 1, "number of parallel Freya invocations (1 = serial; Freya is CPU-heavy)")
		force       = flag.Bool("force", false, "re-process decks that already have a deck_freya_profile row")
		dryRun      = flag.Bool("dry-run", false, "report what would happen without writing to the DB or invoking Freya")
		verbose     = flag.Bool("verbose", false, "log every per-deck decision, not just the summary line")
		sidecarOnly = flag.Bool("sidecar-only", false, "never invoke Freya; only upsert from existing .profile.json files")
	)
	flag.Parse()

	if *dbPath == "" {
		fmt.Fprintln(os.Stderr, "snapshot-backfill: --db is required")
		flag.Usage()
		os.Exit(2)
	}

	cfg := backfillConfig{
		DecksDir:    *decksDir,
		OwnerFilter: *ownerFilter,
		Limit:       *limit,
		Concurrency: *concurrency,
		Force:       *force,
		DryRun:      *dryRun,
		Verbose:     *verbose,
	}

	var invoker freyaInvoker
	if !*sidecarOnly {
		bin := resolveFreyaBin(*freyaBin)
		if bin == "" {
			log.Printf("snapshot-backfill: no hexdek-freya binary found; running in sidecar-only mode")
		} else {
			invoker = execFreya{bin: bin}
		}
	}

	sqlDB, err := sql.Open("sqlite", *dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(max(1, *concurrency))

	ctx := context.Background()
	t0 := time.Now()
	stats, err := runBackfill(ctx, sqlDB, cfg, invoker)
	elapsed := time.Since(t0)
	log.Printf("snapshot-backfill done in %s: %s", elapsed.Round(time.Millisecond), stats)
	if err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

// resolveFreyaBin picks the freya binary in this order:
//  1. --freya-bin if explicitly set and exists
//  2. hexdek-freya on $PATH
//  3. ./hexdek-freya in cwd
//
// Returns "" if none of the three resolve; main() degrades to
// sidecar-only in that case so a missing binary doesn't abort.
func resolveFreyaBin(explicit string) string {
	if explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit
		}
		return ""
	}
	if p, err := exec.LookPath("hexdek-freya"); err == nil {
		return p
	}
	const fallback = "./hexdek-freya"
	if _, err := os.Stat(fallback); err == nil {
		return fallback
	}
	return ""
}
