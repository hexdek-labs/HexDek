package per_card

import (
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKefka wires Kefka, Court Mage // Kefka, Ruler of Ruin (DFC).
//
// Front face — Kefka, Court Mage (Legendary Creature — Human Wizard, 4/5):
//
//	Whenever Kefka enters or attacks, each player discards a card.
//	Then you draw a card for each card type among cards discarded
//	this way.
//	{8}: Each opponent sacrifices a permanent of their choice.
//	Transform Kefka. Activate only as a sorcery.
//
// Back face — Kefka, Ruler of Ruin (Legendary Creature — Avatar Wizard, 5/7):
//
//	Flying
//	Whenever an opponent loses life during your turn, you draw that
//	many cards.
//
// Implementation:
//   - ETB + creature_attacks (filtered to Kefka herself) — drive the
//     wheel-of-discard via a shared helper. Snapshot each player's
//     graveyard size, run DiscardN(1, "random"), then walk the new
//     graveyard tail to count distinct CARD TYPES (CR §205.2a — the
//     printed types: artifact, creature, enchantment, instant, land,
//     planeswalker, sorcery, tribal, battle). Supertypes (legendary,
//     basic, snow) and subtypes don't count.
//   - Activated ability — each opponent sacrifices a permanent of THEIR
//     choice (we approximate "their choice" with the sacVictimScore
//     heuristic on each opponent's own board: they'll feed the worst
//     piece). Then TransformPermanent flips Kefka.
//   - Back face — life_lost trigger gated on perm.Transformed AND
//     active player == perm.Controller. Draws ctx["amount"] cards.
//
// DFC dispatch: register the full slash name plus each face — the
// registry's " // " split fallback only catches pre-transform names.
func registerKefka(r *Registry) {
	r.OnETB("Kefka, Court Mage // Kefka, Ruler of Ruin", kefkaWheelETB)
	r.OnETB("Kefka, Court Mage", kefkaWheelETB)
	r.OnTrigger("Kefka, Court Mage // Kefka, Ruler of Ruin", "creature_attacks", kefkaWheelAttack)
	r.OnTrigger("Kefka, Court Mage", "creature_attacks", kefkaWheelAttack)
	r.OnActivated("Kefka, Court Mage // Kefka, Ruler of Ruin", kefkaActivateTransform)
	r.OnActivated("Kefka, Court Mage", kefkaActivateTransform)

	// Back face — life_lost during your turn.
	r.OnTrigger("Kefka, Court Mage // Kefka, Ruler of Ruin", "life_lost", kefkaRulerOfRuinLifeLost)
	r.OnTrigger("Kefka, Court Mage", "life_lost", kefkaRulerOfRuinLifeLost)
	r.OnTrigger("Kefka, Ruler of Ruin", "life_lost", kefkaRulerOfRuinLifeLost)
}

func kefkaWheelETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	// Front-face only: post-transform the back face is an Avatar Wizard
	// without an ETB clause. Gate on Transformed=false defensively.
	if perm.Transformed {
		return
	}
	kefkaWheel(gs, perm, "etb")
}

func kefkaWheelAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	if perm.Transformed {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	kefkaWheel(gs, perm, "attack")
}

func kefkaWheel(gs *gameengine.GameState, perm *gameengine.Permanent, source string) {
	const slug = "kefka_court_mage_wheel"

	// Snapshot graveyard sizes so we can identify which cards each player
	// just discarded (DiscardN doesn't return them).
	preLen := make([]int, len(gs.Seats))
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		preLen[i] = len(s.Graveyard)
	}

	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		if len(s.Hand) == 0 {
			continue
		}
		gameengine.DiscardN(gs, i, 1, "random")
	}

	// Walk each seat's freshly added graveyard cards and union their
	// printed card types. Per CR §205.2a, only true card types count
	// (not supertypes or subtypes).
	typesSeen := map[string]bool{}
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		for j := preLen[i]; j < len(s.Graveyard); j++ {
			c := s.Graveyard[j]
			if c == nil {
				continue
			}
			for _, t := range c.Types {
				switch strings.ToLower(t) {
				case "artifact", "creature", "enchantment", "instant",
					"land", "planeswalker", "sorcery", "tribal", "battle":
					typesSeen[strings.ToLower(t)] = true
				}
			}
		}
	}

	drawn := 0
	for range typesSeen {
		if c := drawOne(gs, perm.Controller, perm.Card.DisplayName()); c != nil {
			drawn++
		}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"trigger_source": source,
		"types_count":    len(typesSeen),
		"drew":           drawn,
	})
}

func kefkaActivateTransform(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "kefka_court_mage_activate_transform"
	if gs == nil || src == nil {
		return
	}
	if src.Transformed {
		return
	}

	sacrificed := map[int]string{}
	for _, oppIdx := range gs.Opponents(src.Controller) {
		opp := gs.Seats[oppIdx]
		if opp == nil || opp.Lost {
			continue
		}
		victim := chooseOpponentSacOfTheirChoice(gs, oppIdx)
		if victim == nil {
			continue
		}
		name := victim.Card.DisplayName()
		gameengine.SacrificePermanent(gs, victim, "kefka_court_mage_activate")
		sacrificed[oppIdx] = name
	}

	if !gameengine.TransformPermanent(gs, src, "kefka_activated_transform") {
		emitPartial(gs, slug, src.Card.DisplayName(),
			"transform_failed_face_data_missing")
		return
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":       src.Controller,
		"sacrificed": sacrificed,
		"to":         "Kefka, Ruler of Ruin",
	})
}

// chooseOpponentSacOfTheirChoice picks the permanent an opponent would
// most willingly part with — they pick, so we model the worst piece on
// their board: prefer tokens, then summoning-sick non-utility creatures,
// then highest-CMC non-engine permanents. Falls back to any non-land.
func chooseOpponentSacOfTheirChoice(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	if seat < 0 || seat >= len(gs.Seats) {
		return nil
	}
	s := gs.Seats[seat]
	if s == nil {
		return nil
	}
	// Pass 1: any token.
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil || p.IsLand() {
			continue
		}
		if cardHasType(p.Card, "token") {
			return p
		}
	}
	// Pass 2: lowest-CMC non-land permanent (creatures preferred since
	// opponents protect their commanders / engines first).
	var best *gameengine.Permanent
	bestCMC := 99
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil || p.IsLand() {
			continue
		}
		cmc := cardCMC(p.Card)
		if cmc < bestCMC {
			bestCMC = cmc
			best = p
		}
	}
	return best
}

func kefkaRulerOfRuinLifeLost(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "kefka_ruler_of_ruin_life_lost_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// Back-face only.
	if !perm.Transformed {
		return
	}
	// Only during your turn.
	if gs.Active != perm.Controller {
		return
	}
	lossSeat, ok := ctx["seat"].(int)
	if !ok {
		return
	}
	if lossSeat == perm.Controller {
		return // only opponents
	}
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}

	drawn := 0
	for i := 0; i < amount; i++ {
		if c := drawOne(gs, perm.Controller, perm.Card.DisplayName()); c == nil {
			break
		}
		drawn++
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"loss_seat": lossSeat,
		"amount":    amount,
		"drew":      drawn,
		"turn":      strconv.Itoa(gs.Turn),
	})
}
