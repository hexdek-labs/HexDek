package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// target_priority_signals_r60_test.go — pins the R60 round 13+
// removal-target penalties added to ChooseTarget.

// mkTargetCreature builds a creature permanent with given P/T, controller,
// keywords, and oracle text. AST.Abilities holds a single Triggered or
// Keyword entry so OracleTextLower and HasKeyword both see the right
// data.
func mkTargetCreature(gs *gameengine.GameState, controller int, name string, power, toughness int, keywords []string, oracle string) *gameengine.Permanent {
	ast := &gameast.CardAST{Name: name}
	for _, kw := range keywords {
		ast.Abilities = append(ast.Abilities, &gameast.Keyword{Name: kw})
	}
	if oracle != "" {
		ast.Abilities = append(ast.Abilities, &gameast.Triggered{Raw: oracle})
	}
	card := &gameengine.Card{
		Name:          name,
		AST:           ast,
		Types:         []string{"creature"},
		BasePower:     power,
		BaseToughness: toughness,
	}
	p := &gameengine.Permanent{
		Card:       card,
		Controller: controller,
		Owner:      controller,
		Flags:      map[string]int{},
	}
	gs.Seats[controller].Battlefield = append(gs.Seats[controller].Battlefield, p)
	return p
}

// -----------------------------------------------------------------------
// removalTargetIndestructiblePenalty
// -----------------------------------------------------------------------

func TestRemovalTargetIndestructiblePenalty_NilIsZero(t *testing.T) {
	if got := removalTargetIndestructiblePenalty(nil); got != 0 {
		t.Errorf("nil permanent should return 0; got %.2f", got)
	}
}

func TestRemovalTargetIndestructiblePenalty_VanillaIsZero(t *testing.T) {
	gs := newTestGame(t, 2)
	p := mkTargetCreature(gs, 1, "Bear", 2, 2, nil, "")
	if got := removalTargetIndestructiblePenalty(p); got != 0 {
		t.Errorf("vanilla creature should return 0; got %.2f", got)
	}
}

func TestRemovalTargetIndestructiblePenalty_IndestructibleFires(t *testing.T) {
	gs := newTestGame(t, 2)
	p := mkTargetCreature(gs, 1, "Avacyn", 8, 8, []string{"indestructible"}, "")
	if got := removalTargetIndestructiblePenalty(p); got != -5.0 {
		t.Errorf("indestructible creature should return -5.0; got %.2f", got)
	}
}

// -----------------------------------------------------------------------
// removalTargetDeathPayoffPenalty
// -----------------------------------------------------------------------

func TestRemovalTargetDeathPayoffPenalty_NilIsZero(t *testing.T) {
	if got := removalTargetDeathPayoffPenalty(nil); got != 0 {
		t.Errorf("nil permanent should return 0; got %.2f", got)
	}
}

func TestRemovalTargetDeathPayoffPenalty_VanillaIsZero(t *testing.T) {
	gs := newTestGame(t, 2)
	p := mkTargetCreature(gs, 1, "Bear", 2, 2, nil, "")
	if got := removalTargetDeathPayoffPenalty(p); got != 0 {
		t.Errorf("vanilla creature should return 0; got %.2f", got)
	}
}

func TestRemovalTargetDeathPayoffPenalty_HangarbackFires(t *testing.T) {
	gs := newTestGame(t, 2)
	p := mkTargetCreature(gs, 1, "Hangarback Walker", 1, 1, nil,
		"when this creature dies, create x 1/1 colorless thopter artifact creature tokens with flying, where x is the number of +1/+1 counters on it")
	if got := removalTargetDeathPayoffPenalty(p); got != -1.5 {
		t.Errorf("Hangarback (when this dies → tokens) should return -1.5; got %.2f", got)
	}
}

func TestRemovalTargetDeathPayoffPenalty_PersistFires(t *testing.T) {
	gs := newTestGame(t, 2)
	p := mkTargetCreature(gs, 1, "Murderous Redcap", 2, 2, []string{"persist"}, "")
	if got := removalTargetDeathPayoffPenalty(p); got != -1.5 {
		t.Errorf("Murderous Redcap (persist) should return -1.5; got %.2f", got)
	}
}

func TestRemovalTargetDeathPayoffPenalty_GlobalDiesTriggerNotPenalized(t *testing.T) {
	// Blood Artist has a global "whenever a creature dies" trigger —
	// it gets value from OTHER deaths, not its own. Killing the Blood
	// Artist itself doesn't give the opp re-ETB value (no self-recursion
	// or self-death payoff). hasDeathPayoffValue's "self-reference only"
	// gate is the right semantics for this signal.
	gs := newTestGame(t, 2)
	p := mkTargetCreature(gs, 1, "Blood Artist", 0, 1, nil,
		"whenever a creature dies, target player loses 1 life and you gain 1 life")
	if got := removalTargetDeathPayoffPenalty(p); got != 0 {
		t.Errorf("Blood Artist (global creature-dies, no self payoff) should return 0; got %.2f", got)
	}
}

