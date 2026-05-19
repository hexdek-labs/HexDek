package per_card

import (
	"sort"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerIronManTitanOfInnovation wires Iron Man, Titan of Innovation.
//
// Oracle text (Scryfall, verified):
//
//	Flying, haste
//	Genius Industrialist — Whenever Iron Man attacks, create a Treasure
//	token, then you may sacrifice a noncreature artifact. If you do,
//	search your library for an artifact card with mana value equal to 1
//	plus the sacrificed artifact's mana value, put it onto the
//	battlefield tapped, then shuffle.
//
// Implementation (R41 stub port):
//   - Flying, haste: AST keyword pipeline.
//   - "creature_attacks" trigger gated on attacker_perm == perm.
//     Always creates a Treasure. The sacrifice clause is optional
//     ("you may"); AI policy is greedy-upside — sac iff there is a
//     library artifact at exactly (sac.CMC + 1). The sac is chosen by
//     ascending CMC so we trade up the smallest possible piece for the
//     biggest reachable artifact (matches the Magda/Krark-Clan ramp
//     pattern: spend a 0/1-MV chunk for a 1/2-MV rock when no bigger
//     match exists).
//   - The freshly-minted Treasure (CMC 0) is a valid sac candidate,
//     so on a board with no other noncreature artifacts Iron Man can
//     still tutor a 1-MV artifact every attack.
func registerIronManTitanOfInnovation(r *Registry) {
	r.OnTrigger("Iron Man, Titan of Innovation", "creature_attacks", ironManGeniusIndustrialist)
}

func ironManGeniusIndustrialist(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "iron_man_titan_genius_industrialist"
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

	// Step 1: create a Treasure.
	gameengine.CreateTreasureToken(gs, perm.Controller)

	// Step 2: choose a noncreature-artifact sac whose CMC+1 has a hit
	// in the library. Order candidates by ascending CMC (sac the
	// smallest sufficient piece). The Treasure we just made is in this
	// pool — it's a noncreature artifact with CMC 0.
	type candidate struct {
		perm *gameengine.Permanent
		cmc  int
	}
	var candidates []candidate
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsArtifact() {
			continue
		}
		if p.IsCreature() {
			continue
		}
		candidates = append(candidates, candidate{perm: p, cmc: cardCMC(p.Card)})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].cmc < candidates[j].cmc
	})

	var sacPick *gameengine.Permanent
	var tutored *gameengine.Card
	var sacCMC int
	for _, c := range candidates {
		want := c.cmc + 1
		hit := tutorPickFromLibrary(seat.Library, tutorFilter{
			typeRequired: "artifact",
			cmcOp:        tutorCmpEq,
			cmcN:         want,
		})
		if hit != nil {
			sacPick = c.perm
			sacCMC = c.cmc
			tutored = hit
			break
		}
	}

	if sacPick == nil || tutored == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":          perm.Controller,
			"treasure":      true,
			"sacrificed":    false,
			"sac_candidate": len(candidates),
		})
		return
	}

	sacName := sacPick.Card.DisplayName()
	gameengine.SacrificePermanent(gs, sacPick, "iron_man_genius_industrialist")
	gameengine.MoveCard(gs, tutored, perm.Controller, "library", "battlefield_tapped", slug)
	shuffleLibraryPerCard(gs, perm.Controller)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"treasure":     true,
		"sacrificed":   sacName,
		"sac_cmc":      sacCMC,
		"tutored":      tutored.DisplayName(),
		"tutored_cmc":  sacCMC + 1,
	})
}
