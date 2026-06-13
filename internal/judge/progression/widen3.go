package progression

// widen3.go — PROGRESSION phase 3 (r63): coverage toward the full
// corpus. The in-scope vocabulary grows from ~1,847 triggers by the
// largest unmeasured clusters (same independence contract as phases
// 1-2: expectations derive from the AST trigger + raw oracle wording,
// firings observed purely as state deltas; the engine's dispatch is
// never consulted to FORM an expectation, only driven as the stimulus):
//
//	combat_damage_player  "whenever ~ deals combat damage to a player"
//	                      (self-wordings; 538 corpus triggers)
//	cast family           cast_any / cast_spell / any_player_cast /
//	                      opp_cast / cast_filtered with raw-recovered
//	                      filters: any / noncreature (the prowess
//	                      class) / instant-or-sorcery / creature /
//	                      artifact / enchantment (~1,000)
//	you_attack            "whenever you attack," bare (93)
//	begin-combat          "at the beginning of combat on your turn" /
//	                      "of each combat" (228) — the widening found
//	                      the engine dropped ALL of these (see
//	                      combat.go combatBeginTriggerScope fix)
//	draw step             "at the beginning of your draw step" (14)
//	ETB-ally family       etb_or_another / another_typed_enters /
//	                      tribe_you_control_etb with raw-recovered
//	                      subtype/color filters (~290)
//
// Every scope gate is FAIL-CLOSED: unrecognized wordings, "if" riders,
// and multi-token filters stay out of scope rather than guessed.

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/judge/outcome"
)

// CheckAny chains every PROGRESSION check family (phases 1-3) for one
// (card, trigger) pair. The single entry point drivers should use.
func CheckAny(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	if f, ran := CheckTrigger(cardName, t); ran {
		return f, true
	}
	if f, ran := CheckPhaseTrigger(cardName, t); ran {
		return f, true
	}
	if f, ran := CheckLTBTrigger(cardName, t); ran {
		return f, true
	}
	if f, ran := CheckCombatDamageTrigger(cardName, t); ran {
		return f, true
	}
	if f, ran := CheckCastTrigger(cardName, t); ran {
		return f, true
	}
	if f, ran := CheckYouAttackTrigger(cardName, t); ran {
		return f, true
	}
	if f, ran := CheckCombatBeginTrigger(cardName, t); ran {
		return f, true
	}
	if f, ran := CheckAllyETBTrigger(cardName, t); ran {
		return f, true
	}
	if f, ran := checkAnyPhase3b(cardName, t); ran {
		return f, true
	}
	if f, ran := checkAnyPhase3c(cardName, t); ran {
		return f, true
	}
	if f, ran := checkAnyPhase3d(cardName, t); ran {
		return f, true
	}
	if f, ran := checkAnyPhase3e(cardName, t); ran {
		return f, true
	}
	if f, ran := checkAnyPhase3f(cardName, t); ran {
		return f, true
	}
	return nil, false
}

func matchSet(set []*outcome.Delta, actual *outcome.Delta) bool {
	for _, exp := range set {
		if actual.Equal(exp) {
			return true
		}
	}
	return false
}

// cleanRider reports whether the raw has no conditional rider that the
// parser drops (intervening-if class) — fail closed when " if " appears.
func cleanRider(raw string) bool {
	return !strings.Contains(strings.ToLower(raw), " if ")
}

// ---------------------------------------------------------------------------
// combat_damage_player — "whenever ~ deals combat damage to a player"
// ---------------------------------------------------------------------------

// InScopeCombatDamageTrigger keeps self-dealing wordings only (the
// parser drops actors, so "equipped creature deals…" parses identically
// — those stay out until carried).
func InScopeCombatDamageTrigger(t *gameast.Triggered) bool {
	if t == nil || t.Effect == nil || t.InterveningIf != nil {
		return false
	}
	tr := t.Trigger
	if tr.Actor != nil || tr.TargetFilter != nil || tr.Condition != nil ||
		tr.Phase != "" || tr.Controller != "" {
		return false
	}
	if tr.Event != "combat_damage_player" {
		return false
	}
	raw := strings.ToLower(t.Raw)
	if !cleanRider(raw) {
		return false
	}
	return strings.Contains(raw, "whenever ~ deals combat damage to a player") ||
		strings.Contains(raw, "whenever this creature deals combat damage to a player")
}

