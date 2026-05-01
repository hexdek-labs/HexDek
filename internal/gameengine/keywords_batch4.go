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
	return true
}

// ---------------------------------------------------------------------------
// Offspring — CR §702.175
// ---------------------------------------------------------------------------
//
// Pay an extra cost when casting. When the creature ETBs, if offspring was
// paid, create a 1/1 token copy (same name, types, colors).

// ApplyOffspring creates a 1/1 token copy of perm. Called at ETB when the
// offspring cost was paid during casting.
func ApplyOffspring(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}

	// Build types for the token, preserving creature subtypes.
	var tokenTypes []string
	for _, t := range perm.Card.Types {
		if t != "token" {
			tokenTypes = append(tokenTypes, t)
		}
	}

	tok := CreateCreatureToken(gs, seatIdx, perm.Card.DisplayName()+" (Offspring)",
		tokenTypes, 1, 1)
	if tok != nil && tok.Card != nil {
		tok.Card.Colors = perm.Card.Colors
	}

	gs.LogEvent(Event{
		Kind:   "offspring",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
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

// ApplyImpending sets up a permanent that was cast for its impending cost.
// It enters with N time counters and is not treated as a creature until
// all time counters are removed.
func ApplyImpending(gs *GameState, perm *Permanent, counters int) {
	if gs == nil || perm == nil || counters <= 0 {
		return
	}
	perm.AddCounter("time", counters)
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["impending"] = 1
	perm.Flags["not_creature_while_impending"] = 1

	gs.LogEvent(Event{
		Kind:   "impending",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: counters,
		Details: map[string]interface{}{
			"rule": "702.176",
		},
	})
}

// TickImpending removes one time counter from each impending permanent
// during its controller's upkeep. When the last counter is removed, the
// permanent becomes a creature (the impending flag is cleared).
func TickImpending(gs *GameState) {
	if gs == nil {
		return
	}
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, p := range seat.Battlefield {
			if p == nil || p.Flags == nil || p.Flags["impending"] <= 0 {
				continue
			}
			if p.Counters == nil || p.Counters["time"] <= 0 {
				continue
			}
			p.AddCounter("time", -1)
			remaining := p.Counters["time"]
			if remaining <= 0 {
				delete(p.Flags, "impending")
				delete(p.Flags, "not_creature_while_impending")
				gs.LogEvent(Event{
					Kind:   "impending_complete",
					Seat:   p.Controller,
					Source: p.Card.DisplayName(),
					Details: map[string]interface{}{
						"rule": "702.176",
					},
				})
			} else {
				gs.LogEvent(Event{
					Kind:   "impending_tick",
					Seat:   p.Controller,
					Source: p.Card.DisplayName(),
					Amount: remaining,
					Details: map[string]interface{}{
						"rule": "702.176",
					},
				})
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Spree — CR §702.172
// ---------------------------------------------------------------------------
//
// Choose one or more modes when casting. Each mode has an additional cost.
// The spell does only what the chosen modes say.

// ApplySpree records the chosen modes on a stack item. modesChosen is a
// bitmask (bit 0 = mode 1, bit 1 = mode 2, etc.).
func ApplySpree(gs *GameState, item *StackItem, modesChosen int) {
	if gs == nil || item == nil || modesChosen == 0 {
		return
	}
	if item.CostMeta == nil {
		item.CostMeta = map[string]interface{}{}
	}
	item.CostMeta["spree_modes"] = modesChosen

	name := ""
	if item.Card != nil {
		name = item.Card.DisplayName()
	}

	// Count chosen modes.
	count := 0
	m := modesChosen
	for m > 0 {
		count += m & 1
		m >>= 1
	}

	gs.LogEvent(Event{
		Kind:   "spree",
		Seat:   item.Controller,
		Source: name,
		Amount: count,
		Details: map[string]interface{}{
			"modes_bitmask": modesChosen,
			"rule":          "702.172",
		},
	})
}

// ---------------------------------------------------------------------------
// Boast — CR §702.142
// ---------------------------------------------------------------------------
//
// Activate only if this creature attacked this turn, and only once per turn.

// CanBoast returns true if the permanent can activate its boast ability:
// it must have attacked this turn and not already boasted this turn.
func CanBoast(perm *Permanent) bool {
	if perm == nil || perm.Flags == nil {
		return false
	}
	if perm.Flags["attacked_this_turn"] <= 0 && perm.Flags["attacking"] <= 0 {
		return false
	}
	if perm.Flags["boasted_this_turn"] > 0 {
		return false
	}
	return true
}

// ActivateBoast marks the permanent as having boasted this turn.
// Callers should check CanBoast first. The actual boast effect is
// card-specific and resolved by per-card handlers.
func ActivateBoast(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["boasted_this_turn"] = 1

	gs.LogEvent(Event{
		Kind:   "boast",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.142",
		},
	})
}

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

// FireExtortTriggers fires extort triggers for all permanents controlled
// by casterSeat that have the extort keyword. Each instance is paid
// separately (1 generic mana as MVP proxy for {W/B}).
func FireExtortTriggers(gs *GameState, casterSeat int) {
	if gs == nil || casterSeat < 0 || casterSeat >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[casterSeat]
	if seat == nil {
		return
	}

	for _, perm := range seat.Battlefield {
		if perm == nil || perm.Controller != casterSeat || !perm.HasKeyword("extort") {
			continue
		}
		if seat.ManaPool < 1 {
			break
		}
		seat.ManaPool -= 1
		SyncManaAfterSpend(seat)

		opps := gs.Opponents(casterSeat)
		totalDrained := 0
		for _, oppIdx := range opps {
			opp := gs.Seats[oppIdx]
			if opp == nil {
				continue
			}
			opp.Life -= 1
			totalDrained++
			gs.LogEvent(Event{
				Kind:   "life_change",
				Seat:   oppIdx,
				Amount: -1,
				Source: perm.Card.DisplayName(),
				Details: map[string]interface{}{
					"reason": "extort",
				},
			})
		}
		if totalDrained > 0 {
			GainLife(gs, casterSeat, totalDrained, perm.Card.DisplayName())
			gs.LogEvent(Event{
				Kind:   "life_change",
				Seat:   casterSeat,
				Amount: totalDrained,
				Source: perm.Card.DisplayName(),
				Details: map[string]interface{}{
					"reason": "extort",
				},
			})
		}
		gs.LogEvent(Event{
			Kind:   "extort_trigger",
			Seat:   casterSeat,
			Source: perm.Card.DisplayName(),
			Amount: totalDrained,
			Details: map[string]interface{}{
				"drained":  totalDrained,
				"mana_paid": 1,
				"rule":     "702.101",
			},
		})
	}
}

// ---------------------------------------------------------------------------
// Tribute N — CR §702.109
// ---------------------------------------------------------------------------
//
// As this creature ETBs, an opponent may put N +1/+1 counters on it.
// If they don't, a triggered ability fires (card-specific).

// ApplyTribute offers the defending player a choice. In the engine's greedy
// AI, the opponent always refuses tribute (maximizing the controller's
// punishment trigger). Returns true if tribute was paid (counters placed),
// false if refused.
func ApplyTribute(gs *GameState, perm *Permanent, n int) bool {
	if gs == nil || perm == nil || n <= 0 {
		return false
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}

	// Choose an opponent. In a multiplayer game, pick the first active one.
	opps := gs.Opponents(seatIdx)
	if len(opps) == 0 {
		return false
	}

	// AI heuristic: opponent accepts tribute (pays the counters) if N <= 2,
	// reasoning that a small stat boost is less dangerous than most tribute
	// punishment triggers. For N > 2 the opponent refuses, letting the
	// punishment fire rather than giving a large power/toughness boost.
	tributePaid := n <= 2

	if tributePaid {
		perm.AddCounter("+1/+1", n)
		gs.InvalidateCharacteristicsCache()
	}

	gs.LogEvent(Event{
		Kind:   "tribute",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Amount: n,
		Details: map[string]interface{}{
			"paid":     tributePaid,
			"opponent": opps[0],
			"rule":     "702.109",
		},
	})
	return tributePaid
}

// ---------------------------------------------------------------------------
// Outlast — CR §702.107
// ---------------------------------------------------------------------------
//
// Sorcery-speed, tap: put a +1/+1 counter on this creature.

// ActivateOutlast taps the creature and places a +1/+1 counter on it.
// Returns true if the activation succeeded (creature was untapped and it's
// the controller's main phase).
func ActivateOutlast(gs *GameState, perm *Permanent) bool {
	if gs == nil || perm == nil {
		return false
	}
	if perm.Tapped {
		return false
	}
	if !perm.IsCreature() {
		return false
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}

	// Sorcery-speed restriction: must be controller's turn during a main phase.
	if gs.Active != seatIdx {
		return false
	}
	if gs.Phase != "main" && gs.Step != "precombat_main" && gs.Step != "postcombat_main" {
		return false
	}

	perm.Tapped = true
	perm.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()

	gs.LogEvent(Event{
		Kind:   "outlast",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.107",
		},
	})
	return true
}

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

	// Push a copy of the spell.
	copyItem := &StackItem{
		Card:       item.Card,
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
	return true
}

// ---------------------------------------------------------------------------
// Devour N — CR §702.82
// ---------------------------------------------------------------------------
//
// As this creature ETBs, you may sacrifice any number of creatures. This
// creature enters with N * (sacrificed count) +1/+1 counters.

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
				"devour_n":  n,
				"sacrificed": 0,
				"rule":      "702.82",
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
	perm.AddCounter("+1/+1", counters)
	gs.InvalidateCharacteristicsCache()

	gs.LogEvent(Event{
		Kind:   "devour",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Amount: counters,
		Details: map[string]interface{}{
			"devour_n":  n,
			"sacrificed": len(victims),
			"rule":      "702.82",
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
	perm.AddCounter("+1/+1", counters)
	gs.InvalidateCharacteristicsCache()

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

// ApplyUnleash applies the unleash choice. The greedy AI always chooses
// to unleash (the +1/+1 counter is typically worth more than blocking).
func ApplyUnleash(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	// Greedy: always unleash.
	perm.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()

	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["unleashed"] = 1

	gs.LogEvent(Event{
		Kind:   "unleash",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.98",
		},
	})
}

// ---------------------------------------------------------------------------
// Bloodthirst N — CR §702.54
// ---------------------------------------------------------------------------
//
// If an opponent was dealt damage this turn, this creature enters the
// battlefield with N +1/+1 counters on it.

// ApplyBloodthirst checks if an opponent of the controller was dealt damage
// this turn and, if so, places N +1/+1 counters on perm.
func ApplyBloodthirst(gs *GameState, perm *Permanent, n int) {
	if gs == nil || perm == nil || n <= 0 {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}

	// Check if any opponent was dealt damage this turn.
	opponentDamaged := false
	opps := gs.Opponents(seatIdx)
	for _, oppIdx := range opps {
		opp := gs.Seats[oppIdx]
		if opp == nil {
			continue
		}
		if opp.Flags != nil && opp.Flags["damage_taken_this_turn"] > 0 {
			opponentDamaged = true
			break
		}
	}

	if !opponentDamaged {
		gs.LogEvent(Event{
			Kind:   "bloodthirst_miss",
			Seat:   seatIdx,
			Source: perm.Card.DisplayName(),
			Amount: n,
			Details: map[string]interface{}{
				"rule": "702.54",
			},
		})
		return
	}

	perm.AddCounter("+1/+1", n)
	gs.InvalidateCharacteristicsCache()

	gs.LogEvent(Event{
		Kind:   "bloodthirst",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Amount: n,
		Details: map[string]interface{}{
			"rule": "702.54",
		},
	})
}

// ---------------------------------------------------------------------------
// Absorb N — CR §702.64
// ---------------------------------------------------------------------------
//
// If a source would deal damage to this creature, prevent N of that damage.

// ApplyAbsorb reduces incoming damage by n (minimum 0). Returns the damage
// amount after absorption.
func ApplyAbsorb(perm *Permanent, damage int, n int) int {
	if perm == nil || damage <= 0 || n <= 0 {
		return damage
	}
	after := damage - n
	if after < 0 {
		after = 0
	}
	return after
}

// ---------------------------------------------------------------------------
// Fortify — CR §702.67
// ---------------------------------------------------------------------------
//
// Attach this Fortification to target land you control. (Like Equip, but
// for lands instead of creatures.)

// ActivateFortify attaches the fortification to a target land.
func ActivateFortify(gs *GameState, perm *Permanent, target *Permanent) {
	if gs == nil || perm == nil || target == nil {
		return
	}
	if !target.IsLand() {
		return
	}
	if perm.Controller != target.Controller {
		return
	}

	// Detach from previous land if any.
	perm.AttachedTo = target

	gs.LogEvent(Event{
		Kind:   "fortify",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"target": target.Card.DisplayName(),
			"rule":   "702.67",
		},
	})
}

// ---------------------------------------------------------------------------
// Champion a [type] — CR §702.72
// ---------------------------------------------------------------------------
//
// When this creature ETBs, sacrifice it unless you exile another [type]
// you control. When this creature leaves the battlefield, return the
// exiled card to the battlefield.

// ApplyChampion exiles a creature of the specified type. If no valid
// creature is found, the champion is sacrificed.
func ApplyChampion(gs *GameState, perm *Permanent, creatureType string) {
	if gs == nil || perm == nil {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}

	typeLower := strings.ToLower(creatureType)

	// Find a valid creature to exile (not perm itself).
	var victim *Permanent
	var victimIdx int
	for i, p := range seat.Battlefield {
		if p == nil || p == perm {
			continue
		}
		if p.Card == nil {
			continue
		}
		// Check if the permanent has the required type.
		matched := false
		for _, t := range p.Card.Types {
			if strings.ToLower(t) == typeLower {
				matched = true
				break
			}
		}
		if matched {
			victim = p
			victimIdx = i
			break
		}
	}

	if victim == nil {
		// No valid target — per CR §702.72, the "When this enters" exile
		// trigger simply doesn't resolve if there is no valid target.
		// The champion stays on the battlefield without exiling anything.
		gs.LogEvent(Event{
			Kind:   "champion_no_target",
			Seat:   seatIdx,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"required_type": creatureType,
				"rule":          "702.72",
			},
		})
		return
	}

	// Exile the victim.
	seat.Battlefield = append(seat.Battlefield[:victimIdx], seat.Battlefield[victimIdx+1:]...)
	exiledCard := victim.Card
	exiledCard.FaceDown = false
	MoveCard(gs, exiledCard, seatIdx, "battlefield", "exile", "champion")

	// Track the exiled card on the champion for the LTB trigger.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["champion_active"] = 1

	// Register a delayed trigger: when perm leaves the battlefield,
	// return the exiled card.
	championPerm := perm
	exiled := exiledCard
	gs.RegisterDelayedTrigger(&DelayedTrigger{
		TriggerAt:      "on_event",
		ControllerSeat: seatIdx,
		SourceCardName: perm.Card.DisplayName() + " (champion LTB)",
		OneShot:        true,
		ConditionFn: func(gs *GameState, ev *Event) bool {
			if ev == nil {
				return false
			}
			return (ev.Kind == "dies" || ev.Kind == "zone_change") &&
				ev.Source == championPerm.Card.DisplayName()
		},
		EffectFn: func(gs *GameState) {
			if seatIdx < 0 || seatIdx >= len(gs.Seats) {
				return
			}
			s := gs.Seats[seatIdx]
			if s == nil {
				return
			}
			// Find the exiled card and return it.
			for _, c := range s.Exile {
				if c == exiled {
					removeCardFromZone(gs, seatIdx, exiled, "exile")
					returned := &Permanent{
						Card:       exiled,
						Controller: seatIdx,
						Owner:      exiled.Owner,
						Timestamp:  gs.NextTimestamp(),
						Counters:   map[string]int{},
						Flags:      map[string]int{},
					}
					s.Battlefield = append(s.Battlefield, returned)
					RegisterReplacementsForPermanent(gs, returned)
					FirePermanentETBTriggers(gs, returned)
					gs.InvalidateCharacteristicsCache()
					gs.LogEvent(Event{
						Kind:   "champion_return",
						Seat:   seatIdx,
						Source: exiled.DisplayName(),
						Details: map[string]interface{}{
							"rule": "702.72",
						},
					})
					break
				}
			}
		},
	})

	gs.LogEvent(Event{
		Kind:   "champion",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"exiled":        victim.Card.DisplayName(),
			"required_type": creatureType,
			"rule":          "702.72",
		},
	})
}

// ---------------------------------------------------------------------------
// Prowl — CR §702.76
// ---------------------------------------------------------------------------
//
// You may cast this spell for its prowl cost if a creature that shares a
// creature type with it dealt combat damage to a player this turn.

// CanCastForProwl returns true if a creature sharing a type with card
// dealt combat damage to a player this turn, enabling the prowl alt cost.
func CanCastForProwl(gs *GameState, seatIdx int, card *Card) bool {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}

	// Extract creature subtypes from the card.
	cardTypes := card.Types
	if len(cardTypes) == 0 {
		return false
	}

	// Check if any creature the player controls has dealt combat damage
	// this turn and shares a creature type.
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() || p.Card == nil {
			continue
		}
		if p.Flags == nil || p.Flags["dealt_combat_damage_this_turn"] <= 0 {
			continue
		}
		// Check for shared creature type.
		for _, pt := range p.Card.Types {
			ptLower := strings.ToLower(pt)
			// Skip non-subtype entries.
			if ptLower == "creature" || ptLower == "token" || ptLower == "legendary" {
				continue
			}
			for _, ct := range cardTypes {
				if strings.ToLower(ct) == ptLower {
					return true
				}
			}
		}
	}

	return false
}

// Ensure the strings import is used.
var _ = strings.ToLower
