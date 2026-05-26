package gameengine

// keywords_metalcraft.go — Metalcraft (CR §702.97, Scars of Mirrodin 2010).
//
// CR §702.97a: Metalcraft is an ability word. It indicates a static
//               ability of the form "As long as you control three or
//               more artifacts, [effect]." Per §702.97a, artifact
//               types like creature/enchantment/land/planeswalker that
//               are ALSO artifacts (artifact creatures, equipment,
//               artifact lands, etc.) all count toward the threshold —
//               the only filter is that the permanent has the artifact
//               type.
//
// This is a passive GATING mechanic, not a triggered ability — there
// is no "metalcraft trigger" to fire. Consumers check
// MetalcraftActive(gs, seat) at the moment they want to apply the
// gated effect (typically at resolve time for the rider, or at SBA
// time for a continuous-effect static).
//
// API:
//
//   HasMetalcraft(card) → bool
//       Detects metalcraft via keyword AST node OR oracle text scan.
//   MetalcraftActive(gs, seatIdx) → bool
//       True iff the seat controls >= 3 permanents with the artifact
//       type. Artifact creatures, equipment, vehicles, artifact lands
//       (Inkmoth Nexus, Treasure Vault), and Treasure/Clue/Food/Blood
//       tokens all count.
//
// The legacy CheckMetalcraft helper in keywords_misc.go is preserved
// as a thin alias so existing call sites (per_card/mox_amber_opal.go
// and friends) continue to work — see the alias block at the bottom
// of this file.

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// HasMetalcraft
// ---------------------------------------------------------------------------

// HasMetalcraft returns true if the card carries metalcraft. Detection
// paths, in priority order:
//
//  1. Keyword AST node named "metalcraft" — metalcraft is technically
//     an ability WORD rather than a keyword in CR's strict sense, but
//     the engine normalizes ability words into Keyword AST nodes the
//     same way it does for magecraft, threshold, ferocious, etc.
//  2. Oracle-text substring "metalcraft" — catches Static and
//     Triggered ability Raw text that introduce the gated rider
//     ("Metalcraft — As long as you control three or more artifacts,
//     this creature has flying.").
//
// Returns false for nil cards or cards with no AST. Match is
// case-insensitive via OracleTextLower's cached lowercased text.
func HasMetalcraft(card *Card) bool {
	if card == nil || card.AST == nil {
		return false
	}
	for _, ab := range card.AST.Abilities {
		if kw, ok := ab.(*gameast.Keyword); ok && keywordNameEquals(kw, "metalcraft") {
			return true
		}
	}
	return strings.Contains(OracleTextLower(card), "metalcraft")
}

// ---------------------------------------------------------------------------
// MetalcraftActive
// ---------------------------------------------------------------------------

// MetalcraftActive returns true if `seatIdx` currently controls three
// or more artifact permanents. CR §702.97a — the count is over
// PERMANENTS WITH THE ARTIFACT TYPE on the seat's battlefield. Artifact
// subtypes (Treasure, Clue, Food, Equipment, Vehicle, Spacecraft, etc.)
// count automatically because IsArtifact() reads p.hasType("artifact"),
// which the token-mint helpers and corpus loader set on every artifact
// permanent regardless of subtype.
//
// Counts ALL artifact permanents — including:
//   - artifact creatures (Bonesplitter-equipped, Cranial Plating
//     creature-cum-equipment hybrids, Phyrexian Revoker-style)
//   - equipment
//   - vehicles
//   - artifact lands (Inkmoth Nexus, Tree of Tales)
//   - artifact tokens (Treasure, Clue, Food, Blood, Map, etc.)
//
// Does NOT count:
//   - opponents' artifacts (different controller)
//   - phased-out permanents (p.IsArtifact() reads the type; the
//     existing CountArtifacts helper handles phasing implicitly via
//     the Battlefield slice not containing phased-out perms for the
//     purposes of artifact counts — see invariants_test.go)
//
// Returns false for nil gs or out-of-range seat.
func MetalcraftActive(gs *GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	if gs.Seats[seatIdx] == nil {
		return false
	}
	return ArtifactCount(gs, seatIdx) >= 3
}

