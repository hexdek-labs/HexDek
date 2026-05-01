package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRivazOfTheClaw wires Rivaz of the Claw (Commander Legends).
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Legendary Creature — Lizard Warlock. {1}{B}{R}. 3/3. Menace.
//	{T}: Add two mana in any combination of colors. Spend this mana only
//	to cast Dragon creature spells.
//	Once during each of your turns, you may cast a Dragon creature spell
//	from your graveyard.
//	Whenever you cast a Dragon creature spell from your graveyard, it
//	gains "When this creature dies, exile it."
//
// Implementation:
//   - Menace is wired through the AST keyword pipeline.
//   - "{T}: Add two mana in any combination, spend only on Dragon creature
//     spells": tap-for-mana pacted through the AST keyword pipeline. The
//     "spend only on Dragons" restriction is a typed-mana restriction
//     beyond this engine's untyped ManaPool, so we emitPartial on ETB.
//   - "Once during each of your turns, you may cast a Dragon creature
//     spell from your graveyard": modeled as a controller-restricted
//     ZoneCastPermission registered on each Dragon-creature card in the
//     controller's graveyard. Permissions are refreshed on Rivaz ETB,
//     on each of the controller's upkeep steps, and on every creature
//     death (so a Dragon dying mid-turn becomes castable next turn).
//     The once-per-turn limit is enforced by a custom CanPayFn that
//     checks a turn-keyed seat flag, set by PayFn after the cast resolves.
//   - "Whenever you cast a Dragon creature spell from your graveyard,
//     it gains 'When this creature dies, exile it'": handled via two
//     triggers — spell_cast (mark a turn-keyed flag with the cast card's
//     pointer) and permanent_etb (when the matching card ETBs as a
//     creature, register an exile-on-leave-battlefield replacement,
//     which is the engine's standard model for the "exile if dies"
//     family — see RegisterUnearthExileReplacement).
func registerRivazOfTheClaw(r *Registry) {
	r.OnETB("Rivaz of the Claw", rivazETB)
	r.OnTrigger("Rivaz of the Claw", "upkeep_controller", rivazUpkeep)
	r.OnTrigger("Rivaz of the Claw", "creature_dies", rivazCreatureDies)
	r.OnTrigger("Rivaz of the Claw", "spell_cast", rivazSpellCast)
	r.OnTrigger("Rivaz of the Claw", "permanent_etb", rivazPermETB)
}

func rivazETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, "rivaz_mana_restriction", perm.Card.DisplayName(),
		"spend_only_on_dragon_creatures_typed_mana_unimplemented")
	rivazRefreshGrants(gs, perm)
}

func rivazUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	// Reset the once-per-turn flag on the controller's seat.
	seat := gs.Seats[perm.Controller]
	if seat != nil {
		if seat.Flags == nil {
			seat.Flags = map[string]int{}
		}
		delete(seat.Flags, "rivaz_graveyard_cast_used")
	}
	rivazRefreshGrants(gs, perm)
}

func rivazCreatureDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	dyingCard, _ := ctx["card"].(*gameengine.Card)
	ownerSeat, _ := ctx["owner_seat"].(int)
	if dyingCard == nil {
		return
	}
	if ownerSeat != perm.Controller {
		return
	}
	if !cardHasType(dyingCard, "dragon") || !cardHasType(dyingCard, "creature") {
		return
	}
	rivazRegisterGrant(gs, perm.Controller, dyingCard, perm.Card.DisplayName())
}

// rivazRefreshGrants registers a graveyard-cast permission on every
// Dragon creature card currently in Rivaz's controller's graveyard.
func rivazRefreshGrants(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "rivaz_grant_dragon_graveyard_casts"
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	registered := 0
	for _, c := range s.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "dragon") || !cardHasType(c, "creature") {
			continue
		}
		rivazRegisterGrant(gs, seat, c, perm.Card.DisplayName())
		registered++
	}
	if registered > 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":       seat,
			"registered": registered,
		})
	}
}

// rivazRegisterGrant attaches a once-per-turn ZoneCastPermission to the
// given graveyard card. Idempotent: re-registers cleanly if already
// granted.
func rivazRegisterGrant(gs *gameengine.GameState, seat int, card *gameengine.Card, sourceName string) {
	if gs == nil || card == nil {
		return
	}
	if gs.ZoneCastGrants == nil {
		gs.ZoneCastGrants = map[*gameengine.Card]*gameengine.ZoneCastPermission{}
	}
	gs.ZoneCastGrants[card] = &gameengine.ZoneCastPermission{
		Zone:              gameengine.ZoneGraveyard,
		Keyword:           "rivaz_dragon_graveyard_cast",
		ManaCost:          -1, // pay normal mana cost
		RequireController: seat,
		SourceName:        sourceName,
		AdditionalCosts: []*gameengine.AdditionalCost{
			{
				Kind:  "rivaz_once_per_turn",
				Label: "once per your turn limit",
				CanPayFn: func(g *gameengine.GameState, idx int) bool {
					if g == nil || idx < 0 || idx >= len(g.Seats) {
						return false
					}
					ss := g.Seats[idx]
					if ss == nil || ss.Flags == nil {
						return true
					}
					return ss.Flags["rivaz_graveyard_cast_used"] != g.Turn+1
				},
				PayFn: func(g *gameengine.GameState, idx int) bool {
					if g == nil || idx < 0 || idx >= len(g.Seats) {
						return false
					}
					ss := g.Seats[idx]
					if ss == nil {
						return false
					}
					if ss.Flags == nil {
						ss.Flags = map[string]int{}
					}
					ss.Flags["rivaz_graveyard_cast_used"] = g.Turn + 1
					return true
				},
			},
		},
	}
}

func rivazSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	castZone, _ := ctx["cast_zone"].(string)
	if castZone != "graveyard" {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	if !cardHasType(card, "dragon") || !cardHasType(card, "creature") {
		return
	}
	// Mark the card so the next permanent_etb that matches gets the rider.
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	key := "rivaz_pending_rider_t" + strconv.Itoa(gs.Turn)
	// We can't store a pointer in Flags (int map), so stamp by counting:
	// use a per-perm flag instead — see rivazPermETB which keys by card
	// type and turn. We just bump a counter the ETB consumes.
	seat.Flags[key]++
	emit(gs, "rivaz_dragon_graveyard_cast_marked", perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"spell": card.DisplayName(),
		"turn":  gs.Turn,
	})
}

func rivazPermETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	enteringPerm, _ := ctx["permanent"].(*gameengine.Permanent)
	if enteringPerm == nil || enteringPerm.Card == nil {
		return
	}
	if enteringPerm == perm {
		return
	}
	if !cardHasType(enteringPerm.Card, "dragon") || !enteringPerm.IsCreature() {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Flags == nil {
		return
	}
	key := "rivaz_pending_rider_t" + strconv.Itoa(gs.Turn)
	if seat.Flags[key] <= 0 {
		return
	}
	seat.Flags[key]--
	if seat.Flags[key] == 0 {
		delete(seat.Flags, key)
	}
	gameengine.RegisterUnearthExileReplacement(gs, enteringPerm)
	emit(gs, "rivaz_dragon_exile_rider", perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"dragon": enteringPerm.Card.DisplayName(),
	})
}
