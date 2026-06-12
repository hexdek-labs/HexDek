package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// per_card_batch_o_r60.go — 5 more commander-tier handler ports. Same
// shape as batch K and M: one handler per card, one regression per
// handler, AddResetHook for test-side Reset() rebuilds.
//
//   - Saradoc, Master of Buckland   — ETB-or-another power<=2 creature
//                                      → mint 1/1 W Halfling token
//   - Sokka, Lateral Strategist     — Sokka + 1+ other attacker → draw 1
//   - Zhang He, Wei General         — Zhang He attacks → other creatures
//                                      you control +1/+0 EOT
//   - Loran of the Third Path       — ETB: destroy up to one target
//                                      artifact or enchantment
//   - Captain America, Team Leader  — another Hero ETBs → it gains
//                                      vigilance + haste EOT, +1/+1
//                                      counter on it and on Captain
//                                      America

func init() {
	registerPerCardBatchOR60(Global())
	AddResetHook(registerPerCardBatchOR60)
}

func registerPerCardBatchOR60(r *Registry) {
	r.OnETB("Saradoc, Master of Buckland", saradocMasterOfBucklandETB)
	r.OnTrigger("Saradoc, Master of Buckland", "permanent_etb", saradocOnAnotherETB)
	r.OnTrigger("Sokka, Lateral Strategist", "creature_attacks", sokkaLateralStrategistOnAttack)
	r.OnTrigger("Zhang He, Wei General", "creature_attacks", zhangHeWeiGeneralOnAttack)
	r.OnETB("Loran of the Third Path", loranOfTheThirdPathETB)
	r.OwnsETBTrigger("Loran of the Third Path")
	r.OnTrigger("Captain America, Team Leader", "permanent_etb", captainAmericaOnAnotherHeroETB)
}

// ---------------------------------------------------------------------------
// Saradoc, Master of Buckland
// ---------------------------------------------------------------------------
//
// Oracle: "Whenever Saradoc or another nontoken creature you control
// with power 2 or less enters, create a 1/1 white Halfling creature
// token. Tap two other untapped Halflings you control: ..."
//
// ETB hook fires for Saradoc's own entry; permanent_etb trigger fires
// for every subsequent permanent entering. The "tap two Halflings"
// activated cost is a separate parser-side surface — emitPartial.
func saradocMasterOfBucklandETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	saradocMintHalfling(gs, perm, perm)
}

func saradocOnAnotherETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || entering == perm || entering.Card == nil {
		return
	}
	if entering.Controller != perm.Controller {
		return
	}
	if !entering.IsCreature() {
		return
	}
	// Nontoken filter — engine marks tokens via Permanent.IsToken()
	// (state.go:1331, reads "token" in Card.Types). Saradoc's printed
	// trigger explicitly says "nontoken", so a token Halfling entering
	// must not chain another mint.
	if entering.IsToken() {
		return
	}
	if entering.Card.BasePower > 2 {
		return
	}
	saradocMintHalfling(gs, perm, entering)
}

func saradocMintHalfling(gs *gameengine.GameState, perm *gameengine.Permanent, trigger *gameengine.Permanent) {
	const slug = "saradoc_master_of_buckland_halfling_mint"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	tok := gameengine.CreateCreatureToken(gs, perm.Controller, "Halfling", []string{"creature", "halfling"}, 1, 1)
	if tok == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "token_creation_failed", nil)
		return
	}
	if tok.Card != nil {
		tok.Card.Colors = []string{"W"}
	}
	triggerName := ""
	if trigger != nil && trigger.Card != nil {
		triggerName = trigger.Card.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    perm.Controller,
		"trigger": triggerName,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"tap_two_halflings_activated_cost_not_modeled_at_per_card_layer")
}

