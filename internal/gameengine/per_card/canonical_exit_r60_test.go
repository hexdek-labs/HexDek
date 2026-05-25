package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// canonical_exit_r60_test.go — regression tests for the May 11 grinder
// forensics sibling sweep (CLAUDE.md issue log 2026-05-16). Five per_card
// handlers (etrata_the_silencer, zabaz, zimone_and_dina, bilbo, thassa)
// previously routed battlefield exits through the package-local
// removePermanent + moveCardBetweenZones pair, bypassing the canonical
// gameengine.ExilePermanent / DestroyPermanent / SacrificePermanent /
// BouncePermanent APIs and silently skipping:
//
//   - §614 would_be_exiled / would_die replacement chains
//   - §903.9b commander zone redirect
//   - DetachAll for auras + equipment
//   - UnregisterReplacementsForPermanent for the leaving permanent
//   - LTB / dies / sacrifice triggers via FireZoneChangeTriggers
//   - "exile" / "destroy" / "sacrifice" event log entries
//
// Each test below registers a §614 replacement, attaches an aura, and
// invokes the per_card hook. It then asserts the canonical machinery
// fired — replacement applied, aura detached, canonical event logged.

// attachAura is a tiny helper that flags one perm as attached to another
// so DetachAll has something observable to clear.
func attachAura(aura, target *gameengine.Permanent) {
	aura.AttachedTo = target
}

// registerReplacementWatcher registers a no-op §614 replacement on the
// given EventType. It returns a pointer to an int that gets incremented
// whenever ApplyFn fires — letting the test assert the replacement chain
// was reached. Setting cancel=true cancels the event entirely.
func registerReplacementWatcher(gs *gameengine.GameState, eventType string, cancel bool) *int {
	counter := new(int)
	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      eventType,
		HandlerID:      "test:watcher:" + eventType,
		ControllerSeat: 0,
		Timestamp:      0,
		Category:       gameengine.CategoryOther,
		Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
			return true
		},
		ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
			*counter++
			if cancel {
				ev.Cancelled = true
			}
		},
	})
	return counter
}

// -----------------------------------------------------------------------------
// 1. Etrata, the Silencer — exile target + library shuffle
// -----------------------------------------------------------------------------

func TestEtrataSilencer_ExileFiresWouldBeExiledReplacement(t *testing.T) {
	gs := newGame(t, 2)
	etrata := addPerm(gs, 0, "Etrata, the Silencer", "creature", "legendary")
	target := addPerm(gs, 1, "Grizzly Bears", "creature")
	target.Card.BasePower = 2
	target.Card.BaseToughness = 2

	exileWatch := registerReplacementWatcher(gs, "would_be_exiled", false)

	etrataSilencerCombat(gs, etrata, map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Etrata, the Silencer",
		"defender_seat": 1,
	})

	if *exileWatch == 0 {
		t.Error("would_be_exiled replacement was not consulted — handler still bypasses §614")
	}
	if hasEvent(gs, "exile") < 1 {
		t.Errorf("expected canonical 'exile' event from ExilePermanent; got events: %+v",
			eventKinds(gs))
	}
}

func TestEtrataSilencer_AuraDetachesOnExile(t *testing.T) {
	gs := newGame(t, 2)
	etrata := addPerm(gs, 0, "Etrata, the Silencer", "creature", "legendary")
	target := addPerm(gs, 1, "Grizzly Bears", "creature")
	target.Card.BasePower = 2
	target.Card.BaseToughness = 2
	aura := addPerm(gs, 1, "Pacifism", "enchantment", "aura")
	attachAura(aura, target)

	etrataSilencerCombat(gs, etrata, map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Etrata, the Silencer",
		"defender_seat": 1,
	})

	if aura.AttachedTo == target {
		t.Error("aura still attached to exiled creature — DetachAll did not fire")
	}
}

func TestEtrataSilencer_LibraryShuffleEmitsBounce(t *testing.T) {
	gs := newGame(t, 2)
	etrata := addPerm(gs, 0, "Etrata, the Silencer", "creature", "legendary")
	addPerm(gs, 1, "Grizzly Bears", "creature")
	addLibrary(gs, 0, "A", "B", "C") // seed library so the shuffle has work

	etrataSilencerCombat(gs, etrata, map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Etrata, the Silencer",
		"defender_seat": 1,
	})

	if hasEvent(gs, "bounce") < 1 {
		t.Errorf("expected canonical 'bounce' event for Etrata library shuffle; events: %+v",
			eventKinds(gs))
	}
}

