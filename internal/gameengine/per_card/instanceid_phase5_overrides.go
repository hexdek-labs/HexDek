package per_card

// InstanceID Phase 5 — per-card-override handlers for the 5 model-breakers
// surfaced by Probe A (PR #745). Per design v2 §5, each entry wires its
// CopyMechanism(s) at ETB so the audit trail + ExileLinkageIntegrity /
// copy-mechanism invariants see them before any trigger fires.
//
// Model-breakers (per docs/instanceid-system-v2-r60.md §5 table):
//
//	Enolc                — partner_conditional_copy
//	Ertai's Meddling     — exile_zone_snapshot (CopyRestriction.SourceZone)
//	Soulflayer           — NOT a copy effect; keyword grant via GrantedAbilities
//	Mirage Mirror        — multi-mechanism (Upkeep + Activated)
//	Sakashima the Impostor — BypassesLegendRule flag + standard ETBOnce copy
//
// Wiring strategy: each override registers an OnETB handler that stamps
// the CopyMechanisms slice + special-case Permanent fields. Existing
// functional handlers (Mirage Mirror's mirageMirrorActivated, Sakashima's
// custom copy path in batch_al_clones_r60.go) remain authoritative for
// the game-rules side; this file only adds the metadata layer the Phase
// 5 invariants need.

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// PerCardOverride keys — match design §5 model-breaker table verbatim.
const (
	OverrideEnolcPartnerConditional = "partner_conditional_copy"
	OverrideErtaiExileSnapshot      = "exile_zone_snapshot"
	OverrideSoulflayerKeywordGrant  = "keyword_grant_not_copy"
	OverrideMirageMirrorMultiArm    = "mirror_multi_arm"
	OverrideSparkDoubleP1P1Rider    = "spark_double_p1p1_rider"
	OverridePhantasmalImageFragile  = "phantasmal_image_fragile_rider"
	OverrideSakashimaLegendBypass   = "sakashima_legend_bypass"
)

func init() {
	registerInstanceIDPhase5Overrides(Global())
	AddResetHook(registerInstanceIDPhase5Overrides)
}

func registerInstanceIDPhase5Overrides(r *Registry) {
	if r == nil {
		return
	}
	r.OnETB("Enolc", enolcRegisterCopyMechanism)
	r.OnETB("Ertai's Meddling", ertaiMeddlingRegisterCopyMechanism)
	r.OnETB("Soulflayer", soulflayerRegisterKeywordGrant)
	r.OnETB("Mirage Mirror", mirageMirrorRegisterCopyMechanism)
	r.OnETB("Sakashima the Impostor", sakashimaImpostorRegisterCopyMechanism)
	r.OnETB("Sakashima of a Thousand Faces", sakashimaThousandFacesRegisterCopyMechanism)
}

// enolcRegisterCopyMechanism — "partner_conditional_copy": Enolc copies a
// target permanent ONLY when its partner condition is satisfied (a
// matching partner is on the battlefield). The override flags this so
// the §616 chain + invariant scans know to consult ctx["partner_present"]
// before firing the copy.
func enolcRegisterCopyMechanism(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	gameengine.AddCopyMechanism(perm, gameengine.CopyMechanism{
		TriggerSource:   gameengine.CopyTriggerETB,
		Duration:        gameengine.CopyDurationPermanent,
		Target:          gameengine.CopyTargetSelf,
		PerCardOverride: OverrideEnolcPartnerConditional,
	})
	emit(gs, "iid_phase5_copy_mechanism", "Enolc", map[string]interface{}{
		"override": OverrideEnolcPartnerConditional,
		"seat":     perm.Controller,
	})
}

// ertaiMeddlingRegisterCopyMechanism — "exile_zone_snapshot": Ertai's
// Meddling snapshots a spell on the stack and exiles it. The copy
// SOURCE is in the exile zone, not the battlefield — drives the
// CopyRestriction.SourceZone field on the mechanism record.
func ertaiMeddlingRegisterCopyMechanism(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	gameengine.AddCopyMechanism(perm, gameengine.CopyMechanism{
		TriggerSource: gameengine.CopyTriggerEventCondition,
		Duration:      gameengine.CopyDurationUntilNextUpkeep,
		Target:        gameengine.CopyTargetSelf,
		Restriction: &gameengine.CopyRestriction{
			ControllerSeat: -1,
			SourceZone:     "exile",
		},
		PerCardOverride: OverrideErtaiExileSnapshot,
	})
	emit(gs, "iid_phase5_copy_mechanism", "Ertai's Meddling", map[string]interface{}{
		"override":    OverrideErtaiExileSnapshot,
		"source_zone": "exile",
		"seat":        perm.Controller,
	})
}

