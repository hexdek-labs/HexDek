package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r85 — batch 9 of the keyword call-site migration: the last ten live
// layer-blind p.HasKeyword reads at the top level of internal/gameengine.
//
// Superset re-probed before writing any of this, across AST,
// GrantedAbilities, Flags["kw:"] and keyword counters for every keyword
// the batch touches. It does NOT come back clean the way r83's twelve
// cells did: nine cells answer raw-true / layer-false, all of them on
// the counter surface, for split_second / renown / prowess / extort /
// inspired / increment / soulbond.
//
// Those are not holes in the layer query. p.HasKeyword's counter check
// is a bare name match on the legacy Counters map, so Counters["prowess"]
// makes it answer true; the layer fold resolves each counter through the
// registry and only folds types categorised KeywordGrant — the twelve
// real §122.1c keyword counters. There is no prowess counter in Magic.
// This is the same defect already pinned for +1/+1 by
// TestKeywordSupersetR66_PlusOneCounterIsNotAKeyword, and the migration
// corrects it rather than inheriting it. Pinned below so the claim is
// enforced and not just written down.
//
// haste and ward both fold correctly (registry entries exist for them),
// which is why the divergence is visible at all rather than uniform.

func r85Bogus(t *testing.T, kw string) {
	t.Helper()
	gs := newUPGame(t)
	p := upCreature(gs, 0, "Countered "+kw, 2)
	p.Counters[kw] = 1
	gs.InvalidateCharacteristicsCache()
	if !p.HasKeyword(kw) {
		t.Fatalf("precondition: the raw query is expected to answer true for a bogus %q counter", kw)
	}
	if keywordActive(gs, p, kw) {
		t.Errorf("a %q counter is not a §122.1c keyword counter and must not grant %s", kw, kw)
	}
}

func TestKeywordMigrationR85_BogusCounterDoesNotGrant(t *testing.T) {
	for _, kw := range []string{"prowess", "extort", "renown", "inspired", "increment", "soulbond", "split_second"} {
		r85Bogus(t, kw)
	}
}

// --------------------------------------------------------------- split second

func r85StackSource(gs *GameState, p *Permanent) {
	gs.Stack = append(gs.Stack, &StackItem{Source: p, Controller: p.Controller})
}

// §702.61a — a source stripped by Humility has no split second and must
// not lock the table out of responding.
func TestKeywordMigrationR85_SplitSecondStrip(t *testing.T) {
	gs := newUPGame(t)
	p := upCreature(gs, 0, "Humbled Second", 2)
	p.Flags["kw:split_second"] = 1
	gs.InvalidateCharacteristicsCache()
	if !p.HasKeyword("split_second") {
		t.Fatal("precondition: the stamp should be raw-visible")
	}
	r85StackSource(gs, p)
	if !SplitSecondActive(gs) {
		t.Fatal("precondition: the stamped source should lock the stack before the strip")
	}
	r70Strip(gs, p, "r85_split_second_strip")
	if SplitSecondActive(gs) {
		t.Error("a stripped split second still locked the stack")
	}
}

// The grant direction: a §613-granted split second is real and must lock.
func TestKeywordMigrationR85_SplitSecondGrant(t *testing.T) {
	gs := newUPGame(t)
	p := upCreature(gs, 0, "Granted Second", 2)
	r85StackSource(gs, p)
	if SplitSecondActive(gs) {
		t.Fatal("precondition: a plain source must not lock the stack")
	}
	r70Grant(gs, p, "split_second", "r85_split_second_grant")
	if p.HasKeyword("split_second") {
		t.Fatal("precondition: the raw query must NOT see a layer grant")
	}
	if !SplitSecondActive(gs) {
		t.Error("a layer-granted split second did not lock the stack")
	}
}

// ---------------------------------------------------------------------- ward

func r85TargetItem(gs *GameState, target *Permanent, caster int) *StackItem {
	return &StackItem{
		Controller: caster,
		Targets:    []Target{{Kind: TargetKindPermanent, Permanent: target, Seat: target.Controller}},
	}
}

// §702.21a — a creature stripped by Humility has no ward and must not tax
// the spell targeting it.
func TestKeywordMigrationR85_WardStrip(t *testing.T) {
	gs := newUPGame(t)
	p := upCreature(gs, 0, "Humbled Warder", 2)
	p.Flags["kw:ward"] = 1
	gs.InvalidateCharacteristicsCache()
	gs.Seats[1].ManaPool = 0

	item := r85TargetItem(gs, p, 1)
	CheckWardOnTargeting(gs, item)
	if !item.Countered {
		t.Fatal("precondition: an unpayable ward should counter the spell before the strip")
	}

	r70Strip(gs, p, "r85_ward_strip")
	item2 := r85TargetItem(gs, p, 1)
	CheckWardOnTargeting(gs, item2)
	if item2.Countered {
		t.Error("a stripped ward still countered the spell")
	}
}

