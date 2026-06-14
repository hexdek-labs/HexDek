package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// priority_window_triggers_r60_test.go — R60 audit of hat's priority-
// window awareness when the stack top is a TRIGGERED (or activated)
// ability rather than a cast spell.
//
// Survey finding: every threat-detection branch in ChooseResponse was
// gated on `top.Card != nil`. Triggered abilities have top.Card == nil
// but top.Source != nil — so the hat saw a zero-score, oracle-text-
// less stack item and silently short-circuited every must-counter
// heuristic. The hat couldn't tell whether the trigger was a
// game-deciding Etali / Sheoldred / Smothering Tithe payload or a
// random "deals 1 damage to a creature" ability.
//
// Two fixes:
//   1. `effectiveResponseCard(top)` resolves top.Card for spells or
//      top.Source.Card for triggers/activations — so the existing
//      isComboRelevant / OracleTextLower / vulnerableToSet / cardsSeen
//      branches all light up for triggers automatically.
//   2. A new high-value-trigger pattern set forces mustCounter when
//      the trigger's source oracle matches Etali / Zur / Narset
//      attack-trigger value extraction, mass-draw, mass-drain, or
//      Sheoldred-shape "each opponent draws / loses" payloads.
//
// stackItemScore also picks up triggers correctly now (CMC of source
// card + a flat +2 ability-on-stack surcharge so the gate has
// something to compare against the minScore threshold).

// triggerItemFor builds a StackItem representing a triggered ability
// from `sourcePerm` with no top.Card. Mirrors what PushTriggeredAbility
// produces in the engine — the Source pointer carries the permanent
// whose oracle text describes the trigger payload.
func triggerItemFor(sourcePerm *gameengine.Permanent, controller int) *gameengine.StackItem {
	return &gameengine.StackItem{
		Kind:       "triggered",
		Source:     sourcePerm,
		Controller: controller,
	}
}

// spellItemFor builds a spell StackItem with the given card. The
// hat's ChooseResponse expects spells to have top.Card set and
// top.Source nil; this matches that contract.
func spellItemFor(card *gameengine.Card, controller int) *gameengine.StackItem {
	return &gameengine.StackItem{
		Card:       card,
		Controller: controller,
	}
}

// -----------------------------------------------------------------------------
// effectiveResponseCard — unit
// -----------------------------------------------------------------------------

func TestEffectiveResponseCard_SpellReturnsTopCard(t *testing.T) {
	c := newTestCardMinimal("Lightning Bolt", []string{"instant"}, 1, nil)
	top := spellItemFor(c, 1)
	if got := effectiveResponseCard(top); got != c {
		t.Errorf("spell path should return top.Card; got %v", got)
	}
}

func TestEffectiveResponseCard_TriggerReturnsSourceCard(t *testing.T) {
	gs := newTestGame(t, 2)
	srcCard := newTestCardMinimal("Etali, Primal Storm", []string{"creature"}, 6,
		&gameast.CardAST{
			Name:      "Etali, Primal Storm",
			Abilities: []gameast.Ability{&gameast.Static{Raw: "Whenever Etali attacks, exile the top card of each player's library."}},
		})
	src := newTestPermanent(gs.Seats[1], srcCard, 6, 6)
	top := triggerItemFor(src, 1)
	if got := effectiveResponseCard(top); got != srcCard {
		t.Errorf("trigger path should return top.Source.Card; got %v", got)
	}
}

func TestEffectiveResponseCard_NilSafe(t *testing.T) {
	if got := effectiveResponseCard(nil); got != nil {
		t.Errorf("nil top must return nil; got %v", got)
	}
	if got := effectiveResponseCard(&gameengine.StackItem{}); got != nil {
		t.Errorf("empty item must return nil; got %v", got)
	}
}

// -----------------------------------------------------------------------------
// isAbilityOnStack — unit
// -----------------------------------------------------------------------------

func TestIsAbilityOnStack_TriggeredKind(t *testing.T) {
	top := &gameengine.StackItem{Kind: "triggered", Source: &gameengine.Permanent{}}
	if !isAbilityOnStack(top) {
		t.Errorf("Kind=triggered should classify as ability-on-stack")
	}
}

func TestIsAbilityOnStack_ActivatedKind(t *testing.T) {
	top := &gameengine.StackItem{Kind: "activated", Source: &gameengine.Permanent{}}
	if !isAbilityOnStack(top) {
		t.Errorf("Kind=activated should classify as ability-on-stack")
	}
}

