package gameengine

// InstanceID orphan sweep — Phase E residual-tail closure for the Phase 4
// ZoneConservation census per docs/instanceid-system-v2-r60.md §13.
//
// Background: Phase D (PR #773) routed every canonical token-removal path
// through `markPermanentCeaseIfToken` and added a universal cease at
// `gs.removePermanent`, dropping the 25k-game ZoneConservation violation
// count from 2,904,066 → 98,230 (96.6% reduction). The residual hits are
// shapes the Phase D chokepoints cannot reach by construction:
//
//   (1) Token *Card pointers that leave the battlefield via a path that
//       bypasses BOTH `gs.removePermanent` AND every per_card splice
//       chokepoint (e.g., direct slice manipulation in obscure helpers,
//       or transient-window leaks where a *Card briefly lives only in a
//       sideband zone like ZoneCastGrants / MadnessExile / PlotExile,
//       then is purged without re-routing).
//
//   (2) Non-token *Card pointers (basic lands, commanders, real cards)
//       that get dropped from every reachable zone — typically because
//       the gap-walk's same-pointer purge removed a stale zone reference
//       just before the caller intended to re-add the card to a new
//       zone, but a §614 replacement or other intervening effect
//       redirected the destination away, leaving the *Card unrouted.
//
//   (3) Control-change cycles where token Cards transfer seats and the
//       transient window between old-controller-remove and new-controller-
//       add coincides with an SBA pass.
//
// All three shapes manifest as the same observable: an InstanceID is in
// `MintedInstanceIDs`, not in `CeasedInstanceIDs`, but its *Card is in no
// zone the census walks. PR #773's diagnostic enrichment surfaces the
// card name in the violation message.
//
// CR §704.5d models tokens as ceasing to exist when in a zone other than
// the battlefield. The sweep extends the principle: when a Card's only
// reference disappears — regardless of provenance — the engine formally
// retires its InstanceID rather than leak it. The audit event captures
// provenance + card name so future bug-hunting can replay the leak shape
// (Phase F successor work).
//
// Cessation is one-way: once an ID is in `CeasedInstanceIDs`, it stays
// out of the (Minted - Ceased) expected set forever. If a re-mint path
// later re-uses the same *Card pointer with a fresh ID (the
// EnsureTokenInstanceID safety net), the new ID is independently tracked.

// SweepOrphanedInstanceIDs is the exported chokepoint. Walks every
// minted-but-not-ceased InstanceID; for any whose *Card is absent from
// the same set of zones `checkZoneConservationByInstanceID` walks, marks
// the ID ceased and emits an `iid_orphan_sweep` audit event. Returns
// the count of IDs swept.
//
// Called once per StateBasedActions invocation, after the main SBA loop
// converges (see sba.go). Safe to call standalone from per_card handlers
// or tests. nil-safe; no-op when InstanceID census is empty (legacy mode).
//
// AB-provenance IDs are skipped — they're ephemeral AbilityInstance IDs
// excluded from the census expectation by design (matches the
// invariants.go:282 filter).
func SweepOrphanedInstanceIDs(gs *GameState) int {
	if gs == nil || len(gs.MintedInstanceIDs) == 0 {
		return 0
	}
	present := collectPresentInstanceIDsForSweep(gs)

	// r63 conservation firespot (live-grinder zone_conservation cluster:
	// Circuit Mender / Mesa Enchantress / Dire Tactics / Migration Path,
	// each "present in zone … but not in (Minted − Ceased) — stale ceased
	// entry"). RESURRECTION pass — the inverse of the orphan cease below.
	//
	// Cessation is ONE-WAY (no delete from CeasedInstanceIDs anywhere), but
	// this sweep ceases any OG card transiently ABSENT from every census
	// zone — and such windows exist beyond the one ResolvingCards covers
	// (the stack→destination limbo). A multi-step operation (shuffle,
	// library search, zone-cast-grant creation, a per_card handler) that an
	// SBA pass interrupts runs this sweep while the card is momentarily in
	// no walked zone; the ID is ceased; when the card REAPPEARS in a real
	// zone its now-permanently-ceased ID trips the fabrication census
	// forever after. The leak surfaces across diverse zones precisely
	// because the over-cease is path-agnostic.
	//
	// Repair generally rather than enumerating every transient window: an
	// OG card that is demonstrably PRESENT in a census-walked zone, whose
	// owner is still in the game, MUST NOT be ceased — CR §110, it exists,
	// and the only legitimate OG cessation is §800.4a owner-left (excluded
	// by the owner gate) or a §720.4 game restart (clears all zones). So a
	// present + ceased + owner-in-game OG id is a contradiction that can
	// only arise from an over-cease; un-cease it. This is provably safe for
	// the conservation invariant (a present card belongs in Minted−Ceased)
	// and cannot mask the §800.4a removal-completeness concern — cards
	// whose owner has left are intentionally left ceased.
	for id := range present {
		if _, ceased := gs.CeasedInstanceIDs[id]; !ceased {
			continue
		}
		if len(id) < 4 || id[2:4] != "OG" {
			continue // only real OG cards; TK/CP/AB keep their lifecycles
		}
		if owner, ok := gs.MintedInstanceIDOwners[id]; ok &&
			owner >= 0 && owner < len(gs.Seats) &&
			gs.Seats[owner] != nil && gs.Seats[owner].LeftGame {
			continue // owner left the game — §800.4a domain, do not mask
		}
		delete(gs.CeasedInstanceIDs, id)
		name := ""
		if gs.MintedInstanceIDNames != nil {
			name = gs.MintedInstanceIDNames[id]
		}
		gs.LogEvent(Event{
			Kind:   "iid_orphan_resurrect",
			Seat:   -1,
			Target: -1,
			Source: name,
			Details: map[string]interface{}{
				"instance_id": id,
				"card_name":   name,
				"rule":        "110_present_card_not_ceased",
				"reason":      "ceased_OG_card_present_in_zone_owner_in_game",
			},
		})
	}

	swept := 0
	for id := range gs.MintedInstanceIDs {
		if _, ceased := gs.CeasedInstanceIDs[id]; ceased {
			continue
		}
		if len(id) < 4 {
			continue
		}
		prov := id[2:4]
		if prov == "AB" {
			continue
		}
		if _, ok := present[id]; ok {
			continue
		}
		name := ""
		if gs.MintedInstanceIDNames != nil {
			name = gs.MintedInstanceIDNames[id]
		}
		MarkInstanceIDCeased(gs, id)
		gs.LogEvent(Event{
			Kind:   "iid_orphan_sweep",
			Seat:   -1,
			Target: -1,
			Source: name,
			Details: map[string]interface{}{
				"instance_id": id,
				"provenance":  prov,
				"card_name":   name,
				"rule":        "704.5d_extended",
				"reason":      "minted_card_absent_from_all_zones",
			},
		})
		swept++
	}
	return swept
}