// ---------------------------------------------------------------------------
// Sokka, Lateral Strategist
// ---------------------------------------------------------------------------
//
// Oracle: "Vigilance. Whenever Sokka and at least one other creature
// attack, draw a card."
//
// fireTrigger dispatches creature_attacks once per attacker. We gate
// on attacker_perm == Sokka (so the trigger fires off Sokka's own
// attack event), then scan controller's battlefield for any other
// IsAttacking creature. Vigilance is AST keyword pipeline.
func sokkaLateralStrategistOnAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sokka_lateral_strategist_group_attack_draw"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	hasOther := false
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || !p.IsCreature() {
			continue
		}
		if p.IsAttacking() {
			hasOther = true
			break
		}
	}
	if !hasOther {
		return
	}
	drawOne(gs, perm.Controller, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

// ---------------------------------------------------------------------------
// Zhang He, Wei General
// ---------------------------------------------------------------------------
//
// Oracle: "Horsemanship. Whenever Zhang He attacks, each other
// creature you control gets +1/+0 until end of turn."
//
// Horsemanship is AST keyword pipeline. Anthem fires only when Zhang
// He herself is the attacker (per oracle "Whenever Zhang He attacks");
// stamps plus_power_until_eot on every OTHER creature on her side.
func zhangHeWeiGeneralOnAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "zhang_he_wei_general_attack_anthem"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	bumped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || !p.IsCreature() {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		p.Flags["plus_power_until_eot"]++
		bumped++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"bumped": bumped,
	})
}

// ---------------------------------------------------------------------------
// Loran of the Third Path
// ---------------------------------------------------------------------------
//
// Oracle: "Vigilance. When Loran enters, destroy up to one target
// artifact or enchantment. {T}: You and target opponent each draw a
// card."
//
// ETB picks an opponent's artifact OR enchantment and DestroyPermanent
// it. Targeting prefers higher-impact: enchantments first (typically
// fewer of them and harder to remove), then artifacts. The activated
// draw ability is a separate parser surface — emitPartial.
func loranOfTheThirdPathETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "loran_of_the_third_path_etb_destroy"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	var pick *gameengine.Permanent
	for pass := 0; pass < 2 && pick == nil; pass++ {
		wantType := "enchantment"
		if pass == 1 {
			wantType = "artifact"
		}
		for i, s := range gs.Seats {
			if i == perm.Controller || s == nil || s.Lost {
				continue
			}
			for _, p := range s.Battlefield {
				if p == nil || p.Card == nil {
					continue
				}
				if cardHasType(p.Card, wantType) {
					pick = p
					break
				}
			}
			if pick != nil {
				break
			}
		}
	}
	if pick == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"action": "no_target",
		})
		return
	}
	target := pick.Card.DisplayName()
	gameengine.DestroyPermanent(gs, pick, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": target,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"tap_each_opp_draw_activated_ability_handled_by_ast_activated_path")
}

// ---------------------------------------------------------------------------
// Captain America, Team Leader
// ---------------------------------------------------------------------------
//
// Oracle: "Whenever another Hero you control enters, it gains
// vigilance and haste until end of turn. Put a +1/+1 counter on that
// Hero and a +1/+1 counter on Captain America."
//
// permanent_etb gate: entering perm != Captain America, controller
// matches, IsCreature, and Card has the "hero" subtype. Flags +
// counters mirror the recent Inspired/Class consumer batches.
func captainAmericaOnAnotherHeroETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "captain_america_team_leader_hero_etb"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || entering == perm || entering.Card == nil {
		return
	}
	if entering.Controller != perm.Controller {
		return
	}
	if !entering.IsCreature() {
		return
	}
	if !cardHasSubtype(entering.Card, "hero") {
		return
	}
	if entering.Flags == nil {
		entering.Flags = map[string]int{}
	}
	entering.Flags["kw:vigilance"] = 1
	entering.Flags["kw:haste"] = 1
	entering.Flags["kw_vigilance_until_eot"] = 1
	entering.Flags["kw_haste_until_eot"] = 1
	entering.AddCounter("+1/+1", 1)
	perm.AddCounter("+1/+1", 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    perm.Controller,
		"trigger": entering.Card.DisplayName(),
		"self_p1": perm.Counters["+1/+1"],
	})
}
