// Package outcome is the OUTCOME dimension of the Hex Judge (r63
// phase 1): an AST-derived, engine-independent interpreter of expected
// post-resolution state deltas.
//
// The engine resolves a card's AST through resolve.go; this package
// re-interprets the SAME AST into an expected delta — what SHOULD
// change (zones, counts, life, damage, counters) — without touching the
// engine's execution path. The harness (harness.go) snapshots a game
// state, lets the engine resolve, diffs, and asserts actual == expected.
// Divergence means an engine bug or a parser bug: parity against
// INTENT, per effect, at scale. Goldilocks checks something-changed;
// this checks the-RIGHT-thing-changed.
//
// Phase 1 covers a tractable subset where the expected delta is
// unambiguous (strict whitelist; everything else returns ok=false and
// is skipped, never guessed):
//
//	draw N (self) · deal N damage (single target) · gain/lose N life
//	(self / each opponent) · create N tokens · destroy/exile single
//	target · put N +1/+1 (or named) counters
//
// Expectations are AGGREGATES (total life lost, total marked damage,
// per-seat zone counts) so they hold regardless of WHICH legal target
// the engine's policy picks — target identity is policy, totals are
// rules.
package outcome

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Delta is an aggregate expected/actual state change. Maps are sparse:
// a missing key means "expected unchanged" and the comparator treats
// missing == 0.
type Delta struct {
	LifeBySeat        map[int]int // life total change per seat
	HandBySeat        map[int]int
	LibraryBySeat     map[int]int
	GraveyardBySeat   map[int]int
	ExileBySeat       map[int]int
	BattlefieldBySeat map[int]int // permanent COUNT change per seat
	MarkedDamage      int         // total marked damage added across all permanents
	CountersByKind    map[string]int
	// r63 part-2 widened dimensions:
	Tapped   int // change in total tapped-permanent count
	PowerSum int // change in summed effective power across battlefields
	ToughSum int // change in summed effective toughness
	// r63 part-4 widened dimensions:
	PoolBySeat map[int]int // mana-pool total change per seat
	LiveStack  int         // change in count of non-countered stack items
}

func NewDelta() *Delta {
	return &Delta{
		LifeBySeat:        map[int]int{},
		HandBySeat:        map[int]int{},
		LibraryBySeat:     map[int]int{},
		GraveyardBySeat:   map[int]int{},
		ExileBySeat:       map[int]int{},
		BattlefieldBySeat: map[int]int{},
		CountersByKind:    map[string]int{},
		PoolBySeat:        map[int]int{},
	}
}

// Equal reports whether two deltas describe the same aggregate change.
func (d *Delta) Equal(o *Delta) bool {
	eqMap := func(a, b map[int]int) bool {
		for k, v := range a {
			if b[k] != v {
				return false
			}
		}
		for k, v := range b {
			if a[k] != v {
				return false
			}
		}
		return true
	}
	if !eqMap(d.LifeBySeat, o.LifeBySeat) || !eqMap(d.HandBySeat, o.HandBySeat) ||
		!eqMap(d.LibraryBySeat, o.LibraryBySeat) || !eqMap(d.GraveyardBySeat, o.GraveyardBySeat) ||
		!eqMap(d.ExileBySeat, o.ExileBySeat) || !eqMap(d.BattlefieldBySeat, o.BattlefieldBySeat) ||
		!eqMap(d.PoolBySeat, o.PoolBySeat) {
		return false
	}
	if d.LiveStack != o.LiveStack {
		return false
	}
	if d.MarkedDamage != o.MarkedDamage {
		return false
	}
	if d.Tapped != o.Tapped || d.PowerSum != o.PowerSum || d.ToughSum != o.ToughSum {
		return false
	}
	for k, v := range d.CountersByKind {
		if o.CountersByKind[k] != v {
			return false
		}
	}
	for k, v := range o.CountersByKind {
		if d.CountersByKind[k] != v {
			return false
		}
	}
	return true
}

