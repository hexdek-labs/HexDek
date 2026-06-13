package progression

// widen7.go — PROGRESSION saturation sweep (r63): multiple remaining
// trigger families measured to drive PROGRESSION toward parity with the
// engine trigger surface. Same independence contract: expectations derive
// from the AST trigger + raw oracle wording, firings observed purely as
// state deltas, engine dispatch only ever DRIVEN as the stimulus.
//
//	draw_card  "whenever you draw a card," (35 corpus) — controller-gated
//	           draw payoff. Stimulus: FireDrawCardASTTriggers after a draw.

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/judge/outcome"
)

// checkAnyPhase3e chains the saturation-sweep families; CheckAny calls it
// after 3d.
func checkAnyPhase3e(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	if f, ran := CheckDrawCardTrigger(cardName, t); ran {
		return f, true
	}
	if f, ran := CheckSacrificeTrigger(cardName, t); ran {
		return f, true
	}
	return nil, false
}

// ---------------------------------------------------------------------------
// sacrifice_filtered — "whenever you sacrifice a <filter>,"
// ---------------------------------------------------------------------------

// sacFilter classifies the victim filter recovered from the raw wording.
type sacFilter struct {
	typ     string // "creature" / "artifact" / "enchantment" / "land" / "any"
	another bool
}

// InScopeSacrificeTrigger recovers the controller-scoped "whenever you
// sacrifice a <filter>," wording. Unrecognized filters fail closed.
func InScopeSacrificeTrigger(t *gameast.Triggered) (sacFilter, bool) {
	none := sacFilter{}
	if t == nil || t.Effect == nil || t.InterveningIf != nil {
		return none, false
	}
	tr := t.Trigger
	if tr.Actor != nil || tr.TargetFilter != nil || tr.Condition != nil ||
		tr.Phase != "" || tr.Controller != "" {
		return none, false
	}
	if tr.Event != "sacrifice_filtered" {
		return none, false
	}
	raw := strings.ToLower(t.Raw)
	if !cleanRider(raw) || !strings.Contains(raw, "whenever you sacrifice") {
		return none, false
	}
	// The FIRE scenario places an extra matching victim on the controller's
	// board, so anthem effects scoped to "creatures you control get" scale
	// with the post-placement count the single-application expectation can't
	// model (board-scaling class — Zhao, Ruthless Admiral). And an effect
	// that itself induces a sacrifice ("X sacrifices a creature" — Ruthless
	// Deathfang) cascades through the same dispatch, which the bare
	// single-snapshot expectation can't compose. Fail closed on both.
	effRaw := raw[strings.Index(raw, "sacrifice"):]
	if strings.Contains(effRaw, "creatures you control get") ||
		strings.Contains(effRaw, "sacrifices a") || strings.Contains(effRaw, "sacrifices another") {
		return none, false
	}
	another := strings.Contains(raw, "sacrifice another")
	switch {
	case strings.Contains(raw, "sacrifice a creature,"), strings.Contains(raw, "sacrifice another creature,"):
		return sacFilter{"creature", another}, true
	case strings.Contains(raw, "sacrifice an artifact,"), strings.Contains(raw, "sacrifice another artifact,"):
		return sacFilter{"artifact", another}, true
	case strings.Contains(raw, "sacrifice an enchantment,"), strings.Contains(raw, "sacrifice another enchantment,"):
		return sacFilter{"enchantment", another}, true
	case strings.Contains(raw, "sacrifice a land,"), strings.Contains(raw, "sacrifice another land,"):
		return sacFilter{"land", another}, true
	case strings.Contains(raw, "sacrifice a permanent,"), strings.Contains(raw, "sacrifice another permanent,"):
		return sacFilter{"any", another}, true
	}
	return none, false
}

