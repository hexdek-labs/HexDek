package gameengine

import "strings"

// keywords_batch4.go — Newer set keywords + remaining triggers.
// Batch 4: Saddle, Offspring, Impending, Spree, Boast, Extort
// (standalone), Tribute, Outlast, Hideaway, Conspire, Devour, Unleash,
// Bloodthirst, Absorb, Fortify, Champion, Prowl.

// ---------------------------------------------------------------------------
// Saddle N — CR §702.171
// ---------------------------------------------------------------------------
//
// Tap any number of untapped creatures you control with total power >= N.
// The Vehicle becomes "saddled" until end of turn.

// ActivateSaddle attempts to saddle the mount by tapping creatures the
// controller controls whose total power meets or exceeds saddlePower.
// Returns true if saddling succeeded.
func ActivateSaddle(gs *GameState, mount *Permanent, saddlePower int) bool {
	if gs == nil || mount == nil || saddlePower <= 0 {
		return false
	}
	seatIdx := mount.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}

	// Gather untapped creatures (excluding the mount itself) sorted by
	// descending power so we greedily tap the fewest creatures possible.
	type candidate struct {
		perm  *Permanent
		power int
	}
	var candidates []candidate
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || p == mount || !p.IsCreature() || p.Tapped {
			continue
		}
		candidates = append(candidates, candidate{perm: p, power: p.Power()})
	}

	// Greedy: pick highest-power creatures first.
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].power > candidates[i].power {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	total := 0
	var tapped []*Permanent
	for _, c := range candidates {
		if total >= saddlePower {
			break
		}
		c.perm.Tapped = true
		total += c.power
		tapped = append(tapped, c.perm)
	}

	if total < saddlePower {
		// Undo taps — can't meet the saddle cost.
		for _, p := range tapped {
			p.Tapped = false
		}
		return false
	}

	if mount.Flags == nil {
		mount.Flags = map[string]int{}
	}
	mount.Flags["saddled"] = 1
	mount.SaddlersThisTurn = append(mount.SaddlersThisTurn, tapped...)

	gs.LogEvent(Event{
		Kind:   "saddle",
		Seat:   seatIdx,
		Source: mount.Card.DisplayName(),
		Amount: saddlePower,
		Details: map[string]interface{}{
			"creatures_tapped": len(tapped),
			"total_power":      total,
			"rule":             "702.171",
		},
	})
	// Sibling of the SaddleMount path — fire the Mount-saddled fan-out
	// for per_card observers. Gated on the Mount subtype inside the
	// helper.
	FireMountSaddledTriggers(gs, mount)
	return true
}

// ---------------------------------------------------------------------------
// Offspring — CR §702.175
// ---------------------------------------------------------------------------
//
// Pay an extra cost when casting. When the creature ETBs, if offspring was
// paid, "create a token that's a copy of it, except it's 1/1" (CR §702.175a).
// The token copies every copiable characteristic (name, mana cost, color,
// types, subtypes, supertypes, abilities — via MintTokenAsCopyOf's DeepCopy)
// and then a copy-modification sets base power/toughness to 1/1.

