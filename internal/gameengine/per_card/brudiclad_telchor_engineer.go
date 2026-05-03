package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBrudicladTelchorEngineer wires Brudiclad, Telchor Engineer.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	Creature tokens you control have haste.
//	At the beginning of combat on your turn, create a 1/1 blue Phyrexian
//	Myr artifact creature token. Then you may choose a token you control.
//	If you do, each other token you control becomes a copy of that token.
//
// Implementation:
//   - "Creature tokens you control have haste" is a continuous grant-keyword
//     effect (CR §613). The engine's AST/layers pipeline handles blanket
//     keyword grants; we emit a partial so the coverage gap is tracked.
//   - OnTrigger("combat_begin"): fires at the beginning of combat on
//     Brudiclad's controller's active turn. Steps:
//     1. Create one 1/1 blue Phyrexian Myr artifact creature token via
//        gameengine.CreateCreatureToken.
//     2. From ALL tokens the controller now controls (including the
//        freshly-created Myr), pick the "best" one by combined power +
//        toughness (tiebreak: earliest Timestamp).
//     3. For each OTHER token the controller controls, deep-copy the
//        chosen token's Card so Name / Types / BasePower / BaseToughness /
//        Colors / TypeLine all match (CR §706.10: token copies are not
//        card objects in any zone, but their characteristics mirror the
//        copied permanent). Modifications and counters are not copied —
//        the existing token's Modifications/Counters/Flags are wiped to
//        avoid double-counting ongoing effects (comparable to how Saheeli
//        and Mirage Mirror handle temporary copies in the engine).
//     4. emitPartial for the blanket haste clause (handled by layers
//        pipeline, not per-card enforcement).
//
// emitPartial: haste-grant is a continuous blanket keyword effect; the
// engine's layer-6 stack handles it via the card's AST — per-card enforcement
// is not needed, but the gap is flagged for Heimdall/Muninn tracking.
func registerBrudicladTelchorEngineer(r *Registry) {
	r.OnTrigger("Brudiclad, Telchor Engineer", "combat_begin", brudicladCombatBegin)
}

func brudicladCombatBegin(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "brudiclad_telchor_engineer_combat"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	// Gate: only fires on Brudiclad's controller's turn.
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}

	// De-dupe: extra-combat phases would otherwise double-fire on the
	// same turn. Coalesce on (turn, combat_instance).
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	dedupeKey := "brudiclad_combat_" + strconv.Itoa(gs.Turn)
	if perm.Flags[dedupeKey] > 0 {
		return
	}
	perm.Flags[dedupeKey] = 1

	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	// Step 1: Create a 1/1 blue Phyrexian Myr artifact creature token.
	// Types must include "token", "artifact", "creature", and the subtypes
	// "phyrexian" + "myr" so IsToken(), IsArtifact(), IsCreature() all pass.
	myrToken := gameengine.CreateCreatureToken(gs, seat,
		"Phyrexian Myr Token",
		[]string{"artifact", "creature", "phyrexian", "myr"},
		1, 1,
	)
	// Tag the Myr blue so color-matters effects see it correctly.
	if myrToken != nil && myrToken.Card != nil {
		myrToken.Card.Colors = []string{"U"}
		myrToken.Card.TypeLine = "Token Artifact Creature — Phyrexian Myr"
	}

	emitPartial(gs, slug, perm.Card.DisplayName(),
		"haste_grant_to_creature_tokens_is_continuous_effect_handled_by_layers")

	// Step 2: Choose the "best" token — highest (power + toughness),
	// tiebreak by lowest Timestamp (most-established on the battlefield,
	// yielding consistent copy targets across engine frames).
	chosen := brudicladPickBestToken(gs, seat)
	if chosen == nil || chosen.Card == nil {
		// No tokens at all (shouldn't happen — we just created the Myr —
		// but be defensive). Still emit so the token creation is recorded.
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":          seat,
			"myr_created":   true,
			"tokens_copied": 0,
			"reason":        "no_token_chosen",
		})
		return
	}

	// Step 3: For each OTHER token the controller controls, make it a
	// copy of the chosen token by swapping out its Card for a deep copy of
	// the chosen token's Card. Modifications and counters are cleared so
	// ongoing effects from the previous identity do not bleed into the new
	// copy (mirrors how the engine handles temporary clone effects).
	// A snapshot of the battlefield is taken before mutation so we don't
	// iterate over a slice we are modifying.
	snapshot := append([]*gameengine.Permanent(nil), s.Battlefield...)
	copied := 0
	for _, p := range snapshot {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsToken() {
			continue
		}
		if p == chosen {
			continue
		}
		// CR §706.2: a copy of a permanent has the same copiable values
		// (name, mana cost, color, type, rules text, P/T) but does NOT
		// inherit counters or continuous effects that have been applied.
		newCard := chosen.Card.DeepCopy()
		newCard.Owner = seat
		// Ensure "token" tag is present — DeepCopy preserves Types slice
		// which already has it from chosen, but be explicit.
		if !hasType(newCard.Types, "token") {
			newCard.Types = append([]string{"token"}, newCard.Types...)
		}
		p.Card = newCard
		// Wipe instance state so old counters/flags don't misrepresent the
		// new identity.
		p.Counters = map[string]int{}
		p.Modifications = nil
		// SummoningSick follows the PERMANENT, not the copy's card — a
		// token that was already on the battlefield does not become
		// summoning-sick because its characteristics changed (CR §303.4f
		// analogy; continuous copy effect mid-turn preserves sick status).
		gs.InvalidateCharacteristicsCache()
		copied++
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"myr_created":   true,
		"chosen_token":  chosen.Card.DisplayName(),
		"tokens_copied": copied,
	})
}

// brudicladPickBestToken returns the token on the controller's battlefield
// with the highest combined (power + toughness). Ties are broken by lowest
// Timestamp (most-established permanent). Returns nil if no tokens exist.
func brudicladPickBestToken(gs *gameengine.GameState, controller int) *gameengine.Permanent {
	if gs == nil || controller < 0 || controller >= len(gs.Seats) {
		return nil
	}
	s := gs.Seats[controller]
	if s == nil {
		return nil
	}

	var best *gameengine.Permanent
	bestScore := -1 << 30
	bestTS := 1<<62 - 1

	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsToken() {
			continue
		}
		score := p.Power() + p.Toughness()
		if score > bestScore || (score == bestScore && p.Timestamp < bestTS) {
			bestScore = score
			bestTS = p.Timestamp
			best = p
		}
	}
	return best
}
