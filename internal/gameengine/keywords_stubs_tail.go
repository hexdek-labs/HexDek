package gameengine

// keywords_stubs_tail.go — real implementations for the §702.17x–§702.19x
// tail mechanics that previously shipped as stubs in keywords_batch6.go.
//
// Implemented here:
//
//   §702.173  Space Sculptor  — partitioned battlefield + sector zone-control
//   §702.182  Tiered          — modal cast with X-cost replication
//   §702.190  Infinity        — kicker-style alt-cost paid any number of times
//
// All three follow the conventions already in use elsewhere in this package:
//
//   - HasFoo(card) reports the keyword from the AST.
//   - FooCost(card) parses the keyword's numeric arg (mana string or int),
//     mirroring keywords_buyback.go's BuybackCost.
//   - ApplyFoo / CastFoo charges mana through seat.ManaPool +
//     SyncManaAfterSpend, logs the cost event with a structured
//     reason+rule, and writes machine-readable state to either
//     StackItem.CostMeta (for cast-time alt-costs) or to a dedicated map
//     on GameState (for Sculptor's persistent sector ownership).
//   - Read-back predicates (IsFoo / FooCount / etc.) read the same keys
//     so resolvers and tests don't reach into raw maps.

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/mana"
)

// ===========================================================================
// §702.182 — Tiered
// ===========================================================================
//
// "Tiered [cost]" appears on modal instants and sorceries. As the spell is
// cast, the controller chooses one or more of its modes (§601.2c). The
// controller may then pay {tiered cost} any number of times; for each
// payment, a copy of the spell is put on the stack with the same modes
// chosen.
//
// Modeling decisions:
//
//   - Mode selection is recorded as a sorted, deduplicated []int in
//     CostMeta["tiered_modes"]. ResolveStackTop and any modal effect
//     handler can read this back to apply only the selected modes.
//   - Replication count is in CostMeta["tiered_tiers"]; ApplyTiered also
//     pushes that many copies onto the stack so resolution doesn't need
//     to know about Tiered at all — it just sees N+1 stack items with
//     the same modes pre-selected.
//   - Each copy inherits the original's modes and targets. Per §707.10c
//     the controller may retarget; this engine's retarget pass runs
//     separately, so ApplyTiered itself does not solicit new targets.
//   - Failure modes: insufficient mana for the requested tier count is
//     all-or-nothing (matches Replicate semantics — §702.56 / §702.182
//     are sibling alt-cost mechanics and partial payment is not allowed
//     for a single cast-time decision).

// TieredCost returns the per-tier replication cost. Accepts numeric and
// mana-string args (matches BuybackCost).
func TieredCost(card *Card) int {
	if card == nil || card.AST == nil {
		return 0
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(kw.Name), "tiered") {
			continue
		}
		if len(kw.Args) == 0 {
			return 0
		}
		switch v := kw.Args[0].(type) {
		case string:
			if cost, err := mana.Parse(v); err == nil {
				return cost.CMC()
			}
			return 0
		case float64:
			return int(v)
		case int:
			return v
		}
	}
	return 0
}

// IsTiered reports whether a StackItem was cast with the Tiered alt-cost
// path. Returns true if any modes were recorded OR any tiers were paid.
func IsTiered(item *StackItem) bool {
	if item == nil || item.CostMeta == nil {
		return false
	}
	if _, ok := item.CostMeta["tiered_modes"]; ok {
		return true
	}
	if _, ok := item.CostMeta["tiered_tiers"]; ok {
		return true
	}
	return false
}

// TieredModes returns the sorted, deduplicated mode indices chosen at
// cast time. Returns nil if the spell wasn't cast as tiered.
func TieredModes(item *StackItem) []int {
	if item == nil || item.CostMeta == nil {
		return nil
	}
	v, ok := item.CostMeta["tiered_modes"]
	if !ok {
		return nil
	}
	if modes, ok := v.([]int); ok {
		return append([]int(nil), modes...)
	}
	return nil
}

