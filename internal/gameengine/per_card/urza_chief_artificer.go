package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerUrzaChiefArtificer wires Urza, Chief Artificer (BRC, 2022).
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	{3}{W}{U}{B}, 4/5 Legendary Creature — Human Artificer
//	Affinity for artifact creatures (This spell costs {1} less to cast
//	  for each artifact creature you control.)
//	Artifact creatures you control have menace.
//	At the beginning of your end step, create a 0/0 colorless Construct
//	  artifact creature token with "This token gets +1/+1 for each
//	  artifact you control."
//
// Implementation:
//   - Affinity for artifact creatures: cost reduction wired in
//     cost_modifiers.go via the per-permanent "Urza, Chief Artificer"
//     branch, which counts artifact creatures the controller controls
//     and applies a CostModReduction of that count when Urza himself is
//     being cast. (See cost_modifiers.go.)
//   - "Artifact creatures you control have menace" — granted via
//     GrantedAbilities on Urza's own ETB (existing artifact creatures)
//     and on every subsequent permanent_etb for artifact creatures
//     controlled by Urza's controller. We do NOT remove the grant if
//     Urza later leaves; emitPartial flags this.
//   - End-step Construct token: at our end step, create one 0/0 colorless
//     Construct artifact creature token. Its "+1/+1 per artifact you
//     control" anthem is baked in at creation time (engine has no per-
//     token continuous anthem hook for tokens) — same shape Urza, Lord
//     High Artificer uses. emitPartial flags the dynamic anthem
//     limitation.
func registerUrzaChiefArtificer(r *Registry) {
	r.OnETB("Urza, Chief Artificer", urzaChiefETB)
	r.OnTrigger("Urza, Chief Artificer", "permanent_etb", urzaChiefMenaceGrant)
	r.OnTrigger("Urza, Chief Artificer", "end_step", urzaChiefEndStep)
}

func urzaChiefETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "urza_chief_menace_grant_existing"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	granted := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsArtifact() || !p.IsCreature() {
			continue
		}
		if !urzaChiefHasGrantedMenace(p) {
			p.GrantedAbilities = append(p.GrantedAbilities, "menace")
			granted++
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    perm.Controller,
		"granted": granted,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"menace_grant_not_revoked_when_urza_leaves_battlefield")
}

func urzaChiefMenaceGrant(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "urza_chief_menace_grant_etb"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	target, _ := ctx["perm"].(*gameengine.Permanent)
	if target == nil || target.Card == nil {
		return
	}
	if target.Controller != perm.Controller {
		return
	}
	if !target.IsArtifact() || !target.IsCreature() {
		return
	}
	if urzaChiefHasGrantedMenace(target) {
		return
	}
	target.GrantedAbilities = append(target.GrantedAbilities, "menace")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": target.Card.DisplayName(),
	})
}

func urzaChiefEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "urza_chief_construct_token"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, ok := ctx["active_seat"].(int)
	if !ok || activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	// Bake the "+1/+1 per artifact you control" anthem in at creation.
	// Counts include the token itself once it ETBs as an artifact, which
	// would normally be a +1 for itself; the printed wording on Urza's
	// token is "for each artifact you control" (the token included), so
	// pre-include +1 for self after ETB.
	artifacts := 0
	for _, p := range seat.Battlefield {
		if p != nil && p.IsArtifact() {
			artifacts++
		}
	}
	// Token is itself an artifact creature, so it adds 1 to the count
	// upon entering. Bake in (artifacts + 1) for "+1/+1 per artifact".
	boost := artifacts + 1
	token := &gameengine.Card{
		Name:          "Construct Token",
		Owner:         perm.Controller,
		Types:         []string{"token", "artifact", "creature", "construct"},
		BasePower:     boost,
		BaseToughness: boost,
	}
	enterBattlefieldWithETB(gs, perm.Controller, token, false)
	gs.LogEvent(gameengine.Event{
		Kind:   "create_token",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"token":       "Construct Token",
			"reason":      "urza_chief_end_step",
			"power":       boost,
			"tough":       boost,
			"anthem_base": "+1/+1 per artifact you control",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"artifacts_pre":  artifacts,
		"token_pt":       boost,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"construct_token_anthem_baked_at_creation_not_dynamic_per_artifact")
}

func urzaChiefHasGrantedMenace(p *gameengine.Permanent) bool {
	if p == nil {
		return false
	}
	for _, a := range p.GrantedAbilities {
		if a == "menace" {
			return true
		}
	}
	return false
}