// soulflayerRegisterKeywordGrant — Soulflayer is NOT a copy effect per
// Probe A. Its "gains keywords from exiled creatures" pattern is a
// keyword-grant via GrantedAbilities, not §706. The override exists so
// audit tools can confirm Soulflayer is flagged and NOT counted by any
// copy-mechanism invariant.
func soulflayerRegisterKeywordGrant(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	// Intentionally NO CopyMechanism here. Just emit the marker so the
	// audit knows Soulflayer is keyword-grant family.
	emit(gs, "iid_phase5_keyword_grant", "Soulflayer", map[string]interface{}{
		"override": OverrideSoulflayerKeywordGrant,
		"note":     "not_a_copy_effect",
		"seat":     perm.Controller,
	})
}

// mirageMirrorRegisterCopyMechanism — multi-mechanism: Mirage Mirror has
// TWO independent copy capabilities per Probe A:
//
//  1. Upkeep-triggered permanent copy (older printing variant) — not on
//     standard Mirage Mirror but Probe A grouped the family.
//  2. Activated {2} until-EOT copy (the printed Mirage Mirror text).
//
// Both registered so the audit walker sees the multi-arm shape.
func mirageMirrorRegisterCopyMechanism(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	gameengine.AddCopyMechanism(perm, gameengine.CopyMechanism{
		TriggerSource:   gameengine.CopyTriggerUpkeep,
		Duration:        gameengine.CopyDurationPermanent,
		Target:          gameengine.CopyTargetSelf,
		PerCardOverride: OverrideMirageMirrorMultiArm,
	})
	gameengine.AddCopyMechanism(perm, gameengine.CopyMechanism{
		TriggerSource:   gameengine.CopyTriggerActivated,
		Duration:        gameengine.CopyDurationUntilEOT,
		Target:          gameengine.CopyTargetSelf,
		PerCardOverride: OverrideMirageMirrorMultiArm,
	})
	emit(gs, "iid_phase5_copy_mechanism", "Mirage Mirror", map[string]interface{}{
		"override":   OverrideMirageMirrorMultiArm,
		"arm_count":  2,
		"seat":       perm.Controller,
	})
}

// sakashimaImpostorRegisterCopyMechanism — ETB-as-copy with legend-rule
// bypass (Sakashima's printed text: "You may have ~ enter as a copy of
// any creature on the battlefield, except it's legendary in addition to
// its other types and you can have two legendary permanents with the same
// name").
func sakashimaImpostorRegisterCopyMechanism(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	perm.BypassesLegendRule = true
	gameengine.AddCopyMechanism(perm, gameengine.CopyMechanism{
		TriggerSource:   gameengine.CopyTriggerETB,
		Duration:        gameengine.CopyDurationPermanent,
		Target:          gameengine.CopyTargetSelf,
		PerCardOverride: OverrideSakashimaLegendBypass,
	})
	emit(gs, "iid_phase5_copy_mechanism", "Sakashima the Impostor", map[string]interface{}{
		"override":      OverrideSakashimaLegendBypass,
		"legend_bypass": true,
		"seat":          perm.Controller,
	})
}

// sakashimaThousandFacesRegisterCopyMechanism — companion handler for the
// upgrade printing. Same shape as the Impostor entry; both grant legend-
// rule bypass to permanents the Sakashima-controller controls per the
// Probe A family taxonomy.
func sakashimaThousandFacesRegisterCopyMechanism(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	perm.BypassesLegendRule = true
	gameengine.AddCopyMechanism(perm, gameengine.CopyMechanism{
		TriggerSource:   gameengine.CopyTriggerETB,
		Duration:        gameengine.CopyDurationPermanent,
		Target:          gameengine.CopyTargetSelf,
		PerCardOverride: OverrideSakashimaLegendBypass,
	})
	emit(gs, "iid_phase5_copy_mechanism", "Sakashima of a Thousand Faces", map[string]interface{}{
		"override":      OverrideSakashimaLegendBypass,
		"legend_bypass": true,
		"seat":          perm.Controller,
	})
}
