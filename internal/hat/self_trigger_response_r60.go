package hat

import (
	"strings"
	"sync/atomic"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// selfTriggerCounterFires is a process-lifetime atomic counter
// incremented every time ChooseResponse fires a counter against a
// self-controlled trigger. Surfaces observability for the gauntlet
// validation in docs/hat-self-trigger-gauntlet.md without requiring
// audit-log retention. Reset via ResetSelfTriggerCounterFires;
// readable via SelfTriggerCounterFires.
var selfTriggerCounterFires atomic.Int64

// SelfTriggerCounterFires returns the cumulative count of self-trigger
// counter fires across the process lifetime.
func SelfTriggerCounterFires() int64 {
	return selfTriggerCounterFires.Load()
}

// ResetSelfTriggerCounterFires zeroes the counter. Useful for
// gauntlet-scoped measurements that want a clean baseline before
// running games.
func ResetSelfTriggerCounterFires() {
	selfTriggerCounterFires.Store(0)
}

// self_trigger_response_r60.go — R60 round 15+ ChooseResponse audit fix.
//
// Audit: ChooseResponse at yggdrasil.go:6118 short-circuits with
//   if top.Controller == seatIdx || top.Countered { return nil }
// — self-controlled stack items are NEVER responded to. This is the
// correct default for spells (countering our own spell mid-cast is
// almost always wrong), but it misses one real edge case: self-
// controlled TRIGGERS that would harm us.
//
// Concrete scenario: I cast Mulldrifter while controlling Underworld
// Dreams. Mulldrifter ETBs, "draw 2" trigger fires. If I let it
// resolve, Underworld Dreams pings me for 2 (one per card drawn).
// At low life, countering my own trigger saves the game. The same
// pattern fires with Curse of Wizardry, Spelltithe Enforcer-shape
// punishers, and various "whenever you draw a card, you lose N life"
// effects.
//
// shouldCounterOwnTrigger gates the narrow window where countering
// self-controlled triggers is correct, so the broad "skip self-items"
// rule at yggdrasil.go:6118 stays intact for everything else.

// damageOnDrawNeedles are oracle-text substrings on a permanent the
// hat ALREADY CONTROLS that signal "drawing a card hurts me." False
// positives are bad (we'd counter our own value triggers); false
// negatives lose a corner-case rescue but no normal play. Conservative
// list: exact-phrasing matches only.
var damageOnDrawNeedles = []string{
	"whenever you draw a card, you lose",
	"whenever you draw, you lose",
	"whenever you draw a card, draws ... life", // Underworld Dreams family
	"whenever a player draws a card, that player loses",
	"underworld dreams", // explicit name match for the canonical case (engine renders the trigger via card name when raw text is missing)
}

// controllerHasDamageOnDrawPunisher returns the total damage per card
// drawn that the controller's own board punishes them for. Sums across
// multiple stacked punishers (Underworld Dreams + Curse of Wizardry =
// 2 per card). Cards that ping ALL players (Howling Mine-shape) only
// hit us if we're the one drawing — which we are, since the trigger
// is ours.
//
// Returns 0 when no punisher is found.
func controllerHasDamageOnDrawPunisher(gs *gameengine.GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return 0
	}
	totalPerDraw := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || p.PhasedOut {
			continue
		}
		ot := gameengine.OracleTextLower(p.Card)
		if ot == "" {
			continue
		}
		// Underworld Dreams shape — "Whenever an opponent draws a card,
		// that player loses 1 life." But mirror cases (Curse of Wizardry,
		// Spiteful Visions, Black Vise) hit us too. We match the
		// "draws ... loses ... life" shape, which catches the universal
		// version. Punishers limited to "opponent draws" don't apply
		// when WE'RE drawing — skip those.
		hitsUs := false
		switch {
		case strings.Contains(ot, "whenever you draw a card, you lose"),
			strings.Contains(ot, "whenever you draw, you lose"):
			hitsUs = true
		case strings.Contains(ot, "whenever a player draws a card") &&
			strings.Contains(ot, "loses"):
			hitsUs = true
		case strings.Contains(p.Card.DisplayName(), "Underworld Dreams"):
			hitsUs = true // explicit punisher
		case strings.Contains(p.Card.DisplayName(), "Spiteful Visions"):
			hitsUs = true // hits everyone, including us
		case strings.Contains(p.Card.DisplayName(), "Curse of Wizardry"):
			hitsUs = true
		}
		if !hitsUs {
			continue
		}
		// Most punishers deal 1 per draw. Spiteful Visions deals 2.
		// Without a robust number parser, default to 1 per match and
		// add an explicit override for known higher-damage punishers.
		dmg := 1
		if strings.Contains(p.Card.DisplayName(), "Spiteful Visions") {
			dmg = 2
		}
		totalPerDraw += dmg
	}
	return totalPerDraw
}

