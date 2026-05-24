package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// tla_flashback_grants_r60.go — R60 sweep of graveyard-flashback grant
// cards that share Iroh, Grand Lotus's pattern but had no per_card
// handler.
//
// Surfaces covered here:
//
//   - A-Lier, Disciple of the Drowned — Alchemy variant of Lier.
//     ETB-registered continuous grant, OnlyActiveTurn=true ("as long
//     as it's your turn").
//
//   - Return the Past — red Enchantment. ETB-registered continuous
//     grant, OnlyActiveTurn=true ("during your turn").
//
//   - Backdraft Hellkite — red Dragon. Attack-triggered EOT mass
//     grant for every instant/sorcery in the controller's graveyard.
//     Uses the one-shot RegisterEOTGraveyardFlashbackGrant primitive
//     introduced for Past in Flames / Will of the Jeskai.
//
// All three reuse the GraveyardFlashbackGrant primitive from
// keywords_flashback_grant.go. Cost predicate is
// PrintedMassFlashbackCost (every i/s in the graveyard gets flashback
// at its printed mana cost; nothing else qualifies).

func init() {
	registerTLAFlashbackGrantsR60(Global())
	AddResetHook(registerTLAFlashbackGrantsR60)
}

func registerTLAFlashbackGrantsR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnETB("A-Lier, Disciple of the Drowned", aLierDiscipleETB)
	r.OnTrigger("A-Lier, Disciple of the Drowned", "permanent_ltb", aLierDiscipleLTB)

	r.OnETB("Return the Past", returnThePastETB)
	r.OnTrigger("Return the Past", "permanent_ltb", returnThePastLTB)

	r.OnTrigger("Backdraft Hellkite", "attacks", backdraftHellkiteAttacks)
}

// -----------------------------------------------------------------------------
// A-Lier, Disciple of the Drowned (Alchemy)
// -----------------------------------------------------------------------------
//
// Oracle text:
//
//	Spells can't be countered.
//	As long as it's your turn, each instant and sorcery card in your
//	graveyard has flashback. The flashback cost is equal to that
//	card's mana cost.
//
// The uncounterable surface on A-Lier is handled by the AST keyword
// pipeline (or by reusing Lier's spell_cast hook if/when the alchemy
// alias is wired). Here we only wire the active-turn-gated flashback
// grant — that's the gap.

func aLierDiscipleETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	grant := &gameengine.GraveyardFlashbackGrant{
		Controller:      perm.Controller,
		SourceTimestamp: perm.Timestamp,
		SourceName:      perm.Card.DisplayName(),
		OnlyActiveTurn:  true,
		CostFor:         gameengine.PrintedMassFlashbackCost,
	}
	gameengine.RegisterGraveyardFlashbackGrant(gs, grant)
	emit(gs, "a_lier_disciple_of_the_drowned_flashback_grant", perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"only_active_turn": true,
	})
}

func aLierDiscipleLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gameengine.ExpireGraveyardFlashbackGrantsBySource(gs, perm.Timestamp)
}

// -----------------------------------------------------------------------------
// Return the Past
// -----------------------------------------------------------------------------
//
// Oracle text ({4}{R}{R} Enchantment):
//
//	During your turn, each instant and sorcery card in your graveyard
//	has flashback. Its flashback cost is equal to its mana cost.

func returnThePastETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	grant := &gameengine.GraveyardFlashbackGrant{
		Controller:      perm.Controller,
		SourceTimestamp: perm.Timestamp,
		SourceName:      perm.Card.DisplayName(),
		OnlyActiveTurn:  true,
		CostFor:         gameengine.PrintedMassFlashbackCost,
	}
	gameengine.RegisterGraveyardFlashbackGrant(gs, grant)
	emit(gs, "return_the_past_flashback_grant", perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"only_active_turn": true,
	})
}

func returnThePastLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gameengine.ExpireGraveyardFlashbackGrantsBySource(gs, perm.Timestamp)
}

// -----------------------------------------------------------------------------
// Backdraft Hellkite
// -----------------------------------------------------------------------------
//
// Oracle text ({3}{R}{R} Creature — Dragon):
//
//	Flying
//	Whenever this creature attacks, each instant and sorcery card in
//	your graveyard gains flashback until end of turn. The flashback
//	cost is equal to its mana cost.
//
// This is a one-shot mass grant emitted at attack-trigger resolution
// time — no continuous source-tied grant, so we use
// RegisterEOTGraveyardFlashbackGrant which the cleanup-step sweep
// flushes at end of turn.
//
// The "attacks" trigger fires for each creature attack event; combat.go
// supplies ctx["attacker_perm"] so we can scope the grant to attacks
// that this specific Hellkite participated in (the registry dispatches
// the handler for every Backdraft Hellkite on the battlefield, but
// each call should only register a grant when this Hellkite is the
// attacker).

func backdraftHellkiteAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "backdraft_hellkite_attack_flashback"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	attacker, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if attacker != perm {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	gameengine.RegisterEOTGraveyardFlashbackGrant(gs, seat, perm.Card.DisplayName(), gameengine.PrintedMassFlashbackCost)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"graveyard_len": len(gs.Seats[seat].Graveyard),
	})
}
