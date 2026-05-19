package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R42 stub-batch ports — five gen_*.go pure-stub handlers ported into
// real per-card behaviour. Avoids the r36 set (Ashling Flame Dancer,
// Magnus the Red, Morlun, Tannuk, Toph Hardheaded Teacher), the r37
// set (Old One Eye, Lara Croft, Maha, Norman Osborn, Thrun), and the
// r41 set (Cleopatra, Iron Man, Rakdos, The Locust God, Yorion). All
// five picks appear in data/decks/moxfield_300/ as deck commanders.
//
//   - Shiko and Narset, Unified:    flurry second-spell → copy/draw
//   - Rosnakht, Heir of Rohgahh:    heroic → 0/1 red Kobold token
//   - Jon Irenicus, Shattered One:  end_step donate + own/not-control draw
//   - Zhulodok, Void Gorger:        7+MV colorless cast → double cascade
//   - The Second Doctor:            no-max-hand-size + end_step "may draw"

// ---------------------------------------------------------------------------
// Shiko and Narset, Unified
// ---------------------------------------------------------------------------

func TestShikoNarset_FlurryCopyBranchOnSecondInstantOrSorcery(t *testing.T) {
	gs := newGame(t, 2)
	shiko := stampCreaturePT(addPerm(gs, 0, "Shiko and Narset, Unified", "creature", "legendary"), 3, 3)
	gs.Seats[0].Flags = map[string]int{"spells_cast_this_turn": 2}

	spell := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	shikoNarsetFlurry(gs, shiko, map[string]interface{}{
		"caster_seat": 0,
		"spell_name":  "Lightning Bolt",
		"card":        spell,
		"is_creature": false,
	})
	if hasEvent(gs, "per_card_handler") < 1 {
		t.Fatalf("expected per_card_handler event for copy branch; events=%+v", gs.EventLog)
	}
	if hasEvent(gs, "per_card_partial") < 1 {
		t.Errorf("copy branch should emit a partial breadcrumb (spell-copy pipeline not modeled)")
	}
}

func TestShikoNarset_FlurryDrawBranchOnSecondCreatureSpell(t *testing.T) {
	gs := newGame(t, 2)
	shiko := stampCreaturePT(addPerm(gs, 0, "Shiko and Narset, Unified", "creature", "legendary"), 3, 3)
	gs.Seats[0].Flags = map[string]int{"spells_cast_this_turn": 2}
	addLibrary(gs, 0, "Counterspell")
	preHand := len(gs.Seats[0].Hand)

	spell := &gameengine.Card{Name: "Grizzly Bears", Owner: 0, Types: []string{"creature"}}
	shikoNarsetFlurry(gs, shiko, map[string]interface{}{
		"caster_seat": 0,
		"spell_name":  "Grizzly Bears",
		"card":        spell,
		"is_creature": true,
	})
	if got := len(gs.Seats[0].Hand) - preHand; got != 1 {
		t.Fatalf("flurry draw branch should draw 1, got delta %d", got)
	}
}

func TestShikoNarset_DoesNotFireOnFirstSpell(t *testing.T) {
	gs := newGame(t, 2)
	shiko := stampCreaturePT(addPerm(gs, 0, "Shiko and Narset, Unified", "creature", "legendary"), 3, 3)
	gs.Seats[0].Flags = map[string]int{"spells_cast_this_turn": 1}
	addLibrary(gs, 0, "Counterspell")

	spell := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	shikoNarsetFlurry(gs, shiko, map[string]interface{}{
		"caster_seat": 0,
		"spell_name":  "Lightning Bolt",
		"card":        spell,
		"is_creature": false,
	})
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("should not draw on first spell of turn")
	}
}

func TestShikoNarset_IgnoresOpponentCasts(t *testing.T) {
	gs := newGame(t, 2)
	shiko := stampCreaturePT(addPerm(gs, 0, "Shiko and Narset, Unified", "creature", "legendary"), 3, 3)
	gs.Seats[1].Flags = map[string]int{"spells_cast_this_turn": 2}
	addLibrary(gs, 0, "Counterspell")

	spell := &gameengine.Card{Name: "Lightning Bolt", Owner: 1, Types: []string{"instant"}}
	shikoNarsetFlurry(gs, shiko, map[string]interface{}{
		"caster_seat": 1,
		"spell_name":  "Lightning Bolt",
		"card":        spell,
		"is_creature": false,
	})
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("flurry must not fire on opponent's cast")
	}
}

// ---------------------------------------------------------------------------
// Rosnakht, Heir of Rohgahh
// ---------------------------------------------------------------------------

