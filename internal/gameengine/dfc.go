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
//   - EvaluateDayNightAtTurnStart(gs) → §730.2a transition at turn start
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
//   §730.2a  Day + previous active cast 0 spells → night.
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
	// Fire transform trigger so per-card handlers (Ulrich, Ajani, etc.)
	// can react to face changes.
	FireCardTrigger(gs, "transform", map[string]interface{}{
		"seat":      p.Controller,
		"perm_name": toName,
		"from_name": fromName,
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
	// CR §603.2 — "when this creature is turned face up" triggers fire on
	// the face-down → face-up transition. The chokepoint covers every
	// face-up path (morph/megamorph, disguise, cloak, manifest, "turn ~
	// face up" effects). Before r63d these AST triggers were never
	// dispatched (see FireTurnedFaceUpTriggers).
	FireTurnedFaceUpTriggers(gs, p)
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

// EvaluateDayNightAtTurnStart applies CR §730.2a at the start of each
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

// SwapToBackFace mutates a Card so its runtime identity (Name, Types,
// TypeLine, CMC) reflects the BACK face of an MDFC instead of the front
// (CR §712.11). Used at ETB by the two paths that put an MDFC onto the
// battlefield as its back face:
//
//  1. Casting the back face — handled inline in resolvePermanentSpellETB
//     (stack.go) when CastingBackFace is set; that path predates this
//     helper but performs the same swap.
//  2. Playing the back-face land as a special action — tryPlayLand
//     (tournament/turn.go) calls this after picking an MDFC whose front
//     face is not a land but whose back face is.
//
// Without this swap, the permanent on the battlefield retains the front
// face's instant/sorcery types, which §205.2 forbids (and Feynman flags
// as a critical permanent_types violation).
//
// REVERSE-MDFC GUARD: when the front face is already a land and the back
// face is an instant/sorcery (e.g. "Midgar, City of Mako // Reactor
// Raid"), swapping would discard the printed front-face land identity
// and replace it with a non-permanent type — the exact §205.2 violation
// the forward-direction swap is supposed to prevent. Refuse the swap.
// All known call sites already gate against this case; the inline check
// here is defensive depth so a future caller can't introduce the
// regression.
//
// The combined "Front // Back" runtime type signature that the deck
// parser produces (e.g. ["instant", "//", "land", "mountain"] from a
// type_line of "Instant // Land — Mountain") is replaced wholesale by
// the back-face Types, so the post-swap card carries only its actual
// land identity.
//
// AST is left alone: front-face abilities (an instant's effect) shouldn't
// fire on a land permanent, but the existing engine reads Types — not
// AST — for permanent-type SBAs and the §205 invariant. A more thorough
// fix that also swaps AST is deferred until the corpus loader carries a
// per-face AST cache.
//
// Returns true on a successful swap, false if the card isn't an MDFC,
// has no back-face data, or is a reverse MDFC (front-land/back-spell).
func SwapToBackFace(c *Card) bool {
	if c == nil || !c.IsMDFC() {
		return false
	}
	if IsReverseMDFC(c) {
		// Reverse MDFC — front is the land we want to keep. Clear any
		// transient CastingBackFace flag so a stale flip doesn't leak
		// into a downstream consumer, but otherwise leave the card's
		// Types/Name/TypeLine alone.
		c.CastingBackFace = false
		return false
	}
	c.Name = c.BackFaceName
	if len(c.BackFaceTypes) > 0 {
		c.Types = append([]string(nil), c.BackFaceTypes...)
	}
	if c.BackFaceTypeLine != "" {
		c.TypeLine = c.BackFaceTypeLine
	}
	if c.BackFaceCMC > 0 {
		c.CMC = c.BackFaceCMC
	}
	c.CastingBackFace = false
	return true
}

// IsReverseMDFC reports whether an MDFC's printed FRONT face is a land
// and the BACK face is NOT — the "Midgar, City of Mako // Reactor Raid"
// shape. Battlefield-entry helpers use this to skip the back-face swap:
// the front-face land is already the correct permanent identity, and
// swapping to the spell-typed back face would trip §205.2.
//
// Returns false for non-MDFCs, land/land MDFCs, and the standard
// spell-front/land-back MDFCs that SwapToBackFace was originally
// written for.
func IsReverseMDFC(c *Card) bool {
	if c == nil || !c.IsMDFC() {
		return false
	}
	return MDFCFrontFaceIsLand(c) && !MDFCBackFaceIsLand(c)
}

// MDFCFrontFaceIsLand reports whether an MDFC's printed FRONT face is a
// land. Reads c.TypeLine, which the deckparser populates with the full
// Scryfall "Front // Back" type line. The substring before "//" is the
// front face's printed type. False for non-MDFCs and empty type lines.
//
// Used by the land-play path to distinguish "this is genuinely a land
// in hand" (front face is a land — e.g. a normal basic) from "this is
// an MDFC being played as its back-face land" (front is instant/sorcery,
// back is a land). Only the latter needs SwapToBackFace.
func MDFCFrontFaceIsLand(c *Card) bool {
	if c == nil || !c.IsMDFC() {
		return false
	}
	front := c.TypeLine
	if i := strings.Index(front, "//"); i >= 0 {
		front = front[:i]
	}
	return strings.Contains(strings.ToLower(front), "land")
}

// MDFCBackFaceIsLand reports whether an MDFC's BACK face is a land,
// i.e. the back face's printed types include "land". Used as the gate
// for SwapToBackFace in the land-play path.
func MDFCBackFaceIsLand(c *Card) bool {
	if c == nil || !c.IsMDFC() {
		return false
	}
	for _, t := range c.BackFaceTypes {
		if strings.EqualFold(t, "land") {
			return true
		}
	}
	if c.BackFaceTypeLine != "" {
		return strings.Contains(strings.ToLower(c.BackFaceTypeLine), "land")
	}
	return false
}

// EnsureMDFCBackFaceForBattlefield is the canonical "card is about to
// enter the battlefield via a non-cast path" hook for MDFCs whose back
// face is a land. Reanimation, tutor-onto-battlefield, "put X onto the
// battlefield" effects, return-from-exile, unearth, etc. all bypass
// the casting code path that resolvePermanentSpellETB uses; without
// this swap the card lands with its front-face (instant/sorcery)
// Types and trips the §205 permanent_type SBA.
//
// Gates:
//   - card is non-nil
//   - card is not a token (token copies of MDFCs preserve the source's
//     visible face — they don't carry the printed back-face Types in
//     a meaningful way)
//   - back face is a land per MDFCBackFaceIsLand
//
// Idempotent — SwapToBackFace clears CastingBackFace and overwrites
// Types/Name/CMC with the back-face values, so calling on an already
// swapped card is a no-op in observable terms.
//
// Returns true on a successful swap, false if any gate failed.
func EnsureMDFCBackFaceForBattlefield(c *Card) bool {
	if c == nil {
		return false
	}
	if cardIsToken(c) {
		return false
	}
	if !MDFCBackFaceIsLand(c) {
		return false
	}
	return SwapToBackFace(c)
}

// StripAdventureHalfTypes drops the adventure-half (or split-card half)
// types from a Card's runtime Types slice when the card is on (or about
// to enter) the battlefield. Peer to SwapToBackFace; covers the same
// class of "Front // Back type_line" parser leak but for layouts the
// MDFC swap doesn't handle:
//
//   - layout=adventure   (e.g., "Virtue of Knowledge // Vantress Visions",
//                          "Adventurous Eater // Have a Bite")
//   - layout=split       ("Fire // Ice", "Wear // Tear")
//   - layout=aftermath   ("Driven // Despair")
//   - any other layout the deckparser populates with a combined
//     "Front // Back" type_line
//
// CR §715 (adventurer cards) and §709 (split cards): the front face's
// characteristics are the only ones present once the card is a
// permanent on the battlefield. The adventure/back half exists only
// while the card is on the stack as the alternate-cost spell.
//
// The deckparser's parseTypes splits "Creature — Human Warlock // Sorcery"
// on whitespace and produces ["creature", "human", "warlock", "//",
// "sorcery"]. That "sorcery" leak is what trips §205.2 / Feynman's
// permanent_types invariant when the creature half resolves onto the
// battlefield. This function detects the "//" pseudo-token and drops
// it plus everything after it; TypeLine is trimmed to match.
//
// Gates:
//   - card is non-nil
//   - card is not a token (token Types are engine-assigned and don't
//     contain the parser leak)
//   - Types contains "//" (otherwise no leak to strip — no-op)
//   - the prefix-before-"//" contains at least one permanent type
//     (creature/artifact/enchantment/planeswalker/land/battle); if the
//     front face has no permanent type, refuse to mutate — that
//     situation shouldn't reach the battlefield and stripping would
//     mask a real bug elsewhere.
//
// Idempotent — safe to call multiple times; the second call is a no-op
// because the "//" marker is gone from Types after the first.
//
// Order with SwapToBackFace at battlefield-entry sites: SwapToBackFace
// fires first and replaces Types wholesale with BackFaceTypes (parsed
// from the back face's separate type_line, no leak). This stripper
// runs second and is a no-op for the swapped MDFC. For Adventures
// (and other non-MDFC split layouts), SwapToBackFace is a no-op
// (gated by IsMDFC) and the stripper handles them.
//
// Returns true if Types was modified, false otherwise.
func StripAdventureHalfTypes(c *Card) bool {
	if c == nil || cardIsToken(c) {
		return false
	}
	splitIdx := -1
	for i, t := range c.Types {
		if t == "//" {
			splitIdx = i
			break
		}
	}
	if splitIdx < 0 {
		return false
	}
	// Refuse to strip if the front face has no permanent type. Lets a
	// genuinely-broken card surface the violation instead of being
	// silently scrubbed clean.
	hasPermanentType := false
	for i := 0; i < splitIdx; i++ {
		switch c.Types[i] {
		case "creature", "artifact", "enchantment", "planeswalker",
			"land", "battle":
			hasPermanentType = true
		}
		if hasPermanentType {
			break
		}
	}
	if !hasPermanentType {
		return false
	}
	c.Types = append([]string(nil), c.Types[:splitIdx]...)
	if i := strings.Index(c.TypeLine, "//"); i >= 0 {
		c.TypeLine = strings.TrimSpace(c.TypeLine[:i])
	}
	return true
}

// StripToBackHalfTypes is the metadata-free complement of
// StripAdventureHalfTypes for the FORWARD spell//permanent shape
// (mdfc-backface r63). A Card whose combined "Front // Back" type_line
// parsed to Types like ["sorcery", "//", "land", ...] but whose
// BackFace* fields were never populated (chaos-harness builders,
// un-supplemented MetaDBs) gets no help from the MDFC swap — IsMDFC()
// is false without BackFaceName — and StripAdventureHalfTypes refuses
// because the front half has no permanent type. Yet if such a card is
// entering the battlefield, the BACK half is the only face that can
// legally be there (the front face is an instant/sorcery and CR
// §304.4/§307.1 bar it from being a permanent), so the back-half types
// are the correct permanent identity: spell//land MDFCs played as
// lands (Agadeem's Awakening, Malakir Rebirth, …) and spell//creature
// DFCs resolving as the creature (Visitor from Planet Q).
//
// Gates (mirror StripAdventureHalfTypes, inverted halves):
//   - card is non-nil and not a token
//   - Types contains the "//" pseudo-token
//   - the front half (before "//") has NO permanent type — when it
//     does, the front face is the battlefield identity and
//     StripAdventureHalfTypes owns the cleanup
//   - the back half (after "//") HAS a permanent type — a pure
//     spell//spell split card is refused so the §205 violation
//     surfaces instead of being silently scrubbed
//
// Name and CMC are intentionally left alone: without face metadata
// there is no authoritative back-face name/cost, and handlers key off
// the combined oracle name. Idempotent — the "//" marker is gone from
// Types after the first call.
//
// Returns true if Types was modified, false otherwise.
func StripToBackHalfTypes(c *Card) bool {
	if c == nil || cardIsToken(c) {
		return false
	}
	splitIdx := -1
	for i, t := range c.Types {
		if t == "//" {
			splitIdx = i
			break
		}
	}
	if splitIdx < 0 {
		return false
	}
	if typesHavePermanentType(c.Types[:splitIdx]) {
		return false
	}
	if !typesHavePermanentType(c.Types[splitIdx+1:]) {
		return false
	}
	c.Types = append([]string(nil), c.Types[splitIdx+1:]...)
	if i := strings.Index(c.TypeLine, "//"); i >= 0 {
		c.TypeLine = strings.TrimSpace(c.TypeLine[i+len("//"):])
	}
	return true
}

// typesHavePermanentType reports whether the slice carries at least one
// CR §205.2 permanent card type.
func typesHavePermanentType(types []string) bool {
	for _, t := range types {
		switch t {
		case "creature", "artifact", "enchantment", "planeswalker",
			"land", "battle":
			return true
		}
	}
	return false
}

// EnsureBattlefieldFrontFace is the canonical "card is about to enter
// the battlefield via any path" hook for face-cleanup. Composes the
// MDFC back-face swap and the half-type strippers in the right order:
//
//  1. EnsureMDFCBackFaceForBattlefield — if the card is an MDFC whose
//     back face is a land (per its BackFace* metadata), swap Types
//     wholesale to the back-face values. No-op for non-MDFCs and for
//     cards whose back-face metadata was never populated.
//  2. StripAdventureHalfTypes — if Types still carries a "//" leak and
//     the FRONT half has a permanent type (adventure / split / reverse
//     MDFC), drop everything from "//" onward. No-op when
//     SwapToBackFace already replaced Types.
//  3. StripToBackHalfTypes — if Types still carries the leak and only
//     the BACK half has a permanent type (metadata-less spell//land
//     MDFC, spell//creature DFC), keep the back-half types instead.
//
// The two strippers gate on mutually exclusive front-half shapes, so
// at most one fires; a spell//spell split card passes through all
// three untouched and surfaces its §205 violation downstream.
//
// Call sites that previously invoked EnsureMDFCBackFaceForBattlefield
// can switch to this function to pick up the adventure handling for
// free; existing call sites that don't switch get the MDFC fix only
// (which is what they previously had).
//
// Idempotent.
func EnsureBattlefieldFrontFace(c *Card) {
	EnsureMDFCBackFaceForBattlefield(c)
	StripAdventureHalfTypes(c)
	StripToBackHalfTypes(c)
}
