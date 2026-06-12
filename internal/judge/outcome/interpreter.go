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
		!eqMap(d.ExileBySeat, o.ExileBySeat) || !eqMap(d.BattlefieldBySeat, o.BattlefieldBySeat) {
		return false
	}
	if d.MarkedDamage != o.MarkedDamage {
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
	return fmt.Sprintf("life=%v hand=%v lib=%v gy=%v exile=%v bf=%v dmg=%d counters=%v",
		d.LifeBySeat, d.HandBySeat, d.LibraryBySeat, d.GraveyardBySeat,
		d.ExileBySeat, d.BattlefieldBySeat, d.MarkedDamage, d.CountersByKind)
}

// BoardSpec describes the synthetic board the harness builds, so the
// interpreter can derive candidate existence without consulting engine
// internals. Counts are permanents per category on the OPPONENT seat;
// the source permanent sits on the controller's battlefield.
type BoardSpec struct {
	Controller   int
	Opponent     int
	OppCreatures int
	OppArtifacts int
	OppEnchants  int
	OppLands     int
	OwnCreatures int // bystanders beside the source
	SrcIsCreature bool
	LibrarySize  int // per seat
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
		n, ok := e.Count.IntVal()
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
		n, ok := e.Amount.IntVal()
		if !ok || n <= 0 || !filterIsSelfPlayer(e.Target) {
			return false
		}
		d.LifeBySeat[spec.Controller] += n
		return true

	case *gameast.LoseLife:
		n, ok := e.Amount.IntVal()
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
		n, ok := e.Amount.IntVal()
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
		n, ok := e.Count.IntVal()
		if !ok || n <= 0 || e.IsCopyOf != nil {
			return false
		}
		d.BattlefieldBySeat[spec.Controller] += n
		return true

	case *gameast.Destroy:
		exp, ok := singleRemovalExpectation(spec, e.Target)
		if !ok {
			return false
		}
		if exp {
			d.BattlefieldBySeat[spec.Opponent]--
			d.GraveyardBySeat[spec.Opponent]++
		}
		return true

	case *gameast.Exile:
		if e.Until != "" {
			return false // linked-return shapes: phase 2
		}
		exp, ok := singleRemovalExpectation(spec, e.Target)
		if !ok {
			return false
		}
		if exp {
			d.BattlefieldBySeat[spec.Opponent]--
			d.ExileBySeat[spec.Opponent]++
		}
		return true

	case *gameast.CounterMod:
		if e.Op != "put" && e.Op != "" {
			return false
		}
		n, ok := e.Count.IntVal()
		if !ok || n <= 0 {
			return false
		}
		kind := e.CounterKind
		if kind == "" {
			kind = "+1/+1"
		}
		switch normBase(e.Target.Base) {
		case "self", "it", "this", "":
			d.CountersByKind[kind] += n
			return true
		case "creature":
			if e.Target.Quantifier == "each" || e.Target.Quantifier == "all" {
				return false // fan-out: phase 2
			}
			if spec.OppCreatures+spec.OwnCreatures == 0 && !spec.SrcIsCreature {
				return false
			}
			d.CountersByKind[kind] += n
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
		candidates = spec.OppArtifacts
	case "enchantment":
		candidates = spec.OppEnchants
	case "land":
		candidates = spec.OppLands
	case "permanent":
		candidates = spec.OppCreatures + spec.OppArtifacts + spec.OppEnchants + spec.OppLands
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
