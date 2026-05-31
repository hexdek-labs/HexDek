package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestBanisherPriest_LTBDispatch_ViaDestroyPermanent pins the
// fix for Loki r60 fresh-main ExileLinkageIntegrity cluster (Prison
// Barricade + Leonardo, Sewer Samurai — 68/72 hits).
//
// Pre-fix shape: zone_change.go removes the leaving perm from
// seat.Battlefield BEFORE FireCardTrigger("permanent_ltb"), and
// fireTrigger in registry.go only walked the battlefield. The leaving
// perm's own self-LTB handlers (ltbReturnLinkedExile here) never
// dispatched, leaving LinkedExile / ExiledByMe / ExiledByTimestamp
// orphaned indefinitely — surfaced by the new ExileLinkageIntegrity
// invariant (added in InstanceID v2 Phase 3-4).
//
// Fix: fireTrigger now ALSO dispatches against ctx["perm"] for
// leaving-perm-shape events. This test exercises the FULL canonical
// destroy → zone_change → FireCardTrigger path, NOT the direct
// ReturnLinkedExile primitive call used by older Phase 3 tests
// (instanceid_phase3_test.go) — those bypass fireTrigger entirely
// and didn't catch this gap.
func TestBanisherPriest_LTBDispatch_ViaDestroyPermanent(t *testing.T) {
	gs := newTestGS(2)

	priestCard := &gameengine.Card{Name: "Banisher Priest", Owner: 0, InstanceID: "h0OGVC000001", Types: []string{"creature"}}
	preyCard := &gameengine.Card{Name: "Goblin Guide", Owner: 1, InstanceID: "h1OGVC000002", Types: []string{"creature"}}
	gs.MintedInstanceIDs = map[string]struct{}{
		priestCard.InstanceID: {},
		preyCard.InstanceID:   {},
	}
	gs.MintedInstanceIDNames = map[string]string{
		priestCard.InstanceID: priestCard.Name,
		preyCard.InstanceID:   preyCard.Name,
	}

	priest := &gameengine.Permanent{
		Card: priestCard, Controller: 0, Owner: 0, Timestamp: 10,
		Flags: map[string]int{}, Counters: map[string]int{},
	}
	prey := &gameengine.Permanent{
		Card: preyCard, Controller: 1, Owner: 1, Timestamp: 11,
		Flags: map[string]int{}, Counters: map[string]int{},
	}
	gs.Seats[0].Battlefield = []*gameengine.Permanent{priest}
	gs.Seats[1].Battlefield = []*gameengine.Permanent{prey}

	// ETB: priest exiles prey via the canonical ExileLinked primitive.
	banisherPriestETB(gs, priest)

	if prey.Card.ExiledByTimestamp != priest.Timestamp {
		t.Fatalf("post-ETB: prey.ExiledByTimestamp=%d, want %d", prey.Card.ExiledByTimestamp, priest.Timestamp)
	}
	if len(priest.LinkedExile) != 1 || priest.LinkedExile[0] != preyCard {
		t.Fatalf("post-ETB: priest.LinkedExile=%v, want [Goblin Guide]", priest.LinkedExile)
	}

	// Destroy the priest via the canonical zone-change path, NOT a direct
	// ReturnLinkedExile call. This is what Loki exercises in the wild.
	gameengine.DestroyPermanent(gs, priest, nil)

	// The LTB handler must have fired and routed prey back to its owner's
	// battlefield (or wherever ReturnLinkedExile sends it), clearing the
	// linkage on both sides.
	if prey.Card.ExiledByTimestamp != 0 {
		t.Fatalf("post-LTB: prey.ExiledByTimestamp=%d, want 0 (LTB-return missed)", prey.Card.ExiledByTimestamp)
	}
	if len(priest.LinkedExile) != 0 {
		t.Fatalf("post-LTB: priest.LinkedExile=%v, want empty", priest.LinkedExile)
	}
	if len(priest.ExiledByMe) != 0 {
		t.Fatalf("post-LTB: priest.ExiledByMe=%v, want empty", priest.ExiledByMe)
	}

	// Prey must no longer be in any seat's exile zone.
	for _, s := range gs.Seats {
		for _, c := range s.Exile {
			if c == preyCard {
				t.Fatalf("post-LTB: prey still in seat %d exile — LTB-return missed", s.Idx)
			}
		}
	}

	// ExileLinkageIntegrity must be clean.
	for _, inv := range gameengine.AllInvariants() {
		if inv.Name != "ExileLinkageIntegrity" {
			continue
		}
		if err := inv.Check(gs); err != nil {
			t.Fatalf("ExileLinkageIntegrity: %v", err)
		}
	}
}

