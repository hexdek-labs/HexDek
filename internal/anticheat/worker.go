package anticheat

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
	"github.com/hexdek/hexdek/internal/seedcontract"
)

// ReplayOutcome is the subset of game state the worker needs to
// compare against the claim. We deliberately don't pull the full
// seedcontract.Outcome into this layer — the worker only checks the
// observable fields a tampered DB row could have flipped (winner,
// turns).
type ReplayOutcome struct {
	Winner int
	Turns  int
	Detail string // free-form replay diagnostic

	// Inconclusive is the fail-closed hat-parity signal (r63 C-H4). When
	// true the worker MUST NOT sanction or mark the game verified: the
	// verifier could not establish that the replay used the same hat the
	// live game ran with, so a (winner, turns) divergence is ambiguous
	// between an honest game replayed with the wrong hat and a spoofed
	// one. Zero value (false) keeps the actionable pass/fail path, so a
	// stub verifier that reproduces the hat needs no change.
	Inconclusive bool
}

// Verifier is the abstraction the worker uses to run a deterministic
// replay of one queued game. Production wires HeimdallVerifier (which
// calls heimdall.VerifyReplay); tests wire a stub that returns
// canned outcomes.
type Verifier interface {
	Verify(ctx context.Context, row db.VerificationQueueRow) (ReplayOutcome, error)
}

// HeimdallVerifier replays games via the Phase 1 ReplayContext +
// SeedContract. The worker hands it a verification_queue row; the
// verifier reconstructs a SeedContract from the row's fields, runs
// heimdall.VerifyReplay, and surfaces (winner, turns) from the
// replayed result.
//
// Reconstructing the contract from row fields is intentional for
// Phase 2: the row IS the trust boundary. If the schedule wrote a
// row with the same fields the showmatch persisted, replay will
// confirm the original outcome. If an attacker mutated showmatch_game
// AFTER the row was enqueued, the queue row still has the original
// inputs — and the replay will diverge from the (forged) claim.
//
// Phase 1's signed contract is layered ON TOP of this in production:
// when SignedContract is non-empty (set by callers that have a
// stored, signed contract for the game), the verifier runs
// CheckIntegrity before replay, catching key-protected DB tampering.
type HeimdallVerifier struct {
	rc            *heimdall.ReplayContext
	contractKey   []byte
	engineVersion string
}

func NewHeimdallVerifier(rc *heimdall.ReplayContext, contractKey []byte, engineVersion string) *HeimdallVerifier {
	return &HeimdallVerifier{
		rc:            rc,
		contractKey:   contractKey,
		engineVersion: engineVersion,
	}
}

// Verify runs the deterministic replay and returns the replayed
// (winner, turns). The CheckIntegrity short-circuit fires inside
// VerifyReplay when contract.Sig is non-empty.
func (v *HeimdallVerifier) Verify(ctx context.Context, row db.VerificationQueueRow) (ReplayOutcome, error) {
	if v.rc == nil {
		return ReplayOutcome{}, errors.New("heimdall verifier: no ReplayContext")
	}
	if row.NSeats <= 0 || row.NSeats > seedcontract.MaxSeats {
		return ReplayOutcome{}, fmt.Errorf("invalid n_seats: %d", row.NSeats)
	}

	// Fail-closed hat-identity gate (r63 C-H4). The default replay hat
	// (heimdall's Yggdrasil(nil,30)) matches NEITHER live path: tournament
	// games run GreedyHat, and showmatch games run Yggdrasil + the deck's
	// evolving Curse-pool DNA + Freya strategy + budget 50 + dimStats —
	// none of which is recorded where this verifier can read it. Replaying
	// such a game with the wrong hat produces a different (winner, turns)
	// and would FALSE-POSITIVE cauterize an honest contributor. So we only
	// render a sanctionable verdict when the caller has supplied
	// rc.HatFactory, i.e. explicitly asserted "this replay uses the same
	// hat the live game ran with." Without it the game is UNVERIFIABLE,
	// not failed. (Reproducing the showmatch Curse hat — recording its
	// state into the contract + deterministic pool replay — is the larger
	// follow-up tracked in plan-anticheat-ch4-hat-identity.md; it is also
	// gated on C-H1 engine seed determinism.)
	if v.rc.HatFactory == nil {
		return ReplayOutcome{
			Inconclusive: true,
			Winner:       -1,
			Detail: "hat identity not reproduced: rc.HatFactory unset, so the replay " +
				"cannot prove parity with the live game's hat — not sanctionable (C-H4)",
		}, nil
	}
	in := seedcontract.Inputs{
		RNGSeed:       row.RNGSeed,
		NSeats:        row.NSeats,
		EngineVersion: v.engineVersion,
		SealedAtUnix:  row.EnqueuedAt,
	}
	for i := 0; i < seedcontract.MaxSeats && i < len(row.DeckKeys); i++ {
		in.DeckKeys[i] = row.DeckKeys[i]
	}
	contract := seedcontract.New(in)
	contract.Seal(seedcontract.Outcome{
		Winner:    row.ClaimedWinner,
		Turns:     row.ClaimedTurns,
		EndReason: "claimed",
	})
	if len(v.contractKey) > 0 {
		contract.Sign(v.contractKey)
	}

	// ReplayClaim, not VerifyReplay: the contract above carries only a
	// partial "claimed" outcome (no elimination order / final life), so
	// its outcome digest can never equal a real replay's. VerifyReplay's
	// digest comparison therefore always failed and stamped a misleading
	// "outcome digest mismatch" detail onto even PASSED rows; the worker's
	// real verdict is field equality on (winner, turns) below. ReplayClaim
	// runs the same integrity + engine-version gates and replay, minus the
	// meaningless comparison (r63 anticheat residual C-H2 #4).
	res, err := heimdall.ReplayClaim(v.rc, contract, v.contractKey)
	if err != nil {
		return ReplayOutcome{}, err
	}
	return ReplayOutcome{
		Winner: res.ReplayedOutcome.Winner,
		Turns:  res.ReplayedOutcome.Turns,
		Detail: res.Detail,
	}, nil
}

