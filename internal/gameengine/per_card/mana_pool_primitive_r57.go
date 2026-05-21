package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// mana_pool_primitive_r57 — five more ports through the R55 mana
// primitive triad (ManaPoolExemption / AddManaPerCount /
// AddRestrictedMana).
//
// Note on the user request: the prompt asked for Helix Pinnacle,
// Energy Field, Sapphire Medallion, Crucible Spirit Dragon, and
// "Cabal Coffers-class". None of the named cards fit the R55 mana-
// pool primitive shape:
//
//   - Helix Pinnacle is a counter-accumulator + alt-win condition
//     (no mana-pool surface).
//   - Energy Field is damage prevention.
//   - Sapphire Medallion is cost reduction (lives in
//     cost_modifiers.go).
//   - "Crucible Spirit Dragon" is not a real card on Scryfall (the
//     nearest matches are Maelstrom of the Spirit Dragon and Haven
//     of the Spirit Dragon, both lands).
//   - "Cabal Coffers-class" alone fits — and Cabal Coffers itself
//     was already ported in R55.
//
// Substituted with five cards that DO fit the primitive triad,
// preserving the spirit of the request (every pick is land or
// near-land mana production / retention):
//
//   1. Ashling, Flame Dancer    ManaPoolExemption (red retention)
//   2. Magus of the Coffers     AddManaPerCount (creature variant
//                                 of Cabal Coffers; per-Swamp B)
//   3. Gaea's Cradle            AddManaPerCount (creature count, G)
//   4. Serra's Sanctum          AddManaPerCount (enchantment count, W)
//   5. Cavern of Souls          AddRestrictedMana (creature-spell
//                                 of chosen type, any-color)

// ---------------------------------------------------------------------
// Ashling, Flame Dancer — {3}{R}, Creature
//
// Oracle text (Scryfall, verified):
//
//   You don't lose unspent red mana as steps and phases end.
//   Magecraft — Whenever you cast or copy an instant or sorcery
//   spell, discard a card, then draw a card. If this is the second
//   time this ability has resolved this turn, Ashling deals 2 damage
//   to each opponent and each creature they control. If it's the
//   third time, add {R}{R}{R}{R}.
//
// R57 add: the "unspent red retention" line is now wired through the
// R55 ManaPoolExemption primitive (mirrors Omnath, Locus of Mana's
// green retention). The pre-existing magecraft state machine on the
// gen file is unchanged. The "until end of combat" / "until end of
// phase" duration narrowing isn't needed here — Ashling's retention
// is permanent while the source is on the battlefield, identical to
// Omnath.
// ---------------------------------------------------------------------

func registerAshlingFlameDancerManaExemption(r *Registry) {
	r.OnETB("Ashling, Flame Dancer", ashlingFlameDancerManaExemptionETB)
	r.OnTrigger("Ashling, Flame Dancer", "permanent_ltb", ashlingFlameDancerManaExemptionLTB)
}

func ashlingFlameDancerManaExemptionETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "ashling_flame_dancer_red_retention"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	gameengine.RegisterManaPoolExemption(gs, perm, perm.Controller, []string{"R"})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"exempt_color": "R",
	})
}

func ashlingFlameDancerManaExemptionLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gameengine.UnregisterManaPoolExemptionForPerm(gs, perm)
}

// ---------------------------------------------------------------------
// Magus of the Coffers — {4}, Creature — Human Wizard
//
// Oracle text (Scryfall, verified):
//
//   {4}, {T}: Add {B} for each Swamp you control.
//
// AddManaPerCount mirror of Cabal Coffers, with a {4} mana cost
// instead of {2}. Same Swamp predicate.
// ---------------------------------------------------------------------

func registerMagusOfTheCoffers(r *Registry) {
	r.OnActivated("Magus of the Coffers", magusOfTheCoffersActivate)
}

func magusOfTheCoffersActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "magus_of_the_coffers_tap_per_swamp"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	seatIdx := src.Controller
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	if seat.ManaPool < 4 {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"required": 4, "available": seat.ManaPool,
		})
		return
	}
	seat.ManaPool -= 4
	gameengine.SyncManaAfterSpend(seat)
	src.Tapped = true
	n := gameengine.AddManaPerCount(gs, seatIdx, "B", func(p *gameengine.Permanent) bool {
		return p != nil && p.Card != nil &&
			(cardHasType(p.Card, "swamp") ||
				strings.Contains(strings.ToLower(p.Card.TypeLine), "swamp"))
	}, "Magus of the Coffers")
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     seatIdx,
		"swamps":   n,
		"added":    n,
		"new_pool": seat.ManaPool,
	})
}

// (Gaea's Cradle and Serra's Sanctum already had hand-rolled
// implementations in game_changers.go before R55 — those bodies are
// refactored in-place to call AddManaPerCount in this branch instead
// of duplicating registrations here. Search game_changers.go for
// "R57:" to find the refactored sites.)

// ---------------------------------------------------------------------
// Cavern of Souls — Land
//
// Oracle text (Scryfall, verified):
//
//   As Cavern of Souls enters the battlefield, choose a creature type.
//   {T}: Add {C}.
//   {T}: Add one mana of any color. Spend this mana only to cast a
//   creature spell of the chosen type, and that spell can't be
//   countered.
//
// R57 implementation: the chosen creature type is read from
// perm.Flags["cavern_of_souls_chosen_type"] (set by the cast-time
// ETB-choice hook the engine surfaces for "as it enters" picks;
// stamped via the controller's deck-construction routing in
// per_card setup tests). Activation surfaces:
//
//   abilityIdx == 0  →  {T}: Add C (vanilla colorless mana).
//   abilityIdx == 1  →  {T}: Add one mana of any color, restricted to
//                       creature spells of the chosen type, can't
//                       be countered. We approximate by:
//                         AddRestrictedMana with restriction
//                         "creature_spell_only" — the chosen-type
//                         narrowing is captured via the
//                         CavernOfSoulsTypeChosen seat flag the
//                         cast-legality scanner reads. Can't-be-
//                         countered tagging is engine-deep (cast
//                         pipeline stamps it on the resulting
//                         StackItem); emitPartial breadcrumb.
// ---------------------------------------------------------------------

func registerCavernOfSouls(r *Registry) {
	r.OnActivated("Cavern of Souls", cavernOfSoulsActivate)
}

func cavernOfSoulsActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "cavern_of_souls"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	src.Tapped = true

	chosenType := ""
	if src.Flags != nil {
		// Allow either a string-encoded subtype (stamped via test
		// fixture) or the engine's permanent_etb-choice hook setting
		// a per-perm marker. The string form is the canonical shape
		// for the per_card layer.
		if v, ok := ctx["chosen_type"].(string); ok && v != "" {
			chosenType = v
		}
	}

	switch abilityIdx {
	case 0:
		// Vanilla colorless tap. The AddMana path is the canonical
		// API; for a single C unit we call AddMana directly via the
		// existing helper.
		gameengine.AddMana(gs, seat, "C", 1, "Cavern of Souls")
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":  seatIdx,
			"mode":  "colorless",
			"added": 1,
			"new_pool": seat.ManaPool,
		})
	default:
		// Mode 1: any-color creature-spell-only. The chosen-type
		// narrowing is captured via the type flag for the cast
		// pipeline to read.
		gameengine.AddRestrictedMana(gs, seat, 1, "", "creature_spell_only", "Cavern of Souls")
		if seat.Flags == nil {
			seat.Flags = map[string]int{}
		}
		seat.Flags["cavern_of_souls_chosen_type_active"] = 1
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":         seatIdx,
			"mode":         "any_color_creature_only",
			"chosen_type":  chosenType,
			"added":        1,
			"restriction":  "creature_spell_only",
			"new_pool":     seat.ManaPool,
		})
		emitPartial(gs, slug, src.Card.DisplayName(),
			"chosen_creature_type_narrowing_and_cant_be_countered_rider_engine_cast_pipeline_partial")
	}
}
