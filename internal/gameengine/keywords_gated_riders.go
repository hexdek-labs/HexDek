package gameengine

// keywords_gated_riders.go — unifying dispatch for the resolve-time
// gated riders (Threshold §702.74, Metalcraft §702.97, Hellbent
// §702.45, Spell Mastery §702.106). Each rider has its own
// ApplyXxxRider executor that independently checks HasXxx + XxxActive
// and resolves the tagged payload; this file batches them so the
// resolver only needs to know one entry point.
//
// MaxSpeed (§702.178) is intentionally NOT included here. Max speed
// uses the same shape but is wired separately in resolveSequence
// (round 25) because it has its own counter rationale and was
// shipped before this round's bundling.
//
// Ordering: the three riders fire in the order Threshold → Metalcraft
// → Hellbent. A card carrying multiple gated riders (rare but legal —
// e.g. "Threshold — ..." plus "Metalcraft — ...") gets each rider
// evaluated independently, and any that are active fire in order. The
// ordering is deterministic so corpus tests don't flake; the choice
// of order is by §-section ascending alphabetical (Hellbent §702.45,
// Metalcraft §702.97, Threshold §702.74) — but we keep declaration
// order for code-reading clarity (Threshold first since it's the
// oldest pattern in the codebase). If a card needs a specific firing
// order across riders, the per_card layer can short-circuit by
// resolving only one rider's effect.

// resolveGatedRider dispatches Threshold + Metalcraft + Hellbent +
// Spell Mastery rider executors for `src`. Returns the number of
// riders that actually fired. Safe to call with nil gs / nil src —
// each ApplyXxxRider is itself nil-safe and no-ops when its
// preconditions aren't met.
//
// Called from resolveSequence at the END of the outer Sequence
// resolution (guarded by gs.Flags["_gated_rider_depth"] so nested
// Sequence nodes can't re-fire).
//
// Constellation (§702.131) is NOT included here despite the round-33
// task spec asking for it; constellation is a triggered ability on
// enchantment ETB, wired via OnConstellationETB in
// keywords_spell_mastery_riders.go and called from
// FirePermanentETBTriggers (etb_dispatch.go) — the timing-correct
// hook point.
func resolveGatedRider(gs *GameState, src *Permanent) int {
	fired := 0
	if ApplyThresholdRider(gs, src) {
		fired++
	}
	if ApplyMetalcraftRider(gs, src) {
		fired++
	}
	if ApplyHellbentRider(gs, src) {
		fired++
	}
	// Round-33 batch — same shape as the three above: Raid, Ferocious,
	// Revolt, Delirium, Coven. All five are ability words and have no
	// CR entry of their own (§207.2c); the §702.x numbers this comment
	// used to carry belonged to unrelated keywords. All are
	// nil/no-card safe.
	if ApplyRaidRider(gs, src) {
		fired++
	}
	if ApplyFerociousRider(gs, src) {
		fired++
	}
	if ApplyRevoltRider(gs, src) {
		fired++
	}
	if ApplyDeliriumRider(gs, src) {
		fired++
	}
	if ApplyCovenRider(gs, src) {
		fired++
	}
	// Round-33 (this branch) — Spell Mastery §702.106. Joins the
	// bundle alongside the round-33-elsewhere batch above.
	if ApplySpellMasteryRider(gs, src) {
		fired++
	}
	return fired
}
