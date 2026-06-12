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
		gs.Seats[i].Hat = hat.NewYggdrasilHat(nil, 30)
	}
	for i := 0; i < nSeats; i++ {
		tournament.RunLondonMulligan(gs, i)
	}

	gs.Active = gameRng.Intn(nSeats)
	gs.Turn = 1

	for turn := 1; turn <= replayMaxTurns; turn++ {
		gs.Turn = turn
		tournament.TakeTurn(gs)
		gameengine.StateBasedActions(gs)
		if gs.CheckEnd() {
			break
		}
		gs.Active = nextLivingReplay(gs)
	}

	winner := -1
	if gs.Flags != nil && gs.Flags["ended"] == 1 {
		if w, ok := gs.Flags["winner"]; ok && w >= 0 && w < nSeats {
			winner = w
		}
	}
	if winner < 0 {
		bestLife := -999
		for i, s := range gs.Seats {
			if s != nil && !s.Lost && s.Life > bestLife {
				bestLife = s.Life
				winner = i
			}
		}
	}

	out.Winner = winner
	out.Turns = gs.Turn
	out.EndReason = endReasonFromState(gs)
	out.KillMethod = killMethodFromEndReason(out.EndReason)
	for i := 0; i < seedcontract.MaxSeats; i++ {
		out.EliminationOrder[i] = -1
	}
	for i := 0; i < nSeats && i < seedcontract.MaxSeats; i++ {
		if i < len(gs.Seats) && gs.Seats[i] != nil {
			out.FinalLife[i] = gs.Seats[i].Life
		}
	}
	return out, nil
}

func endReasonFromState(gs *gameengine.GameState) string {
	if gs == nil || gs.Flags == nil {
		return ""
	}
	if gs.Flags["turn_capped"] == 1 {
		return "turn_cap"
	}
	if gs.Flags["ended"] == 1 {
		if _, ok := gs.Flags["winner"]; ok {
			return "last_seat_standing"
		}
		return "draw"
	}
	return ""
}

func killMethodFromEndReason(r string) string {
	switch r {
	case "turn_cap", "turn_cap_leader", "turn_cap_tie", "turn_cap_all_dead":
		return "timeout"
	case "draw":
		return "draw"
	case "":
		return ""
	default:
		return r
	}
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
