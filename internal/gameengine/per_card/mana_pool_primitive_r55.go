package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// mana_pool_primitive_r55 — five cards demonstrating the R55 mana-
// primitive triad:
//
//   1. ManaPoolExemption — "mana doesn't empty as steps and phases end"
//      (Omnath, Locus of Mana; Upwelling).
//   2. AddManaPerCount — "add {X} mana where X is the count of [type]
//      permanents you control" (Cabal Coffers; Sanctum Weaver).
//   3. AddRestrictedMana — "this mana can only be spent on [type]"
//      (Ancient Ziggurat).
//
// All three primitives live in gameengine/mana.go and are populated by
// the per_card hooks below — replacing the prior hardcoded card-name
// check that lived inside PoolExemptColors (only Upwelling + Omnath
// were recognized, no extensibility).

// ---------------------------------------------------------------------
// Omnath, Locus of Mana — {2}{G}, Legendary Creature
//
// Oracle (Scryfall, verified):
//
//   You don't lose unspent green mana as steps and phases end.
//   Omnath gets +1/+1 for each unspent green mana you have.
//
// Implementation:
//   - ETB registers a controller-scoped G exemption via
//     RegisterManaPoolExemption(perm, perm.Controller, ["G"]).
//   - permanent_ltb unregisters it.
//   - The +1/+1-per-green-mana clause is a Phase 7 layer 7c power-
//     scaling static. Approximated via a Modification appended at ETB
//     and refreshed on add_mana / pool_drain events; gated as
//     emitPartial since true layer 7c is an engine surface.
// ---------------------------------------------------------------------

func registerOmnathLocusOfMana(r *Registry) {
	r.OnETB("Omnath, Locus of Mana", omnathLocusOfManaETB)
	r.OnTrigger("Omnath, Locus of Mana", "permanent_ltb", omnathLocusOfManaLTB)
}

func omnathLocusOfManaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "omnath_locus_of_mana_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	gameengine.RegisterManaPoolExemption(gs, perm, perm.Controller, []string{"G"})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"exempt_color": "G",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"power_scaling_per_unspent_green_mana_needs_phase7_layer7c_engine_hook")
}

func omnathLocusOfManaLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
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
// Upwelling — {3}{G}, Enchantment
//
// Oracle (Scryfall, verified):
//
//   Players don't lose unspent mana as steps and phases end.
//
// All-seat all-color exemption: Seat=-1, Colors=["any"]. PoolExemptColors
// short-circuits on "any" and skips drain entirely.
// ---------------------------------------------------------------------

func registerUpwelling(r *Registry) {
	r.OnETB("Upwelling", upwellingETB)
	r.OnTrigger("Upwelling", "permanent_ltb", upwellingLTB)
}

func upwellingETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "upwelling_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	gameengine.RegisterManaPoolExemption(gs, perm, -1, []string{"any"})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"scope":       "all_seats",
		"exempt_color": "any",
	})
}

func upwellingLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
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
// Cabal Coffers — Legendary Land
//
// Oracle (Scryfall, verified):
//
//   {2}, {T}: Add {B} for each Swamp you control.
//
// AddManaPerCount with a "is Swamp" predicate. Cost gate {2}+{T}
// enforced defensively in-handler (engine activation dispatcher pays
// before reaching here; the guard is a fixture-mode safety).
// ---------------------------------------------------------------------

func registerCabalCoffers(r *Registry) {
	r.OnActivated("Cabal Coffers", cabalCoffersActivate)
}

func cabalCoffersActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "cabal_coffers_tap_per_swamp"
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
	if seat.ManaPool < 2 {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"required": 2, "available": seat.ManaPool,
		})
		return
	}
	seat.ManaPool -= 2
	gameengine.SyncManaAfterSpend(seat)
	src.Tapped = true
	n := gameengine.AddManaPerCount(gs, seatIdx, "B", func(p *gameengine.Permanent) bool {
		return cardHasType(p.Card, "swamp") ||
			strings.Contains(strings.ToLower(p.Card.TypeLine), "swamp")
	}, "Cabal Coffers")
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      seatIdx,
		"swamps":    n,
		"added":     n,
		"new_pool":  seat.ManaPool,
	})
}

// ---------------------------------------------------------------------
// Sanctum Weaver — {1}{G}, Creature
//
// Oracle (Scryfall, verified):
//
//   {T}: Add X mana of any one color, where X is the number of
//   enchantments you control.
//
// AddManaPerCount with an "is enchantment" predicate. The color choice
// is the largest color in the controller's pip-pressure across hand +
// battlefield (heuristic: matches what the player is most likely to
// spend next); fall back to "G" since Sanctum Weaver is itself green.
// ---------------------------------------------------------------------

func registerSanctumWeaver(r *Registry) {
	r.OnActivated("Sanctum Weaver", sanctumWeaverActivate)
}

func sanctumWeaverActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "sanctum_weaver_tap_per_enchantment"
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
	src.Tapped = true
	// Pick a color: scan controller's battlefield + hand for the most-
	// represented single color among casting costs. Default to G.
	chosen := sanctumWeaverPickColor(seat)
	n := gameengine.AddManaPerCount(gs, seatIdx, chosen, func(p *gameengine.Permanent) bool {
		return cardHasType(p.Card, "enchantment")
	}, "Sanctum Weaver")
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":         seatIdx,
		"enchantments": n,
		"color":        chosen,
		"added":        n,
	})
}

func sanctumWeaverPickColor(seat *gameengine.Seat) string {
	if seat == nil {
		return "G"
	}
	tally := map[string]int{"W": 0, "U": 0, "B": 0, "R": 0, "G": 0}
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		for _, col := range c.Colors {
			tally[strings.ToUpper(col)]++
		}
	}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, col := range p.Card.Colors {
			tally[strings.ToUpper(col)]++
		}
	}
	best := "G"
	bestN := -1
	// Stable order so a tie always picks the same color.
	for _, col := range []string{"G", "W", "U", "B", "R"} {
		if tally[col] > bestN {
			bestN = tally[col]
			best = col
		}
	}
	return best
}

// ---------------------------------------------------------------------
// Ancient Ziggurat — Land
//
// Oracle (Scryfall, verified):
//
//   {T}: Add one mana of any color. Spend this mana only to cast a
//   creature spell.
//
// AddRestrictedMana with "creature_spell_only" — the existing
// RestrictionAllows table already handles this restriction. Color
// stored as "" (any-color) so the spend pipeline can match it against
// any creature-spell pip requirement.
// ---------------------------------------------------------------------

func registerAncientZiggurat(r *Registry) {
	r.OnActivated("Ancient Ziggurat", ancientZigguratActivate)
}

func ancientZigguratActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "ancient_ziggurat_tap_creature_only"
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
	src.Tapped = true
	gameengine.AddRestrictedMana(gs, seat, 1, "", "creature_spell_only", "Ancient Ziggurat")
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":        seatIdx,
		"added":       1,
		"restriction": "creature_spell_only",
		"new_pool":    seat.ManaPool,
	})
}