// CheckSacrificeTrigger: FIRE — the controller sacrifices a matching victim
// (drives FireSacrificeASTTriggers; the victim is NOT removed — only the
// trigger's own effect is isolated). PHANTOM — the opponent sacrifices a
// matching victim; the controller's "you sacrifice" must stay silent.
func CheckSacrificeTrigger(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	sf, ok := InScopeSacrificeTrigger(t)
	if !ok {
		return nil, false
	}
	if perCardOwned(cardName, "sacrifice", "creature_sacrificed", "artifact_sacrificed", "permanent_sacrificed") {
		return nil, false
	}
	spec := progressionSpec()
	expectedSet, ok := expectedFireSet(spec, t.Effect)
	if !ok {
		return nil, false
	}

	victimTypes := []string{"creature"}
	switch sf.typ {
	case "artifact":
		victimTypes = []string{"artifact"}
	case "enchantment":
		victimTypes = []string{"enchantment"}
	case "land":
		victimTypes = []string{"land"}
	}

	var findings []*Finding
	run := func(sacSeat int) *outcome.Delta {
		gs, bearer := outcome.BuildBoardForSpec(spec, cardName)
		bearer.Card.AST = wrapSingle(t)
		// A separate matching victim owned by the sacrificing seat (never the
		// bearer, so "another" wordings match too).
		victim := vanillaPerm(gs, sacSeat)
		victim.Card.Types = append([]string{}, victimTypes...)
		place(gs, victim)
		before := outcome.Snap(gs)
		gameengine.FireSacrificeASTTriggers(gs, sacSeat, victim)
		return outcome.DiffSnapshots(before, outcome.Snap(gs))
	}

	actual := run(0)
	if !matchSet(expectedSet, actual) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "sacrifice_filtered", Check: "fire",
			Expected: describeSet(expectedSet), Actual: actual.String(), Raw: t.Raw,
		})
	}
	actualOpp := run(1)
	if !actualOpp.Equal(outcome.NewDelta()) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "sacrifice_filtered", Check: "controller_gate_phantom",
			Expected: "no change (opponent sacrificed)", Actual: actualOpp.String(), Raw: t.Raw,
		})
	}
	emitAll(findings)
	return findings, true
}

// ---------------------------------------------------------------------------
// draw_card — "whenever you draw a card,"
// ---------------------------------------------------------------------------

// InScopeDrawCardTrigger keeps the controller-scoped "whenever you draw a
// card," wording. "your second/third card each turn" is the distinct
// you_whenever event; "except the first" (§614.6) and "that much" riders
// are dropped by the parser, so they fail closed.
func InScopeDrawCardTrigger(t *gameast.Triggered) bool {
	if t == nil || t.Effect == nil || t.InterveningIf != nil {
		return false
	}
	tr := t.Trigger
	if tr.Actor != nil || tr.TargetFilter != nil || tr.Condition != nil ||
		tr.Phase != "" || tr.Controller != "" {
		return false
	}
	if tr.Event != "draw_card" {
		return false
	}
	raw := strings.ToLower(t.Raw)
	if !cleanRider(raw) || strings.Contains(raw, "except") || strings.Contains(raw, "that much") {
		return false
	}
	return strings.Contains(raw, "whenever you draw a card,")
}

// CheckDrawCardTrigger: FIRE — the controller draws (FireDrawCardASTTriggers
// for the controller); the draw payoff must produce its effect's delta once.
// CONTROLLER-GATE PHANTOM — an opponent draws; the controller's "you draw"
// trigger must stay silent.
func CheckDrawCardTrigger(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	if !InScopeDrawCardTrigger(t) {
		return nil, false
	}
	if perCardOwned(cardName, "card_drawn", "draw_card") {
		return nil, false
	}
	spec := progressionSpec()
	expectedSet, ok := expectedFireSet(spec, t.Effect)
	if !ok {
		return nil, false
	}

	var findings []*Finding
	run := func(drawerSeat int) *outcome.Delta {
		gs, bearer := outcome.BuildBoardForSpec(spec, cardName)
		bearer.Card.AST = wrapSingle(t)
		before := outcome.Snap(gs)
		gameengine.FireDrawCardASTTriggers(gs, drawerSeat)
		return outcome.DiffSnapshots(before, outcome.Snap(gs))
	}

	actual := run(0)
	if !matchSet(expectedSet, actual) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "draw_card", Check: "fire",
			Expected: describeSet(expectedSet), Actual: actual.String(), Raw: t.Raw,
		})
	}
	actualOpp := run(1)
	if !actualOpp.Equal(outcome.NewDelta()) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "draw_card", Check: "controller_gate_phantom",
			Expected: "no change (opponent drew)", Actual: actualOpp.String(), Raw: t.Raw,
		})
	}
	emitAll(findings)
	return findings, true
}
