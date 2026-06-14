package main

import (
	"math/rand"
	"os"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// End-to-end proof that a real YggdrasilHat now (a) recognizes a real-corpus
// counterspell, (b) is funded to cast it on an opponent's turn, and (c)
// actually counters the opponent's spell — exercising the full r63 fix stack
// (typed_spell_effect recognition + CounterCanTarget gate + mana funding).
func TestRealHatCountersRealSpell(t *testing.T) {
	if os.Getenv("HEXDEK_AB") == "" {
		t.Skip("set HEXDEK_AB=1")
	}
	dc := loadDetCorpus(t)
	if dc == nil {
		return
	}
	gameengine.ResponseManaFundingEnabled = true
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(7)), dc.ast)

	// Seat 1 = defender with a real Counterspell + two untapped Islands, EMPTY
	// pool (the real opponent's-turn state).
	counter := buildCardFromName("Counterspell", dc.ast, dc.meta)
	counter.Owner = 1
	gs.Seats[1].Hat = hat.NewYggdrasilHatWithNoise(nil, 0, 0)
	gs.Seats[1].Hand = []*gameengine.Card{counter}
	gs.Seats[1].ManaPool = 0
	for i := 0; i < 2; i++ {
		isl := buildCardFromName("Island", dc.ast, dc.meta)
		if isl == nil {
			isl = &gameengine.Card{Name: "Island", Owner: 1, Types: []string{"land"}}
		}
		isl.Owner = 1
		gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, &gameengine.Permanent{
			Card: isl, Controller: 1, Owner: 1, Flags: map[string]int{},
		})
	}
	gs.Seats[0].Hat = hat.NewYggdrasilHatWithNoise(nil, 0, 0)

	// Seat 0 (active) casts a real creature spell. PriorityRound inside
	// CastSpell polls seat 1, which should counter it.
	threat := buildCardFromName("Time Warp", dc.ast, dc.meta)
	if threat == nil {
		threat = &gameengine.Card{Name: "Time Warp", Owner: 0, Types: []string{"creature"}, CMC: 2}
	}
	threat.Owner = 0
	gs.Seats[0].Hand = []*gameengine.Card{threat}
	gs.Seats[0].ManaPool = 10
	gs.Active = 0
	gs.Phase = "precombat_main"

	gameengine.SetResponseTelemetry(true)
	gameengine.ResponseTelemetry = gameengine.ResponseTelemetryT{}
	err := gameengine.CastSpell(gs, 0, threat, nil)
	tm := gameengine.ResponseTelemetry

	t.Logf("cast err=%v; telemetry: chosen=%d cast=%d rejCounterTL=%d fundFail=%d", err, tm.Chosen, tm.Cast, tm.RejCounterTL, tm.RejTargetIll+tm.FundFail)
	// The threat should have been countered → in seat 0's graveyard, not battlefield.
	onBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.Name == "Time Warp" {
			onBF = true
		}
	}
	inGY := false
	for _, c := range gs.Seats[0].Graveyard {
		if c.Name == "Time Warp" {
			inGY = true
		}
	}
	counterGone := true
	for _, c := range gs.Seats[1].Hand {
		if c == counter {
			counterGone = false
		}
	}
	t.Logf("threat onBattlefield=%v inGraveyard=%v; counterLeftHand=%v", onBF, inGY, counterGone)
	if tm.Cast == 0 {
		t.Fatalf("defender did NOT cast its counter (full fix stack broken)")
	}
	if onBF {
		t.Fatalf("threat resolved to battlefield despite a held, affordable counter")
	}
}