func TestRosnakht_HeroicMintsKoboldToken(t *testing.T) {
	gs := newGame(t, 2)
	ros := stampCreaturePT(addPerm(gs, 0, "Rosnakht, Heir of Rohgahh", "creature", "kobold", "legendary"), 1, 2)
	preBF := len(gs.Seats[0].Battlefield)

	bolt := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	rosnakhtHeroicSpawn(gs, ros, map[string]interface{}{
		"source":      ros,
		"spell":       bolt,
		"caster_seat": 0,
	})

	if got := len(gs.Seats[0].Battlefield) - preBF; got != 1 {
		t.Fatalf("heroic should mint exactly 1 token, got delta %d", got)
	}
	var tok *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p == ros || p.Card == nil {
			continue
		}
		if p.Card.Name == "Kobolds of Kher Keep" {
			tok = p
			break
		}
	}
	if tok == nil {
		t.Fatal("Kobolds of Kher Keep token not found")
	}
	if tok.Card.BasePower != 0 || tok.Card.BaseToughness != 1 {
		t.Errorf("kobold token P/T = %d/%d, want 0/1",
			tok.Card.BasePower, tok.Card.BaseToughness)
	}
	hasR := false
	for _, c := range tok.Card.Colors {
		if c == "R" {
			hasR = true
		}
	}
	if !hasR {
		t.Errorf("kobold token colors = %v, want includes R", tok.Card.Colors)
	}
}

func TestRosnakht_IgnoresHeroicForOtherSource(t *testing.T) {
	gs := newGame(t, 2)
	ros := stampCreaturePT(addPerm(gs, 0, "Rosnakht, Heir of Rohgahh", "creature", "kobold", "legendary"), 1, 2)
	other := addPerm(gs, 0, "Other Hero", "creature")
	preBF := len(gs.Seats[0].Battlefield)

	rosnakhtHeroicSpawn(gs, ros, map[string]interface{}{
		"source":      other,
		"spell":       &gameengine.Card{Name: "X"},
		"caster_seat": 0,
	})
	if got := len(gs.Seats[0].Battlefield) - preBF; got != 0 {
		t.Errorf("rosnakht should not fire for a different heroic source, got delta %d", got)
	}
}

// ---------------------------------------------------------------------------
// Jon Irenicus, Shattered One
// ---------------------------------------------------------------------------

func TestJonIrenicus_EndStepDonatesWeakestCreature(t *testing.T) {
	gs := newGame(t, 3)
	jon := stampCreaturePT(addPerm(gs, 0, "Jon Irenicus, Shattered One", "creature", "legendary"), 3, 4)
	// Multiple gift candidates — lowest-power should be picked.
	weak := stampCreaturePT(addPerm(gs, 0, "Servo", "creature"), 1, 1)
	stampCreaturePT(addPerm(gs, 0, "Hero", "creature"), 4, 4)
	// Opponents — seat 1 healthier, seat 2 weaker, expect seat 2.
	gs.Seats[1].Life = 30
	gs.Seats[2].Life = 12

	jonIrenicusDonate(gs, jon, map[string]interface{}{"active_seat": 0})

	if weak.Counters["+1/+1"] != 2 {
		t.Errorf("weak should have 2 +1/+1 counters, got %d", weak.Counters["+1/+1"])
	}
	if !weak.Tapped {
		t.Errorf("donated creature should be tapped")
	}
	if !gameengine.IsGoaded(weak, gs.Turn) {
		t.Errorf("donated creature should be goaded")
	}
	if weak.Flags["cant_be_sacrificed"] != 1 {
		t.Errorf("donated creature should have cant_be_sacrificed flag set")
	}
	if hasEvent(gs, "per_card_handler") < 1 {
		t.Errorf("expected per_card_handler event")
	}
	// Verify the partial breadcrumb fired (control change side).
	if hasEvent(gs, "per_card_partial") < 1 {
		t.Errorf("expected per_card_partial breadcrumb for control change")
	}
}

func TestJonIrenicus_EndStepNoOpForOpponentTurn(t *testing.T) {
	gs := newGame(t, 2)
	jon := stampCreaturePT(addPerm(gs, 0, "Jon Irenicus, Shattered One", "creature", "legendary"), 3, 4)
	gift := stampCreaturePT(addPerm(gs, 0, "Servo", "creature"), 1, 1)

	jonIrenicusDonate(gs, jon, map[string]interface{}{"active_seat": 1})
	if gift.Counters["+1/+1"] != 0 {
		t.Errorf("must not fire on opponent's end step; counters=%d", gift.Counters["+1/+1"])
	}
}

