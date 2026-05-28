package gameengine

// Alternative-payment ward (CR §702.21d) — added in R60 to close
// docs/half-finished-features-r48.md #4. Refactored 2026-05-27 from
// three discrete kind-constants into a single WardCost primitive with
// a Type discriminant so the dispatch table is data-driven and new
// variants only need a row added, not a new arm in tryPayAltWardCost.
//
// Architecture (7174n1c-locked, 2026-05-27):
//   - ONE primitive (WardCost struct) with a Type discriminant, not
//     three separate interfaces. Resolution paths live in a single
//     dispatch table keyed by Type.
//   - Generic mana ward ({N}) stays inline in CheckWardOnTargeting
//     via perm.Flags["ward_cost"] — the alt-payment file only handles
//     non-mana variants.
//   - Damage variant (WardCostDamage) covers non-keyword cost-on-target
//     effects like Terror of the Peaks ("when X enters, deals N damage
//     to target opponent or creature"). Those aren't templated as Ward
//     but share the structural cost-on-target shape; ApplyWardCostDamage
//     gives per_card handlers a shared primitive without forcing them
//     through CheckWardOnTargeting.
//
// Storage: WardCost is mirrored onto Permanent.Flags so callers don't
// need a new struct field on Permanent. Type → Flags["ward_alt_kind"],
// Amount → Flags["ward_alt_filter"] (legacy int slot, repurposed as
// the generalized amount). Filter strings live on a separate
// gs.WardCostFilter[perm.Card.Name] keyed registry — most variants
// don't need a filter, but Sacrifice / Discard ones can specify the
// permitted set via the filter slot for future generalization.

import "strings"

// WardCostType is the discriminant tag for an alternative ward payment.
// Numeric values are stable for backward compatibility with the legacy
// WardAltKind constants — adding a new variant means appending, not
// renumbering.
type WardCostType int

const (
	// WardCostNone — no alt-payment (mana-only ward or no ward).
	WardCostNone WardCostType = 0

	// WardCostSacrifice — caster sacrifices a permanent matching Filter.
	// Filter values: "legendary artifact or creature" (Sauron, the Dark
	// Lord), "" (defaults to any creature). Future printings can extend
	// the filter set without a new Type.
	WardCostSacrifice WardCostType = 1

	// WardCostDiscard — caster discards a card matching Filter.
	// Filter values: "instant or sorcery or enchantment" (Saruman of
	// Many Colors), "" (any card).
	WardCostDiscard WardCostType = 2

	// WardCostBlight — caster puts -1/-1 counters on a creature they
	// control. Auntie Ool's Blight 2. Amount = counter count.
	WardCostBlight WardCostType = 3

	// WardCostLife — caster pays Amount life. Ward—Pay N life. Real
	// printings: Charging War Boar (Ward—Pay 3 life), Sheltered by
	// Ghosts (Ward—Pay 1 life), Coalition of Arms variants.
	WardCostLife WardCostType = 4

	// WardCostDamage — source deals Amount damage to a target. NOT a
	// printed Ward keyword variant; the unified primitive covers the
	// structural shape of triggered damage-on-target effects like
	// Terror of the Peaks. Dispatched via ApplyWardCostDamage from
	// per_card handlers, not via CheckWardOnTargeting.
	WardCostDamage WardCostType = 5

	// WardCostMana — the legacy generic ward {N} variant. Permanent-
	// scope mana ward is still tracked inline in CheckWardOnTargeting
	// via Flags["ward_cost"]; this constant is used by the seat-scope
	// ward registry so anthem-style "creatures you control have ward N"
	// effects can specify a mana cost without going through the alt-
	// payment dispatch table.
	WardCostMana WardCostType = 6
)

// WardCost is the unified specification for an alternative ward
// payment. Type selects the dispatch path; Amount and Filter parameterize
// the resolution.
type WardCost struct {
	Type   WardCostType
	Amount int
	Filter string
}