func (d *Delta) String() string {
	return fmt.Sprintf("life=%v hand=%v lib=%v gy=%v exile=%v bf=%v dmg=%d counters=%v tapped=%d pow=%d tough=%d pool=%v stack=%d",
		d.LifeBySeat, d.HandBySeat, d.LibraryBySeat, d.GraveyardBySeat,
		d.ExileBySeat, d.BattlefieldBySeat, d.MarkedDamage, d.CountersByKind,
		d.Tapped, d.PowerSum, d.ToughSum, d.PoolBySeat, d.LiveStack)
}

// scaffoldCreaturePT is the printed power=toughness of every creature
// the harness places (4/4 keeps phase-1 damage sublethal-observable and
// makes removal P/T deltas deterministic).
const scaffoldCreaturePT = 4

// BoardSpec describes the synthetic board the harness builds, so the
// interpreter can derive candidate existence without consulting engine
// internals. Counts are permanents per category on the OPPONENT seat;
// the source permanent sits on the controller's battlefield.
type BoardSpec struct {
	Controller    int
	Opponent      int
	OppCreatures  int
	OppArtifacts  int
	OppEnchants   int
	OppLands      int
	OwnCreatures  int // bystanders beside the source
	SrcIsCreature bool
	LibrarySize   int // per seat
	// r63 part-2 board additions:
	OppTappedArtifacts int // tapped artifacts on the opponent (untap candidates)
	OwnTappedLands     int // tapped lands on the controller (untap candidates)
	HandSize           int // cards in each seat's hand (discard candidates)
	XValue             int // pinned value for {X} amounts (gs.Flags["x"])
	// r63 part-4 board additions:
	LibraryLands       int // basic land cards inside each LibrarySize (tutor candidates)
	GraveyardCreatures int // 4/4 creature cards in each seat's graveyard (reanimate candidates)
	StackSpells        int // opponent-controlled instant spells on the stack (counter candidates)
}

// Expect computes the expected aggregate delta for `eff` resolved by
// `controller`'s source on the BoardSpec board. ok=false means the
// effect is out of phase-1 scope and must be skipped (never guessed).
func Expect(spec BoardSpec, eff gameast.Effect) (*Delta, bool) {
	d := NewDelta()
	if !accumulate(spec, eff, d) {
		return nil, false
	}
	return d, true
}

