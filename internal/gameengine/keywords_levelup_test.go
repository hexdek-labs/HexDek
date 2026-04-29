package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ===========================================================================
// Test helpers for level up
// ===========================================================================

func newLevelUpGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(2, rng, nil)
	// Set to main phase for sorcery-speed activation.
	gs.Step = "precombat_main"
	gs.Phase = "main"
	gs.Active = 0
	return gs
}

// makeLevelerPerm creates a leveler creature on the battlefield with the
// specified base P/T, level up cost, and bracket data encoded in the AST.
//
// Oracle text format for a leveler card:
//
//	Level up {cost}
//	LEVEL minLevel-maxLevel
//	P/T
//	keywords...
//	LEVEL minLevel+
//	P/T
//	keywords...
func makeLevelerPerm(gs *GameState, seat int, name string, basePow, baseTough int, rawCost string, oracle string) *Permanent {
	card := &Card{
		Name:          name,
		Owner:         seat,
		BasePower:     basePow,
		BaseToughness: baseTough,
		Types:         []string{"creature"},
	}

	// Build AST abilities from oracle text lines.
	abilities := buildLevelerAST(rawCost, oracle)
	card.AST = &gameast.CardAST{
		Name:        name,
		Abilities:   abilities,
		FullyParsed: true,
	}

	p := &Permanent{
		Card:       card,
		Controller: seat,
		Owner:      seat,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// buildLevelerAST builds AST abilities for a leveler card.
func buildLevelerAST(rawCost string, oracle string) []gameast.Ability {
	var abilities []gameast.Ability

	// Add the level up keyword.
	abilities = append(abilities, &gameast.Keyword{
		Name: "level up",
		Raw:  "level up " + rawCost,
	})

	// Parse oracle text for LEVEL headers and data.
	// This adds Static(class_level_band) for each bracket, plus keywords.
	lines := splitLines(oracle)
	for _, line := range lines {
		line = trimSpace(line)
		if line == "" {
			continue
		}
		// Check for LEVEL header.
		m := levelBandRegex.FindStringSubmatch(line)
		if m != nil {
			lo := mustAtoi(m[1])
			var hi interface{}
			if m[2] == "+" {
				hi = nil // open-ended
			} else if m[3] != "" {
				hi = float64(mustAtoi(m[3]))
			} else {
				hi = float64(lo)
			}
			var args []interface{}
			args = append(args, float64(lo))
			args = append(args, hi)
			abilities = append(abilities, &gameast.Static{
				Modification: &gameast.Modification{
					ModKind: "class_level_band",
					Args:    args,
				},
				Raw: line,
			})
			continue
		}
		// Check for P/T line.
		pm := ptRegex.FindStringSubmatch(line)
		if pm != nil {
			abilities = append(abilities, &gameast.Static{
				Modification: &gameast.Modification{
					ModKind: "spell_effect",
					Args: []interface{}{
						&gameast.Modification{
							ModKind: "parsed_effect_residual",
							Args:    []interface{}{"bare P/T " + line},
						},
					},
				},
				Raw: line,
			})
			continue
		}
		// Otherwise it's a keyword line.
		abilities = append(abilities, &gameast.Keyword{
			Name: line,
			Raw:  line,
		})
	}

	return abilities
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimSpace(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\r') {
		i++
	}
	j := len(s)
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t' || s[j-1] == '\r') {
		j--
	}
	return s[i:j]
}

func mustAtoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

// ===========================================================================
// Test: Level Up Activation Adds Counter
// ===========================================================================

func TestLevelUp_ActivationAddsCounter(t *testing.T) {
	gs := newLevelUpGame(t)
	gs.Seats[0].ManaPool = 10

	perm := makeLevelerPerm(gs, 0, "Kargan Dragonlord", 2, 2, "{r}{r}",
		"LEVEL 4-7\n4/4\nflying\nLEVEL 8+\n8/8\nflying\ntrample")

	// Activate level up once.
	err := ActivateLevelUp(gs, 0, perm)
	if err != nil {
		t.Fatalf("ActivateLevelUp failed: %v", err)
	}

	if perm.Counters["level"] != 1 {
		t.Errorf("expected 1 level counter, got %d", perm.Counters["level"])
	}

	// Activate again.
	err = ActivateLevelUp(gs, 0, perm)
	if err != nil {
		t.Fatalf("second ActivateLevelUp failed: %v", err)
	}

	if perm.Counters["level"] != 2 {
		t.Errorf("expected 2 level counters, got %d", perm.Counters["level"])
	}
}

// ===========================================================================
// Test: Sorcery-Speed Restriction
// ===========================================================================

func TestLevelUp_SorcerySpeedRestriction(t *testing.T) {
	gs := newLevelUpGame(t)
	gs.Seats[0].ManaPool = 10

	perm := makeLevelerPerm(gs, 0, "Student of Warfare", 1, 1, "{w}",
		"LEVEL 2-6\n3/3\nfirst strike\nLEVEL 7+\n4/4\ndouble strike")

	// Should work during main phase.
	err := ActivateLevelUp(gs, 0, perm)
	if err != nil {
		t.Fatalf("should succeed during main phase: %v", err)
	}

	// Not the controller's turn.
	gs.Active = 1
	err = ActivateLevelUp(gs, 0, perm)
	if err == nil {
		t.Error("should fail when not active player's turn")
	}
	gs.Active = 0

	// Not during main phase.
	gs.Step = "combat"
	err = ActivateLevelUp(gs, 0, perm)
	if err == nil {
		t.Error("should fail during combat phase")
	}
	gs.Step = "precombat_main"

	// Stack not empty.
	gs.Stack = append(gs.Stack, &StackItem{Card: &Card{Name: "Lightning Bolt"}})
	err = ActivateLevelUp(gs, 0, perm)
	if err == nil {
		t.Error("should fail with non-empty stack")
	}
	gs.Stack = nil

	// Wrong controller.
	err = ActivateLevelUp(gs, 1, perm)
	if err == nil {
		t.Error("should fail for non-controller")
	}
}

// ===========================================================================
// Test: Insufficient Mana
// ===========================================================================

func TestLevelUp_InsufficientMana(t *testing.T) {
	gs := newLevelUpGame(t)
	gs.Seats[0].ManaPool = 0

	perm := makeLevelerPerm(gs, 0, "Kargan Dragonlord", 2, 2, "{r}{r}",
		"LEVEL 4-7\n4/4\nflying\nLEVEL 8+\n8/8\nflying\ntrample")

	err := ActivateLevelUp(gs, 0, perm)
	if err == nil {
		t.Error("should fail with insufficient mana")
	}
	if perm.Counters["level"] != 0 {
		t.Errorf("should not have added counter, got %d", perm.Counters["level"])
	}
}

// ===========================================================================
// Test: P/T Changes at Bracket Boundaries
// ===========================================================================

func TestLevelUp_PTChangesAtBrackets(t *testing.T) {
	gs := newLevelUpGame(t)
	gs.Seats[0].ManaPool = 100

	// Kargan Dragonlord: 2/2 base, LEVEL 4-7 = 4/4, LEVEL 8+ = 8/8
	perm := makeLevelerPerm(gs, 0, "Kargan Dragonlord", 2, 2, "{r}{r}",
		"LEVEL 4-7\n4/4\nflying\nLEVEL 8+\n8/8\nflying\ntrample")

	// Level 0: base P/T.
	bracket := GetLevelBracket(perm)
	if bracket != nil {
		t.Error("at level 0, should have no bracket (use base P/T)")
	}
	p, to := GetEffectiveLevelPT(perm)
	if p != 2 || to != 2 {
		t.Errorf("level 0: expected 2/2, got %d/%d", p, to)
	}

	// Level up to 3 — still below first bracket.
	for i := 0; i < 3; i++ {
		if err := ActivateLevelUp(gs, 0, perm); err != nil {
			t.Fatalf("level up %d failed: %v", i+1, err)
		}
	}
	bracket = GetLevelBracket(perm)
	if bracket != nil {
		t.Error("at level 3, should have no matching bracket (first bracket is 4-7)")
	}
	p, to = GetEffectiveLevelPT(perm)
	if p != 2 || to != 2 {
		t.Errorf("level 3: expected 2/2, got %d/%d", p, to)
	}

	// Level up to 4 — first bracket kicks in.
	if err := ActivateLevelUp(gs, 0, perm); err != nil {
		t.Fatalf("level up 4 failed: %v", err)
	}
	bracket = GetLevelBracket(perm)
	if bracket == nil {
		t.Fatal("at level 4, should match first bracket")
	}
	if bracket.Power != 4 || bracket.Toughness != 4 {
		t.Errorf("level 4: expected bracket 4/4, got %d/%d", bracket.Power, bracket.Toughness)
	}

	// Level up to 7 — still in first bracket.
	for i := 0; i < 3; i++ {
		if err := ActivateLevelUp(gs, 0, perm); err != nil {
			t.Fatalf("level up %d failed: %v", 5+i, err)
		}
	}
	bracket = GetLevelBracket(perm)
	if bracket == nil || bracket.Power != 4 || bracket.Toughness != 4 {
		t.Errorf("level 7: expected bracket 4/4")
	}

	// Level up to 8 — second bracket.
	if err := ActivateLevelUp(gs, 0, perm); err != nil {
		t.Fatalf("level up 8 failed: %v", err)
	}
	bracket = GetLevelBracket(perm)
	if bracket == nil {
		t.Fatal("at level 8, should match second bracket")
	}
	if bracket.Power != 8 || bracket.Toughness != 8 {
		t.Errorf("level 8: expected bracket 8/8, got %d/%d", bracket.Power, bracket.Toughness)
	}

	// Level up to 12 — still in second bracket (open-ended).
	for i := 0; i < 4; i++ {
		if err := ActivateLevelUp(gs, 0, perm); err != nil {
			t.Fatalf("level up %d failed: %v", 9+i, err)
		}
	}
	bracket = GetLevelBracket(perm)
	if bracket == nil || bracket.Power != 8 || bracket.Toughness != 8 {
		t.Errorf("level 12: expected bracket 8/8")
	}
}

// ===========================================================================
// Test: Abilities Granted at Bracket Thresholds
// ===========================================================================

func TestLevelUp_AbilitiesGrantedAtBrackets(t *testing.T) {
	gs := newLevelUpGame(t)
	gs.Seats[0].ManaPool = 100

	// Student of Warfare: 1/1 base, LEVEL 2-6 = 3/3 + first strike,
	// LEVEL 7+ = 4/4 + double strike
	perm := makeLevelerPerm(gs, 0, "Student of Warfare", 1, 1, "{w}",
		"LEVEL 2-6\n3/3\nfirst strike\nLEVEL 7+\n4/4\ndouble strike")

	// Level 0: no bracket keywords.
	bracket := GetLevelBracket(perm)
	if bracket != nil {
		t.Error("level 0 should have no bracket")
	}

	// Level up to 2: first strike bracket.
	for i := 0; i < 2; i++ {
		if err := ActivateLevelUp(gs, 0, perm); err != nil {
			t.Fatalf("level up %d failed: %v", i+1, err)
		}
	}
	bracket = GetLevelBracket(perm)
	if bracket == nil {
		t.Fatal("level 2: expected bracket")
	}
	if !containsKeyword(bracket.Keywords, "first strike") {
		t.Errorf("level 2: expected first strike in keywords, got %v", bracket.Keywords)
	}

	// Level up to 7: double strike bracket.
	for i := 0; i < 5; i++ {
		if err := ActivateLevelUp(gs, 0, perm); err != nil {
			t.Fatalf("level up %d failed: %v", 3+i, err)
		}
	}
	bracket = GetLevelBracket(perm)
	if bracket == nil {
		t.Fatal("level 7: expected bracket")
	}
	if !containsKeyword(bracket.Keywords, "double strike") {
		t.Errorf("level 7: expected double strike in keywords, got %v", bracket.Keywords)
	}
	// Should NOT still have first strike (moved to a new bracket).
	if containsKeyword(bracket.Keywords, "first strike") {
		t.Errorf("level 7: should NOT have first strike (that was bracket 2-6), got %v", bracket.Keywords)
	}
}

func containsKeyword(keywords []string, target string) bool {
	for _, kw := range keywords {
		if kw == target {
			return true
		}
	}
	return false
}

// ===========================================================================
// Test: Interaction with +1/+1 Counters
// ===========================================================================

func TestLevelUp_InteractionWithPlusOneCounters(t *testing.T) {
	gs := newLevelUpGame(t)
	gs.Seats[0].ManaPool = 100

	// Kargan Dragonlord: 2/2 base, LEVEL 4-7 = 4/4
	perm := makeLevelerPerm(gs, 0, "Kargan Dragonlord", 2, 2, "{r}{r}",
		"LEVEL 4-7\n4/4\nflying\nLEVEL 8+\n8/8\nflying\ntrample")

	// Add some +1/+1 counters.
	perm.AddCounter("+1/+1", 3)

	// Level 0: base 2/2 + 3 counters = 5/5.
	if perm.Power() != 5 || perm.Toughness() != 5 {
		t.Errorf("level 0 with +3/+3: expected 5/5, got %d/%d", perm.Power(), perm.Toughness())
	}

	// Level up to 4, apply bracket effects.
	for i := 0; i < 4; i++ {
		if err := ActivateLevelUp(gs, 0, perm); err != nil {
			t.Fatalf("level up %d failed: %v", i+1, err)
		}
	}

	// Apply bracket effects to set the P/T modification.
	ApplyLevelBracketEffects(gs, perm)

	// Bracket P/T = 4/4, +1/+1 counters = +3/+3.
	// With the level_bracket modification, Power() = base(2) + counters(3) + delta(2) = 7.
	// Toughness() = base(2) + counters(3) + delta(2) = 7.
	// (delta = bracket 4 - base 2 = 2)
	expectedP := 4 + 3 // bracket base + counters
	expectedT := 4 + 3
	if perm.Power() != expectedP || perm.Toughness() != expectedT {
		t.Errorf("level 4 with +3/+3: expected %d/%d, got %d/%d",
			expectedP, expectedT, perm.Power(), perm.Toughness())
	}
}

// ===========================================================================
// Test: Level Up Cost Parsing
// ===========================================================================

func TestLevelUp_CostParsing(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected int
	}{
		{"single colored", "level up {r}", 1},
		{"double colored", "level up {r}{r}", 2},
		{"generic", "level up {4}", 4},
		{"mixed", "level up {2}{g}", 3},
		{"single white", "level up {w}", 1},
		{"generic 1", "level up {1}", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := parseLevelUpCostFromRaw(tt.raw)
			if cost != tt.expected {
				t.Errorf("parseLevelUpCostFromRaw(%q) = %d, want %d", tt.raw, cost, tt.expected)
			}
		})
	}
}

