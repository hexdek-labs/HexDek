package progression

// widen4.go — PROGRESSION phase 3b (r63): the next band from the
// phase-3 report's unmeasured-trigger map. Same independence contract.
//
//	gain_life          "whenever you gain life," (77 corpus) — the
//	                   widening found ALL of them silent (alias fired
//	                   per_card only; engine fix FireLifeGainedASTTriggers)
//	becomes_tapped     self + enchanted/equipped wordings (82) — same
//	                   silent-alias bug (FireTapEventASTTriggers)
//	cast colors        "a <color> spell" / "a multicolored spell" (79)
//	enter_or_attack    self dual-stimulus forms (122)
//	type_leaves_battlefield  self "leaves the battlefield" forms (110)
//	deals_damage       self "deals damage" forms (87), combat-driven

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/judge/outcome"
)

// CheckAny2 chains the phase-3b families; CheckAny calls it last.
func checkAnyPhase3b(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	if f, ran := CheckGainLifeTrigger(cardName, t); ran {
		return f, true
	}
	if f, ran := CheckBecomesTappedTrigger(cardName, t); ran {
		return f, true
	}
	if f, ran := CheckEnterOrAttackTrigger(cardName, t); ran {
		return f, true
	}
	if f, ran := CheckSelfLTBTypeTrigger(cardName, t); ran {
		return f, true
	}
	if f, ran := CheckDealsDamageTrigger(cardName, t); ran {
		return f, true
	}
	return nil, false
}

// ---------------------------------------------------------------------------
// gain_life — "whenever you gain life,"
// ---------------------------------------------------------------------------

func InScopeGainLifeTrigger(t *gameast.Triggered) bool {
	if t == nil || t.Effect == nil || t.InterveningIf != nil {
		return false
	}
	tr := t.Trigger
	if tr.Actor != nil || tr.TargetFilter != nil || tr.Condition != nil ||
		tr.Phase != "" || tr.Controller != "" {
		return false
	}
	if tr.Event != "gain_life" {
		return false
	}
	raw := strings.ToLower(t.Raw)
	// "that much" amounts depend on the gained amount — out of the
	// fixed-amount interpreter's reach; threshold/first-time riders out.
	if strings.Contains(raw, "that much") || !cleanRider(raw) {
		return false
	}
	return strings.Contains(raw, "whenever you gain life,")
}

// CheckGainLifeTrigger: FIRE — the controller gains 2 life; the
// stimulus's own +2 is subtracted. PHANTOM — the opponent gains life.
func CheckGainLifeTrigger(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	if !InScopeGainLifeTrigger(t) {
		return nil, false
	}
	if perCardOwned(cardName, "life_gained", "life_change") {
		return nil, false
	}
	spec := progressionSpec()
	expectedSet, ok := expectedFireSet(spec, t.Effect)
	if !ok {
		return nil, false
	}

	var findings []*Finding
	run := func(gainSeat int) *outcome.Delta {
		gs, bearer := outcome.BuildBoardForSpec(spec, cardName)
		bearer.Card.AST = wrapSingle(t)
		before := outcome.Snap(gs)
		gameengine.GainLife(gs, gainSeat, 2, "test-stimulus")
		actual := outcome.DiffSnapshots(before, outcome.Snap(gs))
		actual.LifeBySeat[gainSeat] -= 2
		if actual.LifeBySeat[gainSeat] == 0 {
			delete(actual.LifeBySeat, gainSeat)
		}
		return actual
	}

	actual := run(0)
	if !matchSet(expectedSet, actual) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "gain_life", Check: "fire",
			Expected: describeSet(expectedSet), Actual: actual.String(), Raw: t.Raw,
		})
	}
	actual2 := run(1)
	if !actual2.Equal(outcome.NewDelta()) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "gain_life", Check: "controller_gate_phantom",
			Expected: "no change (opponent gained)", Actual: actual2.String(), Raw: t.Raw,
		})
	}
	emitAll(findings)
	return findings, true
}

// ---------------------------------------------------------------------------
// becomes_tapped — self + enchanted/equipped wordings
// ---------------------------------------------------------------------------