func TestIsAbilityOnStack_LegacyInferenceFromSource(t *testing.T) {
	// Engine-internal pushes may leave Kind empty; Source non-nil with
	// Card nil is the legacy signature for a triggered/activated ability.
	top := &gameengine.StackItem{Source: &gameengine.Permanent{}}
	if !isAbilityOnStack(top) {
		t.Errorf("Card nil + Source non-nil should infer ability-on-stack")
	}
}

func TestIsAbilityOnStack_SpellReturnsFalse(t *testing.T) {
	top := &gameengine.StackItem{Card: &gameengine.Card{}}
	if isAbilityOnStack(top) {
		t.Errorf("spell (Card non-nil) must not classify as ability")
	}
}

// -----------------------------------------------------------------------------
// stackItemScore now considers triggers
// -----------------------------------------------------------------------------

// TestStackItemScore_TriggerCarriesSourceScore pins that triggers no
// longer silently return 0. The source's CMC + threat-word bumps +
// the flat +2 ability surcharge should yield a meaningful score so
// the minScore gate can fire.
func TestStackItemScore_TriggerCarriesSourceScore(t *testing.T) {
	gs := newTestGame(t, 2)
	srcCard := newTestCardMinimal("Vorinclex", []string{"creature"}, 8,
		&gameast.CardAST{
			Name:      "Vorinclex",
			Abilities: []gameast.Ability{&gameast.Static{Raw: "Whenever Vorinclex attacks, destroy all of target opponent's lands."}},
		})
	src := newTestPermanent(gs.Seats[1], srcCard, 7, 6)
	top := triggerItemFor(src, 1)

	score := stackItemScore(top)
	// Expected: 8 (CMC) + 5 (destroy all) + 2 (ability surcharge) = 15.
	if score < 10 {
		t.Errorf("trigger source with destroy-all + CMC=8 should produce ≥ 10 score; got %d", score)
	}
}

// TestStackItemScore_SpellPathUnchanged pins that the spell path
// still scores as it did pre-R60 (no regression on the common case).
func TestStackItemScore_SpellPathUnchanged(t *testing.T) {
	spell := newTestCardMinimal("Wrath of God", []string{"sorcery"}, 4,
		&gameast.CardAST{
			Name:      "Wrath of God",
			Abilities: []gameast.Ability{&gameast.Static{Raw: "Destroy all creatures."}},
		})
	top := spellItemFor(spell, 1)
	score := stackItemScore(top)
	// Spell: 4 (CMC) + 5 (destroy all). No ability surcharge.
	if score != 9 {
		t.Errorf("Wrath spell should score exactly 9 (4 CMC + 5 destroy-all); got %d", score)
	}
}

// -----------------------------------------------------------------------------
// ChooseResponse — must-counter on high-value triggers
// -----------------------------------------------------------------------------

// abilityCounterInHand builds a Stifle/Disallow-shape counter — "counter
// target ability" — whose filter LEGALLY targets triggered/activated
// abilities (CR §701.5h via matchesCounterFilter's "abilities" base). These
// trigger-counter tests must hold this, not a "counter target spell"
// (counterInHand), because a spell-only counter cannot legally target an
// ability — the r63 CounterCanTarget gate in ChooseResponse correctly refuses
// to offer it, and at runtime PriorityRound already rejected it
// (counter_filter_mismatch), so the r60 trigger-counter feature was inert in
// real games until paired with a legal counter.
func abilityCounterInHand() *gameengine.Card {
	return newTestCardMinimal("Disallow", []string{"instant"}, 3,
		&gameast.CardAST{
			Name: "Disallow",
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Effect: &gameast.CounterSpell{
						Target: gameast.Filter{Base: "abilities", Targeted: true},
					},
				},
			},
		})
}

// hatWithCounter wires a midrange YggdrasilHat onto seat 0 with an
// ability-capable counter + enough mana to cast it.
func hatWithCounter(t *testing.T) *gameengine.GameState {
	t.Helper()
	gs := newTestGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{abilityCounterInHand()}
	gs.Seats[0].ManaPool = 6
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
	gs.Seats[0].Hat = h
	return gs
}

