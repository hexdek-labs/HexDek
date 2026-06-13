package heimdall

import (
	"errors"
	"fmt"
	"math/rand"
	"runtime"

	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/seedcontract"
	"github.com/hexdek/hexdek/internal/tournament"
)

// VerifyResult is the outcome of a single contract replay. Detail
// reads "ok" on success, otherwise the first failed check (digest /
// signature / replay outcome).
type VerifyResult struct {
	OK              bool
	Detail          string
	ReplayedOutcome seedcontract.Outcome
	ReplayedDigest  string
}

// VerifyReplay re-executes the game described by contract.Inputs and
// confirms the recomputed outcome digest matches contract.OutcomeDigest.
// Phase 1 anti-cheat: a forged Outcome (e.g. claimed winner that
// differs from the actual winner of a deterministic replay) surfaces
// here as a digest mismatch.
//
// Two checks happen in sequence:
//
//  1. CheckIntegrity (digests + signature). Detects mutation of any
//     stored field after signing. Cheap — O(1).
//  2. Deterministic replay. Re-runs the game from RNG seed + deck
//     keys, then computes a fresh outcome digest. Expensive — O(game).
//
// Both must pass for the contract to be considered honest. The
// returned ReplayedOutcome is filled in regardless so callers can log
// the discrepancy when verification fails.
//
// Determinism prerequisites:
//   - Same engine version (verifier checks contract.EngineVersion
//     against rc-supplied build tag if non-empty)
//   - Same AST corpus loaded into rc
//   - Decks resolvable from rc.DeckDir using the contract's deck keys
//   - Hat factory in the replay matches what was used at game-start
//     (Phase 1 uses the simplified YggdrasilHat — same as
//     ReplayWithObservation)
func VerifyReplay(rc *ReplayContext, contract *seedcontract.SeedContract, key []byte) (VerifyResult, error) {
	gateFail, out, ran, err := runReplayGated(rc, contract, key)
	if err != nil {
		return gateFail, err
	}
	if !ran {
		// Integrity or engine-version gate failed; gateFail carries the
		// detail and OK==false, and no replay was attempted.
		return gateFail, nil
	}

	rederivedDigest := digestOutcomeFromContract(out)
	res := VerifyResult{
		ReplayedOutcome: out,
		ReplayedDigest:  rederivedDigest,
	}
	if rederivedDigest != contract.OutcomeDigest {
		res.Detail = fmt.Sprintf("outcome digest mismatch: claimed %s replayed %s",
			contract.OutcomeDigest, rederivedDigest)
		return res, nil
	}
	res.OK = true
	res.Detail = "ok"
	return res, nil
}

// ReplayClaim runs the deterministic replay for the worker's claim-check
// path and returns the replayed outcome — WITHOUT comparing outcome
// digests. The worker reconstructs the contract from a verification-queue
// row carrying only a PARTIAL outcome (EndReason "claimed", zero
// elimination order and final life), so its outcome digest can never
// equal a real replay's. The pre-r63 worker still called VerifyReplay,
// whose digest comparison therefore ALWAYS failed; the worker ignored
// that verdict and used field equality on (winner, turns) instead, but
// the meaningless "outcome digest mismatch" detail leaked onto every
// PASSED row's audit record. ReplayClaim drops the structurally
// meaningless comparison and reports an honest detail; the integrity and
// engine-version gates still run, and the caller decides the verdict from
// ReplayedOutcome. (r63 anticheat residual C-H2 #4.)
func ReplayClaim(rc *ReplayContext, contract *seedcontract.SeedContract, key []byte) (VerifyResult, error) {
	gateFail, out, ran, err := runReplayGated(rc, contract, key)
	if err != nil {
		return gateFail, err
	}
	if !ran {
		return gateFail, nil
	}
	return VerifyResult{
		OK:              true,
		Detail:          "ok (claim replay; outcome-digest comparison not applicable)",
		ReplayedOutcome: out,
		ReplayedDigest:  digestOutcomeFromContract(out),
	}, nil
}

