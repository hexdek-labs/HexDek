package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Bargain tests — CR §702.166
// ---------------------------------------------------------------------------

func newBargainGame(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(57)), nil)
}

// bargainSpellCard builds a sorcery/instant that carries the bargain
// keyword on its AST.
func bargainSpellCard(name string, owner int) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"sorcery"},
		CMC:   2,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "bargain"},
			},
		},
	}
}

func nonBargainSpellCard(name string, owner int) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"sorcery"},
		AST: &gameast.CardAST{
			Name:      name,
			Abilities: []gameast.Ability{},
		},
	}
}

func addTypePerm(gs *GameState, owner int, name string, types ...string) *Permanent {
	card := &Card{
		Name:  name,
		Owner: owner,
		Types: append([]string{}, types...),
		AST:   &gameast.CardAST{Name: name},
	}
	perm := &Permanent{
		Card:       card,
		Controller: owner,
		Owner:      owner,
		Flags:      map[string]int{},
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Seats[owner].Battlefield = append(gs.Seats[owner].Battlefield, perm)
	return perm
}

// ---------------------------------------------------------------------------
// HasBargain detector
// ---------------------------------------------------------------------------

func TestHasBargain_Detects(t *testing.T) {
	if !HasBargain(bargainSpellCard("Beseech the Mirror", 0)) {
		t.Fatal("HasBargain should detect bargain keyword")
	}
}

func TestHasBargain_Negative(t *testing.T) {
	if HasBargain(nonBargainSpellCard("Lightning Bolt", 0)) {
		t.Fatal("HasBargain should be false for a non-bargain card")
	}
	if HasBargain(nil) {
		t.Fatal("HasBargain(nil) should be false")
	}
}

// ---------------------------------------------------------------------------
// (a) Valid sacrifice succeeds + CostMeta stamped
// ---------------------------------------------------------------------------

func TestCastWithBargain_ValidArtifactSacrificeSucceeds(t *testing.T) {
	gs := newBargainGame(t)
	gs.Active = 0
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, bargainSpellCard("Spell", 0))
	artifact := addTypePerm(gs, 0, "Sol Ring", "artifact")

	spell := gs.Seats[0].Hand[0]
	if err := CastWithBargain(gs, 0, spell, artifact); err != nil {
		t.Fatalf("CastWithBargain returned error: %v", err)
	}
	// Sacrifice landed: Sol Ring no longer on battlefield.
	for _, p := range gs.Seats[0].Battlefield {
		if p == artifact {
			t.Fatal("bargain target should be sacrificed (off battlefield)")
		}
	}
	// Stack item pushed with bargained=true.
	if len(gs.Stack) != 1 {
		t.Fatalf("expected 1 stack item, got %d", len(gs.Stack))
	}
	item := gs.Stack[0]
	if item.Card != spell {
		t.Fatal("stack item Card should reference the bargain spell")
	}
	if item.CostMeta == nil {
		t.Fatal("CostMeta should be populated")
	}
	if v, ok := item.CostMeta["bargained"]; !ok || v != true {
		t.Fatalf("CostMeta[bargained] = %v, want true", item.CostMeta["bargained"])
	}
	if v, ok := item.CostMeta["bargain_target"]; !ok || v != "Sol Ring" {
		t.Fatalf("CostMeta[bargain_target] = %v, want \"Sol Ring\"", item.CostMeta["bargain_target"])
	}
}

func TestCastWithBargain_EnchantmentSacrificeSucceeds(t *testing.T) {
	gs := newBargainGame(t)
	spell := bargainSpellCard("Spell", 0)
	enchant := addTypePerm(gs, 0, "Rhystic Study", "enchantment")
	if err := CastWithBargain(gs, 0, spell, enchant); err != nil {
		t.Fatalf("CastWithBargain on enchantment failed: %v", err)
	}
	if len(gs.Stack) != 1 || gs.Stack[0].CostMeta["bargained"] != true {
		t.Fatal("expected bargained stack item")
	}
}

func TestCastWithBargain_TokenSacrificeSucceeds(t *testing.T) {
	gs := newBargainGame(t)
	spell := bargainSpellCard("Spell", 0)
	// A creature token (token + creature both on Types).
	tok := addTypePerm(gs, 0, "Treasure Token", "artifact", "token")
	if err := CastWithBargain(gs, 0, spell, tok); err != nil {
		t.Fatalf("CastWithBargain on token failed: %v", err)
	}
	if len(gs.Stack) != 1 || gs.Stack[0].CostMeta["bargained"] != true {
		t.Fatal("expected bargained stack item")
	}
}

