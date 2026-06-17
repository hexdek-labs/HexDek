package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Live-grinder r63: dynamic-count X effects ("deals/puts/creates X …, where X
// is the number of …") under-resolved to 0/1 because the bare "x" amount read
// gs.Flags["x"] (the cast {X}, which is 0 for these cards) and the parser left
// the "where X is <count>" clause only on the source ability's Raw. These
// regressions pin all four reported cards. They build the permanent with its
// real AST (Triggered/effect + Raw clause) — the same shape the loader
// produces — and resolve with NO cast X preset (the real-game condition).

func dxCreature(gs *GameState, seat int, name string, types ...string) *Permanent {
	tl := append([]string{"creature"}, types...)
	p := &Permanent{
		Card:       &Card{Name: name, Types: tl, BasePower: 1, BaseToughness: 1},
		Controller: seat, Owner: seat,
		Counters: map[string]int{}, Flags: map[string]int{}, Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// Thundering Sparkmage — ETB deals X damage, X = creatures in your party.
func TestDynamicX_ThunderingSparkmage(t *testing.T) {
	gs := newTestGameState(2)
	// A 3-role party (cleric, rogue, warrior); source is a plain creature so it
	// doesn't add a 4th role.
	dxCreature(gs, 0, "Cl", "cleric")
	dxCreature(gs, 0, "Ro", "rogue")
	dxCreature(gs, 0, "Wa", "warrior")
	victim := dxCreature(gs, 1, "Victim")
	victim.Card.BaseToughness = 10
	eff := &gameast.Damage{Amount: *gameast.NumStr("x"), Target: gameast.Filter{Base: "creature", Targeted: true}}
	src := &Permanent{
		Card: &Card{Name: "Thundering Sparkmage", Types: []string{"creature", "human"},
			AST: &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Triggered{
				Effect: eff,
				Raw:    "when this creature enters, it deals x damage to target creature, where x is the number of creatures in your party",
			}}}},
		Controller: 0, Owner: 0, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)

	if got := CountParty(gs, 0); got != 3 {
		t.Fatalf("setup: party = %d, want 3", got)
	}
	ResolveEffect(gs, src, eff)
	if victim.MarkedDamage != 3 {
		t.Fatalf("Sparkmage should deal X=party=3 damage, marked %d", victim.MarkedDamage)
	}
}

// Priest of the Crossing — end step: put X +1/+1 on each creature you control,
// X = creatures that died under your control this turn.
func TestDynamicX_PriestOfTheCrossing(t *testing.T) {
	gs := newTestGameState(2)
	gs.Seats[0].Turn.CreaturesDied = 3
	a := dxCreature(gs, 0, "AllyA")
	b := dxCreature(gs, 0, "AllyB")
	eff := &gameast.CounterMod{Op: "put", Count: *gameast.NumStr("x"), CounterKind: "+1/+1",
		Target: gameast.Filter{Base: "creature", Quantifier: "each", YouControl: true}}
	src := &Permanent{
		Card: &Card{Name: "Priest of the Crossing", Types: []string{"creature"},
			AST: &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Triggered{
				Effect: eff,
				Raw:    "at the beginning of each end step, put x +1/+1 counters on each creature you control, where x is the number of creatures that died under your control this turn",
			}}}},
		Controller: 0, Owner: 0, Counters: map[string]int{}, Flags: map[string]int{}, Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)

	ResolveEffect(gs, src, eff)
	if a.Counters["+1/+1"] != 3 || b.Counters["+1/+1"] != 3 {
		t.Fatalf("Priest should put X=died=3 +1/+1 on each ally, got A=%d B=%d", a.Counters["+1/+1"], b.Counters["+1/+1"])
	}
}