// CreateOffspringToken mints the §702.175a token copy of parent. It mirrors
// resolveCreateTokenCopy (the canonical create-a-copy path) so token doublers
// apply (FireCreateTokenEvent), the token_created trigger fires, the mint is
// recorded for lineage audits, and each token is a freshly-minted *Card with
// its own InstanceID (no pointer aliasing — the CardIdentity discipline). The
// only divergence from a plain copy is the 1/1 base P/T override.
//
// Callers gate the "offspring cost was paid" condition; this helper assumes it
// was paid and unconditionally mints. Token copies are skipped by callers so a
// minted offspring token never recursively spawns another.
func CreateOffspringToken(gs *GameState, parent *Permanent) {
	if gs == nil || parent == nil || parent.Card == nil {
		return
	}
	controller := parent.Controller
	if controller < 0 || controller >= len(gs.Seats) || gs.Seats[controller] == nil {
		return
	}

	// Token doublers (Doubling Season / Parallel Lives) act on the 1-copy
	// base count, exactly as for any other create-a-copy effect.
	baseCount := 1
	count := 1
	if modified, cancelled := FireCreateTokenEvent(gs, controller, count, parent); cancelled {
		count = 0
	} else if modified > 0 {
		count = modified
	}

	enablerID := currentMintEnablerID(gs)
	mintEvent := TokenMintEvent{
		SourceInstanceID: enablerID,
		SourceName:       sourceName(parent),
		TargetSeat:       controller,
		BaseCount:        baseCount,
		FinalCount:       count,
		AtGameTick:       gs.EffectTimestamp,
		EffectsApplied:   gs.PendingTokenMintChain,
		BaseCharacteristics: CopiableCharacteristics{
			Name:             parent.Card.DisplayName(),
			Types:            append([]string(nil), parent.Card.Types...),
			Colors:           append([]string(nil), parent.Card.Colors...),
			SourceInstanceID: parent.Card.InstanceID,
		},
	}
	gs.PendingTokenMintChain = nil

	for i := 0; i < count; i++ {
		// MintTokenAsCopyOf DeepCopys the source card, clears the inherited
		// InstanceID, and mints a fresh TK ID — so the token is a faithful
		// copy with no *Card aliasing back to the parent.
		card := MintTokenAsCopyOf(gs, parent.Card, controller, enablerID)
		if card == nil {
			continue
		}
		// CR §702.175a "except it's 1/1": the copy-modification sets base P/T.
		card.BasePower = 1
		card.BaseToughness = 1
		if card.InstanceID != "" {
			mintEvent.MintedTokenIDs = append(mintEvent.MintedTokenIDs, card.InstanceID)
		}
		p := &Permanent{
			Card:                   card,
			Controller:             controller,
			SummoningSick:          true,
			Timestamp:              gs.NextTimestamp(),
			Counters:               map[string]int{},
			Flags:                  map[string]int{},
			CopiedTargetInstanceID: mintEvent.BaseCharacteristics.SourceInstanceID,
		}
		gs.Seats[controller].Battlefield = append(gs.Seats[controller].Battlefield, p)
		RegisterReplacementsForPermanent(gs, p)
		// The token's own ETB triggers fire (it's a real entering permanent).
		FirePermanentETBTriggers(gs, p)
	}

	if count > 0 || len(mintEvent.MintedTokenIDs) > 0 {
		RecordTokenMintEvent(gs, mintEvent)
	}

	// token_created for token-matters payoffs (same re-entrancy guard the
	// canonical copy path uses).
	if count > 0 && (gs.Flags == nil || gs.Flags["in_token_trigger"] == 0) {
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["in_token_trigger"] = 1
		FireCardTrigger(gs, "token_created", map[string]interface{}{
			"controller_seat": controller,
			"count":           count,
			"types":           parent.Card.Types,
			"source":          sourceName(parent),
		})
		gs.Flags["in_token_trigger"] = 0
	}

	gs.LogEvent(Event{
		Kind:   "offspring",
		Seat:   controller,
		Source: parent.Card.DisplayName(),
		Amount: count,
		Details: map[string]interface{}{
			"token_p_t": "1/1",
			"rule":      "702.175",
		},
	})
}

// ---------------------------------------------------------------------------
// Impending N — CR §702.176
// ---------------------------------------------------------------------------
//
// Cast for impending cost: enters with N time counters, is not a creature
// while it has time counters. Remove one time counter at upkeep. When the
// last is removed, it becomes a creature.

// ---------------------------------------------------------------------------
// Spree — CR §702.172
// ---------------------------------------------------------------------------
//
// Choose one or more modes when casting. Each mode has an additional cost.
// The spell does only what the chosen modes say.

// ---------------------------------------------------------------------------
// Boast — CR §702.142
// ---------------------------------------------------------------------------
//
// Activate only if this creature attacked this turn, and only once per turn.

// ---------------------------------------------------------------------------
// Extort — CR §702.101 (standalone trigger helper)
// ---------------------------------------------------------------------------
//
// Whenever you cast a spell, you may pay {W/B}. If you do, each opponent
// loses 1 life and you gain that much life. Each extort instance triggers
// separately.
//
// NOTE: The core extort logic is also inline in cast_counts.go. This
// standalone function allows callers (e.g. per-card hooks) to explicitly
// fire extort triggers outside the normal cast path.

