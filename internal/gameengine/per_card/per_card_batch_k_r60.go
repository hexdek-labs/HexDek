package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// per_card_batch_k_r60.go — 5 legendary-creature handler ports for cards
// the corpus survey flagged as commander-tier with no per_card file.
// Same shape as the prior r60 consumer batches: one handler per card,
// one regression test per handler, AddResetHook so the registrations
// survive test-side Reset() rebuilds.
//
//   - Kambal, Consul of Allocation   — opp casts noncreature → drain 2
//   - Reyav, Master Smith            — equipped/enchanted attacker
//                                       gains double strike EOT
//   - April, Reporter of the Weird   — combat dmg to player → draw N,
//                                       discard 1
//   - Donatello, Rad Scientist       — ETB: tap up to 3 opp creatures
//                                       + stun counter each
//   - Veronica, Dissident Scribe     — attacks → discard a card to
//                                       draw one (auto-accept when
//                                       hand has cards). Junk-token
//                                       half kept as breadcrumb.

func init() {
	registerPerCardBatchKR60(Global())
	AddResetHook(registerPerCardBatchKR60)
}

func registerPerCardBatchKR60(r *Registry) {
	r.OnTrigger("Kambal, Consul of Allocation", "spell_cast_by_opponent", kambalConsulOnOppCast)
	r.OnTrigger("Reyav, Master Smith", "creature_attacks", reyavMasterSmithOnAttack)
	r.OnTrigger("April, Reporter of the Weird", "combat_damage_to_player", aprilReporterOnDamage)
	r.OnETB("Donatello, Rad Scientist", donatelloRadScientistETB)
	r.OnTrigger("Veronica, Dissident Scribe", "creature_attacks", veronicaDissidentScribeAttack)
}

// ---------------------------------------------------------------------------
// Kambal, Consul of Allocation
// ---------------------------------------------------------------------------
//
// Oracle: "Whenever an opponent casts a noncreature spell, that player
// loses 2 life and you gain 2 life."
//
// Gate on caster != controller AND the cast card not having "creature"
// in its types. Rhystic Study's spell_cast_by_opponent handler is the
// shape model — same ctx contract (caster_seat + card).
func kambalConsulOnOppCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "kambal_consul_noncreature_drain"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	caster, _ := ctx["caster_seat"].(int)
	if caster == perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	for _, t := range card.Types {
		if t == "creature" {
			return
		}
	}
	gameengine.LoseLife(gs, caster, 2, perm.Card.DisplayName())
	gameengine.GainLife(gs, perm.Controller, 2, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"opp_seat":    caster,
		"spell":       card.DisplayName(),
		"life_swing":  4,
	})
}

// ---------------------------------------------------------------------------
// Reyav, Master Smith
// ---------------------------------------------------------------------------
//
// Oracle: "Whenever a creature you control that's enchanted or equipped
// attacks, that creature gains double strike until end of turn."
//
// Gate on attacker controller == Reyav's controller AND attacker has
// at least one Aura or Equipment attached to it. Stamp the EOT
// double-strike grant via plus_kw_double_strike_until_eot, matching
// the plus_power_until_eot pattern used by Strider/landfall.
func reyavMasterSmithOnAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "reyav_master_smith_attached_double_strike"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk == nil || atk.Card == nil || atk.Controller != perm.Controller {
		return
	}
	if !atk.IsCreature() {
		return
	}
	// Scan the controller's battlefield for an attached Aura or
	// Equipment with AttachedTo == attacker. (We don't trust a single
	// AttachedTo field on the attacker because the attachment lives on
	// the Aura/Equipment, not the attacker.)
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	hasAttachment := false
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || p.AttachedTo != atk {
			continue
		}
		if cardHasType(p.Card, "aura") || cardHasType(p.Card, "equipment") {
			hasAttachment = true
			break
		}
	}
	if !hasAttachment {
		return
	}
	if atk.Flags == nil {
		atk.Flags = map[string]int{}
	}
	atk.Flags["kw:double_strike"] = 1
	atk.Flags["kw_double_strike_until_eot"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"attacker":     atk.Card.DisplayName(),
	})
}

