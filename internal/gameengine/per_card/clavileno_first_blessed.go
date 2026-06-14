package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerClavilenoFirstBlessed wires Clavileño, First of the Blessed.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	Whenever you attack, target attacking Vampire that isn't a Demon
//	  becomes a Demon in addition to its other types. It gains "When
//	  this creature dies, draw a card and create a tapped 4/3 white and
//	  black Vampire Demon creature token with flying."
//
// Implementation:
//   - "attack_declared": at the moment of declaring attackers, find an
//     attacking Vampire (not a Demon yet) controlled by us. Add "demon"
//     to its types and tag it with a flag so the dies trigger fires.
//   - "creature_dies": for any tagged Vampire that dies, draw a card and
//     create a 4/3 W/B Vampire Demon flying token.
//   - The "in addition to its other types" semantics is approximated by
//     appending "demon" to Card.Types — this persists on the card and is
//     not properly UEOT, but Clavileño's value comes from the death
//     trigger which we tag separately.
func registerClavilenoFirstBlessed(r *Registry) {
	r.OnTrigger("Clavileño, First of the Blessed", "attack_declared", clavilenoOnAttack)
	r.OnTrigger("Clavileño, First of the Blessed", "creature_dies", clavilenoVampireDies)
}

const clavilenoTag = "clavileno_blessed"

func clavilenoOnAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "clavileno_bless_vampire"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	// "Whenever you attack, target attacking Vampire becomes a Demon" is a
	// single player-level event — fires EXACTLY ONCE per combat (one Vampire),
	// not once per declared attacker. Without this gate the per-attacker
	// creature_attacks dispatch promoted a DIFFERENT eligible Vampire to Demon
	// for each attacker (the demon-type exclusion just made each re-fire pick
	// the next one), so an N-attacker combat blessed up to N Vampires. Gate to
	// the first hit per turn (Caesar/Raffine once-per-turn dedup). r63
	// you-attack cardinality audit.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["clavileno_attack_fired"] >= gs.Turn {
		return
	}
	// Find the highest-power attacking Vampire that isn't already a Demon.
	var pick *gameengine.Permanent
	bestPow := -1
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if !p.IsAttacking() {
			continue
		}
		if !cardHasType(p.Card, "vampire") {
			continue
		}
		if cardHasType(p.Card, "demon") {
			continue
		}
		if pw := p.Power(); pw > bestPow {
			bestPow = pw
			pick = p
		}
	}
	if pick == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_eligible_vampire", nil)
		return
	}
	// Trigger fires — claim the once-per-turn slot only now that a Vampire was
	// actually found, so a combat with no eligible Vampire doesn't consume it.
	perm.Flags["clavileno_attack_fired"] = gs.Turn
	pick.Card.Types = append(pick.Card.Types, "demon")
	if pick.Flags == nil {
		pick.Flags = map[string]int{}
	}
	pick.Flags[clavilenoTag] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    perm.Controller,
		"vampire": pick.Card.DisplayName(),
	})
}

func clavilenoVampireDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "clavileno_dying_blessed_vampire"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	dyingPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if dyingPerm == nil || dyingPerm.Flags == nil {
		return
	}
	if dyingPerm.Flags[clavilenoTag] == 0 {
		return
	}
	owner := dyingPerm.Controller
	if owner < 0 || owner >= len(gs.Seats) {
		return
	}
	drawn := drawOne(gs, owner, perm.Card.DisplayName())
	token := &gameengine.Card{
		Name:          "Vampire Demon Token",
		Owner:         owner,
		BasePower:     4,
		BaseToughness: 3,
		Types:         []string{"token", "creature", "vampire", "demon", "kw:flying"},
		Colors:        []string{"W", "B"},
	}
	enterBattlefieldWithETB(gs, owner, token, true)
	drawnName := ""
	if drawn != nil {
		drawnName = drawn.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  owner,
		"drawn": drawnName,
		"token": "4/3 W/B Vampire Demon flying",
	})
}