// CheckCombatDamageTrigger: FIRE — the bearer (its controller's only
// creature) attacks unblocked and the combat damage step runs; the
// expected delta composes the effect with the bearer's own 4 combat
// damage (subtracted, along with its declaration tap). PHANTOM — a
// vanilla bystander deals the damage instead.
func CheckCombatDamageTrigger(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	if !InScopeCombatDamageTrigger(t) {
		return nil, false
	}
	if perCardOwned(cardName, "combat_damage_player") {
		return nil, false
	}
	// Counter-placing effects on a card that ALSO carries a per_card
	// counter_placed handler (Hapatra's "create a 1/1 Snake whenever you
	// put -1/-1 counters on a creature") are unsound to model in isolation:
	// wrapSingle strips the AST snake-maker, but the per_card handler is
	// name-keyed and still fires when the trigger's effect places the
	// counter, adding a token the single-trigger expectation cannot predict.
	// The engine IS correct (the snake is real). Skip (PROGRESSION r63 FP).
	if strings.Contains(strings.ToLower(t.Raw), "counter") &&
		perCardOwned(cardName, "counter_placed") {
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
		// Undo the attacker's own combat damage to the defending seat.
		actual.LifeBySeat[1] += dmg
		if actual.LifeBySeat[1] == 0 {
			delete(actual.LifeBySeat, 1)
		}
		return actual
	}

	actual := runDamage(true)
	if !matchSet(expectedSet, actual) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "combat_damage_player", Check: "fire",
			Expected: describeSet(expectedSet), Actual: actual.String(), Raw: t.Raw,
		})
	}
	actual2 := runDamage(false)
	if !actual2.Equal(outcome.NewDelta()) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "combat_damage_player", Check: "phantom",
			Expected: "no change (another creature dealt the damage)", Actual: actual2.String(), Raw: t.Raw,
		})
	}
	emitAll(findings)
	return findings, true
}

// ---------------------------------------------------------------------------
// Cast family — "whenever you/a player/an opponent cast(s) <filter> spell"
// ---------------------------------------------------------------------------

type castScopeSpec struct {
	Who    string // "you" / "any" / "opp"
	Filter string // "any" / "noncreature" / "iss" / "creature" / "artifact" / "enchantment"
}

// InScopeCastTrigger classifies the cast-trigger family with
// raw-recovered who/filter. Fail-closed on anything else (mana-value
// riders, "from exile", copy clauses, …).
func InScopeCastTrigger(t *gameast.Triggered) (castScopeSpec, bool) {
	none := castScopeSpec{}
	if t == nil || t.Effect == nil || t.InterveningIf != nil {
		return none, false
	}
	tr := t.Trigger
	if tr.Actor != nil || tr.TargetFilter != nil || tr.Condition != nil ||
		tr.Phase != "" || tr.Controller != "" {
		return none, false
	}
	switch tr.Event {
	case "cast_any", "cast_spell", "cast_filtered", "any_player_cast", "opp_cast":
	default:
		return none, false
	}
	raw := strings.ToLower(t.Raw)
	if !cleanRider(raw) {
		return none, false
	}
	var who, rest string
	switch {
	case strings.Contains(raw, "whenever you cast "):
		who = "you"
		rest = raw[strings.Index(raw, "whenever you cast ")+len("whenever you cast "):]
	case strings.Contains(raw, "whenever a player casts "):
		who = "any"
		rest = raw[strings.Index(raw, "whenever a player casts ")+len("whenever a player casts "):]
	case strings.Contains(raw, "whenever an opponent casts "):
		who = "opp"
		rest = raw[strings.Index(raw, "whenever an opponent casts "):]
		rest = rest[len("whenever an opponent casts "):]
	default:
		return none, false
	}
	// The filter phrase must terminate at "spell," — riders fail closed.
	var filter string
	switch {
	case strings.HasPrefix(rest, "a spell,"):
		filter = "any"
	case strings.HasPrefix(rest, "a noncreature spell,"):
		filter = "noncreature"
	case strings.HasPrefix(rest, "an instant or sorcery spell,"):
		filter = "iss"
	case strings.HasPrefix(rest, "a creature spell,"):
		filter = "creature"
	case strings.HasPrefix(rest, "an artifact spell,"):
		filter = "artifact"
	case strings.HasPrefix(rest, "an enchantment spell,"):
		filter = "enchantment"
	case strings.HasPrefix(rest, "a multicolored spell,"):
		filter = "multicolored"
	case strings.HasPrefix(rest, "a white spell,"):
		filter = "color:W"
	case strings.HasPrefix(rest, "a blue spell,"):
		filter = "color:U"
	case strings.HasPrefix(rest, "a black spell,"):
		filter = "color:B"
	case strings.HasPrefix(rest, "a red spell,"):
		filter = "color:R"
	case strings.HasPrefix(rest, "a green spell,"):
		filter = "color:G"
	default:
		return none, false
	}
	return castScopeSpec{Who: who, Filter: filter}, true
}