// ===========================================================================
// Test: Level Bracket Parsing
// ===========================================================================

func TestLevelUp_BracketParsing(t *testing.T) {
	gs := newLevelUpGame(t)

	// Coralhelm Commander: LEVEL 2-3 = 3/3 flying, LEVEL 4+ = 4/4 flying + anthem
	perm := makeLevelerPerm(gs, 0, "Coralhelm Commander", 2, 2, "{1}",
		"LEVEL 2-3\n3/3\nflying\nLEVEL 4+\n4/4\nflying")

	brackets := ParseLevelBrackets(perm.Card)
	if len(brackets) != 2 {
		t.Fatalf("expected 2 brackets, got %d", len(brackets))
	}

	// First bracket: 2-3.
	if brackets[0].MinLevel != 2 || brackets[0].MaxLevel != 3 {
		t.Errorf("bracket 0: expected 2-3, got %d-%d", brackets[0].MinLevel, brackets[0].MaxLevel)
	}
	if brackets[0].Power != 3 || brackets[0].Toughness != 3 {
		t.Errorf("bracket 0: expected 3/3, got %d/%d", brackets[0].Power, brackets[0].Toughness)
	}

	// Second bracket: 4+.
	if brackets[1].MinLevel != 4 || brackets[1].MaxLevel != -1 {
		t.Errorf("bracket 1: expected 4+, got %d-%d", brackets[1].MinLevel, brackets[1].MaxLevel)
	}
	if brackets[1].Power != 4 || brackets[1].Toughness != 4 {
		t.Errorf("bracket 1: expected 4/4, got %d/%d", brackets[1].Power, brackets[1].Toughness)
	}
}