// Siegfried, Famed Swordsman — ETB: mill 3, then put X +1/+1 on self,
// X = twice the number of creature cards in your graveyard (counted AFTER the
// mill, so the where-clause must resolve live).
func TestDynamicX_SiegfriedFamedSwordsman(t *testing.T) {
	gs := newTestGameState(2)
	// Library: 2 creatures + 1 noncreature on top → mill 3 puts 2 creatures in GY.
	gs.Seats[0].Library = append(gs.Seats[0].Library,
		&Card{Name: "Bear", Types: []string{"creature"}, Owner: 0},
		&Card{Name: "Wolf", Types: []string{"creature"}, Owner: 0},
		&Card{Name: "Bolt", Types: []string{"instant"}, Owner: 0},
		&Card{Name: "Filler", Types: []string{"land"}, Owner: 0},
	)
	seq := &gameast.Sequence{Items: []gameast.Effect{
		&gameast.Mill{Count: *gameast.NumInt(3), Target: gameast.Filter{Base: "self"}},
		&gameast.CounterMod{Op: "put", Count: *gameast.NumStr("x"), CounterKind: "+1/+1", Target: gameast.Filter{Base: "self"}},
	}}
	src := &Permanent{
		Card: &Card{Name: "Siegfried, Famed Swordsman", Types: []string{"creature"},
			AST: &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Triggered{
				Effect: seq,
				Raw:    "when siegfried enters, mill three cards. then put x +1/+1 counters on siegfried, where x is twice the number of creature cards in your graveyard",
			}}}},
		Controller: 0, Owner: 0, Counters: map[string]int{}, Flags: map[string]int{}, Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)

	ResolveEffect(gs, src, seq)
	// 2 creature cards milled → X = 2 × 2 = 4.
	if got := src.Counters["+1/+1"]; got != 4 {
		t.Fatalf("Siegfried: want X=2×(2 GY creatures)=4 +1/+1 counters, got %d", got)
	}
}

// Defenders of Humanity — {X}{2}{W} enchantment whose ETB creates X 2/2
// tokens, where X is the cast X. The permanent stamps perm.Flags["chosen_x"];
// the ETB-trigger effect must read it (it resolves in a later frame where
// gs.Flags["x"] is 0).
func TestDynamicX_DefendersOfHumanity_CastX(t *testing.T) {
	gs := newTestGameState(2)
	eff := &gameast.CreateToken{Count: *gameast.NumStr("x"), PT: &[2]int{2, 2},
		Types: []string{"creature", "astartes", "warrior"}, Color: []string{"W"}, Keywords: []string{"vigilance"}}
	src := &Permanent{
		Card: &Card{Name: "Defenders of Humanity", Types: []string{"enchantment"},
			AST: &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Triggered{
				Effect: eff,
				Raw:    "when this enchantment enters, create x 2/2 white astartes warrior creature tokens with vigilance",
			}}}},
		Controller: 0, Owner: 0, Flags: map[string]int{"chosen_x": 3}, Counters: map[string]int{}, Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)

	before := len(gs.Seats[0].Battlefield)
	ResolveEffect(gs, src, eff)
	created := len(gs.Seats[0].Battlefield) - before
	if created != 3 {
		t.Fatalf("Defenders cast for X=3 should create 3 tokens, created %d", created)
	}
}

// Voja, Jaws of the Conclave — attack trigger: put X +1/+1 on each creature you
// control, X = the number of ELVES you control (a subtype count). Two prior
// defects: (1) the basis was matched against the WHOLE ability Raw, so the
// generic "creatures you control" case grabbed the effect's TARGET clause
// ("on each creature you control") and counted ALL creatures (5) instead of the
// elves (3); (2) subtype counts weren't recognized at all. Voja itself is a
// Wolf, so it must NOT be counted among the elves.
func TestDynamicX_VojaSubtypeCount(t *testing.T) {
	gs := newTestGameState(2)
	dxCreature(gs, 0, "Elf A", "elf")
	dxCreature(gs, 0, "Elf B", "elf")
	dxCreature(gs, 0, "Elf C", "elf")
	ally := dxCreature(gs, 0, "Bear") // non-elf creature you control
	eff := &gameast.CounterMod{Op: "put", Count: *gameast.NumStr("x"), CounterKind: "+1/+1",
		Target: gameast.Filter{Base: "creature", Quantifier: "each", YouControl: true}}
	src := &Permanent{
		Card: &Card{Name: "Voja, Jaws of the Conclave", Types: []string{"creature", "wolf"},
			AST: &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Triggered{
				Effect: eff,
				// Note the target clause "each creature you control" precedes the
				// basis clause "where x is the number of elves you control".
				Raw: "whenever voja attacks, put x +1/+1 counters on each creature you control, where x is the number of elves you control",
			}}}},
		Controller: 0, Owner: 0, Counters: map[string]int{}, Flags: map[string]int{}, Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)

	ResolveEffect(gs, src, eff)
	// X = 3 elves (NOT 5 total creatures). Voja (Wolf) is not an elf.
	if ally.Counters["+1/+1"] != 3 {
		t.Fatalf("Voja should put X=elves=3 +1/+1 on each creature, got %d (5 ⇒ the old target-clause miscount)", ally.Counters["+1/+1"])
	}
}

