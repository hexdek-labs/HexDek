package analytics

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// interaction_matrix_r60_test.go — pins per-seat-pair InteractionMatrix
// math + render. The matrix is dashboard-ready as a heatmap or
// chord-diagram input across all analyzed games.

// mkDeclareAttackers fabricates the canonical declare_attackers event
// shape combat.go emits: Seat=attackerSeat, Details["attackers"] =
// []map{attacker, defender_seat}.
func mkDeclareAttackers(attackerSeat int, pairs ...struct {
	attacker      string
	defender_seat int
}) gameengine.Event {
	attackers := make([]map[string]interface{}, 0, len(pairs))
	for _, p := range pairs {
		attackers = append(attackers, map[string]interface{}{
			"attacker":      p.attacker,
			"defender_seat": p.defender_seat,
		})
	}
	return gameengine.Event{
		Kind:    "declare_attackers",
		Seat:    attackerSeat,
		Details: map[string]interface{}{"attackers": attackers},
	}
}

// mkBlockers fabricates the canonical blockers event shape.
func mkBlockers(defenderSeat int, pairs ...struct {
	attacker string
	blockers []string
}) gameengine.Event {
	out := make([]map[string]interface{}, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, map[string]interface{}{
			"attacker": p.attacker,
			"blockers": p.blockers,
		})
	}
	return gameengine.Event{
		Kind:    "blockers",
		Seat:    defenderSeat,
		Details: map[string]interface{}{"pairs": out},
	}
}

// mkCombatDamage fabricates a damage event with combat=true and
// target_kind=player.
func mkCombatDamage(attackerSeat, targetSeat, amount int) gameengine.Event {
	return gameengine.Event{
		Kind:   "damage",
		Seat:   attackerSeat,
		Target: targetSeat,
		Amount: amount,
		Source: "Some Attacker",
		Details: map[string]interface{}{
			"combat":      true,
			"target_kind": "player",
		},
	}
}

// mkCounterSpell fabricates a counter_spell event.
func mkCounterSpell(casterSeat, targetSeat int, targetCard string) gameengine.Event {
	return gameengine.Event{
		Kind:   "counter_spell",
		Seat:   casterSeat,
		Target: targetSeat,
		Source: "generic_counter",
		Details: map[string]interface{}{
			"target_card": targetCard,
		},
	}
}

// TestInteractionMatrix_AttackAndDamage pins the simplest happy path:
// seat 0 declares 2 attackers at seat 2, both connect for 5 damage.
// Cell[0][2] = {Attacks: 2, PlayerDamage: 10, Blocks: 0, Counters: 0}.
func TestInteractionMatrix_AttackAndDamage(t *testing.T) {
	events := []gameengine.Event{
		mkDeclareAttackers(0,
			struct{ attacker string; defender_seat int }{"Goblin A", 2},
			struct{ attacker string; defender_seat int }{"Goblin B", 2},
		),
		mkCombatDamage(0, 2, 5),
		mkCombatDamage(0, 2, 5),
	}
	ga := AnalyzeGame(events, 4, []string{"A", "B", "C", "D"}, -1, 1, nil, nil)
	if ga.Interactions == nil || ga.Interactions.Seats != 4 {
		t.Fatalf("Interactions = %+v, want allocated 4-seat matrix", ga.Interactions)
	}
	cell := ga.Interactions.Cells[0][2]
	if cell.Attacks != 2 {
		t.Errorf("Attacks = %d, want 2", cell.Attacks)
	}
	if cell.PlayerDamage != 10 {
		t.Errorf("PlayerDamage = %d, want 10", cell.PlayerDamage)
	}
	if cell.Blocks != 0 || cell.Counters != 0 {
		t.Errorf("Blocks/Counters non-zero: %+v", cell)
	}

	// Reverse direction stays empty.
	if rev := ga.Interactions.Cells[2][0]; rev.Attacks != 0 || rev.PlayerDamage != 0 {
		t.Errorf("reverse cell should be empty, got %+v", rev)
	}
}

