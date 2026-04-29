// Package hat hosts the concrete implementations of
// github.com/hexdek/hexdek/internal/gameengine.Hat.
//
// The interface itself is declared in gameengine to avoid an import
// cycle (gameengine.Seat references Hat; Hat implementations reference
// gameengine types). This package is where policies live.
//
// Two hats ship with Phase 10:
//
//   - GreedyHat — byte-equivalent to the pre-Phase-10 inline
//                 engine heuristics. Used as the baseline in parity
//                 tests so Phase 12 can diff the two engines without
//                 mode-driven drift.
//   - PokerHat  — adaptive HOLD/CALL/RAISE hat with a 7-dimensional
//                 threat score, archetype detection, and a RAISE
//                 cascade driven by observed player_mode_change
//                 events. Mirrors scripts/extensions/policies/poker.py.
//
// Every method reads game/seat state and returns a decision without
// mutating the game. The only method that updates internal state is
// ObserveEvent (broadcast from GameState.LogEvent to every seat's Hat).
//
// Swappability
// ------------
// Hats are per-seat fields. To run a mixed tournament:
//
//	gs.Seats[0].Hat = &hat.GreedyHat{}
//	gs.Seats[1].Hat = hat.NewPokerHat()
//	gs.Seats[2].Hat = &hat.GreedyHat{}
//	gs.Seats[3].Hat = hat.NewPokerHat()
//
// The engine NEVER branches on hat type — that's the load-bearing
// architectural contract. (See gameengine/hat.go doc comment.)
package hat

