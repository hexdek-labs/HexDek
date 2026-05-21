package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKujaGenomeSorcererTranceKujaFateDefied wires Kuja, Genome
// Sorcerer // Trance Kuja, Fate Defied.
//
// Oracle text (front face):
//
//	At the beginning of your end step, create a tapped 0/1 black
//	Wizard creature token with "Whenever you cast a noncreature spell,
//	this token deals 1 damage to each opponent." Then if you control
//	four or more Wizards, transform Kuja.
//	Flare Star — If a Wizard you control would deal damage to a
//	permanent or player, it deals double that damage instead.
//
// Implementation:
//   - End step trigger: spawn a 0/1 black Wizard token (tapped). The
//     "this token deals 1 to each opponent on noncreature cast" sub-trigger
//     is engine-deep (per-token grants); set a per-token marker flag and
//     emit a partial. Transform on 4+ Wizards: count wizards on
//     controller's battlefield, if >=4 stamp perm.Flags["kuja_transformed"].
//   - "Wizard damage doubled" replacement: engine-deep DealDamage hook;
//     set per-seat flag.
func registerKujaGenomeSorcererTranceKujaFateDefied(r *Registry) {
	r.OnETB("Kuja, Genome Sorcerer // Trance Kuja, Fate Defied", kujaETBSetDamageDoubler)
	r.OnTrigger("Kuja, Genome Sorcerer // Trance Kuja, Fate Defied", "end_step", kujaEndStepSpawnAndCheckTransform)
	// Also accept short-name binding so engines that dispatch by single-face name find us.
	r.OnETB("Kuja, Genome Sorcerer", kujaETBSetDamageDoubler)
	r.OnTrigger("Kuja, Genome Sorcerer", "end_step", kujaEndStepSpawnAndCheckTransform)
	// R51 batch I: LTB clears the wizard-damage-doubler seat flag
	// (only when no OTHER Kuja remains on the controller's
	// battlefield) so a removed Kuja doesn't leave the Flare-Star
	// replacement contract active for downstream consumers.
	r.OnTrigger("Kuja, Genome Sorcerer // Trance Kuja, Fate Defied", "permanent_ltb", kujaLTBClearFlag)
	r.OnTrigger("Kuja, Genome Sorcerer", "permanent_ltb", kujaLTBClearFlag)
}

func kujaLTBClearFlag(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil {
			continue
		}
		dn := normalizeName(p.Card.DisplayName())
		if dn == normalizeName("Kuja, Genome Sorcerer") ||
			dn == normalizeName("Kuja, Genome Sorcerer // Trance Kuja, Fate Defied") ||
			dn == normalizeName("Trance Kuja, Fate Defied") {
			return
		}
	}
	if seat.Flags != nil {
		delete(seat.Flags, "kuja_wizard_damage_doubler_active")
	}
}

func kujaETBSetDamageDoubler(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "kuja_etb_wizard_damage_doubler"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["kuja_wizard_damage_doubler_active"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"Wizard damage doubling needs DealDamage replacement hook; flag set for downstream consumers")
}

func kujaEndStepSpawnAndCheckTransform(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "kuja_end_step_wizard_token_and_transform"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	active, _ := ctx["active_seat"].(int)
	if active != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	wiz := &gameengine.Card{
		Name:          "Wizard Token",
		Owner:         perm.Controller,
		BasePower:     0,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "wizard"},
		Colors:        []string{"B"},
		TypeLine:      "Token Creature — Wizard",
	}
	enterBattlefieldWithETB(gs, perm.Controller, wiz, true)
	// Mark the token so a future per-token trigger handler can grant it
	// the "noncreature spell → 1 damage to each opp" trigger.
	for _, p := range seat.Battlefield {
		if p != nil && p.Card == wiz {
			if p.Flags == nil {
				p.Flags = map[string]int{}
			}
			p.Flags["kuja_wizard_token_noncreature_pinger"] = 1
			break
		}
	}
	wizards := countCreatureType(gs, perm.Controller, "wizard")
	if wizards >= 4 {
		if perm.Flags == nil {
			perm.Flags = map[string]int{}
		}
		perm.Flags["kuja_transformed"] = 1
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"DFC transform to Trance Kuja needs DFC transform hook; flag set for downstream consumers")
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"wizards_total":    wizards,
		"transformed":      perm.Flags["kuja_transformed"],
	})
}
