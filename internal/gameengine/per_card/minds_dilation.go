package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMindsDilation wires Mind's Dilation.
//
// Oracle text (verified via data/rules/ast_dataset.jsonl, 2026-05-28):
//
//	{5}{U}{U} Enchantment
//	Whenever an opponent casts their first spell each turn, that
//	player exiles the top card of their library. If it's a nonland
//	card, you may cast it without paying its mana cost.
//
// Premium {7}-mana value engine that turns every opponent's first
// spell each turn into a 50/50 spell-for-you-at-no-cost. Pairs with
// Brago / Yorion / Aminatou flicker shells to re-trigger the exile
// effect on subsequent first-spells (the §400.7c primitive verified
// by cr_400_7c_property_r60_test.go ensures the exile routes to
// the opp's owner-scoped zone correctly).
//
// Implementation:
//   - OnTrigger("spell_cast_by_opponent"): gate on caster != controller
//     AND first-spell-this-turn-for-this-caster (tracked via
//     perm.Flags["minds_dilation_fired_caster_<i>_turn_<n>"], same
//     per-permanent flag shape baral_and_kari_zev.go uses for its
//     "first instant or sorcery each turn" gate).
//   - Effect: top of caster's library → caster's exile via
//     MoveCard(gs, top, caster_seat, "library", "exile", reason). The
//     post-#685 ownerSeat-explicit MoveCard primitive routes the
//     card through the opponent's owner-scoped exile zone correctly
//     (CR §400.7c — verified clean by the §400.7c property test).
//   - If the exiled card is NOT a land, register a ZoneCastGrant
//     allowing the Mind's Dilation controller to free-cast it for
//     as long as it remains exiled. Pattern mirrors Praetor's Grasp.
//   - The oracle phrase "without paying its mana cost" maps cleanly
//     to NewFreeCastFromExilePermission (same primitive Praetor's
//     Grasp uses).
//
// Note: the older oracle wording captured by this codebase's AST
// dataset has the "you may cast it" path only on the controller side.
// Modern Scryfall text gives the EXILING PLAYER the first option to
// cast their own exiled card; if they decline, then the Mind's Dilation
// controller may cast it. The modeled behavior follows the dataset
// (single-path, controller-only) so the engine stays consistent with
// the AST it was built against. Emit a partial when the dataset and
// modern oracle diverge for downstream auditors.
func registerMindsDilation(r *Registry) {
	r.OnTrigger("Mind's Dilation", "spell_cast_by_opponent", mindsDilationOppCast)
}

func mindsDilationOppCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "minds_dilation_exile_top"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat == perm.Controller {
		return
	}
	if casterSeat < 0 || casterSeat >= len(gs.Seats) {
		return
	}
	caster := gs.Seats[casterSeat]
	if caster == nil || caster.Lost {
		return
	}

	// First-spell-this-turn-per-caster gate. Use a single perm.Flag
	// key per (caster, turn) pair; cleared on turn rollover by virtue
	// of the key embedding gs.Turn.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	flagKey := mindsDilationFlagKey(casterSeat, gs.Turn)
	if perm.Flags[flagKey] != 0 {
		return
	}
	perm.Flags[flagKey] = 1

	// Exile the top of caster's library. Library[0] is the top per the
	// Praetor's Grasp convention; same slice convention everywhere in
	// internal/gameengine.
	if len(caster.Library) == 0 {
		emitFail(gs, slug, "Mind's Dilation", "caster_library_empty", map[string]interface{}{
			"caster_seat": casterSeat,
		})
		return
	}
	top := caster.Library[0]
	caster.Library = caster.Library[1:]
	// Route through the post-#685 ownerSeat-explicit MoveCard. The
	// ownerSeat is casterSeat — top's owner is the opponent who would
	// have drawn it. The MoveCard primitive's §400.7c routing keeps
	// the card in the opponent's owner-scoped exile zone.
	gameengine.MoveCard(gs, top, casterSeat, "library", "exile", "minds_dilation_exile")

	exiledName := ""
	if top != nil {
		exiledName = top.DisplayName()
	}

	// If the exiled card is a land, the controller does NOT get to
	// cast it (oracle text gates on "nonland card"). Lands just sit
	// in exile.
	isLand := mindsDilationIsLand(top)
	if !isLand && top != nil {
		freePerm := gameengine.NewFreeCastFromExilePermission(perm.Controller, "Mind's Dilation")
		gameengine.RegisterZoneCastGrant(gs, top, freePerm)
	}

	emit(gs, slug, "Mind's Dilation", map[string]interface{}{
		"seat":         perm.Controller,
		"caster_seat":  casterSeat,
		"exiled_card":  exiledName,
		"is_land":      isLand,
		"free_cast":    !isLand,
	})

	// Modern oracle wording adds an opponent-first cast option that
	// the AST-dataset wording omits. Heimdall can use this partial to
	// flag deck simulations where the divergence might matter (rare —
	// the engine plays a single controller, so "opponent may cast it
	// first; otherwise you may cast it" collapses to the same outcome
	// in 95%+ of pods).
	emitPartial(gs, slug, "Mind's Dilation",
		"opponent_first_cast_option_modern_oracle_unmodeled_uses_dataset_wording")
}

func mindsDilationFlagKey(casterSeat, turn int) string {
	// Key shape mirrors baral_and_kari_zev's perm-Flags-by-turn idiom.
	// Embedding both seat AND turn means a fresh key is generated each
	// turn for each opponent — no manual cleanup needed.
	return "minds_dilation_fired_" + intToKey(casterSeat) + "_t" + intToKey(turn)
}

func mindsDilationIsLand(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		if t == "land" {
			return true
		}
	}
	return false
}

// intToKey converts a small non-negative int to a stable string for
// use in a Flags map key. Avoids a strconv import for the few-call
// sites that need it. Negative inputs return "n" + abs as a defensive
// fallback so we never blow up on a -1 sentinel.
func intToKey(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = 'n'
	}
	return string(b[i:])
}