// ===========================================================================
// Test: No Level Up on Non-Leveler
// ===========================================================================

func TestLevelUp_NonLeveler(t *testing.T) {
	gs := newLevelUpGame(t)
	gs.Seats[0].ManaPool = 10

	// Regular creature, no level up.
	perm := addMiscBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")

	err := ActivateLevelUp(gs, 0, perm)
	if err == nil {
		t.Error("should fail on non-leveler creature")
	}
}

// ===========================================================================
// Test: Level Up event is logged
// ===========================================================================

func TestLevelUp_EventLogged(t *testing.T) {
	gs := newLevelUpGame(t)
	gs.Seats[0].ManaPool = 10

	perm := makeLevelerPerm(gs, 0, "Zulaport Enforcer", 1, 1, "{4}",
		"LEVEL 1-2\n3/3\nLEVEL 3+\n5/5")

	initialEvents := len(gs.EventLog)

	err := ActivateLevelUp(gs, 0, perm)
	if err != nil {
		t.Fatalf("ActivateLevelUp failed: %v", err)
	}

	found := false
	for _, e := range gs.EventLog[initialEvents:] {
		if e.Kind == "level_up" {
			found = true
			if e.Amount != 1 {
				t.Errorf("expected level_up event amount=1, got %d", e.Amount)
			}
			break
		}
	}
	if !found {
		t.Error("expected level_up event in log")
	}
}