// VerificationWorker drains the verification_queue. One worker is
// enough — replay is CPU-bound and database contention on a single
// queue table is not the bottleneck.
type VerificationWorker struct {
	db        *sql.DB
	verifier  Verifier
	cauterize *CauterizeService
	pollEvery time.Duration
	logger    *log.Logger
}

// VerificationWorkerOptions carries optional configuration. Zero
// values produce sensible defaults.
type VerificationWorkerOptions struct {
	PollEvery time.Duration // default 5s
	Logger    *log.Logger   // default log.Default()
}

func NewVerificationWorker(database *sql.DB, verifier Verifier, cauterize *CauterizeService, opts VerificationWorkerOptions) *VerificationWorker {
	if opts.PollEvery <= 0 {
		opts.PollEvery = 5 * time.Second
	}
	if opts.Logger == nil {
		opts.Logger = log.Default()
	}
	return &VerificationWorker{
		db:        database,
		verifier:  verifier,
		cauterize: cauterize,
		pollEvery: opts.PollEvery,
		logger:    opts.Logger,
	}
}

// Run loops until ctx is cancelled, calling ProcessOne every
// pollEvery and on every successful processing burst (so a backlog
// drains as fast as the verifier can chew it).
func (w *VerificationWorker) Run(ctx context.Context) {
	t := time.NewTicker(w.pollEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		// Drain everything pending without sleeping between rows.
		for {
			processed, err := w.ProcessOne(ctx)
			if err != nil {
				w.logger.Printf("anticheat: worker: %v", err)
			}
			if !processed {
				break
			}
		}
	}
}

// ProcessOne claims the next pending row, runs the verifier, writes
// the terminal status, and (on failure) calls cauterize. Returns
// true when a row was processed.
func (w *VerificationWorker) ProcessOne(ctx context.Context) (bool, error) {
	row, err := db.ClaimNextVerification(ctx, w.db)
	if err != nil {
		return false, fmt.Errorf("claim: %w", err)
	}
	if row == nil {
		return false, nil
	}

	out, verifyErr := w.verifier.Verify(ctx, *row)
	if verifyErr != nil {
		// Treat verifier errors (e.g. missing corpus, missing deck
		// file) as "error" status, not "failed". An attacker
		// shouldn't be able to dodge cauterize by triggering a
		// transient infrastructure error.
		_ = db.FinishVerification(ctx, w.db, row.QueueID, "error", -1, 0, verifyErr.Error())
		return true, fmt.Errorf("verify queue=%d: %w", row.QueueID, verifyErr)
	}

	// Fail-closed hat-parity verdict (r63 C-H4): the verifier could not
	// establish that the replay used the live game's hat. Finish
	// non-sanctioning — never cauterize, never mark verified — so an
	// honest game whose hat we cannot reproduce is quarantined for review
	// rather than treated as a forgery.
	if out.Inconclusive {
		_ = db.FinishVerification(ctx, w.db, row.QueueID, "inconclusive", -1, 0, out.Detail)
		return true, nil
	}

	match := out.Winner == row.ClaimedWinner && out.Turns == row.ClaimedTurns
	if match {
		if err := db.FinishVerification(ctx, w.db, row.QueueID, "passed",
			out.Winner, out.Turns, out.Detail); err != nil {
			return true, fmt.Errorf("finish-passed: %w", err)
		}
		_ = db.MarkGameVerified(ctx, w.db, row.GameID)
		return true, nil
	}

	// Mismatch — cauterize the contributor under review on this row.
	reason := fmt.Sprintf(
		"replay mismatch on game %d: claimed (winner=%d, turns=%d), replayed (winner=%d, turns=%d). %s",
		row.GameID, row.ClaimedWinner, row.ClaimedTurns, out.Winner, out.Turns, out.Detail,
	)
	if err := db.FinishVerification(ctx, w.db, row.QueueID, "failed",
		out.Winner, out.Turns, reason); err != nil {
		return true, fmt.Errorf("finish-failed: %w", err)
	}
	_ = db.MarkGameUnverified(ctx, w.db, row.GameID)
	if w.cauterize != nil {
		if _, cerr := w.cauterize.ApplyOnFailure(ctx, row.DeckKey, row.QueueID, reason); cerr != nil {
			w.logger.Printf("anticheat: cauterize %s: %v", row.DeckKey, cerr)
		}
	}
	return true, nil
}
