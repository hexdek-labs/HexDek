package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r98 — the ExileOnResolve audit (item (M)). Every per_card site that
// sets the flag was checked against the card's printed oracle text,
// re-fetched in-session. Eleven were correct. Three were not, and these
// pin the three.
//
// The flag is INERT on the resolve path for a permanent spell — the
// exile branch in stack.go is an else-if after the ETB arm, measured —
// so these assert on the OFF-BOARD path, which has no permanent-spell
// check: postStackZoneForOffboard consults ShouldExileOnResolve
// directly. That split is the same shape as the r96 escape defect,
// where the counter path and the resolve path had to be fixed
// separately.

// grantForR98 returns the zone-cast grant registered on card, or nil.
func grantForR98(gs *gameengine.GameState, card *gameengine.Card) *gameengine.ZoneCastPermission {
	return gs.ZoneCastGrants[card]
}

// ---------------------------------------------------------------------
// Emry, Lurker of the Loch
//
// Oracle (r98): "{T}: Choose target artifact card in your graveyard.
// You may cast that card this turn. (You still pay its costs. Timing
// rules still apply.)" — no exile clause of any kind. The handler used
// to set ExileOnResolve and justify it by quoting a clause that is on
// Kess and Maestros Ascendancy, not on Emry.
// ---------------------------------------------------------------------

func TestExileOnResolveR98_EmryGrantDoesNotExile(t *testing.T) {
	gs := newGame(t, 2)
	Reset()
	emry := r89Creature(gs, 0, "Emry, Lurker of the Loch", "creature", "merfolk")
	art := &gameengine.Card{Name: "R98 Relic", Owner: 0, Types: []string{"artifact"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, art)

	gameengine.InvokeActivatedHook(gs, emry, 0, nil)

	g := grantForR98(gs, art)
	if g == nil {
		t.Fatalf("fixture: Emry registered no grant on the graveyard artifact")
	}
	if g.ExileOnResolve {
		t.Errorf("Emry's grant sets ExileOnResolve; the card has no exile clause, " +
			"so a countered Emry-cast artifact is exiled instead of going to the graveyard")
	}
}

// ---------------------------------------------------------------------
// Tannuk, Steadfast Second — warp.
//
// Oracle (r98): "...warp {2}{R}. (You may cast a card from your hand
// for its warp cost. Exile that permanent at the beginning of the next
// end step, then you may cast it from exile on a later turn.)"
//
// Warp exiles the PERMANENT at the next end step, after it has entered.
// ExileOnResolve exiles the SPELL as it leaves the stack, which is a
// different event; it cannot model warp's half and does harm on the
// countered path.
// ---------------------------------------------------------------------

func TestExileOnResolveR98_TannukWarpPolicyDoesNotExile(t *testing.T) {
	gs := newGame(t, 2)
	Reset()
	tannuk := r89Creature(gs, 0, "Tannuk, Steadfast Second", "creature", "dwarf")

	tannukETBHasteAnthem(gs, tannuk)

	found := false
	for _, pol := range gs.ZoneCastPolicies {
		if pol == nil || pol.HandlerID != "tannuk_warp_2r_grant" {
			continue
		}
		found = true
		if pol.ExileOnResolve {
			t.Errorf("Tannuk's warp policy sets ExileOnResolve; warp exiles the PERMANENT " +
				"at the next end step, not the spell as it leaves the stack")
		}
	}
	if !found {
		t.Skip("Tannuk's warp policy not registered by this dispatch shape")
	}
}

// ---------------------------------------------------------------------
// Force of Negation — the oracle text IS correct ("exile it instead of
// putting it into its owner's graveyard"); the defect was that the
// handler ASSIGNED a fresh CostMeta map, discarding whatever the
// countered spell already carried. Asserted positively: the pre-existing
// key survives AND the exile flag is set.
// ---------------------------------------------------------------------

func TestExileOnResolveR98_ForceOfNegationKeepsExistingCostMeta(t *testing.T) {
	gs := newGame(t, 2)
	Reset()
	victim := &gameengine.StackItem{
		Card:       &gameengine.Card{Name: "R98 Victim", Owner: 1, Types: []string{"enchantment"}},
		Controller: 1,
		CostMeta: map[string]interface{}{
			"zone_cast_keyword": "flashback",
			"escape_cast":       true,
		},
	}
	gameengine.PushStackItem(gs, victim)

	fon := &gameengine.StackItem{
		Card:       &gameengine.Card{Name: "Force of Negation", Owner: 0, Types: []string{"instant"}},
		Controller: 0,
	}
	forceOfNegationResolve(gs, fon)

	if !victim.Countered {
		t.Skip("Force of Negation did not counter in this fixture shape")
	}
	if v, _ := victim.CostMeta["exile_on_resolve"].(bool); !v {
		t.Errorf("Force of Negation did not set exile_on_resolve")
	}
	if victim.CostMeta["zone_cast_keyword"] != "flashback" {
		t.Errorf("Force of Negation discarded the victim's pre-existing CostMeta "+
			"(zone_cast_keyword gone); got %+v", victim.CostMeta)
	}
	if v, _ := victim.CostMeta["escape_cast"].(bool); !v {
		t.Errorf("Force of Negation discarded the victim's escape_cast marker; got %+v",
			victim.CostMeta)
	}
}
