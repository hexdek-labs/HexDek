package gameengine

// CR §712 (double-faced cards) + §726 (Day/Night) + §702.144
// (Daybound) + §702.145 (Nightbound).
//
// This file mirrors scripts/playloop.py's day/night and transform
// helpers:
//
//   - TransformPermanent(gs, p, reason) → §712 face swap
//   - HasDayboundOrNightboundPermanent(gs) → §726.2 trigger
//   - MaybeBecomeDay(gs, reason) → §726.2 state-initial transition
//   - SetDayNight(gs, new, reason, rule) → shared transition primitive
//   - ApplyDayboundNightboundTransforms(gs) → §702.144/145 sweep
//   - EvaluateDayNightAtTurnStart(gs) → §726.3a transition at turn start
//
// Contracts:
//
//   - Transforming a non-DFC permanent is a no-op that returns false;
//     callers can assert on the return when they're sure the permanent
//     should have been a DFC.
//   - A DFC permanent's Card.AST must start on FrontFaceAST; the Card
//     loader populates FrontFaceAST + BackFaceAST at ETB (see
//     InitDFCFaces below).
//   - SetDayNight is idempotent — if the new state equals the current
//     state, nothing happens and no event is logged.
//
// Comp-rules citations (data/rules/MagicCompRules-20260227.txt):
//
//   §712.1   A double-faced card has two faces, a front and a back;
//            only one face is up at a time.
//   §712.2   A double-faced card enters the battlefield with its
//            front face up by default.
//   §712.3   Transform swaps which face is up. The permanent keeps
//            its counters, attachments, etc.
//   §712.8   A permanent gets a new timestamp when it transforms.
//   §726.2   The game begins "neither day nor night." It becomes day
//            the first time a permanent with daybound or nightbound
//            enters the battlefield.
//   §726.3a  Day + previous active cast 0 spells → night.
//            Night + previous active cast 2+ spells → day.
//   §702.144 Daybound — while night, daybound creatures transform.
//   §702.145 Nightbound — while day, nightbound creatures transform.

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// HasDayboundOrNightboundPermanent returns true iff any permanent on
// any battlefield carries daybound or nightbound (on either face).
// §726.2 trigger.
func HasDayboundOrNightboundPermanent(gs *GameState) bool {
	if gs == nil {
		return false
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if astHasKeyword(p.Card.AST, "daybound") ||
				astHasKeyword(p.Card.AST, "nightbound") {
				return true
			}
			if astHasKeyword(p.FrontFaceAST, "daybound") ||
				astHasKeyword(p.FrontFaceAST, "nightbound") {
				return true
			}
			if astHasKeyword(p.BackFaceAST, "daybound") ||
				astHasKeyword(p.BackFaceAST, "nightbound") {
				return true
			}
		}
	}
	return false
}

// astHasKeyword returns true iff the CardAST's abilities contain a
// Keyword node matching `name` (case-insensitive).
func astHasKeyword(ast *gameast.CardAST, name string) bool {
	if ast == nil {
		return false
	}
	target := strings.ToLower(name)
	for _, ab := range ast.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if strings.ToLower(kw.Name) == target {
			return true
		}
	}
	return false
}

// permanentActiveFaceHasKeyword returns true iff the permanent's
// CURRENTLY-ACTIVE face carries the named keyword. Reads from
// perm.Card.AST (which is kept swapped in sync with perm.Transformed
// by TransformPermanent).
func permanentActiveFaceHasKeyword(p *Permanent, name string) bool {
	if p == nil || p.Card == nil {
		return false
	}
	return astHasKeyword(p.Card.AST, name)
}

