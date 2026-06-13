package gameengine

import "testing"

// §608.2g self-relocation guard (r63 eliminated-seat-frontier hunt,
// CardIdentity double-zone class). A non-permanent spell goes to its
// owner's graveyard "as the final part of its resolution" ONLY while it
// is still the spell on the stack. If the spell's OWN effect already
// moved its card to another zone — Green Sun's Zenith "Shuffle ~ into
// its owner's library", a self-exiling or self-bouncing sorcery — the
// engine must NOT also drop the card into the graveyard, or the card
// double-zones (library + graveyard). Seed-77 game 939 (Green Sun's
// Zenith) produced 254 CardIdentity hits from exactly this path.

// countCardInZones returns how many of the listed zones contain the card
// pointer across all seats (battlefield counted via Permanent.Card).
func countCardInZones(gs *GameState, card *Card) (lib, gy, exile, bf int) {
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Library {
			if c == card {
				lib++
			}
		}
		for _, c := range s.Graveyard {
			if c == card {
				gy++
			}
		}
		for _, c := range s.Exile {
			if c == card {
				exile++
			}
		}
		for _, p := range s.Battlefield {
			if p != nil && p.Card == card {
				bf++
			}
		}
	}
	return
}

func TestResolve608_2g_SelfRelocatedSpell_NotAlsoGraveyarded(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.EventPolicy = EventLogFull

	// A sorcery whose resolution (modeled here by pre-placing the card in
	// the owner's library, the way Green Sun's Zenith's self-shuffle arm
	// leaves it) is mid-resolve on the stack.
	sorc := &Card{Name: "Self Shuffle Sorcery", Owner: 0, Types: []string{"sorcery"}}
	gs.Seats[0].Library = append(gs.Seats[0].Library, sorc)
	gs.Stack = append(gs.Stack, &StackItem{
		ID: 1, Controller: 0, Card: sorc, Kind: "spell",
	})

	ResolveStackTop(gs)

	lib, gy, _, _ := countCardInZones(gs, sorc)
	if lib != 1 {
		t.Errorf("self-relocated spell should remain in library exactly once, got lib=%d", lib)
	}
	if gy != 0 {
		t.Errorf("§608.2g must NOT also send a self-relocated spell to the graveyard, got gy=%d (double-zone bug)", gy)
	}
	if len(gs.Stack) != 0 {
		t.Errorf("stack should be empty after resolution, got %d", len(gs.Stack))
	}
}

func TestResolve608_3a_RelocatedPermanentSpell_NotAlsoBattlefielded(t *testing.T) {
	// §608.3a: a permanent spell whose card was moved off the stack by a
	// triggered ability resolving ABOVE it (Possibility Storm / Knowledge
	// Pool "that player exiles it") has left the stack and does NOT also
	// enter the battlefield. Modeled by pre-placing the creature card in
	// exile (where Possibility Storm put it) while its spell StackItem
	// resolves. Seed-13 Cryptothrall / seed-7 Mysidian Elder shape.
	gs := NewGameState(2, nil, nil)
	gs.EventPolicy = EventLogFull

	creat := &Card{Name: "Exiled Mid-Cast Creature", Owner: 0,
		Types: []string{"creature"}, BasePower: 2, BaseToughness: 2}
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, creat)
	gs.Stack = append(gs.Stack, &StackItem{
		ID: 1, Controller: 0, Card: creat, Kind: "spell",
	})

	ResolveStackTop(gs)

	_, gy, exile, bf := countCardInZones(gs, creat)
	if exile != 1 {
		t.Errorf("relocated permanent spell should remain in exile exactly once, got exile=%d", exile)
	}
	if bf != 0 {
		t.Errorf("§608.3a must NOT also put a relocated permanent spell onto the battlefield, got bf=%d (double-zone bug)", bf)
	}
	if gy != 0 {
		t.Errorf("relocated permanent spell must not go to graveyard either, got gy=%d", gy)
	}
}

func TestResolve608_2g_VanillaSorcery_StillGraveyarded(t *testing.T) {
	// Control: an ordinary sorcery NOT relocated by its own effect still
	// goes to the graveyard per §608.2g.
	gs := NewGameState(2, nil, nil)
	gs.EventPolicy = EventLogFull

	sorc := &Card{Name: "Plain Sorcery", Owner: 0, Types: []string{"sorcery"}}
	gs.Stack = append(gs.Stack, &StackItem{
		ID: 1, Controller: 0, Card: sorc, Kind: "spell",
	})

	ResolveStackTop(gs)

	lib, gy, _, bf := countCardInZones(gs, sorc)
	if gy != 1 {
		t.Errorf("a vanilla sorcery must go to the graveyard on resolution, got gy=%d", gy)
	}
	if lib != 0 || bf != 0 {
		t.Errorf("a vanilla sorcery must not appear elsewhere, got lib=%d bf=%d", lib, bf)
	}
}
