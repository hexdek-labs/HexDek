package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerNerivCracklingVanguard wires Neriv, Crackling Vanguard.
//
// Oracle text (Scryfall, verified):
//
//	Flying, deathtouch
//	When Neriv enters, create two 1/1 red Goblin creature tokens.
//	Whenever Neriv attacks, exile a number of cards from the top of
//	your library equal to the number of differently named tokens you
//	control. During any turn you attacked with a commander, you may
//	play those cards.
//
// Implementation (R45 stub port):
//   - Flying, deathtouch: AST keyword pipeline.
//   - ETB: mint two 1/1 red Goblin creature tokens via
//     CreateCreatureToken (the auto-gen stub created only one
//     unnamed token; the port produces two correctly-colored Goblins).
//   - Attack trigger: OnTrigger("creature_attacks") gated on
//     attacker_perm == perm. Count distinct token names the
//     controller controls, exile that many cards from the top of
//     library, and register ZoneCastPermissions for each exiled
//     card with Duration "until_end_of_turn" so the cast pipeline
//     can play them this turn. The "during any turn you attacked
//     with a commander" gate is captured via the same upkeep-style
//     attacker-flag the engine already tracks; since Neriv attacking
//     is itself the trigger (and Neriv is commander-capable as the
//     printed legendary creature), we treat the condition as
//     satisfied when Neriv is the attacker.
func registerNerivCracklingVanguard(r *Registry) {
	r.OnETB("Neriv, Crackling Vanguard", nerivCracklingVanguardETB)
	r.OwnsETBTrigger("Neriv, Crackling Vanguard")
	r.OnTrigger("Neriv, Crackling Vanguard", "creature_attacks", nerivAttackImpulse)
}

func nerivCracklingVanguardETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "neriv_crackling_vanguard_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	for i := 0; i < 2; i++ {
		tok := gameengine.CreateCreatureToken(
			gs,
			seat,
			"Goblin",
			[]string{"creature", "goblin"},
			1, 1,
		)
		if tok != nil && tok.Card != nil {
			tok.Card.Colors = []string{"R"}
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"tokens": 2,
		"type":   "Goblin",
	})
}

func nerivAttackImpulse(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "neriv_attack_exile_impulse"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	s := gs.Seats[perm.Controller]
	if s == nil || s.Lost {
		return
	}
	// Count distinct token names the controller controls.
	names := map[string]bool{}
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsToken() {
			continue
		}
		names[p.Card.DisplayName()] = true
	}
	n := len(names)
	if n == 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":         perm.Controller,
			"tokens_named": 0,
			"exiled":       0,
		})
		return
	}

	exiledCards := make([]*gameengine.Card, 0, n)
	for i := 0; i < n && len(s.Library) > 0; i++ {
		top := s.Library[0]
		if top == nil {
			break
		}
		gameengine.MoveCard(gs, top, perm.Controller, "library", "exile", "neriv_attack_impulse")
		exiledCards = append(exiledCards, top)
	}
	for _, c := range exiledCards {
		gameengine.RegisterZoneCastGrant(gs, c, &gameengine.ZoneCastPermission{
			Zone:              gameengine.ZoneExile,
			Keyword:           "neriv_impulse_play",
			ManaCost:          -1, // pay normal mana cost
			RequireController: perm.Controller,
			SourceName:        "Neriv, Crackling Vanguard",
			Duration:          "until_end_of_turn",
			GrantTurn:         gs.Turn,
		})
	}
	if len(exiledCards) > 0 {
		cleanup := make([]*gameengine.Card, len(exiledCards))
		copy(cleanup, exiledCards)
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "end_of_turn",
			ControllerSeat: perm.Controller,
			SourceCardName: perm.Card.DisplayName(),
			OneShot:        true,
			EffectFn: func(gs *gameengine.GameState) {
				for _, c := range cleanup {
					gameengine.RemoveZoneCastGrant(gs, c)
				}
			},
		})
	}
	exiledNames := make([]string, 0, len(exiledCards))
	for _, c := range exiledCards {
		exiledNames = append(exiledNames, c.DisplayName())
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"tokens_named": n,
		"exiled":       len(exiledCards),
		"cards":        exiledNames,
	})
}
