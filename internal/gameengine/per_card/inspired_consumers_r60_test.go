package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// inspired_consumers_r60 — 6 handlers wired in inspired_consumers_r60.go
// covering the BNG Inspired keyword family (CR §702.124).
// -----------------------------------------------------------------------------

// makeInspiredPerm builds a tapped permanent with the inspired
// keyword granted via the kw:inspired flag (the runtime keyword-grant
// path Permanent.HasKeyword consults). Returns the perm ready to be
// untapped via UntapPermanent so the engine fires
// FireInspiredTriggers naturally.
//
// The default 2/2 stats keep SBA §704.5f happy — a perm with no
// BasePower/BaseToughness is a 0/0, which gets destroyed
// immediately when the trigger handler invokes
// InvalidateCharacteristicsCache. The default is intentionally
// generic (most BNG inspired creatures sit in the 1-4 P/T range).
// Individual tests override when they need exact stats (e.g. Felhide
// copying a creature with a specific P/T).
func makeInspiredPerm(gs *gameengine.GameState, seat int, name string) *gameengine.Permanent {
	p := addPerm(gs, seat, name, "creature")
	p.Flags["kw:inspired"] = 1
	p.Tapped = true
	p.Card.BasePower = 2
	p.Card.BaseToughness = 2
	return p
}

// triggerInspired flips perm tapped→untapped via the engine's canonical
// mid-turn untap entry point so the inspired trigger fires naturally
// (rather than calling FireInspiredTriggers directly). Exercises the
// same code path UntapAll uses during the untap step.
func triggerInspired(gs *gameengine.GameState, p *gameengine.Permanent) {
	gameengine.UntapPermanent(gs, p, "test_inspired_trigger")
}

// -----------------------------------------------------------------------------
// Oreskos Sun Guide — Inspired: gain 2 life
// -----------------------------------------------------------------------------

func TestOreskosSunGuide_InspiredGains2Life(t *testing.T) {
	gs := newGame(t, 2)
	guide := makeInspiredPerm(gs, 0, "Oreskos Sun Guide")
	startLife := gs.Seats[0].Life

	triggerInspired(gs, guide)

	if gs.Seats[0].Life != startLife+2 {
		t.Errorf("expected +2 life from inspired, got %d → %d", startLife, gs.Seats[0].Life)
	}
}

// -----------------------------------------------------------------------------
// Sphinx's Disciple — Inspired: draw a card
// -----------------------------------------------------------------------------

func TestSphinxsDisciple_InspiredDraws(t *testing.T) {
	gs := newGame(t, 2)
	disciple := makeInspiredPerm(gs, 0, "Sphinx's Disciple")
	addLibrary(gs, 0, "Top1", "Top2")
	startHand := len(gs.Seats[0].Hand)

	triggerInspired(gs, disciple)

	if len(gs.Seats[0].Hand) != startHand+1 {
		t.Errorf("expected +1 card in hand, got hand size %d → %d", startHand, len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].Hand[startHand].DisplayName() != "Top1" {
		t.Errorf("expected Top1 drawn, got %s", gs.Seats[0].Hand[startHand].DisplayName())
	}
}

func TestSphinxsDisciple_InspiredEmptyLibraryFails(t *testing.T) {
	gs := newGame(t, 2)
	disciple := makeInspiredPerm(gs, 0, "Sphinx's Disciple")

	triggerInspired(gs, disciple)

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed on empty library; events=%+v", gs.EventLog)
	}
}

// -----------------------------------------------------------------------------
// Pheres-Band Tromper — Inspired: +1/+1 counter on self
// -----------------------------------------------------------------------------

func TestPheresBandTromper_InspiredAddsCounter(t *testing.T) {
	gs := newGame(t, 2)
	tromper := makeInspiredPerm(gs, 0, "Pheres-Band Tromper")
	if tromper.Counters["+1/+1"] != 0 {
		t.Fatalf("expected 0 counters pre-trigger, got %d", tromper.Counters["+1/+1"])
	}

	triggerInspired(gs, tromper)

	if tromper.Counters["+1/+1"] != 1 {
		t.Errorf("expected 1 +1/+1 counter after inspired, got %d", tromper.Counters["+1/+1"])
	}
}