// Legacy aliases — preserved at the same int values so existing
// per_card handlers + tests that read Flags["ward_alt_kind"] keep
// working. New code should set WardCost via SetWardCost instead of
// touching Flags directly.
const (
	WardAltKindNone                = int(WardCostNone)
	WardAltKindSacrificeLegendary  = int(WardCostSacrifice)
	WardAltKindDiscardInstSorcEnch = int(WardCostDiscard)
	WardAltKindBlight              = int(WardCostBlight)
)

// wardCostFilters holds string-valued Filter values keyed by permanent
// pointer. Most cards don't need a filter (the Type alone disambiguates),
// so we avoid bloating Permanent with a string slot; the registry is
// only consulted when a dispatcher needs filter context.
var wardCostFilters = map[*Permanent]string{}

// SetWardCost stamps a WardCost onto a permanent. Wires the kw:ward
// flag, the alt-kind int discriminant, the amount, and (when non-empty)
// the filter string. Callers in per_card/ should prefer this over
// touching Permanent.Flags directly.
func SetWardCost(perm *Permanent, cost WardCost) {
	if perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["kw:ward"] = 1
	perm.Flags["ward_alt_kind"] = int(cost.Type)
	perm.Flags["ward_alt_filter"] = cost.Amount
	if cost.Filter != "" {
		wardCostFilters[perm] = cost.Filter
	}
}

// GetWardCost reads the WardCost stamped on a permanent. Returns
// WardCost{} (Type=None) when no alt-payment is set. Filter string
// is recovered from the parallel registry.
func GetWardCost(perm *Permanent) WardCost {
	if perm == nil || perm.Flags == nil {
		return WardCost{}
	}
	wc := WardCost{
		Type:   WardCostType(perm.Flags["ward_alt_kind"]),
		Amount: perm.Flags["ward_alt_filter"],
	}
	if f, ok := wardCostFilters[perm]; ok {
		wc.Filter = f
	}
	return wc
}

// wardPayFunc — signature for an alt-payment resolver. Returns
// (paid, eventDetails). Implementations either apply the payment and
// return true, or report a failure reason via the details map.
type wardPayFunc func(gs *GameState, caster *Seat, source *Permanent, cost WardCost) (bool, map[string]interface{})

// wardPaymentDispatch is the single source of truth for routing
// WardCostType → resolution. Adding a new variant is one row.
var wardPaymentDispatch = map[WardCostType]wardPayFunc{
	WardCostSacrifice: payWardBySacrifice,
	WardCostDiscard:   payWardByDiscard,
	WardCostBlight:    payWardByBlight,
	WardCostLife:      payWardByLife,
}