func InScopeBecomesTappedTrigger(t *gameast.Triggered) (attached bool, ok bool) {
	if t == nil || t.Effect == nil || t.InterveningIf != nil {
		return false, false
	}
	tr := t.Trigger
	if tr.Actor != nil || tr.TargetFilter != nil || tr.Condition != nil ||
		tr.Phase != "" || tr.Controller != "" {
		return false, false
	}
	if tr.Event != "becomes_tapped" {
		return false, false
	}
	raw := strings.ToLower(t.Raw)
	if !cleanRider(raw) {
		return false, false
	}
	switch {
	case strings.Contains(raw, "this creature becomes tapped"),
		strings.Contains(raw, "this land becomes tapped"),
		strings.Contains(raw, "this artifact becomes tapped"),
		strings.Contains(raw, "this permanent becomes tapped"),
		strings.Contains(raw, "~ becomes tapped"):
		return false, true
	case strings.Contains(raw, "enchanted creature becomes tapped"),
		strings.Contains(raw, "enchanted land becomes tapped"),
		strings.Contains(raw, "enchanted permanent becomes tapped"),
		strings.Contains(raw, "equipped creature becomes tapped"):
		return true, true
	}
	return false, false
}

// CheckBecomesTappedTrigger: FIRE — the bearer (or the permanent it
// enchants) transitions to tapped via the engine's tap dispatch.
// PHANTOM — a bystander taps.
func CheckBecomesTappedTrigger(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	isAttached, ok := InScopeBecomesTappedTrigger(t)
	if !ok {
		return nil, false
	}
	if perCardOwned(cardName, "tap_event") {
		return nil, false
	}
	spec := progressionSpec()
	expectedSet, ok := expectedFireSet(spec, t.Effect)
	if !ok {
		return nil, false
	}

	var findings []*Finding
	tapVia := func(gs *gameengine.GameState, p *gameengine.Permanent) {
		p.Tapped = true
		gameengine.FireTapEventASTTriggers(gs, p)
	}

	// FIRE.
	gs, bearer := outcome.BuildBoardForSpec(spec, cardName)
	bearer.Card.AST = wrapSingle(t)
	target := bearer
	if isAttached {
		host := vanillaPerm(gs, 0)
		place(gs, host)
		bearer.AttachedTo = host
		target = host
	}
	before := outcome.Snap(gs)
	tapVia(gs, target)
	actual := outcome.DiffSnapshots(before, outcome.Snap(gs))
	subtractTap(actual, 1) // the stimulus tap itself
	if !matchSet(expectedSet, actual) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "becomes_tapped", Check: "fire",
			Expected: describeSet(expectedSet), Actual: actual.String(), Raw: t.Raw,
		})
	}

	// PHANTOM: an unrelated bystander taps.
	gs2, bearer2 := outcome.BuildBoardForSpec(spec, cardName)
	bearer2.Card.AST = wrapSingle(t)
	v := vanillaPerm(gs2, 0)
	place(gs2, v)
	before2 := outcome.Snap(gs2)
	tapVia(gs2, v)
	actual2 := outcome.DiffSnapshots(before2, outcome.Snap(gs2))
	subtractTap(actual2, 1)
	if !actual2.Equal(outcome.NewDelta()) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "becomes_tapped", Check: "phantom",
			Expected: "no change (a bystander tapped)", Actual: actual2.String(), Raw: t.Raw,
		})
	}
	emitAll(findings)
	return findings, true
}

// ---------------------------------------------------------------------------
// enter_or_attack — self dual-stimulus forms
// ---------------------------------------------------------------------------

func InScopeEnterOrAttackTrigger(t *gameast.Triggered) bool {
	if t == nil || t.Effect == nil || t.InterveningIf != nil {
		return false
	}
	tr := t.Trigger
	if tr.Actor != nil || tr.TargetFilter != nil || tr.Condition != nil ||
		tr.Phase != "" || tr.Controller != "" {
		return false
	}
	if tr.Event != "enter_or_attack" {
		return false
	}
	raw := strings.ToLower(t.Raw)
	if !cleanRider(raw) {
		return false
	}
	return strings.Contains(raw, "whenever this creature enters or attacks,") ||
		strings.Contains(raw, "whenever ~ enters or attacks,")
}

