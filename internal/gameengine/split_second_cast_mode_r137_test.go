package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r137 — a conditional split second must actually lock the table.
//
// Molten Disaster, live in 4 decks, printed in 2007: "Kicker {R} / If this
// spell was kicked, it has split second." Its split second is NOT a
// Keyword node. The parser emits exactly this, verified against the live
// ast_dataset.jsonl on 2026-10-16:
//
//	Static{Modification{Kind: "if_intervening_tail", Args: [
//	    Modification{Kind: "cast_mode_check", Args: ["kicked"]},
//	    "it has split second"]}}
//
// cardHasSplitSecond walks for *gameast.Keyword only, so a KICKED Molten
// Disaster left everyone free to respond — the player paid the kicker and
// got nothing for it. CR §702.61a.
//
// The wider measurement behind this: 152 corpus cards carry a
// cast_mode_check and the engine had no handler for the node kind at all.

// r137Disaster builds the real Molten Disaster shape. kicked controls
// only whether the CAST stamped the flag, never the card text.
func r137Disaster(kicked interface{}) *StackItem {
	card := &Card{
		Name: "Molten Disaster",
		AST: &gameast.CardAST{
			Name: "Molten Disaster",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "kicker", Args: []any{"{r}"}},
				&gameast.Static{Modification: &gameast.Modification{
					ModKind: "if_intervening_tail",
					Args: []any{
						&gameast.Modification{ModKind: "cast_mode_check", Args: []any{"kicked"}},
						"it has split second",
					},
				}},
			},
		},
	}
	item := &StackItem{Kind: "spell", Card: card, CostMeta: map[string]interface{}{}}
	if kicked != nil {
		item.CostMeta["kicked"] = kicked
	}
	return item
}

func r137Stack(items ...*StackItem) *GameState {
	return &GameState{Stack: items}
}

// CATCHES THE BUG.
func TestR137_KickedMoltenDisasterLocksTheTable(t *testing.T) {
	if !SplitSecondActive(r137Stack(r137Disaster(true))) {
		t.Fatal("a KICKED Molten Disaster has split second (CR §702.61a) and must " +
			"stop opponents casting; the player paid {R} for exactly this")
	}
	// Multikicker-style count stamps carry the same meaning.
	if !SplitSecondActive(r137Stack(r137Disaster(2))) {
		t.Error("a count stamp of 2 is kicked")
	}
}

// MUST NEVER REGRESS: unkicked is a normal spell. Granting split second
// here would lock three opponents out of responding for free, which is
// the worse error of the two.
func TestR137_UnkickedMoltenDisasterDoesNotLockTheTable(t *testing.T) {
	if SplitSecondActive(r137Stack(r137Disaster(false))) {
		t.Fatal("an UNKICKED Molten Disaster has no split second")
	}
}

// MUST NEVER REGRESS: an unanswerable condition grants nothing. The flag
// absent means the cast path never considered kicker, not that it was
// kicked.
func TestR137_UnknownCastModeGrantsNothing(t *testing.T) {
	if SplitSecondActive(r137Stack(r137Disaster(nil))) {
		t.Fatal("no kicked stamp at all is UNKNOWN, and unknown must not grant " +
			"split second to the whole table")
	}
	// A mode with no tracked state is unknown rather than false.
	item := r137Disaster(true)
	if _, known := castModeSatisfied(item, "five_or_more_mana_spent"); known {
		t.Error("five_or_more_mana_spent has no tracked state and must report unknown")
	}
	if _, known := castModeSatisfied(nil, "kicked"); known {
		t.Error("a nil item cannot answer anything")
	}
}

// MUST NEVER REGRESS: printed split second keeps working through the
// original Keyword arm (Krosan Grip, 55 decks; Quagnoth; Angel's Grace).
func TestR137_PrintedSplitSecondStillWorks(t *testing.T) {
	card := &Card{Name: "Krosan Grip", AST: &gameast.CardAST{
		Name:      "Krosan Grip",
		Abilities: []gameast.Ability{&gameast.Keyword{Name: "split_second"}},
	}}
	if !SplitSecondActive(r137Stack(&StackItem{Kind: "spell", Card: card})) {
		t.Fatal("printed split second must still lock the table")
	}
	if !cardHasSplitSecond(card) {
		t.Error("the printed-keyword arm is unchanged")
	}
}

// MUST NEVER REGRESS: an ordinary spell locks nothing, and a countered
// split-second spell stops applying (§702.61a is "while on the stack").
func TestR137_OrdinaryAndCounteredSpellsLockNothing(t *testing.T) {
	plain := &StackItem{Kind: "spell", Card: &Card{Name: "Shock", AST: &gameast.CardAST{Name: "Shock"}}}
	if SplitSecondActive(r137Stack(plain)) {
		t.Fatal("an ordinary spell has no split second")
	}
	countered := r137Disaster(true)
	countered.Countered = true
	if SplitSecondActive(r137Stack(countered)) {
		t.Fatal("a countered spell must not keep locking the table")
	}
	if SplitSecondActive(r137Stack()) || SplitSecondActive(nil) {
		t.Error("an empty or nil stack locks nothing")
	}
}

// The cast-zone modes are answerable from the StackItem and must be.
func TestR137_CastZoneModesAreAnswerable(t *testing.T) {
	cases := []struct{ zone, mode string; want bool }{
		{"graveyard", "from_graveyard", true},
		{"hand", "from_graveyard", false},
		{"exile", "from_exile", true},
		{"", "from_hand", true}, // "" is the documented default for a hand cast
		{"graveyard", "from_hand", false},
	}
	for _, c := range cases {
		item := &StackItem{CastZone: c.zone, CostMeta: map[string]interface{}{}}
		got, known := castModeSatisfied(item, c.mode)
		if !known {
			t.Errorf("zone %q mode %q: must be answerable from CastZone", c.zone, c.mode)
			continue
		}
		if got != c.want {
			t.Errorf("zone %q mode %q: got %v want %v", c.zone, c.mode, got, c.want)
		}
	}
}
