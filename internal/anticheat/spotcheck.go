// Package-level orchestrator for the deterministic-replay spot-check
// pipeline. The three components — SpotCheckScheduler (scheduler.go),
// VerificationWorker (worker.go), and CauterizeService (cauterize.go)
// — are independently usable, but production callers want to start
// the whole pipeline with one call. That's what Service does.
//
// Lifecycle:
//
//   svc, err := anticheat.NewService(db, rc, cfg)
//   svc.Start(ctx)   // spawns scheduler-tick + verification-worker
//   ...
//   svc.Stop()       // cancels + waits for both goroutines
//
// The scheduler loop scans showmatch_game for finished rows where
// verified=0 and the game isn't already in verification_queue, then
// asks the underlying SpotCheckScheduler to roll dice and enqueue.
// The verification worker is the existing VerificationWorker.Run
// loop — Service just owns its goroutine.
//
// Backwards compatibility: NewService rejects a nil db or nil
// ReplayContext, so a deployment that hasn't built a corpus-loaded
// ReplayContext yet (tests, dev mode) simply doesn't construct a
// Service. Showmatch checks for nil and runs without spot-checks
// when absent.

package anticheat

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/hexdek/hexdek/internal/heimdall"
)

// Config tunes the orchestrator. Zero values get sensible defaults.
type Config struct {
	// SampleRate is the per-game inclusion probability the scheduler
	// uses. The scheduler clamps to [MinSamplingRate, MaxSamplingRate]
	// internally; pass anything in that range here.
	SampleRate float64

	// SchedulerInterval is how often we scan showmatch_game for new
	// rows to consider sampling. Cheap query; small interval is fine.
	SchedulerInterval time.Duration

	// VerifierPollEvery passes through to VerificationWorkerOptions.
	// The worker drains the queue on every tick; this just sets the
	// quiet-period sleep between drains.
	VerifierPollEvery time.Duration

	// LookbackWindow caps how far back the scheduler scans on each
	// tick. Games older than this are skipped — verification_queue
	// shouldn't grow unbounded if the verifier stalls.
	LookbackWindow time.Duration

	// EngineVersion is stamped onto synthesised SeedContracts when
	// the worker rebuilds them from queue rows. Empty is acceptable;
	// digest comparison is the load-bearing check.
	EngineVersion string

	// ContractKey is the HMAC key the verifier uses for the
	// internally-synthesised SeedContracts. Internal-only — the
	// queue row is the source of truth for inputs and the claim, so
	// a fresh per-server random key is fine. Must be ≥16 bytes.
	ContractKey []byte

	// RngSeed seeds the SpotCheckScheduler's per-game Bernoulli
	// picker. Pass 0 for time-seeded (production); non-zero for
	// reproducible test runs.
	RngSeed int64

	// Logger overrides log.Default(). Useful in tests that capture
	// output; production should leave this nil.
	Logger *log.Logger
}

// Defaults — load-bearing constants live here so a misconfigured
// deployment has predictable behaviour, not silent feature loss.
const (
	DefaultSampleRate        = 0.03
	DefaultSchedulerInterval = 30 * time.Second
	DefaultVerifierPollEvery = 5 * time.Second
	DefaultLookbackWindow    = 6 * time.Hour
)

// withDefaults returns a Config with zero fields replaced by the
// canonical default. Out-of-range SampleRate is forwarded as-is so
// SpotCheckScheduler's existing clamp logic owns that decision.
func (c Config) withDefaults() Config {
	if c.SampleRate <= 0 {
		c.SampleRate = DefaultSampleRate
	}
	if c.SchedulerInterval <= 0 {
		c.SchedulerInterval = DefaultSchedulerInterval
	}
	if c.VerifierPollEvery <= 0 {
		c.VerifierPollEvery = DefaultVerifierPollEvery
	}
	if c.LookbackWindow <= 0 {
		c.LookbackWindow = DefaultLookbackWindow
	}
	if c.Logger == nil {
		c.Logger = log.Default()
	}
	return c
}