import (
	"sort"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Compile-time check that GreedyHat satisfies gameengine.Hat.
var _ gameengine.Hat = (*GreedyHat)(nil)

// Deprecated: GreedyHat is superseded by YggdrasilHat (budget=0 for equivalent behavior).
// Retained only for Phase 12 parity test byte-equivalence with the Python engine.
//
// GreedyHat is the baseline heuristic. Stateless — two GreedyHat
// instances are interchangeable, so tournament runners may share a
// single *GreedyHat across every seat.
type GreedyHat struct{}

// ---------------------------------------------------------------------
// Mulligan
// ---------------------------------------------------------------------

// ChooseMulligan — §103.4. Mulligan if the opening hand has 0-1 lands
// to prevent zero-activity games where a player is stuck with no mana
// sources. Only mulligans once (keeps on 6).
func (*GreedyHat) ChooseMulligan(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card) bool {
	landCount := 0
	for _, c := range hand {
		if c != nil {
			for _, t := range c.Types {
				if t == "land" {
					landCount++
					break
				}
			}
		}
	}
	// Mulligan if 0-1 lands and haven't mulliganed yet (hand size = 7).
	if landCount <= 1 && len(hand) >= 7 {
		return true
	}
	return false
}

// ---------------------------------------------------------------------
// Land drop + casting
// ---------------------------------------------------------------------

// ChooseLandToPlay returns the first land in hand — matches the
// pre-Phase-10 inline "take the first land" behavior.
func (*GreedyHat) ChooseLandToPlay(gs *gameengine.GameState, seatIdx int, lands []*gameengine.Card) *gameengine.Card {
	if len(lands) == 0 {
		return nil
	}
	return lands[0]
}

// CardCategory classifies a card's strategic role for cast prioritization.
type CardCategory int

const (
	CatRamp    CardCategory = iota // Mana rocks, dorks, ramp spells
	CatDraw                        // Card draw / selection
	CatRemoval                     // Targeted removal
	CatCounter                     // Counterspells (usually held)
	CatCombo                       // Known combo pieces
	CatThreat                      // Big creatures, planeswalkers, impactful enchantments
	CatUtility                     // Everything else
)

// categorizeCard determines the strategic role of a card for cast ordering.
func categorizeCard(c *gameengine.Card) CardCategory {
	if c == nil {
		return CatUtility
	}

	// Name-based overrides for cards whose primary use differs from their
	// card type. These cards should be used for their non-cast ability,
	// not played as vanilla creatures/spells.
	name := strings.ToLower(c.DisplayName())
	switch name {
	case "simian spirit guide", "elvish spirit guide":
		// Exile from hand for mana, not cast as a vanilla creature.
		return CatRamp
	case "force of will", "fierce guardianship", "pact of negation",
		"force of negation", "misdirection":
		// Pitch/free counter, not a 5-mana beater.
		return CatCounter
	}

	if gameengine.CardHasCounterSpell(c) {
		return CatCounter
	}
	if isRampCard(c) {
		return CatRamp
	}
	// Draw detection: check AST for draw effects.
	if c.AST != nil {
		for _, ab := range c.AST.Abilities {
			if a, ok := ab.(*gameast.Activated); ok && a.Effect != nil {
				if a.Effect.Kind() == "draw" {
					return CatDraw
				}
			}
		}
	}
	// Oracle text draw heuristic.
	ot := strings.ToLower(gameengine.OracleTextLower(c))
	if strings.Contains(ot, "draw a card") || strings.Contains(ot, "draw cards") ||
		strings.Contains(ot, "draw two") || strings.Contains(ot, "draw three") {
		return CatDraw
	}
	// Removal detection.
	if c.AST != nil {
		for _, ab := range c.AST.Abilities {
			if a, ok := ab.(*gameast.Activated); ok && a.Effect != nil {
				k := a.Effect.Kind()
				if k == "destroy" || k == "exile" || k == "bounce" || k == "damage" {
					return CatRemoval
				}
			}
		}
	}
	if strings.Contains(ot, "destroy") || strings.Contains(ot, "exile") ||
		strings.Contains(ot, "return target") {
		return CatRemoval
	}
	// Threat: big creatures, planeswalkers.
	if typeLineContains(c, "planeswalker") {
		return CatThreat
	}
	if typeLineContains(c, "creature") && gameengine.ManaCostOf(c) >= 4 {
		return CatThreat
	}
	return CatUtility
}

// ChooseCastFromHand — three-phase game-aware cast ordering.
// Skips counterspells (saved for opponents' stack items).
//
// Phase 1 (turns 1-12): RAMP FIRST
//   - Priority: mana rocks (CMC 0-2) > mana dorks > ramp spells > draw > other
//   - Sol Ring, Mana Crypt, Arcane Signet, signets, talismans all here.
//
// Phase 2 (turns 13-24): DEVELOP
//   - Priority: card draw > removal > threats > utility
//
// Phase 3 (turns 25+): CLOSE
//   - Priority: biggest threats > removal > everything else
//
// ALWAYS: ramp at CMC 0-1 is cast regardless of phase (Sol Ring is ALWAYS first).
func (*GreedyHat) ChooseCastFromHand(gs *gameengine.GameState, seatIdx int, castable []*gameengine.Card) *gameengine.Card {
	pool := make([]*gameengine.Card, 0, len(castable))
	for _, c := range castable {
		if c == nil {
			continue
		}
		if gameengine.CardHasCounterSpell(c) {
			continue
		}
		pool = append(pool, c)
	}
	if len(pool) == 0 {
		return nil
	}

	turn := 0
	if gs != nil {
		turn = gs.Turn
	}

	// ALWAYS: ultra-cheap ramp (CMC 0-1) gets cast first regardless of phase.
	// Sol Ring, Mana Crypt, Chrome Mox, Mox Diamond, etc.
	var ultraRamp []*gameengine.Card
	var rest []*gameengine.Card
	for _, c := range pool {
		if isUltraCheapRamp(c) {
			ultraRamp = append(ultraRamp, c)
		} else {
			rest = append(rest, c)
		}
	}
	if len(ultraRamp) > 0 {
		sort.SliceStable(ultraRamp, func(i, j int) bool {
			ci, cj := gameengine.ManaCostOf(ultraRamp[i]), gameengine.ManaCostOf(ultraRamp[j])
			if ci != cj {
				return ci < cj
			}
			return ultraRamp[i].DisplayName() < ultraRamp[j].DisplayName()
		})
		return ultraRamp[0]
	}

	pool = rest
	if len(pool) == 0 {
		return nil
	}

	// Phase 1: RAMP (turns 1-12, rounds 1-3 in 4-player)
	if turn <= 12 {
		var ramp, draw, other []*gameengine.Card
		for _, c := range pool {
			switch categorizeCard(c) {
			case CatRamp:
				ramp = append(ramp, c)
			case CatDraw:
				draw = append(draw, c)
			default:
				other = append(other, c)
			}
		}
		// Ramp first (cheapest first).
		if len(ramp) > 0 {
			sort.SliceStable(ramp, func(i, j int) bool {
				ci, cj := gameengine.ManaCostOf(ramp[i]), gameengine.ManaCostOf(ramp[j])
				if ci != cj {
					return ci < cj
				}
				return ramp[i].DisplayName() < ramp[j].DisplayName()
			})
			return ramp[0]
		}
		// Then draw (cheapest first).
		if len(draw) > 0 {
			sort.SliceStable(draw, func(i, j int) bool {
				ci, cj := gameengine.ManaCostOf(draw[i]), gameengine.ManaCostOf(draw[j])
				if ci != cj {
					return ci < cj
				}
				return draw[i].DisplayName() < draw[j].DisplayName()
			})
			return draw[0]
		}
		// Then anything else, biggest first.
		pool = other
		if len(pool) == 0 {
			return nil
		}
	}

	// Phase 2: DEVELOP (turns 13-24)
	if turn >= 13 && turn <= 24 {
		var draw, removal, threats, ramp, other []*gameengine.Card
		for _, c := range pool {
			switch categorizeCard(c) {
			case CatDraw:
				draw = append(draw, c)
			case CatRemoval:
				removal = append(removal, c)
			case CatThreat:
				threats = append(threats, c)
			case CatRamp:
				ramp = append(ramp, c)
			default:
				other = append(other, c)
			}
		}
		// Still cast ramp if available (cheapest first).
		if len(ramp) > 0 {
			sort.SliceStable(ramp, func(i, j int) bool {
				return gameengine.ManaCostOf(ramp[i]) < gameengine.ManaCostOf(ramp[j])
			})
			return ramp[0]
		}
		// Draw > removal > threats > other.
		for _, bucket := range [][]*gameengine.Card{draw, removal, threats, other} {
			if len(bucket) == 0 {
				continue
			}
			// Within each bucket: biggest first (get the most impact).
			sort.SliceStable(bucket, func(i, j int) bool {
				ci, cj := gameengine.ManaCostOf(bucket[i]), gameengine.ManaCostOf(bucket[j])
				if ci != cj {
					return ci > cj
				}
				return bucket[i].DisplayName() < bucket[j].DisplayName()
			})
			return bucket[0]
		}
		return nil
	}

	// Phase 3: CLOSE (turns 25+) — biggest threats first.
	sort.SliceStable(pool, func(i, j int) bool {
		ci, cj := gameengine.ManaCostOf(pool[i]), gameengine.ManaCostOf(pool[j])
		if ci != cj {
			return ci > cj // higher CMC first
		}
		return pool[i].DisplayName() < pool[j].DisplayName()
	})
	return pool[0]
}

// isUltraCheapRamp returns true if c is a 0-1 CMC fast mana card that
// should ALWAYS be cast first regardless of game phase. This catches
// Sol Ring, Mana Crypt, Chrome Mox, mana dorks, rituals, spirit guides, etc.
func isUltraCheapRamp(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	cmc := gameengine.ManaCostOf(c)
	if cmc > 1 {
		return false
	}

	// Known 0-1 CMC fast mana by name.
	name := strings.ToLower(c.DisplayName())
	fastMana := []string{
		"sol ring", "mana crypt", "mana vault",
		"chrome mox", "mox diamond", "mox opal", "mox amber",
		"lotus petal", "lion's eye diamond",
		"jeweled lotus", "arcane signet",
		"elvish mystic", "llanowar elves", "birds of paradise",
		"deathrite shaman", "noble hierarch",
		"wild growth", "utopia sprawl",
		"dark ritual", "cabal ritual",
		"simian spirit guide", "elvish spirit guide",
	}
	for _, fm := range fastMana {
		if strings.Contains(name, fm) {
			return true
		}
	}

	// Generic: any artifact with CMC 0-1.
	if cmc <= 1 && typeLineContains(c, "artifact") {
		return true
	}

	return false
}

// isRampCard returns true if c is a mana-producing card that should be
// prioritized in early turns: artifacts with CMC <= 3, or creatures with
// CMC <= 2 that have a mana-producing activated ability.
func isRampCard(c *gameengine.Card) bool {
	if c == nil {
		return false
	}

	// Ultra-cheap ramp is always ramp.
	if isUltraCheapRamp(c) {
		return true
	}

	cmc := gameengine.ManaCostOf(c)

	// Artifact with CMC <= 3: Sol Ring, Mana Crypt, Signets, etc.
	if cmc <= 3 && typeLineContains(c, "artifact") {
		return true
	}

	// Creature with CMC <= 2 that has a mana ability (Birds of Paradise,
	// Llanowar Elves, Deathrite Shaman, Elvish Mystic).
	if cmc <= 2 && typeLineContains(c, "creature") {
		// Check AST for add_mana activated ability.
		if c.AST != nil {
			for _, ab := range c.AST.Abilities {
				if a, ok := ab.(*gameast.Activated); ok {
					if a.Effect != nil && a.Effect.Kind() == "add_mana" {
						return true
					}
				}
			}
		}
		// Fallback: check oracle text for common mana-dork patterns.
		ot := strings.ToLower(gameengine.OracleTextLower(c))
		if strings.Contains(ot, "add {") || strings.Contains(ot, "add one mana") ||
			strings.Contains(ot, "add {g}") || strings.Contains(ot, "add {b}") ||
			strings.Contains(ot, "add {u}") || strings.Contains(ot, "add {r}") ||
			strings.Contains(ot, "add {w}") {
			return true
		}
	}

	return false
}

// ChooseActivation — heuristic activation picker. Prioritizes abilities
// by strategic value:
//   1. Draw / tutor abilities (Sensei's Divining Top, Sylvan Library)
//   2. Removal / damage abilities (Walking Ballista, Goblin Bombardment)
//   3. Token creation abilities
//   4. Pump abilities (Giant Growth effects)
//   5. Equipment equip abilities (attach to best creature)
//   6. Other abilities
//
// Picks the first high-priority affordable option. Returns nil if no
// option seems worth activating.
func (*GreedyHat) ChooseActivation(gs *gameengine.GameState, seatIdx int, options []gameengine.Activation) *gameengine.Activation {
	if len(options) == 0 {
		return nil
	}

	// Score each option and pick the best.
	bestScore := 0
	var bestOpt *gameengine.Activation
	for i := range options {
		opt := &options[i]
		score := scoreActivation(gs, seatIdx, opt)
		if score > bestScore {
			bestScore = score
			bestOpt = opt
		}
	}
	if bestOpt == nil || bestScore <= 0 {
		return nil
	}
	return bestOpt
}

// scoreActivation assigns a priority score to an activation option.
// Higher = more desirable. 0 = skip.
func scoreActivation(gs *gameengine.GameState, seatIdx int, opt *gameengine.Activation) int {
	if opt == nil || opt.Permanent == nil || opt.Permanent.Card == nil {
		return 0
	}
	perm := opt.Permanent
	card := perm.Card

	// Check AST for the activated ability's effect.
	if card.AST == nil || opt.Ability < 0 || opt.Ability >= len(card.AST.Abilities) {
		return 1 // per_card handler, give minimal score
	}
	ab, ok := card.AST.Abilities[opt.Ability].(*gameast.Activated)
	if !ok || ab.Effect == nil {
		return 1
	}

	return scoreEffect(ab.Effect, gs, seatIdx, perm)
}

// scoreEffect recursively scores an effect for activation priority.
func scoreEffect(e gameast.Effect, gs *gameengine.GameState, seatIdx int, perm *gameengine.Permanent) int {
	if e == nil {
		return 0
	}
	switch eff := e.(type) {
	case *gameast.Draw:
		return 50 // Draw is very high priority
	case *gameast.Tutor:
		return 48 // Tutor is similarly high
	case *gameast.Damage:
		return 40 // Removal/damage
	case *gameast.Destroy:
		return 42 // Targeted destruction
	case *gameast.Exile:
		return 41 // Exile removal
	case *gameast.Bounce:
		return 35 // Bounce
	case *gameast.CreateToken:
		return 30 // Token creation
	case *gameast.Buff:
		// Pump/buff is only useful during combat.
		if gs != nil && gs.Phase == "combat" {
			return 25
		}
		return 0 // Skip pump outside combat
	case *gameast.AddMana:
		return 0 // Mana abilities are handled separately
	case *gameast.GainLife:
		return 5 // Low priority
	case *gameast.LoseLife:
		return 38 // Drain effects targeting opponents
	case *gameast.Mill:
		return 15 // Mill
	case *gameast.Scry:
		return 20 // Scry (library manipulation)
	case *gameast.Sequence:
		// Score the best sub-effect.
		best := 0
		for _, sub := range eff.Items {
			s := scoreEffect(sub, gs, seatIdx, perm)
			if s > best {
				best = s
			}
		}
		return best
	case *gameast.Choice:
		// Score the best option.
		best := 0
		for _, sub := range eff.Options {
			s := scoreEffect(sub, gs, seatIdx, perm)
			if s > best {
				best = s
			}
		}
		return best
	case *gameast.Optional_:
		return scoreEffect(eff.Body, gs, seatIdx, perm)
	case *gameast.Conditional:
		return scoreEffect(eff.Body, gs, seatIdx, perm)
	}

	// Check oracle text for equip patterns.
	raw := strings.ToLower(gameengine.OracleTextLower(perm.Card))
	if strings.Contains(raw, "equip") {
		// Only equip if we have creatures to equip to.
		if seatIdx >= 0 && seatIdx < len(gs.Seats) {
			seat := gs.Seats[seatIdx]
			hasCreature := false
			for _, p := range seat.Battlefield {
				if p != nil && p.IsCreature() && p != perm {
					hasCreature = true
					break
				}
			}
			if hasCreature {
				return 20
			}
		}
		return 0
	}

	return 1 // Unknown effect, minimal score
}

// ---------------------------------------------------------------------
// Combat
// ---------------------------------------------------------------------

// ChooseAttackers — attack with every legal attacker. Matches the
// inline declare-attackers loop and Python GreedyHat.
func (*GreedyHat) ChooseAttackers(gs *gameengine.GameState, seatIdx int, legal []*gameengine.Permanent) []*gameengine.Permanent {
	out := make([]*gameengine.Permanent, 0, len(legal))
	for _, p := range legal {
		if p != nil {
			out = append(out, p)
		}
	}
	return out
}

// ChooseAttackTarget picks the LEADING living opponent (highest life +
// board power combined) to apply political pressure and prevent runaway
// leaders. This mirrors real Commander politics where the table gangs up
// on whoever is ahead. Breaks ties by APNAP distance. §506.1.
func (*GreedyHat) ChooseAttackTarget(gs *gameengine.GameState, seatIdx int, attacker *gameengine.Permanent, legalDefenders []int) int {
	if len(legalDefenders) == 0 {
		return seatIdx
	}
	if len(legalDefenders) == 1 {
		return legalDefenders[0]
	}
	n := len(gs.Seats)
	bestIdx := legalDefenders[0]
	bestScore := leaderScore(gs.Seats[bestIdx])
	bestDist := (bestIdx - seatIdx + n) % n
	for _, d := range legalDefenders[1:] {
		if d < 0 || d >= len(gs.Seats) {
			continue
		}
		s := gs.Seats[d]
		if s == nil || s.Lost {
			continue
		}
		score := leaderScore(s)
		dist := (d - seatIdx + n) % n
		if score > bestScore || (score == bestScore && dist < bestDist) {
			bestIdx, bestScore, bestDist = d, score, dist
		}
	}
	return bestIdx
}

// leaderScore calculates a score that reflects how "ahead" a player is.
// Life + total board power + high-CMC permanents. The highest-scoring
// player is the one everyone should be targeting.
func leaderScore(seat *gameengine.Seat) int {
	if seat == nil || seat.Lost {
		return -1000000
	}
	score := seat.Life
	for _, p := range seat.Battlefield {
		if p == nil {
			continue
		}
		if p.IsCreature() {
			pw := p.Power()
			if pw > 0 {
				score += pw * 2
			}
		}
		if p.Card != nil && gameengine.ManaCostOf(p.Card) >= 4 {
			score += 3
		}
	}
	return score
}

// AssignBlockers — deadliest-attacker-first; pick smallest surviving
// blocker, else chump only if the defender dies this turn otherwise.
// Ports Python GreedyHat.declare_blockers.
func (h *GreedyHat) AssignBlockers(gs *gameengine.GameState, seatIdx int, attackers []*gameengine.Permanent) map[*gameengine.Permanent][]*gameengine.Permanent {
	out := make(map[*gameengine.Permanent][]*gameengine.Permanent, len(attackers))
	for _, a := range attackers {
		out[a] = nil
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return out
	}
	seat := gs.Seats[seatIdx]
	// Pool of legal, untapped creatures available to block.
	pool := make([]*gameengine.Permanent, 0, len(seat.Battlefield))
	for _, p := range seat.Battlefield {
		if p != nil && p.IsCreature() && !p.Tapped {
			pool = append(pool, p)
		}
	}
	// Priority: deathtouch (treat as +10 power), double strike (+5),
	// else raw power. Deadliest first.
	type rank struct {
		a     *gameengine.Permanent
		score int
	}
	ranks := make([]rank, 0, len(attackers))
	incoming := 0
	for _, a := range attackers {
		if a == nil {
			continue
		}
		dt := 0
		if a.HasKeyword("deathtouch") {
			dt = 10
		}
		ds := 0
		if a.HasKeyword("double strike") || a.HasKeyword("double_strike") {
			ds = 5
		}
		ranks = append(ranks, rank{a, -(a.Power() + dt + ds)})
		mul := 1
		if ds > 0 {
			mul = 2
		}
		incoming += a.Power() * mul
	}
	sort.SliceStable(ranks, func(i, j int) bool { return ranks[i].score < ranks[j].score })

	used := make(map[*gameengine.Permanent]bool, len(pool))
	life := seat.Life

	for _, r := range ranks {
		atk := r.a
		if atk == nil {
			continue
		}
		// Gather legal blockers (not yet used).
		legal := make([]*gameengine.Permanent, 0, len(pool))
		for _, b := range pool {
			if used[b] {
				continue
			}
			if gameengine.CanBlock(atk, b) {
				legal = append(legal, b)
			}
		}
		if len(legal) == 0 {
			continue
		}
		willDieIfUnblocked := life-incoming <= 0
		atkDT := atk.HasKeyword("deathtouch")
		// Survivors: blockers that outlive the attacker (skip for deathtouch).
		var survivors []*gameengine.Permanent
		if !atkDT {
			for _, b := range legal {
				if b.Toughness()-b.MarkedDamage > atk.Power() {
					survivors = append(survivors, b)
				}
			}
		}
		sort.SliceStable(survivors, func(i, j int) bool {
			ai, aj := survivors[i], survivors[j]
			if ai.Power()+ai.Toughness() != aj.Power()+aj.Toughness() {
				return ai.Power()+ai.Toughness() < aj.Power()+aj.Toughness()
			}
			return ai.Toughness() < aj.Toughness()
		})

		var chosen []*gameengine.Permanent
		if len(survivors) > 0 {
			chosen = []*gameengine.Permanent{survivors[0]}
		} else if willDieIfUnblocked {
			// Chump with the smallest legal.
			smallest := legal[0]
			for _, b := range legal[1:] {
				if b.Power()+b.Toughness() < smallest.Power()+smallest.Toughness() {
					smallest = b
				}
			}
			chosen = []*gameengine.Permanent{smallest}
		}
		// Menace: need a second blocker, else bail.
		if len(chosen) > 0 && (atk.HasKeyword("menace")) {
			extras := make([]*gameengine.Permanent, 0, len(legal))
			for _, b := range legal {
				if b != chosen[0] {
					extras = append(extras, b)
				}
			}
			if len(extras) == 0 {
				chosen = nil
			} else {
				sort.SliceStable(extras, func(i, j int) bool {
					return extras[i].Power()+extras[i].Toughness() < extras[j].Power()+extras[j].Toughness()
				})
				chosen = append(chosen, extras[0])
			}
		}
		if len(chosen) == 0 {
			continue
		}
		for _, b := range chosen {
			used[b] = true
		}
		out[atk] = chosen
		// Update incoming for trample accounting.
		atkDmg := atk.Power()
		if atk.HasKeyword("double strike") || atk.HasKeyword("double_strike") {
			atkDmg *= 2
		}
		if atk.HasKeyword("trample") {
			totalT := 0
			for _, b := range chosen {
				totalT += b.Toughness() - b.MarkedDamage
			}
			leak := atkDmg - totalT
			if leak < 0 {
				leak = 0
			}
			incoming -= (atkDmg - leak)
		} else {
			incoming -= atkDmg
		}
	}
	return out
}

// ---------------------------------------------------------------------
// Stack response
// ---------------------------------------------------------------------

// ChooseResponse — counter opponents' spells when the threat score is
// ≥ 3 AND we have an affordable counter in hand. Matches Python
// GreedyHat.respond_to_stack_item (the "_get_response" port).
func (*GreedyHat) ChooseResponse(gs *gameengine.GameState, seatIdx int, top *gameengine.StackItem) *gameengine.StackItem {
	if top == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	if top.Controller == seatIdx {
		return nil
	}
	if top.Countered {
		return nil
	}
	// Defense-in-depth: split-second / sorcery-speed restrictions.
	if gameengine.SplitSecondActive(gs) {
		return nil
	}
	if gameengine.OppRestrictsDefenderToSorcerySpeed(gs, seatIdx) {
		return nil
	}
	// Simple threat heuristic: counter anything non-trivial. A real
	// port of _stack_item_threat_score would weigh win-the-game /
	// mass-removal higher; for the baseline-parity lane we just
	// require a non-nil effect.
	if top.Effect == nil && top.Card == nil {
		return nil
	}
	// Find a counter in hand we can afford that can actually target
	// the spell on the stack (filter-aware: Negate can't counter a
	// creature, Dispel can't counter a sorcery, etc.).
	seat := gs.Seats[seatIdx]
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		ceff := gameengine.CounterSpellEffectOf(c)
		if ceff == nil {
			continue
		}
		gameengine.EnsureTypedPool(seat)
		if gameengine.ManaCostOf(c) > seat.Mana.Total() {
			continue
		}
		// Check that the counterspell's filter actually matches the
		// target spell on the stack.
		if !gameengine.CounterCanTarget(ceff, top) {
			continue
		}
		// Returned StackItem is read-only advice — the engine decides
		// whether to actually mutate hand / manaPool.
		return &gameengine.StackItem{
			Controller: seatIdx,
			Card:       c,
			Effect:     ceff,
		}
	}
	return nil
}