// TransformPermanent swaps a DFC permanent's active face (CR §712.3).
// Returns true on success, false if the permanent isn't a DFC.
//
// On transform:
//   - p.Transformed toggles.
//   - p.Card.AST swaps between FrontFaceAST and BackFaceAST.
//   - p.Card.Name swaps between FrontFaceName and BackFaceName.
//   - p.Timestamp is refreshed (§712.8).
//   - Characteristics cache is invalidated (§613 re-tagging).
//   - A `transform` event is logged.
//
// Unlike the Python side which reconstructs a new CardEntry on every
// transform, the Go side mutates p.Card in place — the runtime Card
// struct is per-permanent by convention (tokens/copies already get
// their own Card instance), so we don't accidentally leak state across
// other game objects. Callers that share a Card pointer across
// multiple Permanents (which is illegal anyway) would need a deep
// copy.
func TransformPermanent(gs *GameState, p *Permanent, reason string) bool {
	if gs == nil || p == nil || p.Card == nil {
		return false
	}
	if p.FrontFaceAST == nil || p.BackFaceAST == nil {
		// Not a DFC — log a no-op event so callers can observe.
		gs.LogEvent(Event{
			Kind:   "transform_noop",
			Seat:   p.Controller,
			Source: p.Card.DisplayName(),
			Details: map[string]interface{}{
				"reason": reason,
				"rule":   "712.1",
				"cause":  "not_dfc",
			},
		})
		return false
	}
	frontActive := !p.Transformed
	targetFront := !frontActive
	fromName := p.Card.DisplayName()
	var toName string
	if targetFront {
		p.Card.AST = p.FrontFaceAST
		if p.FrontFaceName != "" {
			p.Card.Name = p.FrontFaceName
		}
		toName = p.FrontFaceName
	} else {
		p.Card.AST = p.BackFaceAST
		if p.BackFaceName != "" {
			p.Card.Name = p.BackFaceName
		}
		toName = p.BackFaceName
	}
	p.Transformed = !p.Transformed
	// §712.8 — new timestamp.
	p.Timestamp = gs.NextTimestamp()
	// §613 re-tagging: invalidate the cached characteristics so the
	// next read picks up the swap.
	gs.InvalidateCharacteristicsCache()
	gs.LogEvent(Event{
		Kind:   "transform",
		Seat:   p.Controller,
		Source: fromName,
		Details: map[string]interface{}{
			"rule":            "712.3",
			"to_face":         toName,
			"now_transformed": p.Transformed,
			"reason":          reason,
		},
	})
	return true
}

// TurnFaceUp turns a face-down permanent (morph, manifest, Ixidron'd)
// face-up. Per CR §702.36e, a face-down permanent can be turned face-up
// at any time its controller could pay the morph cost (as a special
// action that doesn't use the stack). On turning face-up:
//   - Card.FaceDown is cleared.
//   - The permanent's full characteristics are restored (name, types,
//     abilities, P/T). The layer system handles this via the face-down
//     override check in BaseCharacteristics.
//   - A "turn_face_up" event is logged.
//   - Characteristics cache is invalidated.
//
// The morph cost payment is NOT enforced here — the caller (Hat or
// per-card handler) is responsible for paying the cost before calling
// TurnFaceUp. This keeps the function usable for manifest and other
// face-up effects too.
func TurnFaceUp(gs *GameState, p *Permanent, reason string) bool {
	if gs == nil || p == nil || p.Card == nil {
		return false
	}
	if !p.Card.FaceDown {
		return false // already face-up
	}
	p.Card.FaceDown = false
	// §712.8 equivalent — new timestamp on face change.
	p.Timestamp = gs.NextTimestamp()
	gs.InvalidateCharacteristicsCache()
	gs.LogEvent(Event{
		Kind:   "turn_face_up",
		Seat:   p.Controller,
		Source: p.Card.DisplayName(),
		Details: map[string]interface{}{
			"reason": reason,
			"rule":   "702.36e",
		},
	})
	return true
}

// SetDayNight transitions the game's day/night state. Mirrors Python
// `_set_day_night`. Idempotent — does nothing if `newState` already
// matches gs.DayNight. On an actual transition, emits the
// `day_night_change` event and fires ApplyDayboundNightboundTransforms.
//
// Valid `newState`: DayNightNeither, DayNightDay, DayNightNight.
func SetDayNight(gs *GameState, newState, reason, rule string) {
	if gs == nil {
		return
	}
	if gs.DayNight == newState {
		return
	}
	prev := gs.DayNight
	gs.DayNight = newState
	gs.LogEvent(Event{
		Kind:   "day_night_change",
		Seat:   -1,
		Target: -1,
		Details: map[string]interface{}{
			"from_state": prev,
			"to_state":   newState,
			"reason":     reason,
			"rule":       rule,
		},
	})
	ApplyDayboundNightboundTransforms(gs)
}