// castVanilla casts a 0-cost vanilla spell of the given kind from
// `seat`'s hand and returns the cast+resolution movement delta to
// subtract: instants go hand→graveyard; creatures go hand→battlefield
// (1/1 body joins the sums).
func castVanilla(gs *gameengine.GameState, seat int, kind string) *outcome.Delta {
	card := &gameengine.Card{
		Name:  "Vanilla " + kind + " Spell",
		Owner: seat,
	}
	// The vanilla card is APPENDED to hand inside this helper — after
	// the caller's `before` snapshot — so the append (+1) and the cast's
	// hand removal (−1) cancel: the composed movement carries NO hand
	// entry, only the destination zone.
	move := outcome.NewDelta()
	switch kind {
	case "creature":
		card.Types = []string{"creature"}
		card.BasePower, card.BaseToughness = 1, 1
		move.BattlefieldBySeat[seat] = 1
		move.PowerSum, move.ToughSum = 1, 1
	case "artifact":
		card.Types = []string{"artifact"}
		move.BattlefieldBySeat[seat] = 1
	case "enchantment":
		card.Types = []string{"enchantment"}
		move.BattlefieldBySeat[seat] = 1
	default: // instant (optionally colored: kind "color:R" / "multicolored")
		card.Types = []string{"instant"}
		if kind == "multicolored" {
			card.Colors = []string{"U", "R"}
		} else if letter, ok := strings.CutPrefix(kind, "color:"); ok {
			card.Colors = []string{letter}
		}
		move.GraveyardBySeat[seat] = 1
	}
	gs.Seats[seat].Hand = append(gs.Seats[seat].Hand, card)
	_ = gameengine.CastSpell(gs, seat, card, nil)
	return move
}

// matchesCastFilter reports whether the vanilla kind satisfies the
// recovered filter.
func matchesCastFilter(filter, kind string) bool {
	switch filter {
	case "any":
		return true
	case "noncreature":
		return kind != "creature"
	case "iss":
		return kind == "instant"
	case "creature":
		return kind == "creature"
	case "artifact":
		return kind == "artifact"
	case "enchantment":
		return kind == "enchantment"
	case "multicolored":
		return kind == "multicolored"
	}
	if strings.HasPrefix(filter, "color:") {
		return kind == filter // colored vanilla kinds carry the filter tag
	}
	return false
}