// ---------------------------------------------------------------------
// Targeting + modes
// ---------------------------------------------------------------------

// ChooseTarget — first legal target. Matches Python's baseline
// "pick first" behavior for tests & parity; replaces the pre-Phase-10
// PickTarget(...)[0] shortcut.
func (*GreedyHat) ChooseTarget(gs *gameengine.GameState, seatIdx int, filter gameast.Filter, legal []gameengine.Target) gameengine.Target {
	if len(legal) == 0 {
		return gameengine.Target{Kind: gameengine.TargetKindNone}
	}
	return legal[0]
}

// ChooseMode — always index 0. Matches Python legacy Choice resolver.
func (*GreedyHat) ChooseMode(gs *gameengine.GameState, seatIdx int, modes []gameast.Effect) int {
	if len(modes) == 0 {
		return -1
	}
	return 0
}

// ---------------------------------------------------------------------
// Commander
// ---------------------------------------------------------------------

// ShouldCastCommander — greedy: yes, always cast when affordable. The
// engine already verified mana availability before calling.
func (*GreedyHat) ShouldCastCommander(gs *gameengine.GameState, seatIdx int, commanderName string, tax int) bool {
	return true
}

// ShouldRedirectCommanderZone — §903.9a / b: the OWNER chooses. Greedy
// always redirects (protects the commander for tax-insulated recasts).
func (*GreedyHat) ShouldRedirectCommanderZone(gs *gameengine.GameState, seatIdx int, commander *gameengine.Card, to string) bool {
	return true
}