// runReplayGated runs the two pre-replay gates (signature/digest
// integrity, then the engine-version gate) and, if both pass, the
// deterministic replay. It is the shared core of VerifyReplay (which adds
// the outcome-digest comparison) and ReplayClaim (which does not).
//
// Return contract:
//   - nil rc / nil contract → (zero, zero, false, err): replay can't run.
//   - a gate fails → (gateFail with Detail set and OK==false, zero, false,
//     nil): no replay attempted, so callers must not read ReplayedOutcome.
//   - replay errors → (gateFail{Detail:"replay: ..."}, zero, false, err).
//   - success → (zero VerifyResult, replayed outcome, true, nil).
func runReplayGated(rc *ReplayContext, contract *seedcontract.SeedContract, key []byte) (VerifyResult, seedcontract.Outcome, bool, error) {
	var zero seedcontract.Outcome
	if rc == nil {
		return VerifyResult{}, zero, false, errors.New("verify: nil ReplayContext")
	}
	if contract == nil {
		return VerifyResult{}, zero, false, errors.New("verify: nil contract")
	}

	// Stage 1: signature + digest integrity.
	if err := contract.CheckIntegrity(key); err != nil {
		return VerifyResult{Detail: "integrity: " + err.Error()}, zero, false, nil
	}

	// Stage 1.5: engine-version gate. A replay on a different engine
	// build is not evidence of tampering — surface it as its own
	// detail instead of a confusing digest mismatch. Both sides must
	// opt in (non-empty) for the check to fire, so existing callers
	// that never set rc.EngineVersion keep their behavior.
	if rc.EngineVersion != "" && contract.EngineVersion != "" &&
		rc.EngineVersion != contract.EngineVersion {
		return VerifyResult{Detail: fmt.Sprintf(
			"engine version mismatch: contract sealed on %q, verifier running %q — replay would not be comparable",
			contract.EngineVersion, rc.EngineVersion)}, zero, false, nil
	}

	// Stage 2: deterministic replay. Reuses the existing replay path
	// but captures the final game state rather than routing
	// observations to sinks.
	out, err := replayForOutcome(rc, contract)
	if err != nil {
		return VerifyResult{Detail: "replay: " + err.Error()}, zero, false, err
	}
	return VerifyResult{}, out, true, nil
}