// CheckCastTrigger: FIRE/PHANTOM across the who × filter matrix —
// matching cast by the right caster fires once; a wrong-caster or
// wrong-kind cast stays silent.
func CheckCastTrigger(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	spec0, ok := InScopeCastTrigger(t)
	if !ok {
		return nil, false
	}
	if perCardOwned(cardName, "spell_cast", "noncreature_spell_cast",
		"instant_or_sorcery_cast", "creature_spell_cast", "spell_cast_by_opponent") {
		return nil, false
	}
	spec := progressionSpec()
	expectedSet, ok := expectedFireSet(spec, t.Effect)
	if !ok {
		return nil, false
	}

	var findings []*Finding
	run := func(casterSeat int, kind string) *outcome.Delta {
		gs, bearer := outcome.BuildBoardForSpec(spec, cardName)
		bearer.Card.AST = wrapSingle(t)
		before := outcome.Snap(gs)
		move := castVanilla(gs, casterSeat, kind)
		actual := outcome.DiffSnapshots(before, outcome.Snap(gs))
		subtractDelta(actual, move)
		return actual
	}

	// Pick a FIRE kind matching the filter and a PHANTOM kind missing it.
	fireKind := "instant"
	switch {
	case spec0.Filter == "creature":
		fireKind = "creature"
	case spec0.Filter == "artifact":
		fireKind = "artifact"
	case spec0.Filter == "enchantment":
		fireKind = "enchantment"
	case spec0.Filter == "multicolored" || strings.HasPrefix(spec0.Filter, "color:"):
		fireKind = spec0.Filter // colored instant
	}
	missKind := ""
	switch {
	case spec0.Filter == "noncreature" || spec0.Filter == "iss":
		missKind = "creature"
	case spec0.Filter == "creature" || spec0.Filter == "artifact" || spec0.Filter == "enchantment":
		missKind = "instant"
	case spec0.Filter == "multicolored" || strings.HasPrefix(spec0.Filter, "color:"):
		missKind = "instant" // colorless instant misses any color filter
	}

	// FIRE by the right caster.
	fireSeat := 0
	if spec0.Who == "opp" {
		fireSeat = 1
	}
	actual := run(fireSeat, fireKind)
	if !matchSet(expectedSet, actual) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "cast/" + spec0.Who + "/" + spec0.Filter, Check: "fire",
			Expected: describeSet(expectedSet), Actual: actual.String(), Raw: t.Raw,
		})
	}

	// Caster-scope phantom / second fire.
	otherSeat := 1 - fireSeat
	actualOther := run(otherSeat, fireKind)
	if spec0.Who == "any" {
		if !matchSet(expectedSet, actualOther) {
			findings = append(findings, &Finding{
				CardName: cardName, Event: "cast/" + spec0.Who + "/" + spec0.Filter, Check: "any_scope_fire",
				Expected: describeSet(expectedSet), Actual: actualOther.String(), Raw: t.Raw,
			})
		}
	} else if !actualOther.Equal(outcome.NewDelta()) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "cast/" + spec0.Who + "/" + spec0.Filter, Check: "caster_gate_phantom",
			Expected: "no change (wrong caster)", Actual: actualOther.String(), Raw: t.Raw,
		})
	}

	// Kind-filter phantom.
	if missKind != "" {
		actualMiss := run(fireSeat, missKind)
		if !actualMiss.Equal(outcome.NewDelta()) {
			findings = append(findings, &Finding{
				CardName: cardName, Event: "cast/" + spec0.Who + "/" + spec0.Filter, Check: "filter_phantom",
				Expected: "no change (non-matching spell kind)", Actual: actualMiss.String(), Raw: t.Raw,
			})
		}
	}
	emitAll(findings)
	return findings, true
}

// ---------------------------------------------------------------------------
// you_attack — "whenever you attack,"
// ---------------------------------------------------------------------------

func InScopeYouAttackTrigger(t *gameast.Triggered) bool {
	if t == nil || t.Effect == nil || t.InterveningIf != nil {
		return false
	}
	tr := t.Trigger
	if tr.Actor != nil || tr.TargetFilter != nil || tr.Condition != nil ||
		tr.Phase != "" || tr.Controller != "" {
		return false
	}
	if tr.Event != "you_attack" {
		return false
	}
	raw := strings.ToLower(t.Raw)
	// Bare wording only — "with one or more <type>" and "if" riders out.
	return strings.Contains(raw, "whenever you attack,") && cleanRider(raw)
}

