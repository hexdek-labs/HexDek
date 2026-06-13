// Package progression is the PROGRESSION dimension of the Hex Judge
// (r63 phase 1): trigger-correctness checking, independent of the
// engine's trigger dispatch.
//
// The engine decides which triggered abilities fire by walking its own
// dispatch machinery; this package re-derives the EXPECTED firings from
// first principles — the AST trigger condition + the stimulus the
// scenario itself applied — and observes the ACTUAL firings purely as
// STATE DELTAS (composing on the OUTCOME dimension's interpreter: a
// trigger firing is observable as its effect's expected delta; a
// missed trigger is a zero delta; a double-fire is a 2× delta; a
// phantom fire is a non-zero delta where none belongs). No engine
// trigger code is consulted to form expectations, and no event-log
// introspection is needed for checks 1-3 — the one event-stream-based
// assertion is the APNAP ordering scenario (check 4), documented below.
//
// Phase-1 trigger vocabulary (bare conditions only — no actor /
// target_filter / condition / phase / controller / intervening-if):
//
//	etb    "when this enters"          — 3,970 corpus triggers
//	attack "whenever this attacks"     — 1,041
//	die    "when this dies"            —   642
//
// For each in-scope (card, trigger) the harness runs:
//
//	FIRE     stimulus matches  → delta must equal the effect's expected
//	         delta exactly once (wrong amount OR double-fire both fail)
//	PHANTOM  stimulus does NOT match (a vanilla bystander enters / dies
//	         / attacks instead) → delta must be exactly zero
package progression

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/judge/outcome"
)

// Finding is one trigger-correctness divergence.
type Finding struct {
	CardName string
	Event    string // etb / die / attack
	Check    string // "fire" / "phantom"
	Expected string
	Actual   string
	Raw      string
}

// InScopeTrigger reports whether a Triggered ability is in the phase-1
// vocabulary and returns its normalized event name.
func InScopeTrigger(t *gameast.Triggered) (string, bool) {
	if t == nil || t.Effect == nil || t.InterveningIf != nil {
		return "", false
	}
	tr := t.Trigger
	if tr.Actor != nil || tr.TargetFilter != nil || tr.Condition != nil ||
		tr.Phase != "" || tr.Controller != "" {
		return "", false
	}
	raw := strings.ToLower(t.Raw)
	switch tr.Event {
	case "etb":
		// Bare etb reliably means "when this enters" in the corpus.
		return "etb", true
	case "die", "dies":
		// The parser drops actor filters ("whenever one or more OTHER
		// creatures you control die" parses to a bare die event —
		// Vengeful Townsfolk, first-audit parser finding). Phase 1
		// keeps only explicit self-death raws.
		if strings.Contains(raw, "other") || strings.Contains(raw, "another") ||
			strings.Contains(raw, "you control") {
			return "", false
		}
		// Attached-die wordings ("when enchanted creature dies, return
		// this card…", Angelic Destiny class) observe the HOST's death,
		// not the bearer's — the §603.10 look-back item, out of scope
		// (phase-3b false-positive fix: the bare scenario killed the
		// AURA and expected its host-death effect).
		if strings.Contains(raw, "enchanted") || strings.Contains(raw, "equipped") {
			return "", false
		}
		// Bolster's "your creature with the least toughness" restriction
		// is dropped by the parser (CounterMod with a bare creature
		// target) — first-audit parser-fidelity finding (Abzan
		// Skycaptain). Out of phase-1 scope.
		if strings.Contains(raw, "bolster") {
			return "", false
		}
		return "die", true
	case "attack", "attacks":
		// Same parser gap on attack actors ("a creature you control
		// with haste attacks" → bare attack — Ognis / Elite Scaleguard
		// / Beastmaster class). Self-attack raws only.
		if !strings.Contains(raw, "this creature attacks") &&
			!strings.Contains(raw, "~ attacks") &&
			!strings.Contains(raw, "this attacks") {
			return "", false
		}
		return "attack", true
	}
	return "", false
}

