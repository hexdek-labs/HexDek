package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSovereignOkinecAhau wires Sovereign Okinec Ahau.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Ward {2}
//	Whenever Sovereign Okinec Ahau attacks, for each creature you
//	control with power greater than that creature's base power, put a
//	number of +1/+1 counters on that creature equal to the difference.
//
// Mana cost: {2}{G}{W}. P/T 3/4.
//
// Implementation:
//   - Ward {2} is intrinsic to the card AST keyword stack — no per-card
//     work needed; the engine's CheckWardOnTargeting already handles it
//     when the AST keyword is loaded. We also stamp Permanent.Flags
//     ["ward_cost"] = 2 on ETB so test fixtures without a full AST still
//     observe the trigger.
//   - "creature_attacks": when Okinec is the declared attacker, walk
//     Okinec's controller's battlefield. For each creature whose current
//     Power() exceeds its Card.BasePower (i.e., it has been pumped by
//     +1/+1 counters or static modifications), add +1/+1 counters equal
//     to the difference. Token creatures and creatures whose printed
//     base power is 0 still qualify whenever current power > base.
//   - Counter additions snowball within the same trigger: each iteration
//     reads Power() at trigger-resolution time, so an earlier creature's
//     buff doesn't affect a later one's diff calculation (we snapshot
//     diffs first, then apply).
func registerSovereignOkinecAhau(r *Registry) {
	r.OnETB("Sovereign Okinec Ahau", sovereignOkinecAhauETB)
	r.OnTrigger("Sovereign Okinec Ahau", "creature_attacks", sovereignOkinecAhauAttack)
}

func sovereignOkinecAhauETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "sovereign_okinec_ahau_static"
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["kw:ward"] = 1
	perm.Flags["ward_cost"] = 2
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"ward_cost": 2,
	})
}

func sovereignOkinecAhauAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sovereign_okinec_ahau_attack_pump"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atkPerm, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atkPerm != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}

	// Snapshot diffs before mutating to avoid feedback loops within this
	// trigger — each creature's bonus is computed against the state at
	// trigger resolution, not after earlier creatures grow.
	type pump struct {
		target *gameengine.Permanent
		diff   int
	}
	var pumps []pump
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		base := p.Card.BasePower
		cur := p.Power()
		diff := cur - base
		if diff <= 0 {
			continue
		}
		pumps = append(pumps, pump{target: p, diff: diff})
	}

	type buffed struct {
		Name     string `json:"name"`
		Diff     int    `json:"diff"`
		Counters int    `json:"counters"`
	}
	var buffs []buffed
	for _, pp := range pumps {
		pp.target.AddCounter("+1/+1", pp.diff)
		buffs = append(buffs, buffed{
			Name:     pp.target.Card.DisplayName(),
			Diff:     pp.diff,
			Counters: pp.target.Counters["+1/+1"],
		})
	}
	if len(buffs) > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"buffed_count":   len(buffs),
		"buffed_targets": buffs,
	})
}