// TestChooseResponse_TriggerMustCounter_EtaliAttackTrigger pins the
// Etali pattern — "exile the top card of each player's library" is
// a high-value impulse-cast trigger worth countering.
func TestChooseResponse_TriggerMustCounter_EtaliAttackTrigger(t *testing.T) {
	gs := hatWithCounter(t)
	h := gs.Seats[0].Hat.(*YggdrasilHat)

	src := newTestPermanent(gs.Seats[1],
		newTestCardMinimal("Etali, Primal Storm", []string{"creature"}, 6,
			&gameast.CardAST{
				Name:      "Etali, Primal Storm",
				Abilities: []gameast.Ability{&gameast.Static{Raw: "Whenever Etali, Primal Storm attacks, exile the top card of each player's library."}},
			}),
		7, 6)
	top := triggerItemFor(src, 1)

	resp := h.ChooseResponse(gs, 0, top)
	if resp == nil {
		t.Fatalf("hat MUST counter Etali's attack trigger (impulse-cast each player's top)")
	}
}

// TestChooseResponse_TriggerMustCounter_SheoldredOnDraw pins the
// Sheoldred drain trigger — "Whenever an opponent draws a card, that
// player loses 2 life and you gain 2 life." Stacked from a wheel,
// the hat sees one drain trigger per opp draw — must counter.
func TestChooseResponse_TriggerMustCounter_SheoldredOnDraw(t *testing.T) {
	gs := hatWithCounter(t)
	h := gs.Seats[0].Hat.(*YggdrasilHat)

	src := newTestPermanent(gs.Seats[1],
		newTestCardMinimal("Sheoldred, the Apocalypse", []string{"creature"}, 4,
			&gameast.CardAST{
				Name:      "Sheoldred, the Apocalypse",
				Abilities: []gameast.Ability{&gameast.Static{Raw: "Whenever an opponent draws a card, that player loses 2 life and you gain 2 life."}},
			}),
		4, 5)
	top := triggerItemFor(src, 1)

	resp := h.ChooseResponse(gs, 0, top)
	if resp == nil {
		t.Fatalf("hat MUST counter Sheoldred's drain trigger")
	}
}

// TestChooseResponse_TriggerMustCounter_AttackTriggerTutor pins
// the Zur / Narset family — attack-trigger search-your-library that
// fetches a key piece. The "search your library" pattern fires
// mustCounter for triggers.
func TestChooseResponse_TriggerMustCounter_AttackTriggerTutor(t *testing.T) {
	gs := hatWithCounter(t)
	h := gs.Seats[0].Hat.(*YggdrasilHat)

	src := newTestPermanent(gs.Seats[1],
		newTestCardMinimal("Zur the Enchanter", []string{"creature", "legendary"}, 4,
			&gameast.CardAST{
				Name:      "Zur the Enchanter",
				Abilities: []gameast.Ability{&gameast.Static{Raw: "Whenever Zur attacks, you may search your library for an enchantment card with mana value 3 or less and put it onto the battlefield."}},
			}),
		1, 4)
	top := triggerItemFor(src, 1)

	resp := h.ChooseResponse(gs, 0, top)
	if resp == nil {
		t.Fatalf("hat MUST counter Zur's attack-trigger tutor")
	}
}

// TestChooseResponse_TriggerLowImpactDoesNotMustCounter pins the
// negative side: a trigger from a low-impact source ("when this
// creature attacks, scry 1") should NOT force mustCounter. The hat
// can pass if the score-gate also doesn't trip.
func TestChooseResponse_TriggerLowImpactDoesNotMustCounter(t *testing.T) {
	gs := hatWithCounter(t)
	h := gs.Seats[0].Hat.(*YggdrasilHat)

	src := newTestPermanent(gs.Seats[1],
		newTestCardMinimal("Snapping Sailback", []string{"creature"}, 2,
			&gameast.CardAST{
				Name:      "Snapping Sailback",
				Abilities: []gameast.Ability{&gameast.Static{Raw: "Whenever this creature attacks, scry 1."}},
			}),
		2, 2)
	top := triggerItemFor(src, 1)

	// We can't directly assert mustCounter=false (it's internal).
	// Indirect proof: with a 2-CMC source + flat +2 ability surcharge
	// (score=4) AND midrange minScore=3 — the score gate would tip
	// over to "counter". To prove mustCounter ISN'T forced, we set
	// the hat to AHEAD positioning so minScore += 2 (→ 5), past the
	// raw score (4). Without mustCounter, the hat will pass.
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 5 // we're far ahead by life metric
	resp := h.ChooseResponse(gs, 0, top)
	if resp != nil {
		t.Errorf("low-impact scry trigger should not force a counter when we're far ahead; got %v",
			resp.Card.DisplayName())
	}
}