// pluralizeCreatureType must map singular subtypes to their printed plural so a
// controlled creature's subtype matches the basis noun without singularizing
// the (ambiguous) plural. Pins the f→ves / y→ies / +s branches.
func TestDynamicX_PluralizeCreatureType(t *testing.T) {
	cases := map[string]string{
		"elf": "elves", "wolf": "wolves", "dwarf": "dwarves",
		"ally": "allies", "zombie": "zombies", "goblin": "goblins",
		"wizard": "wizards", "vampire": "vampires", "human": "humans",
	}
	for sing, want := range cases {
		if got := pluralizeCreatureType(sing); got != want {
			t.Errorf("pluralizeCreatureType(%q) = %q, want %q", sing, got, want)
		}
	}
}

// Guard: a NON-creature "where X is the number of <type> you control" basis
// (lands / artifacts) must NOT be miscounted as a creature subtype — the noun
// is rejected so the effect falls through to the unchanged path (no regression).
func TestDynamicX_NonCreatureBasis_NotMiscountedAsSubtype(t *testing.T) {
	for _, basis := range []string{
		"the number of lands you control",
		"the number of artifacts you control",
		"the number of enchantments you control",
	} {
		if noun, ok := subtypeYouControlNoun(basis); ok {
			t.Errorf("subtypeYouControlNoun(%q) treated %q as a creature subtype; must reject non-creature bases", basis, noun)
		}
	}
	// And a real subtype basis IS recognized.
	if noun, ok := subtypeYouControlNoun("the number of elves you control"); !ok || noun != "elves" {
		t.Fatalf("subtypeYouControlNoun(elves) = (%q,%v), want (elves,true)", noun, ok)
	}
}

// Guard: a genuine cast-{X} spell (no where-clause) still reads the cast X and
// is unaffected by the where-clause fallback.
func TestDynamicX_CastXSpellUnaffected(t *testing.T) {
	gs := newTestGameState(2)
	gs.Flags["x"] = 5
	victim := dxCreature(gs, 1, "Target")
	victim.Card.BaseToughness = 20
	eff := &gameast.Damage{Amount: *gameast.NumStr("x"), Target: gameast.Filter{Base: "creature", Targeted: true}}
	src := &Permanent{Card: &Card{Name: "Fireball", Types: []string{"instant"}}, Controller: 0, Owner: 0, Flags: map[string]int{}}
	ResolveEffect(gs, src, eff)
	if victim.MarkedDamage != 5 {
		t.Fatalf("cast X=5 spell should deal 5, got %d", victim.MarkedDamage)
	}
}

// --- create-X-TOKENS count basis (r63 token-count firespot follow-up) ---
//
// resolveCreateToken evaluates its count through evalNumber, so the same
// where-X / cast-X resolution that fixed counter counts also drives token
// counts. These pin the create-X-tokens path with an ESTABLISHED board, which
// the live OUTCOME/PROGRESSION checkers cannot model from the lossy AST (they
// pin X to a fixed value / mark the pair out of scope) — the engine must still
// resolve the real basis.

func dxCountSubtype(gs *GameState, seat int, sub string) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, ty := range p.Card.Types {
			if ty == sub {
				n++
				break
			}
		}
	}
	return n
}