func TestPheresBandTromper_InspiredStacksAcrossMultipleUntaps(t *testing.T) {
	gs := newGame(t, 2)
	tromper := makeInspiredPerm(gs, 0, "Pheres-Band Tromper")

	// Repeated untap cycles — each transition adds a counter (CR §702.124).
	for i := 0; i < 3; i++ {
		triggerInspired(gs, tromper)
		tromper.Tapped = true
	}

	if tromper.Counters["+1/+1"] != 3 {
		t.Errorf("expected 3 counters after 3 untap cycles, got %d", tromper.Counters["+1/+1"])
	}
}

// -----------------------------------------------------------------------------
// Servant of Tymaret — Inspired: each opp loses 1, you gain equal life
// -----------------------------------------------------------------------------

func TestServantOfTymaret_InspiredDrainsAndGains(t *testing.T) {
	gs := newGame(t, 4)
	servant := makeInspiredPerm(gs, 0, "Servant of Tymaret")
	startLife0 := gs.Seats[0].Life
	startLife1 := gs.Seats[1].Life
	startLife2 := gs.Seats[2].Life
	startLife3 := gs.Seats[3].Life

	triggerInspired(gs, servant)

	if gs.Seats[1].Life != startLife1-1 {
		t.Errorf("opp 1 should lose 1 life, got %d → %d", startLife1, gs.Seats[1].Life)
	}
	if gs.Seats[2].Life != startLife2-1 {
		t.Errorf("opp 2 should lose 1 life, got %d → %d", startLife2, gs.Seats[2].Life)
	}
	if gs.Seats[3].Life != startLife3-1 {
		t.Errorf("opp 3 should lose 1 life, got %d → %d", startLife3, gs.Seats[3].Life)
	}
	// 3 opps × 1 life each = 3 life gained.
	if gs.Seats[0].Life != startLife0+3 {
		t.Errorf("controller should gain 3 life (3 opps lost 1 each), got %d → %d", startLife0, gs.Seats[0].Life)
	}
}

func TestServantOfTymaret_InspiredSkipsLostOpponents(t *testing.T) {
	gs := newGame(t, 4)
	servant := makeInspiredPerm(gs, 0, "Servant of Tymaret")
	gs.Seats[2].Lost = true
	startLife0 := gs.Seats[0].Life

	triggerInspired(gs, servant)

	// Only seats 1 and 3 (alive opps) lose life; seat 2 is skipped.
	if gs.Seats[0].Life != startLife0+2 {
		t.Errorf("should gain 2 (only 2 alive opps), got %d → %d", startLife0, gs.Seats[0].Life)
	}
}

// -----------------------------------------------------------------------------
// Pain Seer — Inspired: reveal top, to hand, lose life = CMC
// -----------------------------------------------------------------------------

func TestPainSeer_InspiredRevealsToHandAndPaysLife(t *testing.T) {
	gs := newGame(t, 2)
	seer := makeInspiredPerm(gs, 0, "Pain Seer")
	// Top of library is a CMC-4 card.
	gs.Seats[0].Library = append(gs.Seats[0].Library, &gameengine.Card{
		Name:  "BigSpell",
		Owner: 0,
		CMC:   4,
		Types: []string{"sorcery"},
	})
	startLife := gs.Seats[0].Life

	triggerInspired(gs, seer)

	// Card should be in hand.
	found := false
	for _, c := range gs.Seats[0].Hand {
		if c.DisplayName() == "BigSpell" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected BigSpell in hand after Pain Seer inspired")
	}
	if gs.Seats[0].Life != startLife-4 {
		t.Errorf("expected -4 life (CMC 4), got %d → %d", startLife, gs.Seats[0].Life)
	}
}

