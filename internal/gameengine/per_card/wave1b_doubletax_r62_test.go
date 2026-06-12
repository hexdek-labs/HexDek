package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// r62 — Wave-1b double-dispatch class, closed. Two mechanisms made
// handlers fire twice per event:
//   (a) inline name-based observer arms in cast_counts.go duplicating
//       per_card handlers (Storm-Kiln, Niv-Mizzet Parun, Smothering
//       Tithe, Orcish Bowmasters — arms deleted, per_card authoritative);
//   (b) registry-level double registration: alias collapse
//       ("opponent_draws"+"card_drawn" → same canonical) and cross-file
//       duplicate register calls (registerDefaults + sibling batch
//       inits) — closed generically by OnTrigger's same-handler dedupe.
// Each test pins EXACT effect counts through the real dispatch.
// -----------------------------------------------------------------------------

func wave1bGame(nSeats int) *gameengine.GameState {
	gs := gameengine.NewGameState(nSeats, nil, nil)
	for i := range gs.Seats {
		for j := 0; j < 10; j++ {
			gs.Seats[i].Library = append(gs.Seats[i].Library,
				&gameengine.Card{Name: "Filler", Owner: i, Types: []string{"sorcery"}})
		}
	}
	return gs
}

func wave1bPerm(gs *gameengine.GameState, seat int, name string, types ...string) *gameengine.Permanent {
	if len(types) == 0 {
		types = []string{"creature"}
	}
	p := &gameengine.Permanent{
		Card: &gameengine.Card{Name: name, Owner: seat, Types: types,
			BasePower: 2, BaseToughness: 2},
		Controller: seat,
		Owner:      seat,
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func countTokens(gs *gameengine.GameState, seat int, name string) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == name {
			n++
		}
	}
	return n
}

func TestWave1b_SmotheringTithe_ExactlyOneTreasurePerDraw(t *testing.T) {
	gs := wave1bGame(2)
	wave1bPerm(gs, 1, "Smothering Tithe", "enchantment")

	gameengine.FireCardTrigger(gs, "card_drawn", map[string]interface{}{
		"seat": 0, "drawer_seat": 0, "card": "Filler", "source": "draw",
	})
	if got := countTokens(gs, 1, "Treasure Token"); got != 1 {
		t.Fatalf("Smothering Tithe should mint exactly 1 Treasure per opponent draw, got %d (pre-r62: 2 via dual registration)", got)
	}
}

func TestWave1b_OrcishBowmasters_ExactlyOnePing_DrawStepExempt(t *testing.T) {
	gs := wave1bGame(2)
	wave1bPerm(gs, 1, "Orcish Bowmasters")
	before := gs.Seats[0].Life

	gameengine.FireCardTrigger(gs, "card_drawn", map[string]interface{}{
		"seat": 0, "drawer_seat": 0, "card": "Filler", "source": "draw",
	})
	if got := before - gs.Seats[0].Life; got != 1 {
		t.Fatalf("Bowmasters should ping exactly 1 per opponent draw, got %d (pre-r62: 2 via dual registration)", got)
	}

	// CR 614.6 — the first draw of the drawer's draw step is exempt.
	before = gs.Seats[0].Life
	gameengine.FireCardTrigger(gs, "card_drawn", map[string]interface{}{
		"seat": 0, "drawer_seat": 0, "card": "Filler", "source": "draw",
		"is_draw_step_draw": true,
	})
	if gs.Seats[0].Life != before {
		t.Fatalf("Bowmasters must not trigger on the first draw-step draw (CR 614.6); life moved %d", before-gs.Seats[0].Life)
	}
}

func TestWave1b_ConsecratedSphinx_DrawsExactlyTwo(t *testing.T) {
	gs := wave1bGame(2)
	wave1bPerm(gs, 1, "Consecrated Sphinx")
	handBefore := len(gs.Seats[1].Hand)

	gameengine.FireCardTrigger(gs, "card_drawn", map[string]interface{}{
		"seat": 0, "drawer_seat": 0, "card": "Filler", "source": "draw",
	})
	if got := len(gs.Seats[1].Hand) - handBefore; got != 2 {
		t.Fatalf("Sphinx should draw exactly 2 per opponent draw, got %d (pre-r62: 4 via dual registration)", got)
	}
}

