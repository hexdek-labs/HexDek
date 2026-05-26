package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// investigate_consumers_r60.go — per_card OnTrigger handlers for the
// investigate / Clue family (CR §701.15, primarily SOI / EMA / MID
// support).
//
// Per the Versailles Phase 1B audit (PR #477 §3) `investigate` is one
// of the 29 engine-emitted events with no per_card consumer wired,
// despite a sizable corpus of investigate-causing payoffs. The engine
// has the full plumbing — ApplyInvestigateEffect mints Clue tokens,
// FireInvestigateTriggers fires the `investigate` event for observers
// — but no per_card handler subscribed to either causing investigates
// from a triggered ability OR listening on the resulting event.
//
// This file wires 8 handlers across three trigger shapes:
//
//   (1) Investigate CAUSERS — fire on some triggering event (ETB,
//       cast, combat damage, sacrifice, etc.) and call
//       ApplyInvestigateEffect + FireInvestigateTriggers. The
//       FireInvestigateTriggers call is essential: it lets listeners
//       (Erdwal, Tireless Tracker) react to the new investigate.
//
//   (2) Investigate LISTENERS — listen on the `investigate` event
//       fired by FireInvestigateTriggers and act on it (Erdwal's
//       first-time-per-turn doubler).
//
//   (3) Clue PAYOFFS — listen on `permanent_sacrificed` and filter
//       for Clue type to recognise the controller cashing in a Clue
//       (Tireless Tracker's +1/+1 counter on sac-clue).

func init() {
	registerInvestigateConsumersR60(Global())
	AddResetHook(registerInvestigateConsumersR60)
}

func registerInvestigateConsumersR60(r *Registry) {
	if r == nil {
		return
	}
	// Causers.
	r.OnTrigger("Lonis, Cryptozoologist", "permanent_etb", lonisCryptozoologistETB)
	r.OnTrigger("Thraben Inspector", "permanent_etb", thrabenInspectorETB)
	r.OnTrigger("Briarbridge Tracker", "permanent_etb", briarbridgeTrackerETB)
	r.OnTrigger("Bygone Bishop", "creature_spell_cast", bygoneBishopCreatureCast)
	r.OnTrigger("Trail of Evidence", "instant_or_sorcery_cast", trailOfEvidenceCast)
	r.OnTrigger("Tireless Tracker", "permanent_etb", tirelessTrackerLandETB)
	r.OnTrigger("Ongoing Investigation", "combat_damage_player", ongoingInvestigationCombatDamage)
	// Listener.
	r.OnTrigger("Erdwal Illuminator", "investigate", erdwalIlluminatorInvestigate)
	// Clue payoff (sacrifice listener).
	r.OnTrigger("Tireless Tracker", "permanent_sacrificed", tirelessTrackerSacClue)
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// runInvestigate mints n Clue tokens for seatIdx AND fires the
// investigate trigger fan-out. Centralising this ensures every causer
// fires the event so listeners (Erdwal) see it.
func runInvestigate(gs *gameengine.GameState, perm *gameengine.Permanent, seatIdx, n int) {
	if n <= 0 {
		return
	}
	gameengine.ApplyInvestigateEffect(gs, seatIdx, n)
	// FireInvestigateTriggers fires once per investigate event. Per
	// CR §701.15 "investigating an additional time" semantics, callers
	// that need to investigate N>1 in one go fire one trigger per
	// investigate. We mirror that by calling FireInvestigateTriggers
	// once per Clue minted.
	for i := 0; i < n; i++ {
		gameengine.FireInvestigateTriggers(gs, seatIdx, perm, nil)
	}
}

// isClueCard returns true if the card's types include "clue".
func isClueCard(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		if strings.EqualFold(t, "clue") {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// (1) Causers
// -----------------------------------------------------------------------------

// Lonis, Cryptozoologist — {G}{U} Legendary Creature — Snake Elf Scout
//
// Whenever another nontoken creature you control enters, investigate.
func lonisCryptozoologistETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "lonis_cryptozoologist_investigate"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	enteringPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if enteringPerm == nil || enteringPerm.Card == nil {
		return
	}
	// "another" — exclude Lonis herself.
	if enteringPerm == perm {
		return
	}
	// "nontoken creature" — must be creature, must not be token.
	if !enteringPerm.IsCreature() || enteringPerm.IsToken() {
		return
	}
	// "you control" — the entering perm's controller must match Lonis's.
	enteringSeat, _ := ctx["controller_seat"].(int)
	if enteringSeat != perm.Controller {
		return
	}
	runInvestigate(gs, perm, perm.Controller, 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"entering": enteringPerm.Card.DisplayName(),
	})
}

// Thraben Inspector — {W} Creature — Human Soldier
//
// When this creature enters, investigate.
func thrabenInspectorETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "thraben_inspector_investigate"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	enteringPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if enteringPerm != perm {
		return
	}
	runInvestigate(gs, perm, perm.Controller, 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

// Briarbridge Tracker — {2}{G} Creature — Human Scout
//
// When this creature enters, investigate. (The "as long as you control
// a token, this creature gets +2/+0" continuous effect is a static
// modifier, not a triggered ability — out of scope for this PR.)
func briarbridgeTrackerETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "briarbridge_tracker_investigate"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	enteringPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if enteringPerm != perm {
		return
	}
	runInvestigate(gs, perm, perm.Controller, 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

// Bygone Bishop — {2}{W} Creature — Spirit Cleric
//
// Whenever you cast a creature spell with mana value 3 or less,
// investigate.
func bygoneBishopCreatureCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "bygone_bishop_investigate"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	if card.CMC > 3 {
		return
	}
	runInvestigate(gs, perm, perm.Controller, 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"spell": card.DisplayName(),
		"cmc":   card.CMC,
	})
}

// Trail of Evidence — {2}{U} Enchantment
//
// Whenever you cast an instant or sorcery spell, investigate.
func trailOfEvidenceCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "trail_of_evidence_investigate"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	runInvestigate(gs, perm, perm.Controller, 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

// Tireless Tracker — {1}{G} Creature — Human Scout
//
// Trigger A (landfall): Whenever a land you control enters, investigate.
// Trigger B (sac-clue): Whenever you sacrifice a Clue, put a +1/+1
//
//	counter on this creature.
func tirelessTrackerLandETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "tireless_tracker_landfall"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	enteringPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if enteringPerm == nil {
		return
	}
	if !enteringPerm.IsLand() {
		return
	}
	enteringSeat, _ := ctx["controller_seat"].(int)
	if enteringSeat != perm.Controller {
		return
	}
	runInvestigate(gs, perm, perm.Controller, 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
		"land": enteringPerm.Card.DisplayName(),
	})
}