// -----------------------------------------------------------------------------
// 2. Bilbo, Birthday Celebrant — exile-self activation cost
// -----------------------------------------------------------------------------

func TestBilbo_ExileSelfFiresWouldBeExiledReplacement(t *testing.T) {
	gs := newGame(t, 2)
	bilbo := addPerm(gs, 0, "Bilbo, Birthday Celebrant", "creature", "legendary")
	gs.Seats[0].Life = 111 // gate
	gs.Seats[0].ManaPool = 5

	exileWatch := registerReplacementWatcher(gs, "would_be_exiled", false)

	bilboBirthdayCelebrantActivate(gs, bilbo, 0, nil)

	if *exileWatch == 0 {
		t.Error("would_be_exiled replacement was not consulted — Bilbo still bypasses §614")
	}
	if hasEvent(gs, "exile") < 1 {
		t.Errorf("expected canonical 'exile' event for Bilbo self-exile; events: %+v",
			eventKinds(gs))
	}
}

func TestBilbo_AuraDetachesOnSelfExile(t *testing.T) {
	gs := newGame(t, 2)
	bilbo := addPerm(gs, 0, "Bilbo, Birthday Celebrant", "creature", "legendary")
	aura := addPerm(gs, 0, "Pacifism", "enchantment", "aura")
	attachAura(aura, bilbo)
	gs.Seats[0].Life = 111
	gs.Seats[0].ManaPool = 5

	bilboBirthdayCelebrantActivate(gs, bilbo, 0, nil)

	if aura.AttachedTo == bilbo {
		t.Error("aura still attached to exiled Bilbo — DetachAll did not fire")
	}
}

// -----------------------------------------------------------------------------
// 3. Thassa, Deep-Dwelling — blink target creature
// -----------------------------------------------------------------------------

func TestThassaDeepDwelling_BlinkFiresWouldBeExiledReplacement(t *testing.T) {
	gs := newGame(t, 2)
	thassa := addPerm(gs, 0, "Thassa, Deep-Dwelling", "creature", "legendary")
	bears := addPerm(gs, 0, "Grizzly Bears", "creature")
	bears.Card.BasePower = 2
	bears.Card.BaseToughness = 2

	exileWatch := registerReplacementWatcher(gs, "would_be_exiled", false)

	thassaDeepDwellingEndStep(gs, thassa, nil)

	if *exileWatch == 0 {
		t.Error("would_be_exiled replacement was not consulted — Thassa blink still bypasses §614")
	}
	if hasEvent(gs, "exile") < 1 {
		t.Errorf("expected canonical 'exile' event for Thassa blink-out; events: %+v",
			eventKinds(gs))
	}
}

func TestThassaDeepDwelling_AuraDetachesOnBlinkExile(t *testing.T) {
	gs := newGame(t, 2)
	thassa := addPerm(gs, 0, "Thassa, Deep-Dwelling", "creature", "legendary")
	bears := addPerm(gs, 0, "Grizzly Bears", "creature")
	bears.Card.BasePower = 2
	bears.Card.BaseToughness = 2
	aura := addPerm(gs, 0, "Rancor", "enchantment", "aura")
	attachAura(aura, bears)

	thassaDeepDwellingEndStep(gs, thassa, nil)

	if aura.AttachedTo == bears {
		t.Error("aura still attached to blinked creature — DetachAll did not fire")
	}
}

// -----------------------------------------------------------------------------
// 4. Zabaz, the Glimmerwasp — destroy target artifact
// -----------------------------------------------------------------------------

func TestZabaz_DestroyFiresWouldDieReplacement(t *testing.T) {
	gs := newGame(t, 2)
	zabaz := addPerm(gs, 0, "Zabaz, the Glimmerwasp", "creature", "legendary", "artifact")
	addPerm(gs, 0, "Sol Ring", "artifact")

	dieWatch := registerReplacementWatcher(gs, "would_die", false)

	zabazDestroyArtifact(gs, zabaz)

	if *dieWatch == 0 {
		t.Error("would_die replacement was not consulted — Zabaz still bypasses §614")
	}
	if hasEvent(gs, "destroy") < 1 {
		t.Errorf("expected canonical 'destroy' event from DestroyPermanent; events: %+v",
			eventKinds(gs))
	}
}

