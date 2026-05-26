package gameengine

// Alternative-payment ward (CR §702.21d) — added in R60 to close the
// docs/half-finished-features-r48.md #4 deferred item. Generic mana
// ward ({N}) is handled inline in CheckWardOnTargeting via
// perm.Flags["ward_cost"]; this file extends the mechanism to the
// three non-mana ward shapes that printed cards use in this corpus:
//
//   1. Ward—Sacrifice a legendary artifact or legendary creature
//      (Sauron, the Dark Lord)
//   2. Ward—Discard an enchantment, instant, or sorcery card
//      (Saruman of Many Colors)
//   3. Ward—Blight N (put N -1/-1 counters on a creature you control)
//      (Auntie Ool, Cursewretch)
//
// Each card's ETB handler stamps two flags on the source permanent:
//
//   - perm.Flags["ward_alt_kind"]   — one of WardAltKindSacrificeLegendary
//                                     / Discard / Blight (constants below)
//   - perm.Flags["ward_alt_filter"] — kind-specific argument (currently
//                                     only Blight uses it: number of
//                                     counters to place)
//
// CheckWardOnTargeting calls tryPayAltWardCost when it sees ward_alt_kind
// set. The function decides whether the caster can pay, applies the
// payment if so, and counters the spell otherwise (CR §702.21c). Returns
// nothing — sets item.Countered when the spell fizzles.
//
// Policy: "pay if affordable" is the default for all three kinds. A
// future per-Hat alt-WardPayer interface can override.

const (
	WardAltKindNone                = 0
	WardAltKindSacrificeLegendary  = 1 // sacrifice a legendary artifact or creature
	WardAltKindDiscardInstSorcEnch = 2 // discard an instant, sorcery, or enchantment card
	WardAltKindBlight              = 3 // put N -1/-1 counters on a creature you control
)

// tryPayAltWardCost dispatches to the alt-payment handler for the kind
// stamped on perm.Flags["ward_alt_kind"]. On success: applies the
// payment (sacrifice / discard / counter-placement) and emits a
// "ward_alt_paid" event. On failure: sets item.Countered = true and
// emits a "ward_counter" event (same shape as the mana-ward fail path
// in CheckWardOnTargeting so log consumers don't need to learn a new
// event kind).
func tryPayAltWardCost(gs *GameState, item *StackItem, perm *Permanent) {
	if gs == nil || item == nil || perm == nil {
		return
	}
	caster := gs.Seats[item.Controller]
	if caster == nil {
		return
	}
	kind := perm.Flags["ward_alt_kind"]
	filter := perm.Flags["ward_alt_filter"]

	var paid bool
	var detail map[string]interface{}
	switch kind {
	case WardAltKindSacrificeLegendary:
		paid, detail = payWardBySacrificeLegendary(gs, caster, perm)
	case WardAltKindDiscardInstSorcEnch:
		paid, detail = payWardByDiscardInstSorcEnch(gs, caster, perm)
	case WardAltKindBlight:
		n := filter
		if n <= 0 {
			n = 1 // defensive default
		}
		paid, detail = payWardByBlight(gs, caster, perm, n)
	default:
		// Unknown kind — treat as unpayable (caller counters the spell).
		paid = false
		detail = map[string]interface{}{"unknown_kind": kind}
	}

	if paid {
		base := map[string]interface{}{
			"rule":        "702.21d",
			"ward_target": perm.Card.DisplayName(),
			"spell":       itemName(item),
			"kind":        kind,
		}
		for k, v := range detail {
			base[k] = v
		}
		gs.LogEvent(Event{
			Kind:    "ward_alt_paid",
			Seat:    item.Controller,
			Source:  perm.Card.DisplayName(),
			Details: base,
		})
		return
	}

	// Couldn't pay — counter the spell.
	item.Countered = true
	base := map[string]interface{}{
		"rule":        "702.21c",
		"ward_target": perm.Card.DisplayName(),
		"spell":       itemName(item),
		"caster_seat": item.Controller,
		"kind":        kind,
	}
	for k, v := range detail {
		base[k] = v
	}
	gs.LogEvent(Event{
		Kind:    "ward_counter",
		Seat:    perm.Controller,
		Source:  perm.Card.DisplayName(),
		Details: base,
	})
}

