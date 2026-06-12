package outcome

import (
	"fmt"
	"math/rand"
	"strings"

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
	pool                           []int // seat.ManaPool totals
	liveStack                      int   // non-countered stack items
}

func snap(gs *gameengine.GameState) snapshot {
	n := len(gs.Seats)
	s := snapshot{
		life: make([]int, n), hand: make([]int, n), lib: make([]int, n),
		gy: make([]int, n), exile: make([]int, n), bf: make([]int, n),
		counters: map[string]int{}, pool: make([]int, n),
	}
	for _, it := range gs.Stack {
		if it != nil && !it.Countered {
			s.liveStack++
		}
	}
	for i, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		s.life[i] = seat.Life
		s.pool[i] = seat.ManaPool
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
	for i := range before.pool {
		if v := after.pool[i] - before.pool[i]; v != 0 {
			d.PoolBySeat[i] = v
		}
	}
	d.LiveStack = after.liveStack - before.liveStack
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
		LibraryLands: 4, GraveyardCreatures: 2,
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
		// Library: LibraryLands basic lands inside the LibrarySize total
		// (tutor candidates), creature fillers for the rest. Count-space
		// expectations (draw/mill) only read the total.
		lands := spec.LibraryLands
		if lands > spec.LibrarySize {
			lands = spec.LibrarySize
		}
		for i := 0; i < spec.LibrarySize-lands; i++ {
			gs.Seats[seat].Library = append(gs.Seats[seat].Library, &gameengine.Card{
				Name: fmt.Sprintf("Filler %d", i), Owner: seat, Types: []string{"creature"},
			})
		}
		for i := 0; i < lands; i++ {
			gs.Seats[seat].Library = append(gs.Seats[seat].Library, &gameengine.Card{
				Name: fmt.Sprintf("Library Wastes %d", i), Owner: seat,
				Types: []string{"basic", "land"}, TypeLine: "basic land — wastes",
			})
		}
		// Graveyard: known 4/4 creature cards (reanimation candidates).
		for i := 0; i < spec.GraveyardCreatures; i++ {
			gs.Seats[seat].Graveyard = append(gs.Seats[seat].Graveyard, &gameengine.Card{
				Name: fmt.Sprintf("GY Bear %d", i), Owner: seat,
				Types: []string{"creature"}, BasePower: scaffoldCreaturePT, BaseToughness: scaffoldCreaturePT,
			})
		}
	}
	// Stack: opponent-controlled instant spells (counterspell candidates).
	// Appended directly (no PushStackItem) so no cast/push triggers fire.
	for i := 0; i < spec.StackSpells; i++ {
		gs.Stack = append(gs.Stack, &gameengine.StackItem{
			ID:         1000 + i,
			Controller: spec.Opponent,
			Kind:       "spell",
			Card: &gameengine.Card{
				Name: fmt.Sprintf("Stack Filler %d", i), Owner: spec.Opponent,
				Types: []string{"instant"},
			},
		})
	}
	return gs, src
}

// RunEffect resolves one effect through the engine on a fresh board and
// compares against the interpreter's expectation. Returns (finding,
// ran): ran=false means out of phase-1 scope; finding==nil with
// ran=true means PASS.
func RunEffect(cardName, raw string, eff gameast.Effect) (*Finding, bool) {
	spec := DefaultSpec()
	if treeNeedsStackSpell(eff) {
		// A live opponent spell on the stack ONLY for counter/copy
		// shapes — a resident stack item must not perturb every other
		// check.
		spec.StackSpells = 1
	}
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
	f := &Finding{
		CardName: cardName,
		Kind:     eff.Kind(),
		Expected: expDesc,
		Actual:   actual.String(),
		Raw:      raw,
	}
	EmitFinding(f) // origin tap — Judge router
	return f, true
}

// ExtractEffects pulls the resolvable effect trees off a CardAST the
// same way the engine's resolution paths do: Triggered effects,
// Activated effects, and Static typed_spell_effect wrappers.
func ExtractEffects(ast *gameast.CardAST) []ExtractedEffect {
	if ast == nil {
		return nil
	}
	// Full-card raw text — the self-delta-trigger scope gate below needs
	// the OTHER abilities' text (Niv-Mizzet's draw-punch lives in a
	// different ability than the draw being tested).
	var allRaw strings.Builder
	for _, ab := range ast.Abilities {
		switch a := ab.(type) {
		case *gameast.Triggered:
			allRaw.WriteString(strings.ToLower(a.Raw) + "\n")
		case *gameast.Static:
			allRaw.WriteString(strings.ToLower(a.Raw) + "\n")
		case *gameast.Activated:
			allRaw.WriteString(strings.ToLower(a.Raw) + "\n")
		}
	}
	cardRaw := allRaw.String()
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
					continue
				}
				// Nested Modification args (draw_per / lose_life_per
				// for-each family) deserialize as *Modification, which
				// is not an Effect — wrap them the way the engine's
				// typed_spell_effect resolution does (r63 phase 3).
				if m, ok := arg.(*gameast.Modification); ok && m != nil {
					out = append(out, ExtractedEffect{
						Effect: &gameast.ModificationEffect{ModKind: m.ModKind, Args: m.Args},
						Raw:    a.Raw,
					})
				}
			}
		}
	}
	// Scope gate (r63 OUTCOME residual round): drop effects whose card
	// carries ANOTHER trigger firing on the tested effect's own delta
	// class (Niv-Mizzet "whenever you draw" + a draw effect; Dina
	// "whenever you gain life" + a gain; Hapatra "-1/-1 counters are
	// put" + a counter placement). The engine rightly resolves the
	// secondary trigger on the live board; the interpreter models the
	// tested effect alone, so the comparison is unsound — out of scope,
	// verified engine-correct on every instance.
	filtered := out[:0]
	for _, ex := range out {
		if selfDeltaTriggerCollision(cardRaw, ex.Raw) {
			continue
		}
		filtered = append(filtered, ex)
	}
	return filtered
}

// selfDeltaTriggerCollision — see the scope-gate comment in
// ExtractEffects. Raw-text-level classification, matching the corpus
// walk's established style.
func selfDeltaTriggerCollision(cardRaw, effRaw string) bool {
	e := strings.ToLower(effRaw)
	if strings.Contains(e, "draw") &&
		(strings.Contains(cardRaw, "whenever you draw") || strings.Contains(cardRaw, "whenever a player draws")) {
		return true
	}
	if strings.Contains(e, "gain") && strings.Contains(cardRaw, "whenever you gain life") {
		return true
	}
	if strings.Contains(e, "counter") &&
		(strings.Contains(cardRaw, "counters are put on") || strings.Contains(cardRaw, "whenever you put one or more")) {
		return true
	}
	return false
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

// ---------------------------------------------------------------------------
// Exported observation API (r63 progression dimension) — the PROGRESSION
// trigger-correctness checker composes on the same state-delta
// observation: a trigger firing is observable as its effect's delta.
// ---------------------------------------------------------------------------

// StateSnapshot is an opaque aggregate observation of a game state.
type StateSnapshot struct{ s snapshot }

// Snap captures the aggregate observable state.
func Snap(gs *gameengine.GameState) StateSnapshot { return StateSnapshot{snap(gs)} }

// DiffSnapshots computes the actual Delta between two snapshots.
func DiffSnapshots(before, after StateSnapshot) *Delta { return diff(before.s, after.s) }

// BuildBoardForSpec is the exported board builder (spec-first argument
// order for external callers).
func BuildBoardForSpec(spec BoardSpec, srcName string) (*gameengine.GameState, *gameengine.Permanent) {
	return BuildBoard(spec, srcName)
}