// TestInteractionMatrix_BlockAttribution pins that a blockers event
// resolves back to the matching declare_attackers' seat. Seat 0
// declares Goblin A targeting seat 2; seat 2 blocks with Wall.
// Block lands on Cell[0][2].
func TestInteractionMatrix_BlockAttribution(t *testing.T) {
	events := []gameengine.Event{
		mkDeclareAttackers(0,
			struct{ attacker string; defender_seat int }{"Goblin A", 2},
		),
		mkBlockers(2,
			struct{ attacker string; blockers []string }{"Goblin A", []string{"Wall of Bone"}},
		),
	}
	ga := AnalyzeGame(events, 3, []string{"A", "B", "C"}, -1, 1, nil, nil)
	cell := ga.Interactions.Cells[0][2]
	if cell.Blocks != 1 {
		t.Errorf("Blocks = %d, want 1", cell.Blocks)
	}
	// Reverse cell stays empty — block from defender's side shouldn't
	// land on Cell[2][0].
	if ga.Interactions.Cells[2][0].Blocks != 0 {
		t.Errorf("reverse cell Blocks should be 0; got %+v", ga.Interactions.Cells[2][0])
	}
}

// TestInteractionMatrix_BlockMultipleBlockersPerAttacker pins that a
// 2-blocker assignment to one attacker counts as ONE Block (per the
// metric definition: per-attacker, not per-blocker). Defends against
// an "easy mistake" implementation that increments per blocker name.
func TestInteractionMatrix_BlockMultipleBlockersPerAttacker(t *testing.T) {
	events := []gameengine.Event{
		mkDeclareAttackers(1,
			struct{ attacker string; defender_seat int }{"Trampler", 3},
		),
		mkBlockers(3,
			struct{ attacker string; blockers []string }{"Trampler", []string{"Chump 1", "Chump 2", "Chump 3"}},
		),
	}
	ga := AnalyzeGame(events, 4, []string{"A", "B", "C", "D"}, -1, 1, nil, nil)
	if ga.Interactions.Cells[1][3].Blocks != 1 {
		t.Errorf("Blocks = %d, want 1 (per-attacker, not per-blocker)", ga.Interactions.Cells[1][3].Blocks)
	}
}

// TestInteractionMatrix_CounterAttribution pins counter_spell
// attribution into the matrix. Seat 0 counters seat 1's spell; lands
// on Cell[0][1].
func TestInteractionMatrix_CounterAttribution(t *testing.T) {
	events := []gameengine.Event{
		mkCounterSpell(0, 1, "Wheel of Fortune"),
		mkCounterSpell(0, 1, "Demonic Tutor"),
		mkCounterSpell(2, 1, "Force of Will"),
	}
	ga := AnalyzeGame(events, 4, []string{"A", "B", "C", "D"}, -1, 1, nil, nil)
	if ga.Interactions.Cells[0][1].Counters != 2 {
		t.Errorf("Cells[0][1].Counters = %d, want 2", ga.Interactions.Cells[0][1].Counters)
	}
	if ga.Interactions.Cells[2][1].Counters != 1 {
		t.Errorf("Cells[2][1].Counters = %d, want 1", ga.Interactions.Cells[2][1].Counters)
	}
	// Reverse stays empty.
	if ga.Interactions.Cells[1][0].Counters != 0 {
		t.Errorf("reverse Cells[1][0] non-zero: %+v", ga.Interactions.Cells[1][0])
	}
}

// TestInteractionMatrix_NonCombatDamageIgnored pins that non-combat
// damage (Lightning Bolt to a player) does NOT contribute to
// PlayerDamage in the matrix. The cell is specifically combat
// attribution; non-combat damage is tracked separately by source card.
func TestInteractionMatrix_NonCombatDamageIgnored(t *testing.T) {
	events := []gameengine.Event{
		{
			Kind:   "damage",
			Seat:   0,
			Target: 1,
			Amount: 3,
			Source: "Lightning Bolt",
			Details: map[string]interface{}{
				"combat":      false,
				"target_kind": "player",
			},
		},
	}
	ga := AnalyzeGame(events, 2, []string{"A", "B"}, -1, 1, nil, nil)
	if ga.Interactions.Cells[0][1].PlayerDamage != 0 {
		t.Errorf("PlayerDamage = %d, want 0 (non-combat damage shouldn't count)",
			ga.Interactions.Cells[0][1].PlayerDamage)
	}
}