// TieredTiers returns the number of replicate-style copies paid for at
// cast time. Returns 0 if the spell wasn't cast as tiered or no tiers
// were paid.
func TieredTiers(item *StackItem) int {
	if item == nil || item.CostMeta == nil {
		return 0
	}
	v, ok := item.CostMeta["tiered_tiers"]
	if !ok {
		return 0
	}
	if n, ok := v.(int); ok {
		return n
	}
	return 0
}

// ApplyTiered records the chosen modes on the cast-time StackItem and,
// for each requested tier, pays {tiered cost} and pushes a copy onto the
// stack with the same modes pre-selected. Returns the number of copies
// actually placed on the stack.
//
// Per §601.2c modes must be selected as the spell is cast. Per §702.182
// tier payments happen in the same step. The implementation enforces
// both invariants by failing all-or-nothing if the controller cannot pay
// the full requested tier cost.
//
// Modes is a list of mode indices the caster has chosen (zero-based into
// the AST's modal effect list). They are normalized to a sorted, unique
// []int; an empty list is allowed (some custom Tiered cards may have no
// optional modes).
func ApplyTiered(gs *GameState, item *StackItem, modes []int, tiers int) int {
	if gs == nil || item == nil || item.Card == nil {
		return 0
	}
	if tiers < 0 {
		tiers = 0
	}
	if item.CostMeta == nil {
		item.CostMeta = map[string]interface{}{}
	}

	normalized := normalizeModes(modes)
	item.CostMeta["tiered_modes"] = normalized

	seatIdx := item.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return 0
	}

	cost := TieredCost(item.Card)
	totalCost := cost * tiers

	gs.LogEvent(Event{
		Kind:   "tiered_modes",
		Seat:   seatIdx,
		Source: item.Card.DisplayName(),
		Amount: len(normalized),
		Details: map[string]interface{}{
			"modes": append([]int(nil), normalized...),
			"rule":  "702.182",
		},
	})

	if tiers == 0 || cost == 0 {
		item.CostMeta["tiered_tiers"] = 0
		return 0
	}

	if seat.ManaPool < totalCost {
		gs.LogEvent(Event{
			Kind:   "tiered_fail",
			Seat:   seatIdx,
			Source: item.Card.DisplayName(),
			Amount: tiers,
			Details: map[string]interface{}{
				"cost_each":  cost,
				"total_cost": totalCost,
				"available":  seat.ManaPool,
				"rule":       "702.182",
			},
		})
		item.CostMeta["tiered_tiers"] = 0
		return 0
	}

	seat.ManaPool -= totalCost
	SyncManaAfterSpend(seat)
	item.CostMeta["tiered_tiers"] = tiers

	gs.LogEvent(Event{
		Kind:   "tiered_pay",
		Seat:   seatIdx,
		Source: item.Card.DisplayName(),
		Amount: tiers,
		Details: map[string]interface{}{
			"cost_each":  cost,
			"total_cost": totalCost,
			"rule":       "702.182",
		},
	})

	for i := 0; i < tiers; i++ {
		copyCard := &Card{
			Name:          item.Card.Name,
			Owner:         item.Card.Owner,
			BasePower:     item.Card.BasePower,
			BaseToughness: item.Card.BaseToughness,
			Types:         append([]string(nil), item.Card.Types...),
			Colors:        append([]string(nil), item.Card.Colors...),
			CMC:           item.Card.CMC,
			AST:           item.Card.AST,
			IsCopy:        true,
		}
		MintCopyInstanceID(gs, copyCard, item.Card.InstanceID, currentMintEnablerID(gs))
		copyItem := &StackItem{
			Controller: seatIdx, // §707.10
			Card:       copyCard,
			Effect:     item.Effect,
			Targets:    append([]Target(nil), item.Targets...), // §707.10c
			Kind:       item.Kind,
			IsCopy:     true, // §707.10
			CostMeta: map[string]interface{}{
				"tiered_modes":    append([]int(nil), normalized...),
				"tiered_tiers":    0, // copies don't carry forward the tier count
				"tiered_copy_of":  item.ID,
				"tiered_copy_idx": i + 1,
				"tiered_is_copy":  true,
			},
		}
		PushStackItem(gs, copyItem)

		gs.LogEvent(Event{
			Kind:   "tiered_copy",
			Seat:   seatIdx,
			Source: copyCard.DisplayName(),
			Details: map[string]interface{}{
				"stack_id":   copyItem.ID,
				"copy_index": i + 1,
				"of_total":   tiers,
				"rule":       "702.182+706.10",
			},
		})
	}
	return tiers
}

