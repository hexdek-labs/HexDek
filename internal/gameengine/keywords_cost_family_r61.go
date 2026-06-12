package gameengine

// keywords_cost_family_r61.go — r61 PR-5 cast-time cost-mechanic helpers.
//
// PR-4 (#1005) wired kicker/multikicker into CastSpell's optional-cost
// stage. PR-5 extends the same pattern to the rest of the cost-mechanic
// family. Most mechanics already had self-contained helpers (CastBuyback,
// CastWithSurge, ApplyReplicate, ApplyConspire, …) but no CastSpell caller.
// This file adds the few small glue helpers CastSpell still needed:
//
//   - HasConspire         — keyword reader (mirrors HasReplicate / HasBuyback).
//   - conspireEligibleCount — counts untapped color-sharing creatures so the
//                             Hat is never asked to pay an impossible cost.
//   - ApplyCasualty       — pays the casualty additional cost (sacrifice a
//                           creature of power ≥ N) AND copies the spell
//                           (CR §702.153). PayCasualty alone only sacrifices;
//                           the "copy this spell" half lives here.
//   - ApplyBargain        — pays the bargain additional cost (sacrifice an
//                           artifact/enchantment/token) and stamps the
//                           CostMeta["bargained"] flag the per-card OnResolve
//                           consumers (per_card/bargain_consumers_r60.go) read.
//
// Both ApplyCasualty and ApplyBargain route their spell copy through the
// canonical MintSpellCopy + PushStackItem chokepoint — same shape as
// ApplyConspire — so the §707.10 cease branch retires the COPY's InstanceID
// rather than aliasing the source *Card (the Aziza/Phase-G zone-leak class).

// HasConspire returns true if the card has the conspire keyword.
func HasConspire(card *Card) bool {
	return cardHasKeywordByName(card, "conspire")
}

// conspireEligibleCount counts the untapped creatures `seatIdx` controls
// that share a color with `card` — the conspire additional cost taps two of
// them (CR §702.78a). Returns the count (capped at 2 for the caller's gate).
func conspireEligibleCount(gs *GameState, seatIdx int, card *Card) int {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || len(card.Colors) == 0 {
		return 0
	}
	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() || p.Tapped {
			continue
		}
		for _, sc := range card.Colors {
			shared := false
			for _, pc := range p.Card.Colors {
				if sc == pc {
					shared = true
					break
				}
			}
			if shared {
				count++
				break
			}
		}
		if count >= 2 {
			break
		}
	}
	return count
}

// ApplyCasualty pays the casualty additional cost — sacrifice a creature of
// power ≥ minPower — and, if paid, copies the spell on the stack above the
// original (CR §702.153a/b: "When you do, copy this spell"). Returns true if
// the cost was paid and a copy was made. The sacrifice routes through
// PayCasualty (cheapest-CMC sufficient-power greedy selection + canonical
// SacrificePermanent), the copy through MintSpellCopy + PushStackItem.
func ApplyCasualty(gs *GameState, seatIdx int, item *StackItem, minPower int) bool {
	if gs == nil || item == nil || item.Card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	if !PayCasualty(gs, seatIdx, minPower) {
		return false
	}
	copyCard := MintSpellCopy(gs, item.Card)
	copyItem := &StackItem{
		Card:       copyCard,
		Controller: seatIdx,
		Effect:     item.Effect,
		Targets:    append([]Target(nil), item.Targets...),
		Kind:       item.Kind,
		IsCopy:     true,
		CostMeta:   map[string]interface{}{"casualty_copy": true},
	}
	PushStackItem(gs, copyItem)
	gs.LogEvent(Event{
		Kind:   "casualty_copy",
		Seat:   seatIdx,
		Source: item.Card.DisplayName(),
		Details: map[string]interface{}{
			"stack_id": copyItem.ID,
			"rule":     "702.153b",
		},
	})
	return true
}

// ApplyBargain pays the bargain additional cost — sacrifice an artifact,
// enchantment, or token (CR §702.176a) — using the cheapest-CMC greedy
// candidate, and stamps item.CostMeta["bargained"]=true so the per-card
// OnResolve consumers branch on the spell's "if this spell was bargained"
// clause. Returns true if the cost was paid. Bargain does NOT copy the spell
// (unlike conspire/casualty) — it powers up a single resolution.
func ApplyBargain(gs *GameState, seatIdx int, item *StackItem) bool {
	if gs == nil || item == nil || item.Card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	victim := findBargainCandidate(gs, seat)
	if victim == nil {
		return false
	}
	name := ""
	if victim.Card != nil {
		name = victim.Card.DisplayName()
	}
	if !sacrificePermanentImpl(gs, victim, nil, "bargain") {
		return false
	}
	if item.CostMeta == nil {
		item.CostMeta = map[string]interface{}{}
	}
	item.CostMeta["bargained"] = true
	gs.LogEvent(Event{
		Kind:   "bargain_paid",
		Seat:   seatIdx,
		Source: item.Card.DisplayName(),
		Details: map[string]interface{}{
			"sacrificed": name,
			"rule":       "702.176",
		},
	})
	FireCardTrigger(gs, "bargain_paid", map[string]interface{}{
		"seat":       seatIdx,
		"card":       item.Card,
		"sacrificed": name,
	})
	return true
}
