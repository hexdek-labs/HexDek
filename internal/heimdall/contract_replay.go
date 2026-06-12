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
	if rc == nil {
		return VerifyResult{}, errors.New("verify: nil ReplayContext")
	}
	if contract == nil {
		return VerifyResult{}, errors.New("verify: nil contract")
	}

	// Stage 1: signature + digest integrity.
	if err := contract.CheckIntegrity(key); err != nil {
		return VerifyResult{Detail: "integrity: " + err.Error()}, nil
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
			contract.EngineVersion, rc.EngineVersion)}, nil
	}

	// Stage 2: deterministic replay. Reuses the existing replay path
	// but captures the final game state rather than routing
	// observations to sinks.
	out, err := replayForOutcome(rc, contract)
	if err != nil {
		return VerifyResult{Detail: "replay: " + err.Error()}, err
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
	gs.RetainEvents = false

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
	// Known residual gap: the cap here is replayMaxTurns (80 ==
	// tournament.DefaultMaxTurns); games sealed by a runner configured
	// with a non-default MaxTurnsPerGame can diverge at the cap. The
	// contract schema does not carry max-turns yet — see
	// /tmp/fable-review plan note (schema bump territory).
	elim := tournament.NewElimTracker()
	elim.Mark(gs)

	startingSeat := gs.Active
	round := 1
	if gs.Flags == nil {
		gs.Flags = make(map[string]int)
	}
	gs.Flags["round"] = round
	ended := false
	for turn := 1; turn <= replayMaxTurns; turn++ {
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
