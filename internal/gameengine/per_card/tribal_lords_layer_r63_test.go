package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// tribal_lords_layer_r63_test.go — behavioral audit of the Varina zombie-tribal
// lords (Death Baron, Lord of the Accursed, Diregraf Captain, Undead Warchief)
// against the CR §613 layer-system properties. Pre-fix these lords applied their
// buff as a one-shot Modifications snapshot at ETB (applyTribalBuff), which:
//   - never granted Death Baron's deathtouch (property c),
//   - never tracked creatures entering after the lord / surviving the lord (d),
//   - leaked forever after the lord left (no "while_source_on_battlefield"
//     expiry exists), and
//   - excluded Undead Warchief from its own ("no other") buff (e).
//
// The fix registers proper layer-7c P/T + layer-6 keyword continuous effects.

// lordZombie adds a vanilla 2/2 Zombie creature for `seat`.
func lordZombie(gs *gameengine.GameState, seat int, name string) *gameengine.Permanent {
	return addCreature(gs, seat, name, 2, 2, "zombie")
}

func ptOf(gs *gameengine.GameState, p *gameengine.Permanent) (int, int) {
	return gs.PowerOf(p), gs.ToughnessOf(p)
}

// (a)+(c)+(f): Death Baron gives other zombies +1/+1 AND deathtouch via layers,
// not via a +1/+1 counter.
func TestLord_DeathBaron_BuffsAndDeathtouch_ViaLayers(t *testing.T) {
	gs := newGame(t, 2)
	z := lordZombie(gs, 0, "Walking Corpse")
	baron := addCreature(gs, 0, "Death Baron", 2, 2, "zombie")
	gameengine.InvokeETBHook(gs, baron)

	if p, tg := ptOf(gs, z); p != 3 || tg != 3 {
		t.Errorf("Death Baron: zombie should be 3/3, got %d/%d", p, tg)
	}
	if !gs.HasKeywordOf(z, "deathtouch") {
		t.Error("Death Baron: zombie should have deathtouch (property c) — missing")
	}
	// (f) NOT a +1/+1 counter.
	if z.Counters["+1/+1"] != 0 {
		t.Errorf("Death Baron buff must not be a +1/+1 counter, got %d", z.Counters["+1/+1"])
	}
	// Death Baron is a Zombie but "other Zombies" excludes itself.
	if p, _ := ptOf(gs, baron); p != 2 {
		t.Errorf("Death Baron should not buff itself (other), got power %d", p)
	}
	// Also buffs Skeletons you control.
	sk := addCreature(gs, 0, "Bone Picker", 1, 1, "skeleton")
	gs.InvalidateCharacteristicsCache()
	if p, tg := ptOf(gs, sk); p != 2 || tg != 2 {
		t.Errorf("Death Baron should buff Skeletons too, got %d/%d", p, tg)
	}
	if !gs.HasKeywordOf(sk, "deathtouch") {
		t.Error("Death Baron: skeleton should have deathtouch")
	}
}

// (b): multiple anthems stack additively.
func TestLord_AnthemsStackAdditively(t *testing.T) {
	gs := newGame(t, 2)
	z := lordZombie(gs, 0, "Walking Corpse")
	lota := addCreature(gs, 0, "Lord of the Accursed", 2, 2, "zombie")
	cap := addCreature(gs, 0, "Diregraf Captain", 2, 2, "zombie")
	gameengine.InvokeETBHook(gs, lota)
	gameengine.InvokeETBHook(gs, cap)
	if p, tg := ptOf(gs, z); p != 4 || tg != 4 {
		t.Errorf("two +1/+1 lords should stack to 4/4, got %d/%d", p, tg)
	}
}

// (d): dynamic membership — a zombie entering AFTER the lord still gets the
// buff, and the lord leaving removes it from everything.
func TestLord_DynamicMembershipAndLordLeaves(t *testing.T) {
	gs := newGame(t, 2)
	lota := addCreature(gs, 0, "Lord of the Accursed", 2, 2, "zombie")
	gameengine.InvokeETBHook(gs, lota)

	// Zombie enters after the lord.
	late := lordZombie(gs, 0, "Latecomer")
	gs.InvalidateCharacteristicsCache()
	if p, tg := ptOf(gs, late); p != 3 || tg != 3 {
		t.Errorf("(d) zombie entering after the lord should be 3/3, got %d/%d", p, tg)
	}

	// Lord leaves — its continuous effect must tear down.
	gs.UnregisterContinuousEffectsForPermanent(lota)
	gs.InvalidateCharacteristicsCache()
	if p, tg := ptOf(gs, late); p != 2 || tg != 2 {
		t.Errorf("(d) after the lord leaves the buff must vanish, got %d/%d", p, tg)
	}
}

// (e): self-reference — Undead Warchief's buff has NO "other", so it includes
// itself; Lord of the Accursed's "Other" excludes itself.
func TestLord_SelfReference_WarchiefIncludesItself(t *testing.T) {
	gs := newGame(t, 2)
	warchief := addCreature(gs, 0, "Undead Warchief", 3, 3, "zombie")
	gameengine.InvokeETBHook(gs, warchief)
	// "Zombie creatures you control get +2/+1" — includes Undead Warchief.
	if p, tg := ptOf(gs, warchief); p != 5 || tg != 4 {
		t.Errorf("(e) Undead Warchief should buff itself to 5/4, got %d/%d", p, tg)
	}

	gs2 := newGame(t, 2)
	lota := addCreature(gs2, 0, "Lord of the Accursed", 2, 2, "zombie")
	gameengine.InvokeETBHook(gs2, lota)
	if p, _ := ptOf(gs2, lota); p != 2 {
		t.Errorf("(e) Lord of the Accursed (Other) must not buff itself, got power %d", p)
	}
}

