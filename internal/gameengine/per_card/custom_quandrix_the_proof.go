package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerQuandrixTheProofCustom wires the ETB +1/+1-doubler and the
// optional from-command-zone counter distribution. The auto-generated
// gen_quandrix_the_proof.go was a partial-emit stub; both the file and
// its batch_generated.go register call were deleted in Versailles
// Phase 2I — this is now the sole registration site, called from
// registry.go::registerDefaults.
//
// Oracle text (Strixhaven / Commander 2021, {3}{G}{U}):
//
//	Flying, hexproof from creatures
//	When Quandrix, the Proof enters the battlefield, double the number
//	of +1/+1 counters on each creature you control. Then if Quandrix
//	entered the battlefield from the command zone, distribute X +1/+1
//	counters among any number of target creatures you control, where
//	X is the number of cards in your hand.
//
// Implementation:
//   - OnETB: walk every creature on the controller's battlefield;
//     existing[+1/+1] *= 2 by adding `existing` more counters.
//   - From-command-zone gate: we approximate it via the Permanent's
//     CastZone flag (ZoneCommand) — set by the cast pipeline when
//     casting from the command zone. If true, distribute X counters
//     to the highest-power creature (greedy single-target heuristic;
//     matches how the AI typically plays "any number of targets").
//   - Flying / hexproof from creatures — AST keyword pipeline.
func registerQuandrixTheProofCustom(r *Registry) {
	r.OnETB("Quandrix, the Proof", quandrixTheProofETB)
}

func quandrixTheProofETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "quandrix_the_proof_etb_double_counters"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Lost {
		return
	}

	doubled := 0
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		existing := 0
		if p.Counters != nil {
			existing = p.Counters["+1/+1"]
		}
		if existing > 0 {
			p.AddCounter("+1/+1", existing) // doubles via add-equal
			doubled++
		}
	}
	gs.InvalidateCharacteristicsCache()

	// From-command-zone gate. Best-effort: check perm.Flags["from_command"]
	// (set by some cast-pipeline branches) or fall back to the controller's
	// commander cast count > 0 for THIS card. If neither flag is reliable,
	// we still emit the partial so audits flag the gap.
	fromCommand := false
	if perm.Flags != nil && perm.Flags["from_command_zone"] == 1 {
		fromCommand = true
	}
	if !fromCommand && seat.CommanderCastCounts != nil && seat.CommanderCastCounts[perm.Card.Name] > 0 {
		fromCommand = true
	}

	distributed := 0
	if fromCommand {
		x := len(seat.Hand)
		if x > 0 {
			// Greedy: pile all counters on the highest-power creature.
			var target *gameengine.Permanent
			for _, p := range seat.Battlefield {
				if p == nil || !p.IsCreature() {
					continue
				}
				if target == nil || p.Power() > target.Power() {
					target = p
				}
			}
			if target != nil {
				target.AddCounter("+1/+1", x)
				distributed = x
				gs.InvalidateCharacteristicsCache()
			}
		}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         seatIdx,
		"doubled":      doubled,
		"distributed":  distributed,
		"from_command": fromCommand,
	})
	if !fromCommand {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"from_command_zone_detection_partial_distribute_x_clause_skipped")
	}
}
