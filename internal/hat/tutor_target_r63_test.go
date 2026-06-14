package hat

// R63 — strategy-aware tutor target selection. Before this change the engine's
// generic tutor resolver grabbed the first matching library card (or, at best,
// the highest-CMC one) and never consulted Freya intelligence. These tests pin
// YggdrasilHat.ChooseTutorTargets: a near-complete combo line's missing piece
// and the deck's prioritized tutor targets must beat a bigger but irrelevant
// bomb.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

func libCreature(name string, cmc int) *gameengine.Card {
	return newTestCardMinimal(name, []string{"creature"}, cmc, nil)
}

// ----------------------------------------------------------------------------
// Combo near-miss: with one piece already in hand, an unrestricted tutor must
// search up the sole missing piece — not the highest-CMC card.
// ----------------------------------------------------------------------------

func TestChooseTutorTargets_GrabsMissingComboPiece(t *testing.T) {
	gs := newTestGame(t, 2)

	sp := &StrategyProfile{
		Archetype: ArchetypeCombo,
		ComboPieces: []ComboPlan{
			{Pieces: []string{"Thassa's Oracle", "Demonic Consultation"}, Type: "infinite"},
		},
	}
	h := NewYggdrasilHat(sp, 0)
	gs.Seats[0].Hat = h

	// One piece is already in hand; the other is the missing piece.
	gs.Seats[0].Hand = append(gs.Seats[0].Hand,
		newTestCardMinimal("Demonic Consultation", []string{"instant"}, 1, nil))

	oracle := libCreature("Thassa's Oracle", 2)
	bomb := libCreature("Craterhoof Behemoth", 8)
	chaff := libCreature("Grizzly Bears", 2)
	cands := []*gameengine.Card{bomb, oracle, chaff}

	got := h.ChooseTutorTargets(gs, 0, cands, 1, gameast.Filter{Base: "creature"}, "hand")
	if len(got) != 1 {
		t.Fatalf("expected 1 pick, got %d", len(got))
	}
	if got[0].Name != "Thassa's Oracle" {
		t.Errorf("expected missing combo piece Thassa's Oracle, got %s", got[0].Name)
	}
}

// ----------------------------------------------------------------------------
// Freya tutor-priority list: the first entry on TutorTargets wins over a
// higher-CMC non-target.
// ----------------------------------------------------------------------------

func TestChooseTutorTargets_HonorsTutorPriority(t *testing.T) {
	gs := newTestGame(t, 2)

	sp := &StrategyProfile{
		Archetype:    ArchetypeControl,
		TutorTargets: []string{"Rhystic Study", "Smothering Tithe"},
	}
	h := NewYggdrasilHat(sp, 0)
	gs.Seats[0].Hat = h

	rhystic := newTestCardMinimal("Rhystic Study", []string{"enchantment"}, 3, nil)
	tithe := newTestCardMinimal("Smothering Tithe", []string{"enchantment"}, 4, nil)
	bomb := newTestCardMinimal("Expropriate", []string{"sorcery"}, 9, nil)
	cands := []*gameengine.Card{bomb, tithe, rhystic}

	got := h.ChooseTutorTargets(gs, 0, cands, 1, gameast.Filter{Base: "card"}, "hand")
	if len(got) != 1 {
		t.Fatalf("expected 1 pick, got %d", len(got))
	}
	if got[0].Name != "Rhystic Study" {
		t.Errorf("expected top tutor target Rhystic Study, got %s", got[0].Name)
	}
}

// ----------------------------------------------------------------------------
// Multi-card tutor: returns exactly `count` distinct cards, combo piece first.
// ----------------------------------------------------------------------------

