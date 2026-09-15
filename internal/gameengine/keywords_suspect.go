package gameengine

// keywords_suspect.go — Suspect (CR §701.60, Murders at Karlov Manor 2024)
// as a real designation toggle with menace grant + can't-block restriction
// that survive end-of-turn cleanup.
//
// CR §701.60a-c: "Suspect [creature]" — that creature becomes suspected,
//                gains menace and can't block. The designation persists until
//                the creature leaves the battlefield or another effect removes
//                it. Suspected creatures cannot be chosen as blockers
//                (CR §509.1b filter).
//
// Engine surface:
//
//   - SuspectCreature(gs, perm)
//       Stamps the designation. Sets perm.Flags["suspected"]=1 AND
//       perm.Flags["kw:menace"]=1 so Permanent.HasKeyword("menace")
//       reports true through the runtime-keyword channel. Critically
//       the flag-based grant is NOT cleared by ScanExpiredDurations
//       (which only walks GrantedAbilities and Modifications), so the
//       suspected status persists across turns until UnsuspectCreature
//       removes it.
//
//   - UnsuspectCreature(gs, perm)
//       Removes the designation: deletes both flags. The §701.62b
//       "investigate this creature" payoff lands here; per-card handlers
//       invoke UnsuspectCreature when their investigate-the-suspect
//       trigger resolves.
//
//   - IsSuspected(perm) bool
//       Reads perm.Flags["suspected"]. Backs the canBlockGS gate AND
//       any per-card "while suspected" rider.
//
// Integration with the block-declaration filter lives in combat.go's
// canBlockGS — see the IsSuspected branch added there. The existing
// resolve_helpers.go inline "suspect" ModificationEffect case is updated
// to delegate to SuspectCreature so both paths produce the same state
// shape (flag + menace, persistent).

// ---------------------------------------------------------------------------
// IsSuspected
// ---------------------------------------------------------------------------

// IsSuspected reports whether the permanent carries the suspected
// designation. CR §701.62a.
func IsSuspected(perm *Permanent) bool {
	if perm == nil || perm.Flags == nil {
		return false
	}
	return perm.Flags["suspected"] > 0
}

// ---------------------------------------------------------------------------
// SuspectCreature
// ---------------------------------------------------------------------------

// SuspectCreature stamps the suspected designation on `perm` and grants
// it menace via the runtime-keyword flag channel. CR §701.62a.
//
// Storage:
//   - perm.Flags["suspected"] = 1 — the designation itself
//   - perm.Flags["kw:menace"] = 1 — menace via Permanent.HasKeyword's
//     runtime-flag branch (combat.go HasKeyword reads kw:<name> flags)
//
// Both flags persist across turns since ScanExpiredDurations only
// clears Modifications and GrantedAbilities — Flags entries survive
// until explicitly removed.
//
// Idempotent: calling SuspectCreature on an already-suspected permanent
// is a no-op (no second event emitted).
//
// No-op when `perm` is nil or not a creature — suspecting a non-creature
// is meaningless per §701.62a.
func SuspectCreature(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if !perm.IsCreature() {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["suspected"] > 0 {
		return
	}
	perm.Flags["suspected"] = 1
	perm.Flags["kw:menace"] = 1
	gs.InvalidateCharacteristicsCache()

	name := ""
	if perm.Card != nil {
		name = perm.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "suspect",
		Seat:   perm.Controller,
		Source: name,
		Details: map[string]interface{}{
			"rule": "701.62a",
		},
	})
	FireCardTrigger(gs, "suspected", map[string]interface{}{
		"perm":            perm,
		"card_name":       name,
		"controller_seat": perm.Controller,
	})
}

// ---------------------------------------------------------------------------
// UnsuspectCreature
// ---------------------------------------------------------------------------

// UnsuspectCreature removes the suspected designation and the menace
// grant that came with it. CR §701.62b — investigating a suspected
// creature is the canonical clear.
//
// Idempotent: calling on a non-suspected permanent is a no-op.
//
// Note: this only removes the menace flag that Suspect stamped. If the
// creature has menace from its printed AST or a separate granted ability,
// HasKeyword("menace") will still return true via those channels. The
// flag-clear is targeted at the Suspect-installed menace grant only.
func UnsuspectCreature(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil || perm.Flags == nil {
		return
	}
	if perm.Flags["suspected"] == 0 {
		return
	}
	delete(perm.Flags, "suspected")
	delete(perm.Flags, "kw:menace")
	gs.InvalidateCharacteristicsCache()

	name := ""
	if perm.Card != nil {
		name = perm.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "unsuspect",
		Seat:   perm.Controller,
		Source: name,
		Details: map[string]interface{}{
			"rule": "701.62b",
		},
	})
	FireCardTrigger(gs, "unsuspected", map[string]interface{}{
		"perm":            perm,
		"card_name":       name,
		"controller_seat": perm.Controller,
	})
}