// ---------------------------------------------------------------------------
// Tribute N — CR §702.121
// ---------------------------------------------------------------------------
// HasTribute / TributeAmount / ApplyTribute / WasTributeAccepted /
// WasTributeRefused / TributeResolved / TributeOpponent live in
// keywords_tribute.go. The ETB choice is implemented as a two-callback
// flow (controller chooses opponent → opponent decides yes/no) with
// per-card "tribute_resolved" trigger fan-out for the §702.121b
// "if tribute wasn't paid" payoff.

// ---------------------------------------------------------------------------
// Outlast — CR §702.107
// ---------------------------------------------------------------------------
//
// Sorcery-speed, tap: put a +1/+1 counter on this creature.

// ---------------------------------------------------------------------------
// Hideaway N — CR §702.75
// ---------------------------------------------------------------------------
//
// When this permanent ETBs, look at the top N cards of your library, exile
// one face down, and put the rest on the bottom of your library in a
// random order.

// ApplyHideaway exiles one of the top N cards face-down and puts the rest
// on the bottom of the library in random order.
func ApplyHideaway(gs *GameState, perm *Permanent, n int) {
	if gs == nil || perm == nil || n <= 0 {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || len(seat.Library) == 0 {
		return
	}

	// Look at top N.
	lookCount := n
	if lookCount > len(seat.Library) {
		lookCount = len(seat.Library)
	}
	looked := make([]*Card, lookCount)
	copy(looked, seat.Library[:lookCount])
	seat.Library = seat.Library[lookCount:]

	if len(looked) == 0 {
		return
	}

	// Pick the first card to exile face-down (greedy: pick index 0).
	chosen := looked[0]
	rest := looked[1:]

	MoveCard(gs, chosen, seatIdx, "library", "exile", "face-down-exile")
	chosen.FaceDown = true

	// Track the hidden card on the permanent so it can be cast later.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["hideaway"] = 1

	// Put the rest on the bottom in random order.
	if gs.Rng != nil && len(rest) > 1 {
		gs.Rng.Shuffle(len(rest), func(i, j int) {
			rest[i], rest[j] = rest[j], rest[i]
		})
	}
	seat.Library = append(seat.Library, rest...)

	gs.LogEvent(Event{
		Kind:   "hideaway",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Amount: lookCount,
		Details: map[string]interface{}{
			"exiled": chosen.DisplayName(),
			"rule":   "702.75",
		},
	})
}

// ---------------------------------------------------------------------------
// Conspire — CR §702.78
// ---------------------------------------------------------------------------
//
// As you cast this spell, you may tap two untapped creatures you control
// that each share a color with it. If you do, copy the spell.

// ApplyConspire taps two creatures sharing a color with the spell and
// creates a copy of the spell on the stack. Returns true if conspire
// succeeded.
func ApplyConspire(gs *GameState, seatIdx int, item *StackItem) bool {
	if gs == nil || item == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	if item.Card == nil || len(item.Card.Colors) == 0 {
		return false
	}

	spellColors := item.Card.Colors
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}

	// Find two untapped creatures sharing a color with the spell.
	var tapped []*Permanent
	for _, p := range seat.Battlefield {
		if len(tapped) >= 2 {
			break
		}
		if p == nil || !p.IsCreature() || p.Tapped {
			continue
		}
		if p.Card == nil {
			continue
		}
		// Check color overlap.
		shared := false
		for _, sc := range spellColors {
			for _, pc := range p.Card.Colors {
				if sc == pc {
					shared = true
					break
				}
			}
			if shared {
				break
			}
		}
		if shared {
			tapped = append(tapped, p)
		}
	}

	if len(tapped) < 2 {
		return false
	}

	tapped[0].Tapped = true
	tapped[1].Tapped = true

	// Push a copy of the spell. Route through MintSpellCopy so the copy
	// carries a fresh CP-provenance InstanceID with lineage to the source,
	// rather than aliasing the source *Card pointer. Without this,
	// stack.go's §707.10 cease branch would retire the SOURCE's InstanceID
	// when the conspire copy resolves — the source card living in the
	// graveyard / hand / wherever post-cast would then be flagged as
	// fabrication by checkZoneConservation (Phase G sibling-site closure,
	// same shape as Aziza per `docs/zonecons-phase-g-aziza-r60.md`).
	copyCard := MintSpellCopy(gs, item.Card)
	copyItem := &StackItem{
		Card:       copyCard,
		Controller: seatIdx,
		Effect:     item.Effect,
		IsCopy:     true,
		CostMeta:   map[string]interface{}{"conspire_copy": true},
	}
	PushStackItem(gs, copyItem)

	gs.LogEvent(Event{
		Kind:   "conspire",
		Seat:   seatIdx,
		Source: item.Card.DisplayName(),
		Details: map[string]interface{}{
			"tapped_1": tapped[0].Card.DisplayName(),
			"tapped_2": tapped[1].Card.DisplayName(),
			"rule":     "702.78",
		},
	})
	// CR §702.137a / "whenever you copy a spell" — the conspire copy fires
	// the canonical copy-trigger fan-out (magecraft + spell_copied).
	FireSpellCopyTriggers(gs, seatIdx, copyCard, item.Card)
	return true
}