// accumulate folds one effect (recursing through Sequence) into d.
func accumulate(spec BoardSpec, eff gameast.Effect, d *Delta) bool {
	switch e := eff.(type) {
	case *gameast.Sequence:
		for _, item := range e.Items {
			if !accumulate(spec, item, d) {
				return false
			}
		}
		return true

	case *gameast.Draw:
		n, ok := amountVal(spec, e.Count)
		if !ok || n <= 0 || n > spec.LibrarySize {
			return false
		}
		if !filterIsSelfPlayer(e.Target) {
			return false
		}
		d.HandBySeat[spec.Controller] += n
		d.LibraryBySeat[spec.Controller] -= n
		return true

	case *gameast.GainLife:
		n, ok := amountVal(spec, e.Amount)
		if !ok || n <= 0 || !filterIsSelfPlayer(e.Target) {
			return false
		}
		d.LifeBySeat[spec.Controller] += n
		return true

	case *gameast.LoseLife:
		n, ok := amountVal(spec, e.Amount)
		if !ok || n <= 0 {
			return false
		}
		switch normBase(e.Target.Base) {
		case "self", "you", "", "controller":
			d.LifeBySeat[spec.Controller] -= n
			return true
		case "each_opponent", "each opponent":
			d.LifeBySeat[spec.Opponent] -= n
			return true
		case "player", "each_player":
			// "each player loses N" includes the CONTROLLER (first
			// corpus audit: 24 over-claims where the interpreter
			// expected opponent-only — Blood-Toll Harpy class).
			if e.Target.Quantifier == "each" || e.Target.Quantifier == "all" {
				d.LifeBySeat[spec.Controller] -= n
				d.LifeBySeat[spec.Opponent] -= n
				return true
			}
			// Contextual single-player references ("that player",
			// "defending player", policy-picked "target player") are
			// unresolvable standalone — out of phase-1 scope (the
			// engine context-defaults them; Liliana's Caress class).
			return false
		}
		return false

	case *gameast.Damage:
		n, ok := amountVal(spec, e.Amount)
		if !ok || n <= 0 || e.Divided {
			return false
		}
		q := e.Target.Quantifier
		if q == "each" || q == "all" || q == "each_player" {
			return false // fan-out shapes: phase 2
		}
		if len(e.Target.Extra) > 0 || len(e.Target.CreatureTypes) > 0 ||
			len(e.Target.ColorFilter) > 0 {
			// Combat/subtype/color-constrained targets (attacking or
			// blocking…) may have no scaffold candidate — out of scope
			// (first corpus audit: 11 over-claims, Iron Verdict class).
			return false
		}
		switch normBase(e.Target.Base) {
		case "each_opponent":
			d.LifeBySeat[spec.Opponent] -= n
			return true
		case "player", "opponent", "target_player", "target player":
			// Single player target: the engine policy may pick either
			// seat in principle; phase-1 pins ONLY the aggregate "total
			// life lost == n" via a relaxed claim: on the 2-seat board
			// every observed engine pick is the opponent, and the unit
			// suite pins that. Contextual refs bail above via LoseLife;
			// damage "target player" is policy-targeted, not contextual.
			d.LifeBySeat[spec.Opponent] -= n
			return true
		case "creature":
			// Single creature target — aggregate: marked damage == n,
			// provided a creature candidate exists.
			if spec.OppCreatures+spec.OwnCreatures == 0 && !spec.SrcIsCreature {
				return false
			}
			// Below lethal-for-scaffold threshold only: phase 1 boards
			// use 4/4 creatures and never run SBAs mid-check, so marked
			// damage is fully observable.
			d.MarkedDamage += n
			return true
		case "any", "any_target", "any target", "":
			// Could hit a player or a creature: out of aggregate scope
			// unless we pin totals across both pools — phase 2.
			return false
		}
		return false

	case *gameast.CreateToken:
		n, ok := amountVal(spec, e.Count)
		if !ok || n <= 0 || e.IsCopyOf != nil {
			return false
		}
		d.BattlefieldBySeat[spec.Controller] += n
		if e.Tapped {
			d.Tapped += n
		}
		// Effective-P/T side effect: creature tokens contribute their
		// printed body to the board P/T sums.
		if e.PT != nil {
			d.PowerSum += e.PT[0] * n
			d.ToughSum += e.PT[1] * n
		} else {
			for _, t := range e.Types {
				if t == "creature" {
					return false // creature token with unknown body
				}
			}
		}
		return true

	case *gameast.Destroy:
		k, tappedHit, ok := removalCount(spec, e.Target)
		if !ok {
			return false
		}
		d.BattlefieldBySeat[spec.Opponent] -= k
		d.GraveyardBySeat[spec.Opponent] += k
		d.Tapped -= tappedHit
		if normBase(e.Target.Base) == "creature" {
			d.PowerSum -= scaffoldCreaturePT * k
			d.ToughSum -= scaffoldCreaturePT * k
		}
		return true

	case *gameast.Exile:
		if e.Until != "" {
			return false // linked-return shapes: phase 2
		}
		k, tappedHit, ok := removalCount(spec, e.Target)
		if !ok {
			return false
		}
		d.BattlefieldBySeat[spec.Opponent] -= k
		d.ExileBySeat[spec.Opponent] += k
		d.Tapped -= tappedHit
		if normBase(e.Target.Base) == "creature" {
			d.PowerSum -= scaffoldCreaturePT * k
			d.ToughSum -= scaffoldCreaturePT * k
		}
		return true

	case *gameast.CounterMod:
		if e.Op != "put" && e.Op != "" {
			return false
		}
		n, ok := amountVal(spec, e.Count)
		if !ok || n <= 0 {
			return false
		}
		kind := e.CounterKind
		if kind == "" {
			kind = "+1/+1"
		}
		ptShift := 0
		switch kind {
		case "+1/+1":
			ptShift = n
		case "-1/-1":
			ptShift = -n
		}
		switch normBase(e.Target.Base) {
		case "self", "it", "this", "":
			d.CountersByKind[kind] += n
			if spec.SrcIsCreature {
				d.PowerSum += ptShift
				d.ToughSum += ptShift
			}
			return true
		case "creature":
			if e.Target.Quantifier == "each" || e.Target.Quantifier == "all" {
				return false // fan-out: phase 2
			}
			if spec.OppCreatures+spec.OwnCreatures == 0 && !spec.SrcIsCreature {
				return false
			}
			d.CountersByKind[kind] += n
			d.PowerSum += ptShift
			d.ToughSum += ptShift
			return true
		}
		return false
	}
	return false
}

