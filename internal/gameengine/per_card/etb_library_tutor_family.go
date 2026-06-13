package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// etb_library_tutor_family.go — generic handler for the
// "When this creature enters, [you may] search your library for a
// <filter> card, reveal it, put it into your hand, then shuffle." family.
//
// Members covered (all share the same algorithm):
//
//   - Trophy Mage         (artifact card with mana value 3)
//   - Treasure Mage       (artifact card with mana value 6 or greater)
//   - Trinket Mage        (artifact card with mana value 1 or less)
//   - Stoneforge Mystic   (Equipment card)
//   - Heliod's Pilgrim    (Aura card)
//   - Spellseeker         (instant or sorcery card with mana value 2 or less)
//   - Imperial Recruiter  (creature card with power 2 or less)
//   - Fierce Empath       (creature card with mana value 6 or greater)
//   - Thalia's Lancers    (legendary card)
//
// Notes on the "you may" rider — every member of this family except
// Imperial Recruiter is "you may search..." (optional). The AI policy
// is always to opt in: a hand-filtered tutor is monotone upside, the
// shuffle cost is irrelevant under our model, and Hat's lategame
// evaluator scores the resulting hand strictly better than passing.
// Imperial Recruiter and a handful of "search your library" (no "you
// may") cards are mandatory; the algorithm runs the same way.
//
// Cards intentionally NOT in this family:
//
//   - Rune-Scarred Demon — already has a dedicated handler that picks
//     the highest-CMC nonland (the unfiltered "any card" case wants a
//     smarter chooser than this generic filter loop).
//   - Vibrance — gated on "if {G}{G} was spent", with an extra "you
//     gain 2 life" rider; covered by evoke_color_gate_family.
//   - Hand-rolled siblings (Tiamat, Yasharn, Cloud, etc.) keep their
//     bespoke implementations.
//
// Filter algebra:
//
//   - typeRequired: "artifact" / "creature" / "instant_or_sorcery" /
//     "" (any). Matched against Card.Types via cardHasType (or
//     either-or for instant_or_sorcery).
//   - subtypeRequired: "equipment" / "aura" / "" (any). Matched via
//     cardHasSubtype against the card's subtypes.
//   - legendaryRequired: if true, card must have "legendary" in Types.
//   - cmcOp + cmcN: "==" / "<=" / ">=" / "" (no CMC gate). Compared
//     against Card.CMC (set by corpus loader from Scryfall mana_value).
//   - powerOp + powerN: same flavors against Card.BasePower (set by
//     deckparser from Scryfall's printed power).
//
// Picking strategy: walk the library in order, return the FIRST card
// that matches. Hat doesn't have per-card preference scoring inside
// the library yet (that's a Freya/Yggdrasil cross-cut), so first-match
// is the deterministic, reproducible choice. Engine tests rely on
// determinism.

type tutorFilterCmpOp int

const (
	tutorCmpNone tutorFilterCmpOp = iota
	tutorCmpEq
	tutorCmpLE
	tutorCmpGE
)

type tutorFilter struct {
	typeRequired      string // "artifact" / "creature" / "instant_or_sorcery" / ""
	subtypeRequired   string // "equipment" / "aura" / ""
	legendaryRequired bool

	cmcOp tutorFilterCmpOp
	cmcN  int

	powerOp tutorFilterCmpOp
	powerN  int
}

type etbLibraryTutorEntry struct {
	cardName string
	filter   tutorFilter
}