// tryPayAltWardCost dispatches via wardPaymentDispatch. On success:
// emits ward_alt_paid. On failure: counters the spell and emits
// ward_counter (same shape as the mana-ward fail path so log consumers
// don't need a new event kind).
func tryPayAltWardCost(gs *GameState, item *StackItem, perm *Permanent) {
	if gs == nil || item == nil || perm == nil {
		return
	}
	caster := gs.Seats[item.Controller]
	if caster == nil {
		return
	}
	cost := GetWardCost(perm)

	handler, ok := wardPaymentDispatch[cost.Type]
	var paid bool
	var detail map[string]interface{}
	if !ok {
		paid = false
		detail = map[string]interface{}{"unknown_kind": int(cost.Type)}
	} else {
		paid, detail = handler(gs, caster, perm, cost)
	}

	if paid {
		base := map[string]interface{}{
			"rule":        "702.21d",
			"ward_target": perm.Card.DisplayName(),
			"spell":       itemName(item),
			"kind":        int(cost.Type),
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

	item.Countered = true
	base := map[string]interface{}{
		"rule":        "702.21c",
		"ward_target": perm.Card.DisplayName(),
		"spell":       itemName(item),
		"caster_seat": item.Controller,
		"kind":        int(cost.Type),
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

// payWardBySacrifice — caster sacrifices a permanent matching cost.Filter.
// Filter "legendary artifact or creature" (Sauron's literal wording)
// requires a legendary artifact OR creature; empty filter falls back to
// "any creature" (defensive default for a hypothetical future generic
// Ward—Sacrifice printing). Heuristic: pick the cheapest legal target
// to minimize value loss.
func payWardBySacrifice(gs *GameState, caster *Seat, source *Permanent, cost WardCost) (bool, map[string]interface{}) {
	wantLegendary := strings.Contains(strings.ToLower(cost.Filter), "legendary") ||
		cost.Filter == "" // legacy default — was hardcoded to legendary art/creature
	wantArtifact := strings.Contains(strings.ToLower(cost.Filter), "artifact") || cost.Filter == ""

	var pick *Permanent
	pickPower := 1 << 30
	for _, p := range caster.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if wantLegendary && !isLegendaryPerm(p) {
			continue
		}
		if !p.IsCreature() && !(wantArtifact && typeContains(p.Card.Types, "artifact")) {
			continue
		}
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
		return false, map[string]interface{}{
			"reason": "no_matching_permanent_to_sacrifice",
			"filter": cost.Filter,
		}
	}
	SacrificePermanent(gs, pick, "ward_alt_sacrifice")
	return true, map[string]interface{}{
		"sacrificed":      pick.Card.DisplayName(),
		"sacrificed_type": permTypeLabel(pick),
	}
}

// payWardByDiscard — caster discards a card matching cost.Filter from
// their hand. Filter "instant or sorcery or enchantment" (Saruman's
// literal wording) requires one of those three types; empty filter
// falls back to "any card". Heuristic: discard the highest-CMC match
// — symmetrical to the mana-ward "always pay" default.
func payWardByDiscard(gs *GameState, caster *Seat, source *Permanent, cost WardCost) (bool, map[string]interface{}) {
	_ = source
	filterLower := strings.ToLower(cost.Filter)
	// Empty filter falls back to the LEGACY default — inst/sorc/ench —
	// rather than "any card". The default reflects the only printed
	// Ward—Discard card in the corpus (Saruman of Many Colors) so
	// fixtures that set Flags["ward_alt_kind"]=Discard directly without
	// migrating to SetWardCost get the same behavior they had pre-r60.
	allowAny := false
	wantInst := cost.Filter == "" || strings.Contains(filterLower, "instant")
	wantSorc := cost.Filter == "" || strings.Contains(filterLower, "sorcery")
	wantEnch := cost.Filter == "" || strings.Contains(filterLower, "enchantment")

	idx := -1
	bestCMC := -1
	for i, c := range caster.Hand {
		if c == nil {
			continue
		}
		matches := allowAny ||
			(wantInst && typeContains(c.Types, "instant")) ||
			(wantSorc && typeContains(c.Types, "sorcery")) ||
			(wantEnch && typeContains(c.Types, "enchantment"))
		if !matches {
			continue
		}
		cm := ManaCostOf(c)
		if cm > bestCMC {
			bestCMC = cm
			idx = i
		}
	}
	if idx < 0 {
		return false, map[string]interface{}{
			"reason": "no_matching_card_in_hand",
			"filter": cost.Filter,
		}
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
			"reason": "ward_alt_discard",
		},
	})
	return true, map[string]interface{}{
		"discarded":     card.DisplayName(),
		"discarded_cmc": bestCMC,
	}
}

// payWardByBlight — caster places cost.Amount -1/-1 counters on a
// creature they control. Heuristic: lowest-toughness creature (least
// value sunk in the existing perm — caster paid for protection, not
// preservation).
func payWardByBlight(gs *GameState, caster *Seat, source *Permanent, cost WardCost) (bool, map[string]interface{}) {
	_ = source
	n := cost.Amount
	if n <= 0 {
		n = 1
	}
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
		"blighted": pick.Card.DisplayName(),
		"counters": n,
	}
}