// The grant direction: an anthem-granted ward taxes like a printed one.
func TestKeywordMigrationR85_WardGrant(t *testing.T) {
	gs := newUPGame(t)
	p := upCreature(gs, 0, "Anthem Warder", 2)
	gs.Seats[1].ManaPool = 0

	item := r85TargetItem(gs, p, 1)
	CheckWardOnTargeting(gs, item)
	if item.Countered {
		t.Fatal("precondition: a plain creature must not tax a spell")
	}

	r70Grant(gs, p, "ward", "r85_ward_grant")
	if p.HasKeyword("ward") {
		t.Fatal("precondition: the raw query must NOT see a layer grant")
	}
	item2 := r85TargetItem(gs, p, 1)
	CheckWardOnTargeting(gs, item2)
	if !item2.Countered {
		t.Error("a layer-granted ward did not tax the spell targeting it")
	}
}

// -------------------------------------------------------------------- renown

// §702.86a — a creature stripped by Humility has no renown and must not
// become renowned on connecting.
func TestKeywordMigrationR85_RenownStrip(t *testing.T) {
	gs := newUPGame(t)
	p := upCreature(gs, 0, "Humbled Renowner", 2)
	p.Flags["kw:renown"] = 1
	gs.InvalidateCharacteristicsCache()
	r70Strip(gs, p, "r85_renown_strip")

	if ApplyRenownOnCombatDamage(gs, p, 1) {
		t.Error("a stripped renown still fired")
	}
	if IsRenowned(p) {
		t.Error("a stripped creature became renowned")
	}
}

// The grant direction: the old comment promised a global "creatures you
// control have renown 1" effect would light up here. It did not, because
// a §613 effect does not write GrantedAbilities. It does now.
func TestKeywordMigrationR85_RenownGrant(t *testing.T) {
	gs := newUPGame(t)
	p := upCreature(gs, 0, "Anthem Renowner", 2)
	r70Grant(gs, p, "renown", "r85_renown_grant")
	if p.HasKeyword("renown") {
		t.Fatal("precondition: the raw query must NOT see a layer grant")
	}
	if !ApplyRenownOnCombatDamage(gs, p, 1) {
		t.Fatal("a layer-granted renown did not fire")
	}
	if !IsRenowned(p) {
		t.Error("a layer-granted renown did not mark the creature renowned")
	}
}

// ------------------------------------------------------------------ inspired

func r85Inspired(t *testing.T, gs *GameState, p *Permanent) bool {
	t.Helper()
	before := len(gs.EventLog)
	FireInspiredTriggers(gs, p)
	for _, e := range gs.EventLog[before:] {
		if e.Kind == "inspired_trigger" {
			return true
		}
	}
	return false
}

// §702.124 — a permanent stripped by Humility has no inspired and must
// not fire on untapping.
func TestKeywordMigrationR85_InspiredStrip(t *testing.T) {
	gs := newUPGame(t)
	p := upCreature(gs, 0, "Humbled Inspired", 2)
	p.Flags["kw:inspired"] = 1
	gs.InvalidateCharacteristicsCache()
	if !r85Inspired(t, gs, p) {
		t.Fatal("precondition: the stamped keyword should fire before the strip")
	}
	r70Strip(gs, p, "r85_inspired_strip")
	if r85Inspired(t, gs, p) {
		t.Error("a stripped inspired still fired")
	}
}

func TestKeywordMigrationR85_InspiredGrant(t *testing.T) {
	gs := newUPGame(t)
	p := upCreature(gs, 0, "Anthem Inspired", 2)
	if r85Inspired(t, gs, p) {
		t.Fatal("precondition: a plain permanent must not fire inspired")
	}
	r70Grant(gs, p, "inspired", "r85_inspired_grant")
	if p.HasKeyword("inspired") {
		t.Fatal("precondition: the raw query must NOT see a layer grant")
	}
	if !r85Inspired(t, gs, p) {
		t.Error("a layer-granted inspired did not fire")
	}
}

// ------------------------------------------------------------------ soulbond