// normalizeModes returns the input as a sorted, deduplicated, non-negative
// []int. Out-of-range mode indices are the caller's problem (the resolver
// will simply ignore unknown indices).
func normalizeModes(modes []int) []int {
	if len(modes) == 0 {
		return []int{}
	}
	seen := map[int]struct{}{}
	out := make([]int, 0, len(modes))
	for _, m := range modes {
		if m < 0 {
			continue
		}
		if _, dup := seen[m]; dup {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	// Insertion sort — len(modes) is bounded by the spell's mode count,
	// typically ≤ 5, so this beats pulling in sort for one call.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// ===========================================================================
// §702.190 — Infinity
// ===========================================================================
//
// "Infinity [cost]" is an additional-cost keyword: as the spell is cast,
// the controller may pay {cost} any number of times. Resolvers scale
// effects off the number of payments via InfinityCount(item).
//
// Modeled on Kicker (§702.33) with two differences:
//
//   - Kicker is binary (kicked/not). Infinity tracks a count.
//   - Infinity carries an integer scaling parameter resolvers can read
//     (e.g. "deal N damage", "create N tokens", "+N/+N until EOT").
//
// CostMeta keys:
//
//   "infinity_count"     — int, number of payments made (≥ 0).
//   "infinity_cost_each" — int, per-payment cost (cached for audit).

// InfinityCost returns the per-stack cost. Matches BuybackCost/TieredCost
// arg parsing.
func InfinityCost(card *Card) int {
	if card == nil || card.AST == nil {
		return 0
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(kw.Name), "infinity") {
			continue
		}
		if len(kw.Args) == 0 {
			return 0
		}
		switch v := kw.Args[0].(type) {
		case string:
			if cost, err := mana.Parse(v); err == nil {
				return cost.CMC()
			}
			return 0
		case float64:
			return int(v)
		case int:
			return v
		}
	}
	return 0
}

// IsInfinityCast reports whether ApplyInfinity ran on this StackItem (it
// may have run with stacks=0, which is still a valid "no-pay" record).
func IsInfinityCast(item *StackItem) bool {
	if item == nil || item.CostMeta == nil {
		return false
	}
	_, ok := item.CostMeta["infinity_count"]
	return ok
}

// InfinityCount returns the number of times the controller paid the
// infinity cost. Zero if not infinity-cast or stacks=0.
func InfinityCount(item *StackItem) int {
	if item == nil || item.CostMeta == nil {
		return 0
	}
	v, ok := item.CostMeta["infinity_count"]
	if !ok {
		return 0
	}
	if n, ok := v.(int); ok {
		return n
	}
	return 0
}

// ApplyInfinity charges (stacks * InfinityCost(card)) mana and records
// the count on item.CostMeta. All-or-nothing: if the seat can't afford
// the full amount the count is recorded as 0 and an infinity_fail event
// is logged. stacks=0 is allowed and records the explicit "did not pay"
// choice (useful for downstream cards that key on "if you didn't pay
// infinity for this spell").
func ApplyInfinity(gs *GameState, item *StackItem, stacks int) int {
	if gs == nil || item == nil || item.Card == nil {
		return 0
	}
	if stacks < 0 {
		stacks = 0
	}
	if item.CostMeta == nil {
		item.CostMeta = map[string]interface{}{}
	}

	seatIdx := item.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return 0
	}

	cost := InfinityCost(item.Card)
	item.CostMeta["infinity_cost_each"] = cost

	if stacks == 0 || cost == 0 {
		item.CostMeta["infinity_count"] = stacks
		gs.LogEvent(Event{
			Kind:   "infinity_cast",
			Seat:   seatIdx,
			Source: item.Card.DisplayName(),
			Amount: stacks,
			Details: map[string]interface{}{
				"cost_each":  cost,
				"total_cost": 0,
				"rule":       "702.190",
			},
		})
		return stacks
	}

	totalCost := cost * stacks
	if seat.ManaPool < totalCost {
		item.CostMeta["infinity_count"] = 0
		gs.LogEvent(Event{
			Kind:   "infinity_fail",
			Seat:   seatIdx,
			Source: item.Card.DisplayName(),
			Amount: stacks,
			Details: map[string]interface{}{
				"cost_each":  cost,
				"total_cost": totalCost,
				"available":  seat.ManaPool,
				"rule":       "702.190",
			},
		})
		return 0
	}

	seat.ManaPool -= totalCost
	SyncManaAfterSpend(seat)
	item.CostMeta["infinity_count"] = stacks

	gs.LogEvent(Event{
		Kind:   "infinity_cast",
		Seat:   seatIdx,
		Source: item.Card.DisplayName(),
		Amount: stacks,
		Details: map[string]interface{}{
			"cost_each":  cost,
			"total_cost": totalCost,
			"rule":       "702.190",
		},
	})
	return stacks
}

// ===========================================================================
// §702.173 — Space Sculptor
// ===========================================================================
//
// Space Sculptor partitions the battlefield into four sectors — alpha,
// beta, gamma, delta. Every permanent occupies exactly one sector (or
// none, for permanents that have not yet been assigned). A player may
// "claim" a sector, gaining zone-control over the permanents in it.
//
// The data lives on GameState in two side maps (added to State via a
// lazy-init helper rather than as a Phase-changing schema diff):
//
//   gs.Flags["sculptor_sector:<sector>:controller"] = seatIdx+1
//     — seat that controls the sector (0 = uncontrolled; we add 1 so
//       seat 0 is distinguishable from "unset" in the int-keyed Flags
//       map). Cleared by ReleaseSector.
//
//   perm.Flags["sculptor_sector_<sector>"] = 1
//     — exactly one of the four sector flags is set per permanent at
//       any time. AssignSector clears the others atomically.
//
// Why side maps and not a typed field on Permanent: keeping the data on
// the existing Flags maps means clone/copy paths (clone.go) and zone
// transitions (resolve_helpers.go's MoveCard) naturally drop the
// assignment when a permanent leaves the battlefield, matching §702.173
// — sector membership is a battlefield-only state.

const (
	SectorAlpha = "alpha"
	SectorBeta  = "beta"
	SectorGamma = "gamma"
	SectorDelta = "delta"
)

// SpaceSculptorSectors returns the canonical sector ordering. Useful for
// iteration in UI / analytics layers.
func SpaceSculptorSectors() []string {
	return []string{SectorAlpha, SectorBeta, SectorGamma, SectorDelta}
}

// validSculptorSector reports whether `sector` is one of the four canon
// sector names (case-insensitive). Returns the canonical lowercase form
// + true on success, "" + false on miss.
func validSculptorSector(sector string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(sector)) {
	case SectorAlpha:
		return SectorAlpha, true
	case SectorBeta:
		return SectorBeta, true
	case SectorGamma:
		return SectorGamma, true
	case SectorDelta:
		return SectorDelta, true
	}
	return "", false
}