func TestPainSeer_InspiredCMCZeroDoesNotLoseLife(t *testing.T) {
	gs := newGame(t, 2)
	seer := makeInspiredPerm(gs, 0, "Pain Seer")
	gs.Seats[0].Library = append(gs.Seats[0].Library, &gameengine.Card{
		Name:  "Mox",
		Owner: 0,
		CMC:   0,
		Types: []string{"artifact"},
	})
	startLife := gs.Seats[0].Life

	triggerInspired(gs, seer)

	if gs.Seats[0].Life != startLife {
		t.Errorf("CMC 0 reveal must not lose life, got %d → %d", startLife, gs.Seats[0].Life)
	}
}

// -----------------------------------------------------------------------------
// Felhide Spiritbinder — Inspired: pay {1}{R} (→ 2 mana), create haste
// enchantment token copy, exile at next end step
// -----------------------------------------------------------------------------

func TestFelhideSpiritbinder_InspiredCopiesTargetCreature(t *testing.T) {
	gs := newGame(t, 2)
	binder := makeInspiredPerm(gs, 0, "Felhide Spiritbinder")
	gs.Seats[0].ManaPool = 5
	// Friendly target — Big Threat with 5/5 stats.
	target := addPerm(gs, 0, "Big Threat", "creature")
	target.Card.BasePower, target.Card.BaseToughness = 5, 5

	preCount := len(gs.Seats[0].Battlefield)
	triggerInspired(gs, binder)

	// Mana paid: 5 - 2 = 3.
	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("expected mana pool 3 after paying 2, got %d", gs.Seats[0].ManaPool)
	}
	// A new permanent should have entered the battlefield.
	if len(gs.Seats[0].Battlefield) != preCount+1 {
		t.Errorf("expected 1 new token on battlefield, got delta %d", len(gs.Seats[0].Battlefield)-preCount)
	}
	// Find the token: it should have haste + enchantment in Types.
	var token *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card == nil || p == binder || p == target {
			continue
		}
		token = p
		break
	}
	if token == nil {
		t.Fatalf("could not find the minted token")
	}
	if token.Flags["kw:haste"] != 1 {
		t.Errorf("token must have kw:haste flag, got %d", token.Flags["kw:haste"])
	}
	if !cardHasType(token.Card, "enchantment") {
		t.Errorf("token must be an enchantment in addition to its other types, got %v", token.Card.Types)
	}
	if token.Card.BasePower != 5 || token.Card.BaseToughness != 5 {
		t.Errorf("token must copy P/T 5/5, got %d/%d", token.Card.BasePower, token.Card.BaseToughness)
	}
}

func TestFelhideSpiritbinder_InspiredFailsWithoutMana(t *testing.T) {
	gs := newGame(t, 2)
	_ = makeInspiredPerm(gs, 0, "Felhide Spiritbinder")
	binder := gs.Seats[0].Battlefield[0]
	gs.Seats[0].ManaPool = 1 // not enough for {1}{R} = 2
	target := addPerm(gs, 0, "Big Threat", "creature")
	// Give Big Threat real stats so SBA §704.5f doesn't kill it
	// independently — we want the assertion to fail ONLY on token
	// creation, not on the test fixture's 0/0 collateral damage.
	target.Card.BasePower, target.Card.BaseToughness = 5, 5

	triggerInspired(gs, binder)

	// Look for any Spiritbinder-token perm — its name carries the
	// "(Spiritbinder token)" rider so we can distinguish it from the
	// vanilla setup perms.
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if cardHasType(p.Card, "token") {
			t.Errorf("must not mint a token without sufficient mana; found %s", p.Card.DisplayName())
		}
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when mana insufficient; events=%+v", gs.EventLog)
	}
}

