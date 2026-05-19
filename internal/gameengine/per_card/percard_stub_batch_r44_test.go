package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R44 stub-batch ports — five gen_*.go pure-stub handlers from the
// alphabetical FIRST half (a-m), avoiding R36/R37/R41/R42/R42b/R43 sets.
//
// Picks:
//   - Alisaie Leveilleur            Dualcast 2nd-spell cost reduction
//   - Cecily, Haunted Mage           attack draw+lose + free-cast partial
//   - Extus, Oriq Overlord           Magecraft return creature from gy
//   - Jaxis, the Troublemaker        activated copy-creature with die-draw
//   - Fire Lord Zuko                 firebending X + exile-cast/etb counter

// ---------------------------------------------------------------------------
// Alisaie Leveilleur — Dualcast cost reduction
// ---------------------------------------------------------------------------

func TestAlisaie_SecondSpellGetsTwoOff(t *testing.T) {
	gs := newGame(t, 2)
	stampCreaturePT(addPerm(gs, 0, "Alisaie Leveilleur", "creature", "legendary"), 2, 2)
	gs.Seats[0].Turn.SpellsCast = 1 // one spell already cast this turn

	spell := &gameengine.Card{Name: "Doom Blade", Owner: 0, Types: []string{"instant"}}
	mods := gameengine.ScanCostModifiers(gs, spell, 0)
	total := 0
	for _, m := range mods {
		if m.Source == "Alisaie Leveilleur" {
			total += m.Amount
		}
	}
	if total != 2 {
		t.Errorf("expected Alisaie discount of 2 on 2nd spell, got %d (mods=%+v)", total, mods)
	}
}

func TestAlisaie_FirstSpellGetsNoDiscount(t *testing.T) {
	gs := newGame(t, 2)
	stampCreaturePT(addPerm(gs, 0, "Alisaie Leveilleur", "creature", "legendary"), 2, 2)
	gs.Seats[0].Turn.SpellsCast = 0

	spell := &gameengine.Card{Name: "Doom Blade", Owner: 0, Types: []string{"instant"}}
	mods := gameengine.ScanCostModifiers(gs, spell, 0)
	for _, m := range mods {
		if m.Source == "Alisaie Leveilleur" {
			t.Errorf("first spell should NOT get Alisaie discount; mod=%+v", m)
		}
	}
}

func TestAlisaie_ThirdSpellGetsNoDiscount(t *testing.T) {
	gs := newGame(t, 2)
	stampCreaturePT(addPerm(gs, 0, "Alisaie Leveilleur", "creature", "legendary"), 2, 2)
	gs.Seats[0].Turn.SpellsCast = 2

	spell := &gameengine.Card{Name: "Doom Blade", Owner: 0, Types: []string{"instant"}}
	mods := gameengine.ScanCostModifiers(gs, spell, 0)
	for _, m := range mods {
		if m.Source == "Alisaie Leveilleur" {
			t.Errorf("third spell should NOT get Alisaie discount; mod=%+v", m)
		}
	}
}

// ---------------------------------------------------------------------------
// Cecily, Haunted Mage — attack trigger
// ---------------------------------------------------------------------------

func TestCecily_ETBStampsMaxHandSize11(t *testing.T) {
	gs := newGame(t, 2)
	cecily := stampCreaturePT(addPerm(gs, 0, "Cecily, Haunted Mage", "creature", "legendary"), 3, 4)
	cecilyHauntedMageETB(gs, cecily)
	if got := gs.Seats[0].Flags["max_hand_size"]; got != 11 {
		t.Errorf("expected max_hand_size = 11, got %d", got)
	}
}