// Chief of the Foundry — "Other artifact creatures you control get +1/+1",
// dynamically + auto-cleanup (was a leaking ETB snapshot).
func TestLord_ChiefOfTheFoundry_ArtifactAnthem_Dynamic(t *testing.T) {
	gs := newGame(t, 2)
	chief := addCreature(gs, 0, "Chief of the Foundry", 2, 2, "artifact")
	gameengine.InvokeETBHook(gs, chief)

	bot := addCreature(gs, 0, "Ornithopter", 0, 2, "artifact") // enters AFTER
	human := addCreature(gs, 0, "Soldier", 2, 2, "human")
	gs.InvalidateCharacteristicsCache()

	if p, tg := ptOf(gs, bot); p != 1 || tg != 3 {
		t.Errorf("artifact creature entering after Chief should be 1/3, got %d/%d", p, tg)
	}
	if p, _ := ptOf(gs, human); p != 2 {
		t.Errorf("non-artifact must not be buffed, got power %d", p)
	}
	gs.UnregisterContinuousEffectsForPermanent(chief)
	gs.InvalidateCharacteristicsCache()
	if p, tg := ptOf(gs, bot); p != 0 || tg != 2 {
		t.Errorf("after Chief leaves buff must vanish, got %d/%d", p, tg)
	}
}

// Morophon — "Other creatures you control of the chosen type get +1/+1" as a
// dynamic layer effect (was a flag-guarded snapshot).
func TestLord_Morophon_ChosenTypeAnthem_Dynamic(t *testing.T) {
	gs := newGame(t, 2)
	// Seed two zombies so the auto-chooser picks "zombie".
	z1 := lordZombie(gs, 0, "Ghoul A")
	moro := addCreature(gs, 0, "Morophon, the Boundless", 0, 0, "shapeshifter")
	gameengine.InvokeETBHook(gs, moro)

	if p, tg := ptOf(gs, z1); p != 3 || tg != 3 {
		t.Fatalf("Morophon should buff chosen-type (zombie) to 3/3, got %d/%d "+
			"(auto-chooser may not have picked zombie)", p, tg)
	}
	late := lordZombie(gs, 0, "Ghoul B") // enters after Morophon
	gs.InvalidateCharacteristicsCache()
	if p, tg := ptOf(gs, late); p != 3 || tg != 3 {
		t.Errorf("zombie entering after Morophon should be 3/3, got %d/%d", p, tg)
	}
	gs.UnregisterContinuousEffectsForPermanent(moro)
	gs.InvalidateCharacteristicsCache()
	if p, _ := ptOf(gs, z1); p != 2 {
		t.Errorf("after Morophon leaves the buff must vanish, got power %d", p)
	}
}

// Samut — "Other creatures you control get +X/+0, where X is your speed":
// dynamic-VALUE layer effect tracking the live speed counter.
func TestLord_Samut_SpeedScaledAnthem_Dynamic(t *testing.T) {
	gs := newGame(t, 2)
	other := addCreature(gs, 0, "Bear", 2, 2)
	samut := addCreature(gs, 0, "Samut, the Driving Force", 4, 4)
	gameengine.InvokeETBHook(gs, samut) // speed starts at 1

	if p, tg := ptOf(gs, other); p != 3 || tg != 2 {
		t.Errorf("at speed 1 the +X/+0 anthem should make Bear 3/2, got %d/%d", p, tg)
	}
	// Speed climbs — the SAME effect must now read the new value (no re-stamp).
	gs.Seats[0].Flags["speed"] = 3
	gs.InvalidateCharacteristicsCache()
	if p, tg := ptOf(gs, other); p != 5 || tg != 2 {
		t.Errorf("at speed 3 Bear should be 5/2 (power-only), got %d/%d", p, tg)
	}
	// Samut itself is excluded ("Other creatures").
	if p := gs.PowerOf(samut); p != 4 {
		t.Errorf("Samut must not buff itself, got power %d", p)
	}
	gs.UnregisterContinuousEffectsForPermanent(samut)
	gs.InvalidateCharacteristicsCache()
	if p := gs.PowerOf(other); p != 2 {
		t.Errorf("after Samut leaves the anthem must vanish, got power %d", p)
	}
}

// scope: only the controller's creatures and only the named type are affected.
func TestLord_ScopeControllerAndType(t *testing.T) {
	gs := newGame(t, 2)
	lord := addCreature(gs, 0, "Lord of the Accursed", 2, 2, "zombie")
	gameengine.InvokeETBHook(gs, lord)

	oppZombie := lordZombie(gs, 1, "Enemy Ghoul")
	myHuman := addCreature(gs, 0, "Soldier", 2, 2, "human")
	gs.InvalidateCharacteristicsCache()

	if p, _ := ptOf(gs, oppZombie); p != 2 {
		t.Errorf("opponent's zombie must not be buffed, got power %d", p)
	}
	if p, _ := ptOf(gs, myHuman); p != 2 {
		t.Errorf("your non-zombie must not be buffed, got power %d", p)
	}
}
