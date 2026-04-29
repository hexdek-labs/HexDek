package gameengine

// Storm keyword (CR §702.40).
//
//   702.40a  "Storm is a triggered ability. 'Storm' means 'When you cast
//            this spell, copy it for each other spell cast before it this
//            turn. You may choose new targets for the copies.'"
//   706.10   A copy of a spell is NOT cast — it doesn't trigger cast-based
//            abilities, doesn't trigger Storm again, doesn't pay costs, and
//            doesn't go through §601.
//
// Implementation: when CastSpell pushes a storm-bearing stack item, call
// ApplyStormCopies. It pushes (gs.SpellsCastThisTurn - 1) copies onto the
// stack above the original. LIFO resolution means copies resolve first —
// which also matches §405.2 (triggered abilities go on the stack above the
// spell that triggered them).
//
// Name-based lookup table mirrors scripts/playloop.py :: _STORM_CARDS. We
// keep a fixed set because the parser's Keyword emission for "storm" is
// uniform but some cards (e.g. Tendrils of Agony, Brain Freeze) have
// post-storm reminder-text paragraphs that can stress-test the parser in
// ways orthogonal to this work.

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameast"
)

// stormCards is the canonical list of printed Storm-bearing instants and
// sorceries. Membership is the primary check for whether a spell's cast
// fires the Storm trigger; a secondary AST walk below handles any card
// whose oracle text carries a Keyword(name="storm") node but isn't in this
// list (rare/new-set cards).
var stormCards = map[string]bool{
	"Grapeshot":         true,
	"Tendrils of Agony": true,
	"Brain Freeze":      true,
	"Mind's Desire":     true,
	"Haze of Rage":      true,
	"Inner Fire":        true,
	"Flusterstorm":      true,
	"Wing Shards":       true,
	"Volcanic Awakening": true,
	"Empty the Warrens": true,
	"Maelstrom Nexus":   true,
}

// HasStormKeyword returns true if the card carries Storm (CR §702.40).
// Checks the explicit name set first (fast + robust), falls back to AST
// keyword walk for off-list cards.
func HasStormKeyword(card *Card) bool {
	if card == nil {
		return false
	}
	if stormCards[card.DisplayName()] {
		return true
	}
	if card.AST == nil {
		return false
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if kw.Name == "" {
			continue
		}
		if equalFoldSimple(kw.Name, "storm") {
			return true
		}
	}
	return false
}

// equalFoldSimple is an ASCII-only case-insensitive comparison suitable for
// MTG keyword names (all ASCII).
func equalFoldSimple(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// ApplyStormCopies puts (gs.SpellsCastThisTurn - 1) copies of `original`
// onto the stack above it, matching CR §702.40a.
//
// Contract:
//   - Call AFTER IncrementCastCount (so gs.SpellsCastThisTurn already
//     includes the storm spell itself).
//   - Call BEFORE the priority round (copies must land on the stack
//     before any response window opens).
//   - `original` must be the StackItem pushed for the storm-bearing cast.
//     We don't re-verify storm here — the caller already gated on
//     HasStormKeyword.
//   - Returns the number of copies made (for logging + tests).
//   - Copies do NOT trigger observers (§706.10) and do NOT call
//     IncrementCastCount.
//
// Each copy is a fresh StackItem pointing at a fresh Card whose CMC is 0
// (free to resolve), whose Name is suffixed with "(storm copy N)" so logs
// distinguish them, and whose AST is shared with the original.
func ApplyStormCopies(gs *GameState, original *StackItem, controller int) int {
	if gs == nil || original == nil || original.Card == nil {
		return 0
	}
	if gs.SpellsCastThisTurn <= 1 {
		// The storm spell itself is the first (or only) cast this turn —
		// no prior casts, no copies.
		return 0
	}
	copies := gs.SpellsCastThisTurn - 1
	gs.LogEvent(Event{
		Kind:   "storm_trigger",
		Seat:   controller,
		Source: original.Card.DisplayName(),
		Amount: copies,
		Details: map[string]interface{}{
			"spells_cast_this_turn": gs.SpellsCastThisTurn,
			"copies":                copies,
			"rule":                  "702.40a",
		},
	})
	for i := 0; i < copies; i++ {
		baseName := original.Card.DisplayName()
		copyCard := &Card{
			// Share the AST — safe because CardAST is immutable.
			AST:           original.Card.AST,
			Name:          fmt.Sprintf("%s (storm copy %d)", baseName, i+1),
			Owner:         original.Card.Owner,
			BasePower:     original.Card.BasePower,
			BaseToughness: original.Card.BaseToughness,
			Types:         append([]string(nil), original.Card.Types...),
			Colors:        append([]string(nil), original.Card.Colors...),
			CMC:           0, // copies cost nothing
			TypeLine:      original.Card.TypeLine,
		}
		copyItem := &StackItem{
			Controller: controller,
			Card:       copyCard,
			Effect:     original.Effect,
			// Copies inherit the original's targets; real rules let the
			// controller pick new targets. MVP: reuse. A future "pick new
			// targets" policy hook lives on top of this without changing
			// the storm primitive.
			Targets: append([]Target(nil), original.Targets...),
			IsCopy:  true, // CR §706.10 — ceases to exist on resolution
		}
		// Push directly (bypass PushStackItem's "cast" logging — copies
		// aren't cast).
		copyItem.ID = nextStackID(gs)
		gs.Stack = append(gs.Stack, copyItem)
		gs.LogEvent(Event{
			Kind:   "stack_push_storm_copy",
			Seat:   controller,
			Source: copyCard.Name,
			Details: map[string]interface{}{
				"stack_id":   copyItem.ID,
				"stack_size": len(gs.Stack),
				"rule":       "702.40a+706.10",
			},
		})
	}
	return copies
}