// ---------------------------------------------------------------------------
// April, Reporter of the Weird
// ---------------------------------------------------------------------------
//
// Oracle: "Whenever April deals combat damage to a player, draw that
// many cards, then discard a card."
//
// Gate on damager_perm == perm (the trigger says "April deals", not
// "a creature you control deals"). Draw N, then discard 1 (auto-pick:
// the engine's discard helper applies AI policy; we just call it).
func aprilReporterOnDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "april_reporter_draw_discard"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	dmgPerm, _ := ctx["damager_perm"].(*gameengine.Permanent)
	if dmgPerm != nil && dmgPerm != perm {
		return
	}
	// Fallback when damager_perm wasn't threaded: the engine populates
	// source_seat + source_card on combat_damage_player. Gate on those so
	// "April deals" matches name + controller (not "any of my creatures
	// deals"). Legacy damager_seat is a secondary fallback.
	if dmgPerm == nil {
		srcSeat, ok := ctx["source_seat"].(int)
		if !ok {
			srcSeat, ok = ctx["damager_seat"].(int)
		}
		if !ok || srcSeat != perm.Controller {
			return
		}
		if srcName, hasName := ctx["source_card"].(string); hasName && srcName != perm.Card.DisplayName() {
			return
		}
	}
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	drawn := 0
	for i := 0; i < amount; i++ {
		if drawOne(gs, perm.Controller, perm.Card.DisplayName()) == nil {
			break
		}
		drawn++
	}
	if drawn > 0 && len(seat.Hand) > 0 {
		// Discard the first card in hand (engine's broader policy is
		// random-ish; per_card layer doesn't pick a "best" discard).
		// Route through DiscardCard for §702.34a Madness / §702.187
		// Mayhem / Necropotence reroute / card_discarded trigger /
		// Turn.Discarded stat. Pre-r60-normalize this path direct-
		// spliced seat.Hand and silently bypassed every one of those.
		card := seat.Hand[0]
		gameengine.DiscardCard(gs, card, perm.Controller)
		gs.LogEvent(gameengine.Event{
			Kind:   "discard",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"slug": slug,
				"card": card.DisplayName(),
			},
		})
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"target":   ctx["target_seat"],
		"damage":   amount,
		"drew":     drawn,
	})
}

// ---------------------------------------------------------------------------
// Donatello, Rad Scientist
// ---------------------------------------------------------------------------
//
// Oracle: "Vigilance. When Donatello enters, tap up to three target
// creatures your opponents control. Put a stun counter on each of
// them."
//
// Vigilance is AST keyword pipeline. ETB picks up to three opponent
// creatures (any untapped first; fall back to already-tapped to still
// land the stun counter on each picked), taps each, and places one
// stun counter. The Watcher in the Water is the shape precedent for
// stun counters from a per_card handler.
func donatelloRadScientistETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "donatello_rad_scientist_etb_tap_stun"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	picked := []*gameengine.Permanent{}
	// First pass: untapped opp creatures (more impactful tap).
	for i, s := range gs.Seats {
		if i == perm.Controller || s == nil || s.Lost {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() || p.Tapped {
				continue
			}
			picked = append(picked, p)
			if len(picked) >= 3 {
				break
			}
		}
		if len(picked) >= 3 {
			break
		}
	}
	// Second pass: top up with tapped opp creatures so the stun
	// counter still lands on three targets if available.
	if len(picked) < 3 {
		for i, s := range gs.Seats {
			if i == perm.Controller || s == nil || s.Lost {
				continue
			}
			for _, p := range s.Battlefield {
				if p == nil || p.Card == nil || !p.IsCreature() || !p.Tapped {
					continue
				}
				picked = append(picked, p)
				if len(picked) >= 3 {
					break
				}
			}
			if len(picked) >= 3 {
				break
			}
		}
	}
	for _, p := range picked {
		p.Tapped = true
		p.AddCounter("stun", 1)
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"tapped": len(picked),
	})
}

// ---------------------------------------------------------------------------
// Veronica, Dissident Scribe
// ---------------------------------------------------------------------------
//
// Oracle: "Menace. Whenever Veronica attacks, you may discard a card.
// If you do, draw a card. Whenever you discard one or more nonland
// cards for the first time each turn, create a Junk token."
//
// Wire the attack-trigger discard-to-draw with auto-accept when the
// controller has at least one card in hand (cantrip is even-EV in a
// vacuum but cycles a card for digging; AI policy accepts). The
// Junk-token half hangs off card_discarded which has no per_card
// consumer wired and would need its own batch — breadcrumb the gap.
func veronicaDissidentScribeAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "veronica_dissident_scribe_discard_draw"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	if len(seat.Hand) == 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "empty_hand_no_discard", nil)
		return
	}
	card := seat.Hand[0]
	seat.Hand = seat.Hand[1:]
	seat.Graveyard = append(seat.Graveyard, card)
	gs.LogEvent(gameengine.Event{
		Kind:   "discard",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"slug": slug,
			"card": card.DisplayName(),
		},
	})
	drawOne(gs, perm.Controller, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"discarded": card.DisplayName(),
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"junk_token_on_first_nonland_discard_per_turn_requires_card_discarded_consumer_batch")
}