// ---------------------------------------------------------------------
// Replacements / discard
// ---------------------------------------------------------------------

// OrderReplacements — §616.1: self-controlled first (timestamp break
// within each group). The affected player is the chooser per §616.1;
// the engine already filtered to the seat that matters.
func (*GreedyHat) OrderReplacements(gs *gameengine.GameState, seatIdx int, candidates []*gameengine.ReplacementEffect) []*gameengine.ReplacementEffect {
	if len(candidates) <= 1 {
		return candidates
	}
	out := make([]*gameengine.ReplacementEffect, len(candidates))
	copy(out, candidates)
	sort.SliceStable(out, func(i, j int) bool {
		// Self-controlled first.
		aSelf := out[i] != nil && out[i].ControllerSeat == seatIdx
		bSelf := out[j] != nil && out[j].ControllerSeat == seatIdx
		if aSelf != bSelf {
			return aSelf
		}
		if out[i] == nil || out[j] == nil {
			return out[i] != nil
		}
		return out[i].Timestamp < out[j].Timestamp
	})
	return out
}

// ChooseDiscard — highest-CMC N first. Matches Python GreedyHat.
func (*GreedyHat) ChooseDiscard(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card, n int) []*gameengine.Card {
	if n <= 0 || len(hand) == 0 {
		return nil
	}
	ranked := make([]*gameengine.Card, len(hand))
	copy(ranked, hand)
	sort.SliceStable(ranked, func(i, j int) bool {
		ci, cj := gameengine.ManaCostOf(ranked[i]), gameengine.ManaCostOf(ranked[j])
		if ci != cj {
			return ci > cj
		}
		return ranked[i].DisplayName() < ranked[j].DisplayName()
	})
	if n > len(ranked) {
		n = len(ranked)
	}
	return ranked[:n]
}

