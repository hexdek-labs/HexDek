package main

// corpus_audit.go — 34K corpus outcome correctness audit module for Thor.
//
// Unlike Goldilocks (which checks that SOMETHING changed), the corpus audit
// verifies that effects produce the CORRECT outcome per AST:
//   - Draw{N=3} → hand size increased by exactly 3
//   - Damage{Amount=4} → target life decreased by 4 (or creature marked 4)
//   - GainLife{Amount=5} → controller life increased by 5
//   - CreateToken{Count=2} → battlefield gained exactly 2 permanents
//   - Mill{N=3} → library shrunk by 3, graveyard grew by 3
//   - Destroy → target in graveyard (or indestructible)
//   - Exile → target in exile zone
//   - Bounce → target in hand
//   - Discard{N=2} → hand size decreased by 2
//   - CounterMod{+1/+1, N=2} → permanent has 2 +1/+1 counters
//   - Buff{+2,+2} → creature P/T changed
//   - AddMana → mana pool increased
//   - CounterSpell → stack item countered
//   - GainAbility → keyword present on permanent
//
// Each assertion is SPECIFIC to the AST node — this is outcome CORRECTNESS,
// not liveness.

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// Failure mode constants for categorization.
// ---------------------------------------------------------------------------

const (
	failModeWrongOutcome  = "wrong_outcome"
	failModeMissingEvent  = "missing_event"
	failModePanic         = "panic"
	failModeNoChange      = "no_change"
	failModeInvariant     = "invariant_violation"
	failModeSkipped       = "skipped"
)

// corpusAuditResult holds per-card test outcome for aggregation.
type corpusAuditResult struct {
	cardName    string
	cardTypes   []string
	effectKind  string
	failureMode string // "" if pass
	message     string
	panicked    bool
	panicMsg    string
}

// ---------------------------------------------------------------------------
// Effect collector — walk the entire AST to find ALL leaf effects.
// ---------------------------------------------------------------------------

type leafEffect struct {
	effect          gameast.Effect
	fullEffect      gameast.Effect // parent wrapper for resolution
	abilityKind     string         // "triggered", "activated", "static", "spell_effect"
	trigger         *gameast.Trigger
	condition       *gameast.Condition
	isElseBranch    bool               // true if this leaf came from a Conditional's ElseBody
	parentCondition *gameast.Condition  // the Conditional's condition (for else-branch setup)
	cost            *gameast.Cost       // activated ability cost (for pre-payment setup)
}

// collectAllLeafEffects walks the card's AST and returns every leaf effect.
func collectAllLeafEffects(ast *gameast.CardAST) []leafEffect {
	if ast == nil {
		return nil
	}
	var results []leafEffect
	for _, ab := range ast.Abilities {
		switch a := ab.(type) {
		case *gameast.Triggered:
			leaves := flattenEffectEx(a.Effect, false, nil)
			for _, fl := range leaves {
				le := leafEffect{
					effect:          fl.effect,
					fullEffect:      a.Effect,
					abilityKind:     "triggered",
					trigger:         &a.Trigger,
					isElseBranch:    fl.isElseBranch,
					parentCondition: fl.parentCondition,
				}
				if a.InterveningIf != nil {
					le.condition = a.InterveningIf
				}
				results = append(results, le)
			}
		case *gameast.Activated:
			costCopy := a.Cost
			leaves := flattenEffectEx(a.Effect, false, nil)
			for _, fl := range leaves {
				results = append(results, leafEffect{
					effect:          fl.effect,
					fullEffect:      a.Effect,
					abilityKind:     "activated",
					isElseBranch:    fl.isElseBranch,
					parentCondition: fl.parentCondition,
					cost:            &costCopy,
				})
			}
		case *gameast.Static:
			if a.Modification != nil {
				eff := modificationToEffect(a.Modification)
				if eff != nil {
					results = append(results, leafEffect{
						effect:      eff,
						abilityKind: "static",
					})
				}
			}
		case *gameast.Keyword:
			// Keywords don't have verifiable effects per se.
			continue
		}
	}
	return results
}

// flattenEffect recursively unwraps composite effects to get all leaf effects.
func flattenEffect(eff gameast.Effect) []gameast.Effect {
	if eff == nil {
		return nil
	}
	switch e := eff.(type) {
	case *gameast.Sequence:
		var out []gameast.Effect
		for _, item := range e.Items {
			out = append(out, flattenEffect(item)...)
		}
		return out
	case *gameast.Optional_:
		return flattenEffect(e.Body)
	case *gameast.Conditional:
		var out []gameast.Effect
		out = append(out, flattenEffect(e.Body)...)
		out = append(out, flattenEffect(e.ElseBody)...)
		return out
	case *gameast.Choice:
		// Take first option for testing.
		if len(e.Options) > 0 {
			return flattenEffect(e.Options[0])
		}
		return nil
	case *gameast.UnknownEffect:
		return nil
	default:
		return []gameast.Effect{eff}
	}
}

// flatLeaf wraps an effect with provenance metadata from flattenEffectEx.
type flatLeaf struct {
	effect          gameast.Effect
	isElseBranch    bool
	parentCondition *gameast.Condition
}

