package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// backup_counter_doubler_r63_test.go — mechanic-probe regression for BACKUP
// (CR §702.167). Backup's ETB +1/+1 counter is a §122/§616 counter placement,
// so it must route through the doubler chain (Doubling Season / Primal Vigor /
// Hardened Scales) and fire counter_placed — exactly like the r63
// counter-pipeline sweep. Archpriest of Shadows (the only backup card) placed
// it with raw AddCounter, bypassing both. These pin the doubler-aware path on
// the two target branches.

// (a) Backup onto a DIFFERENT creature: the +1/+1 counter is doubled by a
// +1/+1 doubler (Primal Vigor → 2 counters).
func TestArchpriestBackup_CounterRespectsDoublers_OtherTarget(t *testing.T) {
	gs := newGame(t, 2)
	archpriest := addPerm(gs, 0, "Archpriest of Shadows", "creature", "human", "warlock")
	archpriest.Card.BasePower, archpriest.Card.BaseToughness = 2, 2
	// A higher-power creature → chosen as the backup target.
	bear := addPerm(gs, 0, "Grizzly Bears", "creature", "bear")
	bear.Card.BasePower, bear.Card.BaseToughness = 3, 3
	// A +1/+1 doubler on the battlefield.
	pv := addPerm(gs, 0, "Primal Vigor", "enchantment")
	gameengine.RegisterPrimalVigor(gs, pv)

	archpriestOfShadowsETB(gs, archpriest)

	if got := bear.Counters["+1/+1"]; got != 2 {
		t.Errorf("backup +1/+1 counter must respect doublers (Primal Vigor → 2); got %d", got)
	}
	if got := archpriest.Counters["+1/+1"]; got != 0 {
		t.Errorf("backup counter goes on the chosen target, not the backup creature; archpriest got %d", got)
	}
}

// (c) Backup with no other creature targets ITSELF — still a real counter
// placement, so the doubler applies; no ability grant happens.
func TestArchpriestBackup_CounterRespectsDoublers_SelfTarget(t *testing.T) {
	gs := newGame(t, 2)
	archpriest := addPerm(gs, 0, "Archpriest of Shadows", "creature", "human", "warlock")
	archpriest.Card.BasePower, archpriest.Card.BaseToughness = 2, 2
	pv := addPerm(gs, 0, "Primal Vigor", "enchantment")
	gameengine.RegisterPrimalVigor(gs, pv)

	archpriestOfShadowsETB(gs, archpriest)

	if got := archpriest.Counters["+1/+1"]; got != 2 {
		t.Errorf("self-target backup +1/+1 counter must respect doublers (Primal Vigor → 2); got %d", got)
	}
	// Self-target grants nothing (it already has its own abilities).
	if archpriest.Flags["kw:deathtouch"] != 0 {
		t.Error("self-target backup must not grant deathtouch to itself via the backup grant")
	}
}