// §702.95 — a creature stripped by Humility has no soulbond and must not
// pair.
func TestKeywordMigrationR85_SoulbondStrip(t *testing.T) {
	gs := newUPGame(t)
	sb := upCreature(gs, 0, "Humbled Bonder", 2)
	sb.Flags["kw:soulbond"] = 1
	gs.InvalidateCharacteristicsCache()
	r70Strip(gs, sb, "r85_soulbond_strip")
	mate := upCreature(gs, 0, "Vanilla Mate", 2)

	FireSoulbondTriggers(gs, 0, mate)
	if IsPaired(sb) || IsPaired(mate) {
		t.Error("a stripped soulbond still paired")
	}
}

func TestKeywordMigrationR85_SoulbondGrant(t *testing.T) {
	gs := newUPGame(t)
	sb := upCreature(gs, 0, "Anthem Bonder", 2)
	r70Grant(gs, sb, "soulbond", "r85_soulbond_grant")
	if sb.HasKeyword("soulbond") {
		t.Fatal("precondition: the raw query must NOT see a layer grant")
	}
	mate := upCreature(gs, 0, "Vanilla Mate", 2)

	FireSoulbondTriggers(gs, 0, mate)
	if !IsPaired(sb) || !IsPaired(mate) {
		t.Error("a layer-granted soulbond did not pair")
	}
}

// ---------------------------------------------------- cast observers: prowess

func r85Spell() *Card {
	return &Card{Name: "Bolt", Owner: 0, Types: []string{"instant"}, Colors: []string{"R"}}
}

// §702.108 — a creature stripped by Humility has no prowess.
func TestKeywordMigrationR85_ProwessStrip(t *testing.T) {
	gs := newUPGame(t)
	p := upCreature(gs, 0, "Humbled Prowess", 2)
	p.Flags["kw:prowess"] = 1
	gs.InvalidateCharacteristicsCache()
	r70Strip(gs, p, "r85_prowess_strip")

	FireCastTriggerObservers(gs, r85Spell(), 0, false)
	if len(p.Modifications) != 0 {
		t.Errorf("a stripped prowess still pumped: %+v", p.Modifications)
	}
}

func TestKeywordMigrationR85_ProwessGrant(t *testing.T) {
	gs := newUPGame(t)
	p := upCreature(gs, 0, "Anthem Prowess", 2)
	r70Grant(gs, p, "prowess", "r85_prowess_grant")
	if p.HasKeyword("prowess") {
		t.Fatal("precondition: the raw query must NOT see a layer grant")
	}
	FireCastTriggerObservers(gs, r85Spell(), 0, false)
	if len(p.Modifications) != 1 {
		t.Fatalf("a layer-granted prowess did not pump: %+v", p.Modifications)
	}
	if gs.PowerOf(p) != 3 {
		t.Errorf("prowess pump not effective: power = %d, want 3", gs.PowerOf(p))
	}
}

// ----------------------------------------------------- cast observers: extort

// §702.101 — a permanent stripped by Humility has no extort.
func TestKeywordMigrationR85_ExtortStrip(t *testing.T) {
	gs := newUPGame(t)
	p := upCreature(gs, 0, "Humbled Extorter", 2)
	p.Flags["kw:extort"] = 1
	gs.InvalidateCharacteristicsCache()
	gs.Seats[0].ManaPool = 5
	oppLife := gs.Seats[1].Life
	r70Strip(gs, p, "r85_extort_strip")

	FireCastTriggerObservers(gs, r85Spell(), 0, false)
	if gs.Seats[1].Life != oppLife {
		t.Errorf("a stripped extort still drained: opponent %d -> %d", oppLife, gs.Seats[1].Life)
	}
}

func TestKeywordMigrationR85_ExtortGrant(t *testing.T) {
	gs := newUPGame(t)
	p := upCreature(gs, 0, "Anthem Extorter", 2)
	gs.Seats[0].ManaPool = 5
	oppLife := gs.Seats[1].Life
	r70Grant(gs, p, "extort", "r85_extort_grant")
	if p.HasKeyword("extort") {
		t.Fatal("precondition: the raw query must NOT see a layer grant")
	}
	FireCastTriggerObservers(gs, r85Spell(), 0, false)
	if gs.Seats[1].Life >= oppLife {
		t.Errorf("a layer-granted extort did not drain: opponent still at %d", gs.Seats[1].Life)
	}
}

// -------------------------------------------------- cast observers: increment