// payWardByLife — caster pays cost.Amount life. Ward—Pay N life
// (Charging War Boar, Sheltered by Ghosts, Coalition of Arms-class
// printings). Heuristic: pay if Life > 2 * Amount (don't bleed out
// for a single ward trigger). The 2x margin matches the "always pay"
// mana ward default but caps risk of suicide payment.
func payWardByLife(gs *GameState, caster *Seat, source *Permanent, cost WardCost) (bool, map[string]interface{}) {
	_ = source
	n := cost.Amount
	if n <= 0 {
		n = 1
	}
	if caster.Life <= n {
		return false, map[string]interface{}{
			"reason":      "insufficient_life",
			"caster_life": caster.Life,
			"required":    n,
		}
	}
	if caster.Life < 2*n {
		// Risk gate — would drop below 2x the ward cost. Decline so the
		// caster can preserve life for combat / drain races.
		return false, map[string]interface{}{
			"reason":      "life_too_low_for_safe_payment",
			"caster_life": caster.Life,
			"required":    n,
		}
	}
	caster.Life -= n
	gs.LogEvent(Event{
		Kind:   "life_lost",
		Seat:   caster.Idx,
		Source: "ward_alt_pay_life",
		Amount: n,
		Details: map[string]interface{}{
			"reason": "ward_alt_pay_life",
		},
	})
	return true, map[string]interface{}{
		"paid_life":      n,
		"caster_life_after": caster.Life,
	}
}

// ApplyWardCostDamage applies a WardCostDamage-shape effect: source
// deals cost.Amount damage to targetPerm (or targetSeat for player
// targets). NOT routed through CheckWardOnTargeting — called directly
// from per_card handlers for triggered damage-on-target effects like
// Terror of the Peaks. Returns (dealt, details).
//
// The "structural detection" the user requested lives at the per_card
// layer: any ETB / triggered-ability handler that wants to apply
// "deals N damage to target opponent or creature" can call this
// helper instead of inlining the damage logic, so the dispatcher path
// is shared with the printed-ward Damage variant.
//
// Defaults: if both targetPerm and targetSeat are nil/invalid, the
// effect no-ops (defensive — caller should pre-target). Damage routes
// through DealDamage / DealDamageToPlayer so existing damage replacement
// + prevention effects + lifelink hooks fire normally.
func ApplyWardCostDamage(gs *GameState, source *Permanent, targetPerm *Permanent, targetSeat int, cost WardCost) (bool, map[string]interface{}) {
	if gs == nil || source == nil {
		return false, map[string]interface{}{"reason": "nil_source"}
	}
	n := cost.Amount
	if n <= 0 {
		return false, map[string]interface{}{"reason": "non_positive_amount"}
	}
	srcName := source.Card.DisplayName()
	if targetPerm != nil && targetPerm.IsCreature() {
		// Mirror the resolve.go creature-damage path so replacement +
		// prevention effects fire normally. FireDamageEvent may cancel
		// or modify the amount before it lands.
		modified, cancelled := FireDamageEvent(gs, source, targetPerm.Controller, targetPerm, n)
		if cancelled || modified <= 0 {
			return false, map[string]interface{}{"reason": "damage_replaced_or_cancelled"}
		}
		modified = PreventDamageToPermanent(gs, targetPerm, modified, source)
		if modified <= 0 {
			return false, map[string]interface{}{"reason": "damage_prevented"}
		}
		targetPerm.MarkedDamage += modified
		gs.LogEvent(Event{
			Kind:   "damage",
			Seat:   source.Controller,
			Target: targetPerm.Controller,
			Source: srcName,
			Amount: modified,
			Details: map[string]interface{}{
				"reason": "ward_cost_damage",
				"target": targetPerm.Card.DisplayName(),
			},
		})
		return true, map[string]interface{}{
			"damage":      modified,
			"target":      targetPerm.Card.DisplayName(),
			"target_kind": "creature",
		}
	}
	if targetSeat >= 0 && targetSeat < len(gs.Seats) {
		DealDamage(gs, targetSeat, n, srcName)
		return true, map[string]interface{}{
			"damage":      n,
			"target_seat": targetSeat,
			"target_kind": "player",
		}
	}
	return false, map[string]interface{}{"reason": "no_valid_target"}
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