// Chancellor of the Forge — ETB create X 1/1 goblins, X = creatures you control
// (where-X basis). With 2 allies + Chancellor on the board, X = 3.
func TestDynamicX_ChancellorTokensWhereX(t *testing.T) {
	gs := newTestGameState(2)
	dxCreature(gs, 0, "Ally1")
	dxCreature(gs, 0, "Ally2")
	eff := &gameast.CreateToken{Count: *gameast.NumStr("x"), PT: &[2]int{1, 1}, Color: []string{"R"},
		Types: []string{"phyrexian", "goblin"}, Keywords: []string{"haste"}}
	src := &Permanent{
		Card: &Card{Name: "Chancellor of the Forge", Types: []string{"creature"},
			AST: &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Triggered{Effect: eff,
				Raw: "when this creature enters, create x 1/1 red phyrexian goblin creature tokens with haste, where x is the number of creatures you control"}}}},
		Controller: 0, Owner: 0, Flags: map[string]int{}, Counters: map[string]int{}, Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	before := dxCountSubtype(gs, 0, "goblin")
	ResolveEffect(gs, src, eff)
	if got := dxCountSubtype(gs, 0, "goblin") - before; got != 3 {
		t.Fatalf("Chancellor should create X=creatures-you-control=3 goblins, got %d", got)
	}
}

// Farmer Cotton — {X}{G}{W} ETB create X 1/1 halflings, X = cast {X}. The
// permanent stamps perm.Flags["chosen_x"]; the create-X-tokens path must read
// it (resolves in a later frame where gs.Flags["x"] is 0).
func TestDynamicX_FarmerCottonTokensCastX(t *testing.T) {
	gs := newTestGameState(2)
	eff := &gameast.CreateToken{Count: *gameast.NumStr("x"), PT: &[2]int{1, 1}, Color: []string{"W"},
		Types: []string{"halfling"}}
	src := &Permanent{
		Card: &Card{Name: "Farmer Cotton", Types: []string{"creature"},
			AST: &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Triggered{Effect: eff,
				Raw: "when this creature enters, create x 1/1 white halfling creature tokens and x food tokens"}}}},
		Controller: 0, Owner: 0, Flags: map[string]int{"chosen_x": 3}, Counters: map[string]int{}, Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	before := dxCountSubtype(gs, 0, "halfling")
	ResolveEffect(gs, src, eff)
	if got := dxCountSubtype(gs, 0, "halfling") - before; got != 3 {
		t.Fatalf("Farmer Cotton cast for X=3 should create 3 halflings, got %d", got)
	}
}

// --- non-creature permanent subtype + greatest-shared-type X bases (r63
// goldilocks-dynamic-x cluster) ---
//
// Real engine gaps found while triaging the live-grinder dynamic-X cluster (the
// OUTCOME harness masked them by pinning gs.Flags["x"]=3, but in real play
// gs.Flags["x"]==0 and the basis must be read from the board). Cast-X token
// creators (March of the Canonized / Triceraton / Wildfire) and party-count
// damage (Thundering Sparkmage) were already correct; these two were not.