func TestWave1b_Nekusar_ExactlyOneDamagePerDraw(t *testing.T) {
	gs := wave1bGame(2)
	wave1bPerm(gs, 1, "Nekusar, the Mindrazer")
	before := gs.Seats[0].Life

	gameengine.FireCardTrigger(gs, "card_drawn", map[string]interface{}{
		"seat": 0, "drawer_seat": 0, "card": "Filler", "source": "draw",
	})
	if got := before - gs.Seats[0].Life; got != 1 {
		t.Fatalf("Nekusar should deal exactly 1 per draw, got %d (pre-r62: 2 via dual registration)", got)
	}
}

func TestWave1b_StormKiln_ExactlyOneTreasurePerCast(t *testing.T) {
	gs := wave1bGame(2)
	kiln := wave1bPerm(gs, 0, "Storm-Kiln Artist")
	kiln.Card.AST = &gameast.CardAST{
		Name:      "Storm-Kiln Artist",
		Abilities: []gameast.Ability{&gameast.Keyword{Name: "magecraft"}},
	}
	spell := &gameengine.Card{Name: "Shock", Owner: 0, Types: []string{"instant", "cost:1"}}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, spell)
	gs.Seats[0].ManaPool = 1
	gameengine.EnsureTypedPool(gs.Seats[0])
	gs.Active = 0
	gs.Phase = "main"

	if err := gameengine.CastSpell(gs, 0, spell, nil); err != nil {
		t.Fatalf("CastSpell: %v", err)
	}
	if got := countTokens(gs, 0, "Treasure Token"); got != 1 {
		t.Fatalf("Storm-Kiln should mint exactly 1 Treasure per instant/sorcery cast, got %d (pre-r62: 2 via inline+per_card double dispatch)", got)
	}
}

func TestWave1b_NivMizzetParun_DrawsExactlyOnePerCast(t *testing.T) {
	gs := wave1bGame(2)
	wave1bPerm(gs, 0, "Niv-Mizzet, Parun")
	// NOT a real card name — "Opt" collided with the registry's per_card
	// resolve handler (scry 1, draw a card), whose printed draw read as a
	// phantom double during test development.
	spell := &gameengine.Card{Name: "Wave1b Probe Bolt", Owner: 0, Types: []string{"instant", "cost:1"}}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, spell)
	gs.Seats[0].ManaPool = 1
	gameengine.EnsureTypedPool(gs.Seats[0])
	gs.Active = 0
	gs.Phase = "main"
	libBefore := len(gs.Seats[0].Library)

	if err := gameengine.CastSpell(gs, 0, spell, nil); err != nil {
		t.Fatalf("CastSpell: %v", err)
	}
	if got := libBefore - len(gs.Seats[0].Library); got != 1 {
		t.Fatalf("Niv-Mizzet should draw exactly 1 per own instant/sorcery cast, got %d (pre-r62: 2 via inline+per_card double dispatch)", got)
	}
}

// Registry-level guard: registering the SAME handler twice for events
// that share a canonical name is a single registration; two DISTINCT
// handlers (a genuine two-ability card) still both register.
func TestWave1b_Registry_OnTriggerDedupesSameHandler(t *testing.T) {
	r := Global()
	h := func(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {}
	r.OnTrigger("Wave1b Dedupe Probe", "card_drawn", h)
	r.OnTrigger("Wave1b Dedupe Probe", "opponent_draws", h) // alias of card_drawn
	h2 := func(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {}
	r.OnTrigger("Wave1b Dedupe Probe", "card_drawn", h2)

	count := 0
	for _, reg := range r.EnumerateOnTriggerRegistrations() {
		if reg.CardName == NormalizeName("Wave1b Dedupe Probe") && reg.Event == "card_drawn" {
			count = reg.HandlerCount
		}
	}
	if count != 2 {
		t.Fatalf("want 2 handlers (h deduped across alias, h2 distinct), got %d", count)
	}
	_ = gameast.Filter{} // keep gameast import if unused elsewhere
}
