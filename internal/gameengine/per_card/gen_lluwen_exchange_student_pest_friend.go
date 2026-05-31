package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLluwenExchangeStudentPestFriend wires Lluwen, Exchange Student // Pest Friend.
//
// Oracle text:
//
//	Front face — Lluwen, Exchange Student ({2}{B}{G}, Legendary Creature —
//	Human Student, 2/4):
//
//	  Lluwen enters prepared. (While it's prepared, you may cast a copy of
//	  its spell. Doing so unprepares it.)
//	  Exile a creature card from your graveyard: Lluwen becomes prepared.
//	  Activate only as a sorcery.
//
//	Back face — Pest Friend ({B/G}, Sorcery):
//
//	  Create a 1/1 black and green Pest creature token with "Whenever this
//	  token attacks, you gain 1 life."
//
// Implementation:
//   - OnETB: set perm.Prepared = true, then auto-resolve a copy of Pest
//     Friend (create the Pest token), then Unprepare.
//   - OnActivated: find a creature card in controller's graveyard, exile
//     it via MoveCard, set perm.Prepared = true, auto-resolve (create
//     Pest token), then Unprepare.
//   - The Pest token has an attack trigger for +1 life. We register an
//     OnTrigger for "Pest Token" on "attack" to gain 1 life.
//   - emitPartial flags the stack/mana-cost simplification.
func registerLluwenExchangeStudentPestFriend(r *Registry) {
	r.OnETB("Lluwen, Exchange Student // Pest Friend", lluwenETB)
	r.OnETB("Lluwen, Exchange Student", lluwenETB)
	r.OnActivated("Lluwen, Exchange Student // Pest Friend", lluwenActivated)
	r.OnActivated("Lluwen, Exchange Student", lluwenActivated)
	// R60 batch 5: wire the Pest Token's "Whenever this token attacks,
	// you gain 1 life" printed-on-token trigger. The token has
	// Flags["pest_attack_lifegain"] = 1 stamped at mint, which gates
	// the GainLife so handlers registered to the generic "Pest Token"
	// name don't fire for unrelated Pest tokens minted by other cards.
	r.OnTrigger("Pest Token", "creature_attacks", lluwenPestAttackLifegain)
}

func lluwenPestAttackLifegain(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "lluwen_pest_attack_lifegain"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	if perm.Flags == nil || perm.Flags["pest_attack_lifegain"] != 1 {
		// Not a Lluwen-minted Pest — some other card created this
		// "Pest Token" without the flag, so the printed-on-token
		// trigger doesn't apply.
		return
	}
	gameengine.GainLife(gs, perm.Controller, 1, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"gained": 1,
	})
}

// lluwenETB fires when Lluwen enters the battlefield. "Lluwen enters
// prepared" — set Prepared, resolve Pest Friend copy, then Unprepare.
func lluwenETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "lluwen_exchange_student_etb_prepared_copy"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	// Mark Lluwen as prepared.
	perm.Prepared = true
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["prepared"] = 1

	// Resolve Pest Friend copy: create a Pest token.
	lluwenCreatePestToken(gs, perm)

	// Unprepare after resolving the copy.
	gameengine.Unprepare(perm)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"copied_back": "Pest Friend",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"prepare_keyword_resolves_back_face_directly_skipping_stack_and_mana_cost")
}

// lluwenActivated fires when the "Exile a creature card from your
// graveyard: Lluwen becomes prepared" ability is activated.
func lluwenActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "lluwen_exchange_student_activate_prepared_copy"
	if gs == nil || src == nil {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Lost {
		return
	}

	// Find a creature card in controller's graveyard to exile.
	var target *gameengine.Card
	for _, c := range seat.Graveyard {
		if c != nil && cardHasType(c, "creature") {
			target = c
			break
		}
	}
	if target == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_creature_in_graveyard", nil)
		return
	}

	// Exile the creature card (cost of the ability).
	gameengine.MoveCard(gs, target, seatIdx, "graveyard", "exile", "lluwen_activate_cost")

	// Mark Lluwen as prepared.
	src.Prepared = true
	if src.Flags == nil {
		src.Flags = map[string]int{}
	}
	src.Flags["prepared"] = 1

	// Resolve Pest Friend copy: create a Pest token.
	lluwenCreatePestToken(gs, src)

	// Unprepare after resolving the copy.
	gameengine.Unprepare(src)

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":         seatIdx,
		"exiled_card":  target.DisplayName(),
		"copied_back":  "Pest Friend",
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"prepare_keyword_resolves_back_face_directly_skipping_stack_and_mana_cost")
}

// lluwenCreatePestToken creates a 1/1 black and green Pest creature token
// with "Whenever this token attacks, you gain 1 life." The attack-trigger
// life gain is handled via the pest token's Flags and a per_card trigger
// on the generic "Pest Token" card name — but since the token's Name
// varies across sets, we tag it via Flags["pest_attack_lifegain"] and
// handle the trigger inline via the combat system's per-card dispatch.
// MVP: the lifegain-on-attack is logged as a partial; the token itself
// is faithfully created.
func lluwenCreatePestToken(gs *gameengine.GameState, src *gameengine.Permanent) {
	if gs == nil || src == nil {
		return
	}
	token := &gameengine.Card{
		Name:          "Pest Token",
		Owner:         src.Controller,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"creature", "token", "pest"},
		Colors:        []string{"B", "G"},
	}
	perm := enterBattlefieldWithETB(gs, src.Controller, token, false)
	if perm != nil {
		if perm.Flags == nil {
			perm.Flags = map[string]int{}
		}
		perm.Flags["pest_attack_lifegain"] = 1
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "create_token",
		Seat:   src.Controller,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"token":  "1/1 B/G Pest",
			"reason": "pest_friend_copy",
		},
	})
}
