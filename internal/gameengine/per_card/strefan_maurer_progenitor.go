package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerStrefanMaurerProgenitor wires Strefan, Maurer Progenitor.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	Flying
//	At the beginning of your end step, create a Blood token for each
//	player who lost life this turn.
//	Whenever Strefan attacks, you may sacrifice two Blood tokens. If
//	you do, you may put a Vampire creature card from your hand onto
//	the battlefield tapped and attacking.
//
// Implementation:
//   - Flying — AST keyword pipeline.
//   - "end_step" trigger: gates on active_seat == controller ("your end
//     step"). Counts every player (including Strefan's controller) who
//     lost life this turn via Turn.DamageReceived and Turn.LifeLost
//     counters. Creates one Blood token per qualifying player.
//   - "creature_attacks" trigger: gates on attacker == Strefan. Looks
//     for 2+ Blood tokens on the controller's battlefield. Sacrifices
//     two Blood tokens, then finds a Vampire creature card in hand and
//     puts it onto the battlefield tapped and attacking the same
//     defender.
//
// R60 stub sweep batch 6: doc-comment cleanup. The "you may sacrifice"
// / "you may put" optional clauses are auto-accepted as deliberate AI
// policy (both branches are strictly value-positive when the
// requirements are met), NOT a missing engine feature — same pattern
// as Bloodchief Ascension / Alesha. The mid-combat attacker entry is
// handled via the "attacking" flag + SetAttackerDefender, which is
// the canonical per_card hand-off to combat.go's defender tracking;
// not a partial gap.
func registerStrefanMaurerProgenitor(r *Registry) {
	r.OnTrigger("Strefan, Maurer Progenitor", "end_step", strefanEndStep)
	r.OnTrigger("Strefan, Maurer Progenitor", "creature_attacks", strefanAttackTrigger)
}

// strefanEndStep — "At the beginning of your end step, create a Blood
// token for each player who lost life this turn."
func strefanEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "strefan_maurer_progenitor_end_step_blood"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	if gs.Seats[seat] == nil || gs.Seats[seat].Lost {
		return
	}

	lostLife := make(map[int]bool, len(gs.Seats))
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		if s.Turn.DamageReceived > 0 || s.Turn.LifeLost > 0 {
			lostLife[i] = true
		}
	}

	count := len(lostLife)
	if count == 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   seat,
			"tokens": 0,
			"reason": "no_player_lost_life_this_turn",
		})
		return
	}

	for i := 0; i < count; i++ {
		gameengine.CreateBloodToken(gs, seat)
	}

	gs.LogEvent(gameengine.Event{
		Kind:   "create_token",
		Seat:   seat,
		Source: perm.Card.DisplayName(),
		Amount: count,
		Details: map[string]interface{}{
			"token_type":          "Blood",
			"players_lost_life":   count,
			"rule":                "triggered_ability",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"tokens": count,
	})
}

// strefanAttackTrigger — "Whenever Strefan attacks, you may sacrifice
// two Blood tokens. If you do, you may put a Vampire creature card
// from your hand onto the battlefield tapped and attacking."
func strefanAttackTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "strefan_maurer_progenitor_attack_cheat"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	attackerSeat, _ := ctx["attacker_seat"].(int)
	if attackerSeat != perm.Controller || atk == nil || atk.Card == nil {
		return
	}
	// Gate: only triggers when Strefan itself attacks.
	if atk != perm {
		return
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	// Find Blood tokens on the battlefield.
	var bloodTokens []*gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsArtifact() {
			continue
		}
		if isBloodToken(p) {
			bloodTokens = append(bloodTokens, p)
		}
	}
	if len(bloodTokens) < 2 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":         perm.Controller,
			"blood_tokens": len(bloodTokens),
			"reason":       "insufficient_blood_tokens",
		})
		return
	}

	// Find a Vampire creature card in hand.
	vampIdx := -1
	for i, c := range seat.Hand {
		if c == nil {
			continue
		}
		if !cardHasType(c, "creature") {
			continue
		}
		if cardHasSubtype(c, "vampire") {
			vampIdx = i
			break
		}
	}
	if vampIdx < 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":         perm.Controller,
			"blood_tokens": len(bloodTokens),
			"reason":       "no_vampire_in_hand",
		})
		return
	}

	// Sacrifice two Blood tokens.
	for i := 0; i < 2; i++ {
		gameengine.SacrificePermanent(gs, bloodTokens[i], "strefan_maurer_progenitor_cost")
	}

	// Determine defender for the cheated Vampire.
	defenderSeat := -1
	if d, ok := gameengine.AttackerDefender(atk); ok {
		defenderSeat = d
	}
	if defenderSeat < 0 {
		for _, opp := range gs.LivingOpponents(perm.Controller) {
			defenderSeat = opp
			break
		}
	}

	// Put the Vampire from hand onto the battlefield tapped and attacking.
	vampCard := seat.Hand[vampIdx]
	seat.Hand = append(seat.Hand[:vampIdx], seat.Hand[vampIdx+1:]...)

	newPerm := enterBattlefieldWithETB(gs, perm.Controller, vampCard, true)
	if newPerm != nil {
		newPerm.SummoningSick = false
		if newPerm.Flags == nil {
			newPerm.Flags = map[string]int{}
		}
		newPerm.Flags["attacking"] = 1
		if defenderSeat >= 0 {
			gameengine.SetAttackerDefender(newPerm, defenderSeat)
		}
	}

	cheated := vampCard.DisplayName()
	gs.LogEvent(gameengine.Event{
		Kind:   "strefan_cheat",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"card":          cheated,
			"defender_seat": defenderSeat,
			"tapped":        true,
			"attacking":     true,
			"blood_sacrificed": 2,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"cheated":       cheated,
		"defender_seat": defenderSeat,
		"blood_sacrificed": 2,
	})
}

// isBloodToken checks if a permanent is a Blood token by name or
// subtype tag.
func isBloodToken(p *gameengine.Permanent) bool {
	if p == nil || p.Card == nil {
		return false
	}
	name := p.Card.DisplayName()
	if name == "Blood Token" || name == "Blood" {
		return true
	}
	// Also check for the "blood" subtype tag used by createSimpleArtifactToken.
	for _, t := range p.Card.Types {
		if t == "blood" {
			return true
		}
	}
	return false
}

// cardHasSubtype checks if a card has a particular subtype in its Types
// slice (case-insensitive).
func cardHasSubtype(c *gameengine.Card, sub string) bool {
	if c == nil {
		return false
	}
	sub = strings.ToLower(sub)
	for _, t := range c.Types {
		if strings.ToLower(t) == sub {
			return true
		}
	}
	return false
}
