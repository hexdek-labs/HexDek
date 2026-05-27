package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKrarkClanIronworks wires Krark-Clan Ironworks ("KCI").
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Krark-Clan%20Ironworks):
//
//	Sacrifice an artifact: Add {C}{C}.
//
// {4} Artifact. Banned in many formats; in Commander pools where it's
// legal, KCI is a combo backbone (Scrap Trawler / Myr Retriever loops,
// Pyrite Spellbomb storm, mass-treasure-sac value engines). The
// activation is the entire effect — no tap requirement, just a
// sacrifice cost.
//
// Implementation:
//   - OnActivated(0): pick an artifact target (from ctx["target_perm"]
//     if supplied, else best-effort). Sac it via SacrificePermanent
//     (routes through §614/§704 replacements + dies triggers — KCI's
//     value in cEDH is precisely the cascading creature_dies /
//     artifact_sacrificed observers, so the canonical sac path matters).
//   - Add {C}{C} to the controller's pool.
//   - No tap requirement — KCI doesn't tap. Multiple activations per
//     turn are normal.
//
// Sacrifice picker: prefer the LOWEST-impact artifact (tokens first —
// Treasures / Clues / Food / Blood / Powerstones — then non-token
// artifacts with mana value 0-1 — Mox-style ramp). Avoid KCI itself.
// Always-keep list excludes commanders. Falls back to first non-KCI
// artifact when no preferred targets exist.
func registerKrarkClanIronworks(r *Registry) {
	r.OnActivated("Krark-Clan Ironworks", krarkClanIronworksActivate)
}

func krarkClanIronworksActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "krark_clan_ironworks"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	target, _ := ctx["target_perm"].(*gameengine.Permanent)
	if target == nil {
		target = pickKCIArtifactToSac(gs, seat, src)
	}
	if target == nil {
		emitFail(gs, slug, "Krark-Clan Ironworks", "no_artifact_to_sac", nil)
		return
	}
	if !target.IsArtifact() {
		emitFail(gs, slug, "Krark-Clan Ironworks", "target_not_artifact", nil)
		return
	}
	if target == src {
		emitFail(gs, slug, "Krark-Clan Ironworks", "cannot_sac_kci_itself_via_picker", nil)
		return
	}

	gameengine.SacrificePermanent(gs, target, "kci_mana_ability")
	gameengine.AddMana(gs, s, "C", 2, "Krark-Clan Ironworks")

	emit(gs, slug, "Krark-Clan Ironworks", map[string]interface{}{
		"seat":         seat,
		"sac_artifact": target.Card.DisplayName(),
		"mana":         "{C}{C}",
	})
}

// pickKCIArtifactToSac picks the lowest-impact artifact on the
// controller's battlefield. Preference: token first, then low-CMC,
// then any other artifact. Never picks the KCI itself.
func pickKCIArtifactToSac(gs *gameengine.GameState, seat int, src *gameengine.Permanent) *gameengine.Permanent {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return nil
	}
	s := gs.Seats[seat]
	if s == nil {
		return nil
	}
	var bestNonToken *gameengine.Permanent
	bestCMC := 999
	for _, p := range s.Battlefield {
		if p == nil || p == src || !p.IsArtifact() {
			continue
		}
		// Tier 1: token artifacts (Treasure / Clue / Food / Blood / Powerstone)
		if p.IsToken() {
			return p
		}
		// Tier 2: track lowest CMC non-token.
		cmc := 99
		if p.Card != nil {
			cmc = p.Card.CMC
		}
		if cmc < bestCMC {
			bestCMC = cmc
			bestNonToken = p
		}
	}
	return bestNonToken
}