func TestCastWithBargain_EmitsBargainEvent(t *testing.T) {
	gs := newBargainGame(t)
	spell := bargainSpellCard("Spell", 0)
	artifact := addTypePerm(gs, 0, "Sol Ring", "artifact")
	before := len(gs.EventLog)

	if err := CastWithBargain(gs, 0, spell, artifact); err != nil {
		t.Fatalf("CastWithBargain failed: %v", err)
	}
	found := false
	for i := before; i < len(gs.EventLog); i++ {
		if gs.EventLog[i].Kind == "bargain" && gs.EventLog[i].Details["sacrificed"] == "Sol Ring" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("CastWithBargain should emit a bargain event with sacrificed=<name>")
	}
}

func TestCastWithBargain_FiresBargainPaidTrigger(t *testing.T) {
	gs := newBargainGame(t)
	spell := bargainSpellCard("Spell", 0)
	artifact := addTypePerm(gs, 0, "Sol Ring", "artifact")

	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	var sawEvent string
	var sawSacrificed string
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev == "bargain_paid" {
			sawEvent = ev
			if s, ok := ctx["sacrificed"].(string); ok {
				sawSacrificed = s
			}
		}
	}
	if err := CastWithBargain(gs, 0, spell, artifact); err != nil {
		t.Fatalf("CastWithBargain failed: %v", err)
	}
	if sawEvent != "bargain_paid" {
		t.Fatal("TriggerHook should observe bargain_paid")
	}
	if sawSacrificed != "Sol Ring" {
		t.Fatalf("trigger ctx[sacrificed] = %q, want \"Sol Ring\"", sawSacrificed)
	}
}

// ---------------------------------------------------------------------------
// (b) Non-AET target rejected
// ---------------------------------------------------------------------------

func TestCastWithBargain_CreatureTargetRejected(t *testing.T) {
	gs := newBargainGame(t)
	spell := bargainSpellCard("Spell", 0)
	creature := addTypePerm(gs, 0, "Grizzly Bears", "creature")
	err := CastWithBargain(gs, 0, spell, creature)
	if err == nil {
		t.Fatal("§702.176a: a creature (not an artifact/enchantment/token) cannot satisfy the bargain cost")
	}
	// Creature not sacrificed.
	stillOnBattlefield := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == creature {
			stillOnBattlefield = true
			break
		}
	}
	if !stillOnBattlefield {
		t.Fatal("rejected bargain must not sacrifice the non-eligible target")
	}
	if len(gs.Stack) != 0 {
		t.Fatal("rejected bargain must not push a stack item")
	}
}

func TestCastWithBargain_LandTargetRejected(t *testing.T) {
	gs := newBargainGame(t)
	spell := bargainSpellCard("Spell", 0)
	land := addTypePerm(gs, 0, "Forest", "land")
	if err := CastWithBargain(gs, 0, spell, land); err == nil {
		t.Fatal("a land is not an artifact/enchantment/token; must reject")
	}
}

// ---------------------------------------------------------------------------
// (c) Opponent's perm rejected
// ---------------------------------------------------------------------------

func TestCastWithBargain_OpponentArtifactRejected(t *testing.T) {
	gs := newBargainGame(t)
	spell := bargainSpellCard("Spell", 0)
	// Artifact controlled by seat 1, not seat 0.
	oppArtifact := addTypePerm(gs, 1, "Sol Ring", "artifact")
	err := CastWithBargain(gs, 0, spell, oppArtifact)
	if err == nil {
		t.Fatal("§702.176a: bargain sacrifices a permanent YOU control; opponent's perm must reject")
	}
	// Opponent's artifact stays on battlefield.
	stillThere := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == oppArtifact {
			stillThere = true
			break
		}
	}
	if !stillThere {
		t.Fatal("rejected bargain must not sacrifice opponent's permanent")
	}
}

// ---------------------------------------------------------------------------
// (d) Cast without bargain leaves flag unset
// ---------------------------------------------------------------------------

func TestCastWithoutBargain_CostMetaUnset(t *testing.T) {
	// Simulate a normal cast path (no CastWithBargain call). The stack
	// item pushed by the normal cast path doesn't set CostMeta["bargained"].
	gs := newBargainGame(t)
	spell := bargainSpellCard("Spell", 0)
	item := &StackItem{
		Card:       spell,
		Controller: 0,
		CastZone:   ZoneHand,
		Effect:     collectSpellEffect(spell),
	}
	PushStackItem(gs, item)

	// Resolution-time handlers gate on CostMeta["bargained"] — when no
	// bargain was paid, the flag is absent (or nil map).
	if item.CostMeta != nil {
		if v, ok := item.CostMeta["bargained"]; ok && v == true {
			t.Fatal("cast without bargain must NOT set CostMeta[bargained]=true")
		}
	}
}

// ---------------------------------------------------------------------------
// (e) CanBargain false when no eligible sac
// ---------------------------------------------------------------------------

