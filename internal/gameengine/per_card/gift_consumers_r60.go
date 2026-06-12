package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// gift_consumers_r60.go — per_card OnResolve handlers for the OTJ
// Gift keyword family (CR §702.192).
//
// Per the Versailles Phase 1B audit (PR #477 §3) `gift_promised` /
// `gift_delivered` are engine-emitted events with no per_card
// consumer wired. The engine has the full plumbing — keywords_gift.go's
// CastGift pushes a StackItem flagged `CostMeta["gift_promised"] =
// true` and fires `gift_promised`; `ResolveGift(gs, item)` mints the
// canonical token (Treasure, Clue, Food, Blood, Map, Gold, Powerstone,
// Junk) to the recipient and fires `gift_delivered`. But the
// resolve-time "if the gift was promised, [bonus]" / "if the gift
// wasn't promised, [penalty]" riders had no per_card handler running
// them, so every Gift card's bonus / penalty branch was inert.
//
// Each handler in this file follows the same shape:
//
//   1. Read item.CostMeta["gift_promised"] (defaults to false).
//   2. If promised: call gameengine.ResolveGift to deliver the token
//      "before its other effects" per oracle text on the cards that
//      use that wording. (ResolveGift no-ops cleanly when the
//      recipient was eliminated mid-flight.)
//   3. Run the base effect.
//   4. If promised (or NOT promised, for the inverted riders): run
//      the conditional rider.
//
// Six cards wired:
//
//   - Blooming Blast    (instant, R)  — 2 dmg to creature; +3 dmg to
//                                      that creature's controller if
//                                      gift was promised.
//   - Crumb and Get It  (instant, W)  — +2/+2 friendly UEOT; also
//                                      indestructible UEOT if promised.
//   - Wear Down         (sorcery, G)  — destroy art/ench; destroy 2
//                                      instead if promised.
//   - Nocturnal Hunger  (instant, B)  — destroy creature; lose 2 life
//                                      if NOT promised (inverted).
//   - Wildfire Howl     (sorcery, R)  — 2 dmg to each creature; also
//                                      1 dmg to any target if promised.
//   - Valley Rally      (instant, R)  — your creatures +2/+0 UEOT;
//                                      target friendly gains first
//                                      strike UEOT if promised.

func init() {
	registerGiftConsumersR60(Global())
	AddResetHook(registerGiftConsumersR60)
}

func registerGiftConsumersR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Blooming Blast", bloomingBlastResolve)
	r.OnResolve("Crumb and Get It", crumbAndGetItResolve)
	r.OnResolve("Wear Down", wearDownResolve)
	r.OnResolve("Nocturnal Hunger", nocturnalHungerResolve)
	r.OnResolve("Wildfire Howl", wildfireHowlResolve)
	r.OnResolve("Valley Rally", valleyRallyResolve)
}

// giftPromised returns (true, true) when CostMeta has gift_promised=true,
// else (false, true). The second return tells the caller whether the
// CostMeta lookup itself succeeded — when false, the StackItem is
// malformed and the handler should abstain.
func giftPromised(item *gameengine.StackItem) (bool, bool) {
	if item == nil || item.CostMeta == nil {
		return false, false
	}
	v, ok := item.CostMeta["gift_promised"].(bool)
	if !ok {
		return false, true // CostMeta exists but key absent → not promised
	}
	return v, true
}

// pickBestOppArtOrEnch returns the highest-CMC opp artifact or
// enchantment. Mirrors Gemrazer / Aura Shards target policy.
func pickBestOppArtOrEnch(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	var best *gameengine.Permanent
	bestCMC := -1
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if !p.IsArtifact() && !p.IsEnchantment() {
				continue
			}
			cmc := cardCMC(p.Card)
			if cmc > bestCMC {
				bestCMC = cmc
				best = p
			}
		}
	}
	return best
}

// pickAnyFriendlyCreature returns any creature seat controls
// (highest-power for stability). Used by Valley Rally for the
// "target creature you control" first-strike grant.
func pickAnyFriendlyCreature(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	s := gs.Seats[seat]
	if s == nil {
		return nil
	}
	var best *gameengine.Permanent
	bestPower := -1
	for _, p := range s.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if pw := p.Power(); pw > bestPower {
			bestPower = pw
			best = p
		}
	}
	return best
}