var etbLibraryTutorEntries = []etbLibraryTutorEntry{
	{
		cardName: "Trophy Mage",
		filter:   tutorFilter{typeRequired: "artifact", cmcOp: tutorCmpEq, cmcN: 3},
	},
	{
		cardName: "Treasure Mage",
		filter:   tutorFilter{typeRequired: "artifact", cmcOp: tutorCmpGE, cmcN: 6},
	},
	{
		cardName: "Trinket Mage",
		filter:   tutorFilter{typeRequired: "artifact", cmcOp: tutorCmpLE, cmcN: 1},
	},
	{
		cardName: "Stoneforge Mystic",
		filter:   tutorFilter{typeRequired: "artifact", subtypeRequired: "equipment"},
	},
	{
		cardName: "Heliod's Pilgrim",
		filter:   tutorFilter{typeRequired: "enchantment", subtypeRequired: "aura"},
	},
	{
		cardName: "Spellseeker",
		filter:   tutorFilter{typeRequired: "instant_or_sorcery", cmcOp: tutorCmpLE, cmcN: 2},
	},
	{
		cardName: "Imperial Recruiter",
		filter:   tutorFilter{typeRequired: "creature", powerOp: tutorCmpLE, powerN: 2},
	},
	{
		cardName: "Fierce Empath",
		filter:   tutorFilter{typeRequired: "creature", cmcOp: tutorCmpGE, cmcN: 6},
	},
	{
		cardName: "Thalia's Lancers",
		filter:   tutorFilter{legendaryRequired: true},
	},
}

func registerEtbLibraryTutorFamily(r *Registry) {
	for _, e := range etbLibraryTutorEntries {
		e := e
		r.OnETB(e.cardName, func(gs *gameengine.GameState, perm *gameengine.Permanent) {
			runEtbLibraryTutor(gs, perm, e)
		})
		// FULLY implements the printed self-ETB tutor — declare ownership so
		// the engine's generic AST push is suppressed (Judge r63 double-fire
		// gate). Without it the AST "search your library …" trigger ALSO
		// resolves, tutoring twice in real games.
		r.OwnsETBTrigger(e.cardName)
	}
}

func runEtbLibraryTutor(gs *gameengine.GameState, perm *gameengine.Permanent, e etbLibraryTutorEntry) {
	slug := "etb_library_tutor_family:" + landFetchSlug(e.cardName)
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	tutored := tutorPickFromLibrary(s.Library, e.filter)
	if tutored == nil {
		// Even on whiff we still shuffle (CR §701.18b — searching a
		// hidden zone shuffles regardless of whether the search found
		// anything).
		shuffleLibraryPerCard(gs, seat)
		emitFail(gs, slug, perm.Card.DisplayName(), "no_matching_card_in_library", map[string]interface{}{
			"seat": seat,
		})
		return
	}

	gameengine.MoveCard(gs, tutored, seat, "library", "hand", slug)
	shuffleLibraryPerCard(gs, seat)

	gs.LogEvent(gameengine.Event{
		Kind:   "search_library",
		Seat:   seat,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"found":  []string{tutored.DisplayName()},
			"reason": slug,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        seat,
		"tutored":     tutored.DisplayName(),
		"tutored_cmc": tutored.CMC,
	})
}

// tutorPickFromLibrary returns the first card matching the filter, or
// nil if none qualifies.
func tutorPickFromLibrary(library []*gameengine.Card, f tutorFilter) *gameengine.Card {
	for _, c := range library {
		if c == nil {
			continue
		}
		if !tutorFilterMatches(c, f) {
			continue
		}
		return c
	}
	return nil
}

func tutorFilterMatches(c *gameengine.Card, f tutorFilter) bool {
	switch f.typeRequired {
	case "":
		// any type ok
	case "instant_or_sorcery":
		if !cardHasType(c, "instant") && !cardHasType(c, "sorcery") {
			return false
		}
	default:
		if !cardHasType(c, f.typeRequired) {
			return false
		}
	}
	if f.subtypeRequired != "" && !cardHasSubtype(c, f.subtypeRequired) {
		return false
	}
	if f.legendaryRequired && !cardHasType(c, "legendary") {
		return false
	}
	if !tutorCmpInt(c.CMC, f.cmcOp, f.cmcN) {
		return false
	}
	if !tutorCmpInt(c.BasePower, f.powerOp, f.powerN) {
		return false
	}
	return true
}

func tutorCmpInt(have int, op tutorFilterCmpOp, want int) bool {
	switch op {
	case tutorCmpNone:
		return true
	case tutorCmpEq:
		return have == want
	case tutorCmpLE:
		return have <= want
	case tutorCmpGE:
		return have >= want
	}
	return true
}
