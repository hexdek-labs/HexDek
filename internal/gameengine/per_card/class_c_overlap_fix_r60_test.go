package per_card

import (
	"testing"
)

// R60 Phase 2I Class C overlap fix regressions. For each of the four
// cards in the cleanup, verify the live registry has exactly the
// expected number of handlers per (card, event) — catches a future
// re-introduction of the duplicate registration that was the source
// of the broken behavior.

// handlerCounts returns (etbCount, triggerCount) for a (card, event)
// pair on the active global registry. Resets the registry first so
// the count reflects what the bootstrap actually installs in
// production, not whatever stray state the previous test left behind.
func handlerCounts(card, event string) (etb int, trigger int) {
	Reset()
	r := Global()
	k := normalizeName(card)
	etb = len(r.etb[k])
	if r.onTrigger[k] != nil {
		trigger = len(r.onTrigger[k][event])
	}
	return etb, trigger
}

// TestToxrillTheCorrosive_NoDuplicateEndStepHandler — the broken gen
// (drew a card on every end_step + minted "1/1 Token Token") used to
// register OnTrigger("Toxrill...", "end_step", toxrillTheCorrosiveTrigger1)
// alongside the custom's correct slime-counter handler. The gen file
// was deleted entirely in R60 Phase 2I; only the custom should remain.
//
// Pin: exactly 1 end_step handler, exactly 0 creature_dies handlers
// (gen also registered a duplicate creature_dies trigger that drew +
// minted; custom doesn't register creature_dies at all — the slug
// trigger lives on the slime counter, applied during end_step).
func TestToxrillTheCorrosive_NoDuplicateEndStepHandler(t *testing.T) {
	_, endStep := handlerCounts("Toxrill, the Corrosive", "end_step")
	if endStep != 1 {
		t.Errorf("Toxrill end_step: want exactly 1 handler (custom slime+destroy), got %d — gen handler may have leaked back", endStep)
	}
	_, dies := handlerCounts("Toxrill, the Corrosive", "creature_dies")
	if dies != 0 {
		t.Errorf("Toxrill creature_dies: want 0 handlers (gen's broken 1/1-Token-Token + draw handler was deleted), got %d", dies)
	}
}

// TestMendicantCoreGuidelight_NoDuplicateSpellCastHandler — pre-fix
// both gen (observation-only emit) and custom (real copy work) ran
// on `spell_cast`. R60 dropped the gen registration so only the
// custom fires.
func TestMendicantCoreGuidelight_NoDuplicateSpellCastHandler(t *testing.T) {
	_, spellCast := handlerCounts("Mendicant Core, Guidelight", "spell_cast")
	if spellCast != 1 {
		t.Errorf("Mendicant Core spell_cast: want exactly 1 handler (custom mendicantMaxSpeedCopy), got %d", spellCast)
	}
	// gen's other registrations (ETB, combat_begin, upkeep_controller)
	// stay; verify they still fire.
	etb, _ := handlerCounts("Mendicant Core, Guidelight", "")
	if etb != 1 {
		t.Errorf("Mendicant Core OnETB: want 1 (mendicantETB), got %d", etb)
	}
	_, combatBegin := handlerCounts("Mendicant Core, Guidelight", "combat_begin")
	if combatBegin != 1 {
		t.Errorf("Mendicant Core combat_begin: want 1 (mendicantBeginCombat), got %d", combatBegin)
	}
}