// -----------------------------------------------------------------------------
// Blooming Blast — {1}{R} Instant, Gift a Treasure
//
// Blooming Blast deals 2 damage to target creature. If the gift was
// promised, Blooming Blast also deals 3 damage to that creature's
// controller.
// -----------------------------------------------------------------------------

func bloomingBlastResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "blooming_blast"
	if gs == nil || item == nil {
		return
	}
	promised, ok := giftPromised(item)
	if !ok {
		return
	}
	if promised {
		gameengine.ResolveGift(gs, item)
	}
	target := pickBestOppCreature(gs, item.Controller)
	if target == nil {
		emitFail(gs, slug, "Blooming Blast", "no_target", map[string]interface{}{
			"seat":     item.Controller,
			"promised": promised,
		})
		return
	}
	target.MarkedDamage += 2
	gs.InvalidateCharacteristicsCache()
	if promised {
		gameengine.DealDamage(gs, target.Controller, 3, "Blooming Blast")
	}
	emit(gs, slug, "Blooming Blast", map[string]interface{}{
		"seat":     item.Controller,
		"target":   target.Card.DisplayName(),
		"promised": promised,
	})
}

// -----------------------------------------------------------------------------
// Crumb and Get It — {W} Instant, Gift a Food
//
// Target creature you control gets +2/+2 until end of turn. If the
// gift was promised, that creature also gains indestructible until
// end of turn.
// -----------------------------------------------------------------------------

func crumbAndGetItResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "crumb_and_get_it"
	if gs == nil || item == nil {
		return
	}
	promised, ok := giftPromised(item)
	if !ok {
		return
	}
	if promised {
		gameengine.ResolveGift(gs, item)
	}
	target := pickBestFriendlyCreature(gs, item.Controller)
	if target == nil {
		emitFail(gs, slug, "Crumb and Get It", "no_friendly_creature", map[string]interface{}{
			"seat":     item.Controller,
			"promised": promised,
		})
		return
	}
	ts := gs.NextTimestamp()
	target.Modifications = append(target.Modifications, gameengine.Modification{
		Power:     2,
		Toughness: 2,
		Duration:  "until_end_of_turn",
		Timestamp: ts,
	})
	if promised {
		if target.Flags == nil {
			target.Flags = map[string]int{}
		}
		target.Flags["kw:indestructible"] = 1
		target.GrantedAbilities = append(target.GrantedAbilities, "indestructible")
	}
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, "Crumb and Get It", map[string]interface{}{
		"seat":     item.Controller,
		"target":   target.Card.DisplayName(),
		"promised": promised,
	})
}

// -----------------------------------------------------------------------------
// Wear Down — {1}{G} Sorcery, Gift a card
//
// Destroy target artifact or enchantment. If the gift was promised,
// instead destroy two target artifacts and/or enchantments.
// -----------------------------------------------------------------------------

func wearDownResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "wear_down"
	if gs == nil || item == nil {
		return
	}
	promised, ok := giftPromised(item)
	if !ok {
		return
	}
	if promised {
		gameengine.ResolveGift(gs, item)
	}
	want := 1
	if promised {
		want = 2
	}
	destroyed := []string{}
	for i := 0; i < want; i++ {
		target := pickBestOppArtOrEnch(gs, item.Controller)
		if target == nil {
			break
		}
		gameengine.DestroyPermanent(gs, target, nil)
		destroyed = append(destroyed, target.Card.DisplayName())
	}
	if len(destroyed) == 0 {
		emitFail(gs, slug, "Wear Down", "no_target_artifact_or_enchantment", map[string]interface{}{
			"seat":     item.Controller,
			"promised": promised,
		})
		return
	}
	emit(gs, slug, "Wear Down", map[string]interface{}{
		"seat":      item.Controller,
		"destroyed": destroyed,
		"promised":  promised,
	})
}

// -----------------------------------------------------------------------------
// Nocturnal Hunger — {2}{B} Instant, Gift a Food
//
// Destroy target creature. If the gift WASN'T promised, you lose 2
// life. (Inverted rider — penalty when you don't deliver the Food.)
// -----------------------------------------------------------------------------

func nocturnalHungerResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "nocturnal_hunger"
	if gs == nil || item == nil {
		return
	}
	promised, ok := giftPromised(item)
	if !ok {
		return
	}
	if promised {
		gameengine.ResolveGift(gs, item)
	}
	target := pickBestOppCreature(gs, item.Controller)
	if target == nil {
		emitFail(gs, slug, "Nocturnal Hunger", "no_target_creature", map[string]interface{}{
			"seat":     item.Controller,
			"promised": promised,
		})
		return
	}
	gameengine.DestroyPermanent(gs, target, nil)
	if !promised {
		gameengine.LoseLife(gs, item.Controller, 2, "Nocturnal Hunger")
	}
	emit(gs, slug, "Nocturnal Hunger", map[string]interface{}{
		"seat":      item.Controller,
		"destroyed": target.Card.DisplayName(),
		"promised":  promised,
	})
}

// -----------------------------------------------------------------------------
// Wildfire Howl — {1}{R}{R} Sorcery, Gift a card
//
// Wildfire Howl deals 2 damage to each creature. If the gift was
// promised, instead Wildfire Howl deals 1 damage to any target AND
// 2 damage to each creature.
// -----------------------------------------------------------------------------

func wildfireHowlResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "wildfire_howl"
	if gs == nil || item == nil {
		return
	}
	promised, ok := giftPromised(item)
	if !ok {
		return
	}
	if promised {
		gameengine.ResolveGift(gs, item)
	}
	// 2 damage to every creature on the battlefield.
	hits := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			p.MarkedDamage += 2
			hits++
		}
	}
	gs.InvalidateCharacteristicsCache()
	// If promised, additionally deal 1 damage to "any target." Approx:
	// pick the biggest opp creature (mirrors the rest of the corpus).
	if promised {
		any := pickBestOppCreature(gs, item.Controller)
		if any != nil {
			any.MarkedDamage += 1
			gs.InvalidateCharacteristicsCache()
		}
	}
	emit(gs, slug, "Wildfire Howl", map[string]interface{}{
		"seat":     item.Controller,
		"hits":     hits,
		"promised": promised,
	})
}

// -----------------------------------------------------------------------------
// Valley Rally — {2}{R} Instant, Gift a Food
//
// Creatures you control get +2/+0 until end of turn. If the gift was
// promised, target creature you control gains first strike until end
// of turn.
// -----------------------------------------------------------------------------

func valleyRallyResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "valley_rally"
	if gs == nil || item == nil {
		return
	}
	promised, ok := giftPromised(item)
	if !ok {
		return
	}
	if promised {
		gameengine.ResolveGift(gs, item)
	}
	s := gs.Seats[item.Controller]
	if s == nil {
		return
	}
	ts := gs.NextTimestamp()
	pumped := 0
	for _, p := range s.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		p.Modifications = append(p.Modifications, gameengine.Modification{
			Power:     2,
			Toughness: 0,
			Duration:  "until_end_of_turn",
			Timestamp: ts,
		})
		pumped++
	}
	gs.InvalidateCharacteristicsCache()
	if promised {
		// Grant first strike to the strongest friendly creature.
		target := pickAnyFriendlyCreature(gs, item.Controller)
		if target != nil {
			if target.Flags == nil {
				target.Flags = map[string]int{}
			}
			target.Flags["kw:first_strike"] = 1
			target.GrantedAbilities = append(target.GrantedAbilities, "first strike")
		}
	}
	emit(gs, slug, "Valley Rally", map[string]interface{}{
		"seat":     item.Controller,
		"pumped":   pumped,
		"promised": promised,
	})
}

// findGiftRecipient pulls the recipient seat index from CostMeta when
// the gift was promised. Returns -1 when absent / invalid.
func findGiftRecipient(item *gameengine.StackItem) int {
	if item == nil || item.CostMeta == nil {
		return -1
	}
	r, _ := item.CostMeta["gift_recipient"].(int)
	return r
}

// findGiftRecipient is preserved for future per_card handlers that
// need it (e.g. Octomancer's 8/8 Octopus token to the recipient is
// not in the canonical-eight ResolveGift switch).
var _ = findGiftRecipient
