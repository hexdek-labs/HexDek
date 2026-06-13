package progression

// widen8.go — PROGRESSION saturation sweep (r63), batch 2: the ally-death
// family. "whenever a/another creature you control dies," and the
// no-controller "whenever a creature dies," (Blood Artist) form. Same
// independence contract — expectations from the AST effect + raw wording,
// firings observed as state deltas, the engine death dispatch driven as the
// stimulus (DestroyPermanent, the real chokepoint).

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/judge/outcome"
)

func checkAnyPhase3f(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	if f, ran := CheckAllyDiesTrigger(cardName, t); ran {
		return f, true
	}
	return nil, false
}

// allyDiesScope is the recovered controller/another scope of an ally-death
// trigger (generic creature only; subtype/nontoken/conditional forms are
// out of scope and fail closed).
type allyDiesScope struct {
	anyController bool
	excludesSelf  bool
}

// InScopeAllyDiesTrigger keeps the generic "a/another creature [you control]
// dies," wordings. Subtype ("an angel you control dies"), nontoken,
// "or planeswalker", and conditional ("with a +1/+1 counter", "power or
// toughness 1 or less", "dealt damage by") forms fail closed.
func InScopeAllyDiesTrigger(t *gameast.Triggered) (allyDiesScope, bool) {
	none := allyDiesScope{}
	if t == nil || t.Effect == nil || t.InterveningIf != nil {
		return none, false
	}
	tr := t.Trigger
	if tr.Actor != nil || tr.TargetFilter != nil || tr.Condition != nil ||
		tr.Phase != "" || tr.Controller != "" {
		return none, false
	}
	switch tr.Event {
	case "creature_dies", "another_creature_dies", "another_creature_dies_any", "tribe_you_control_dies":
	default:
		return none, false
	}
	raw := strings.ToLower(t.Raw)
	if !cleanRider(raw) {
		return none, false
	}
	// Conditional / compound / board-scaling effects the bare scenario can't
	// model (an extra ally creature is on the board for the stimulus).
	if strings.Contains(raw, " with ") || strings.Contains(raw, "power or toughness") ||
		strings.Contains(raw, "dealt damage") || strings.Contains(raw, "or planeswalker") ||
		strings.Contains(raw, "named ") {
		return none, false
	}
	effRaw := raw
	if i := strings.Index(raw, " dies,"); i >= 0 {
		effRaw = raw[i:]
	}
	if strings.Contains(effRaw, "creatures you control get") ||
		strings.Contains(effRaw, "each creature you control") {
		return none, false
	}
	switch {
	case strings.Contains(raw, "whenever another creature you control dies,"):
		return allyDiesScope{excludesSelf: true}, true
	case strings.Contains(raw, "whenever a creature you control dies,"):
		return allyDiesScope{}, true
	case strings.Contains(raw, "whenever a creature dies,"):
		return allyDiesScope{anyController: true}, true
	}
	return none, false
}

// CheckAllyDiesTrigger: FIRE — a separate ally creature on the controller's
// board dies (the bearer's "a creature you control dies" must produce its
// effect once). CONTROLLER-GATE PHANTOM — an OPPONENT's creature dies; a
// controller-scoped trigger must stay silent (an "a creature dies" any-
// controller trigger must instead fire — checked accordingly). Driven
// through DestroyPermanent, the real death chokepoint.
func CheckAllyDiesTrigger(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	scope, ok := InScopeAllyDiesTrigger(t)
	if !ok {
		return nil, false
	}
	if perCardOwned(cardName, "creature_dies", "dies") {
		return nil, false
	}
	spec := progressionSpec()
	expectedSet, ok := expectedFireSet(spec, t.Effect)
	if !ok {
		return nil, false
	}

	var findings []*Finding
	// kill places a vanilla creature on `seat`, destroys it, and returns the
	// delta with that creature's bare death movement subtracted.
	kill := func(seat int) *outcome.Delta {
		gs, bearer := outcome.BuildBoardForSpec(spec, cardName)
		bearer.Card.AST = wrapSingle(t)
		victim := vanillaPerm(gs, seat)
		place(gs, victim)
		before := outcome.Snap(gs)
		gameengine.DestroyPermanent(gs, victim, nil)
		actual := outcome.DiffSnapshots(before, outcome.Snap(gs))
		subtractDeath(actual, seat, 1) // vanilla is a 1/1
		return actual
	}

	// FIRE: an ally (controller's) creature dies.
	actual := kill(0)
	if !matchSet(expectedSet, actual) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "ally_dies", Check: "fire",
			Expected: describeSet(expectedSet), Actual: actual.String(), Raw: t.Raw,
		})
	}

	// Opponent's creature dies.
	actualOpp := kill(1)
	if scope.anyController {
		// "a creature dies" — must ALSO fire for the opponent's creature.
		if !matchSet(expectedSet, actualOpp) {
			findings = append(findings, &Finding{
				CardName: cardName, Event: "ally_dies", Check: "any_controller_fire",
				Expected: describeSet(expectedSet), Actual: actualOpp.String(), Raw: t.Raw,
			})
		}
	} else {
		// "a creature YOU CONTROL dies" — must stay silent.
		if !actualOpp.Equal(outcome.NewDelta()) {
			findings = append(findings, &Finding{
				CardName: cardName, Event: "ally_dies", Check: "controller_gate_phantom",
				Expected: "no change (opponent's creature died)", Actual: actualOpp.String(), Raw: t.Raw,
			})
		}
	}
	emitAll(findings)
	return findings, true
}