func gdxPermOf(gs *GameState, seat int, name string, types ...string) *Permanent {
	p := &Permanent{Card: &Card{Name: name, Types: types}, Controller: seat, Owner: seat,
		Counters: map[string]int{}, Flags: map[string]int{}, Timestamp: gs.NextTimestamp()}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// Southern Air Temple — put X +1/+1 on each creature, X = number of SHRINES you
// control (Shrine is an ENCHANTMENT subtype). Pre-fix the subtype counter only
// looked at creatures, so X resolved to 0 → 1 counter; now it counts all
// permanents with the subtype (3 Shrines → 3).
func TestDynamicX_SouthernAirTemple_ShrineSubtype(t *testing.T) {
	gs := newTestGameState(2)
	gdxPermOf(gs, 0, "Sanctum of Stone Fangs", "enchantment", "shrine")
	gdxPermOf(gs, 0, "Honden of Life's Web", "enchantment", "shrine")
	ally := dxCreature(gs, 0, "Ally")
	eff := &gameast.CounterMod{Op: "put", Count: *gameast.NumStr("x"), CounterKind: "+1/+1",
		Target: gameast.Filter{Base: "creature", Quantifier: "each", YouControl: true}}
	src := &Permanent{Card: &Card{Name: "Southern Air Temple", Types: []string{"enchantment", "shrine"},
		AST: &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Triggered{Effect: eff,
			Raw: "when ~ enters, put x +1/+1 counters on each creature you control, where x is the number of shrines you control"}}}},
		Controller: 0, Owner: 0, Counters: map[string]int{}, Flags: map[string]int{}, Timestamp: gs.NextTimestamp()}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	ResolveEffect(gs, src, eff)
	// 3 Shrines on the battlefield (the two Hondens + the Temple itself).
	if got := ally.Counters["+1/+1"]; got != 3 {
		t.Fatalf("Southern Air Temple should put X=shrines-you-control=3, got %d", got)
	}
}

// Basalt Ravager — deal X damage, X = the GREATEST number of creatures you
// control sharing a creature type. Pre-fix the generic "creatures you control"
// case matched the basis clause and returned the TOTAL creature count (5); now
// it returns the largest shared-type group (3 Goblins).
func TestDynamicX_BasaltRavager_GreatestSharedType(t *testing.T) {
	gs := newTestGameState(2)
	dxCreature(gs, 0, "Goblin A", "goblin")
	dxCreature(gs, 0, "Goblin B", "goblin")
	dxCreature(gs, 0, "Goblin C", "goblin")
	dxCreature(gs, 0, "Lone Wizard", "wizard")
	victim := dxCreature(gs, 1, "Victim")
	victim.Card.BaseToughness = 20
	eff := &gameast.Damage{Amount: *gameast.NumStr("x"), Target: gameast.Filter{Base: "creature", Targeted: true}}
	src := &Permanent{Card: &Card{Name: "Basalt Ravager", Types: []string{"creature", "elemental"},
		AST: &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Triggered{Effect: eff,
			Raw: "when this creature enters, it deals x damage to any target, where x is the greatest number of creatures you control that have a creature type in common"}}}},
		Controller: 0, Owner: 0, Flags: map[string]int{}, Timestamp: gs.NextTimestamp()}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	ResolveEffect(gs, src, eff)
	// 3 Goblins share "goblin"; total creatures you control = 5 (3 goblins,
	// wizard, Ravager). X must be 3, not 5.
	if victim.MarkedDamage != 3 {
		t.Fatalf("Basalt Ravager should deal X=greatest-shared-type=3, got %d", victim.MarkedDamage)
	}
}

// Guard: an ordinary "<creature-subtype> you control" basis (Voja's elves)
// still resolves correctly after generalizing the counter to all permanents.
func TestDynamicX_CreatureSubtypeStillCounts(t *testing.T) {
	gs := newTestGameState(2)
	dxCreature(gs, 0, "Elf A", "elf")
	dxCreature(gs, 0, "Elf B", "elf")
	ally := dxCreature(gs, 0, "Bear")
	eff := &gameast.CounterMod{Op: "put", Count: *gameast.NumStr("x"), CounterKind: "+1/+1",
		Target: gameast.Filter{Base: "creature", Quantifier: "each", YouControl: true}}
	src := &Permanent{Card: &Card{Name: "Elf Lord", Types: []string{"creature", "wolf"},
		AST: &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Triggered{Effect: eff,
			Raw: "whenever ~ attacks, put x +1/+1 counters on each creature you control, where x is the number of elves you control"}}}},
		Controller: 0, Owner: 0, Counters: map[string]int{}, Flags: map[string]int{}, Timestamp: gs.NextTimestamp()}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	ResolveEffect(gs, src, eff)
	if got := ally.Counters["+1/+1"]; got != 2 {
		t.Fatalf("elves-you-control should still resolve to 2 (Elf Lord is a Wolf), got %d", got)
	}
}
