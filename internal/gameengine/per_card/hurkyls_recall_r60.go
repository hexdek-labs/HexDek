package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// hurkyls_recall_r60.go — per_card handler for Hurkyl's Recall.
//
// Oracle text (Scryfall / ast_dataset):
//
//	Return all artifacts target player owns to their hand.
//
// {1}{U} Instant. A premier artifact-hate / combo-protection bounce —
// blanks an opponent's whole artifact board (or, self-targeted, saves
// your own artifacts from a wrath or re-triggers ETBs). Parses to a
// single inert `parsed_effect_residual` node with no mass-bounce
// structure, so it returned NOTHING (the text fallback has no "return
// all artifacts target player owns" shape).
//
// Implementation:
//   - OnResolve. "Target player" — hat policy picks the opponent
//     controlling the most artifacts (maximum disruption); if no
//     opponent has any artifact, declines (no legal/beneficial target).
//     CR note: the ability keys off OWNER, so it returns every artifact
//     that player owns regardless of who controls it — we scan all
//     battlefields for permanents whose Owner == target and that are
//     artifacts, and bounce each to its owner's hand.
func init() {
	registerHurkylsRecallR60(Global())
	AddResetHook(registerHurkylsRecallR60)
}

func registerHurkylsRecallR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Hurkyl's Recall", hurkylsRecallResolve)
}

func hurkylsRecallResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "hurkyls_recall"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Count artifacts owned by each opponent; pick the max.
	count := func(owner int) int {
		n := 0
		for _, s := range gs.Seats {
			if s == nil {
				continue
			}
			for _, p := range s.Battlefield {
				if p != nil && p.Owner == owner && p.IsArtifact() {
					n++
				}
			}
		}
		return n
	}
	target := -1
	best := 0
	for _, opp := range gs.Opponents(seat) {
		if c := count(opp); c > best {
			best = c
			target = opp
		}
	}
	if target < 0 {
		emitFail(gs, slug, "Hurkyl's Recall", "no_artifacts_to_bounce", nil)
		return
	}

	// Snapshot before mutating.
	var arts []*gameengine.Permanent
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.Owner == target && p.IsArtifact() {
				arts = append(arts, p)
			}
		}
	}
	bounced := 0
	for _, p := range arts {
		if gameengine.BouncePermanent(gs, p, nil, "hand") {
			bounced++
		}
	}
	gameengine.StateBasedActions(gs)
	emit(gs, slug, "Hurkyl's Recall", map[string]interface{}{
		"seat":        seat,
		"target_seat": target,
		"bounced":     bounced,
	})
}
