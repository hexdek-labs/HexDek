package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// eerie_consumers_r60 — 6 handlers wired in eerie_consumers_r60.go
// covering the DSK Eerie keyword family (CR §702.182).
// -----------------------------------------------------------------------------

// eerieCtx builds the ctx OnEnchantmentETB / OnRoomFullyUnlocked forward.
func eerieCtx(source, trigger *gameengine.Permanent, cause string) map[string]interface{} {
	return map[string]interface{}{
		"trigger_perm":    trigger,
		"eerie_source":    source,
		"controller_seat": source.Controller,
		"cause":           cause,
	}
}

// -----------------------------------------------------------------------------
// Entity Tracker — draw a card on eerie
// -----------------------------------------------------------------------------

func TestEntityTracker_DrawsOnEerie(t *testing.T) {
	gs := newGame(t, 2)
	tracker := addPerm(gs, 0, "Entity Tracker", "creature")
	tracker.Card.BasePower, tracker.Card.BaseToughness = 2, 2
	addLibrary(gs, 0, "Top", "Next")
	enchant := addPerm(gs, 0, "Wedding Announcement", "enchantment")

	gameengine.FireCardTrigger(gs, "eerie", eerieCtx(tracker, enchant, "enchantment_etb"))

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card drawn from Entity Tracker eerie, got hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestEntityTracker_DrawsOnRoomUnlock(t *testing.T) {
	gs := newGame(t, 2)
	tracker := addPerm(gs, 0, "Entity Tracker", "creature")
	tracker.Card.BasePower, tracker.Card.BaseToughness = 2, 2
	addLibrary(gs, 0, "Top")
	room := addPerm(gs, 0, "Some Room", "enchantment", "room")

	gameengine.FireCardTrigger(gs, "eerie", eerieCtx(tracker, room, "room_unlocked"))

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("eerie must fire on room_unlocked cause too; hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestEntityTracker_IgnoresDispatchToWrongPerm(t *testing.T) {
	gs := newGame(t, 2)
	tracker := addPerm(gs, 0, "Entity Tracker", "creature")
	tracker.Card.BasePower, tracker.Card.BaseToughness = 2, 2
	// Add a SECOND Entity Tracker so the dispatcher could fire the
	// handler against either; eerieGuard's `eerie_source == perm` check
	// must keep them isolated.
	other := addPerm(gs, 0, "Entity Tracker", "creature")
	other.Card.BasePower, other.Card.BaseToughness = 2, 2
	addLibrary(gs, 0, "Top", "Next", "Third")
	enchant := addPerm(gs, 0, "Wedding Announcement", "enchantment")

	// Manually fire with eerie_source = tracker (the first). The
	// dispatcher walks battlefield by name and may call our handler with
	// perm = other; eerieGuard returns nil so other doesn't double-draw.
	// We model this by calling the handler directly (per_card has no way
	// to scope FireCardTrigger to a single bearer).
	entityTrackerEerie(gs, other, eerieCtx(tracker, enchant, "enchantment_etb"))

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("eerie should only fire on the matching eerie_source; hand=%d", len(gs.Seats[0].Hand))
	}

	// And the matching dispatch DOES fire.
	entityTrackerEerie(gs, tracker, eerieCtx(tracker, enchant, "enchantment_etb"))
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("matching dispatch should draw 1; hand=%d", len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// Balemurk Leech — each opp loses 1
// -----------------------------------------------------------------------------

func TestBalemurkLeech_DrainsEachOpp(t *testing.T) {
	gs := newGame(t, 4)
	leech := addPerm(gs, 0, "Balemurk Leech", "creature")
	leech.Card.BasePower, leech.Card.BaseToughness = 2, 1
	for i := 0; i < 4; i++ {
		gs.Seats[i].Life = 20
	}
	enchant := addPerm(gs, 0, "Wedding Announcement", "enchantment")

	gameengine.FireCardTrigger(gs, "eerie", eerieCtx(leech, enchant, "enchantment_etb"))

	if gs.Seats[0].Life != 20 {
		t.Errorf("controller life unchanged; got %d", gs.Seats[0].Life)
	}
	for i := 1; i < 4; i++ {
		if gs.Seats[i].Life != 19 {
			t.Errorf("opp %d should lose 1 life; got %d", i, gs.Seats[i].Life)
		}
	}
}

func TestBalemurkLeech_SkipsLostOpps(t *testing.T) {
	gs := newGame(t, 4)
	leech := addPerm(gs, 0, "Balemurk Leech", "creature")
	leech.Card.BasePower, leech.Card.BaseToughness = 2, 1
	gs.Seats[3].Life = 0
	gs.Seats[3].Lost = true
	enchant := addPerm(gs, 0, "Wedding Announcement", "enchantment")

	gameengine.FireCardTrigger(gs, "eerie", eerieCtx(leech, enchant, "enchantment_etb"))

	if gs.Seats[3].Life != 0 {
		t.Errorf("eliminated opp should not be drained; got %d", gs.Seats[3].Life)
	}
}

// -----------------------------------------------------------------------------
// Skullsnap Nuisance — surveil 1
// -----------------------------------------------------------------------------

func TestSkullsnapNuisance_SurveilsOneOnEerie(t *testing.T) {
	gs := newGame(t, 2)
	nuisance := addPerm(gs, 0, "Skullsnap Nuisance", "creature")
	nuisance.Card.BasePower, nuisance.Card.BaseToughness = 2, 2
	addLibrary(gs, 0, "TopCard")
	enchant := addPerm(gs, 0, "Wedding Announcement", "enchantment")

	// No hat installed → Surveil's default keeps the card on top.
	gameengine.FireCardTrigger(gs, "eerie", eerieCtx(nuisance, enchant, "enchantment_etb"))

	// Verify Surveil emitted its log event (the only stable proxy we
	// have without a hat installed).
	if hasEvent(gs, "surveil") < 1 && hasEvent(gs, "per_card_handler") < 1 {
		t.Errorf("expected surveil or per_card_handler event from Skullsnap Nuisance; got %+v", gs.EventLog)
	}
}

// -----------------------------------------------------------------------------
// Erratic Apparition — self +1/+1 UEOT
// -----------------------------------------------------------------------------

func TestErraticApparition_PumpsSelfOnEerie(t *testing.T) {
	gs := newGame(t, 2)
	app := addPerm(gs, 0, "Erratic Apparition", "creature")
	app.Card.BasePower, app.Card.BaseToughness = 2, 2
	enchant := addPerm(gs, 0, "Wedding Announcement", "enchantment")

	gameengine.FireCardTrigger(gs, "eerie", eerieCtx(app, enchant, "enchantment_etb"))

	if app.Power() != 3 || app.Toughness() != 3 {
		t.Errorf("expected 3/3 after +1/+1 pump, got %d/%d", app.Power(), app.Toughness())
	}
}

func TestErraticApparition_StacksMultipleEerieFires(t *testing.T) {
	gs := newGame(t, 2)
	app := addPerm(gs, 0, "Erratic Apparition", "creature")
	app.Card.BasePower, app.Card.BaseToughness = 2, 2
	enchant := addPerm(gs, 0, "Wedding Announcement", "enchantment")

	for i := 0; i < 3; i++ {
		gameengine.FireCardTrigger(gs, "eerie", eerieCtx(app, enchant, "enchantment_etb"))
	}

	if app.Power() != 5 || app.Toughness() != 5 {
		t.Errorf("expected 5/5 after 3 eerie fires, got %d/%d", app.Power(), app.Toughness())
	}
}

// -----------------------------------------------------------------------------
// Gremlin Tamer — create 1/1 red Gremlin
// -----------------------------------------------------------------------------

func TestGremlinTamer_CreatesGremlinTokenOnEerie(t *testing.T) {
	gs := newGame(t, 2)
	tamer := addPerm(gs, 0, "Gremlin Tamer", "creature")
	tamer.Card.BasePower, tamer.Card.BaseToughness = 2, 2
	enchant := addPerm(gs, 0, "Wedding Announcement", "enchantment")
	pre := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "eerie", eerieCtx(tamer, enchant, "enchantment_etb"))

	if len(gs.Seats[0].Battlefield) != pre+1 {
		t.Errorf("expected battlefield +1 (Gremlin token), pre=%d post=%d",
			pre, len(gs.Seats[0].Battlefield))
	}
	foundGremlin := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.DisplayName() == "Gremlin" && p.Card.BasePower == 1 && p.Card.BaseToughness == 1 {
			foundGremlin = true
			break
		}
	}
	if !foundGremlin {
		t.Errorf("expected a 1/1 Gremlin on battlefield")
	}
}

// -----------------------------------------------------------------------------
// Optimistic Scavenger — +1/+1 counter on highest-power friendly creature
// -----------------------------------------------------------------------------

func TestOptimisticScavenger_PutsCounterOnHighestFriendly(t *testing.T) {
	gs := newGame(t, 2)
	scav := addPerm(gs, 0, "Optimistic Scavenger", "creature")
	scav.Card.BasePower, scav.Card.BaseToughness = 1, 1

	weak := addPerm(gs, 0, "Llanowar Elves", "creature")
	weak.Card.BasePower, weak.Card.BaseToughness = 1, 1

	big := addPerm(gs, 0, "Craterhoof", "creature")
	big.Card.BasePower, big.Card.BaseToughness = 5, 5

	enchant := addPerm(gs, 0, "Wedding Announcement", "enchantment")
	gameengine.FireCardTrigger(gs, "eerie", eerieCtx(scav, enchant, "enchantment_etb"))

	if big.Counters["+1/+1"] != 1 {
		t.Errorf("expected counter on highest-power Craterhoof; got %d", big.Counters["+1/+1"])
	}
	if weak.Counters["+1/+1"] != 0 {
		t.Errorf("weakest creature should not be the target; got %d", weak.Counters["+1/+1"])
	}
}

func TestOptimisticScavenger_IgnoresOpponentCreatures(t *testing.T) {
	gs := newGame(t, 2)
	scav := addPerm(gs, 0, "Optimistic Scavenger", "creature")
	scav.Card.BasePower, scav.Card.BaseToughness = 1, 1

	oppHulk := addPerm(gs, 1, "Massive Dragon", "creature")
	oppHulk.Card.BasePower, oppHulk.Card.BaseToughness = 10, 10

	enchant := addPerm(gs, 0, "Wedding Announcement", "enchantment")
	gameengine.FireCardTrigger(gs, "eerie", eerieCtx(scav, enchant, "enchantment_etb"))

	if oppHulk.Counters["+1/+1"] != 0 {
		t.Errorf("opponent creature must never receive the counter; got %d", oppHulk.Counters["+1/+1"])
	}
	// Sole friendly creature is Scavenger itself.
	if scav.Counters["+1/+1"] != 1 {
		t.Errorf("Scavenger should self-target (sole friendly creature); got %d", scav.Counters["+1/+1"])
	}
}

// -----------------------------------------------------------------------------
// Registry smoke — every handler reachable post-registerDefaults.
// -----------------------------------------------------------------------------

func TestEerieConsumers_AllRegistered(t *testing.T) {
	cards := []string{
		"Entity Tracker",
		"Balemurk Leech",
		"Skullsnap Nuisance",
		"Erratic Apparition",
		"Gremlin Tamer",
		"Optimistic Scavenger",
	}
	reg := Global()
	for _, name := range cards {
		_, hasTrigger := reg.HasCastAndTrigger(normalizeName(name))
		if !hasTrigger {
			t.Errorf("expected OnTrigger handler for %s", name)
		}
	}
}
