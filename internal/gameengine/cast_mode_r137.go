package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// cast_mode_r137.go — evaluating "if this spell was kicked/bargained/..."
//
// The parser emits a `Modification{Kind: "cast_mode_check", Args: [mode]}`
// for every "if this spell was <cast some way>" clause. Measured on the
// live corpus (32,418 rows, 2026-10-16): **152 cards carry one, and the
// engine had no handler for the node kind at all.**
//
//	kicked 98   from_graveyard 14   bargained 9   five_or_more_mana_spent 9
//	additional_cost_paid 8   from_exile 4   foretold 3   madness_cost_paid 3
//	prowl_cost_paid 2   surge_cost_paid 1   from_hand 1
//
// Every one of the 152 sits as the first arg of an `if_intervening_tail`,
// whose handler (resolve_helpers.go) says plainly that it does not look:
//
//	"Evaluate the body part as residual text; the condition is assumed
//	 true (GreedyHat policy: the card is in the deck, so the controller
//	 expects the condition to hold)."
//
// The state these clauses ask about DOES exist — the StackItem carries it
// in CostMeta and CastZone — it was simply never consulted. This is the
// canonical predicate for consulting it.
//
// r137 wires it into one reader (split second, below). The
// if_intervening_tail path needs it too and CANNOT have it yet: that
// handler receives a *Permanent, and for an instant or sorcery that is a
// synthetic transient with no CostMeta, so the StackItem does not reach
// it. gs.ResolvingCards holds *Card, not *StackItem. Threading the item
// through is a plumbing change and is filed, not smuggled in here.

// castModeSatisfied reports whether `item` was cast in the manner `mode`
// names, and whether that question could be answered at all.
//
// The second return is the important one. A mode we cannot evaluate must
// NOT be reported as false: silently answering "no" would turn an unknown
// into a denial and strip effects off cards that earned them. Callers
// decide what to do with an unknown, and r137's caller declines to grant.
func castModeSatisfied(item *StackItem, mode string) (satisfied, known bool) {
	if item == nil {
		return false, false
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "kicked":
		// keywords_batch.go stamps this on every cast, true or false,
		// so its ABSENCE means the spell took a path that never
		// considered kicker — not that it was cast unkicked.
		return costMetaTruthy(item, "kicked")
	case "bargained":
		return costMetaTruthy(item, "bargained")
	case "madness_cost_paid":
		return costMetaTruthy(item, "madness_cast")
	case "surge_cost_paid":
		return costMetaTruthy(item, "surge_cast")
	case "from_graveyard":
		return item.CastZone == "graveyard", true
	case "from_exile":
		return item.CastZone == "exile", true
	case "from_hand":
		// "" is the documented default for a hand cast (StackItem docs).
		return item.CastZone == "" || item.CastZone == "hand", true
	}
	// five_or_more_mana_spent, additional_cost_paid, foretold and
	// prowl_cost_paid have no tracked state on the StackItem today.
	// Unknown, and deliberately not guessed.
	return false, false
}

// costMetaTruthy reads a CostMeta key that may have been stamped as a
// bool or as a count (multikicker stamps a count), and reports whether
// the key was present at all.
func costMetaTruthy(item *StackItem, key string) (satisfied, known bool) {
	v, ok := item.CostMeta[key]
	if !ok {
		return false, false
	}
	switch t := v.(type) {
	case bool:
		return t, true
	case int:
		return t > 0, true
	case float64:
		return t > 0, true
	}
	return false, false
}

// itemGrantsSplitSecond reports whether a spell's own text gives it split
// second under a cast-mode condition that actually held.
//
// CR §702.61a: "Split second is a static ability that functions only
// while the spell with split second is on the stack. 'Split second' means
// 'As long as this spell is on the stack, players can't cast other spells
// or activate abilities that aren't mana abilities.'"
//
// Molten Disaster is the live instance: "If this spell was kicked, it has
// split second." Its split second is NOT a Keyword node — the parser
// emits
//
//	Static{Modification{Kind: "if_intervening_tail", Args: [
//	    Modification{Kind: "cast_mode_check", Args: ["kicked"]},
//	    "it has split second"]}}
//
// and cardHasSplitSecond only ever walked for *gameast.Keyword. So a
// kicked Molten Disaster did not lock the table, which is the whole point
// of paying the kicker. 4 live decks run it.
//
// An unevaluable condition grants nothing: wrongly locking every opponent
// out of responding is the worse of the two errors, and §702.61a is a
// restriction on everyone else at the table.
func itemGrantsSplitSecond(item *StackItem) bool {
	if item == nil || item.Card == nil || item.Card.AST == nil {
		return false
	}
	for _, ab := range item.Card.AST.Abilities {
		st, ok := ab.(*gameast.Static)
		if !ok || st.Modification == nil {
			continue
		}
		mode, tail, ok := castModeTail(st.Modification)
		if !ok || !strings.Contains(strings.ToLower(tail), "split second") {
			continue
		}
		if satisfied, known := castModeSatisfied(item, mode); known && satisfied {
			return true
		}
	}
	return false
}

// castModeTail destructures an `if_intervening_tail` whose condition is a
// `cast_mode_check`, returning the mode and the tail text.
func castModeTail(m interface{}) (mode, tail string, ok bool) {
	kind, args, ok := modShape(m)
	if !ok || kind != "if_intervening_tail" || len(args) < 2 {
		return "", "", false
	}
	condKind, condArgs, ok := modShape(args[0])
	if !ok || condKind != "cast_mode_check" || len(condArgs) < 1 {
		return "", "", false
	}
	mode, _ = condArgs[0].(string)
	tail, _ = args[1].(string)
	if mode == "" || tail == "" {
		return "", "", false
	}
	return mode, tail, true
}

// modShape reads (kind, args) off whichever of the two modification
// shapes the loader produced. A Static carries *gameast.Modification;
// a nested one inside Args may arrive as either that or the Effect
// flavour, so both are accepted rather than assumed.
func modShape(v interface{}) (kind string, args []interface{}, ok bool) {
	switch t := v.(type) {
	case *gameast.Modification:
		if t == nil {
			return "", nil, false
		}
		return t.ModKind, t.Args, true
	case *gameast.ModificationEffect:
		if t == nil {
			return "", nil, false
		}
		return t.ModKind, t.Args, true
	}
	return "", nil, false
}
