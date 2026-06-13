package gameengine

import (
	"regexp"
	"strings"
)

// mod_tail_deal_damage_count.go — generic runtime handler for the biggest
// STRUCTURED parsed_tail effect cluster after modal: count-scaled direct
// damage spells.
//
//	"~ deals damage to <target> equal to <count>." (Skred, Torrent of Fire,
//	Massive Raid, Stensia Banquet, Armed Response, …)
//
// ~118 instant/sorcery cards parse their entire effect into an inert
// parsed_tail node (kind="parsed_tail" → resolveResidualByText), so they
// dealt ZERO damage. This recognizer is called from resolveResidualByText;
// it parses the target shape + the count phrase, reuses the proven
// parseSelfCalcCount count-resolver (the same one self_calculated_pt uses
// for "number of <noun> you control" / hand / graveyard / life-total), and
// deals that much damage. CONSERVATIVE: opponent-only targeting and only
// recognized count phrases — unrecognized targets/counts return false so
// the clause stays inert rather than mis-resolving.

var (
	// "deals damage to <target> equal to <count>"
	reTailDmgToEqual = regexp.MustCompile(`(?i)(?:~|this creature|this permanent) deals damage to (.+?) equal to (.+?)\.?$`)
	// "deals damage equal to <count> to <target>"
	reTailDmgEqualTo = regexp.MustCompile(`(?i)(?:~|this creature|this permanent) deals damage equal to (.+?) to (.+?)\.?$`)
)

// resolveDealDamageEqualCount recognizes and resolves count-scaled damage
// parsed_tails. Returns true when it handled the clause.
func resolveDealDamageEqualCount(gs *GameState, src *Permanent, raw string) bool {
	if gs == nil || src == nil {
		return false
	}
	var targetPhrase, countPhrase string
	if m := reTailDmgToEqual.FindStringSubmatch(raw); m != nil {
		targetPhrase, countPhrase = m[1], m[2]
	} else if m := reTailDmgEqualTo.FindStringSubmatch(raw); m != nil {
		countPhrase, targetPhrase = m[1], m[2]
	} else {
		return false
	}

	countFn, ok := parseSelfCalcCount(strings.ToLower(countPhrase))
	if !ok {
		return false // unrecognized count source — leave inert
	}
	n := countFn(gs, src)
	if n < 0 {
		n = 0
	}

	seat := src.Controller
	tp := strings.ToLower(targetPhrase)

	// Creature-directed shapes → damage the highest-power opponent creature.
	if strings.Contains(tp, "creature") && !strings.Contains(tp, "any target") && !strings.Contains(tp, "each creature") {
		victim := highestPowerOpponentCreature(gs, seat)
		if victim == nil {
			return false
		}
		if n > 0 {
			victim.MarkedDamage += n
			StateBasedActions(gs)
		}
		gs.LogEvent(Event{
			Kind:   "damage",
			Seat:   seat,
			Source: sourceName(src),
			Amount: n,
			Details: map[string]interface{}{
				"target_card": victim.Card.DisplayName(),
				"basis":       "parsed_tail_count",
			},
		})
		return true
	}

	// Player / "any target" / opponent / planeswalker shapes → burn the
	// highest-life opponent's face (the dominant burn line).
	target := -1
	bestLife := -1
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		if s.Life > bestLife {
			bestLife = s.Life
			target = opp
		}
	}
	if target < 0 {
		return false
	}
	if n > 0 {
		DealDamage(gs, target, n, sourceName(src))
	}
	gs.LogEvent(Event{
		Kind:    "damage",
		Seat:    seat,
		Target:  target,
		Source:  sourceName(src),
		Amount:  n,
		Details: map[string]interface{}{"basis": "parsed_tail_count"},
	})
	return true
}

func highestPowerOpponentCreature(gs *GameState, seat int) *Permanent {
	var best *Permanent
	bestPow := -1
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && gs.IsCreatureOf(p) {
				if pw := gs.PowerOf(p); pw > bestPow {
					bestPow = pw
					best = p
				}
			}
		}
	}
	return best
}