// CheckEnterOrAttackTrigger: both halves must fire — ETB and attack —
// and a bystander's ETB must stay silent.
func CheckEnterOrAttackTrigger(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	if !InScopeEnterOrAttackTrigger(t) {
		return nil, false
	}
	if perCardOwned(cardName, "permanent_etb", "etb", "creature_attacks") {
		return nil, false
	}
	spec := progressionSpec()
	expectedSet, ok := expectedFireSet(spec, t.Effect)
	if !ok {
		return nil, false
	}

	var findings []*Finding
	// ETB half.
	gs, bearer := outcome.BuildBoardForSpec(spec, cardName)
	bearer.Card.AST = wrapSingle(t)
	before := outcome.Snap(gs)
	gameengine.FirePermanentETBTriggers(gs, bearer)
	actual := outcome.DiffSnapshots(before, outcome.Snap(gs))
	if !matchSet(expectedSet, actual) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "enter_or_attack", Check: "etb_fire",
			Expected: describeSet(expectedSet), Actual: actual.String(), Raw: t.Raw,
		})
	}

	// Attack half.
	gs2, bearer2 := outcome.BuildBoardForSpec(spec, cardName)
	bearer2.Card.AST = wrapSingle(t)
	bearer2.SummoningSick = false
	wasTapped := bearer2.Tapped
	before2 := outcome.Snap(gs2)
	gameengine.DeclareAttackers(gs2, 0)
	actual2 := outcome.DiffSnapshots(before2, outcome.Snap(gs2))
	if !wasTapped && bearer2.Tapped && stillOnBattlefield(gs2, bearer2) {
		subtractTap(actual2, 1)
	}
	if !matchSet(expectedSet, actual2) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "enter_or_attack", Check: "attack_fire",
			Expected: describeSet(expectedSet), Actual: actual2.String(), Raw: t.Raw,
		})
	}

	// Bystander-ETB phantom.
	gs3, bearer3 := outcome.BuildBoardForSpec(spec, cardName)
	bearer3.Card.AST = wrapSingle(t)
	v := vanillaPerm(gs3, 0)
	place(gs3, v)
	before3 := outcome.Snap(gs3)
	gameengine.FirePermanentETBTriggers(gs3, v)
	actual3 := outcome.DiffSnapshots(before3, outcome.Snap(gs3))
	if !actual3.Equal(outcome.NewDelta()) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "enter_or_attack", Check: "phantom",
			Expected: "no change (a bystander entered)", Actual: actual3.String(), Raw: t.Raw,
		})
	}
	emitAll(findings)
	return findings, true
}

// ---------------------------------------------------------------------------
// type_leaves_battlefield — self "leaves the battlefield" forms
// ---------------------------------------------------------------------------

func InScopeSelfLTBTypeTrigger(t *gameast.Triggered) bool {
	if t == nil || t.Effect == nil || t.InterveningIf != nil {
		return false
	}
	tr := t.Trigger
	if tr.Actor != nil || tr.TargetFilter != nil || tr.Condition != nil ||
		tr.Phase != "" || tr.Controller != "" {
		return false
	}
	if tr.Event != "type_leaves_battlefield" {
		return false
	}
	raw := strings.ToLower(t.Raw)
	if !cleanRider(raw) {
		return false
	}
	// Linked-exile returns ("return the exiled card") depend on
	// O-Ring-linked state the bare scenario doesn't model.
	if strings.Contains(raw, "exiled") {
		return false
	}
	// "put ITS counters on target …" moves the counters the source had
	// when it left — the bare scenario enters the source with no counters,
	// so the engine correctly moves zero. The parser models "its" as a
	// counter KIND, yielding a phantom counters[its:1] expectation. Skip:
	// the magnitude depends on the source's pre-LTB counter state the bare
	// scenario doesn't seed (Broodguard Elite — PROGRESSION r63 FP).
	if strings.Contains(raw, "its counters") {
		return false
	}
	return strings.Contains(raw, "when this creature leaves the battlefield,") ||
		strings.Contains(raw, "when this enchantment leaves the battlefield,") ||
		strings.Contains(raw, "when this artifact leaves the battlefield,") ||
		strings.Contains(raw, "when this permanent leaves the battlefield,") ||
		strings.Contains(raw, "when ~ leaves the battlefield,")
}