// OrderTriggers — §603.3b: baseline keeps triggers in arrival order
// (timestamp-sorted). No strategic reordering for the greedy baseline.
func (*GreedyHat) OrderTriggers(gs *gameengine.GameState, seatIdx int, triggers []*gameengine.StackItem) []*gameengine.StackItem {
	return triggers
}

// ChooseX — greedy: spend all available mana on X. §107.3.
func (*GreedyHat) ChooseX(gs *gameengine.GameState, seatIdx int, card *gameengine.Card, availableMana int) int {
	if availableMana < 0 {
		return 0
	}
	return availableMana
}

// ChooseBottomCards — §103.5 London mulligan: bottom the highest-CMC cards
// (same as discard heuristic). Returns exactly `count` cards.
func (*GreedyHat) ChooseBottomCards(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card, count int) []*gameengine.Card {
	if count <= 0 || len(hand) == 0 {
		return nil
	}
	ranked := make([]*gameengine.Card, len(hand))
	copy(ranked, hand)
	sort.SliceStable(ranked, func(i, j int) bool {
		ci, cj := gameengine.ManaCostOf(ranked[i]), gameengine.ManaCostOf(ranked[j])
		if ci != cj {
			return ci > cj // highest CMC first
		}
		return ranked[i].DisplayName() < ranked[j].DisplayName()
	})
	if count > len(ranked) {
		count = len(ranked)
	}
	return ranked[:count]
}

