package per_card

import (
	"reflect"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// r62 — double-registration generalization (#1034 follow-up). The
// registerDefaults-vs-batch-init cross-file duplication that doubled
// OnTrigger handlers had ALSO double-registered 38 OnETB handlers and 1
// OnActivated handler (every one fired twice per event: Anointed
// Procession, Wayward Servant, Propaganda, Academy Manufactor, the
// zombie-lord family, …). All five registration maps now dedupe on
// handler identity; this guard walks the live registry and fails on ANY
// same-fn duplicate, so a future batch reorganization cannot silently
// reintroduce the class.
// -----------------------------------------------------------------------------

func TestRegistry_NoSameFnDuplicates_AnyMap(t *testing.T) {
	r := Global()
	r.mu.RLock()
	defer r.mu.RUnlock()

	check := func(label, card string, ptrs []uintptr) {
		seen := map[uintptr]bool{}
		for _, p := range ptrs {
			if seen[p] {
				t.Errorf("same handler registered twice: map=%s card=%q — double-fire (the #1034 class)", label, card)
			}
			seen[p] = true
		}
	}
	for card, hs := range r.etb {
		ptrs := make([]uintptr, len(hs))
		for i, h := range hs {
			ptrs[i] = reflect.ValueOf(h).Pointer()
		}
		check("etb", card, ptrs)
	}
	for card, hs := range r.onCast {
		ptrs := make([]uintptr, len(hs))
		for i, h := range hs {
			ptrs[i] = reflect.ValueOf(h).Pointer()
		}
		check("onCast", card, ptrs)
	}
	for card, hs := range r.onResolve {
		ptrs := make([]uintptr, len(hs))
		for i, h := range hs {
			ptrs[i] = reflect.ValueOf(h).Pointer()
		}
		check("onResolve", card, ptrs)
	}
	for card, hs := range r.activated {
		ptrs := make([]uintptr, len(hs))
		for i, h := range hs {
			ptrs[i] = reflect.ValueOf(h).Pointer()
		}
		check("activated", card, ptrs)
	}
	for card, byEvent := range r.onTrigger {
		for event, hs := range byEvent {
			ptrs := make([]uintptr, len(hs))
			for i, h := range hs {
				ptrs[i] = reflect.ValueOf(h).Pointer()
			}
			check("onTrigger:"+event, card, ptrs)
		}
	}
}

// OnETB dedupe contract: same fn re-registered is a no-op; a distinct
// fn (complementary bookkeeping+gameplay pairs like Sakashima's copy
// handler + InstanceID mechanism) still registers.
func TestRegistry_OnETBDedupesSameHandler(t *testing.T) {
	r := Global()
	h := func(gs *gameengine.GameState, perm *gameengine.Permanent) {}
	r.OnETB("Dblreg ETB Probe", h)
	r.OnETB("Dblreg ETB Probe", h)
	h2 := func(gs *gameengine.GameState, perm *gameengine.Permanent) {}
	r.OnETB("Dblreg ETB Probe", h2)

	r.mu.RLock()
	n := len(r.etb[NormalizeName("Dblreg ETB Probe")])
	r.mu.RUnlock()
	if n != 2 {
		t.Fatalf("want 2 handlers (h deduped, h2 distinct), got %d", n)
	}
}

// Celestial Unicorn — the closure-vs-named-fn duplicate IMPLEMENTATION
// (lifegain_counter_family table + dedicated handler) that fn-identity
// dedupe cannot catch: until r62 every life-gain event granted DOUBLE
// +1/+1 counters. Exactly one per event now; one implementation per card.
func TestCelestialUnicorn_ExactlyOneCounterPerLifeGain(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	uni := &gameengine.Permanent{
		Card: &gameengine.Card{Name: "Celestial Unicorn", Owner: 0,
			Types: []string{"creature"}, BasePower: 2, BaseToughness: 2},
		Controller: 0, Owner: 0,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, uni)
	gameengine.FireCardTrigger(gs, "life_gained", map[string]interface{}{
		"seat": 0, "amount": 3,
	})
	if got := uni.Counters["+1/+1"]; got != 1 {
		t.Fatalf("Celestial Unicorn should gain exactly 1 counter per life-gain event, got %d", got)
	}
}

// Tripwire: a card with BOTH a per_card permanent_etb handler AND a
// fully-typed AST observer trigger currently fires exactly ONCE — the
// AST observer half is gated by observerETBMatches' filter strictness
// (and the review-07 C2 actor-null gap). If the AST observer path is
// ever repaired WITHOUT a per_card suppression mechanism, this fails —
// loudly flagging that the per_card-handled staples started
// double-firing through both dispatch systems.
func TestTripwire_ASTPlusPerCard_FiresOnce(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	ws := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name: "Wayward Servant", Owner: 0,
			Types: []string{"creature", "zombie"}, BasePower: 2, BaseToughness: 2,
			TypeLine: "Creature — Zombie Cleric",
			AST: &gameast.CardAST{
				Name: "Wayward Servant",
				Abilities: []gameast.Ability{
					&gameast.Triggered{
						Trigger: gameast.Trigger{Event: "another_subtype_enters",
							Actor: &gameast.Filter{Base: "creature", YouControl: true}},
						Effect: &gameast.Sequence{Items: []gameast.Effect{
							&gameast.LoseLife{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}},
							&gameast.GainLife{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}},
						}},
					},
				},
			},
		},
		Controller: 0, Owner: 0,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, ws)
	zombie := &gameengine.Permanent{
		Card: &gameengine.Card{Name: "Walking Corpse", Owner: 0,
			Types: []string{"creature", "zombie"}, TypeLine: "Creature — Zombie",
			BasePower: 2, BaseToughness: 2},
		Controller: 0, Owner: 0,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, zombie)
	oppBefore := gs.Seats[1].Life
	gameengine.FirePermanentETBTriggers(gs, zombie)
	gameengine.DrainStack(gs)
	if got := oppBefore - gs.Seats[1].Life; got != 1 {
		t.Fatalf("Wayward Servant drained %d on one zombie ETB — if 2, the AST observer path was repaired without per_card suppression (see double-registration-generalization.md)", got)
	}
}