func TestCecily_AttackDrawsAndLosesLife(t *testing.T) {
	gs := newGame(t, 2)
	cecily := stampCreaturePT(addPerm(gs, 0, "Cecily, Haunted Mage", "creature", "legendary"), 3, 4)
	addLibrary(gs, 0, "Drawn")
	preLife := gs.Seats[0].Life

	cecilyHauntedMageAttack(gs, cecily, map[string]interface{}{
		"attacker_perm": cecily,
		"attacker_seat": 0,
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected to draw 1, hand size = %d", len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].Life != preLife-1 {
		t.Errorf("expected -1 life; %d → %d", preLife, gs.Seats[0].Life)
	}
}

func TestCecily_AttackEmits11PlusFreeCastBreadcrumb(t *testing.T) {
	gs := newGame(t, 2)
	cecily := stampCreaturePT(addPerm(gs, 0, "Cecily, Haunted Mage", "creature", "legendary"), 3, 4)
	addLibrary(gs, 0, "Drawn")
	// Pre-stack 11 cards so post-draw the hand reaches 12 ≥ 11.
	for i := 0; i < 11; i++ {
		gs.Seats[0].Hand = append(gs.Seats[0].Hand, &gameengine.Card{Name: "Filler", Owner: 0})
	}

	prePartial := hasEvent(gs, "per_card_partial")
	cecilyHauntedMageAttack(gs, cecily, map[string]interface{}{
		"attacker_perm": cecily,
		"attacker_seat": 0,
	})
	if hasEvent(gs, "per_card_partial") <= prePartial {
		t.Errorf("expected per_card_partial breadcrumb when hand ≥ 11")
	}
}

func TestCecily_AttackIgnoresOtherAttackers(t *testing.T) {
	gs := newGame(t, 2)
	cecily := stampCreaturePT(addPerm(gs, 0, "Cecily, Haunted Mage", "creature", "legendary"), 3, 4)
	other := stampCreaturePT(addPerm(gs, 0, "Llanowar Elves", "creature"), 1, 1)
	addLibrary(gs, 0, "Top")

	cecilyHauntedMageAttack(gs, cecily, map[string]interface{}{
		"attacker_perm": other,
		"attacker_seat": 0,
	})
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("non-Cecily attacker should NOT trigger; hand=%d", len(gs.Seats[0].Hand))
	}
}

// ---------------------------------------------------------------------------
// Extus, Oriq Overlord — Magecraft return
// ---------------------------------------------------------------------------

