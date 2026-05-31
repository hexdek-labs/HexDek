package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Anikthea, Hand of Erebos — {2}{W}{B} 3/3 Legendary Enchantment Creature
// — God.
//
//   Menace
//   Other enchantment creatures you control have menace.
//   Whenever Anikthea enters or attacks, exile up to one target non-Aura
//   enchantment card from your graveyard. Create a token that's a copy
//   of that card, except it's a 3/3 black Zombie creature in addition to
//   its other types.
//
// Implementation: OnETB + OnTrigger creature_attacks both invoke the
// same picker. Greedy "may exile" = always yes (the typical Anikthea
// line). Picks highest-CMC non-Aura enchantment from controller's
// graveyard (most-valuable reanimate). MintTokenAsCopyOf for the token
// (Calix pattern at calix_guided_by_fate.go:158), then stamps
// BasePower/BaseToughness=3, appends "creature"/"zombie"/"pip:B" so
// downstream characteristic queries see the 3/3 black Zombie addendum.
// Original types/abilities preserved per "in addition to its other types".
// Menace static and other-enchantment-creatures-have-menace static are
// keyword-pipeline territory.

func init() {
	registerAniktheaHandOfErebos(Global())
	AddResetHook(registerAniktheaHandOfErebos)
}

func registerAniktheaHandOfErebos(r *Registry) {
	r.OnETB("Anikthea, Hand of Erebos", aniktheaETB)
	r.OnTrigger("Anikthea, Hand of Erebos", "creature_attacks", aniktheaAttack)
}

func aniktheaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	aniktheaReanimate(gs, perm, "etb")
}

func aniktheaAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	aniktheaReanimate(gs, perm, "attack")
}

func aniktheaReanimate(gs *gameengine.GameState, perm *gameengine.Permanent, source string) {
	const slug = "anikthea_zombie_enchantment_copy"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := perm.Controller
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}
	pick := pickHighestCMCNonAuraEnchantmentFromGraveyard(s.Graveyard)
	if pick == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   seat,
			"source": source,
			"reason": "no_eligible_enchantment_in_graveyard",
		})
		return
	}
	copyCard := gameengine.MintTokenAsCopyOf(gs, pick, seat, gameengine.CurrentMintEnablerID(gs))
	if copyCard == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "mint_token_returned_nil", nil)
		return
	}
	copyCard.IsCopy = true
	// "In addition to its other types" — stamp 3/3 BR Zombie creature
	// addendum without dropping the original type/subtype set.
	copyCard.BasePower = 3
	copyCard.BaseToughness = 3
	if !cardHasType(copyCard, "creature") {
		copyCard.Types = append(copyCard.Types, "creature")
	}
	if !cardHasSubtype(copyCard, "zombie") {
		copyCard.Types = append(copyCard.Types, "zombie")
	}
	// Black pip for color identity.
	copyCard.Types = append(copyCard.Types, "pip:B")
	// Strip Legendary supertype (token copies of legendary enchantments
	// shouldn't immediately legend-rule themselves to the graveyard —
	// defense in depth mirroring Calix pattern).
	filtered := copyCard.Types[:0]
	for _, t := range copyCard.Types {
		if t == "legendary" || t == "Legendary" {
			continue
		}
		filtered = append(filtered, t)
	}
	copyCard.Types = filtered
	// Exile the source card from graveyard (oracle: "exile up to one
	// target non-Aura enchantment card from your graveyard").
	moveCardBetweenZones(gs, seat, pick, "graveyard", "exile", "anikthea_exile_for_copy")
	enterBattlefieldWithETB(gs, seat, copyCard, false)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      seat,
		"source":    source,
		"exiled":    pick.DisplayName(),
		"token":     copyCard.DisplayName(),
	})
}

func pickHighestCMCNonAuraEnchantmentFromGraveyard(gy []*gameengine.Card) *gameengine.Card {
	var best *gameengine.Card
	bestCMC := -1
	for _, c := range gy {
		if c == nil || !cardHasType(c, "enchantment") {
			continue
		}
		if cardHasSubtype(c, "aura") {
			continue
		}
		cmc := cardCMC(c)
		if cmc > bestCMC {
			bestCMC = cmc
			best = c
		}
	}
	return best
}