// CheckYouAttackTrigger: FIRE — the bearer's controller declares an
// attack. PHANTOM — the OPPONENT attacks (a "you attack" trigger must
// not fire for the defender).
func CheckYouAttackTrigger(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	if !InScopeYouAttackTrigger(t) {
		return nil, false
	}
	if perCardOwned(cardName, "creature_attacks", "you_attack") {
		return nil, false
	}
	spec := progressionSpec()
	expectedSet, ok := expectedFireSet(spec, t.Effect)
	if !ok {
		return nil, false
	}

	var findings []*Finding
	// FIRE: bearer attacks (only creature its controller has).
	gs, bearer := outcome.BuildBoardForSpec(spec, cardName)
	bearer.Card.AST = wrapSingle(t)
	bearer.SummoningSick = false
	wasTapped := bearer.Tapped
	before := outcome.Snap(gs)
	gameengine.DeclareAttackers(gs, 0)
	actual := outcome.DiffSnapshots(before, outcome.Snap(gs))
	if !wasTapped && bearer.Tapped && stillOnBattlefield(gs, bearer) {
		subtractTap(actual, 1)
	}
	if !matchSet(expectedSet, actual) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "you_attack", Check: "fire",
			Expected: describeSet(expectedSet), Actual: actual.String(), Raw: t.Raw,
		})
	}

	// PHANTOM: the opponent attacks us.
	gs2, bearer2 := outcome.BuildBoardForSpec(spec, cardName)
	bearer2.Card.AST = wrapSingle(t)
	bearer2.SummoningSick = true
	ov := vanillaPerm(gs2, 1)
	ov.SummoningSick = false
	place(gs2, ov)
	gs2.Active = 1
	// The greedy attack policy declares EVERY legal opponent creature
	// (the spec board ships one besides ov) — track all of their taps,
	// not just ov's.
	tappedBefore := map[*gameengine.Permanent]bool{}
	for _, p := range gs2.Seats[1].Battlefield {
		if p != nil {
			tappedBefore[p] = p.Tapped
		}
	}
	before2 := outcome.Snap(gs2)
	gameengine.DeclareAttackers(gs2, 1)
	actual2 := outcome.DiffSnapshots(before2, outcome.Snap(gs2))
	for p, was := range tappedBefore {
		if !was && p.Tapped {
			subtractTap(actual2, 1)
		}
	}
	if !actual2.Equal(outcome.NewDelta()) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "you_attack", Check: "defender_phantom",
			Expected: "no change (opponent attacked)", Actual: actual2.String(), Raw: t.Raw,
		})
	}
	emitAll(findings)
	return findings, true
}

// ---------------------------------------------------------------------------
// Begin-combat phase triggers — combat_start_yours / combat_start_each
// ---------------------------------------------------------------------------

func InScopeCombatBeginTrigger(t *gameast.Triggered) (string, bool) {
	if t == nil || t.Effect == nil || t.InterveningIf != nil {
		return "", false
	}
	tr := t.Trigger
	if tr.Actor != nil || tr.TargetFilter != nil || tr.Condition != nil {
		return "", false
	}
	if tr.Event != "phase" {
		return "", false
	}
	if !cleanRider(t.Raw) {
		return "", false
	}
	switch tr.Phase {
	case "combat_start_yours":
		return "your", true
	case "combat_start_each":
		return "each", true
	}
	return "", false
}

// CheckCombatBeginTrigger drives the engine's begin-combat dispatch
// (the stimulus is fireBeginningOfCombatTriggers via CombatPhase's
// entry — exercised directly through the exported test seam
// FireBeginCombatTriggersForTest) on the controller's combat and on the
// opponent's combat, asserting the your/each scope gate.
func CheckCombatBeginTrigger(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	scope, ok := InScopeCombatBeginTrigger(t)
	if !ok {
		return nil, false
	}
	if perCardOwned(cardName, "combat_begin", "begin_combat") {
		return nil, false
	}
	spec := progressionSpec()
	expectedSet, ok := expectedFireSet(spec, t.Effect)
	if !ok {
		return nil, false
	}

	var findings []*Finding
	run := func(activeSeat int) *outcome.Delta {
		gs, bearer := outcome.BuildBoardForSpec(spec, cardName)
		bearer.Card.AST = wrapSingle(t)
		gs.Active = activeSeat
		before := outcome.Snap(gs)
		gameengine.FireBeginCombatTriggersForTest(gs, activeSeat)
		return outcome.DiffSnapshots(before, outcome.Snap(gs))
	}

	actual := run(0)
	if !matchSet(expectedSet, actual) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "combat_begin/" + scope, Check: "fire",
			Expected: describeSet(expectedSet), Actual: actual.String(), Raw: t.Raw,
		})
	}
	actualOpp := run(1)
	if scope == "your" {
		if !actualOpp.Equal(outcome.NewDelta()) {
			findings = append(findings, &Finding{
				CardName: cardName, Event: "combat_begin/" + scope, Check: "controller_gate_phantom",
				Expected: "no change (opponent's combat)", Actual: actualOpp.String(), Raw: t.Raw,
			})
		}
	} else if !matchSet(expectedSet, actualOpp) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "combat_begin/" + scope, Check: "each_scope_fire",
			Expected: describeSet(expectedSet), Actual: actualOpp.String(), Raw: t.Raw,
		})
	}
	emitAll(findings)
	return findings, true
}

