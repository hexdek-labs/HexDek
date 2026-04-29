// Deprecated: OctoHat was test-only (per 7174n1c) and is superseded by YggdrasilHat
// for all tournament/gameplay use. Retained for engine stress-testing only.
//
// Package hat — OctoHat stress-test policy.
//
// OctoHat intentionally plays badly to force the rules engine to resolve
// as many interactions as possible per turn. Cast every castable spell,
// activate every activatable ability, say YES to every "may", never hold
// mana, never pass priority when a counter is in hand. Winrate with
// OctoHat is meaningless by design.
//
// Use OctoHat to stress:
//   - trigger ordering + reactive firing ("bless you" observer triggers)
//   - cast-count scaling (storm observers fire on every cast)
//   - mana pool churn (every available mana gets spent)
//   - "may" clause optionality (OctoHat always says yes)
//   - multi-mode spell resolution
//   - activated ability dispatch
//   - stack depth + loop detection
//   - counter-war dynamics
//
// Design directive from 7174n1c (2026-04-16):
//   "Octopus-like Hat policy that will encourage each player to cast and
//   activate all available abilities each turn. ... This OctoHat should
//   just try to force every nook and cranny edge case to happen."

package hat

import (
	"sort"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Compile-time check that OctoHat satisfies gameengine.Hat.
var _ gameengine.Hat = (*OctoHat)(nil)

// OctoHat is a stateless stress-test policy.
type OctoHat struct{}

// ---------------------------------------------------------------------
// Mulligan — never mulligan. More cards = more casts = more interactions.
// ---------------------------------------------------------------------

func (*OctoHat) ChooseMulligan(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card) bool {
	return false
}

// ---------------------------------------------------------------------
// Land drop — prefer non-basic (more abilities to trigger).
// ---------------------------------------------------------------------

func (*OctoHat) ChooseLandToPlay(gs *gameengine.GameState, seatIdx int, lands []*gameengine.Card) *gameengine.Card {
	if len(lands) == 0 {
		return nil
	}
	// Prefer non-basic. Tie-break alphabetically for determinism.
	nonBasic := make([]*gameengine.Card, 0, len(lands))
	basic := make([]*gameengine.Card, 0, len(lands))
	for _, c := range lands {
		if c == nil {
			continue
		}
		if strings.Contains(strings.ToLower(c.TypeLine), "basic") {
			basic = append(basic, c)
		} else {
			nonBasic = append(nonBasic, c)
		}
	}
	pool := nonBasic
	if len(pool) == 0 {
		pool = basic
	}
	sort.SliceStable(pool, func(i, j int) bool {
		return pool[i].Name < pool[j].Name
	})
	return pool[0]
}

// ---------------------------------------------------------------------
// Casting — cheapest first, cast everything including counterspells.
// ---------------------------------------------------------------------

func (*OctoHat) ChooseCastFromHand(gs *gameengine.GameState, seatIdx int, castable []*gameengine.Card) *gameengine.Card {
	if len(castable) == 0 {
		return nil
	}
	pool := make([]*gameengine.Card, 0, len(castable))
	for _, c := range castable {
		if c != nil {
			pool = append(pool, c)
		}
	}
	if len(pool) == 0 {
		return nil
	}
	// Sort ascending CMC — cheap first so we can fit more casts per turn.
	sort.SliceStable(pool, func(i, j int) bool {
		if pool[i].CMC != pool[j].CMC {
			return pool[i].CMC < pool[j].CMC
		}
		return pool[i].Name < pool[j].Name
	})
	return pool[0]
}

// ---------------------------------------------------------------------
// Activation — activate the first available ability always.
// ---------------------------------------------------------------------

func (*OctoHat) ChooseActivation(gs *gameengine.GameState, seatIdx int, options []gameengine.Activation) *gameengine.Activation {
	if len(options) == 0 {
		return nil
	}
	return &options[0]
}

// ---------------------------------------------------------------------
// Combat — attack with every creature, block every attacker (up to 2x).
// ---------------------------------------------------------------------

func (*OctoHat) ChooseAttackers(gs *gameengine.GameState, seatIdx int, legal []*gameengine.Permanent) []*gameengine.Permanent {
	out := make([]*gameengine.Permanent, 0, len(legal))
	for _, p := range legal {
		if p != nil {
			out = append(out, p)
		}
	}
	return out
}

// ChooseAttackTarget — rotate targets across opponents based on attacker id.
// Spreads commander damage across seats to stress multi-seat damage tracking.
func (*OctoHat) ChooseAttackTarget(gs *gameengine.GameState, seatIdx int, attacker *gameengine.Permanent, legalDefenders []int) int {
	if len(legalDefenders) == 0 {
		return seatIdx
	}
	if len(legalDefenders) == 1 {
		return legalDefenders[0]
	}
	// Deterministic rotation: use attacker's ID mod num-defenders.
	// Permanent.ID isn't exposed directly — fall back to timestamp.
	idx := 0
	if attacker != nil {
		idx = int(attacker.Timestamp) % len(legalDefenders)
	}
	if idx < 0 {
		idx = 0
	}
	return legalDefenders[idx]
}

// ---------------------------------------------------------------------
// Stack response — counter everything we can.
// ---------------------------------------------------------------------

func (*OctoHat) ChooseResponse(gs *gameengine.GameState, seatIdx int, top *gameengine.StackItem) *gameengine.StackItem {
	if top == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	if top.Controller == seatIdx {
		return nil
	}
	if top.Countered {
		return nil
	}
	if gameengine.SplitSecondActive(gs) {
		return nil
	}
	if gameengine.OppRestrictsDefenderToSorcerySpeed(gs, seatIdx) {
		return nil
	}
	// No threat-score gating — any counter we can afford fires.
	// Filter-aware: only pick counters whose filter matches the target.
	seat := gs.Seats[seatIdx]
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		ceff := gameengine.CounterSpellEffectOf(c)
		if ceff == nil {
			continue
		}
		if gameengine.ManaCostOf(c) > seat.ManaPool {
			continue
		}
		if !gameengine.CounterCanTarget(ceff, top) {
			continue
		}
		return &gameengine.StackItem{
			Controller: seatIdx,
			Card:       c,
			Effect:     ceff,
		}
	}
	return nil
}