func tirelessTrackerSacClue(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "tireless_tracker_sac_clue_counter"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	// "you sacrifice a Clue" — the sac'd permanent must be a Clue and
	// must have been controlled by Tracker's controller.
	sacPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if sacPerm == nil || sacPerm.Card == nil {
		return
	}
	if !isClueCard(sacPerm.Card) {
		return
	}
	sacSeat, _ := ctx["controller_seat"].(int)
	if sacSeat != perm.Controller {
		return
	}
	perm.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"counters": perm.Counters["+1/+1"],
	})
}

// Ongoing Investigation — {1}{G}{U} Enchantment
//
// Whenever one or more creatures you control deal combat damage to a
// player, investigate. (The activated "{1}{G}, exile a creature card
// from your graveyard: investigate. You gain 2 life." is an activated
// ability handled via the AST cost pipeline — out of scope here.)
//
// combat_damage_player fires once per (attacker, defending player)
// pair, but the oracle text says "one or more creatures ... deal combat
// damage to a player" which is the §603.3c batched form — investigate
// ONCE per combat phase per defending player. We dedupe via a per-turn
// flag keyed on the defender seat.
func ongoingInvestigationCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ongoing_investigation_investigate"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	defenderSeat, ok := ctx["defender_seat"].(int)
	if !ok {
		// Fall back to "target_seat" for older ctx shapes.
		defenderSeat, _ = ctx["target_seat"].(int)
	}
	// Per-turn, per-defender dedupe.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	key := "ongoing_invest_fired_" + intToShort(defenderSeat) + "_t"
	if perm.Flags[key] == gs.Turn+1 {
		return
	}
	perm.Flags[key] = gs.Turn + 1

	runInvestigate(gs, perm, perm.Controller, 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"defender": defenderSeat,
	})
}

func intToShort(n int) string {
	if n < 0 {
		return "n" + intToShort(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return intToShort(n/10) + intToShort(n%10)
}

// -----------------------------------------------------------------------------
// (2) Listener
// -----------------------------------------------------------------------------

// Erdwal Illuminator — {1}{U} Creature — Spirit
//
// Whenever you investigate for the first time each turn, investigate an
// additional time.
//
// The first-time-per-turn gate is critical: without it, every Erdwal-
// triggered investigate would itself fire the investigate event and
// recurse forever (engine's trigger_depth cap would eventually swallow
// it but with a noisy event log). The Turn+1 stamped flag breaks the
// recursion at the first re-fire.
func erdwalIlluminatorInvestigate(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "erdwal_illuminator_double_investigate"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	investigatingSeat, _ := ctx["seat"].(int)
	if investigatingSeat != perm.Controller {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["erdwal_doubled_turn"] == gs.Turn+1 {
		return
	}
	perm.Flags["erdwal_doubled_turn"] = gs.Turn + 1

	runInvestigate(gs, perm, perm.Controller, 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}