func TestJonIrenicus_DrawWhenOwnedButNotControlledAttacks(t *testing.T) {
	gs := newGame(t, 2)
	jon := stampCreaturePT(addPerm(gs, 0, "Jon Irenicus, Shattered One", "creature", "legendary"), 3, 4)
	addLibrary(gs, 0, "Card A")
	// Build an "owned by 0 but controlled by 1" attacker.
	atk := stampCreaturePT(addPerm(gs, 1, "Donated Goon", "creature"), 3, 3)
	atk.Owner = 0
	preHand := len(gs.Seats[0].Hand)

	jonIrenicusOwnNotControlDraw(gs, jon, map[string]interface{}{
		"attacker_perm": atk,
		"attacker_seat": 1,
		"attacker_card": atk.Card,
	})
	if got := len(gs.Seats[0].Hand) - preHand; got != 1 {
		t.Fatalf("expected 1 draw when own-but-not-controlled creature attacks, got delta %d", got)
	}
}

func TestJonIrenicus_NoDrawWhenOwnAndControlled(t *testing.T) {
	gs := newGame(t, 2)
	jon := stampCreaturePT(addPerm(gs, 0, "Jon Irenicus, Shattered One", "creature", "legendary"), 3, 4)
	addLibrary(gs, 0, "Card A")
	atk := stampCreaturePT(addPerm(gs, 0, "Goon", "creature"), 3, 3)
	atk.Owner = 0
	preHand := len(gs.Seats[0].Hand)

	jonIrenicusOwnNotControlDraw(gs, jon, map[string]interface{}{
		"attacker_perm": atk,
		"attacker_seat": 0,
		"attacker_card": atk.Card,
	})
	if got := len(gs.Seats[0].Hand) - preHand; got != 0 {
		t.Errorf("own + controlled attack should NOT draw; got delta %d", got)
	}
}

// ---------------------------------------------------------------------------
// Zhulodok, Void Gorger
// ---------------------------------------------------------------------------

func TestZhulodok_DoubleCascadesOnColorless7MVCastFromHand(t *testing.T) {
	gs := newGame(t, 2)
	zhul := stampCreaturePT(addPerm(gs, 0, "Zhulodok, Void Gorger", "creature", "legendary", "eldrazi"), 6, 6)
	// Stack the library: nonland with CMC < 7 should be exiled+free-cast.
	smallSpell := &gameengine.Card{
		Name: "Filler Sorcery", Owner: 0,
		Types: []string{"sorcery", "cmc:3"},
	}
	smallSpell2 := &gameengine.Card{
		Name: "Filler Instant", Owner: 0,
		Types: []string{"instant", "cmc:2"},
	}
	gs.Seats[0].Library = append(gs.Seats[0].Library, smallSpell, smallSpell2)

	cast := &gameengine.Card{
		Name:  "Kozilek, the Great Distortion",
		Owner: 0,
		Types: []string{"creature", "eldrazi", "cmc:10"},
	}
	zhulodokDoubleCascade(gs, zhul, map[string]interface{}{
		"caster_seat": 0,
		"spell_name":  cast.DisplayName(),
		"card":        cast,
		"cast_zone":   "hand",
	})

	// Two cascade_trigger events should have fired.
	if got := hasEvent(gs, "cascade_trigger"); got != 2 {
		t.Fatalf("expected 2 cascade_trigger events, got %d", got)
	}
}

func TestZhulodok_DoesNotFireOnColoredSpell(t *testing.T) {
	gs := newGame(t, 2)
	zhul := stampCreaturePT(addPerm(gs, 0, "Zhulodok, Void Gorger", "creature", "legendary", "eldrazi"), 6, 6)
	gs.Seats[0].Library = append(gs.Seats[0].Library, &gameengine.Card{
		Name: "x", Owner: 0, Types: []string{"sorcery", "cmc:1"},
	})
	cast := &gameengine.Card{
		Name:   "Big Red Dragon",
		Owner:  0,
		Types:  []string{"creature", "cmc:8"},
		Colors: []string{"R"},
	}
	zhulodokDoubleCascade(gs, zhul, map[string]interface{}{
		"caster_seat": 0,
		"spell_name":  cast.DisplayName(),
		"card":        cast,
		"cast_zone":   "hand",
	})
	if hasEvent(gs, "cascade_trigger") != 0 {
		t.Errorf("colored 8-MV spell must not trigger Zhulodok; cascades=%d",
			hasEvent(gs, "cascade_trigger"))
	}
}