// Service is the public orchestrator façade. Owns the scheduler +
// worker + cauterize trio and their goroutines.
type Service struct {
	cfg       Config
	db        *sql.DB
	scheduler *SpotCheckScheduler
	worker    *VerificationWorker
	cauterize *CauterizeService

	// lastScannedFinishedAt is the high-water mark of the most
	// recent finished_at the scheduler tick has examined. Persisted
	// only in memory — if the process restarts the next scan picks
	// up from cutoff = now-LookbackWindow.
	lastScannedFinishedAt int64

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewService wires the trio. The ReplayContext must already have
// its corpus loaded; callers in main() construct it once at startup.
//
// Errors on nil db / nil rc / short ContractKey so a misconfigured
// deployment fails fast at startup rather than silently disabling
// verification.
func NewService(db *sql.DB, rc *heimdall.ReplayContext, cfg Config) (*Service, error) {
	if db == nil {
		return nil, errors.New("anticheat: nil DB")
	}
	if rc == nil {
		return nil, errors.New("anticheat: nil ReplayContext")
	}
	if len(cfg.ContractKey) < 16 {
		return nil, errors.New("anticheat: ContractKey must be >=16 bytes")
	}
	cfg = cfg.withDefaults()

	cauter := NewCauterizeService(db)
	verifier := NewHeimdallVerifier(rc, cfg.ContractKey, cfg.EngineVersion)
	worker := NewVerificationWorker(db, verifier, cauter, VerificationWorkerOptions{
		PollEvery: cfg.VerifierPollEvery,
		Logger:    cfg.Logger,
	})
	scheduler := NewSpotCheckScheduler(db, cfg.SampleRate, cfg.RngSeed)

	return &Service{
		cfg:       cfg,
		db:        db,
		scheduler: scheduler,
		worker:    worker,
		cauterize: cauter,
	}, nil
}

// Start launches the scheduler-tick goroutine and the verification
// worker. Idempotent — a second call while the service is running
// is a no-op. Pass a parent context so callers can cancel the whole
// pipeline at shutdown.
func (s *Service) Start(parent context.Context) {
	if s.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel

	// Verification worker — its existing Run loop owns its own ticker.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.worker.Run(ctx)
	}()

	// Scheduler tick — scan for new finished games, hand the IDs to
	// SelectAndEnqueue. We run an immediate first tick so a
	// freshly-started server covers the lookback window before
	// SchedulerInterval elapses.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		t := time.NewTicker(s.cfg.SchedulerInterval)
		defer t.Stop()
		s.runSchedulerTick(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.runSchedulerTick(ctx)
			}
		}
	}()

	s.cfg.Logger.Printf("anticheat: spot-check service started — rate=%.2f%% lookback=%s",
		s.scheduler.Rate()*100, s.cfg.LookbackWindow)
}

// Stop cancels the loops and waits for them to drain. Safe to call
// before Start (no-op).
func (s *Service) Stop() {
	if s.cancel == nil {
		return
	}
	s.cancel()
	s.cancel = nil
	s.wg.Wait()
}

// runSchedulerTick queries showmatch_game for finished, unsampled
// games inside the lookback window and feeds the IDs to
// SpotCheckScheduler. Sampling is per-game Bernoulli inside
// scheduler.Select, so ~SampleRate of the candidates per tick get
// enqueued.
//
// Concurrency-safe with itself: the candidate query excludes games
// already present in verification_queue (any status), so a tick
// that overlaps the previous one cannot double-enqueue.
func (s *Service) runSchedulerTick(ctx context.Context) {
	cutoff := time.Now().Add(-s.cfg.LookbackWindow).Unix()
	if s.lastScannedFinishedAt > cutoff {
		cutoff = s.lastScannedFinishedAt
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT g.game_id, g.finished_at
		FROM showmatch_game g
		WHERE g.finished_at >= ?
		  AND g.rng_seed != 0
		  AND g.verified = 0
		  AND NOT EXISTS (
		      SELECT 1 FROM verification_queue v WHERE v.game_id = g.game_id
		  )
		ORDER BY g.finished_at ASC
	`, cutoff)
	if err != nil {
		s.cfg.Logger.Printf("anticheat: scheduler scan: %v", err)
		return
	}
	var ids []int64
	var maxFinished int64 = s.lastScannedFinishedAt
	for rows.Next() {
		var id, fin int64
		if err := rows.Scan(&id, &fin); err != nil {
			_ = rows.Close() // best-effort cleanup; scan error is the reportable one
			s.cfg.Logger.Printf("anticheat: scheduler scan: %v", err)
			return
		}
		ids = append(ids, id)
		if fin > maxFinished {
			maxFinished = fin
		}
	}
	_ = rows.Close() // sql.Rows.Close is idempotent; rows.Err below surfaces any real failure
	if err := rows.Err(); err != nil {
		s.cfg.Logger.Printf("anticheat: scheduler scan: %v", err)
		return
	}

	// Advance the watermark even when nothing was selected so the
	// next tick doesn't re-scan the same window. This is intentional:
	// a candidate the dice rolled against and skipped is *done* —
	// it should not get a second roll on the next tick.
	s.lastScannedFinishedAt = maxFinished

	if len(ids) == 0 {
		return
	}
	queueIDs, err := s.scheduler.SelectAndEnqueue(ctx, ids)
	if err != nil {
		s.cfg.Logger.Printf("anticheat: scheduler enqueue: %v", err)
		return
	}
	if len(queueIDs) > 0 {
		s.cfg.Logger.Printf("anticheat: enqueued %d verification(s) from %d candidate(s)",
			len(queueIDs), len(ids))
	}
}

// Scheduler exposes the underlying SpotCheckScheduler for tests +
// admin tooling that wants to enqueue a specific game by hand.
func (s *Service) Scheduler() *SpotCheckScheduler { return s.scheduler }

// Worker exposes the verification worker.
func (s *Service) Worker() *VerificationWorker { return s.worker }

// Cauterize exposes the sanction service.
func (s *Service) Cauterize() *CauterizeService { return s.cauterize }