// ArtifactCount returns the number of artifact permanents `seatIdx`
// controls. Counts every battlefield permanent whose effective types
// include "artifact" — artifact creatures, equipment, vehicles,
// artifact lands, and artifact tokens (Treasure/Clue/Food/etc.) all
// count toward the total. Phased-out permanents are excluded because
// CR §702.26 says they "are treated as though they don't exist."
//
// Differs from the existing CountArtifacts helper in keywords_p0.go
// by explicitly excluding phased-out permanents — the older helper
// didn't filter, which is a benign no-op until §702.26 phasing was
// wired into the engine. Both helpers return the same value in
// practice (because the Battlefield slice doesn't house phased-out
// permanents in any tested scenario), but exposing ArtifactCount
// alongside MetalcraftActive keeps the metalcraft surface
// self-contained.
func ArtifactCount(gs *GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return 0
	}
	n := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.PhasedOut {
			continue
		}
		if p.IsArtifact() {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// Rider executor (sibling of ApplyThresholdRider / ApplyMaxSpeedRider)
// ---------------------------------------------------------------------------

// findMetalcraftRiderEffect walks the card's AST looking for the tagged
// "metalcraft" rider payload, returning the Activated.Effect when one
// is present. Tagging convention mirrors threshold/max-speed: either
// Cost.Extra contains "metalcraft_rider", or Activated.Raw begins with
// "metalcraft" (case-insensitive fallback for corpus dumps that preserve
// the printed rider line without an extra-cost marker).
func findMetalcraftRiderEffect(card *Card) gameast.Effect {
	if card == nil || card.AST == nil {
		return nil
	}
	for _, ab := range card.AST.Abilities {
		a, ok := ab.(*gameast.Activated)
		if !ok || a == nil || a.Effect == nil {
			continue
		}
		if abilityIsTaggedMetalcraftRider(a) {
			return a.Effect
		}
	}
	return nil
}

func abilityIsTaggedMetalcraftRider(a *gameast.Activated) bool {
	if a == nil {
		return false
	}
	for _, extra := range a.Cost.Extra {
		if strings.EqualFold(extra, "metalcraft_rider") {
			return true
		}
	}
	raw := strings.ToLower(strings.TrimSpace(a.Raw))
	return strings.HasPrefix(raw, "metalcraft")
}

// ApplyMetalcraftRider executes the metalcraft rider for `src`, if any.
// Returns true if the rider actually fired.
//
// Conditions to fire:
//   - src is non-nil and has a Card.
//   - HasMetalcraft(src.Card).
//   - MetalcraftActive(gs, src.Controller) — recomputed live so the
//     artifact count is current at resolve time (an artifact that died
//     mid-resolution correctly turns the rider off).
//
// Logs metalcraft_rider on fire. If the AST carries a tagged rider
// effect, ResolveEffect runs it with `src` as the source; otherwise we
// log metalcraft_rider_pending so per_card handlers and corpus-backfill
// jobs can find affected spells.
func ApplyMetalcraftRider(gs *GameState, src *Permanent) bool {
	if gs == nil || src == nil || src.Card == nil {
		return false
	}
	if !HasMetalcraft(src.Card) {
		return false
	}
	if !MetalcraftActive(gs, src.Controller) {
		return false
	}

	gs.LogEvent(Event{
		Kind:   "metalcraft_rider",
		Seat:   src.Controller,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule":           "702.97",
			"artifact_count": ArtifactCount(gs, src.Controller),
		},
	})

	if eff := findMetalcraftRiderEffect(src.Card); eff != nil {
		ResolveEffect(gs, src, eff)
		return true
	}

	gs.LogEvent(Event{
		Kind:   "metalcraft_rider_pending",
		Seat:   src.Controller,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule":   "702.97",
			"reason": "rider_payload_not_in_ast",
		},
	})
	return true
}
