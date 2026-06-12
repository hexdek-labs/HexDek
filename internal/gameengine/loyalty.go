package gameengine

import (
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// loyalty.go — planeswalker loyalty costs (CR §606).
//
// Thor encodes a loyalty ability's cost as a Cost.Extra string: "+N" for
// plus abilities, "−N" (U+2212 minus sign, as printed) or "-N" (ASCII)
// for minus abilities, and "0" for zero-cost abilities. A corpus survey
// of ast_dataset.jsonl shows ALL 321 planeswalkers use this encoding and
// none parse loyalty into Cost.PayLife — but older datasets followed the
// PayLife convention (the turn-runner comment documents it), so
// ActivateAbility also routes a bare PayLife cost on a planeswalker to
// loyalty rather than life.
//
// Per CR §606.5 the cost is paid by putting (plus) or removing (minus)
// that many loyalty counters; an ability whose cost would remove more
// counters than the permanent has can't be activated. Loyalty payment
// deliberately does NOT route through the counter-doubling replacement
// chokepoint: cost payments are not effects, so Doubling Season-style
// replacements never apply to them (they do apply to starting loyalty,
// which is set at ETB, not here).

// LoyaltyCost returns the loyalty delta of an activated ability's cost
// (+N / -N / 0) and whether the ability carries a loyalty cost at all.
// Variable costs ("−X") are not modeled yet and report no loyalty cost,
// preserving pre-r61 behavior for those abilities.
func LoyaltyCost(a *gameast.Activated) (int, bool) {
	if a == nil {
		return 0, false
	}
	for _, x := range a.Cost.Extra {
		if d, ok := parseLoyaltyDelta(x); ok {
			return d, true
		}
	}
	return 0, false
}

// parseLoyaltyDelta parses a single Cost.Extra string as a loyalty cost.
// Only fully-numeric strings with an explicit +/−/- sign (or the bare
// "0") qualify — Cost.Extra also carries rider tags like
// "hellbent_rider" and channel costs like "discard this card", which
// must not match.
func parseLoyaltyDelta(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "0" {
		return 0, true
	}
	neg := false
	switch {
	case strings.HasPrefix(s, "+"):
		s = s[len("+"):]
	case strings.HasPrefix(s, "−"): // − as printed on cards
		neg = true
		s = s[len("−"):]
	case strings.HasPrefix(s, "-"):
		neg = true
		s = s[len("-"):]
	default:
		return 0, false
	}
	if s == "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, false
	}
	if neg {
		n = -n
	}
	return n, true
}

// payLoyaltyCost applies a loyalty-cost delta to a planeswalker at
// activation time. Returns a CastError (and changes nothing) when a
// minus cost exceeds the available loyalty (CR §606.5 / §601.2g analog:
// a cost that can't be paid makes the activation illegal). On success
// the counter is adjusted immediately — if that brings loyalty to 0 the
// §704.5i SBA destroys the walker on the next state-based check, which
// the activation path runs via DrainStack after resolution.
func payLoyaltyCost(gs *GameState, seatIdx int, perm *Permanent, delta int) error {
	cur := 0
	if perm.Counters != nil {
		cur = perm.Counters["loyalty"]
	}
	if delta < 0 && cur < -delta {
		return &CastError{Reason: "insufficient_loyalty"}
	}
	if perm.Counters == nil {
		perm.Counters = map[string]int{}
	}
	perm.Counters["loyalty"] = cur + delta
	gs.LogEvent(Event{
		Kind:   "loyalty_cost",
		Seat:   seatIdx,
		Amount: delta,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"loyalty_before": cur,
			"loyalty_after":  cur + delta,
			"reason":         "activation_cost",
			"rule":           "606.5",
		},
	})
	return nil
}
