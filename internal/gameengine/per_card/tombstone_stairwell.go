package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTombstoneStairwell wires Tombstone Stairwell (Muninn parser-gap
// #57, 13,124 hits).
//
// Oracle text (Scryfall, verified 2026-05-16 via hexdek.dev oracle):
//
//	{2}{B}{B}
//	World Enchantment
//	Cumulative upkeep {1}{B}
//	At the beginning of each upkeep, if this enchantment is on the
//	battlefield, each player creates a 2/2 black Zombie creature token
//	with haste named Tombspawn for each creature card in their
//	graveyard.
//	At the beginning of each end step and when this enchantment leaves
//	the battlefield, destroy all tokens created with this enchantment.
//	They can't be regenerated.
//
// Implementation:
//   - Cumulative upkeep: engine-side (keywords_batch.go::ApplyCumulativeUpkeep).
//     The Type line carries the keyword and the engine accumulates age
//     counters per upkeep; we set perm.Flags["cumulative_upkeep_cost"] = 2
//     at ETB so the cost matches printed {1}{B} (2 mana value).
//   - "upkeep_controller" trigger fires once per upkeep step (event_aliases.go
//     maps it from canonical "upkeep"). Each firing creates Tombspawn
//     tokens for every player based on that player's creature-card
//     count in graveyard. The tokens are tagged with a per-Stairwell
//     timestamp so EOT cleanup picks them up correctly even with
//     multiple Stairwells in play.
//   - "end_step" trigger destroys all Tombspawns linked to this
//     permanent's timestamp.
//   - "permanent_ltb" trigger destroys the same set when the Stairwell
//     itself leaves play. The "can't be regenerated" clause is
//     observable on the destroy event but the engine's regeneration
//     code path is rare in current decks; we route through
//     DestroyPermanent which respects indestructible (per CR §704.5g
//     they're the only relevant interaction here).
func registerTombstoneStairwell(r *Registry) {
	r.OnETB("Tombstone Stairwell", tombstoneStairwellETB)
	r.OnTrigger("Tombstone Stairwell", "upkeep_controller", tombstoneStairwellUpkeep)
	r.OnTrigger("Tombstone Stairwell", "end_step", tombstoneStairwellEndStep)
	r.OnTrigger("Tombstone Stairwell", "permanent_ltb", tombstoneStairwellLTB)
}

func tombstoneStairwellETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	// {1}{B} per age counter → CMC 2.
	perm.Flags["cumulative_upkeep_cost"] = 2
}

func tombstoneStairwellUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "tombstone_stairwell_upkeep_tombspawn"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	linkType := tombstoneStairwellLinkType(perm)
	totals := map[int]int{}
	for _, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		n := 0
		for _, c := range s.Graveyard {
			if c != nil && cardHasType(c, "creature") {
				n++
			}
		}
		if n == 0 {
			continue
		}
		for i := 0; i < n; i++ {
			token := &gameengine.Card{
				Name:          "Tombspawn",
				Owner:         s.Idx,
				Types:         []string{"creature", "token", "zombie", "haste", "pip:B", linkType},
				Colors:        []string{"B"},
				BasePower:     2,
				BaseToughness: 2,
			}
			enterBattlefieldWithETB(gs, s.Idx, token, false)
		}
		totals[s.Idx] = n
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"controller":     perm.Controller,
		"tokens_by_seat": totals,
	})
}

func tombstoneStairwellEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	tombstoneStairwellDestroyLinked(gs, perm, "tombstone_stairwell_end_step_destroy")
}

func tombstoneStairwellLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	// r63 self-gate: destroy the linked Tombspawn tokens only on Tombstone
	// Stairwell's OWN leave. Without this the all-battlefield permanent_ltb
	// walk fires this on every foreign LTB, prematurely wiping the Stairwell's
	// own Zombie army while it is still in play. The shared helper is also
	// used by the legitimate end_step path, so the gate lives here.
	if perm == nil {
		return
	}
	if p, _ := ctx["perm"].(*gameengine.Permanent); p != perm {
		return
	}
	tombstoneStairwellDestroyLinked(gs, perm, "tombstone_stairwell_ltb_destroy")
}

func tombstoneStairwellDestroyLinked(gs *gameengine.GameState, perm *gameengine.Permanent, slug string) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	linkType := tombstoneStairwellLinkType(perm)
	destroyed := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		// Snapshot — DestroyPermanent mutates Battlefield.
		bf := append([]*gameengine.Permanent(nil), s.Battlefield...)
		for _, p := range bf {
			if p == nil || p.Card == nil {
				continue
			}
			if cardHasType(p.Card, linkType) {
				gameengine.DestroyPermanent(gs, p, perm)
				destroyed++
			}
		}
	}
	if destroyed == 0 {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"controller": perm.Controller,
		"destroyed":  destroyed,
	})
}

// tombstoneStairwellLinkType returns the per-Stairwell type tag that
// brands tokens minted by this specific permanent, so EOT cleanup and
// LTB only sweep tokens belonging to the source that triggered.
func tombstoneStairwellLinkType(perm *gameengine.Permanent) string {
	ts := 0
	if perm != nil {
		ts = perm.Timestamp
	}
	// Lowercase + no spaces so cardHasType's strings.ToLower match works.
	return "tombstone_stairwell_token_ts" + intToASCII(ts)
}

func intToASCII(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