// collectPresentInstanceIDsForSweep mirrors the zone walk in
// checkZoneConservationByInstanceID (invariants.go:167–267). Kept as a
// dedicated helper so the sweep's "present" definition stays in lockstep
// with the invariant's — any zone the invariant counts as present must
// also count here, else the sweep would over-cease.
//
// The two walks intentionally share semantics:
//   - LeftGame seats are skipped (their cards' IDs are already ceased by
//     HandleSeatElimination per CR §800.4a).
//   - Merged-card lineage on each Permanent is followed (Phase 8 Mutate
//     and Meld absorb other *Cards into the surviving Permanent; the
//     absorbed Cards retain their IDs).
//   - Sideband zones (ForetellExile, Companion, ZoneCastGrants,
//     MadnessExile, PlotExile, MayhemDiscards, ParadigmExile) all count
//     as zone-presence.
//   - Stack items are counted ONLY for spell items (Source==nil, not a
//     triggered/activated ability) — ability items reference their source
//     Card as a log label, not as zone occupancy.
func collectPresentInstanceIDsForSweep(gs *GameState) map[string]struct{} {
	present := map[string]struct{}{}
	addID := func(c *Card) {
		if c == nil || c.InstanceID == "" {
			return
		}
		present[c.InstanceID] = struct{}{}
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		if s.LeftGame {
			continue
		}
		for _, c := range s.Library {
			addID(c)
		}
		for _, c := range s.Hand {
			addID(c)
		}
		for _, c := range s.Graveyard {
			addID(c)
		}
		for _, c := range s.Exile {
			addID(c)
		}
		for _, c := range s.CommandZone {
			addID(c)
		}
		for _, p := range s.Battlefield {
			if p == nil {
				continue
			}
			addID(p.Card)
			for _, mc := range p.MergedCardPtrs {
				addID(mc)
			}
		}
		for _, c := range s.ForetellExile {
			addID(c)
		}
		if s.Companion != nil {
			addID(s.Companion)
		}
	}
	for c := range gs.ZoneCastGrants {
		addID(c)
	}
	for c := range gs.MadnessExile {
		addID(c)
	}
	for c := range gs.PlotExile {
		addID(c)
	}
	for c := range gs.MayhemDiscards {
		addID(c)
	}
	for _, cards := range gs.ParadigmExile {
		for _, c := range cards {
			addID(c)
		}
	}
	for _, item := range gs.Stack {
		if item == nil {
			continue
		}
		if item.Source != nil || item.Kind == "triggered" || item.Kind == "activated" {
			continue
		}
		addID(item.Card)
	}
	// Mid-resolution limbo window (r63, game-76 Biorhythm): a spell popped
	// off the stack by ResolveStackTop is in no conventional zone until
	// its final routing. When the resolution itself triggers a seat
	// elimination or game end, this sweep runs MID-resolution (CheckEnd →
	// HandleSeatElimination) — the in-flight card must count as present
	// or it gets ceased as an orphan and its graveyard routing becomes a
	// permanent fabrication violation. Lockstep with the census walk.
	for _, c := range gs.ResolvingCards {
		addID(c)
	}
	return present
}