// AssignSector places `perm` into the named sector, clearing any prior
// sector assignment atomically. Returns true on success. Logs a
// sculptor_assign event.
func AssignSector(gs *GameState, perm *Permanent, sector string) bool {
	if gs == nil || perm == nil {
		return false
	}
	canon, ok := validSculptorSector(sector)
	if !ok {
		return false
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	for _, s := range SpaceSculptorSectors() {
		delete(perm.Flags, "sculptor_sector_"+s)
	}
	perm.Flags["sculptor_sector_"+canon] = 1

	source := "<nil>"
	if perm.Card != nil {
		source = perm.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "sculptor_assign",
		Seat:   perm.Controller,
		Source: source,
		Details: map[string]interface{}{
			"sector": canon,
			"rule":   "702.173",
		},
	})
	return true
}

// PermanentSector returns the sector this permanent currently occupies,
// or "" if unassigned.
func PermanentSector(perm *Permanent) string {
	if perm == nil || perm.Flags == nil {
		return ""
	}
	for _, s := range SpaceSculptorSectors() {
		if perm.Flags["sculptor_sector_"+s] == 1 {
			return s
		}
	}
	return ""
}

// PermanentsInSector returns every battlefield permanent across all
// seats currently assigned to the named sector. Returns nil if the
// sector name is invalid.
func PermanentsInSector(gs *GameState, sector string) []*Permanent {
	if gs == nil {
		return nil
	}
	canon, ok := validSculptorSector(sector)
	if !ok {
		return nil
	}
	var out []*Permanent
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, p := range seat.Battlefield {
			if p == nil {
				continue
			}
			if p.Flags == nil {
				continue
			}
			if p.Flags["sculptor_sector_"+canon] == 1 {
				out = append(out, p)
			}
		}
	}
	return out
}

