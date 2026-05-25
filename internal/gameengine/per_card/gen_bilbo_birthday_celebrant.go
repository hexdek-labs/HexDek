package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBilboBirthdayCelebrant wires Bilbo, Birthday Celebrant.
//
// Oracle text (Scryfall, verified 2026-05-09):
//
//	If you would gain life, you gain that much life plus 1 instead.
//	{2}{W}{B}{G}, {T}, Exile Bilbo: Search your library for any number
//	of creature cards, put them onto the battlefield, then shuffle.
//	Activate only if you have 111 or more life.
//
// Implementation:
//   - OnETB registers a §614 `would_gain_life` replacement that adds 1 to
//     any positive life-gain amount the controller would receive.
//     Mirrors Boon Reflection / Alhammarret's Archive style.
//   - OnActivated implements the Birthday Bash. We gate on:
//       * controller has 111+ life
//       * controller can pay {2}{W}{B}{G} (modeled as 5 generic from the
//         engine's mana pool — colored cost is enforced by the engine
//         dispatcher, this is a defensive top-up gate)
//       * Bilbo is untapped
//     Then we tap Bilbo, exile her, and dump every creature card in our
//     library onto the battlefield. The "any number" choice is resolved
//     by always taking ALL creature cards (Bilbo's payoff is volume —
//     leaving creatures on top of a fresh shuffle is strictly worse).
//     Library is shuffled afterward.
func registerBilboBirthdayCelebrant(r *Registry) {
	r.OnETB("Bilbo, Birthday Celebrant", bilboBirthdayCelebrantETB)
	r.OnActivated("Bilbo, Birthday Celebrant", bilboBirthdayCelebrantActivate)
}

func bilboBirthdayCelebrantETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "bilbo_birthday_life_gain_plus_one"
	if gs == nil || perm == nil {
		return
	}
	controller := perm.Controller
	if controller < 0 || controller >= len(gs.Seats) {
		return
	}
	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_gain_life",
		HandlerID:      "Bilbo, Birthday Celebrant:life_plus_one:" + strconv.Itoa(perm.Timestamp),
		SourcePerm:     perm,
		ControllerSeat: controller,
		Timestamp:      perm.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
			return ev != nil && ev.TargetSeat == controller && ev.Count() > 0
		},
		ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
			before := ev.Count()
			ev.SetCount(before + 1)
			gs.LogEvent(gameengine.Event{
				Kind:   "replacement_applied",
				Seat:   controller,
				Source: "Bilbo, Birthday Celebrant",
				Amount: before + 1,
				Details: map[string]interface{}{
					"slug":   slug,
					"rule":   "614",
					"effect": "life_gain_plus_one",
					"before": before,
					"after":  before + 1,
				},
			})
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     controller,
		"replaces": "would_gain_life",
	})
}

func bilboBirthdayCelebrantActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "bilbo_birthday_birthday_bash"
	if gs == nil || src == nil {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	if seat.Life < 111 {
		emitFail(gs, slug, src.Card.DisplayName(), "life_below_111", map[string]interface{}{
			"life": seat.Life,
		})
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	// Defensive cost top-up: {2}{W}{B}{G} = 5 total. Engine dispatcher
	// usually has already deducted; only deduct here if the pool still
	// covers it.
	if seat.ManaPool >= 5 {
		seat.ManaPool -= 5
	}
	src.Tapped = true
	// Exile Bilbo from the battlefield through the canonical exit API so
	// §614 would_be_exiled replacements + §903.9b commander redirect +
	// aura detach + replacement-effect unregister all fire. (Bilbo is a
	// legendary creature commonly used as a commander; the redirect path
	// matters when she's the player's commander.) Source = src so the
	// exile-self attribution is correct in the event log.
	gameengine.ExilePermanent(gs, src, src)

	// Pull every creature card out of the library, then shuffle.
	var keep []*gameengine.Card
	pulled := []*gameengine.Card{}
	for _, c := range seat.Library {
		if c != nil && cardHasType(c, "creature") {
			pulled = append(pulled, c)
		} else {
			keep = append(keep, c)
		}
	}
	seat.Library = keep
	for _, c := range pulled {
		enterBattlefieldWithETB(gs, seatIdx, c, false)
	}
	shuffleLibraryPerCard(gs, seatIdx)

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":             seatIdx,
		"creatures_pulled": len(pulled),
		"life":             seat.Life,
	})
}