// ---------------------------------------------------------------------------
// Devour N — CR §702.82
// ---------------------------------------------------------------------------
//
// As this creature ETBs, you may sacrifice any number of creatures. This
// creature enters with N * (sacrificed count) +1/+1 counters.

// putDevourCounters places the devour +1/+1 counters on the entering creature.
// CR §702.82c / §122.1g — these counters "enter with" the creature, so the
// would_put_counter doubler chain applies (Doubling Season, Hardened Scales,
// Branching Evolution, Conclave Mentor). Raw AddCounter bypassed it — the r63
// counter-pipeline-sweep meta-pattern. As an enters-with counter (not a "put"),
// the counter_placed payoff trigger is intentionally NOT fired, so this routes
// through FirePutCounterEvent (replacement chain only) rather than
// PutCountersTriggered. Devouring zero creatures → 0 counters (a legal no-op).
func putDevourCounters(gs *GameState, perm *Permanent, counters int) {
	if counters > 0 {
		if modified, cancelled := FirePutCounterEvent(gs, perm, "+1/+1", counters, perm); !cancelled && modified > 0 {
			perm.AddCounter("+1/+1", modified)
		}
	}
	gs.InvalidateCharacteristicsCache()
}

// ApplyDevour sacrifices creatures and places +1/+1 counters on perm.
// n is the devour multiplier (Devour 1, Devour 2, etc.). Per CR §702.82,
// "you MAY sacrifice any number of creatures." The AI heuristic sacrifices
// up to 2 creatures, keeping at least 1 creature alive on the battlefield.
func ApplyDevour(gs *GameState, perm *Permanent, n int) {
	if gs == nil || perm == nil || n <= 0 {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}

	// Collect creatures eligible for sacrifice (everything except perm itself).
	var candidates []*Permanent
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || p == perm || !p.IsCreature() {
			continue
		}
		candidates = append(candidates, p)
	}

	if len(candidates) == 0 {
		gs.LogEvent(Event{
			Kind:   "devour",
			Seat:   seatIdx,
			Source: perm.Card.DisplayName(),
			Amount: 0,
			Details: map[string]interface{}{
				"devour_n":   n,
				"sacrificed": 0,
				"rule":       "702.82",
			},
		})
		return
	}

	// AI heuristic: sacrifice up to 2 creatures, but keep at least 1 alive.
	maxSacrifice := 2
	if maxSacrifice > len(candidates) {
		maxSacrifice = len(candidates)
	}
	// Keep at least 1 creature on the battlefield (besides perm itself).
	if maxSacrifice >= len(candidates) && len(candidates) > 0 {
		maxSacrifice = len(candidates) - 1
	}
	if maxSacrifice < 0 {
		maxSacrifice = 0
	}

	victims := candidates[:maxSacrifice]
	for _, v := range victims {
		SacrificePermanent(gs, v, "devour")
	}

	counters := len(victims) * n
	putDevourCounters(gs, perm, counters)

	gs.LogEvent(Event{
		Kind:   "devour",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Amount: counters,
		Details: map[string]interface{}{
			"devour_n":   n,
			"sacrificed": len(victims),
			"rule":       "702.82",
		},
	})
}