// TestRienneAngelOfRebirth_NoDuplicateETBHandler — pre-fix both gen
// (observation-only emit) and custom (anthem-refresh) ran on ETB.
// R60 dropped the gen registration so only the custom fires.
func TestRienneAngelOfRebirth_NoDuplicateETBHandler(t *testing.T) {
	etb, _ := handlerCounts("Rienne, Angel of Rebirth", "")
	if etb != 1 {
		t.Errorf("Rienne OnETB: want exactly 1 handler (custom rienneRefreshAnthemOnETB), got %d", etb)
	}
	// gen's creature_dies (recursion) + custom's permanent_etb/_ltb
	// (anthem refresh) are unique to each side and should remain.
	// Note: "creature_dies" canonicalizes to "die" via event_aliases.go
	// when registered through OnTrigger.
	_, dies := handlerCounts("Rienne, Angel of Rebirth", "die")
	if dies != 1 {
		t.Errorf("Rienne creature_dies (canonical 'die'): want 1 (gen rienneAngelOfRebirthDies), got %d", dies)
	}
	_, permETB := handlerCounts("Rienne, Angel of Rebirth", "permanent_etb")
	if permETB != 1 {
		t.Errorf("Rienne permanent_etb: want 1 (custom anthem-refresh), got %d", permETB)
	}
}

// TestKolodinTriumphCaster_NoDuplicateETBHandler — same observation-
// only gen-vs-real-work-custom pattern as Mendicant Core and Rienne.
// Pin: only the custom OnETB handler remains.
func TestKolodinTriumphCaster_NoDuplicateETBHandler(t *testing.T) {
	etb, _ := handlerCounts("Kolodin, Triumph Caster", "")
	if etb != 1 {
		t.Errorf("Kolodin OnETB: want exactly 1 handler (custom kolodinRefreshAnthemOnETB), got %d", etb)
	}
	// permanent_etb is a functional split: gen does the
	// Mount-saddle / Vehicle-animate one-shots; custom does the
	// haste-anthem refresh. BOTH should remain.
	_, permETB := handlerCounts("Kolodin, Triumph Caster", "permanent_etb")
	if permETB != 2 {
		t.Errorf("Kolodin permanent_etb: want 2 handlers (gen Mount/Vehicle stamping + custom haste-anthem refresh — intentional functional split), got %d", permETB)
	}
}

// TestToxrillTheCorrosive_RegistryHandlerIsCustom — a more semantic
// pin: walking the live end_step handlers for Toxrill and checking
// none of them mints a "1/1 Token Token" or draws a card. Since we
// can't directly identify a function pointer, drive the handler with
// a minimal fixture and assert observable behavior: an end_step on
// an opponent's turn with no slime-bearing creatures should NOT
// draw a card or create any tokens.
func TestToxrillTheCorrosive_RegistryHandlerIsCustom(t *testing.T) {
	gs := newGame(t, 2)
	toxrill := addPerm(gs, 0, "Toxrill, the Corrosive", "creature", "legendary")

	libBefore := len(gs.Seats[0].Library)
	bfBefore := len(gs.Seats[0].Battlefield)
	handBefore := len(gs.Seats[0].Hand)

	// Drive the trigger via the registry, exactly how the engine would.
	r := Global()
	k := normalizeName("Toxrill, the Corrosive")
	for _, h := range r.onTrigger[k]["end_step"] {
		h(gs, toxrill, map[string]interface{}{"active_seat": 1})
	}

	// The broken gen would have drawn a card (hand+1, library-1) AND
	// minted a "1/1 Token Token" (battlefield+1). The correct custom
	// just walks opponent creatures; with zero opponent creatures
	// there's nothing to slime or destroy.
	if len(gs.Seats[0].Library) != libBefore {
		t.Errorf("Toxrill end_step (no opp creatures): library size changed %d → %d — broken gen draw-a-card may be back",
			libBefore, len(gs.Seats[0].Library))
	}
	if len(gs.Seats[0].Hand) != handBefore {
		t.Errorf("Toxrill end_step (no opp creatures): hand size changed %d → %d — broken gen draw may be back",
			handBefore, len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[0].Battlefield) != bfBefore {
		t.Errorf("Toxrill end_step (no opp creatures): battlefield size changed %d → %d — broken gen '1/1 Token Token' mint may be back",
			bfBefore, len(gs.Seats[0].Battlefield))
	}
}