func TestFelhideSpiritbinder_InspiredFailsWithoutTarget(t *testing.T) {
	gs := newGame(t, 2)
	binder := makeInspiredPerm(gs, 0, "Felhide Spiritbinder")
	gs.Seats[0].ManaPool = 5
	// No OTHER creature on the battlefield — only the binder itself,
	// which is excluded by the "another target" filter.
	preCount := len(gs.Seats[0].Battlefield)

	triggerInspired(gs, binder)

	if len(gs.Seats[0].Battlefield) != preCount {
		t.Errorf("must not mint a token without a valid 'another' target")
	}
	if gs.Seats[0].ManaPool != 5 {
		t.Errorf("must not spend mana when target picker fails, got %d", gs.Seats[0].ManaPool)
	}
}

func TestFelhideSpiritbinder_InspiredRegistersEOTExile(t *testing.T) {
	gs := newGame(t, 2)
	binder := makeInspiredPerm(gs, 0, "Felhide Spiritbinder")
	gs.Seats[0].ManaPool = 5
	target := addPerm(gs, 0, "Big Threat", "creature")
	target.Card.BasePower, target.Card.BaseToughness = 5, 5

	preDelayed := len(gs.DelayedTriggers)
	triggerInspired(gs, binder)

	if len(gs.DelayedTriggers) != preDelayed+1 {
		t.Errorf("expected 1 delayed trigger registered for end-step exile, got delta %d",
			len(gs.DelayedTriggers)-preDelayed)
	}
	dt := gs.DelayedTriggers[len(gs.DelayedTriggers)-1]
	if dt.TriggerAt != "next_end_step" {
		t.Errorf("expected next_end_step trigger, got %q", dt.TriggerAt)
	}
}

// -----------------------------------------------------------------------------
// Source guard — verify the per_card dispatcher's
// fires-on-every-matching-perm walk doesn't over-fire when two same-
// name creatures are in play.
// -----------------------------------------------------------------------------

func TestInspiredSourceGuard_OnlyFiresOnUntappedPerm(t *testing.T) {
	gs := newGame(t, 2)
	// Two Oreskos Sun Guides on the battlefield. Only one will actually
	// transition tapped → untapped this turn.
	g1 := makeInspiredPerm(gs, 0, "Oreskos Sun Guide")
	g2 := makeInspiredPerm(gs, 0, "Oreskos Sun Guide")
	g2.Tapped = false // already untapped — no transition, no trigger
	startLife := gs.Seats[0].Life

	triggerInspired(gs, g1)

	// Only one trigger should have landed (g1) → +2 life, not +4.
	if gs.Seats[0].Life != startLife+2 {
		t.Errorf("source guard must prevent over-fire: expected +2 life, got %d → %d",
			startLife, gs.Seats[0].Life)
	}
}

// -----------------------------------------------------------------------------
// Stun-counter veto — §122.4 stun replaces an untap, so no transition
// occurs and the inspired trigger must NOT fire.
// -----------------------------------------------------------------------------

func TestInspiredStunCounterVetoesTrigger(t *testing.T) {
	gs := newGame(t, 2)
	guide := makeInspiredPerm(gs, 0, "Oreskos Sun Guide")
	guide.Counters["stun"] = 1
	startLife := gs.Seats[0].Life

	triggerInspired(gs, guide)

	if gs.Seats[0].Life != startLife {
		t.Errorf("stun-vetoed untap must not fire inspired; got life %d → %d", startLife, gs.Seats[0].Life)
	}
	if !guide.Tapped {
		t.Errorf("stun-vetoed perm must remain tapped (engine §122.4)")
	}
}

// -----------------------------------------------------------------------------
// Registry smoke test — pin that all 6 handlers are wired
// -----------------------------------------------------------------------------

func TestInspiredConsumersR60_AllHandlersRegistered(t *testing.T) {
	cards := []string{
		"Oreskos Sun Guide",
		"Sphinx's Disciple",
		"Pheres-Band Tromper",
		"Servant of Tymaret",
		"Pain Seer",
		"Felhide Spiritbinder",
	}
	reg := Global()
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	for _, name := range cards {
		k := normalizeName(name)
		byEvent := reg.onTrigger[k]
		if byEvent == nil || len(byEvent["inspired"]) == 0 {
			t.Errorf("missing inspired OnTrigger handler for %q", name)
		}
	}
}