// ===========================================================================
// Test: ApplyLevelBracketEffects modifies P/T
// ===========================================================================

func TestLevelUp_ApplyBracketEffects(t *testing.T) {
	gs := newLevelUpGame(t)

	perm := makeLevelerPerm(gs, 0, "Student of Warfare", 1, 1, "{w}",
		"LEVEL 2-6\n3/3\nfirst strike\nLEVEL 7+\n4/4\ndouble strike")

	// Set level counters directly.
	perm.AddCounter("level", 2)

	// Apply bracket effects.
	ApplyLevelBracketEffects(gs, perm)

	// Should have a level_bracket modification.
	hasLevelMod := false
	for _, m := range perm.Modifications {
		if m.Duration == "level_bracket" {
			hasLevelMod = true
			// Delta should be 3-1=2 for power, 3-1=2 for toughness.
			if m.Power != 2 || m.Toughness != 2 {
				t.Errorf("level_bracket mod: expected +2/+2 delta, got +%d/+%d", m.Power, m.Toughness)
			}
		}
	}
	if !hasLevelMod {
		t.Error("expected level_bracket modification after ApplyLevelBracketEffects")
	}

	// P/T should be base(1) + delta(2) = 3/3.
	if perm.Power() != 3 || perm.Toughness() != 3 {
		t.Errorf("expected 3/3 after bracket effects, got %d/%d", perm.Power(), perm.Toughness())
	}

	// Keywords should be granted.
	if !perm.HasKeyword("first strike") {
		t.Error("expected first strike keyword after bracket effects")
	}
}

