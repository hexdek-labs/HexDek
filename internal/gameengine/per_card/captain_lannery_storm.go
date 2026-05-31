package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Captain Lannery Storm — {1}{R} 2/2 Legendary Creature — Human Pirate.
//
//   Haste
//   Whenever Captain Lannery Storm attacks, create a Treasure token.
//   Whenever you sacrifice a Treasure, Captain Lannery Storm gets +1/+0
//   until end of turn.
//
// Implementation:
//   - OnTrigger creature_attacks with attacker_perm == self: spawn
//     Treasure via gameengine.CreateTreasureToken.
//   - OnTrigger artifact_sacrificed (the closest canonical event per
//     zone_change.go:349; no dedicated "treasure_sacrificed" exists).
//     Filter: controller_seat == Lannery's controller AND the sacrificed
//     perm is a Treasure (subtype check). Stamp a Modification with
//     Power=+1 Duration="until_end_of_turn" on Captain Lannery.

func init() {
	registerCaptainLanneryStorm(Global())
	AddResetHook(registerCaptainLanneryStorm)
}

func registerCaptainLanneryStorm(r *Registry) {
	r.OnTrigger("Captain Lannery Storm", "creature_attacks", lanneryAttack)
	r.OnTrigger("Captain Lannery Storm", "artifact_sacrificed", lanneryArtifactSac)
}

func lanneryAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "lannery_attack_treasure"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	gameengine.CreateTreasureToken(gs, perm.Controller)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func lanneryArtifactSac(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "lannery_treasure_sac_pump"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	sacCard, _ := ctx["card"].(*gameengine.Card)
	if sacCard == nil {
		return
	}
	// Filter to Treasure subtype.
	tl := strings.ToLower(strings.Join(sacCard.Types, " "))
	if !strings.Contains(tl, "treasure") {
		return
	}
	perm.Modifications = append(perm.Modifications, gameengine.Modification{
		Power:     1,
		Toughness: 0,
		Duration:  "until_end_of_turn",
		Timestamp: gs.NextTimestamp(),
	})
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"sac_card":    sacCard.DisplayName(),
		"lannery_p":   perm.Power(),
	})
}