// estimateDrawTriggerCardCount returns the number of cards the trigger
// would draw if resolved. Reads the trigger source's oracle text.
// Heuristic — "draw a card" returns 1, "draw two cards" returns 2,
// "draw three cards" returns 3. Unknown phrasings return 0 (no signal).
func estimateDrawTriggerCardCount(top *gameengine.StackItem) int {
	if top == nil || top.Source == nil || top.Source.Card == nil {
		return 0
	}
	ot := gameengine.OracleTextLower(top.Source.Card)
	if strings.Contains(ot, "draw three") {
		return 3
	}
	if strings.Contains(ot, "draw two") {
		return 2
	}
	if strings.Contains(ot, "draw a card") || strings.Contains(ot, "draw cards") {
		return 1
	}
	return 0
}

// shouldCounterOwnTrigger reports whether the hat should attempt to
// counter its own triggered ability rather than let it resolve. Returns
// true only when:
//   1. top is a triggered ability (not a spell, not an activated
//      ability the player intentionally activated this turn)
//   2. The trigger source is our own permanent
//   3. The trigger would deal damage to us via a controller-side
//      punisher in play (draw + Underworld Dreams family)
//   4. The projected damage would equal or exceed our current life
//      (we'd lose the game by letting the trigger resolve)
//
// All four gates must hold — false in any one returns false. This is
// intentionally narrow: countering your own triggers is a pathological
// play that should only fire when it literally saves the game.
func (h *YggdrasilHat) shouldCounterOwnTrigger(gs *gameengine.GameState, seatIdx int, top *gameengine.StackItem) bool {
	if gs == nil || top == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	if !isAbilityOnStack(top) {
		return false
	}
	if top.Kind != "triggered" {
		// Activated abilities are intentional spends by us — don't
		// counter our own activations. (The Kind check is the cleanest
		// disambiguator since isAbilityOnStack covers both kinds.)
		return false
	}
	if top.Controller != seatIdx {
		return false
	}
	// Three independent lethal-projection scenarios — OR-chained so any
	// one being lethal fires the counter. Each scenario is a single
	// pure function that returns true iff resolving the trigger would
	// lose us the game; the dispatcher composition makes adding future
	// self-harm scenarios (Bottomless Pit-shape, etc.) a one-line
	// change here.
	if triggerCausesLethalDrawDamage(gs, seatIdx, top) {
		return true
	}
	// R60 round 16+ (see self_harm_signals_r60.go) — self-mill and
	// stated-life-loss extensions.
	if triggerCausesLethalMill(gs, seatIdx, top) {
		return true
	}
	if triggerCausesLethalLifeLoss(gs, seatIdx, top) {
		return true
	}
	return false
}

// triggerCausesLethalDrawDamage is the extracted-for-symmetry version
// of the original PR #410 inline check. Returns true when the trigger
// would draw cards × our damage-on-draw punisher damage ≥ life.
func triggerCausesLethalDrawDamage(gs *gameengine.GameState, seatIdx int, top *gameengine.StackItem) bool {
	if gs == nil || top == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	if top.Controller != seatIdx {
		return false
	}
	dmgPerDraw := controllerHasDamageOnDrawPunisher(gs, seatIdx)
	if dmgPerDraw == 0 {
		return false
	}
	cards := estimateDrawTriggerCardCount(top)
	if cards == 0 {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	return dmgPerDraw*cards >= seat.Life
}