// ===========================================================================
// Test: Multiple activations don't stack bracket mods
// ===========================================================================

func TestLevelUp_BracketModsNotStacked(t *testing.T) {
	gs := newLevelUpGame(t)

	perm := makeLevelerPerm(gs, 0, "Student of Warfare", 1, 1, "{w}",
		"LEVEL 2-6\n3/3\nfirst strike\nLEVEL 7+\n4/4\ndouble strike")

	perm.AddCounter("level", 3)

	// Apply bracket effects multiple times.
	ApplyLevelBracketEffects(gs, perm)
	ApplyLevelBracketEffects(gs, perm)
	ApplyLevelBracketEffects(gs, perm)

	// Should still be 3/3, not 3 + 3*delta.
	if perm.Power() != 3 || perm.Toughness() != 3 {
		t.Errorf("bracket effects should not stack: expected 3/3, got %d/%d",
			perm.Power(), perm.Toughness())
	}

	// Only one level_bracket modification should exist.
	count := 0
	for _, m := range perm.Modifications {
		if m.Duration == "level_bracket" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 level_bracket modification, got %d", count)
	}
}

// ===========================================================================
// Test: Mana is deducted on activation
// ===========================================================================

func TestLevelUp_ManaDeducted(t *testing.T) {
	gs := newLevelUpGame(t)
	gs.Seats[0].ManaPool = 5

	// Cost = {r}{r} = 2 mana.
	perm := makeLevelerPerm(gs, 0, "Kargan Dragonlord", 2, 2, "{r}{r}",
		"LEVEL 4-7\n4/4\nflying\nLEVEL 8+\n8/8\nflying\ntrample")

	err := ActivateLevelUp(gs, 0, perm)
	if err != nil {
		t.Fatalf("ActivateLevelUp failed: %v", err)
	}

	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("expected 3 mana remaining (5-2), got %d", gs.Seats[0].ManaPool)
	}
}