// ---------------------------------------------------------------------------
// ETB-ally family — etb_or_another / another_typed_enters /
// tribe_you_control_etb
// ---------------------------------------------------------------------------

type allyETBSpec struct {
	includesSelf bool   // "~ or another creature you control enters"
	subtype      string // "" = any creature; single-token subtype otherwise
	color        string // single color word ("green") or ""
}

var allyColorWords = map[string]string{
	"white": "W", "blue": "U", "black": "B", "red": "R", "green": "G",
}

// InScopeAllyETBTrigger recovers the ally-ETB shape from the raw.
func InScopeAllyETBTrigger(t *gameast.Triggered) (allyETBSpec, bool) {
	none := allyETBSpec{}
	if t == nil || t.Effect == nil || t.InterveningIf != nil {
		return none, false
	}
	tr := t.Trigger
	if tr.Actor != nil || tr.TargetFilter != nil || tr.Condition != nil ||
		tr.Phase != "" || tr.Controller != "" {
		return none, false
	}
	switch tr.Event {
	case "etb_or_another", "another_typed_enters", "tribe_you_control_etb":
	default:
		return none, false
	}
	raw := strings.ToLower(t.Raw)
	if !cleanRider(raw) {
		return none, false
	}
	// or-self forms.
	if strings.Contains(raw, "or another creature you control enters,") {
		return allyETBSpec{includesSelf: true}, true
	}
	// "whenever another <X> creature you control enters,"
	if i := strings.Index(raw, "whenever another "); i >= 0 {
		rest := raw[i+len("whenever another "):]
		if strings.HasPrefix(rest, "creature you control enters,") {
			return allyETBSpec{}, true
		}
		j := strings.Index(rest, " creature you control enters,")
		if j > 0 {
			word := rest[:j]
			if !strings.Contains(word, " ") {
				if _, isColor := allyColorWords[word]; isColor {
					return allyETBSpec{color: word}, true
				}
				return allyETBSpec{subtype: word}, true
			}
		}
		return none, false
	}
	// "whenever a/an <tribe> you control enters,"
	for _, lead := range []string{"whenever a ", "whenever an "} {
		if i := strings.Index(raw, lead); i >= 0 {
			rest := raw[i+len(lead):]
			j := strings.Index(rest, " you control enters,")
			if j > 0 {
				word := rest[:j]
				if word == "creature" {
					// includes the bearer per CR — but the scenario only
					// exercises the ally axis; self-ETB is phase-4.
					return allyETBSpec{}, true
				}
				if !strings.Contains(word, " ") {
					return allyETBSpec{subtype: word}, true
				}
			}
			return none, false
		}
	}
	return none, false
}