// CheckSelfLTBTypeTrigger mirrors CheckLTBTrigger's bounce scenarios.
func CheckSelfLTBTypeTrigger(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	if !InScopeSelfLTBTypeTrigger(t) {
		return nil, false
	}
	if perCardOwned(cardName, "permanent_ltb") {
		return nil, false
	}
	spec := progressionSpec()
	effSpec := spec
	effSpec.SrcIsCreature = false
	expectedSet, ok := expectedFireSet(effSpec, t.Effect)
	if !ok {
		return nil, false
	}

	var findings []*Finding
	gs, bearer := outcome.BuildBoardForSpec(spec, cardName)
	bearer.Card.AST = wrapSingle(t)
	before := outcome.Snap(gs)
	gameengine.BouncePermanent(gs, bearer, nil, "hand")
	actual := outcome.DiffSnapshots(before, outcome.Snap(gs))
	subtractBounce(actual, 0, 4)
	if !matchSet(expectedSet, actual) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "type_ltb", Check: "fire",
			Expected: describeSet(expectedSet), Actual: actual.String(), Raw: t.Raw,
		})
	}

	gs2, bearer2 := outcome.BuildBoardForSpec(spec, cardName)
	bearer2.Card.AST = wrapSingle(t)
	v := vanillaPerm(gs2, 0)
	place(gs2, v)
	before2 := outcome.Snap(gs2)
	gameengine.BouncePermanent(gs2, v, nil, "hand")
	actual2 := outcome.DiffSnapshots(before2, outcome.Snap(gs2))
	subtractBounce(actual2, 0, 1)
	if !actual2.Equal(outcome.NewDelta()) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "type_ltb", Check: "phantom",
			Expected: "no change (a bystander left)", Actual: actual2.String(), Raw: t.Raw,
		})
	}
	emitAll(findings)
	return findings, true
}

// ---------------------------------------------------------------------------
// deals_damage — self forms, combat-driven
// ---------------------------------------------------------------------------

func InScopeDealsDamageTrigger(t *gameast.Triggered) bool {
	if t == nil || t.Effect == nil || t.InterveningIf != nil {
		return false
	}
	tr := t.Trigger
	if tr.Actor != nil || tr.TargetFilter != nil || tr.Condition != nil ||
		tr.Phase != "" || tr.Controller != "" {
		return false
	}
	if tr.Event != "deals_damage" {
		return false
	}
	raw := strings.ToLower(t.Raw)
	if !cleanRider(raw) || strings.Contains(raw, "that much") {
		return false
	}
	return strings.Contains(raw, "whenever this creature deals damage,") ||
		strings.Contains(raw, "whenever ~ deals damage,")
}

// CheckDealsDamageTrigger reuses the unblocked-attack damage scenario.
func CheckDealsDamageTrigger(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	if !InScopeDealsDamageTrigger(t) {
		return nil, false
	}
	if perCardOwned(cardName, "deals_damage", "combat_damage_player") {
		return nil, false
	}
	spec := progressionSpec()
	expectedSet, ok := expectedFireSet(spec, t.Effect)
	if !ok {
		return nil, false
	}

	var findings []*Finding
	runDamage := func(useBearer bool) *outcome.Delta {
		gs, bearer := outcome.BuildBoardForSpec(spec, cardName)
		bearer.Card.AST = wrapSingle(t)
		atk := bearer
		dmg := 4
		if useBearer {
			bearer.SummoningSick = false
		} else {
			bearer.SummoningSick = true
			v := vanillaPerm(gs, 0)
			v.SummoningSick = false
			place(gs, v)
			atk = v
			dmg = 1
		}
		wasTapped := atk.Tapped
		before := outcome.Snap(gs)
		attackers := gameengine.DeclareAttackers(gs, 0)
		gameengine.DealCombatDamageStep(gs, attackers,
			map[*gameengine.Permanent][]*gameengine.Permanent{}, false)
		actual := outcome.DiffSnapshots(before, outcome.Snap(gs))
		if !wasTapped && atk.Tapped && stillOnBattlefield(gs, atk) {
			subtractTap(actual, 1)
		}
		actual.LifeBySeat[1] += dmg
		if actual.LifeBySeat[1] == 0 {
			delete(actual.LifeBySeat, 1)
		}
		return actual
	}

	actual := runDamage(true)
	if !matchSet(expectedSet, actual) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "deals_damage", Check: "fire",
			Expected: describeSet(expectedSet), Actual: actual.String(), Raw: t.Raw,
		})
	}
	actual2 := runDamage(false)
	if !actual2.Equal(outcome.NewDelta()) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "deals_damage", Check: "phantom",
			Expected: "no change (a bystander dealt the damage)", Actual: actual2.String(), Raw: t.Raw,
		})
	}
	emitAll(findings)
	return findings, true
}