// ChooseScry — §701.18: keep lands on top early, non-lands on top late.
// Baseline heuristic: keep cards with CMC <= available mana on top,
// bottom the rest. Simple but effective for a greedy player.
func (*GreedyHat) ChooseScry(gs *gameengine.GameState, seatIdx int, cards []*gameengine.Card) (top []*gameengine.Card, bottom []*gameengine.Card) {
	if len(cards) == 0 {
		return nil, nil
	}
	avail := 0
	if seatIdx >= 0 && seatIdx < len(gs.Seats) {
		avail = gameengine.AvailableManaEstimate(gs, gs.Seats[seatIdx])
	}
	for _, c := range cards {
		if c == nil {
			continue
		}
		// Keep lands (always useful) and affordable spells on top.
		cost := gameengine.ManaCostOf(c)
		if typeLineContains(c, "land") || cost <= avail+2 {
			top = append(top, c)
		} else {
			bottom = append(bottom, c)
		}
	}
	// If everything went to bottom, keep at least the cheapest on top.
	if len(top) == 0 && len(bottom) > 0 {
		top = append(top, bottom[0])
		bottom = bottom[1:]
	}
	return top, bottom
}

// ChoosePutBack — put highest-CMC cards back on top. Used by Brainstorm.
// Returns exactly `count` cards from hand, highest CMC first (least
// useful cards go back on top).
func (*GreedyHat) ChoosePutBack(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card, count int) []*gameengine.Card {
	if count <= 0 || len(hand) == 0 {
		return nil
	}
	ranked := make([]*gameengine.Card, len(hand))
	copy(ranked, hand)
	sort.SliceStable(ranked, func(i, j int) bool {
		ci, cj := gameengine.ManaCostOf(ranked[i]), gameengine.ManaCostOf(ranked[j])
		if ci != cj {
			return ci > cj // highest CMC first → put back the most expensive
		}
		return ranked[i].DisplayName() < ranked[j].DisplayName()
	})
	if count > len(ranked) {
		count = len(ranked)
	}
	return ranked[:count]
}