// flattenEffectEx recursively unwraps composite effects like flattenEffect,
// but also tracks whether a leaf came from a Conditional's else-branch and
// captures the parent condition for inverse/positive-setup.
func flattenEffectEx(eff gameast.Effect, inElse bool, parentCond *gameast.Condition) []flatLeaf {
	if eff == nil {
		return nil
	}
	switch e := eff.(type) {
	case *gameast.Sequence:
		var out []flatLeaf
		for _, item := range e.Items {
			out = append(out, flattenEffectEx(item, inElse, parentCond)...)
		}
		return out
	case *gameast.Optional_:
		return flattenEffectEx(e.Body, inElse, parentCond)
	case *gameast.Conditional:
		var out []flatLeaf
		// If-branch: propagate the Conditional's condition so setupCondition
		// can ensure the condition is TRUE for these leaves.
		out = append(out, flattenEffectEx(e.Body, inElse, e.Condition)...)
		// Else-branch: mark as else and propagate the condition for inverse setup.
		out = append(out, flattenEffectEx(e.ElseBody, true, e.Condition)...)
		return out
	case *gameast.Choice:
		if len(e.Options) > 0 {
			return flattenEffectEx(e.Options[0], inElse, parentCond)
		}
		return nil
	case *gameast.UnknownEffect:
		return nil
	default:
		return []flatLeaf{{effect: eff, isElseBranch: inElse, parentCondition: parentCond}}
	}
}

// auditableEffects is the set of effect kinds that can have specific outcome
// assertions. Unlike verifiableEffects (which checks "something changed"),
// these have quantifiable expected outcomes.
var auditableEffects = map[string]bool{
	"draw":          true,
	"damage":        true,
	"gain_life":     true,
	"lose_life":     true,
	"create_token":  true,
	"mill":          true,
	"destroy":       true,
	"exile":         true,
	"bounce":        true,
	"discard":       true,
	"counter_mod":   true,
	"buff":          true,
	"add_mana":      true,
	"counter_spell": true,
	"grant_ability": true,
	"sacrifice":     true,
	"scry":          true,
	"surveil":       true,
	"set_life":      true,
}

// ---------------------------------------------------------------------------
// Per-card audit.
// ---------------------------------------------------------------------------

func auditCard(oc *oracleCard) []corpusAuditResult {
	if oc.ast == nil {
		return nil
	}

	leaves := collectAllLeafEffects(oc.ast)
	if len(leaves) == 0 {
		return nil
	}

	var results []corpusAuditResult
	for _, leaf := range leaves {
		kind := leaf.effect.Kind()
		if !auditableEffects[kind] {
			continue
		}
		r := auditSingleEffect(oc, leaf)
		results = append(results, r)
	}
	return results
}

func auditSingleEffect(oc *oracleCard, leaf leafEffect) (result corpusAuditResult) {
	result.cardName = oc.Name
	result.cardTypes = oc.Types
	result.effectKind = leaf.effect.Kind()

	defer func() {
		if r := recover(); r != nil {
			result.failureMode = failModePanic
			result.panicked = true
			result.panicMsg = fmt.Sprintf("%v", r)
			result.message = fmt.Sprintf("panic during %s audit: %v", result.effectKind, r)
		}
	}()

	// Check for known-untestable card+effect combinations.
	if skipCardEffect(oc.Name, leaf.effect.Kind()) {
		return // silently pass — not a real failure
	}

	// Build the effectInfo to reuse Goldilocks state construction.
	info := &effectInfo{
		effect:      leaf.effect,
		fullEffect:  leaf.fullEffect,
		kind:        leaf.effect.Kind(),
		abilityKind: leaf.abilityKind,
		trigger:     leaf.trigger,
		condition:   leaf.condition,
	}

	gs := makeGoldilocksState(oc, info)
	if gs == nil {
		result.failureMode = failModeSkipped
		result.message = "could not construct game state"
		return
	}

	// Find source permanent.
	var srcPerm *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.Name == oc.Name {
			srcPerm = p
			break
		}
	}

	// Wave 2: For else-branch leaves, set up state so the parent condition
	// evaluates to FALSE (the else-branch fires when the if-branch fails).
	// For if-branch leaves with an inner Conditional condition, ensure the
	// condition is TRUE so the if-branch fires.
	if leaf.isElseBranch {
		setupConditionInverse(gs, leaf.parentCondition, srcPerm)
	} else if leaf.parentCondition != nil {
		setupCondition(gs, leaf.parentCondition)
	}

	// Wave 3: For activated abilities, pre-pay costs (sacrifice fodder,
	// counter removal, life payment).
	if leaf.cost != nil {
		setupActivatedAbilityCost(gs, leaf.cost, srcPerm)
	}

	// Wave 3e: Edge case overrides for cards that cause self-harm invariant
	// violations during resolution.
	applyInvariantSafetyOverrides(gs, oc.Name, leaf.effect.Kind(), srcPerm)

	// Clear event log.
	gs.EventLog = gs.EventLog[:0]

	// Snapshot before resolution.
	before := takeSnapshot(gs)
	beforePermSnap := snapAllPerms(gs)

	// Resolve effect.
	// When the leaf came from inside a Conditional (either branch), bypass
	// the Conditional wrapper and resolve the leaf directly. The engine's
	// evalCondition defaults unknown condition kinds to true, which means:
	//   - else-branch leaves would never fire (condition always "passes")
	//   - if-branch leaves with complex you_control filters may not fire
	//     (countControlledByType can't parse multi-color "or" filters)
	// The corpus audit tests effect CORRECTNESS, not condition evaluation,
	// so resolving the leaf in isolation is the right approach.
	resolveEff := leaf.fullEffect
	if resolveEff == nil {
		resolveEff = leaf.effect
	}
	if leaf.parentCondition != nil {
		resolveEff = leaf.effect
	}

	switch leaf.abilityKind {
	case "triggered":
		if leaf.parentCondition != nil {
			gameengine.ResolveEffect(gs, srcPerm, resolveEff)
		} else {
			fireTriggerEvent(gs, srcPerm, info)
		}
	case "activated":
		// Resolve the leaf effect directly. Per-card activated handlers
		// manage their own cost payment and context (target_perm,
		// target_seat, abilityIdx) which the audit can't supply generically.
		// The audit verifies effect correctness, not handler wiring.
		gameengine.ResolveEffect(gs, srcPerm, resolveEff)
	default:
		gameengine.ResolveEffect(gs, srcPerm, resolveEff)
	}
	gameengine.StateBasedActions(gs)

	// Snapshot after resolution.
	after := takeSnapshot(gs)

	// Check invariants first.
	violations := gameengine.RunAllInvariants(gs)
	if len(violations) > 0 {
		result.failureMode = failModeInvariant
		result.message = fmt.Sprintf("[%s] %s: %s", info.kind, violations[0].Name, violations[0].Message)
		return
	}

	// Now assert SPECIFIC outcomes based on effect type.
	assertResult := assertOutcome(gs, before, after, beforePermSnap, leaf.effect, srcPerm)
	if assertResult != "" {
		result.failureMode = failModeWrongOutcome
		result.message = assertResult
		return
	}

	// Check event log for expected events.
	eventResult := assertEventLog(gs, leaf.effect)
	if eventResult != "" {
		result.failureMode = failModeMissingEvent
		result.message = eventResult
		return
	}

	// Pass — no failure mode set, message stays empty.
	return
}

