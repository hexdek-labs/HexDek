package outcome

import (
	"fmt"
	"math/rand"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// harness.go — the goldilocks-style runner for the OUTCOME dimension:
// build a minimal deterministic board, snapshot, let the ENGINE resolve
// the effect, diff, compare against the interpreter's expected delta.

// Finding is one expected-vs-actual divergence.
type Finding struct {
	CardName string
	Kind     string // effect kind
	Expected string
	Actual   string
	Raw      string // raw oracle clause when available
}

// snapshot captures the aggregate observable state.
type snapshot struct {
	life, hand, lib, gy, exile, bf []int
	markedDamage                   int
	counters                       map[string]int
	tapped, powerSum, toughSum     int
}

func snap(gs *gameengine.GameState) snapshot {
	n := len(gs.Seats)
	s := snapshot{
		life: make([]int, n), hand: make([]int, n), lib: make([]int, n),
		gy: make([]int, n), exile: make([]int, n), bf: make([]int, n),
		counters: map[string]int{},
	}
	for i, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		s.life[i] = seat.Life
		s.hand[i] = len(seat.Hand)
		s.lib[i] = len(seat.Library)
		s.gy[i] = len(seat.Graveyard)
		s.exile[i] = len(seat.Exile)
		s.bf[i] = len(seat.Battlefield)
		for _, p := range seat.Battlefield {
			if p == nil {
				continue
			}
			s.markedDamage += p.MarkedDamage
			if p.Tapped {
				s.tapped++
			}
			s.powerSum += p.Power()
			s.toughSum += p.Toughness()
			for k, v := range p.Counters {
				s.counters[k] += v
			}
		}
	}
	return s
}

// diff computes the actual Delta between two snapshots.
func diff(before, after snapshot) *Delta {
	d := NewDelta()
	for i := range before.life {
		if v := after.life[i] - before.life[i]; v != 0 {
			d.LifeBySeat[i] = v
		}
		if v := after.hand[i] - before.hand[i]; v != 0 {
			d.HandBySeat[i] = v
		}
		if v := after.lib[i] - before.lib[i]; v != 0 {
			d.LibraryBySeat[i] = v
		}
		if v := after.gy[i] - before.gy[i]; v != 0 {
			d.GraveyardBySeat[i] = v
		}
		if v := after.exile[i] - before.exile[i]; v != 0 {
			d.ExileBySeat[i] = v
		}
		if v := after.bf[i] - before.bf[i]; v != 0 {
			d.BattlefieldBySeat[i] = v
		}
	}
	d.MarkedDamage = after.markedDamage - before.markedDamage
	d.Tapped = after.tapped - before.tapped
	d.PowerSum = after.powerSum - before.powerSum
	d.ToughSum = after.toughSum - before.toughSum
	for k, v := range after.counters {
		if delta := v - before.counters[k]; delta != 0 {
			d.CountersByKind[k] += delta
		}
	}
	for k, v := range before.counters {
		if _, seen := after.counters[k]; !seen {
			d.CountersByKind[k] -= v
		}
	}
	return d
}

// DefaultSpec is the phase-1 board: 2 seats, opponent holds one of each
// permanent category (creatures are 4/4 so phase-1 damage amounts stay
// sublethal-observable), controller holds the source + one bystander.
func DefaultSpec() BoardSpec {
	return BoardSpec{
		Controller: 0, Opponent: 1,
		OppCreatures: 1, OppArtifacts: 1, OppEnchants: 1, OppLands: 1,
		OwnCreatures: 1, SrcIsCreature: true, LibrarySize: 10,
		OppTappedArtifacts: 1, OwnTappedLands: 1, HandSize: 5, XValue: 3,
	}
}