func TestChooseTutorTargets_MultiCountDistinct(t *testing.T) {
	gs := newTestGame(t, 2)

	sp := &StrategyProfile{
		Archetype:    ArchetypeMidrange,
		TutorTargets: []string{"Rhystic Study"},
	}
	h := NewYggdrasilHat(sp, 0)
	gs.Seats[0].Hat = h

	a := newTestCardMinimal("Rhystic Study", []string{"enchantment"}, 3, nil)
	b := newTestCardMinimal("Big Bomb", []string{"sorcery"}, 9, nil)
	c := newTestCardMinimal("Mid Spell", []string{"sorcery"}, 5, nil)
	cands := []*gameengine.Card{c, b, a}

	got := h.ChooseTutorTargets(gs, 0, cands, 2, gameast.Filter{Base: "card"}, "hand")
	if len(got) != 2 {
		t.Fatalf("expected 2 picks, got %d", len(got))
	}
	if got[0].Name != "Rhystic Study" {
		t.Errorf("expected tutor target first, got %s", got[0].Name)
	}
	if got[0] == got[1] {
		t.Error("picks must be distinct")
	}
	// Second pick should be the higher-CMC of the remaining two (CMC tiebreak).
	if got[1].Name != "Big Bomb" {
		t.Errorf("expected CMC tiebreak Big Bomb second, got %s", got[1].Name)
	}
}

// ----------------------------------------------------------------------------
// "Let the deck shape the wincon": a deck-synergy engine piece (high-density
// synergy-cluster member) must beat a generic-value card (Eternal Witness),
// even though Eternal Witness is the objectively stronger standalone card and
// is flagged as a value-engine key. This is the exact 7174n1c failure mode.
// ----------------------------------------------------------------------------

func TestChooseTutorTargets_SynergyEngineBeatsGenericValue(t *testing.T) {
	gs := newTestGame(t, 2)

	sp := &StrategyProfile{
		Archetype: ArchetypeMidrange,
		// Eternal Witness is surfaced as a generic value engine key.
		ValueEngineKeys: []string{"Eternal Witness"},
		// The landfall-treasure engine the deck is actually built around
		// lives in a high-density synergy cluster.
		SynergyClusters: []SynergyCluster{
			{
				Name:        "Landfall Treasure",
				Theme:       "landfall",
				Members:     []string{"Tireless Provisioner", "Lotus Cobra", "Scute Swarm"},
				HighDensity: true,
			},
		},
	}
	h := NewYggdrasilHat(sp, 0)
	gs.Seats[0].Hat = h

	witness := libCreature("Eternal Witness", 3)         // generic value, higher CMC
	provisioner := libCreature("Tireless Provisioner", 4) // the synergy engine
	chaff := libCreature("Grizzly Bears", 2)
	cands := []*gameengine.Card{witness, provisioner, chaff}

	got := h.ChooseTutorTargets(gs, 0, cands, 1, gameast.Filter{Base: "creature"}, "hand")
	if len(got) != 1 {
		t.Fatalf("expected 1 pick, got %d", len(got))
	}
	if got[0].Name != "Tireless Provisioner" {
		t.Errorf("expected deck-synergy engine Tireless Provisioner over generic Eternal Witness, got %s", got[0].Name)
	}
}

// ----------------------------------------------------------------------------
// Constrained tutor (transmute / Trinket / Trophy Mage): the engine pre-filters
// the candidate pool to the tutor's legal targets; the chooser must pick the
// best WITHIN that pool. Here the win-line piece is outside the pool, so the
// best legal target (the synergy engine member) should be chosen — never a
// card the tutor could not legally find.
// ----------------------------------------------------------------------------

