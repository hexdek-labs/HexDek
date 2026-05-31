package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// BoardPresence r60-v2 regressions covering three signals layered on
// the existing dimension (effectiveBoardPower with evasion + token
// discount + tap/sick discount + noncreature density + width bonus):
//
//   - Signal A: survivabilityMultiplier — indestructible / hexproof /
//     ward / protection. Pre-r60 a 12/12 Blightsteel and a 12/12
//     vanilla scored identically because removal cost was invisible.
//   - Signal B: infectToxicMultiplier — alternate poison clock. A
//     2-power infect creature has a faster lethal pace than a 2-power
//     vanilla against a fresh-life opp.
//   - Signal C: commander-centric on-field bonus. When Freya flags
//     the deck IsCommanderCentric AND the commander is on board, the
//     commander's strategic value exceeds raw power.

func bpAddCreatureWithKeyword(seat *gameengine.Seat, name string, power, toughness int, keywords ...string) *gameengine.Permanent {
	c := newTestCardMinimal(name, []string{"creature"}, 3, nil)
	p := newTestPermanent(seat, c, power, toughness)
	for _, kw := range keywords {
		addKeyword(p, kw)
	}
	return p
}

// ---------------------------------------------------------------------
// Signal A — survivability multiplier
// ---------------------------------------------------------------------

func TestSurvivabilityMultiplier_Tiers(t *testing.T) {
	cases := []struct {
		name     string
		keywords []string
		oracle   string
		want     float64
	}{
		{"vanilla", nil, "", 1.00},
		{"indestructible alone", []string{"indestructible"}, "", 1.20},
		{"hexproof alone", []string{"hexproof"}, "", 1.10},
		{"shroud alone", []string{"shroud"}, "", 1.10},
		{"ward alone", []string{"ward"}, "", 1.05},
		{"protection only via oracle", nil, "protection from blue", 1.15},
		{"indestructible + hexproof", []string{"indestructible", "hexproof"}, "", 1.30},
		{"blightsteel-like (indestructible + protection)", []string{"indestructible"}, "protection from artifacts", 1.35},
		{"cap saturation (all four)", []string{"indestructible", "hexproof", "ward"}, "protection from creatures", 1.40},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seat := &gameengine.Seat{Idx: 0}
			c := gyCardWithOracle(tc.name, []string{"creature"}, 3, tc.oracle)
			p := newTestPermanent(seat, c, 3, 3)
			for _, kw := range tc.keywords {
				addKeyword(p, kw)
			}
			got := survivabilityMultiplier(p)
			if got != tc.want {
				t.Errorf("%s: survivabilityMultiplier = %.2f want %.2f", tc.name, got, tc.want)
			}
		})
	}
}

func TestScoreBoardR60v2_BlightsteelBeatsVanilla(t *testing.T) {
	mk := func(makeBlightsteel bool) *gameengine.GameState {
		gs := newTestGame(t, 2)
		if makeBlightsteel {
			c := gyCardWithOracle("Blightsteel Colossus", []string{"creature"}, 12, "protection from artifacts")
			p := newTestPermanent(gs.Seats[0], c, 11, 11)
			addKeyword(p, "indestructible")
			addKeyword(p, "infect")
			addKeyword(p, "trample")
		} else {
			c := gyCardWithOracle("Vanilla 11/11", []string{"creature"}, 12, "")
			_ = newTestPermanent(gs.Seats[0], c, 11, 11)
		}
		_ = newTestPermanent(gs.Seats[1],
			newTestCardMinimal("Opp Bear", []string{"creature"}, 2, nil), 2, 2)
		return gs
	}

	ev := NewEvaluator(nil)
	withBlight := ev.scoreBoard(mk(true), 0)
	withVanilla := ev.scoreBoard(mk(false), 0)

	if withBlight <= withVanilla {
		t.Errorf("Blightsteel should out-score same-power vanilla; blight=%.4f vanilla=%.4f",
			withBlight, withVanilla)
	}
	// Blightsteel: 11 raw * 1.20 (trample, pw>=4) * 1.40 (indestructible
	// + protection cap-leaning) * 1.40 (infect) ≈ 25.8 effective.
	// Vanilla: 11. Effective delta ~14, /10 → +1.4 dimension.
	delta := withBlight - withVanilla
	if delta < 0.8 {
		t.Errorf("Blightsteel vs vanilla delta too small: %.4f (want ≥0.8)", delta)
	}
}