// payWardBySacrificeLegendary: the caster must sacrifice a legendary
// artifact OR a legendary creature they control. Returns (true, details)
// on a successful sacrifice; (false, reason) when no legal target exists.
// Heuristic: pick the lowest-power creature first, then lowest-CMC
// artifact — "cheapest legal sacrifice" minimizes value loss to the
// caster.
func payWardBySacrificeLegendary(gs *GameState, caster *Seat, source *Permanent) (bool, map[string]interface{}) {
	var pick *Permanent
	pickPower := 1 << 30
	for _, p := range caster.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !isLegendaryPerm(p) {
			continue
		}
		if !p.IsCreature() && !typeContains(p.Card.Types, "artifact") {
			continue
		}
		// Skip the source itself — the caster doesn't control it (ward
		// is opponent-side), so this guard is belt-and-braces.
		if p == source {
			continue
		}
		pow := p.Card.BasePower
		if !p.IsCreature() {
			pow = ManaCostOf(p.Card)
		}
		if pow < pickPower {
			pickPower = pow
			pick = p
		}
	}
	if pick == nil {
		return false, map[string]interface{}{"reason": "no_legendary_artifact_or_creature_to_sacrifice"}
	}
	SacrificePermanent(gs, pick, "ward_alt_sacrifice_legendary")
	return true, map[string]interface{}{
		"sacrificed":      pick.Card.DisplayName(),
		"sacrificed_type": permTypeLabel(pick),
	}
}

// payWardByDiscardInstSorcEnch: the caster must discard a card from
// their hand that is an instant, sorcery, or enchantment. Returns
// (true, details) on success; (false, reason) when the caster has no
// matching card. Heuristic: discard the highest-CMC matching card to
// minimize topdecks they'd actually want to keep — symmetrical to the
// "always pay" mana ward default.
func payWardByDiscardInstSorcEnch(gs *GameState, caster *Seat, source *Permanent) (bool, map[string]interface{}) {
	_ = source
	idx := -1
	bestCMC := -1
	for i, c := range caster.Hand {
		if c == nil {
			continue
		}
		if !typeContains(c.Types, "instant") && !typeContains(c.Types, "sorcery") && !typeContains(c.Types, "enchantment") {
			continue
		}
		cm := ManaCostOf(c)
		if cm > bestCMC {
			bestCMC = cm
			idx = i
		}
	}
	if idx < 0 {
		return false, map[string]interface{}{"reason": "no_inst_sorc_ench_in_hand"}
	}
	card := caster.Hand[idx]
	caster.Hand = append(caster.Hand[:idx], caster.Hand[idx+1:]...)
	caster.Graveyard = append(caster.Graveyard, card)
	gs.LogEvent(Event{
		Kind:   "discard",
		Seat:   caster.Idx,
		Source: "ward_alt_discard",
		Details: map[string]interface{}{
			"card":   card.DisplayName(),
			"reason": "ward_alt_discard_inst_sorc_ench",
		},
	})
	return true, map[string]interface{}{
		"discarded":      card.DisplayName(),
		"discarded_cmc":  bestCMC,
	}
}

// payWardByBlight: the caster puts n -1/-1 counters on a creature
// they control. Returns (true, details) on success; (false, reason)
// when the caster has no creatures. Heuristic: place on the lowest-
// toughness creature (least value sunk in the existing perm, fastest
// to die to the counters — which is fine because the caster paid
// for protection, not preservation).
func payWardByBlight(gs *GameState, caster *Seat, source *Permanent, n int) (bool, map[string]interface{}) {
	_ = source
	var pick *Permanent
	pickT := 1 << 30
	for _, p := range caster.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		tough := p.Card.BaseToughness
		if tough < pickT {
			pickT = tough
			pick = p
		}
	}
	if pick == nil {
		return false, map[string]interface{}{"reason": "no_creature_to_blight"}
	}
	pick.AddCounter("-1/-1", n)
	gs.InvalidateCharacteristicsCache()
	return true, map[string]interface{}{
		"blighted":  pick.Card.DisplayName(),
		"counters":  n,
	}
}

// isLegendaryPerm tests whether a permanent carries the legendary
// supertype on its Card.Types.
func isLegendaryPerm(p *Permanent) bool {
	if p == nil || p.Card == nil {
		return false
	}
	return typeContains(p.Card.Types, "legendary")
}

func typeContains(types []string, want string) bool {
	for _, t := range types {
		if t == want {
			return true
		}
	}
	return false
}

func permTypeLabel(p *Permanent) string {
	if p == nil {
		return ""
	}
	if p.IsCreature() {
		return "creature"
	}
	if p.Card != nil && typeContains(p.Card.Types, "artifact") {
		return "artifact"
	}
	return ""
}