// TestInteractionMatrix_DiagonalStaysZero pins that self-targeted
// events (a seat attacking itself, blocking itself, countering its
// own spell — all degenerate but possible) don't write to the
// diagonal. Diagonal cells must be zero for clean heatmap rendering.
func TestInteractionMatrix_DiagonalStaysZero(t *testing.T) {
	events := []gameengine.Event{
		mkDeclareAttackers(0,
			struct{ attacker string; defender_seat int }{"SelfTarget", 0},
		),
		mkCounterSpell(1, 1, "Self Counter"),
	}
	ga := AnalyzeGame(events, 3, []string{"A", "B", "C"}, -1, 1, nil, nil)
	for s := 0; s < 3; s++ {
		c := ga.Interactions.Cells[s][s]
		if c.Attacks != 0 || c.PlayerDamage != 0 || c.Blocks != 0 || c.Counters != 0 {
			t.Errorf("diagonal Cells[%d][%d] non-zero: %+v", s, s, c)
		}
	}
}

// TestWriteInteractionMatrices_RenderShape pins the markdown surface:
// per-game heading, From\To header, commander labels on rows and
// columns, and packed cells like "A2 D10 B1 C0".
func TestWriteInteractionMatrices_RenderShape(t *testing.T) {
	events := []gameengine.Event{
		mkDeclareAttackers(0,
			struct{ attacker string; defender_seat int }{"Goblin A", 1},
			struct{ attacker string; defender_seat int }{"Goblin B", 1},
		),
		mkCombatDamage(0, 1, 5),
		mkCombatDamage(0, 1, 5),
		mkBlockers(1,
			struct{ attacker string; blockers []string }{"Goblin A", []string{"Wall"}},
		),
		mkCounterSpell(0, 1, "Bolt"),
	}
	ga := AnalyzeGame(events, 2, []string{"Aggro Cmdr", "Control Cmdr"}, 0, 1, nil, nil)
	r := &AnalyticsReport{
		Analyses:       []*GameAnalysis{ga},
		CommanderNames: []string{"Aggro Cmdr", "Control Cmdr"},
		TotalGames:     1,
	}
	var b strings.Builder
	r.writeInteractionMatrices(&b)
	out := b.String()

	if !strings.Contains(out, "## Interaction Matrix") {
		t.Errorf("missing section header; got:\n%s", out)
	}
	if !strings.Contains(out, "### Game 1") {
		t.Errorf("missing per-game header; got:\n%s", out)
	}
	if !strings.Contains(out, "| From \\ To |") {
		t.Errorf("missing From\\To column header; got:\n%s", out)
	}
	if !strings.Contains(out, "Aggro Cmdr") || !strings.Contains(out, "Control Cmdr") {
		t.Errorf("missing commander labels; got:\n%s", out)
	}
	if !strings.Contains(out, "A2 D10 B1 C1") {
		t.Errorf("expected packed-cell format A2 D10 B1 C1; got:\n%s", out)
	}
}

// TestWriteInteractionMatrices_CapsAtFiveGames pins the render cap +
// tail footnote.
func TestWriteInteractionMatrices_CapsAtFiveGames(t *testing.T) {
	mk := func() *GameAnalysis {
		return AnalyzeGame(
			[]gameengine.Event{mkDeclareAttackers(0,
				struct{ attacker string; defender_seat int }{"X", 1},
			)},
			2, []string{"A", "B"}, -1, 1, nil, nil)
	}
	r := &AnalyticsReport{
		Analyses:       []*GameAnalysis{mk(), mk(), mk(), mk(), mk(), mk(), mk()},
		CommanderNames: []string{"A", "B"},
		TotalGames:     7,
	}
	var b strings.Builder
	r.writeInteractionMatrices(&b)
	out := b.String()
	if strings.Contains(out, "### Game 6") {
		t.Errorf("Game 6 should be capped; got:\n%s", out)
	}
	if !strings.Contains(out, "### Game 5") {
		t.Errorf("Game 5 should render; got:\n%s", out)
	}
	if !strings.Contains(out, "+2 more games") {
		t.Errorf("missing tail footnote; got:\n%s", out)
	}
}

// TestWriteInteractionMatrices_EmptyAnalyses pins the no-data guard.
func TestWriteInteractionMatrices_EmptyAnalyses(t *testing.T) {
	var b strings.Builder
	(&AnalyticsReport{}).writeInteractionMatrices(&b)
	out := b.String()
	if !strings.Contains(out, "## Interaction Matrix") {
		t.Errorf("missing header; got:\n%s", out)
	}
	if !strings.Contains(out, "No game data") {
		t.Errorf("expected empty-state message; got:\n%s", out)
	}
}
