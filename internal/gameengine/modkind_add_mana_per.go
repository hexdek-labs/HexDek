package gameengine

import (
	"regexp"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// modkind_add_mana_per.go — generic coverage for the `add_mana_per`
// scaffold ModKind: activated/triggered mana abilities that add mana
// scaled by a count, e.g.
//
//	{T}: Add {G} for each creature you control.   (Gaea's Cradle)
//	{T}: Add {B} for each Swamp you control.       (Cabal Coffers)
//	{T}: Add {W} for each enchantment you control. (Serra's Sanctum)
//	{T}: Add {U} for each artifact you control.    (Tolarian Academy)
//	{T}: Add {G} for each Forest you control.      (Rofellos)
//	{T}: Add {G} for each Elf on the battlefield.  (Priest of Titania)
//	{T}: Add {C} for each charge counter on this … (Everflowing Chalice)
//
// The AST carries args = [ "{<color>}", "<count basis text>" ]. The base
// `add_mana_effect` kind is handled but `add_mana_per` fell through to
// the switch default — every one of these premier ramp sources produced
// ZERO mana. (Mana abilities resolve inline via ResolveEffect →
// resolveModificationEffect per CR §605.3a, so this seam fires on
// activation.)
//
// The count basis is free text, so this handler parses only the
// high-confidence, unambiguous patterns and adds nothing for the rest
// (identical to the prior inert behavior — never an over-count):
//
//	"<type> you control"        → count matching permanents you control
//	"<type> on the battlefield"  → count matching permanents game-wide
//	"<word> counter on this …"   → count that counter type on the source
//	"counter on ~"               → total counters on the source
//
// Type matching reuses CountPermanentsByType, which returns 0 for any
// basis it can't resolve to real card types (e.g. "creature with power 4
// or greater") — so complex predicates degrade to the prior no-op rather
// than mis-counting. Deferred long-tail bases (storage-counter-removed-
// this-way costs, hand/graveyard/party/opponent counts) are logged with
// matched=false for follow-up.

var (
	reAddManaPerSymbol    = regexp.MustCompile(`\{([WUBRGCwubrgc])\}`)
	reAddManaPerCounter   = regexp.MustCompile(`^(?:([+0-9a-z/]+) )?counters? on (?:this|~|it)\b`)
	reAddManaPerGraveyard = regexp.MustCompile(`^([a-z]+) cards? in your graveyard$`)

	// reAddManaPerResidual matches the count-scaled mana payload that the
	// parser sometimes smuggles into an `untyped_effect` ModKind's args[0]
	// instead of emitting a clean `add_mana_per` node — the shape is
	// "add_{<color>}_per:<count basis>" (Elvish Archdruid: "Add {G} for
	// each Elf you control" → "add_{g}_per:elf you control"). Captures the
	// color symbol and the basis text so resolveAddManaPerResidual can
	// reuse the same count logic as the structured add_mana_per handler.
	reAddManaPerResidual = regexp.MustCompile(`(?i)^add_\{([wubrgc])\}_per:(.+)$`)
)

// resolveAddManaPerResidual handles the "add_{<color>}_per:<basis>" residual
// string (see reAddManaPerResidual). It computes the count from the basis and
// adds that many mana of the named color, reusing addManaPerCount so the basis
// vocabulary stays in lockstep with the structured add_mana_per handler.
// Returns true when the residual matched (handled), false otherwise so the
// caller can fall through to other residual patterns.
func resolveAddManaPerResidual(gs *GameState, src *Permanent, raw string) bool {
	if gs == nil || src == nil {
		return false
	}
	m := reAddManaPerResidual.FindStringSubmatch(raw)
	if m == nil {
		return false
	}
	seat := controllerSeat(src)
	if seat < 0 || seat >= len(gs.Seats) {
		return false
	}
	color := strings.ToUpper(m[1])
	basis := strings.ToLower(strings.TrimSpace(m[2]))
	count, matched := addManaPerCount(gs, src, seat, basis)
	if matched && count > 0 {
		AddMana(gs, gs.Seats[seat], color, count, sourceName(src))
	}
	gs.LogEvent(Event{
		Kind:   "add_mana_per",
		Seat:   seat,
		Source: sourceName(src),
		Amount: count,
		Details: map[string]interface{}{
			"color":    color,
			"basis":    basis,
			"matched":  matched,
			"residual": true,
		},
	})
	return true
}

// resolveAddManaPer handles one add_mana_per ModificationEffect. It is
// invoked from resolveModificationEffect's `add_mana_per` case.
func resolveAddManaPer(gs *GameState, src *Permanent, e *gameast.ModificationEffect) {
	if gs == nil || src == nil {
		return
	}
	seat := controllerSeat(src)
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	color := "C"
	if m := reAddManaPerSymbol.FindStringSubmatch(modArgString(e.Args, 0)); m != nil {
		color = strings.ToUpper(m[1])
	}
	basis := strings.ToLower(strings.TrimSpace(modArgString(e.Args, 1)))

	count, matched := addManaPerCount(gs, src, seat, basis)
	if matched && count > 0 {
		AddMana(gs, gs.Seats[seat], color, count, sourceName(src))
	}
	gs.LogEvent(Event{
		Kind:   "add_mana_per",
		Seat:   seat,
		Source: sourceName(src),
		Amount: count,
		Details: map[string]interface{}{
			"color":   color,
			"basis":   basis,
			"matched": matched,
		},
	})
}

// addManaPerCount computes the per-count multiplier for a basis string.
// The bool reports whether the basis matched a known pattern (used only
// for telemetry — an unmatched basis adds no mana, exactly as before).
func addManaPerCount(gs *GameState, src *Permanent, seat int, basis string) (int, bool) {
	// "<word> counter on this …" / "counter on ~" — counters on the source.
	if m := reAddManaPerCounter.FindStringSubmatch(basis); m != nil {
		if src.Counters == nil {
			return 0, true
		}
		ctype := m[1]
		if ctype == "" {
			total := 0
			for _, v := range src.Counters {
				total += v
			}
			return total, true
		}
		return src.Counters[ctype], true
	}

	// "<type> you control" — permanents of the type the seat controls.
	if t := strings.TrimSuffix(basis, " you control"); t != basis {
		return CountPermanentsByType(gs, seat, t), true
	}

	// "<type> on the battlefield" — permanents of the type game-wide.
	if t := strings.TrimSuffix(basis, " on the battlefield"); t != basis {
		return countPermanentsOfTypeGlobal(gs, t), true
	}

	// "card(s) in your hand" — caster's hand size.
	if basis == "card in your hand" || basis == "cards in your hand" {
		return len(gs.Seats[seat].Hand), true
	}

	// "card(s) in target opponent's hand" — AI targets the opponent
	// holding the most cards (maximizes the ramp).
	if basis == "card in target opponent's hand" || basis == "cards in target opponent's hand" {
		best := 0
		for i := range gs.Seats {
			if i == seat || gs.Seats[i] == nil {
				continue
			}
			if n := len(gs.Seats[i].Hand); n > best {
				best = n
			}
		}
		return best, true
	}

	// "<single-type> card(s) in your graveyard" — count matching cards in
	// the caster's graveyard. Single type word only (a multi-word filter
	// like "black creature" falls through to the deferred no-op rather
	// than over-counting on type alone).
	if m := reAddManaPerGraveyard.FindStringSubmatch(basis); m != nil {
		ctype := m[1]
		n := 0
		for _, c := range gs.Seats[seat].Graveyard {
			if c != nil && cardHasType(c, ctype) {
				n++
			}
		}
		return n, true
	}

	return 0, false
}

// countPermanentsOfTypeGlobal counts permanents of typeStr across every
// seat's battlefield, reusing CountPermanentsByType's matching so basis
// strings it can't resolve to real types contribute 0.
func countPermanentsOfTypeGlobal(gs *GameState, typeStr string) int {
	if gs == nil {
		return 0
	}
	total := 0
	for i := range gs.Seats {
		total += CountPermanentsByType(gs, i, typeStr)
	}
	return total
}
