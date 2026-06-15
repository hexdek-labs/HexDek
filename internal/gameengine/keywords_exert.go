package gameengine

// keywords_exert.go — Exert (CR §702.112, Amonkhet 2017).
//
// CR §702.112a: "You may exert [this creature] as it attacks" is a special
//                action a player may take as they declare attackers.
// CR §702.112b: "An exerted creature won't untap during your next untap step."
//                (A one-time skip, consumed by exactly that one untap step;
//                it untaps normally thereafter.)
// Cards with exert grant a bonus or trigger an ability when exerted.
//
// Engine model (mirrors the SaddleMount / enlist explicit-action primitives —
// a validated, optional action the caller drives; auto-offering the exert
// choice in the live DeclareAttackers/AI is a separate wiring step):
//
//   - HasExert       — thin keyword reader.
//   - ExertCreature  — exert an ATTACKING creature: set the one-shot
//                      skip-next-untap flag and fire the "exert" trigger for
//                      the "when you exert this creature, [bonus]" payoff.
//
// The skip flag is `exert_skip_next_untap` — deliberately DISTINCT from the
// permanent `skip_untap` flag (Basalt/Grim Monolith, which never untap on
// their own). UntapAll consumes (deletes) the exert flag after skipping one
// untap, so the creature untaps normally on the following untap step.

// HasExert reports whether the card has the exert keyword.
func HasExert(card *Card) bool {
	return cardHasKeywordByName(card, "exert")
}

// ExertCreature exerts `perm` as it attacks (CR §702.112). Only an attacking
// creature can be exerted (the action is taken "as it attacks"); exert is
// optional, so the caller decides whether to call this. On success the
// creature is flagged to skip its controller's next untap step (one-shot) and
// the "exert" trigger fires for any "when you exert this creature, [bonus]"
// ability. Returns true if the creature was exerted.
func ExertCreature(gs *GameState, perm *Permanent) bool {
	if gs == nil || perm == nil || perm.Card == nil {
		return false
	}
	if !perm.IsCreature() {
		return false
	}
	// CR §702.112a — a creature can only be exerted as it attacks.
	if !perm.IsAttacking() {
		return false
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	// Already exerted this combat — don't re-fire the trigger.
	if perm.Flags["exert_skip_next_untap"] > 0 {
		return false
	}
	// One-shot skip-next-untap (consumed by exactly one untap step in
	// UntapAll), distinct from the permanent `skip_untap` of Basalt/Grim
	// Monolith.
	perm.Flags["exert_skip_next_untap"] = 1

	gs.LogEvent(Event{
		Kind:   "exert",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.112",
		},
	})
	// "When you exert this creature, [bonus]" — the you_exert_creature → exert
	// alias family. Per-card payoffs consume this.
	FireCardTrigger(gs, "exert", map[string]interface{}{
		"perm":       perm,
		"card":       perm.Card,
		"controller": perm.Controller,
	})
	return true
}