func TestCanBargain_FalseWhenNoEligibleSac(t *testing.T) {
	gs := newBargainGame(t)
	// Only creatures and lands on the battlefield — no artifact, enchantment,
	// or token.
	addTypePerm(gs, 0, "Grizzly Bears", "creature")
	addTypePerm(gs, 0, "Forest", "land")
	if CanBargain(gs, 0) {
		t.Fatal("CanBargain must be false when seat controls no artifact/enchantment/token")
	}
}

func TestCanBargain_TrueWithArtifact(t *testing.T) {
	gs := newBargainGame(t)
	addTypePerm(gs, 0, "Sol Ring", "artifact")
	if !CanBargain(gs, 0) {
		t.Fatal("CanBargain should be true with an artifact on the battlefield")
	}
}

func TestCanBargain_TrueWithEnchantment(t *testing.T) {
	gs := newBargainGame(t)
	addTypePerm(gs, 0, "Rhystic Study", "enchantment")
	if !CanBargain(gs, 0) {
		t.Fatal("CanBargain should be true with an enchantment on the battlefield")
	}
}

func TestCanBargain_TrueWithToken(t *testing.T) {
	gs := newBargainGame(t)
	addTypePerm(gs, 0, "Treasure Token", "artifact", "token")
	if !CanBargain(gs, 0) {
		t.Fatal("CanBargain should be true with a token on the battlefield")
	}
}

func TestCanBargain_IgnoresOpponentPermanents(t *testing.T) {
	gs := newBargainGame(t)
	// Opponent controls an artifact, you control nothing eligible.
	addTypePerm(gs, 1, "Sol Ring", "artifact")
	if CanBargain(gs, 0) {
		t.Fatal("CanBargain must ignore opponent's eligible permanents")
	}
}

func TestEligibleBargainTargets_ListsEligibleOnly(t *testing.T) {
	gs := newBargainGame(t)
	a := addTypePerm(gs, 0, "Sol Ring", "artifact")
	addTypePerm(gs, 0, "Grizzly Bears", "creature") // not eligible
	e := addTypePerm(gs, 0, "Rhystic Study", "enchantment")
	tok := addTypePerm(gs, 0, "Treasure", "artifact", "token")
	addTypePerm(gs, 1, "Opponent Artifact", "artifact") // wrong controller

	got := EligibleBargainTargets(gs, 0)
	if len(got) != 3 {
		t.Fatalf("expected 3 eligible targets, got %d", len(got))
	}
	want := map[*Permanent]bool{a: true, e: true, tok: true}
	for _, p := range got {
		if !want[p] {
			t.Fatalf("unexpected eligible target: %s", p.Card.DisplayName())
		}
	}
}

// ---------------------------------------------------------------------------
// Negative paths
// ---------------------------------------------------------------------------

func TestCastWithBargain_NoKeywordRejected(t *testing.T) {
	gs := newBargainGame(t)
	spell := nonBargainSpellCard("Lightning Bolt", 0)
	artifact := addTypePerm(gs, 0, "Sol Ring", "artifact")
	if err := CastWithBargain(gs, 0, spell, artifact); err == nil {
		t.Fatal("cards without bargain must not run the bargain cost path")
	}
}

func TestCastWithBargain_NilTargetRejected(t *testing.T) {
	gs := newBargainGame(t)
	spell := bargainSpellCard("Spell", 0)
	if err := CastWithBargain(gs, 0, spell, nil); err == nil {
		t.Fatal("nil sacrifice target must reject the cast")
	}
}

// ---------------------------------------------------------------------------
// Nil / invalid-input safety
// ---------------------------------------------------------------------------

func TestCastWithBargain_NilSafe(t *testing.T) {
	gs := newBargainGame(t)
	if err := CastWithBargain(nil, 0, bargainSpellCard("X", 0), &Permanent{}); err == nil {
		t.Fatal("nil game must return error")
	}
	if err := CastWithBargain(gs, -1, bargainSpellCard("X", 0), &Permanent{}); err == nil {
		t.Fatal("invalid seat must return error")
	}
	if err := CastWithBargain(gs, 0, nil, &Permanent{}); err == nil {
		t.Fatal("nil card must return error")
	}
}

func TestCanBargain_NilSafe(t *testing.T) {
	if CanBargain(nil, 0) {
		t.Fatal("nil game CanBargain must be false")
	}
	gs := newBargainGame(t)
	if CanBargain(gs, -1) {
		t.Fatal("invalid seat CanBargain must be false")
	}
	if CanBargain(gs, 99) {
		t.Fatal("out-of-range seat CanBargain must be false")
	}
}

func TestEligibleBargainTargets_NilSafe(t *testing.T) {
	if got := EligibleBargainTargets(nil, 0); got != nil {
		t.Fatal("nil game EligibleBargainTargets must be nil")
	}
	gs := newBargainGame(t)
	if got := EligibleBargainTargets(gs, -1); got != nil {
		t.Fatal("invalid seat EligibleBargainTargets must be nil")
	}
}