// MaybeBecomeDay enforces CR §726.2 — if the game is currently
// "neither" and a daybound/nightbound permanent exists on any
// battlefield, transition to day. Idempotent.
func MaybeBecomeDay(gs *GameState, reason string) {
	if gs == nil {
		return
	}
	if gs.DayNight != DayNightNeither {
		return
	}
	if !HasDayboundOrNightboundPermanent(gs) {
		return
	}
	SetDayNight(gs, DayNightDay, reason, "726.2")
}

// ApplyDayboundNightboundTransforms walks every permanent and flips
// daybound/nightbound faces so the active face matches the current
// day/night state. Mirrors Python apply_daybound_nightbound_transforms.
//
// §702.144: while night, daybound active face → transform to back.
// §702.145: while day, nightbound active face → transform to front.
//
// A no-op when state is DayNightNeither.
func ApplyDayboundNightboundTransforms(gs *GameState) {
	if gs == nil {
		return
	}
	state := gs.DayNight
	if state != DayNightDay && state != DayNightNight {
		return
	}
	// Snapshot pointers to avoid mutation-during-iteration if the
	// transform has a side effect that touches the battlefield.
	var pool []*Permanent
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		pool = append(pool, s.Battlefield...)
	}
	for _, p := range pool {
		if p == nil || p.Card == nil {
			continue
		}
		if p.FrontFaceAST == nil || p.BackFaceAST == nil {
			continue
		}
		hasDaybound := permanentActiveFaceHasKeyword(p, "daybound")
		hasNightbound := permanentActiveFaceHasKeyword(p, "nightbound")
		if state == DayNightDay && hasNightbound {
			TransformPermanent(gs, p, "state_became_day")
		} else if state == DayNightNight && hasDaybound {
			TransformPermanent(gs, p, "state_became_night")
		}
	}
}

// EvaluateDayNightAtTurnStart applies CR §726.3a at the start of each
// turn BEFORE the untap step. Reads gs.SpellsCastByActiveLastTurn
// (populated by the turn loop before rotating active).
//
// Transitions:
//
//	DayNightDay + last active cast 0 spells → DayNightNight
//	DayNightNight + last active cast 2+ spells → DayNightDay
//
// All other combinations: no change.
func EvaluateDayNightAtTurnStart(gs *GameState) {
	if gs == nil {
		return
	}
	last := gs.SpellsCastByActiveLastTurn
	switch gs.DayNight {
	case DayNightDay:
		if last == 0 {
			SetDayNight(gs, DayNightNight,
				"prev_active_cast_zero", "726.3a")
		}
	case DayNightNight:
		if last >= 2 {
			SetDayNight(gs, DayNightDay,
				"prev_active_cast_two_plus", "726.3a")
		}
	}
}

// InitDFCFaces seeds a Permanent's FrontFaceAST / BackFaceAST / face
// names. Call this at ETB for any Permanent whose Card is a DFC.
// The convention: the Card that enters the battlefield already has
// Card.AST pointing at the front face. The corpus loader knows about
// both faces; this helper just wires the per-Permanent cache.
//
// If the corpus doesn't carry back-face information, front == back
// (effectively) and transform will be a no-op.
//
// The front-face name should be the deck-file-facing name (the thing
// the player writes on their decklist). The back-face name is the
// oracle's back-half name. If the Card's .Name is the full
// "Front // Back" oracle name, callers should pre-split it; this
// helper doesn't parse.
func InitDFCFaces(p *Permanent,
	frontAST, backAST *gameast.CardAST,
	frontName, backName string) {
	if p == nil {
		return
	}
	p.FrontFaceAST = frontAST
	p.BackFaceAST = backAST
	p.FrontFaceName = frontName
	p.BackFaceName = backName
}

// (InvalidateCharacteristicsCache lives in layers.go — the real §613
// implementation bumps charCacheEpoch. Transform relies on it to
// ensure the next characteristics read picks up the face swap.)
