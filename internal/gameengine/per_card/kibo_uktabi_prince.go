package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKiboUktabiPrince wires Kibo, Uktabi Prince.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	{T}: Each player creates a colorless artifact token named Banana with
//	"{T}, Sacrifice this token: Add {R} or {G}. You gain 2 life."
//	Whenever an artifact an opponent controls is put into a graveyard
//	from the battlefield, put a +1/+1 counter on each creature you
//	control that's an Ape or a Monkey.
//	Whenever Kibo attacks, defending player sacrifices an artifact of
//	their choice.
//
// Implementation:
//   - OnActivated(0): tap Kibo, mint one Banana token for every living
//     player. Banana = colorless artifact token; we tag it with
//     ["token", "artifact"] so it shows up in artifact-counts and ETB
//     paths. The Banana's own activated ability is wired separately so
//     that a sacrificing player can cash it in for 1 mana + 2 life.
//   - "permanent_ltb" trigger (battlefield → graveyard, opponent
//     artifact): scan Kibo's controller's creatures and add a +1/+1
//     counter to each Ape or Monkey. The trigger is dispatched once per
//     leaver, so multiple artifacts dying in one event each fire (and
//     the Apes / Monkeys stack counters per oracle).
//   - "creature_attacks" trigger gated on attacker == Kibo: the defending
//     player sacrifices an artifact of their choice. AI heuristic for
//     "their choice": pick the lowest-CMC artifact (the defender would
//     pick the least valuable one), preferring tokens (e.g. Treasure /
//     Banana) over signets / mana rocks.
func registerKiboUktabiPrince(r *Registry) {
	r.OnActivated("Kibo, Uktabi Prince", kiboActivate)
	r.OnTrigger("Kibo, Uktabi Prince", "permanent_ltb", kiboArtifactLTB)
	r.OnTrigger("Kibo, Uktabi Prince", "creature_attacks", kiboAttackTrigger)
	// Banana token's own activated ability — tap, sac, add {R}/{G}, gain 2.
	r.OnActivated("Banana", bananaActivate)
}

func kiboActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "kibo_tap_create_bananas"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	src.Tapped = true

	created := 0
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		token := &gameengine.Card{
			Name:  "Banana",
			Owner: i,
			Types: []string{"token", "artifact"},
		}
		enterBattlefieldWithETB(gs, i, token, false)
		gs.LogEvent(gameengine.Event{
			Kind:   "create_token",
			Seat:   i,
			Source: src.Card.DisplayName(),
			Details: map[string]interface{}{
				"token":  "Banana",
				"reason": "kibo_tap_each_player_banana",
			},
		})
		created++
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     src.Controller,
		"bananas":  created,
		"per_seat": 1,
	})
}

// kiboArtifactLTB fires when any permanent leaves the battlefield. We gate
// on (a) the leaver is an artifact, (b) the destination is the graveyard,
// (c) the controller was NOT Kibo's controller (i.e., an opponent's
// artifact died).
func kiboArtifactLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "kibo_opponent_artifact_died_buff"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	toZone, _ := ctx["to_zone"].(string)
	if toZone != "graveyard" {
		return
	}
	leaverCard, _ := ctx["card"].(*gameengine.Card)
	if leaverCard == nil {
		return
	}
	if !cardHasType(leaverCard, "artifact") {
		return
	}
	leaverController, _ := ctx["controller_seat"].(int)
	if leaverController == perm.Controller {
		return
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	buffed := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if cardHasType(p.Card, "ape") || cardHasType(p.Card, "monkey") {
			p.AddCounter("+1/+1", 1)
			buffed++
		}
	}
	if buffed > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":              perm.Controller,
		"opponent_seat":     leaverController,
		"opponent_artifact": leaverCard.DisplayName(),
		"apes_buffed":       buffed,
	})
}

func kiboAttackTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "kibo_attack_defender_sacrifice_artifact"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	defenderSeat, ok := gameengine.AttackerDefender(atk)
	if !ok || defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}
	defender := gs.Seats[defenderSeat]
	if defender == nil || defender.Lost {
		return
	}

	victim := pickKiboSacVictim(defender)
	if victim == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":          perm.Controller,
			"defender_seat": defenderSeat,
			"sacrificed":    nil,
			"reason":        "no_artifact_to_sacrifice",
		})
		return
	}
	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "kibo_attack_force_sac")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"defender_seat": defenderSeat,
		"sacrificed":    victimName,
	})
}

// pickKiboSacVictim chooses the artifact that the defending player would
// sacrifice. Defender picks; defender minimizes loss. Heuristic:
//  1. Prefer tokens (free artifacts — Treasure, Banana, Clue).
//  2. Otherwise pick the lowest-CMC nonland artifact.
func pickKiboSacVictim(defender *gameengine.Seat) *gameengine.Permanent {
	if defender == nil {
		return nil
	}
	var bestToken *gameengine.Permanent
	var bestNonToken *gameengine.Permanent
	bestCMC := 1 << 30
	for _, p := range defender.Battlefield {
		if p == nil || p.Card == nil || !p.IsArtifact() {
			continue
		}
		if cardHasType(p.Card, "token") {
			if bestToken == nil {
				bestToken = p
			}
			continue
		}
		cmc := cardCMC(p.Card)
		if cmc < bestCMC {
			bestCMC = cmc
			bestNonToken = p
		}
	}
	if bestToken != nil {
		return bestToken
	}
	return bestNonToken
}

// bananaActivate handles the Banana token's own activated ability:
// "{T}, Sacrifice this token: Add {R} or {G}. You gain 2 life."
//
// We treat the cost (tap + sacrifice) as caller-paid in spirit, but
// enforce the tap+sac side-effects here so per-card invocations stay
// consistent. The mana payoff is one generic mana (the engine's ManaPool
// is colorless-on-the-rail), and the controller gains 2 life.
func bananaActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "banana_token_tap_sac_mana_lifegain"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
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

	src.Tapped = true
	gameengine.AddManaFromPermanent(gs, seat, src, "R", 1)
	gameengine.GainLife(gs, seatIdx, 2, src.Card.DisplayName())
	gameengine.SacrificePermanent(gs, src, "banana_token_sac_for_mana")

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      seatIdx,
		"mana":      1,
		"life_gain": 2,
	})
}
