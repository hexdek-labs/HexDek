package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerNoctisPrinceOfLucis wires Noctis, Prince of Lucis.
//
// Oracle text (Scryfall, verified):
//
//	Lifelink
//	You may cast artifact spells from your graveyard by paying 3 life
//	in addition to paying their other costs. If you cast a spell this
//	way, that artifact enters with a finality counter on it.
//
// Implementation (R42b stub port):
//   - Lifelink: AST keyword pipeline.
//   - Static graveyard-cast permission: OnETB scans the controller's
//     graveyard and registers a ZoneCastPermission{Zone: graveyard,
//     +3 life additional cost, while_source_on_bf} for every artifact
//     card found. Refresh hooks on permanent_ltb and creature_dies
//     re-scan when an artifact enters the controller's graveyard from
//     the battlefield. The "while_source_on_bf" duration plus
//     SourceTimestamp lets the engine's zone_cast cleanup expire all
//     grants atomically when Noctis leaves.
//   - Refresh gaps: artifact cards entering the graveyard via
//     non-battlefield-leave paths (discard, mill, counter-to-gy,
//     return-from-exile-to-gy) are not currently hooked. The cast
//     pipeline's zone_cast.go scanner does its lookup at cast time,
//     so missing a refresh tick only delays — never blocks —
//     eligibility on the next refresh event.
//   - Finality counter on the resulting permanent: not stamped at
//     the per_card layer. The cast pipeline would need to thread a
//     "cast via noctis" CostMeta marker through to the post-resolve
//     ETB so we could call perm.AddCounter("finality", 1).
//     emitPartial documents the gap.
func registerNoctisPrinceOfLucis(r *Registry) {
	r.OnETB("Noctis, Prince of Lucis", noctisPrinceOfLucisETB)
	r.OnTrigger("Noctis, Prince of Lucis", "permanent_ltb", noctisRefreshGrants)
	r.OnTrigger("Noctis, Prince of Lucis", "creature_dies", noctisRefreshGrants)
	// R52 batch K: finality-counter rider. Every nonland permanent
	// that ETBs under Noctis's controller and whose Card carries the
	// "cast_via_noctis" type tag (stamped by the cast pipeline when
	// the graveyard-cast permission was used) gets a finality counter.
	// The tag is consumed (removed) at ETB so a flicker / re-cast
	// from a different source doesn't re-stamp the counter.
	r.OnTrigger("Noctis, Prince of Lucis", "permanent_etb", noctisStampFinality)
}

func noctisStampFinality(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "noctis_finality_counter_on_etb"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || entering.Card == nil {
		return
	}
	if entering.Controller != perm.Controller {
		return
	}
	tagged := false
	filtered := entering.Card.Types[:0]
	for _, t := range entering.Card.Types {
		if t == "cast_via_noctis" {
			tagged = true
			continue
		}
		filtered = append(filtered, t)
	}
	if !tagged {
		return
	}
	entering.Card.Types = filtered
	entering.AddCounter("finality", 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   entering.Controller,
		"target": entering.Card.DisplayName(),
	})
}

func noctisPrinceOfLucisETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "noctis_prince_of_lucis_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	count := noctisRegisterGraveyardGrants(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"artifacts_in_gy":  count,
		"grants_installed": count,
	})
	// Finality-counter rider wired by noctisStampFinality (R52 batch K).
	// The cast pipeline must stamp "cast_via_noctis" on the spell's
	// Card.Types when the graveyard cast-permission is used; tests set
	// the tag directly to simulate the cast path.
}

func noctisRefreshGrants(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "noctis_refresh_grants"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	count := noctisRegisterGraveyardGrants(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"artifacts_in_gy":  count,
		"grants_installed": count,
	})
}

// noctisRegisterGraveyardGrants installs a pay-3-life graveyard-cast
// permission on every artifact card in Noctis's controller's graveyard.
// Duplicate registrations replace the prior grant (gs.ZoneCastGrants is
// keyed by *Card), so re-scanning is idempotent.
func noctisRegisterGraveyardGrants(gs *gameengine.GameState, perm *gameengine.Permanent) int {
	if gs == nil || perm == nil || perm.Card == nil {
		return 0
	}
	if perm.Controller < 0 || perm.Controller >= len(gs.Seats) {
		return 0
	}
	s := gs.Seats[perm.Controller]
	if s == nil {
		return 0
	}
	count := 0
	for _, c := range s.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "artifact") {
			continue
		}
		gameengine.RegisterZoneCastGrant(gs, c, &gameengine.ZoneCastPermission{
			Zone:     gameengine.ZoneGraveyard,
			Keyword:  "noctis_artifact_cast",
			ManaCost: -1, // pay normal mana cost
			AdditionalCosts: []*gameengine.AdditionalCost{
				{
					Kind:       gameengine.AddCostKindPayLife,
					Label:      "pay 3 life (Noctis)",
					LifeAmount: 3,
				},
			},
			RequireController: perm.Controller,
			SourceName:        "Noctis, Prince of Lucis",
			Duration:          "while_source_on_bf",
			SourceTimestamp:   perm.Timestamp,
			GrantTurn:         gs.Turn,
		})
		count++
	}
	return count
}