// ApplyDevourTyped is the generalized devour that accepts a material type.
// Famished Worldsire uses "Devour land 3" — sacrifices lands instead of
// creatures. The materialType parameter is "creature", "land", "artifact", etc.
func ApplyDevourTyped(gs *GameState, perm *Permanent, n int, materialType string) {
	if gs == nil || perm == nil || n <= 0 {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	matType := strings.ToLower(materialType)

	var candidates []*Permanent
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || p == perm {
			continue
		}
		if matType == "creature" && !p.IsCreature() {
			continue
		}
		if matType == "land" && !p.IsLand() {
			continue
		}
		if matType == "artifact" && (p.Card == nil || !strings.Contains(strings.ToLower(strings.Join(p.Card.Types, " ")), "artifact")) {
			continue
		}
		candidates = append(candidates, p)
	}

	if len(candidates) == 0 {
		gs.LogEvent(Event{
			Kind: "devour", Seat: seatIdx, Source: perm.Card.DisplayName(),
			Amount: 0,
			Details: map[string]interface{}{
				"devour_n": n, "material": materialType, "sacrificed": 0, "rule": "702.82",
			},
		})
		return
	}

	// For lands, sacrifice more aggressively (lands are replaceable via draws).
	maxSac := len(candidates)
	if matType == "creature" {
		maxSac = 2
		if maxSac > len(candidates) {
			maxSac = len(candidates)
		}
		if maxSac >= len(candidates) && len(candidates) > 0 {
			maxSac = len(candidates) - 1
		}
	} else if matType == "land" {
		// Keep at least 2 lands.
		keep := 2
		if maxSac > len(candidates)-keep {
			maxSac = len(candidates) - keep
		}
	}
	if maxSac < 0 {
		maxSac = 0
	}

	victims := candidates[:maxSac]
	for _, v := range victims {
		SacrificePermanent(gs, v, "devour_"+matType)
	}

	counters := len(victims) * n
	putDevourCounters(gs, perm, counters)

	gs.LogEvent(Event{
		Kind: "devour", Seat: seatIdx, Source: perm.Card.DisplayName(),
		Amount: counters,
		Details: map[string]interface{}{
			"devour_n": n, "material": materialType, "sacrificed": len(victims), "rule": "702.82",
		},
	})
}

// ---------------------------------------------------------------------------
// Unleash — CR §702.98
// ---------------------------------------------------------------------------
//
// You may have this creature enter the battlefield with a +1/+1 counter
// on it. It can't block as long as it has a +1/+1 counter on it.

// ---------------------------------------------------------------------------
// Bloodthirst N — CR §702.54
// ---------------------------------------------------------------------------
//
// If an opponent was dealt damage this turn, this creature enters the
// battlefield with N +1/+1 counters on it.

// ---------------------------------------------------------------------------
// Absorb N — CR §702.64
// ---------------------------------------------------------------------------
//
// If a source would deal damage to this creature, prevent N of that damage.

// ---------------------------------------------------------------------------
// Fortify — CR §702.67
// ---------------------------------------------------------------------------
//
// Attach this Fortification to target land you control. (Like Equip, but
// for lands instead of creatures.)

// ---------------------------------------------------------------------------
// Champion a [type] — CR §702.72
// ---------------------------------------------------------------------------
//
// When this creature ETBs, sacrifice it unless you exile another [type]
// you control. When this creature leaves the battlefield, return the
// exiled card to the battlefield.

// ---------------------------------------------------------------------------
// Prowl — CR §702.76
// ---------------------------------------------------------------------------
//
// You may cast this spell for its prowl cost if a creature that shares a
// creature type with it dealt combat damage to a player this turn.

// Ensure the strings import is used.
var _ = strings.ToLower