func TestZhulodok_DoesNotFireBelowSevenMV(t *testing.T) {
	gs := newGame(t, 2)
	zhul := stampCreaturePT(addPerm(gs, 0, "Zhulodok, Void Gorger", "creature", "legendary", "eldrazi"), 6, 6)
	gs.Seats[0].Library = append(gs.Seats[0].Library, &gameengine.Card{
		Name: "x", Owner: 0, Types: []string{"sorcery", "cmc:1"},
	})
	cast := &gameengine.Card{
		Name:  "Borderline Colorless",
		Owner: 0,
		Types: []string{"creature", "cmc:6"},
	}
	zhulodokDoubleCascade(gs, zhul, map[string]interface{}{
		"caster_seat": 0,
		"spell_name":  cast.DisplayName(),
		"card":        cast,
		"cast_zone":   "hand",
	})
	if hasEvent(gs, "cascade_trigger") != 0 {
		t.Errorf("6-MV colorless cast must not trigger Zhulodok")
	}
}

func TestZhulodok_DoesNotFireFromNonHandZone(t *testing.T) {
	gs := newGame(t, 2)
	zhul := stampCreaturePT(addPerm(gs, 0, "Zhulodok, Void Gorger", "creature", "legendary", "eldrazi"), 6, 6)
	gs.Seats[0].Library = append(gs.Seats[0].Library, &gameengine.Card{
		Name: "x", Owner: 0, Types: []string{"sorcery", "cmc:1"},
	})
	cast := &gameengine.Card{
		Name:  "Reanimated Eldrazi",
		Owner: 0,
		Types: []string{"creature", "cmc:10"},
	}
	zhulodokDoubleCascade(gs, zhul, map[string]interface{}{
		"caster_seat": 0,
		"spell_name":  cast.DisplayName(),
		"card":        cast,
		"cast_zone":   "graveyard",
	})
	if hasEvent(gs, "cascade_trigger") != 0 {
		t.Errorf("graveyard-cast must not trigger Zhulodok")
	}
}

// ---------------------------------------------------------------------------
// The Second Doctor
// ---------------------------------------------------------------------------

func TestSecondDoctor_ETBStampsNoMaxHandSizeOnEverySeat(t *testing.T) {
	gs := newGame(t, 4)
	doc := addPerm(gs, 0, "The Second Doctor", "creature", "legendary")
	theSecondDoctorETB(gs, doc)
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		if s.Flags["no_max_hand_size"] != 1 {
			t.Errorf("seat %d should have no_max_hand_size=1, got %d",
				i, s.Flags["no_max_hand_size"])
		}
	}
}

func TestSecondDoctor_EndStepEveryoneDrawsAndOppGetsRestrictionFlag(t *testing.T) {
	gs := newGame(t, 3)
	doc := addPerm(gs, 0, "The Second Doctor", "creature", "legendary")
	addLibrary(gs, 0, "Card 0")
	addLibrary(gs, 1, "Card 1")
	addLibrary(gs, 2, "Card 2")

	theSecondDoctorHowCivilOfYou(gs, doc, map[string]interface{}{"active_seat": 0})

	for i := 0; i < 3; i++ {
		if len(gs.Seats[i].Hand) != 1 {
			t.Errorf("seat %d should have drawn 1, got hand size %d",
				i, len(gs.Seats[i].Hand))
		}
	}
	// Opponents who drew get the restriction flag; controller does not.
	for i := 1; i < 3; i++ {
		if gs.Seats[i].Flags["cant_attack_doctor_controller"] != 1 {
			t.Errorf("opponent %d should have cant_attack_doctor_controller=1 (controller_seat+1), got %d",
				i, gs.Seats[i].Flags["cant_attack_doctor_controller"])
		}
		if gs.Seats[i].Flags["cant_attack_doctor_controller_until_turn"] != gs.Turn+2 {
			t.Errorf("opponent %d expected until_turn=%d, got %d",
				i, gs.Turn+2, gs.Seats[i].Flags["cant_attack_doctor_controller_until_turn"])
		}
	}
	if gs.Seats[0].Flags["cant_attack_doctor_controller"] != 0 {
		t.Errorf("controller should NOT have the restriction flag set on themselves")
	}
}

func TestSecondDoctor_EndStepNoOpOnOpponentTurn(t *testing.T) {
	gs := newGame(t, 3)
	doc := addPerm(gs, 0, "The Second Doctor", "creature", "legendary")
	addLibrary(gs, 0, "Card 0")

	theSecondDoctorHowCivilOfYou(gs, doc, map[string]interface{}{"active_seat": 1})
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("must not fire on opponent's end step")
	}
}