func TestExtus_MagecraftReturnsBestNonlegendaryCreature(t *testing.T) {
	gs := newGame(t, 2)
	extus := stampCreaturePT(addPerm(gs, 0, "Extus, Oriq Overlord // Awaken the Blood Avatar", "creature", "legendary"), 5, 5)

	low := &gameengine.Card{Name: "Mouse", Owner: 0, CMC: 1, Types: []string{"creature", "cmc:1"}}
	high := &gameengine.Card{Name: "Wurm", Owner: 0, CMC: 6, Types: []string{"creature", "cmc:6"}}
	legendary := &gameengine.Card{Name: "Yuriko", Owner: 0, CMC: 3, Types: []string{"creature", "legendary", "cmc:3"}}
	noncreat := &gameengine.Card{Name: "Bolt", Owner: 0, Types: []string{"instant"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, low, high, legendary, noncreat)

	extusMagecraftReturnCreature(gs, extus, map[string]interface{}{
		"caster_seat": 0,
	})

	// High-CMC nonlegendary should be in hand.
	inHand := false
	for _, c := range gs.Seats[0].Hand {
		if c == high {
			inHand = true
			break
		}
	}
	if !inHand {
		t.Fatalf("highest-CMC nonlegendary creature should be returned to hand; hand=%v", hand_names(gs.Seats[0].Hand))
	}
	// Legendary and noncreature should remain in graveyard.
	gyHas := func(c *gameengine.Card) bool {
		for _, x := range gs.Seats[0].Graveyard {
			if x == c {
				return true
			}
		}
		return false
	}
	if !gyHas(legendary) {
		t.Errorf("legendary creature should remain in graveyard")
	}
	if !gyHas(noncreat) {
		t.Errorf("instant should remain in graveyard")
	}
}

func TestExtus_MagecraftSkipsWhenOpponentCasts(t *testing.T) {
	gs := newGame(t, 2)
	extus := stampCreaturePT(addPerm(gs, 0, "Extus, Oriq Overlord // Awaken the Blood Avatar", "creature", "legendary"), 5, 5)
	c := &gameengine.Card{Name: "Bear", Owner: 0, Types: []string{"creature"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, c)

	extusMagecraftReturnCreature(gs, extus, map[string]interface{}{
		"caster_seat": 1, // opponent's cast
	})
	for _, x := range gs.Seats[0].Hand {
		if x == c {
			t.Errorf("opponent's cast should NOT trigger Extus's Magecraft")
		}
	}
}

// ---------------------------------------------------------------------------
// Jaxis, the Troublemaker — copy-creature activated
// ---------------------------------------------------------------------------

func TestJaxis_ActivateCopiesTargetCreatureWithHaste(t *testing.T) {
	gs := newGame(t, 2)
	jaxis := stampCreaturePT(addPerm(gs, 0, "Jaxis, the Troublemaker", "creature", "legendary"), 2, 2)
	target := stampCreaturePT(addPerm(gs, 0, "Big Beast", "creature"), 6, 6)

	preBF := len(gs.Seats[0].Battlefield)
	jaxisActivate(gs, jaxis, 0, map[string]interface{}{
		"creature_perm": target,
	})

	if got := len(gs.Seats[0].Battlefield); got != preBF+1 {
		t.Fatalf("expected +1 token; bf delta=%d", got-preBF)
	}
	var tok *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p == jaxis || p == target {
			continue
		}
		if p.Card != nil && p.Flags["jaxis_token"] == 1 {
			tok = p
			break
		}
	}
	if tok == nil {
		t.Fatal("Jaxis token not found on battlefield")
	}
	if tok.Card.BasePower != 6 || tok.Card.BaseToughness != 6 {
		t.Errorf("token P/T = %d/%d, want 6/6 (copy of target)", tok.Card.BasePower, tok.Card.BaseToughness)
	}
	if tok.Flags["kw:haste"] != 1 {
		t.Errorf("token should have haste flag")
	}
	// Delayed sacrifice should be queued.
	if len(gs.DelayedTriggers) < 1 {
		t.Error("expected next_end_step sacrifice trigger queued")
	}
}

func TestJaxis_TokenDieDrawsCard(t *testing.T) {
	gs := newGame(t, 2)
	jaxis := stampCreaturePT(addPerm(gs, 0, "Jaxis, the Troublemaker", "creature", "legendary"), 2, 2)
	addLibrary(gs, 0, "Drawn")

	// Fake dying perm with jaxis_token flag.
	dyingCard := &gameengine.Card{Name: "Big Beast", Owner: 0, Types: []string{"creature"}}
	dyingPerm := &gameengine.Permanent{
		Card:       dyingCard,
		Owner:      0,
		Controller: 0,
		Counters:   map[string]int{},
		Flags:      map[string]int{"jaxis_token": 1},
	}

	jaxisTokenDiesDraw(gs, jaxis, map[string]interface{}{
		"perm":            dyingPerm,
		"card":            dyingCard,
		"controller_seat": 0,
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("token death should draw 1 card; hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestJaxis_NonJaxisTokenDeathNoDraw(t *testing.T) {
	gs := newGame(t, 2)
	jaxis := stampCreaturePT(addPerm(gs, 0, "Jaxis, the Troublemaker", "creature", "legendary"), 2, 2)
	addLibrary(gs, 0, "Top")

	dyingCard := &gameengine.Card{Name: "Bear", Owner: 0, Types: []string{"creature"}}
	dyingPerm := &gameengine.Permanent{
		Card:       dyingCard,
		Owner:      0,
		Controller: 0,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	jaxisTokenDiesDraw(gs, jaxis, map[string]interface{}{
		"perm":            dyingPerm,
		"card":            dyingCard,
		"controller_seat": 0,
	})
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("non-Jaxis-token death should NOT draw; hand=%d", len(gs.Seats[0].Hand))
	}
}

// ---------------------------------------------------------------------------
// Fire Lord Zuko — firebending X + exile-cast/etb counter
// ---------------------------------------------------------------------------

func TestZuko_FirebendingAddsPowerInRed(t *testing.T) {
	gs := newGame(t, 2)
	zuko := stampCreaturePT(addPerm(gs, 0, "Fire Lord Zuko", "creature", "legendary"), 5, 5)
	preMana := gs.Seats[0].ManaPool

	fireLordZukoFirebending(gs, zuko, map[string]interface{}{
		"attacker_perm": zuko,
		"attacker_seat": 0,
	})

	if got := gs.Seats[0].ManaPool; got != preMana+5 {
		t.Errorf("firebending should add power-many mana; mana %d → %d", preMana, got)
	}
}

func TestZuko_FirebendingIgnoresOtherAttackers(t *testing.T) {
	gs := newGame(t, 2)
	zuko := stampCreaturePT(addPerm(gs, 0, "Fire Lord Zuko", "creature", "legendary"), 5, 5)
	other := stampCreaturePT(addPerm(gs, 0, "Llanowar Elves", "creature"), 1, 1)
	preMana := gs.Seats[0].ManaPool

	fireLordZukoFirebending(gs, zuko, map[string]interface{}{
		"attacker_perm": other,
		"attacker_seat": 0,
	})
	if got := gs.Seats[0].ManaPool; got != preMana {
		t.Errorf("firebending should only fire on Zuko's attack; mana %d → %d", preMana, got)
	}
}

func TestZuko_CastFromExileStampsCounterOnEachCreature(t *testing.T) {
	gs := newGame(t, 2)
	zuko := stampCreaturePT(addPerm(gs, 0, "Fire Lord Zuko", "creature", "legendary"), 5, 5)
	c1 := stampCreaturePT(addPerm(gs, 0, "Bear A", "creature"), 2, 2)
	c2 := stampCreaturePT(addPerm(gs, 0, "Bear B", "creature"), 3, 3)
	stampCreaturePT(addPerm(gs, 0, "Sol Ring", "artifact"), 0, 0)

	fireLordZukoSpellFromExile(gs, zuko, map[string]interface{}{
		"caster_seat": 0,
		"cast_zone":   "exile",
	})

	if c1.Counters["+1/+1"] != 1 || c2.Counters["+1/+1"] != 1 {
		t.Errorf("expected +1/+1 on each creature; c1=%d c2=%d",
			c1.Counters["+1/+1"], c2.Counters["+1/+1"])
	}
	if zuko.Counters["+1/+1"] != 1 {
		t.Errorf("Zuko himself is a creature controller controls; expected +1/+1, got %d",
			zuko.Counters["+1/+1"])
	}
}

func TestZuko_CastFromHandDoesNotStampCounter(t *testing.T) {
	gs := newGame(t, 2)
	zuko := stampCreaturePT(addPerm(gs, 0, "Fire Lord Zuko", "creature", "legendary"), 5, 5)
	c1 := stampCreaturePT(addPerm(gs, 0, "Bear A", "creature"), 2, 2)

	fireLordZukoSpellFromExile(gs, zuko, map[string]interface{}{
		"caster_seat": 0,
		"cast_zone":   "hand",
	})
	if c1.Counters["+1/+1"] != 0 {
		t.Errorf("hand-cast should NOT stamp counters; got %d", c1.Counters["+1/+1"])
	}
}

func TestZuko_ETBFromExileStampsCounter(t *testing.T) {
	gs := newGame(t, 2)
	zuko := stampCreaturePT(addPerm(gs, 0, "Fire Lord Zuko", "creature", "legendary"), 5, 5)
	c1 := stampCreaturePT(addPerm(gs, 0, "Bear A", "creature"), 2, 2)
	entering := stampCreaturePT(addPerm(gs, 0, "Returning Threat", "creature"), 4, 4)

	fireLordZukoEtbFromExile(gs, zuko, map[string]interface{}{
		"perm":            entering,
		"controller_seat": 0,
		"from_zone":       "exile",
	})

	if c1.Counters["+1/+1"] != 1 {
		t.Errorf("ETB from exile should stamp +1/+1 on existing creatures; c1=%d",
			c1.Counters["+1/+1"])
	}
}

func TestZuko_ETBFromGraveyardDoesNotStamp(t *testing.T) {
	gs := newGame(t, 2)
	zuko := stampCreaturePT(addPerm(gs, 0, "Fire Lord Zuko", "creature", "legendary"), 5, 5)
	c1 := stampCreaturePT(addPerm(gs, 0, "Bear A", "creature"), 2, 2)
	entering := stampCreaturePT(addPerm(gs, 0, "Reanimated", "creature"), 4, 4)

	fireLordZukoEtbFromExile(gs, zuko, map[string]interface{}{
		"perm":            entering,
		"controller_seat": 0,
		"from_zone":       "graveyard",
	})
	if c1.Counters["+1/+1"] != 0 {
		t.Errorf("graveyard ETB should NOT stamp; got %d", c1.Counters["+1/+1"])
	}
}
