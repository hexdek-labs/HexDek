package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSyrVondamTheLucent wires Syr Vondam, the Lucent.
//
// Oracle text:
//
//	{2}{W}{B}{B}
//	Legendary Creature — Human Knight
//	Deathtouch, lifelink
//	Whenever Syr Vondam enters or attacks, other creatures you control
//	get +1/+0 and gain deathtouch until end of turn.
//
// Implementation:
//   - Deathtouch / lifelink: AST keyword pipeline.
//   - ETB and creature_attacks (gated to Syr Vondam itself): grant
//     +1/+0 and deathtouch UEOT to other creatures the controller
//     controls, via Permanent.Modifications and GrantedAbilities — the
//     two surfaces the layer system, HasKeyword and the §514.2 cleanup
//     sweep actually read. (This comment previously claimed the
//     Flags["temp_power"] / ["temp_deathtouch"] stamps were what the
//     pipeline consumed. They are not read anywhere; the pump and the
//     deathtouch grant were both inert. The stamps are kept as
//     diagnostics only.)
func registerSyrVondamTheLucent(r *Registry) {
	r.OnETB("Syr Vondam, the Lucent", syrVondamPump)
	r.OnTrigger("Syr Vondam, the Lucent", "creature_attacks", syrVondamAttackPump)
}

func syrVondamPump(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	syrVondamApply(gs, perm, "syr_vondam_etb_pump")
}

func syrVondamAttackPump(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atkPerm, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atkPerm == nil || atkPerm != perm {
		return
	}
	syrVondamApply(gs, perm, "syr_vondam_attack_pump")
}

func syrVondamApply(gs *gameengine.GameState, src *gameengine.Permanent, slug string) {
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	pumped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == src || p.Card == nil {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		pumpUntilEOT(gs, p, 1, 0)
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		grantUntilEOT(p, "deathtouch")
		p.Flags["temp_deathtouch"] = 1
		pumped++
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   src.Controller,
		"pumped": pumped,
	})
}