// progressionSpec is the deterministic scenario board: the bearer will
// be the ONLY creature its controller has (attack determinism), with a
// defender present.
func progressionSpec() outcome.BoardSpec {
	return outcome.BoardSpec{
		Controller: 0, Opponent: 1,
		OppCreatures: 1, OppArtifacts: 1, OppEnchants: 1, OppLands: 1,
		OwnCreatures: 0, SrcIsCreature: true, LibrarySize: 10,
		OppTappedArtifacts: 1, OwnTappedLands: 1, HandSize: 5, XValue: 3,
	}
}

func vanillaPerm(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	c := &gameengine.Card{
		Name: "Vanilla Bystander", Owner: seat,
		Types:     []string{"creature"},
		BasePower: 1, BaseToughness: 1,
	}
	return &gameengine.Permanent{
		Card: c, Controller: seat, Owner: seat,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
}

func place(gs *gameengine.GameState, p *gameengine.Permanent) {
	gs.Seats[p.Controller].Battlefield = append(gs.Seats[p.Controller].Battlefield, p)
}

// expectedFireSet returns the effect's expected delta set, or ok=false
// when the effect is outside the OUTCOME interpreter's scope or its
// delta is unobservable (empty).
func expectedFireSet(spec outcome.BoardSpec, eff gameast.Effect) ([]*outcome.Delta, bool) {
	set, ok := outcome.ExpectSet(spec, eff)
	if !ok {
		return nil, false
	}
	// All-empty sets are unobservable — a fire and a miss look the same.
	observable := false
	empty := outcome.NewDelta()
	for _, d := range set {
		if !d.Equal(empty) {
			observable = true
			break
		}
	}
	if !observable {
		return nil, false
	}
	return set, true
}

// CheckTrigger runs the FIRE and PHANTOM scenarios for one in-scope
// (card, trigger) pair. Returns findings (nil = pass) and ran=false if
// the pair is out of scope.
func CheckTrigger(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	if ev, ok := InScopeTrigger(t); ok {
		switch ev {
		case "etb":
			if perCardOwned(cardName, "etb", "permanent_etb") {
				return nil, false
			}
		case "attack":
			if perCardOwned(cardName, "creature_attacks") {
				return nil, false
			}
		case "die":
			if perCardOwned(cardName, "creature_dies", "dies") {
				return nil, false
			}
		}
	}
	event, ok := InScopeTrigger(t)
	if !ok {
		return nil, false
	}
	spec := progressionSpec()
	// Die-triggers resolve AFTER the bearer has left the battlefield
	// (CR §603.10 look-back fires the trigger, but the effect sees the
	// post-death board) — their expectations are computed against a
	// spec without the bearer-as-source creature.
	effSpec := spec
	if event == "die" {
		effSpec.SrcIsCreature = false
	}
	expectedSet, ok := expectedFireSet(effSpec, t.Effect)
	if !ok {
		return nil, false
	}

	var findings []*Finding
	check := func(checkName string, actual *outcome.Delta, wantFire bool) {
		if wantFire {
			for _, exp := range expectedSet {
				if actual.Equal(exp) {
					return
				}
			}
			findings = append(findings, &Finding{
				CardName: cardName, Event: event, Check: checkName,
				Expected: describeSet(expectedSet), Actual: actual.String(), Raw: t.Raw,
			})
			return
		}
		if !actual.Equal(outcome.NewDelta()) {
			findings = append(findings, &Finding{
				CardName: cardName, Event: event, Check: checkName,
				Expected: "no change (condition not met)", Actual: actual.String(), Raw: t.Raw,
			})
		}
	}

	switch event {
	case "etb":
		// FIRE: bearer enters; snapshot AFTER placement isolates the
		// pure trigger delta.
		gs, bearer := outcome.BuildBoardForSpec(spec, cardName)
		bearer.Card.AST = wrapSingle(t)
		before := outcome.Snap(gs)
		gameengine.FirePermanentETBTriggers(gs, bearer)
		check("fire", outcome.DiffSnapshots(before, outcome.Snap(gs)), true)

		// PHANTOM: bearer already on the battlefield; a VANILLA
		// bystander enters — "when THIS enters" must stay silent.
		gs2, bearer2 := outcome.BuildBoardForSpec(spec, cardName)
		bearer2.Card.AST = wrapSingle(t)
		v := vanillaPerm(gs2, 0)
		place(gs2, v)
		before2 := outcome.Snap(gs2)
		gameengine.FirePermanentETBTriggers(gs2, v)
		check("phantom", outcome.DiffSnapshots(before2, outcome.Snap(gs2)), false)

	case "die":
		// FIRE: the bearer dies. The death movement itself (bf-1, gy+1,
		// P/T sums) is part of the expected delta — compose it.
		gs, bearer := outcome.BuildBoardForSpec(spec, cardName)
		bearer.Card.AST = wrapSingle(t)
		before := outcome.Snap(gs)
		gameengine.DestroyPermanent(gs, bearer, nil)
		actual := outcome.DiffSnapshots(before, outcome.Snap(gs))
		subtractDeath(actual, 0, 4)
		check("fire", actual, true)

		// PHANTOM: a vanilla bystander dies instead.
		gs2, bearer2 := outcome.BuildBoardForSpec(spec, cardName)
		bearer2.Card.AST = wrapSingle(t)
		v := vanillaPerm(gs2, 0)
		place(gs2, v)
		before2 := outcome.Snap(gs2)
		gameengine.DestroyPermanent(gs2, v, nil)
		actual2 := outcome.DiffSnapshots(before2, outcome.Snap(gs2))
		subtractDeath(actual2, 0, 1)
		check("phantom", actual2, false)

	case "attack":
		// FIRE: the bearer (its controller's only creature) attacks.
		gs, bearer := outcome.BuildBoardForSpec(spec, cardName)
		bearer.Card.AST = wrapSingle(t)
		bearer.SummoningSick = false
		wasTapped := bearer.Tapped
		before := outcome.Snap(gs)
		gameengine.DeclareAttackers(gs, 0)
		actual := outcome.DiffSnapshots(before, outcome.Snap(gs))
		// Subtract ONLY the bearer's own attack-declaration tap — a
		// global newly-tapped count would swallow the trigger's own tap
		// effects (Fiend Binder "tap target creature" class, the
		// first-audit harness bug).
		if !wasTapped && bearer.Tapped {
			subtractTap(actual, 1)
		}
		check("fire", actual, true)

		// PHANTOM: a vanilla creature attacks; the bearer stays home
		// (summoning-sick, so the attack policy cannot declare it).
		gs2, bearer2 := outcome.BuildBoardForSpec(spec, cardName)
		bearer2.Card.AST = wrapSingle(t)
		bearer2.SummoningSick = true
		v := vanillaPerm(gs2, 0)
		v.SummoningSick = false
		place(gs2, v)
		vWasTapped := v.Tapped
		before2 := outcome.Snap(gs2)
		gameengine.DeclareAttackers(gs2, 0)
		actual2 := outcome.DiffSnapshots(before2, outcome.Snap(gs2))
		if !vWasTapped && v.Tapped {
			subtractTap(actual2, 1)
		}
		check("phantom", actual2, false)
	}
	emitAll(findings) // origin tap — Judge router
	return findings, true
}

// wrapSingle builds a minimal CardAST holding exactly the trigger under
// test, so unrelated abilities on the same card cannot pollute the
// scenario delta.
func wrapSingle(t *gameast.Triggered) *gameast.CardAST {
	return &gameast.CardAST{Abilities: []gameast.Ability{t}}
}

// subtractDeath removes the bare death movement of a `pt`-bodied
// creature owned by `seat` from the delta, leaving only trigger effects.
func subtractDeath(d *outcome.Delta, seat, pt int) {
	d.BattlefieldBySeat[seat]++
	if d.BattlefieldBySeat[seat] == 0 {
		delete(d.BattlefieldBySeat, seat)
	}
	d.GraveyardBySeat[seat]--
	if d.GraveyardBySeat[seat] == 0 {
		delete(d.GraveyardBySeat, seat)
	}
	d.PowerSum += pt
	d.ToughSum += pt
}

// subtractTap removes the attack-declaration taps from the delta.
func subtractTap(d *outcome.Delta, n int) {
	d.Tapped -= n
}

func describeSet(set []*outcome.Delta) string {
	out := ""
	for i, d := range set {
		if i > 0 {
			out += " | "
		}
		out += d.String()
	}
	return out
}
