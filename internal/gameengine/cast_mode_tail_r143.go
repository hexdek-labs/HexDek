package gameengine

// Reaching cast-mode state from a resolving spell's own text (r143).
//
// r137 built castModeSatisfied and wired it into one reader (split
// second), and filed the rest plainly:
//
//	"The if_intervening_tail path needs it too and CANNOT have it yet:
//	 that handler receives a *Permanent, and for an instant or sorcery
//	 that is a synthetic transient with no CostMeta, so the StackItem
//	 does not reach it. gs.ResolvingCards holds *Card, not *StackItem."
//
// That is the gap this closes. gs.resolvingItems (stack.go) now carries
// the item alongside the card, and the if_intervening_tail handler asks
// the predicate instead of assuming.
//
// 152 corpus cards carry a cast_mode_check, every one of them as arg 0 of
// an if_intervening_tail: kicked 98, from_graveyard 14, bargained 9,
// five_or_more_mana_spent 9, additional_cost_paid 8, from_exile 4,
// foretold 3, madness_cost_paid 3, prowl_cost_paid 2, surge_cost_paid 1,
// from_hand 1. Only the modes castModeSatisfied can actually evaluate
// change behaviour; the rest stay on the old "assume true" path, because
// AN UNKNOWN IS NOT A FALSE and answering "no" would strip effects off
// cards that earned them.

// currentResolvingSpell returns the StackItem of the SPELL currently
// resolving, if the innermost resolution is a spell and it is the spell
// whose text `src` represents.
//
// Both conditions matter. The innermost item may be a triggered or
// activated ability — its CostMeta describes the ABILITY, not the spell
// that made the permanent, so asking it "was this kicked" would read the
// wrong object's cast-time state. And a tail belonging to some other
// permanent's text must not be answered from this item either. When the
// match fails the caller gets nil and falls back to the old behaviour,
// which is the safe direction.
func currentResolvingSpell(gs *GameState, src *Permanent) *StackItem {
	if gs == nil || src == nil || src.Card == nil {
		return nil
	}
	if len(gs.resolvingItems) == 0 {
		return nil
	}
	item := gs.resolvingItems[len(gs.resolvingItems)-1]
	if item == nil || item.Card == nil {
		return nil
	}
	// A spell has no Source permanent; an ability does.
	if item.Source != nil {
		return nil
	}
	if item.Kind != "" && item.Kind != "spell" {
		return nil
	}
	if item.Card != src.Card {
		return nil
	}
	return item
}

// castModeConditionArg reads a cast_mode_check mode off an
// if_intervening_tail's condition argument. The real corpus shape, read
// off the live dataset, is:
//
//	args[0] = Modification{kind: "cast_mode_check", args: ["kicked"]}
//	args[1] = "it also deals 4 damage to target player or planeswalker"
func castModeConditionArg(arg interface{}) (mode string, ok bool) {
	kind, args, ok := modShape(arg)
	if !ok || kind != "cast_mode_check" || len(args) < 1 {
		return "", false
	}
	mode, _ = args[0].(string)
	if mode == "" {
		return "", false
	}
	return mode, true
}

// interveningTailAllows decides whether an if_intervening_tail's body
// should resolve.
//
// It returns (allow, decided). decided=false means the engine could not
// answer and the caller must keep its previous behaviour — the handler's
// long-standing "the condition is assumed true" policy. Only a mode that
// is both a recognised cast_mode_check AND evaluable against the real
// resolving spell produces decided=true.
func interveningTailAllows(gs *GameState, src *Permanent, args []interface{}) (allow, decided bool) {
	if len(args) == 0 {
		return false, false
	}
	mode, ok := castModeConditionArg(args[0])
	if !ok {
		return false, false
	}
	item := currentResolvingSpell(gs, src)
	if item == nil {
		return false, false
	}
	satisfied, known := castModeSatisfied(item, mode)
	if !known {
		return false, false
	}
	return satisfied, true
}
