package gameengine

import "testing"

// r61 hotfix regression: the live turn runner sets gs.Phase = "main"; the
// §307.1 sorcery-speed gate must allow sorceries there (it previously checked
// only the step-named values "precombat_main"/"postcombat_main" as the PHASE,
// so every sorcery cast in a real main phase was silently rejected).
func TestSorcerySpeed_MainPhaseAllowed(t *testing.T) {
	mk := func(phase, step string, stackN int) error {
		gs := NewGameState(2, nil, nil)
		gs.Active = 0
		gs.Phase, gs.Step = phase, step
		gs.Stack = make([]*StackItem, stackN)
		c := &Card{Name: "Probe Sorcery", Owner: 0, Types: []string{"sorcery"}}
		gs.Seats[0].Hand = append(gs.Seats[0].Hand, c)
		return CastSpell(gs, 0, c, nil)
	}
	// Real main phases (what the turn runner actually sets) must ALLOW sorceries.
	if err := mk("main", "precombat_main", 0); err != nil {
		t.Errorf("sorcery in precombat main (gs.Phase=main) should be allowed, got %v", err)
	}
	if err := mk("main", "postcombat_main", 0); err != nil {
		t.Errorf("sorcery in postcombat main (gs.Phase=main) should be allowed, got %v", err)
	}
	// Combat phase must REJECT sorceries.
	if err := mk("combat", "beginning_of_combat", 0); err == nil {
		t.Error("sorcery in combat should be rejected")
	}
	// Main phase but non-empty stack must reject (CR 307.1 empty-stack).
	if err := mk("main", "precombat_main", 1); err == nil {
		t.Error("sorcery with non-empty stack should be rejected")
	}
}
