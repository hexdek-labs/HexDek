package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerUreniOfTheUnwritten wires Ureni of the Unwritten.
//
// Oracle text (TDC, {4}{G}{U}{R}, 7/7 Legendary Spirit Dragon):
//
//	Flying, trample
//	Whenever Ureni enters or attacks, look at the top eight cards of
//	your library. You may put a Dragon creature card from among them
//	onto the battlefield. Put the rest on the bottom of your library
//	in a random order.
//
// Implementation:
//   - Flying/trample: AST keyword pipeline.
//   - OnETB: dig 8, drop best Dragon, bottom rest in random order.
//   - OnTrigger("creature_attacks"): same dig when Ureni declares as
//     attacker.
//
// Dragon-pick heuristic: highest CMC Dragon creature card (you've spent
// {4}{G}{U}{R}, you're cheating in a fatty — pick the biggest fatty).
func registerUreniOfTheUnwritten(r *Registry) {
	r.OnETB("Ureni of the Unwritten", ureniOfTheUnwrittenETB)
	r.OnTrigger("Ureni of the Unwritten", "creature_attacks", ureniOfTheUnwrittenAttack)
}

func ureniOfTheUnwrittenETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	ureniDig(gs, perm, "etb")
}

func ureniOfTheUnwrittenAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	ureniDig(gs, perm, "attack")
}

func ureniDig(gs *gameengine.GameState, perm *gameengine.Permanent, source string) {
	const slug = "ureni_of_the_unwritten_dig"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	if len(seat.Library) == 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "empty_library", map[string]interface{}{
			"trigger": source,
		})
		return
	}

	n := 8
	if len(seat.Library) < n {
		n = len(seat.Library)
	}
	top := append([]*gameengine.Card(nil), seat.Library[:n]...)

	// Pick the highest-CMC Dragon creature card.
	pickIdx := -1
	bestCMC := -1
	for i, c := range top {
		if c == nil {
			continue
		}
		if !cardHasType(c, "creature") || !cardHasType(c, "dragon") {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc > bestCMC {
			bestCMC = cmc
			pickIdx = i
		}
	}

	// Slice off the top N from the library.
	seat.Library = append([]*gameengine.Card(nil), seat.Library[n:]...)

	enteredName := ""
	var rest []*gameengine.Card
	for i, c := range top {
		if i == pickIdx {
			continue
		}
		rest = append(rest, c)
	}

	if pickIdx >= 0 {
		picked := top[pickIdx]
		picked.Owner = perm.Controller
		ent := enterBattlefieldWithETB(gs, perm.Controller, picked, false)
		if ent != nil {
			enteredName = picked.DisplayName()
		}
	}

	// Bottom the rest in random order (gs.Rng for determinism).
	if len(rest) > 0 {
		if gs.Rng != nil {
			gs.Rng.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
		}
		seat.Library = append(seat.Library, rest...)
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"trigger":  source,
		"looked":   n,
		"entered":  enteredName,
		"bottomed": len(rest),
		"cmc":      bestCMC,
	})
}