// §702.191a — a creature stripped by Humility has no increment.
func TestKeywordMigrationR85_IncrementStrip(t *testing.T) {
	gs := newUPGame(t)
	p := upCreature(gs, 0, "Humbled Incrementer", 2)
	p.Flags["kw:increment"] = 1
	gs.InvalidateCharacteristicsCache()
	r70Strip(gs, p, "r85_increment_strip")

	gs.LastCastManaSpent = 5
	FireCastTriggerObservers(gs, r85Spell(), 0, false)
	if p.Counters["+1/+1"] != 0 {
		t.Errorf("a stripped increment still placed a counter: %d", p.Counters["+1/+1"])
	}
}

func TestKeywordMigrationR85_IncrementGrant(t *testing.T) {
	gs := newUPGame(t)
	p := upCreature(gs, 0, "Anthem Incrementer", 2)
	r70Grant(gs, p, "increment", "r85_increment_grant")
	if p.HasKeyword("increment") {
		t.Fatal("precondition: the raw query must NOT see a layer grant")
	}
	gs.LastCastManaSpent = 5
	FireCastTriggerObservers(gs, r85Spell(), 0, false)
	if p.Counters["+1/+1"] != 1 {
		t.Errorf("a layer-granted increment placed %d counters, want 1", p.Counters["+1/+1"])
	}
}

// The widening: §702.191a compares the mana spent against "this creature's
// power or this creature's toughness", which is the EFFECTIVE value. An
// anthemed 2/2 that is really a 5/5 must NOT get a counter off 4 mana.
func TestKeywordMigrationR85_IncrementUsesEffectivePT(t *testing.T) {
	gs := newUPGame(t)
	p := upCreature(gs, 0, "Anthemed Incrementer", 2, "increment")
	r84Anthem(gs, p, 3, "r85_increment_anthem")
	if gs.PowerOf(p) != 5 || gs.ToughnessOf(p) != 5 {
		t.Fatalf("precondition: anthem did not apply — %d/%d", gs.PowerOf(p), gs.ToughnessOf(p))
	}
	if p.Power() != 2 {
		t.Fatalf("precondition: the raw query should still read the printed 2, got %d", p.Power())
	}

	gs.LastCastManaSpent = 4
	FireCastTriggerObservers(gs, r85Spell(), 0, false)
	if p.Counters["+1/+1"] != 0 {
		t.Errorf("increment compared against the printed 2/2 instead of the effective 5/5 — placed %d counters", p.Counters["+1/+1"])
	}
}

// ------------------------------------------------------- self cost reduction

func r85CostReduceCard(clause string) *Card {
	return &Card{
		Name: "Discounted Spell", Owner: 0, Types: []string{"sorcery"},
		AST: &gameast.CardAST{Name: "Discounted Spell", Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "this_spell_colored_cost_reduce",
				Args:    []interface{}{"{2}", "if you control a creature with flying"},
			}},
		}},
	}
}

func r85Reduction(gs *GameState) int {
	mods := selfSpellCostReductionMods(gs, r85CostReduceCard(""), 0)
	total := 0
	for _, m := range mods {
		if m.Kind == CostModReduction {
			total += m.Amount
		}
	}
	return total
}

// A creature stripped by Humility has no flying, so it must stop
// satisfying "if you control a creature with flying" — this one
// under-charges the caster, which the file's own header calls out as
// "a real bug".
func TestKeywordMigrationR85_CostReduceStrip(t *testing.T) {
	gs := newUPGame(t)
	p := upCreature(gs, 0, "Humbled Flyer", 2)
	p.Flags["kw:flying"] = 1
	gs.InvalidateCharacteristicsCache()
	if r85Reduction(gs) != 2 {
		t.Fatal("precondition: a stamped flyer should satisfy the clause before the strip")
	}
	r70Strip(gs, p, "r85_cost_reduce_strip")
	if got := r85Reduction(gs); got != 0 {
		t.Errorf("a stripped flying still bought a %d reduction", got)
	}
}

func TestKeywordMigrationR85_CostReduceGrant(t *testing.T) {
	gs := newUPGame(t)
	p := upCreature(gs, 0, "Anthem Flyer", 2)
	if r85Reduction(gs) != 0 {
		t.Fatal("precondition: a plain creature must not satisfy the clause")
	}
	r70Grant(gs, p, "flying", "r85_cost_reduce_grant")
	if p.HasKeyword("flying") {
		t.Fatal("precondition: the raw query must NOT see a layer grant")
	}
	if got := r85Reduction(gs); got != 2 {
		t.Errorf("a layer-granted flying did not buy the reduction: got %d, want 2", got)
	}
}