// TestBanisherPriest_LTBDispatch_FamilyCoverage extends the canonical
// destroy → LTB-return shape to Oblivion Ring, Detention Sphere, and
// Faceless Butcher — the other three cards registered against
// ltbReturnLinkedExile in banisher_priest_family_r60.go. Pins that the
// fireTrigger ctx["perm"] fallback dispatches for the full family, not
// just whichever single card happened to land in the Loki repro.
func TestBanisherPriest_LTBDispatch_FamilyCoverage(t *testing.T) {
	cases := []struct {
		sourceName string
		preyType   string // for picker satisfaction
	}{
		{"Banisher Priest", "creature"},
		{"Oblivion Ring", "creature"},
		{"Detention Sphere", "creature"},
		{"Faceless Butcher", "creature"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.sourceName, func(t *testing.T) {
			gs := newTestGS(2)
			srcTypes := []string{"creature"}
			if tc.sourceName == "Oblivion Ring" || tc.sourceName == "Detention Sphere" {
				srcTypes = []string{"enchantment"}
			}
			src := &gameengine.Permanent{
				Card:       &gameengine.Card{Name: tc.sourceName, Owner: 0, InstanceID: "hX_" + tc.sourceName, Types: srcTypes},
				Controller: 0, Owner: 0, Timestamp: 20,
				Flags: map[string]int{}, Counters: map[string]int{},
			}
			preyCard := &gameengine.Card{
				Name:       "Grizzly Bears",
				Owner:      1,
				InstanceID: "hY_prey_" + tc.sourceName,
				Types:      []string{tc.preyType},
			}
			prey := &gameengine.Permanent{
				Card: preyCard, Controller: 1, Owner: 1, Timestamp: 21,
				Flags: map[string]int{}, Counters: map[string]int{},
			}
			gs.Seats[0].Battlefield = []*gameengine.Permanent{src}
			gs.Seats[1].Battlefield = []*gameengine.Permanent{prey}
			gs.MintedInstanceIDs = map[string]struct{}{src.Card.InstanceID: {}, preyCard.InstanceID: {}}
			gs.MintedInstanceIDNames = map[string]string{
				src.Card.InstanceID: src.Card.Name,
				preyCard.InstanceID: preyCard.Name,
			}

			// Run the registered ETB for the source.
			r := Global()
			r.mu.RLock()
			etbHandlers := r.etb[normalizeName(tc.sourceName)]
			r.mu.RUnlock()
			if len(etbHandlers) == 0 {
				t.Fatalf("%s: no ETB handler registered", tc.sourceName)
			}
			for _, h := range etbHandlers {
				h(gs, src)
			}

			if prey.Card.ExiledByTimestamp != src.Timestamp {
				t.Fatalf("%s: post-ETB prey.ExiledByTimestamp=%d, want %d",
					tc.sourceName, prey.Card.ExiledByTimestamp, src.Timestamp)
			}

			gameengine.DestroyPermanent(gs, src, nil)

			if prey.Card.ExiledByTimestamp != 0 {
				t.Fatalf("%s: post-LTB prey.ExiledByTimestamp=%d, want 0 — LTB-return missed",
					tc.sourceName, prey.Card.ExiledByTimestamp)
			}
			if len(src.ExiledByMe) != 0 {
				t.Fatalf("%s: post-LTB src.ExiledByMe=%v, want empty", tc.sourceName, src.ExiledByMe)
			}
		})
	}
}

// TestFireTrigger_LeavingPermDedup pins that a perm still on the
// battlefield AND passed via ctx (i.e. some LTB-shape caller fires the
// trigger before detaching) is NOT double-handled. Guards against the
// fireTrigger fallback becoming a regression vector for cards whose
// LTB sequence runs the trigger pre-removal.
//
// Uses a synthetic test-only handler that just counts invocations —
// avoids any LTB chain side effects (Banisher Priest's real handler
// returns the prey, which then triggers SBA chains in turn).
func TestFireTrigger_LeavingPermDedup(t *testing.T) {
	const testCardName = "Dedup Test Sentinel"
	r := Global()
	calls := 0
	r.OnTrigger(testCardName, "permanent_ltb", func(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
		calls++
	})
	// Surgical cleanup — Reset() would nuke every init()-registered
	// handler that lacks an AddResetHook (see CLAUDE.md issue-log
	// 2026-05-17 "test pollution" row), so just delete OUR test entry.
	defer func() {
		r.mu.Lock()
		delete(r.onTrigger, normalizeName(testCardName))
		r.mu.Unlock()
	}()

	gs := newTestGS(2)
	src := &gameengine.Permanent{
		// Use an artifact so SBA §704.5f (creature toughness 0) doesn't
		// destroy the sentinel mid-test and fire a SECOND permanent_ltb.
		Card:       &gameengine.Card{Name: testCardName, Owner: 0, InstanceID: "hZ_dedup_test", Types: []string{"artifact"}},
		Controller: 0, Owner: 0, Timestamp: 30,
		Flags: map[string]int{}, Counters: map[string]int{},
	}
	gs.Seats[0].Battlefield = []*gameengine.Permanent{src}

	// Fire permanent_ltb with perm still on the battlefield AND referenced
	// in ctx — the dedup map in fireTrigger should detect the duplicate.
	fireTrigger(gs, "permanent_ltb", map[string]interface{}{
		"perm":            src,
		"card":            src.Card,
		"controller_seat": src.Controller,
		"to_zone":         "graveyard",
	})
	if calls != 1 {
		var dump []string
		for _, ev := range gs.EventLog {
			dump = append(dump, ev.Kind+":"+ev.Source)
		}
		t.Fatalf("dedup test handler invoked %d times, want exactly 1; events=%v", calls, dump)
	}

	// Cross-check: when perm is NOT on the battlefield (typical LTB
	// shape — already removed by zone_change.go before FireCardTrigger),
	// the ctx fallback must still dispatch.
	gs.Seats[0].Battlefield = nil
	calls = 0
	fireTrigger(gs, "permanent_ltb", map[string]interface{}{
		"perm":            src,
		"card":            src.Card,
		"controller_seat": src.Controller,
		"to_zone":         "graveyard",
	})
	if calls != 1 {
		t.Fatalf("ctx-only dispatch invoked %d times, want exactly 1 (leaving-perm fallback)", calls)
	}
}