// ===========================================================================
// Test: Postcombat main phase works too
// ===========================================================================

func TestLevelUp_PostcombatMain(t *testing.T) {
	gs := newLevelUpGame(t)
	gs.Step = "postcombat_main"
	gs.Seats[0].ManaPool = 10

	perm := makeLevelerPerm(gs, 0, "Student of Warfare", 1, 1, "{w}",
		"LEVEL 2-6\n3/3\nfirst strike\nLEVEL 7+\n4/4\ndouble strike")

	err := ActivateLevelUp(gs, 0, perm)
	if err != nil {
		t.Errorf("should succeed during postcombat main: %v", err)
	}
}

// ===========================================================================
// Test: Level up has no cap (CR §711.4)
// ===========================================================================

func TestLevelUp_NoCap(t *testing.T) {
	gs := newLevelUpGame(t)
	gs.Seats[0].ManaPool = 100

	perm := makeLevelerPerm(gs, 0, "Zulaport Enforcer", 1, 1, "{4}",
		"LEVEL 1-2\n3/3\nLEVEL 3+\n5/5")

	// Level up 20 times — no cap per CR §711.4.
	for i := 0; i < 20; i++ {
		err := ActivateLevelUp(gs, 0, perm)
		if err != nil {
			t.Fatalf("level up %d should succeed (no cap): %v", i+1, err)
		}
	}

	if perm.Counters["level"] != 20 {
		t.Errorf("expected 20 level counters, got %d", perm.Counters["level"])
	}

	// Should be in the 3+ bracket with 5/5.
	bracket := GetLevelBracket(perm)
	if bracket == nil || bracket.Power != 5 || bracket.Toughness != 5 {
		t.Error("at level 20, should be in 3+ bracket with 5/5")
	}
}