// replayForOutcome runs a deterministic replay from the contract's
// inputs and returns the observed outcome. Mirrors ReplayWithObservation
// but skips observation extraction and instead reads winner / turns /
// final-life off the post-game GameState.
func replayForOutcome(rc *ReplayContext, contract *seedcontract.SeedContract) (out seedcontract.Outcome, retErr error) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			retErr = fmt.Errorf("panic: %v\n%s", r, buf[:n])
		}
	}()

	nSeats := contract.NSeats
	if nSeats <= 0 || nSeats > seedcontract.MaxSeats {
		return out, fmt.Errorf("invalid n_seats: %d", nSeats)
	}

	decks := make([]*deckparser.TournamentDeck, nSeats)
	for i := 0; i < nSeats; i++ {
		key := contract.DeckKeys[i]
		if key == "" {
			return out, fmt.Errorf("seat %d has empty deck key", i)
		}
		d, err := rc.resolveDeck(key)
		if err != nil {
			return out, fmt.Errorf("seat %d: %w", i, err)
		}
		decks[i] = d
	}

	gameRng := rand.New(rand.NewSource(contract.RNGSeed))

	gs := gameengine.NewGameState(nSeats, gameRng, rc.Corpus)
	gs.Seed = contract.RNGSeed // r62: hats key deterministic noise off gs.Seed
	gs.EventPolicy = gameengine.EventLogNone

	cmdDecks := make([]*gameengine.CommanderDeck, nSeats)
	for i := 0; i < nSeats; i++ {
		tpl := decks[i]
		lib := deckparser.CloneLibrary(tpl.Library)
		cmdrs := deckparser.CloneCards(tpl.CommanderCards)
		for _, c := range cmdrs {
			c.Owner = i
		}
		for _, c := range lib {
			c.Owner = i
		}
		gameRng.Shuffle(len(lib), func(a, b int) { lib[a], lib[b] = lib[b], lib[a] })
		cmdDecks[i] = &gameengine.CommanderDeck{
			CommanderCards: cmdrs,
			Library:        lib,
		}
	}

	gameengine.SetupCommanderGame(gs, cmdDecks)

	for i := 0; i < nSeats; i++ {
		if rc.HatFactory != nil {
			gs.Seats[i].Hat = rc.HatFactory(i)
			continue
		}
		// Default replay hat. Deterministic because gs.Seed is stamped
		// above (r62 #1020): seeded games reseed the hat's noise RNG
		// from (gs.Seed, seatIdx) on first observation, so the verify
		// path is a pure function of the contract inputs — wall-clock
		// randomness here would make every digest comparison a coin
		// flip (review 08, C-H1/C-H2).
		gs.Seats[i].Hat = hat.NewYggdrasilHat(nil, 30)
	}
	for i := 0; i < nSeats; i++ {
		tournament.RunLondonMulligan(gs, i)
	}

	gs.Active = gameRng.Intn(nSeats)
	gs.Turn = 1

	// r62 (review 08, C-H2): the outcome below is built with the SAME
	// shared bookkeeping the live runner uses — tournament.ElimTracker
	// for elimination slots, tournament.AdjudicateGameEnd for winner /
	// EndReason (including the turn-cap leader adjudication and its
	// Lost/Won mutations), tournament.FinalLifeFromState for life.
	// The pre-r62 replay hardcoded EliminationOrder to all -1 and used
	// a reduced EndReason vocabulary, so an HONEST game's replayed
	// digest never matched its sealed digest.
	//
	// Turn-loop parity with runOneGame: same TakeTurn implementation
	// (TakeTurn == takeTurnImpl(gs, nil)), same SBA call, Mark after
	// SBA, same round-flag bookkeeping, same next-living-seat rule.
	// The cap is rc.MaxTurns (falling back to replayMaxTurns == 80 ==
	// tournament.DefaultMaxTurns when unset). r63 anticheat residual
	// C-H2 #1: a game sealed by a runner configured with a non-default
	// MaxTurnsPerGame used to diverge at the cap here — the replay
	// always capped at 80 — so an honest long game's replayed digest
	// stopped matching its sealed one (and in the worker path that
	// false-positive cauterizes an honest contributor). The caller now
	// supplies the live cap via rc.MaxTurns; binding the cap into the
	// signed contract for malicious-runner detection remains schema-bump
	// territory (see the parked plan).
	maxTurns := replayMaxTurns
	if rc.MaxTurns > 0 {
		maxTurns = rc.MaxTurns
	}
	elim := tournament.NewElimTracker()
	elim.Mark(gs)

	startingSeat := gs.Active
	round := 1
	if gs.Flags == nil {
		gs.Flags = make(map[string]int)
	}
	gs.Flags["round"] = round
	ended := false
	for turn := 1; turn <= maxTurns; turn++ {
		gs.Turn = turn
		gs.Flags["round"] = round
		tournament.TakeTurn(gs)
		gameengine.StateBasedActions(gs)
		elim.Mark(gs)
		if gs.CheckEnd() {
			ended = true
			break
		}
		prev := gs.Active
		gs.Active = nextLivingReplay(gs)
		if gs.Active <= prev || gs.Active == startingSeat {
			round++
		}
	}

	out.Turns = gs.Turn
	winner, endReason := tournament.AdjudicateGameEnd(gs, nSeats, ended)
	elim.FillRemaining(gs)

	out.Winner = winner
	out.EndReason = endReason
	out.EliminationOrder = elim.Slots
	out.FinalLife = tournament.FinalLifeFromState(gs, nSeats)
	// KillMethod is derived, not chosen: CanonicalizeOutcome applies the
	// same (Winner, EndReason) → method rule Seal uses, so the struct we
	// return matches the bytes we digest.
	out = seedcontract.CanonicalizeOutcome(out)
	return out, nil
}

// digestOutcomeFromContract is a thin wrapper that produces the
// outcome digest in a way that's identical to seedcontract.Seal — we
// duplicate the call rather than calling Seal so VerifyReplay never
// mutates the contract argument it received.
func digestOutcomeFromContract(out seedcontract.Outcome) string {
	tmp := seedcontract.SeedContract{}
	tmp.Seal(out)
	return tmp.OutcomeDigest
}