// ---------------------------------------------------------------------
// Signal B — infect/toxic multiplier
// ---------------------------------------------------------------------

func TestInfectToxicMultiplier_Tiers(t *testing.T) {
	cases := []struct {
		name     string
		keywords []string
		want     float64
	}{
		{"vanilla", nil, 1.00},
		{"infect", []string{"infect"}, 1.40},
		{"toxic", []string{"toxic"}, 1.15},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seat := &gameengine.Seat{Idx: 0}
			p := newTestPermanent(seat,
				newTestCardMinimal(tc.name, []string{"creature"}, 3, nil), 2, 2)
			for _, kw := range tc.keywords {
				addKeyword(p, kw)
			}
			got := infectToxicMultiplier(p)
			if got != tc.want {
				t.Errorf("%s: infectToxicMultiplier = %.2f want %.2f", tc.name, got, tc.want)
			}
		})
	}
}

func TestScoreBoardR60v2_InfectCreatureScoresAboveVanilla(t *testing.T) {
	mk := func(infect bool) *gameengine.GameState {
		gs := newTestGame(t, 2)
		p := newTestPermanent(gs.Seats[0],
			newTestCardMinimal("Infectite", []string{"creature"}, 3, nil), 3, 3)
		if infect {
			addKeyword(p, "infect")
		}
		_ = newTestPermanent(gs.Seats[1],
			newTestCardMinimal("Opp Bear", []string{"creature"}, 2, nil), 2, 2)
		return gs
	}

	ev := NewEvaluator(nil)
	withInfect := ev.scoreBoard(mk(true), 0)
	withoutInfect := ev.scoreBoard(mk(false), 0)

	if withInfect <= withoutInfect {
		t.Errorf("infect creature should out-score vanilla; infect=%.4f vanilla=%.4f",
			withInfect, withoutInfect)
	}
}

// ---------------------------------------------------------------------
// Signal C — commander-centric on-field bonus
// ---------------------------------------------------------------------

func TestScoreBoardR60v2_CommanderCentricBonus(t *testing.T) {
	build := func(strategy *StrategyProfile) *gameengine.GameState {
		gs := newTestGame(t, 2)
		gs.CommanderFormat = true
		gs.Seats[0].CommanderNames = []string{"Sram, Senior Edificer"}
		// Sram on the battlefield.
		cmdr := newTestCardMinimal("Sram, Senior Edificer", []string{"creature", "legendary"}, 1, nil)
		newTestPermanent(gs.Seats[0], cmdr, 1, 2)
		_ = newTestPermanent(gs.Seats[1],
			newTestCardMinimal("Opp Bear", []string{"creature"}, 2, nil), 2, 2)
		_ = strategy
		return gs
	}

	ev := NewEvaluator(&StrategyProfile{IsCommanderCentric: true})
	withCentric := ev.scoreBoard(build(&StrategyProfile{IsCommanderCentric: true}), 0)

	evDefault := NewEvaluator(nil)
	noStrategy := evDefault.scoreBoard(build(nil), 0)

	if withCentric <= noStrategy {
		t.Errorf("IsCommanderCentric should add bonus when cmdr is on field; centric=%.4f none=%.4f",
			withCentric, noStrategy)
	}
	// Bonus: +0.30 for one commander on field.
	delta := withCentric - noStrategy
	if delta < 0.25 || delta > 0.35 {
		t.Errorf("commander-centric bonus off: delta=%.4f (want ~0.30)", delta)
	}
}