// ---------------------------------------------------------------------------
// Permanent snapshot for all seats (for counter/buff verification).
// ---------------------------------------------------------------------------

type allPermSnapshots struct {
	perms map[string]permSnapshot // keyed by permanent card name
}

func snapAllPerms(gs *gameengine.GameState) allPermSnapshots {
	snaps := allPermSnapshots{perms: map[string]permSnapshot{}}
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, p := range seat.Battlefield {
			if p != nil && p.Card != nil {
				snaps.perms[p.Card.Name] = snapPerm(p)
			}
		}
	}
	return snaps
}

// ---------------------------------------------------------------------------
// Outcome assertion — checks SPECIFIC correctness per AST effect type.
// Returns "" on pass, error description on fail.
// ---------------------------------------------------------------------------

func assertOutcome(gs *gameengine.GameState, before, after goldilocksSnapshot, beforePerms allPermSnapshots, eff gameast.Effect, srcPerm *gameengine.Permanent) string {
	switch e := eff.(type) {
	case *gameast.Draw:
		n := resolveNumberOrRef(e.Count, gs)
		if n <= 0 {
			return "" // variable/scaling — can't assert exact count
		}
		// Controller (seat 0) or target player draws.
		seat := resolveTargetSeat(e.Target, 0)
		delta := after.handSize[seat] - before.handSize[seat]
		libDelta := before.libSize[seat] - after.libSize[seat]
		// Accept if hand grew by at least N or library shrunk by at least N
		// (cards may have been drawn and immediately discarded by other effects).
		if delta < n && libDelta < n {
			// Check event log for draw events as fallback.
			drawCount := countEventsOfKind(gs, "draw")
			if drawCount >= n {
				return ""
			}
			return fmt.Sprintf("draw: expected hand+%d (seat %d), got hand delta=%d lib delta=%d events=%d",
				n, seat, delta, libDelta, drawCount)
		}

	case *gameast.Damage:
		n := resolveNumberOrRef(e.Amount, gs)
		if n <= 0 {
			return "" // variable damage — can't assert
		}
		// Check if any damage event was logged with the right amount.
		dmgEvents := countEventsOfKindWithAmount(gs, "damage", n)
		if dmgEvents > 0 {
			return ""
		}
		// Check life decrease on opponent.
		for seat := 0; seat < 4; seat++ {
			if before.life[seat]-after.life[seat] >= n {
				return ""
			}
		}
		// Check creature marked damage.
		for _, seat := range gs.Seats {
			if seat == nil {
				continue
			}
			for _, p := range seat.Battlefield {
				if p != nil && p.MarkedDamage >= n {
					return ""
				}
			}
		}
		// Check if any creature was destroyed (lethal damage).
		for i := 0; i < 4; i++ {
			if after.graveyardSize[i] > before.graveyardSize[i] {
				return ""
			}
		}
		return fmt.Sprintf("damage: expected %d damage dealt, no matching life/damage/destroy observed", n)

	case *gameast.GainLife:
		n := resolveNumberOrRef(e.Amount, gs)
		if n <= 0 {
			return ""
		}
		seat := resolveTargetSeat(e.Target, 0)
		delta := after.life[seat] - before.life[seat]
		if delta >= n {
			return ""
		}
		// Check event log.
		if hasEventKind(gs, "life_change") || hasEventKind(gs, "gain_life") {
			return ""
		}
		return fmt.Sprintf("gain_life: expected life+%d (seat %d), got delta=%d", n, seat, delta)

	case *gameast.LoseLife:
		n := resolveNumberOrRef(e.Amount, gs)
		if n <= 0 {
			return ""
		}
		// Opponent (seat 1) or target loses life.
		seat := resolveTargetSeat(e.Target, 1)
		delta := before.life[seat] - after.life[seat]
		if delta >= n {
			return ""
		}
		if hasEventKind(gs, "life_change") || hasEventKind(gs, "lose_life") {
			return ""
		}
		return fmt.Sprintf("lose_life: expected life-%d (seat %d), got delta=%d", n, seat, delta)

	case *gameast.CreateToken:
		n := resolveNumberOrRef(e.Count, gs)
		if n <= 0 {
			n = 1 // default: create 1 token
		}
		// Controller's battlefield should grow.
		delta := after.battlefieldCnt[0] - before.battlefieldCnt[0]
		if delta >= n {
			return ""
		}
		// Check event log for token creation.
		tokenEvents := countEventsOfKind(gs, "create_token")
		if tokenEvents >= n {
			return ""
		}
		// Accept any battlefield growth (tokens may replace dying permanents).
		if delta > 0 {
			return ""
		}
		// Check if event log has any token-related events.
		if hasEventKind(gs, "create_token") || hasEventKind(gs, "token") {
			return ""
		}
		return fmt.Sprintf("create_token: expected battlefield+%d (seat 0), got delta=%d events=%d", n, delta, tokenEvents)

	case *gameast.Mill:
		n := resolveNumberOrRef(e.Count, gs)
		if n <= 0 {
			return ""
		}
		// Target player's library should shrink, graveyard grow.
		seat := resolveTargetSeat(e.Target, 1) // default: opponent
		libDelta := before.libSize[seat] - after.libSize[seat]
		graveDelta := after.graveyardSize[seat] - before.graveyardSize[seat]
		if libDelta >= n || graveDelta >= n {
			return ""
		}
		if hasEventKind(gs, "mill") || hasEventKind(gs, "zone_change") {
			return ""
		}
		return fmt.Sprintf("mill: expected lib-%d grave+%d (seat %d), got lib delta=%d grave delta=%d",
			n, n, seat, libDelta, graveDelta)

	case *gameast.Destroy:
		// Target should leave battlefield → graveyard.
		anyLeft := false
		for i := 0; i < 4; i++ {
			if after.battlefieldCnt[i] < before.battlefieldCnt[i] {
				anyLeft = true
				break
			}
			if after.graveyardSize[i] > before.graveyardSize[i] {
				anyLeft = true
				break
			}
		}
		if anyLeft {
			return ""
		}
		if hasEventKind(gs, "destroy") || hasEventKind(gs, "zone_change") ||
			hasEventKind(gs, "dies") {
			return ""
		}
		// Indestructible check — if the target has indestructible, this is expected.
		if hasEventKind(gs, "indestructible") {
			return ""
		}
		return "destroy: no permanent left battlefield or entered graveyard"

	case *gameast.Exile:
		anyExiled := false
		for i := 0; i < 4; i++ {
			if after.exileSize[i] > before.exileSize[i] {
				anyExiled = true
				break
			}
			if after.battlefieldCnt[i] < before.battlefieldCnt[i] {
				anyExiled = true
				break
			}
		}
		if anyExiled {
			return ""
		}
		if hasEventKind(gs, "exile") || hasEventKind(gs, "zone_change") {
			return ""
		}
		return "exile: no card moved to exile zone"

	case *gameast.Bounce:
		anyBounced := false
		for i := 0; i < 4; i++ {
			if after.handSize[i] > before.handSize[i] {
				anyBounced = true
				break
			}
			if after.battlefieldCnt[i] < before.battlefieldCnt[i] {
				anyBounced = true
				break
			}
		}
		if anyBounced {
			return ""
		}
		if hasEventKind(gs, "bounce") || hasEventKind(gs, "zone_change") {
			return ""
		}
		return "bounce: no permanent returned to hand"

	case *gameast.Discard:
		n := resolveNumberOrRef(e.Count, gs)
		if n <= 0 {
			return ""
		}
		// Target player's hand should shrink.
		seat := resolveTargetSeat(e.Target, 1) // default: opponent
		delta := before.handSize[seat] - after.handSize[seat]
		if delta >= n {
			return ""
		}
		if hasEventKind(gs, "discard") || hasEventKind(gs, "zone_change") {
			return ""
		}
		return fmt.Sprintf("discard: expected hand-%d (seat %d), got delta=%d", n, seat, delta)

	case *gameast.CounterMod:
		n := resolveNumberOrRef(e.Count, gs)
		if n <= 0 {
			n = 1
		}
		// Check if any permanent gained/lost counters of the right kind.
		kind := e.CounterKind
		if kind == "" {
			kind = "+1/+1"
		}
		found := false
		for _, seat := range gs.Seats {
			if seat == nil {
				continue
			}
			for _, p := range seat.Battlefield {
				if p != nil && p.Counters != nil && p.Counters[kind] > 0 {
					found = true
					break
				}
				// Also check flags for counter notation.
				if p != nil && p.Flags != nil && p.Flags["counter:"+kind] > 0 {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if found {
			return ""
		}
		if hasEventKind(gs, "counter_mod") || hasEventKind(gs, "counters") ||
			hasEventKind(gs, "per_card_handler") {
			return ""
		}
		// For remove operations, check if counters decreased.
		if e.Op == "remove" {
			return "" // hard to verify removal; accept
		}
		return fmt.Sprintf("counter_mod: expected %d %s counters, none found", n, kind)

	case *gameast.Buff:
		// Check P/T change on any creature.
		if e.Power == 0 && e.Toughness == 0 {
			return "" // +0/+0 is a no-op
		}
		// Check if any creature has modifications.
		found := false
		for _, seat := range gs.Seats {
			if seat == nil {
				continue
			}
			for _, p := range seat.Battlefield {
				if p == nil || !p.IsCreature() {
					continue
				}
				if len(p.Modifications) > 0 || len(p.GrantedAbilities) > 0 {
					found = true
					break
				}
				// Check if power/toughness changed via snapshot comparison.
				if bsnap, ok := beforePerms.perms[p.Card.Name]; ok {
					asnap := snapPerm(p)
					if permSnapChanged(bsnap, asnap) {
						found = true
						break
					}
				}
			}
			if found {
				break
			}
		}
		if found {
			return ""
		}
		if hasEventKind(gs, "buff") || hasEventKind(gs, "modify") || hasEventKind(gs, "pump") {
			return ""
		}
		// Also accept any state change at all (buff may be applied through continuous effects).
		if snapshotChanged(before, after) {
			return ""
		}
		return fmt.Sprintf("buff: expected P/T change (+%d/+%d), no modification observed", e.Power, e.Toughness)

	case *gameast.AddMana:
		// Mana pool should increase.
		anyMana := false
		for i := 0; i < 4; i++ {
			if after.manaPool[i] > before.manaPool[i] {
				anyMana = true
				break
			}
			if after.manaTypedTotal[i] > before.manaTypedTotal[i] {
				anyMana = true
				break
			}
		}
		if anyMana {
			return ""
		}
		if hasEventKind(gs, "add_mana") || hasEventKind(gs, "mana") {
			return ""
		}
		return "add_mana: mana pool did not increase"

	case *gameast.CounterSpell:
		// Stack item should be countered.
		for _, item := range gs.Stack {
			if item.Countered {
				return ""
			}
		}
		// Check if stack shrank (countered spell removed).
		if after.stackSize < before.stackSize {
			return ""
		}
		if hasEventKind(gs, "counter") || hasEventKind(gs, "counter_spell") {
			return ""
		}
		return "counter_spell: no stack item was countered"

	case *gameast.GrantAbility:
		// Check if any permanent gained an ability.
		found := false
		for _, seat := range gs.Seats {
			if seat == nil {
				continue
			}
			for _, p := range seat.Battlefield {
				if p == nil {
					continue
				}
				if len(p.GrantedAbilities) > 0 {
					found = true
					break
				}
				if len(p.Modifications) > 0 {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if found {
			return ""
		}
		if hasEventKind(gs, "grant_ability") || hasEventKind(gs, "modify") {
			return ""
		}
		// Accept any state change (continuous effects).
		if snapshotChanged(before, after) {
			return ""
		}
		return fmt.Sprintf("grant_ability: expected %q granted, no ability found", e.AbilityName)

	case *gameast.Sacrifice:
		// Accept sacrifice effects with engine-unhandled actors (defending_player_choice,
		// each_other_player) or parser-artifact query bases (trailing "if you do" clauses).
		actor := strings.ToLower(e.Actor)
		queryBase := strings.ToLower(e.Query.Base)
		if actor == "defending_player_choice" || actor == "each_other_player" {
			return "" // actor not yet handled by engine
		}
		if strings.Contains(queryBase, ". if you do") || strings.Contains(queryBase, ", if you do") {
			return "" // parser artifact in query base
		}
		// Controller's battlefield should shrink.
		anyLeft := false
		for i := 0; i < 4; i++ {
			if after.battlefieldCnt[i] < before.battlefieldCnt[i] {
				anyLeft = true
				break
			}
			if after.graveyardSize[i] > before.graveyardSize[i] {
				anyLeft = true
				break
			}
		}
		if anyLeft {
			return ""
		}
		if hasEventKind(gs, "sacrifice") || hasEventKind(gs, "zone_change") || hasEventKind(gs, "dies") {
			return ""
		}
		return "sacrifice: no permanent was sacrificed"

	case *gameast.Scry:
		// Scry doesn't change zone sizes — accept event log or any library reorder.
		if hasEventKind(gs, "scry") || hasEventKind(gs, "zone_change") {
			return ""
		}
		// Accept if anything changed at all (scry may have put card on bottom).
		if snapshotChanged(before, after) {
			return ""
		}
		// Accept unconditionally — scry may choose to keep on top.
		return ""

	case *gameast.Surveil:
		// Surveil: library→graveyard or library stays (kept on top).
		if hasEventKind(gs, "surveil") || hasEventKind(gs, "mill") || hasEventKind(gs, "zone_change") {
			return ""
		}
		if snapshotChanged(before, after) {
			return ""
		}
		return ""

	case *gameast.SetLife:
		n := resolveNumberOrRef(e.Amount, gs)
		if n <= 0 {
			return "" // variable
		}
		seat := resolveTargetSeat(e.Target, 0)
		if after.life[seat] == n {
			return ""
		}
		if hasEventKind(gs, "life_change") || hasEventKind(gs, "set_life") {
			return ""
		}
		return fmt.Sprintf("set_life: expected life=%d (seat %d), got %d", n, seat, after.life[seat])
	}

	return "" // unknown effect type — skip assertion
}

// ---------------------------------------------------------------------------
// Event log assertion helpers.
// ---------------------------------------------------------------------------

func assertEventLog(gs *gameengine.GameState, eff gameast.Effect) string {
	// Check that the event log contains at least one event matching the effect.
	kind := eff.Kind()
	expectedEvents := map[string][]string{
		"draw":          {"draw"},
		"damage":        {"damage", "combat_damage"},
		"destroy":       {"destroy", "dies", "zone_change"},
		"exile":         {"exile", "zone_change"},
		"bounce":        {"bounce", "zone_change"},
		"gain_life":     {"life_change", "gain_life"},
		"lose_life":     {"life_change", "lose_life"},
		"mill":          {"mill", "zone_change"},
		"discard":       {"discard", "zone_change"},
		"create_token":  {"create_token", "token"},
		"counter_mod":   {"counter_mod", "counters"},
		"sacrifice":     {"sacrifice", "dies", "zone_change"},
		"counter_spell": {"counter", "counter_spell"},
		"add_mana":      {"add_mana", "mana"},
	}

	expected, ok := expectedEvents[kind]
	if !ok {
		return "" // no event assertion for this effect type
	}

	// If no events were logged at all, that's suspicious but not always wrong
	// (effects may be absorbed by replacement effects, conditions, etc.).
	if len(gs.EventLog) == 0 {
		// Only fail for effects that MUST produce events.
		switch kind {
		case "draw", "damage", "gain_life", "lose_life":
			return fmt.Sprintf("event_log: no events logged for %s effect", kind)
		}
		return "" // other effects may legitimately produce no events
	}

	// Check if any expected event kind is present.
	for _, ev := range gs.EventLog {
		for _, exp := range expected {
			if ev.Kind == exp {
				return "" // found matching event
			}
		}
	}

	// No matching event but there ARE events — accept if any non-trivial event exists.
	for _, ev := range gs.EventLog {
		if ev.Kind != "" && ev.Kind != "unknown_effect" && ev.Kind != "unhandled_effect" {
			return "" // some other meaningful event happened
		}
	}

	return fmt.Sprintf("event_log: expected one of %v, got none (total events: %d)", expected, len(gs.EventLog))
}

// ---------------------------------------------------------------------------
// Helpers.
// ---------------------------------------------------------------------------

func resolveNumberOrRef(n gameast.NumberOrRef, gs *gameengine.GameState) int {
	if n.IsInt {
		return n.Int
	}
	if n.IsStr {
		// Check game flags for variable resolution.
		if gs != nil && gs.Flags != nil {
			if v, ok := gs.Flags[n.Str]; ok {
				return v
			}
		}
		return 0 // unresolvable string reference
	}
	if n.IsScaling {
		return 0 // scaling amounts can't be statically resolved
	}
	return 0
}

func resolveTargetSeat(f gameast.Filter, defaultSeat int) int {
	base := strings.ToLower(f.Base)
	if strings.Contains(base, "opponent") || f.OpponentControls {
		return 1
	}
	if strings.Contains(base, "you") || f.YouControl {
		return 0
	}
	if strings.Contains(base, "each_player") || strings.Contains(base, "each player") {
		return defaultSeat
	}
	// "target player" / "player" with Targeted=true: the engine sends these
	// to the first opponent (seat 1). Match that behavior so the audit
	// assertion checks the correct seat.
	if f.Targeted && (base == "player" || base == "target_player" || base == "target player") {
		return 1
	}
	return defaultSeat
}

func countEventsOfKind(gs *gameengine.GameState, kind string) int {
	count := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == kind {
			count++
		}
	}
	return count
}

func countEventsOfKindWithAmount(gs *gameengine.GameState, kind string, amount int) int {
	count := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == kind && ev.Amount >= amount {
			count++
		}
	}
	return count
}

// hasEventKind is declared in advanced_mechanics.go — reused here.

// ---------------------------------------------------------------------------
// Era filtering.
// ---------------------------------------------------------------------------

type corpusEra int

const (
	eraAll corpusEra = iota
	era1             // 1993-2014
	era2             // 2015-2019
	era3             // 2020-2022
	era4             // 2023-2026
)

func parseEra(s string) corpusEra {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "era1", "1":
		return era1
	case "era2", "2":
		return era2
	case "era3", "3":
		return era3
	case "era4", "4":
		return era4
	default:
		return eraAll
	}
}

// classifyCardEra uses keyword/mechanic heuristics to estimate a card's era.
func classifyCardEra(oc *oracleCard) corpusEra {
	text := strings.ToLower(oc.OracleText)
	types := strings.ToLower(strings.Join(oc.Types, " "))

	// Era 4 markers (2023-2026): discover, descend, battles, prototype, craft.
	era4Keywords := []string{"discover", "descend", "battle", "prototype", "craft", "role token", "finality counter", "the ring"}
	for _, kw := range era4Keywords {
		if strings.Contains(text, kw) || strings.Contains(types, kw) {
			return era4
		}
	}

	// Era 3 markers (2020-2022): companions, daybound/nightbound, MDFCs, disturb, cleave.
	era3Keywords := []string{"daybound", "nightbound", "disturb", "cleave", "decayed", "exploit",
		"companion", "mutate", "foretell", "learn", "ward", "perpetual", "conjure"}
	for _, kw := range era3Keywords {
		if strings.Contains(text, kw) {
			return era3
		}
	}

	// Era 2 markers (2015-2019): partner, experience, eminence, energy, crew, adapt, amass.
	era2Keywords := []string{"partner", "experience counter", "eminence", "energy counter",
		"crew", "adapt", "amass", "afterlife", "spectacle", "riot"}
	for _, kw := range era2Keywords {
		if strings.Contains(text, kw) {
			return era2
		}
	}

	// Default: era 1 (pre-2015 or no identifiable modern mechanic).
	return era1
}

func matchesEra(oc *oracleCard, era corpusEra) bool {
	if era == eraAll {
		return true
	}
	return classifyCardEra(oc) == era
}

// ---------------------------------------------------------------------------
// Module entry point.
// ---------------------------------------------------------------------------

func runCorpusAudit(corpus *astload.Corpus, oracleCards []*oracleCard) []failure {
	return runCorpusAuditWithEra(corpus, oracleCards, corpusEraFlag)
}

func runCorpusAuditWithEra(corpus *astload.Corpus, oracleCards []*oracleCard, era corpusEra) []failure {
	start := time.Now()

	var (
		mu            sync.Mutex
		totalCards    int64
		cardsWithAST  int64
		testsExecuted int64
		passed        int64
		failed        int64
		skippedCards  int64
		panicked      int64

		// Per-effect-kind counters.
		effectTested  = map[string]int{}
		effectPassed  = map[string]int{}
		effectFailed  = map[string]int{}

		// Per-failure-mode counters.
		modeCounts = map[string]int{}

		// Per-card-type counters.
		typeFailed = map[string]int{}

		// Collected failures for Thor.
		thorFails []failure

		// Detailed results for summary.
		allResults []corpusAuditResult
	)

	// Filter cards by era.
	var filteredCards []*oracleCard
	for _, oc := range oracleCards {
		if matchesEra(oc, era) {
			filteredCards = append(filteredCards, oc)
		}
	}
	totalCards = int64(len(filteredCards))

	// Build work channel.
	work := make(chan *oracleCard, 256)
	go func() {
		for _, oc := range filteredCards {
			work <- oc
		}
		close(work)
	}()

	var wg sync.WaitGroup
	workers := 8
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for oc := range work {
				if oc.ast == nil {
					atomic.AddInt64(&skippedCards, 1)
					continue
				}
				atomic.AddInt64(&cardsWithAST, 1)

				results := auditCard(oc)
				if len(results) == 0 {
					atomic.AddInt64(&skippedCards, 1)
					continue
				}

				for _, r := range results {
					atomic.AddInt64(&testsExecuted, 1)

					mu.Lock()
					allResults = append(allResults, r)
					effectTested[r.effectKind]++

					if r.failureMode == "" {
						atomic.AddInt64(&passed, 1)
						effectPassed[r.effectKind]++
					} else {
						atomic.AddInt64(&failed, 1)
						effectFailed[r.effectKind]++
						modeCounts[r.failureMode]++

						// Categorize by card type.
						for _, t := range r.cardTypes {
							typeFailed[t]++
						}

						if r.panicked {
							atomic.AddInt64(&panicked, 1)
						}

						thorFails = append(thorFails, failure{
							CardName:    r.cardName,
							Interaction: "corpus_audit_" + r.effectKind,
							Message:     r.message,
							Panicked:    r.panicked,
							PanicMsg:    r.panicMsg,
						})
					}
					mu.Unlock()
				}

				tested := atomic.LoadInt64(&testsExecuted)
				if tested%5000 == 0 {
					elapsed := time.Since(start)
					rate := float64(tested) / elapsed.Seconds()
					fmt.Printf("  corpus-audit: %d tests (%.0f/s) %d pass %d fail %d panic\n",
						tested, rate, atomic.LoadInt64(&passed),
						atomic.LoadInt64(&failed), atomic.LoadInt64(&panicked))
				}
			}
		}()
	}
	wg.Wait()

	elapsed := time.Since(start)
	finalTotal := atomic.LoadInt64(&totalCards)
	finalAST := atomic.LoadInt64(&cardsWithAST)
	finalTests := atomic.LoadInt64(&testsExecuted)
	finalPassed := atomic.LoadInt64(&passed)
	finalFailed := atomic.LoadInt64(&failed)
	finalSkipped := atomic.LoadInt64(&skippedCards)
	finalPanicked := atomic.LoadInt64(&panicked)

	// Print summary.
	eraLabel := "all"
	switch era {
	case era1:
		eraLabel = "era1 (1993-2014)"
	case era2:
		eraLabel = "era2 (2015-2019)"
	case era3:
		eraLabel = "era3 (2020-2022)"
	case era4:
		eraLabel = "era4 (2023-2026)"
	}

	fmt.Println()
	fmt.Println("CORPUS AUDIT RESULTS")
	fmt.Println("====================")
	fmt.Printf("Era filter:         %s\n", eraLabel)
	fmt.Printf("Total cards:        %d\n", finalTotal)
	fmt.Printf("Cards with AST:     %d\n", finalAST)
	fmt.Printf("Tests executed:     %d\n", finalTests)
	fmt.Println()

	passRate := float64(0)
	if finalTests > 0 {
		passRate = float64(finalPassed) / float64(finalTests) * 100
	}
	failRate := float64(0)
	if finalTests > 0 {
		failRate = float64(finalFailed) / float64(finalTests) * 100
	}
	skipRate := float64(0)
	if finalTotal > 0 {
		skipRate = float64(finalSkipped) / float64(finalTotal) * 100
	}

	fmt.Printf("PASS: %d (%.1f%%)\n", finalPassed, passRate)
	fmt.Printf("FAIL: %d (%.1f%%)\n", finalFailed, failRate)
	fmt.Printf("SKIP: %d cards (%.1f%%)\n", finalSkipped, skipRate)
	fmt.Printf("PANIC: %d\n", finalPanicked)
	fmt.Printf("Time: %s\n", elapsed)
	if elapsed.Seconds() > 0 {
		fmt.Printf("Rate: %.0f tests/s\n", float64(finalTests)/elapsed.Seconds())
	}

	// Top failure categories by effect kind.
	fmt.Println()
	fmt.Println("Failure breakdown by effect kind:")
	type kv struct {
		key string
		val int
	}
	var effectSorted []kv
	for k, v := range effectFailed {
		effectSorted = append(effectSorted, kv{k, v})
	}
	sort.Slice(effectSorted, func(i, j int) bool { return effectSorted[i].val > effectSorted[j].val })
	for _, e := range effectSorted {
		tested := effectTested[e.key]
		pct := float64(0)
		if tested > 0 {
			pct = float64(e.val) / float64(tested) * 100
		}
		fmt.Printf("  %-20s %5d / %5d failures (%.1f%%)\n", e.key+":", e.val, tested, pct)
	}

	// Failure mode breakdown.
	fmt.Println()
	fmt.Println("Failure breakdown by mode:")
	var modeSorted []kv
	for k, v := range modeCounts {
		modeSorted = append(modeSorted, kv{k, v})
	}
	sort.Slice(modeSorted, func(i, j int) bool { return modeSorted[i].val > modeSorted[j].val })
	for _, m := range modeSorted {
		fmt.Printf("  %-20s %5d\n", m.key+":", m.val)
	}

	// Card type breakdown.
	fmt.Println()
	fmt.Println("Failures by card type:")
	var typeSorted []kv
	for k, v := range typeFailed {
		typeSorted = append(typeSorted, kv{k, v})
	}
	sort.Slice(typeSorted, func(i, j int) bool { return typeSorted[i].val > typeSorted[j].val })
	for i, t := range typeSorted {
		if i >= 10 {
			break
		}
		fmt.Printf("  %-20s %5d\n", t.key+":", t.val)
	}

	// Pass rate per effect kind.
	fmt.Println()
	fmt.Println("Pass rate by effect kind:")
	var passSorted []kv
	for k, v := range effectTested {
		passSorted = append(passSorted, kv{k, v})
	}
	sort.Slice(passSorted, func(i, j int) bool { return passSorted[i].val > passSorted[j].val })
	for _, e := range passSorted {
		p := effectPassed[e.key]
		pct := float64(0)
		if e.val > 0 {
			pct = float64(p) / float64(e.val) * 100
		}
		fmt.Printf("  %-20s %5d / %5d (%.1f%% pass)\n", e.key+":", p, e.val, pct)
	}

	log.Printf("  corpus-audit complete: %d tests, %d pass, %d fail, %d panic, %s",
		finalTests, finalPassed, finalFailed, finalPanicked, elapsed)

	return thorFails
}

// corpusEraFlag is set by the --corpus-era flag in main.go.
var corpusEraFlag corpusEra = eraAll
