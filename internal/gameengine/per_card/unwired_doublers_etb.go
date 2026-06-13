package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// unwired_doublers_etb.go — wires up token/counter DOUBLER replacement
// effects (CR §614 / §122.1g) that were defined in the engine but never
// registered at ETB, so they were fully INERT in real games.
//
// Audit (replacement-effect sweep, skip/doublers category):
//   - The engine's RegisterReplacementsForPermanent switch (replacement.go)
//     only wires Doubling Season + Hardened Scales among the doubler family.
//   - counter_doublers.go defines RegisterPrimalVigor / RegisterBranchingEvolution
//     / RegisterVorinclexMonstrousRaider / RegisterOzolithShatteredSpire /
//     RegisterConclaveMentor — but those functions had ZERO non-test callers
//     (orphaned), so the cards never registered their replacements.
//   - Parallel Lives (pure token doubler) had no registration at all; its
//     would_create_tokens AST kind is dispatched nowhere. (Anointed Procession
//     — the white analogue — is already handled by a token_created TRIGGER
//     approximation in batch17_sweep.go, so it is intentionally NOT wired here
//     to avoid double-doubling.)
//
// Empirically verified inert (Primal Vigor present → +1/+1 counters NOT
// doubled). These OnETB handlers register each card's replacement(s) at the
// per_card ETB hook (fires on every entry — cast, reanimate, blink, token —
// strictly more robust than the cast-only switch path). Cleanup is automatic:
// UnregisterReplacementsForPermanent(p) removes by SourcePerm on LTB.
//
// Idempotency: a guard skips re-registration if the permanent already
// provides a replacement (these cards provide ONLY doubler replacements), so
// a double ETB fire cannot stack the doubler.
func init() {
	registerUnwiredDoublers(Global())
	AddResetHook(registerUnwiredDoublers)
}

func registerUnwiredDoublers(r *Registry) {
	r.OnETB("Primal Vigor", primalVigorETB)
	r.OnETB("Branching Evolution", branchingEvolutionETB)
	r.OnETB("Vorinclex, Monstrous Raider", vorinclexMonstrousRaiderETB)
	r.OnETB("Ozolith, the Shattered Spire", ozolithShatteredSpireETB)
	r.OnETB("Conclave Mentor", conclaveMentorETB)
	r.OnETB("Parallel Lives", parallelLivesETB)
}

// doublerAlreadyWired reports whether perm already sources a replacement.
// These doubler cards provide ONLY their doubler replacement(s), so any
// existing SourcePerm == perm entry means we already registered this entry's
// replacement — skip to avoid stacking on a double ETB fire. On flicker the
// permanent is a fresh pointer with no entries, so it re-registers correctly.
func doublerAlreadyWired(gs *gameengine.GameState, perm *gameengine.Permanent) bool {
	if gs == nil || perm == nil {
		return false
	}
	for _, re := range gs.Replacements {
		if re != nil && re.SourcePerm == perm {
			return true
		}
	}
	return false
}

func primalVigorETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil || doublerAlreadyWired(gs, perm) {
		return
	}
	gameengine.RegisterPrimalVigor(gs, perm)
}

func branchingEvolutionETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil || doublerAlreadyWired(gs, perm) {
		return
	}
	gameengine.RegisterBranchingEvolution(gs, perm)
}

func vorinclexMonstrousRaiderETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil || doublerAlreadyWired(gs, perm) {
		return
	}
	gameengine.RegisterVorinclexMonstrousRaider(gs, perm)
}

func ozolithShatteredSpireETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil || doublerAlreadyWired(gs, perm) {
		return
	}
	gameengine.RegisterOzolithShatteredSpire(gs, perm)
}

func conclaveMentorETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil || doublerAlreadyWired(gs, perm) {
		return
	}
	gameengine.RegisterConclaveMentor(gs, perm)
}

func parallelLivesETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil || doublerAlreadyWired(gs, perm) {
		return
	}
	gameengine.RegisterTokenDoublerControllerOnly(gs, perm)
}