func TestScoreBoardR60v2_CommanderCentric_InertWhenCmdrNotOnField(t *testing.T) {
	build := func() *gameengine.GameState {
		gs := newTestGame(t, 2)
		gs.CommanderFormat = true
		gs.Seats[0].CommanderNames = []string{"Sram, Senior Edificer"}
		// Sram is in command zone (not on battlefield).
		cmdr := newTestCardMinimal("Sram, Senior Edificer", []string{"creature", "legendary"}, 1, nil)
		gs.Seats[0].CommandZone = []*gameengine.Card{cmdr}
		_ = newTestPermanent(gs.Seats[1],
			newTestCardMinimal("Opp Bear", []string{"creature"}, 2, nil), 2, 2)
		return gs
	}

	ev := NewEvaluator(&StrategyProfile{IsCommanderCentric: true})
	got := ev.scoreBoard(build(), 0)

	evDefault := NewEvaluator(nil)
	def := evDefault.scoreBoard(build(), 0)

	// No commander on field → no bonus, so centric and default match.
	if got != def {
		t.Errorf("no on-field commander should mean no bonus; centric=%.4f none=%.4f", got, def)
	}
}

func TestScoreBoardR60v2_CommanderCentric_OnlyInCommanderFormat(t *testing.T) {
	build := func() *gameengine.GameState {
		gs := newTestGame(t, 2)
		gs.CommanderFormat = false // explicitly off
		gs.Seats[0].CommanderNames = []string{"Sram, Senior Edificer"}
		cmdr := newTestCardMinimal("Sram, Senior Edificer", []string{"creature", "legendary"}, 1, nil)
		newTestPermanent(gs.Seats[0], cmdr, 1, 2)
		_ = newTestPermanent(gs.Seats[1],
			newTestCardMinimal("Opp Bear", []string{"creature"}, 2, nil), 2, 2)
		return gs
	}

	ev := NewEvaluator(&StrategyProfile{IsCommanderCentric: true})
	got := ev.scoreBoard(build(), 0)

	evDefault := NewEvaluator(nil)
	def := evDefault.scoreBoard(build(), 0)

	if got != def {
		t.Errorf("non-EDH should not apply centric bonus; centric=%.4f none=%.4f", got, def)
	}
}

// ---------------------------------------------------------------------
// Cross-signal: token-swarm vs single-quality-threat correctly
// differentiates with the new multipliers stacked.
// ---------------------------------------------------------------------

func TestScoreBoardR60v2_OneBlightsteelBeatsFourGoblins(t *testing.T) {
	// 1 Blightsteel (11 raw, indestructible + infect + trample + protection)
	// vs 4 vanilla 1/1 Goblin tokens (4 raw).
	gsBlight := newTestGame(t, 2)
	c := gyCardWithOracle("Blightsteel Colossus", []string{"creature"}, 12, "protection from artifacts")
	p := newTestPermanent(gsBlight.Seats[0], c, 11, 11)
	addKeyword(p, "indestructible")
	addKeyword(p, "infect")
	addKeyword(p, "trample")
	_ = newTestPermanent(gsBlight.Seats[1],
		newTestCardMinimal("Opp", []string{"creature"}, 2, nil), 2, 2)

	gsGoblins := newTestGame(t, 2)
	for i := 0; i < 4; i++ {
		tok := newTestCardMinimal("Goblin Token", []string{"creature", "token"}, 0, nil)
		_ = newTestPermanent(gsGoblins.Seats[0], tok, 1, 1)
	}
	_ = newTestPermanent(gsGoblins.Seats[1],
		newTestCardMinimal("Opp", []string{"creature"}, 2, nil), 2, 2)

	ev := NewEvaluator(nil)
	blightScore := ev.scoreBoard(gsBlight, 0)
	goblinScore := ev.scoreBoard(gsGoblins, 0)

	if blightScore <= goblinScore {
		t.Errorf("1 Blightsteel should out-score 4 goblin tokens (threat quality, not just power); blight=%.4f goblins=%.4f",
			blightScore, goblinScore)
	}
}