// AssignBlockers — block every attacker with up to 2 blockers each,
// chumping freely. Stresses multi-block damage distribution code paths.
func (*OctoHat) AssignBlockers(gs *gameengine.GameState, seatIdx int, attackers []*gameengine.Permanent) map[*gameengine.Permanent][]*gameengine.Permanent {
	out := make(map[*gameengine.Permanent][]*gameengine.Permanent, len(attackers))
	for _, a := range attackers {
		out[a] = nil
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return out
	}
	seat := gs.Seats[seatIdx]
	pool := make([]*gameengine.Permanent, 0, len(seat.Battlefield))
	for _, p := range seat.Battlefield {
		if p != nil && p.IsCreature() && !p.Tapped {
			pool = append(pool, p)
		}
	}
	used := make(map[*gameengine.Permanent]bool, len(pool))
	for _, atk := range attackers {
		if atk == nil {
			continue
		}
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
		// Assign up to 2 blockers per attacker — forces multi-block
		// damage-distribution code to execute.
		limit := 2
		if limit > len(legal) {
			limit = len(legal)
		}
		chosen := legal[:limit]
		for _, b := range chosen {
			used[b] = true
		}
		out[atk] = chosen
	}
	return out
}

// ---------------------------------------------------------------------
// Targeting — first legal target, no threat scoring.
// ---------------------------------------------------------------------

func (*OctoHat) ChooseTarget(gs *gameengine.GameState, seatIdx int, filter gameast.Filter, legal []gameengine.Target) gameengine.Target {
	if len(legal) == 0 {
		return gameengine.Target{Kind: gameengine.TargetKindNone}
	}
	return legal[0]
}

// ---------------------------------------------------------------------
// Modal — pick ALL modes to stress multi-mode resolution paths.
// ---------------------------------------------------------------------

func (*OctoHat) ChooseMode(gs *gameengine.GameState, seatIdx int, modes []gameast.Effect) int {
	if len(modes) == 0 {
		return -1
	}
	// Interface only returns one mode; pick the first.
	// Multi-mode selection would require a protocol change (returning []int).
	// OctoHat's "all modes" design intent needs that extension.
	return 0
}

// ---------------------------------------------------------------------
// Commander decisions — always cast, always redirect to command zone.
// ---------------------------------------------------------------------

func (*OctoHat) ShouldCastCommander(gs *gameengine.GameState, seatIdx int, commanderName string, tax int) bool {
	return true
}