func TestZabaz_AuraDetachesOnDestroy(t *testing.T) {
	gs := newGame(t, 2)
	zabaz := addPerm(gs, 0, "Zabaz, the Glimmerwasp", "creature", "legendary", "artifact")
	sol := addPerm(gs, 0, "Sol Ring", "artifact")
	aura := addPerm(gs, 0, "Whirlermaker", "enchantment", "aura")
	attachAura(aura, sol)

	zabazDestroyArtifact(gs, zabaz)

	if aura.AttachedTo == sol {
		t.Error("aura still attached to destroyed Sol Ring — DetachAll did not fire")
	}
}

func TestZabaz_IndestructibleTargetSurvives(t *testing.T) {
	// Defends the DestroyPermanent indestructible-check path. Pre-fix
	// the raw moveCardBetweenZones forced the artifact into the graveyard
	// regardless of indestructible.
	gs := newGame(t, 2)
	zabaz := addPerm(gs, 0, "Zabaz, the Glimmerwasp", "creature", "legendary", "artifact")
	darksteel := addPerm(gs, 0, "Darksteel Forge", "artifact")
	if darksteel.Flags == nil {
		darksteel.Flags = map[string]int{}
	}
	darksteel.Flags["kw:indestructible"] = 1

	zabazDestroyArtifact(gs, zabaz)

	// Indestructible should have survived — assert via destroy_prevented
	// event (DestroyPermanent logs this when §702.12b kicks in).
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == darksteel {
			found = true
			break
		}
	}
	if !found {
		t.Error("indestructible artifact was destroyed — DestroyPermanent §702.12b check bypassed")
	}
}

// -----------------------------------------------------------------------------
// 5. Zimone and Dina — sacrifice another creature
// -----------------------------------------------------------------------------

func TestZimoneAndDina_SacFiresWouldDieReplacement(t *testing.T) {
	gs := newGame(t, 2)
	zd := addPerm(gs, 0, "Zimone and Dina", "creature", "legendary")
	fodder := addPerm(gs, 0, "Saproling Token", "creature", "token")
	fodder.Card.BasePower = 1
	fodder.Card.BaseToughness = 1
	addLibrary(gs, 0, "A", "B", "C")

	dieWatch := registerReplacementWatcher(gs, "would_die", false)

	zimoneDinaDoOnce(gs, zd, fodder)

	if *dieWatch == 0 {
		t.Error("would_die replacement was not consulted — Zimone/Dina sac still bypasses §614")
	}
	if hasEvent(gs, "sacrifice") < 1 {
		t.Errorf("expected canonical 'sacrifice' event from SacrificePermanent; events: %+v",
			eventKinds(gs))
	}
}

func TestZimoneAndDina_AuraDetachesOnSacrifice(t *testing.T) {
	gs := newGame(t, 2)
	zd := addPerm(gs, 0, "Zimone and Dina", "creature", "legendary")
	fodder := addPerm(gs, 0, "Grizzly Bears", "creature")
	fodder.Card.BasePower = 2
	fodder.Card.BaseToughness = 2
	aura := addPerm(gs, 0, "Pacifism", "enchantment", "aura")
	attachAura(aura, fodder)
	addLibrary(gs, 0, "A", "B")

	zimoneDinaDoOnce(gs, zd, fodder)

	if aura.AttachedTo == fodder {
		t.Error("aura still attached to sacrificed creature — DetachAll did not fire")
	}
}

func TestZimoneAndDina_SacrificeFiresCreatureDiedFlag(t *testing.T) {
	// Defends the descend / creature_died bookkeeping that
	// sacrificePermanentImpl handles automatically — proves the
	// canonical SacrificePermanent path is now in use (the manual
	// moveCardBetweenZones never updated these counters).
	gs := newGame(t, 2)
	zd := addPerm(gs, 0, "Zimone and Dina", "creature", "legendary")
	fodder := addPerm(gs, 0, "Saproling Token", "creature")
	fodder.Card.BasePower = 1
	fodder.Card.BaseToughness = 1
	addLibrary(gs, 0, "A")

	zimoneDinaDoOnce(gs, zd, fodder)

	if gs.Flags["creature_died_this_turn"] < 1 {
		t.Errorf("expected creature_died_this_turn flag to be bumped by SacrificePermanent; flags=%v",
			gs.Flags)
	}
}

// -----------------------------------------------------------------------------
// Test helpers
// -----------------------------------------------------------------------------

func eventKinds(gs *gameengine.GameState) []string {
	out := make([]string, 0, len(gs.EventLog))
	for _, e := range gs.EventLog {
		out = append(out, e.Kind)
	}
	return out
}