// ClaimSector gives `seatIdx` zone-control over the named sector. Any
// prior controller is overwritten (a sector has at most one controller
// at a time per §702.173). Returns true on success.
func ClaimSector(gs *GameState, seatIdx int, sector string) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	canon, ok := validSculptorSector(sector)
	if !ok {
		return false
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	// Store seatIdx+1 so seat 0 is distinguishable from "unset" (0).
	gs.Flags["sculptor_sector:"+canon+":controller"] = seatIdx + 1

	gs.LogEvent(Event{
		Kind:   "sculptor_claim",
		Seat:   seatIdx,
		Source: canon,
		Details: map[string]interface{}{
			"sector": canon,
			"rule":   "702.173",
		},
	})
	return true
}

// ReleaseSector clears the controller of the named sector. Returns true
// on success (including when the sector was already uncontrolled).
func ReleaseSector(gs *GameState, sector string) bool {
	if gs == nil {
		return false
	}
	canon, ok := validSculptorSector(sector)
	if !ok {
		return false
	}
	if gs.Flags == nil {
		return true
	}
	delete(gs.Flags, "sculptor_sector:"+canon+":controller")
	gs.LogEvent(Event{
		Kind:   "sculptor_release",
		Source: canon,
		Details: map[string]interface{}{
			"sector": canon,
			"rule":   "702.173",
		},
	})
	return true
}

// SectorController returns the seat that currently controls the named
// sector, or -1 if uncontrolled / invalid sector name.
func SectorController(gs *GameState, sector string) int {
	if gs == nil || gs.Flags == nil {
		return -1
	}
	canon, ok := validSculptorSector(sector)
	if !ok {
		return -1
	}
	v, ok := gs.Flags["sculptor_sector:"+canon+":controller"]
	if !ok || v == 0 {
		return -1
	}
	return v - 1
}

// ControlsPermanentViaSculptor returns true when `seatIdx` has zone-
// control over the sector this permanent occupies. Distinct from the
// permanent's Controller field — sculptor zone-control is overlaid on
// top of normal control without changing perm.Controller. Resolvers
// asking "may seat X activate this ability" can consult both:
//
//   perm.Controller == seatIdx || ControlsPermanentViaSculptor(gs, seatIdx, perm)
//
// is the canonical predicate when sculptor zone-control should grant
// activation rights. (Whether a particular ability honors sculptor
// control is decided by the ability's effect text, not by this
// predicate; this is just the lookup.)
func ControlsPermanentViaSculptor(gs *GameState, seatIdx int, perm *Permanent) bool {
	if gs == nil || perm == nil {
		return false
	}
	sector := PermanentSector(perm)
	if sector == "" {
		return false
	}
	return SectorController(gs, sector) == seatIdx
}