// singleRemovalExpectation decides whether a single-target removal on
// the scaffold should remove exactly one OPPONENT permanent. ok=false
// = out of scope. The harness board places the only legal candidates
// for these bases on the opponent (own side holds only the source +
// own-creature bystanders, and the engine's policy never self-targets
// removal when an opponent candidate exists — pinned by unit test).
func singleRemovalExpectation(spec BoardSpec, f gameast.Filter) (removesOne bool, ok bool) {
	if f.Quantifier == "each" || f.Quantifier == "all" {
		return false, false // mass removal: phase 2
	}
	if f.YouControl {
		return false, false
	}
	var candidates int
	switch normBase(f.Base) {
	case "creature":
		candidates = spec.OppCreatures
	case "artifact":
		candidates = spec.OppArtifacts + spec.OppTappedArtifacts
	case "enchantment":
		candidates = spec.OppEnchants
	case "land":
		candidates = spec.OppLands
	case "permanent":
		// The policy may pick a creature (body leaves the P/T sums) or a
		// noncreature (no P/T change) — set-valued in principle; out of
		// scope in the single-delta accumulator.
		return false, false
	default:
		return false, false
	}
	if len(f.CreatureTypes) > 0 || len(f.ColorFilter) > 0 || len(f.Extra) > 0 ||
		f.ManaValueOp != "" || f.NonToken {
		// Restricted filters may or may not match scaffold permanents —
		// out of phase-1 scope rather than guessed.
		return false, false
	}
	return candidates > 0, true
}

func filterIsSelfPlayer(f gameast.Filter) bool {
	switch normBase(f.Base) {
	case "", "self", "you", "controller":
		return true
	}
	return false
}

func normBase(b string) string {
	for len(b) > 0 && (b[0] == ' ') {
		b = b[1:]
	}
	return b
}

// removalCount generalizes singleRemovalExpectation to up_to_n picks:
// the engine removes min(count, opponent candidates) — harmful up-to
// never crosses to own permanents post-#1052. tappedHit counts how many
// removed permanents were tapped (the artifact pool holds one tapped
// relic that goes when the untapped pool runs out), so the Tapped
// dimension stays balanced.
func removalCount(spec BoardSpec, f gameast.Filter) (k int, tappedHit int, ok bool) {
	removes, sok := singleRemovalExpectation(spec, f)
	if !sok {
		return 0, 0, false
	}
	if !removes {
		return 0, 0, true
	}
	want := 1
	if f.Quantifier == "up_to_n" || f.Quantifier == "n" {
		if f.Count != nil {
			if v, vok := f.Count.IntVal(); vok && v > 0 {
				want = v
			}
		}
	}
	var pool, tappedPool int
	switch normBase(f.Base) {
	case "creature":
		pool = spec.OppCreatures
	case "artifact":
		pool = spec.OppArtifacts + spec.OppTappedArtifacts
		tappedPool = spec.OppTappedArtifacts
	case "enchantment":
		pool = spec.OppEnchants
	case "land":
		pool = spec.OppLands
	default:
		return 0, 0, false
	}
	if want > pool {
		want = pool
	}
	untapped := pool - tappedPool
	if want > untapped {
		tappedHit = want - untapped
	}
	return want, tappedHit, true
}