func TestChooseTutorTargets_ConstrainedPool_BestWithinPool(t *testing.T) {
	gs := newTestGame(t, 2)

	sp := &StrategyProfile{
		Archetype:    ArchetypeControl,
		TutorTargets: []string{"Thassa's Oracle"}, // the real win-line piece, NOT in the legal pool below
		SynergyClusters: []SynergyCluster{
			{Name: "Artifacts", Theme: "artifact", Members: []string{"Sol Ring"}, HighDensity: true},
		},
	}
	h := NewYggdrasilHat(sp, 0)
	gs.Seats[0].Hat = h

	// Trinket-Mage-style pool: only low-CMC artifacts (the win-line piece is
	// a creature and is absent — the engine already excluded it).
	solRing := newTestCardMinimal("Sol Ring", []string{"artifact"}, 1, nil)
	relic := newTestCardMinimal("Relic of Progenitus", []string{"artifact"}, 1, nil)
	cands := []*gameengine.Card{relic, solRing}

	got := h.ChooseTutorTargets(gs, 0, cands, 1, gameast.Filter{Base: "artifact"}, "hand")
	if len(got) != 1 {
		t.Fatalf("expected 1 pick, got %d", len(got))
	}
	if got[0].Name != "Sol Ring" {
		t.Errorf("expected best-within-pool synergy member Sol Ring, got %s", got[0].Name)
	}
}

// ----------------------------------------------------------------------------
// State-aware contextual target value: a graveyard-scaling answer (escape /
// "X equal to cards in your graveyard") is a premium tutor target ONLY when
// the yard is full. With an EMPTY yard the chooser must NOT over-rank it
// (falls to CMC); with a FULL yard it should win over a bigger generic card.
// ----------------------------------------------------------------------------

func graveyardScalingCard(name string, cmc int) *gameengine.Card {
	ast := &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Static{Raw: "escape—{2}, exile four cards from your graveyard. counter target spell unless its controller pays X, where X is the number of cards in your graveyard."},
		},
	}
	return newTestCardMinimal(name, []string{"instant"}, cmc, ast)
}

func TestChooseTutorTargets_GraveyardContextual(t *testing.T) {
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeControl}, 0)

	scaler := graveyardScalingCard("Murktide Counter", 2) // low CMC, yard-scaling
	bomb := newTestCardMinimal("Expropriate", []string{"sorcery"}, 9, nil)

	// Empty yard: the scaler has no contextual edge → highest-CMC bomb wins.
	gsEmpty := newTestGame(t, 2)
	gsEmpty.Seats[0].Hat = h
	gotEmpty := h.ChooseTutorTargets(gsEmpty, 0, []*gameengine.Card{bomb, scaler}, 1, gameast.Filter{Base: "card"}, "hand")
	if len(gotEmpty) != 1 || gotEmpty[0].Name != "Expropriate" {
		t.Errorf("empty yard: expected highest-CMC Expropriate, got %v", gotEmpty)
	}

	// Full yard: the scaler becomes a premium answer → it wins despite lower CMC.
	gsFull := newTestGame(t, 2)
	gsFull.Seats[0].Hat = h
	for i := 0; i < 9; i++ {
		gsFull.Seats[0].Graveyard = append(gsFull.Seats[0].Graveyard,
			newTestCardMinimal("Dead Card "+itoa(i), []string{"instant"}, 1, nil))
	}
	gotFull := h.ChooseTutorTargets(gsFull, 0, []*gameengine.Card{bomb, scaler}, 1, gameast.Filter{Base: "card"}, "hand")
	if len(gotFull) != 1 || gotFull[0].Name != "Murktide Counter" {
		t.Errorf("full yard: expected contextual graveyard-scaler Murktide Counter, got %v", gotFull)
	}
}

// ----------------------------------------------------------------------------
// No strategy signal → graceful degradation to highest-CMC (engine baseline).
// ----------------------------------------------------------------------------

func TestChooseTutorTargets_NoSignal_HighestCMC(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	gs.Seats[0].Hat = h

	small := libCreature("Grizzly Bears", 2)
	big := libCreature("Craterhoof Behemoth", 8)
	cands := []*gameengine.Card{small, big}

	got := h.ChooseTutorTargets(gs, 0, cands, 1, gameast.Filter{Base: "creature"}, "hand")
	if len(got) != 1 || got[0].Name != "Craterhoof Behemoth" {
		t.Fatalf("expected highest-CMC fallback Craterhoof Behemoth, got %v", got)
	}
}
