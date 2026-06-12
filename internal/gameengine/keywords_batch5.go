package gameengine

// Batch 5 — Remaining keyword actions (CR §701) + misc keyword abilities (CR §702).
//
// KEYWORD ACTIONS (§701):
//   - Fateseal N        — CR §701.29
//   - Clash             — CR §701.30
//   - Manifest          — CR §701.40
//   - Support N         — CR §701.41
//   - Meld              — CR §701.42
//   - Learn             — CR §701.48
//   - Collect Evidence  — CR §701.59
//   - Forage            — CR §701.61
//   - Manifest Dread    — CR §701.62
//   - Endure            — CR §701.63
//
// KEYWORD ABILITIES (§702):
//   - Forecast          — CR §702.57
//   - Transmute         — CR §702.53
//   - Dredge            — CR §702.52 (already exists; skipped)
//   - Rebound           — CR §702.88
//   - Fuse              — CR §702.102
//   - Aftermath         — CR §702.127
//   - Awaken            — CR §702.113
//   - Escalate          — CR §702.115
//   - More Than Meets the Eye — CR §702.162
//   - Living Metal      — CR §702.161

// ---------------------------------------------------------------------------
// §701.29 — Fateseal
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// §701.30 — Clash
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// §701.40 — Manifest
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// §701.41 — Support
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// §701.42 — Meld
// ---------------------------------------------------------------------------

// Meld combines two permanents into a single melded permanent. The two
// components are exiled and a new combined permanent enters the battlefield.
// Returns the melded permanent, or nil on failure.
func Meld(gs *GameState, perm1, perm2 *Permanent) *Permanent {
	if gs == nil || perm1 == nil || perm2 == nil {
		return nil
	}
	if perm1.Card == nil || perm2.Card == nil {
		return nil
	}

	seatIdx := perm1.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil
	}

	name1 := perm1.Card.DisplayName()
	name2 := perm2.Card.DisplayName()

	// Remove both permanents from battlefield.
	removePermanentFromBattlefield(gs, perm1)
	removePermanentFromBattlefield(gs, perm2)

	// Exile both cards (already removed from battlefield above).
	if perm1.Card != nil {
		FireZoneChange(gs, perm1, perm1.Card, seatIdx, "battlefield", "exile")
		FireZoneChangeTriggers(gs, perm1, perm1.Card, "battlefield", "exile")
	}
	if perm2.Card != nil {
		FireZoneChange(gs, perm2, perm2.Card, seatIdx, "battlefield", "exile")
		FireZoneChangeTriggers(gs, perm2, perm2.Card, "battlefield", "exile")
	}

	// Create the melded creature. The actual melded card's stats come from
	// the card data; as a simplified default, sum power/toughness.
	meldedPower := 0
	meldedToughness := 0
	if perm1.Card != nil {
		meldedPower += perm1.Card.BasePower
		meldedToughness += perm1.Card.BaseToughness
	}
	if perm2.Card != nil {
		meldedPower += perm2.Card.BasePower
		meldedToughness += perm2.Card.BaseToughness
	}

	meldedName := name1 + " // " + name2
	melded := CreateCreatureToken(gs, seatIdx, meldedName,
		[]string{"creature"}, meldedPower, meldedToughness)
	if melded != nil {
		if melded.Flags == nil {
			melded.Flags = map[string]int{}
		}
		melded.Flags["melded"] = 1
		// Phase 8: stamp MergedCards / MergeKind for InstanceID lineage
		// so the leave-play unmerge walker can route both components
		// individually to exile per CR §712.3.
		id1 := ""
		id2 := ""
		if perm1.Card != nil {
			id1 = perm1.Card.InstanceID
		}
		if perm2.Card != nil {
			id2 = perm2.Card.InstanceID
		}
		RecordMeldMergeWithCards(gs, melded, id1, id2, perm1.Card, perm2.Card)
	}

	gs.LogEvent(Event{
		Kind:   "meld",
		Seat:   seatIdx,
		Source: meldedName,
		Details: map[string]interface{}{
			"component_1": name1,
			"component_2": name2,
			"rule":        "701.42",
		},
	})

	return melded
}

// ---------------------------------------------------------------------------
// §701.48 — Learn
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// §701.59 — Collect Evidence
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// §701.61 — Forage
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// §701.62 — Manifest Dread
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// §701.63 — Endure
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// §702.57 — Forecast
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// §702.53 — Transmute
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// §702.88 — Rebound
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// §702.102 — Fuse
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// §702.127 — Aftermath
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// §702.113 — Awaken
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// §702.115 — Escalate
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// §702.162 — More Than Meets the Eye
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// §702.161 — Living Metal
// ---------------------------------------------------------------------------