// BuildBoard constructs the deterministic harness state for spec.
// Returns the state and the source permanent.
func BuildBoard(spec BoardSpec, srcName string) (*gameengine.GameState, *gameengine.Permanent) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(63)), nil)
	gs.Phase = "main"
	gs.Active = spec.Controller

	addPerm := func(seat int, name string, pow, tough int, types ...string) *gameengine.Permanent {
		c := &gameengine.Card{
			Name: name, Owner: seat,
			Types:     append([]string{}, types...),
			BasePower: pow, BaseToughness: tough,
		}
		p := &gameengine.Permanent{
			Card: c, Controller: seat, Owner: seat,
			Counters: map[string]int{}, Flags: map[string]int{},
			Timestamp: gs.NextTimestamp(),
		}
		gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
		return p
	}

	for i := 0; i < spec.OppCreatures; i++ {
		addPerm(spec.Opponent, "Opp Bear", 4, 4, "creature")
	}
	for i := 0; i < spec.OppArtifacts; i++ {
		addPerm(spec.Opponent, "Opp Relic", 0, 0, "artifact")
	}
	for i := 0; i < spec.OppEnchants; i++ {
		addPerm(spec.Opponent, "Opp Glyph", 0, 0, "enchantment")
	}
	for i := 0; i < spec.OppLands; i++ {
		addPerm(spec.Opponent, "Opp Wastes", 0, 0, "land", "basic")
	}
	for i := 0; i < spec.OwnCreatures; i++ {
		addPerm(spec.Controller, "Own Bear", 4, 4, "creature")
	}

	for i := 0; i < spec.OppTappedArtifacts; i++ {
		p := addPerm(spec.Opponent, "Opp Tapped Relic", 0, 0, "artifact")
		p.Tapped = true
	}
	for i := 0; i < spec.OwnTappedLands; i++ {
		p := addPerm(spec.Controller, "Own Tapped Wastes", 0, 0, "land", "basic")
		p.Tapped = true
	}

	srcTypes := []string{"artifact"}
	if spec.SrcIsCreature {
		srcTypes = []string{"creature"}
	}
	src := addPerm(spec.Controller, srcName, 4, 4, srcTypes...)

	if spec.XValue > 0 {
		gs.Flags["x"] = spec.XValue
		gs.Flags["var"] = spec.XValue
	}
	for seat := 0; seat < 2; seat++ {
		for i := 0; i < spec.HandSize; i++ {
			gs.Seats[seat].Hand = append(gs.Seats[seat].Hand, &gameengine.Card{
				Name: fmt.Sprintf("Hand Filler %d", i), Owner: seat, Types: []string{"creature"},
			})
		}
		for i := 0; i < spec.LibrarySize; i++ {
			gs.Seats[seat].Library = append(gs.Seats[seat].Library, &gameengine.Card{
				Name: fmt.Sprintf("Filler %d", i), Owner: seat, Types: []string{"creature"},
			})
		}
	}
	return gs, src
}

// RunEffect resolves one effect through the engine on a fresh board and
// compares against the interpreter's expectation. Returns (finding,
// ran): ran=false means out of phase-1 scope; finding==nil with
// ran=true means PASS.
func RunEffect(cardName, raw string, eff gameast.Effect) (*Finding, bool) {
	spec := DefaultSpec()
	expectedSet, ok := ExpectSet(spec, eff)
	if !ok {
		return nil, false
	}
	gs, src := BuildBoard(spec, cardName)
	before := snap(gs)
	resolveThroughEngine(gs, src, eff)
	actual := diff(before, snap(gs))

	expDesc := ""
	for i, exp := range expectedSet {
		if actual.Equal(exp) {
			return nil, true
		}
		if i > 0 {
			expDesc += " | "
		}
		expDesc += exp.String()
	}
	return &Finding{
		CardName: cardName,
		Kind:     eff.Kind(),
		Expected: expDesc,
		Actual:   actual.String(),
		Raw:      raw,
	}, true
}

// ExtractEffects pulls the resolvable effect trees off a CardAST the
// same way the engine's resolution paths do: Triggered effects,
// Activated effects, and Static typed_spell_effect wrappers.
func ExtractEffects(ast *gameast.CardAST) []ExtractedEffect {
	if ast == nil {
		return nil
	}
	var out []ExtractedEffect
	for _, ab := range ast.Abilities {
		switch a := ab.(type) {
		case *gameast.Triggered:
			if a.Effect != nil {
				out = append(out, ExtractedEffect{Effect: a.Effect, Raw: a.Raw})
			}
		case *gameast.Activated:
			if a.Effect != nil {
				out = append(out, ExtractedEffect{Effect: a.Effect, Raw: a.Raw})
			}
		case *gameast.Static:
			if a.Modification == nil || a.Modification.ModKind != "typed_spell_effect" {
				continue
			}
			for _, arg := range a.Modification.Args {
				if eff, ok := arg.(gameast.Effect); ok && eff != nil {
					out = append(out, ExtractedEffect{Effect: eff, Raw: a.Raw})
				}
			}
		}
	}
	return out
}

// ExtractedEffect pairs an effect tree with its raw oracle clause.
type ExtractedEffect struct {
	Effect gameast.Effect
	Raw    string
}

// resolveThroughEngine is the single engine touchpoint, indirected so
// tests can drive deliberate mismatches through the same path.
func resolveThroughEngine(gs *gameengine.GameState, src *gameengine.Permanent, eff gameast.Effect) {
	gameengine.ResolveEffect(gs, src, eff)
}
