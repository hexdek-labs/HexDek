package per_card

import (
	"sort"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// hex_r60.go — per_card handler for Hex.
//
// Oracle text (Scryfall / ast_dataset):
//
//	Destroy six target creatures.
//
// {3}{B}{B}{B} Sorcery. A six-target removal spell (a pseudo-wrath that
// also kills indestructible-less commanders/threats across the table).
// Parses to a single inert `parsed_effect_residual` node with no
// structured multi-target Destroy, so it destroyed NOTHING (the text
// fallback has no "destroy six target creatures" shape).
//
// Implementation:
//   - OnResolve. Hat policy: select up to six creatures, preferring
//     opponents' creatures by descending power (kill the biggest
//     threats first), never targeting the caster's own board while
//     opponent creatures remain. Each goes through DestroyPermanent
//     (indestructible check, §614 replacements, dies/LTB triggers,
//     commander redirect). Printed wording requires six legal targets;
//     when fewer than six opponent creatures exist the spell still
//     destroys what it can (the resolution-time target set is the
//     dominant practical line).
func init() {
	registerHexR60(Global())
	AddResetHook(registerHexR60)
}

func registerHexR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Hex", hexResolve)
}

func hexResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "hex"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Collect opponent creatures, sorted by descending power.
	type cand struct {
		p   *gameengine.Permanent
		pow int
	}
	var cands []cand
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.IsCreature() {
				cands = append(cands, cand{p, p.Power()})
			}
		}
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].pow > cands[j].pow })

	destroyed := 0
	for i := 0; i < len(cands) && destroyed < 6; i++ {
		if gameengine.DestroyPermanent(gs, cands[i].p, nil) {
			destroyed++
		}
	}
	gameengine.StateBasedActions(gs)
	emit(gs, slug, "Hex", map[string]interface{}{
		"seat":      seat,
		"destroyed": destroyed,
	})
}