// CheckAllyETBTrigger: FIRE — a matching ally enters. PHANTOM — an
// opponent's matching creature enters (controller gate), and for typed
// shapes a NON-matching ally enters (type gate).
func CheckAllyETBTrigger(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	spec0, ok := InScopeAllyETBTrigger(t)
	if !ok {
		return nil, false
	}
	if perCardOwned(cardName, "permanent_etb", "etb", "creature_etb") {
		return nil, false
	}
	// Board-scaling effects ("put a +1/+1 counter on EACH creature you
	// control", "CREATURES YOU CONTROL get +1/+1") have a magnitude that
	// depends on how many creatures are on the battlefield when the trigger
	// resolves. The ally scenario adds the entering ally to the bearer, so
	// the engine correctly affects 2 creatures — but the single-application
	// expectedFireSet (computed against the bearer-only spec) models 1.
	// The expectation is structurally unable to model the post-entry board,
	// so skip (the engine IS correct: Cathars' Crusade, Goldnight Commander,
	// Valor in Akros — PROGRESSION r63 false positives).
	{
		lraw := strings.ToLower(t.Raw)
		if strings.Contains(lraw, "each creature you control") ||
			strings.Contains(lraw, "creatures you control get") ||
			strings.Contains(lraw, "each other creature you control") {
			return nil, false
		}
	}
	spec := progressionSpec()
	expectedSet, ok := expectedFireSet(spec, t.Effect)
	if !ok {
		return nil, false
	}

	mkAlly := func(gs *gameengine.GameState, seat int, matching bool) *gameengine.Permanent {
		v := vanillaPerm(gs, seat)
		if matching {
			if spec0.subtype != "" {
				v.Card.Types = append(v.Card.Types, spec0.subtype)
			}
			if spec0.color != "" {
				v.Card.Colors = []string{allyColorWords[spec0.color]}
			}
		}
		return v
	}

	var findings []*Finding
	run := func(seat int, matching bool) *outcome.Delta {
		gs, bearer := outcome.BuildBoardForSpec(spec, cardName)
		bearer.Card.AST = wrapSingle(t)
		v := mkAlly(gs, seat, matching)
		place(gs, v)
		before := outcome.Snap(gs)
		gameengine.FirePermanentETBTriggers(gs, v)
		return outcome.DiffSnapshots(before, outcome.Snap(gs))
	}

	evName := "ally_etb"
	if spec0.subtype != "" {
		evName += "/" + spec0.subtype
	}
	if spec0.color != "" {
		evName += "/" + spec0.color
	}

	// FIRE: matching ally enters.
	actual := run(0, true)
	if !matchSet(expectedSet, actual) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: evName, Check: "fire",
			Expected: describeSet(expectedSet), Actual: actual.String(), Raw: t.Raw,
		})
	}

	// CONTROLLER-GATE PHANTOM: opponent's matching creature enters.
	actualOpp := run(1, true)
	if !actualOpp.Equal(outcome.NewDelta()) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: evName, Check: "controller_gate_phantom",
			Expected: "no change (opponent's creature entered)", Actual: actualOpp.String(), Raw: t.Raw,
		})
	}

	// TYPE-GATE PHANTOM for typed/colored shapes: a plain vanilla ally.
	if spec0.subtype != "" || spec0.color != "" {
		actualPlain := run(0, false)
		if !actualPlain.Equal(outcome.NewDelta()) {
			findings = append(findings, &Finding{
				CardName: cardName, Event: evName, Check: "type_gate_phantom",
				Expected: "no change (non-matching creature entered)", Actual: actualPlain.String(), Raw: t.Raw,
			})
		}
	}
	emitAll(findings)
	return findings, true
}

// subtractDelta removes a composed movement from an observed delta
// (per-seat maps cleaned of zero entries).
func subtractDelta(d, move *outcome.Delta) {
	subMap := func(dst, src map[int]int) {
		for k, v := range src {
			dst[k] -= v
			if dst[k] == 0 {
				delete(dst, k)
			}
		}
	}
	subMap(d.LifeBySeat, move.LifeBySeat)
	subMap(d.HandBySeat, move.HandBySeat)
	subMap(d.LibraryBySeat, move.LibraryBySeat)
	subMap(d.GraveyardBySeat, move.GraveyardBySeat)
	subMap(d.ExileBySeat, move.ExileBySeat)
	subMap(d.BattlefieldBySeat, move.BattlefieldBySeat)
	for k, v := range move.CountersByKind {
		d.CountersByKind[k] -= v
		if d.CountersByKind[k] == 0 {
			delete(d.CountersByKind, k)
		}
	}
	d.MarkedDamage -= move.MarkedDamage
	d.Tapped -= move.Tapped
	d.PowerSum -= move.PowerSum
	d.ToughSum -= move.ToughSum
}
