package gameengine

import (
	"testing"
)

// ---------------------------------------------------------------------------
// P1 #2: Prevention Effects Tests
// ---------------------------------------------------------------------------

func TestPreventDamageToPlayer_NoShields(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	src := &Permanent{
		Card:       &Card{Name: "Fireball"},
		Controller: 0,
	}

	result := PreventDamageToPlayer(gs, 1, 5, src)
	if result != 5 {
		t.Fatalf("expected 5 damage with no shields, got %d", result)
	}
}

func TestPreventDamageToPlayer_FullPrevention(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	src := &Permanent{
		Card:       &Card{Name: "Fireball"},
		Controller: 0,
	}

	AddPreventionShield(gs, PreventionShield{
		TargetSeat: 1,
		Amount:     10,
		SourceCard: "Shield of Valor",
		OneShot:    true,
	})

	result := PreventDamageToPlayer(gs, 1, 5, src)
	if result != 0 {
		t.Fatalf("expected 0 damage (5 prevented by 10-point shield), got %d", result)
	}

	// Shield should have 5 remaining.
	if len(gs.PreventionShields) != 1 || gs.PreventionShields[0].Amount != 5 {
		t.Fatal("shield should have 5 remaining after preventing 5")
	}
}

func TestPreventDamageToPlayer_PartialPrevention(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	src := &Permanent{
		Card:       &Card{Name: "Fireball"},
		Controller: 0,
	}

	AddPreventionShield(gs, PreventionShield{
		TargetSeat: 1,
		Amount:     3,
		SourceCard: "Healing Salve",
		OneShot:    true,
	})

	result := PreventDamageToPlayer(gs, 1, 5, src)
	if result != 2 {
		t.Fatalf("expected 2 damage (3 prevented of 5), got %d", result)
	}

	// Shield consumed (amount dropped to 0 + OneShot).
	if len(gs.PreventionShields) != 0 {
		t.Fatal("consumed one-shot shield should be cleaned up")
	}
}

func TestPreventDamageToPlayer_InfiniteShield(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	src := &Permanent{
		Card:       &Card{Name: "Fireball"},
		Controller: 0,
	}

	AddPreventionShield(gs, PreventionShield{
		TargetSeat: 1,
		Amount:     -1, // infinite
		SourceCard: "Teferi's Protection",
	})

	result := PreventDamageToPlayer(gs, 1, 100, src)
	if result != 0 {
		t.Fatalf("expected 0 damage from infinite shield, got %d", result)
	}
}

func TestPreventDamageToPlayer_ProtectionFromEverything(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[1].Flags = map[string]int{"protection_from_everything": 1}
	src := &Permanent{
		Card:       &Card{Name: "Fireball"},
		Controller: 0,
	}

	result := PreventDamageToPlayer(gs, 1, 10, src)
	if result != 0 {
		t.Fatalf("expected 0 damage from protection_from_everything, got %d", result)
	}
}

func TestPreventDamageToPlayer_PreventAllDamageFlag(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[1].Flags = map[string]int{"prevent_all_damage": 1}
	src := &Permanent{
		Card:       &Card{Name: "Fireball"},
		Controller: 0,
	}

	result := PreventDamageToPlayer(gs, 1, 10, src)
	if result != 0 {
		t.Fatalf("expected 0 damage from prevent_all_damage, got %d", result)
	}
}

func TestPreventDamageToPermanent_Shield(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	target := &Permanent{
		Card:       &Card{Name: "Grizzly Bears"},
		Controller: 1,
	}
	src := &Permanent{
		Card:       &Card{Name: "Fireball"},
		Controller: 0,
	}

	AddPreventionShield(gs, PreventionShield{
		TargetPerm: target,
		TargetSeat: -1,
		Amount:     3,
		SourceCard: "Mark of Asylum",
		OneShot:    true,
	})

	result := PreventDamageToPermanent(gs, target, 2, src)
	if result != 0 {
		t.Fatalf("expected 0 damage (2 prevented by 3-point shield), got %d", result)
	}
}

func TestPreventDamageToPermanent_ColorFilter(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	target := &Permanent{
		Card:       &Card{Name: "Angel"},
		Controller: 1,
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, target)

	// Shield that only prevents red damage.
	AddPreventionShield(gs, PreventionShield{
		TargetPerm:   target,
		TargetSeat:   -1,
		Amount:       5,
		SourceFilter: "R",
		SourceCard:   "Circle of Protection: Red",
		OneShot:      true,
	})

	// Red source — prevented.
	redSrc := &Permanent{
		Card:       &Card{Name: "Lightning Bolt", Colors: []string{"R"}},
		Controller: 0,
	}
	result := PreventDamageToPermanent(gs, target, 3, redSrc)
	if result != 0 {
		t.Fatalf("expected 0 damage from red source (COP:Red), got %d", result)
	}

	// Non-red source — not prevented (shield only matches red).
	// Re-add shield since the previous one was partially consumed.
	AddPreventionShield(gs, PreventionShield{
		TargetPerm:   target,
		TargetSeat:   -1,
		Amount:       5,
		SourceFilter: "R",
		SourceCard:   "Circle of Protection: Red",
		OneShot:      true,
	})

	blueSrc := &Permanent{
		Card:       &Card{Name: "Unsummon", Colors: []string{"U"}},
		Controller: 0,
	}
	result = PreventDamageToPermanent(gs, target, 3, blueSrc)
	if result != 3 {
		t.Fatalf("expected 3 damage from blue source (COP:Red doesn't apply), got %d", result)
	}
}
