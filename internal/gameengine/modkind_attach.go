package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// modkind_attach.go — generic coverage for the `attach` scaffold ModKind:
// Equipment/Auras that attach themselves to a creature when they enter,
// e.g.
//
//	When this Equipment enters, attach it to target creature you control.
//
// The AST carries args = [ "<target filter>" ] (e.g. "creature you
// control", "knight you control", "creature an opponent controls"). The
// `attach` kind fell through to the switch default and did NOTHING, so
// 55 auto-attaching cards entered UNATTACHED — Embercleave, Cranial
// Plating, Maul of the Skyclaves, Mithril Coat, and a long tail of ETB-
// attach Equipment plus a few Auras — giving their equipped-creature
// buffs to nobody.
//
// Why a dedicated handler rather than the existing attach_self_to case:
// that case resolves its target via pickTargetFromModArgs, which for an
// unrecognized hint like "creature you control" falls through to
// pickCreatureTargets(.., wantOpponent=true) — it would attach a
// "creature you control" Equipment to an OPPONENT's creature. This
// handler reads the filter's controller-side ("you control" vs "an
// opponent controls") and any subtype / legendary / non-human constraint
// directly, and picks the controller's highest-power matching creature
// (AI: equip the best threat). No legal target → stays unattached
// (correct for "attach to target creature" with an empty board side).
//
// Equipment buffs are computed from AttachedTo by the layer system, so
// setting AttachedTo is the whole effect (mirrors the attach_self_to
// case's shape).

func resolveAttachOnEnter(gs *GameState, src *Permanent, e *gameast.ModificationEffect) {
	if gs == nil || src == nil {
		return
	}
	seat := controllerSeat(src)
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	filter := strings.ToLower(strings.TrimSpace(modArgString(e.Args, 0)))

	target := pickAttachTarget(gs, src, seat, filter)
	if target != nil {
		src.AttachedTo = target
	}

	attachedName := ""
	if target != nil && target.Card != nil {
		attachedName = target.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "attach",
		Seat:   seat,
		Source: sourceName(src),
		Details: map[string]interface{}{
			"sub_kind":    "attach",
			"filter":      filter,
			"attached":    target != nil,
			"attached_to": attachedName,
		},
	})
}

// pickAttachTarget chooses the creature an `attach`-on-enter Equipment/
// Aura should latch onto: the highest-power creature on the filter's
// controller-side that satisfies any subtype / legendary / non-human
// constraint encoded in the filter text. Returns nil when no legal
// target exists (the permanent then stays unattached).
func pickAttachTarget(gs *GameState, src *Permanent, seat int, filter string) *Permanent {
	wantOpponent := strings.Contains(filter, "opponent")
	var seats []int
	if wantOpponent {
		seats = gs.Opponents(seat)
	} else {
		seats = []int{seat}
	}

	subtype := attachFilterSubtype(filter)
	nonHuman := strings.Contains(filter, "non-human") || strings.Contains(filter, "nonhuman")
	legendary := strings.Contains(filter, "legendary")

	var best *Permanent
	bestPow := -1
	for _, ss := range seats {
		if ss < 0 || ss >= len(gs.Seats) || gs.Seats[ss] == nil {
			continue
		}
		for _, p := range gs.Seats[ss].Battlefield {
			if p == nil || p == src || p.Card == nil || !p.IsCreature() {
				continue
			}
			if subtype != "" && !cardHasSubtype(p.Card, subtype) {
				continue
			}
			if nonHuman && cardHasSubtype(p.Card, "human") {
				continue
			}
			if legendary && !cardHasType(p.Card, "legendary") {
				continue
			}
			if pw := p.Power(); pw > bestPow {
				bestPow = pw
				best = p
			}
		}
	}
	return best
}

// attachFilterSubtype extracts a creature subtype constraint from a
// target-filter string (e.g. "knight you control" → "knight"), or ""
// when the filter is a plain "creature"/adjective form. The subtype is
// the leading word unless it is the generic "creature" or an adjective
// handled separately (legendary / non-human).
func attachFilterSubtype(filter string) string {
	fields := strings.Fields(filter)
	if len(fields) == 0 {
		return ""
	}
	switch fields[0] {
	case "creature", "legendary", "non-human", "nonhuman", "a", "an", "the", "any", "target":
		return ""
	}
	return fields[0]
}