func (*OctoHat) ShouldRedirectCommanderZone(gs *gameengine.GameState, seatIdx int, commander *gameengine.Card, to string) bool {
	return true
}

// ---------------------------------------------------------------------
// Replacements — self-controlled first (matches GreedyHat; OctoHat's
// stress test doesn't care about order, all replacements fire regardless).
// ---------------------------------------------------------------------

func (*OctoHat) OrderReplacements(gs *gameengine.GameState, seatIdx int, candidates []*gameengine.ReplacementEffect) []*gameengine.ReplacementEffect {
	if len(candidates) <= 1 {
		return candidates
	}
	out := make([]*gameengine.ReplacementEffect, 0, len(candidates))
	for _, r := range candidates {
		if r != nil && r.ControllerSeat == seatIdx {
			out = append(out, r)
		}
	}
	for _, r := range candidates {
		if r != nil && r.ControllerSeat != seatIdx {
			out = append(out, r)
		}
	}
	return out
}

// ---------------------------------------------------------------------
// Discard — protect the expensive bombs. Discard LOWEST CMC first.
// ---------------------------------------------------------------------

func (*OctoHat) ChooseDiscard(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card, n int) []*gameengine.Card {
	if n <= 0 || len(hand) == 0 {
		return nil
	}
	ranked := make([]*gameengine.Card, len(hand))
	copy(ranked, hand)
	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].CMC < ranked[j].CMC // ascending
	})
	if n > len(ranked) {
		n = len(ranked)
	}
	return ranked[:n]
}

// ---------------------------------------------------------------------
// Trigger ordering — arrival order (stateless).
// ---------------------------------------------------------------------

func (*OctoHat) OrderTriggers(gs *gameengine.GameState, seatIdx int, triggers []*gameengine.StackItem) []*gameengine.StackItem {
	return triggers
}

// ---------------------------------------------------------------------
// X costs — spend everything to max out X. §107.3.
// ---------------------------------------------------------------------

func (*OctoHat) ChooseX(gs *gameengine.GameState, seatIdx int, card *gameengine.Card, availableMana int) int {
	if availableMana < 0 {
		return 0
	}
	return availableMana
}

// ---------------------------------------------------------------------
// Mulligan bottom cards — bottom the cheapest (keep bombs). §103.5.
// ---------------------------------------------------------------------

func (*OctoHat) ChooseBottomCards(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card, count int) []*gameengine.Card {
	if count <= 0 || len(hand) == 0 {
		return nil
	}
	ranked := make([]*gameengine.Card, len(hand))
	copy(ranked, hand)
	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].CMC < ranked[j].CMC // ascending — cheapest first
	})
	if count > len(ranked) {
		count = len(ranked)
	}
	return ranked[:count]
}

// ---------------------------------------------------------------------
// Put-back — put the cheapest cards back (keep bombs in hand).
// ---------------------------------------------------------------------

func (*OctoHat) ChoosePutBack(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card, count int) []*gameengine.Card {
	if count <= 0 || len(hand) == 0 {
		return nil
	}
	ranked := make([]*gameengine.Card, len(hand))
	copy(ranked, hand)
	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].CMC < ranked[j].CMC // ascending — cheapest first
	})
	if count > len(ranked) {
		count = len(ranked)
	}
	return ranked[:count]
}

// ---------------------------------------------------------------------
// Scry / Surveil — put everything on top (maximize cards seen).
// ---------------------------------------------------------------------

func (*OctoHat) ChooseScry(gs *gameengine.GameState, seatIdx int, cards []*gameengine.Card) (top []*gameengine.Card, bottom []*gameengine.Card) {
	// Keep everything on top to maximize interactions.
	return append([]*gameengine.Card{}, cards...), nil
}

func (*OctoHat) ChooseSurveil(gs *gameengine.GameState, seatIdx int, cards []*gameengine.Card) (graveyard []*gameengine.Card, top []*gameengine.Card) {
	// Put everything in graveyard to fuel reanimation / escape / delve.
	return append([]*gameengine.Card{}, cards...), nil
}

// ---------------------------------------------------------------------
// Observation — no-op (stateless stress policy).
// ---------------------------------------------------------------------

func (*OctoHat) ShouldConcede(gs *gameengine.GameState, seatIdx int) bool { return false }

func (*OctoHat) ObserveEvent(gs *gameengine.GameState, seatIdx int, event *gameengine.Event) {
}