// ChooseSurveil — §701.46: put expensive cards in graveyard (fuel for
// reanimation), keep cheap/lands on top.
func (*GreedyHat) ChooseSurveil(gs *gameengine.GameState, seatIdx int, cards []*gameengine.Card) (graveyard []*gameengine.Card, top []*gameengine.Card) {
	if len(cards) == 0 {
		return nil, nil
	}
	avail := 0
	if seatIdx >= 0 && seatIdx < len(gs.Seats) {
		avail = gameengine.AvailableManaEstimate(gs, gs.Seats[seatIdx])
	}
	for _, c := range cards {
		if c == nil {
			continue
		}
		cost := gameengine.ManaCostOf(c)
		if typeLineContains(c, "land") || cost <= avail+1 {
			top = append(top, c)
		} else {
			graveyard = append(graveyard, c)
		}
	}
	return graveyard, top
}

// ObserveEvent — no-op for the stateless baseline.
func (*GreedyHat) ShouldConcede(gs *gameengine.GameState, seatIdx int) bool { return false }

func (*GreedyHat) ObserveEvent(gs *gameengine.GameState, seatIdx int, event *gameengine.Event) {
}

// ---------------------------------------------------------------------
// helpers (shared with PokerHat)
// ---------------------------------------------------------------------

// typeLineContains returns true if any of the card's AST-cached types
// contains substr (lowercased). Tokens with no AST rely on Card.Types
// being spelled out.
func typeLineContains(card *gameengine.Card, substr string) bool {
	if card == nil {
		return false
	}
	ss := strings.ToLower(substr)
	for _, t := range card.Types {
		if strings.Contains(strings.ToLower(t), ss) {
			return true
		}
	}
	return false
}