// -----------------------------------------------------------------------
// Integration: ChooseTarget prefers a normal threat over indestructible
// -----------------------------------------------------------------------

// TestChooseTarget_PrefersMortalOverIndestructible pins the indestructible
// penalty integration. Two legal removal targets — a 5/5 vanilla mortal
// AND an 8/8 indestructible. Pre-fix, the indestructible scored higher
// (more power → +1.5 vs +1.5 wash, but commander tag or finisher set
// might push it). With the -5.0 penalty, the vanilla mortal wins.
func TestChooseTarget_PrefersMortalOverIndestructible(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40

	mortal := mkTargetCreature(gs, 1, "Big Vanilla", 5, 5, nil, "")
	indest := mkTargetCreature(gs, 1, "Big Indestructible", 8, 8, []string{"indestructible"}, "")

	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	h.Noise = 0 // disable scoring noise for deterministic test

	legal := []gameengine.Target{
		{Kind: gameengine.TargetKindPermanent, Permanent: mortal},
		{Kind: gameengine.TargetKindPermanent, Permanent: indest},
	}
	picked := h.ChooseTarget(gs, 0, gameast.Filter{Base: "permanent"}, legal)
	if picked.Permanent != mortal {
		got := "<nil>"
		if picked.Permanent != nil && picked.Permanent.Card != nil {
			got = picked.Permanent.Card.Name
		}
		t.Errorf("expected to pick the mortal 5/5 over the indestructible 8/8 (destroy is wasted on indestructible); got %s",
			got)
	}
}

// TestChooseTarget_PrefersNonPayoffOverHangarback pins the death-payoff
// penalty integration. Two comparable threats — a 4/4 vanilla AND a
// Hangarback Walker shape. Hangarback's death triggers spawn tokens
// (gives opp value); the vanilla 4/4 just dies. Hat should prefer
// the vanilla.
func TestChooseTarget_PrefersNonPayoffOverHangarback(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40

	vanilla := mkTargetCreature(gs, 1, "Vanilla 4/4", 4, 4, nil, "")
	hangar := mkTargetCreature(gs, 1, "Hangarback Walker", 4, 4, nil,
		"when this creature dies, create x 1/1 colorless thopter artifact creature tokens with flying")

	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	h.Noise = 0

	legal := []gameengine.Target{
		{Kind: gameengine.TargetKindPermanent, Permanent: vanilla},
		{Kind: gameengine.TargetKindPermanent, Permanent: hangar},
	}
	picked := h.ChooseTarget(gs, 0, gameast.Filter{Base: "permanent"}, legal)
	if picked.Permanent != vanilla {
		got := "<nil>"
		if picked.Permanent != nil && picked.Permanent.Card != nil {
			got = picked.Permanent.Card.Name
		}
		t.Errorf("expected vanilla 4/4 (clean kill) over Hangarback (gives tokens on death); got %s", got)
	}
}

// TestChooseTarget_IndestructiblePenaltyDoesntOverpowerCriticalThreat
// confirms the -5.0 penalty doesn't completely wreck cases where the
// indestructible IS the right target. Specifically: an indestructible
// PLANESWALKER about to ult (PW gets +5.0 ult-now bonus) should still
// outscore a vanilla 2/2. Net for indestructible PW: -5.0 + 5.0 = 0;
// vanilla 2/2 base scoring is ~1.0 + power*0.3 = ~1.6. So the vanilla
// 2/2 wins. That's fine — destroy still does nothing to indestructible
// even if it's a critical threat; the right call is a different removal
// or to bounce.
//
// This test pins the boundary: don't pick the indestructible PW even
// when it's about to ult, because the spell does nothing.
func TestChooseTarget_DoesNotPickIndestructiblePlaneswalkerOverVanilla(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40

	vanilla := mkTargetCreature(gs, 1, "Bear", 2, 2, nil, "")
	pwCard := &gameengine.Card{
		Name:     "Indestructible PW",
		Types:    []string{"planeswalker"},
		TypeLine: "Legendary Planeswalker",
		AST: &gameast.CardAST{
			Name:      "Indestructible PW",
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "indestructible"}},
		},
	}
	pw := &gameengine.Permanent{
		Card:       pwCard,
		Controller: 1,
		Owner:      1,
		Counters:   map[string]int{"loyalty": 5},
		Flags:      map[string]int{},
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, pw)

	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	h.Noise = 0

	legal := []gameengine.Target{
		{Kind: gameengine.TargetKindPermanent, Permanent: vanilla},
		{Kind: gameengine.TargetKindPermanent, Permanent: pw},
	}
	picked := h.ChooseTarget(gs, 0, gameast.Filter{Base: "permanent"}, legal)
	if picked.Permanent != vanilla {
		got := "<nil>"
		if picked.Permanent != nil && picked.Permanent.Card != nil {
			got = picked.Permanent.Card.Name
		}
		t.Errorf("destroy on indestructible PW does nothing — should pick the mortal vanilla instead; got %s", got)
	}
}
