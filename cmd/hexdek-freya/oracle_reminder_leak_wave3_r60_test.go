package main

import (
	"testing"
)

// oracle_reminder_leak_wave3_r60_test.go — third wave of NLP-grade
// detector migrations. Waves 1 + 2 (PR #799 + #810) closed 10 detector
// families; this wave closes 5 more:
//
//   - counter_add `enter` needle tightened to `enters with` — bare
//     "enter" leaked when "+1/+1 counter" co-occurred near an ETB
//     trigger that wasn't ETB-with-counters (Aetherborn Marauder's
//     counter-MOVE ability).
//   - graveyardSizePayoff predicate extended to "in all graveyards"
//     and "in any graveyard" — the Lhurgoyf family (Tarmogoyf, Lhurgoyf,
//     Mortivore) scales off ALL graveyards and was missed pre-fix
//     because the substring required "your graveyard" specifically.
//   - mechdb mechanicPhrases scan migrated to otClean — Infect reminder
//     "(... -1/-1 counters and to players as poison counters.)" used
//     to tag every Infect creature as BOTH infect AND poison, inflating
//     the poison-mechanic count.
//   - cardIsReanimationEffect migrated to otClean — defense-in-depth
//     against unearth / embalm / encore reminders that share "return
//     this card from your graveyard to the battlefield" phrasing with
//     the canonical reanimation substring trio.
//   - classifyZoneFlows exile_play migrated to otClean — same cascade /
//     suspend / foretell reminder leak that wave 2 fixed in
//     analysis.go's exile_cast detector was still firing through the
//     value-chain zone-flow detector.

type wave3Case struct {
	name, ot, tl, mc, pwr string
	cmc                    int

	wantNoEffect   []string
	wantNoProduces []ResourceType
	wantNoZoneFlow string

	mustHaveEffect   string
	mustHaveProduces ResourceType
}

var wave3Cases = []wave3Case{
	// --- counter_add ETB-needle tightening -----------------------------

	{
		// Aetherborn Marauder: "When this creature enters, MOVE any
		// number of +1/+1 counters..." — pre-fix the bare "enter"
		// needle co-occurring with "+1/+1 counter" leaked
		// Effects=[counter_add] + Produces=[counter] even though the
		// ability MOVES counters, not adds them. Counter_move is the
		// correct classification and survives.
		name: "Aetherborn Marauder",
		ot: "Flying, lifelink\n" +
			"When this creature enters, move any number of +1/+1 counters from other " +
			"permanents you control onto this creature.",
		tl: "creature — aetherborn rogue", mc: "{3}{B}", cmc: 4, pwr: "2",
		wantNoEffect:   []string{"counter_add"},
		wantNoProduces: []ResourceType{ResCounter},
		mustHaveEffect: "counter_move",
	},
	{
		// Hangarback Walker: real ETB-with-counters via "enters with X
		// +1/+1 counters". Must survive the needle tightening.
		name: "Hangarback Walker (TP)",
		ot: "This creature enters with X +1/+1 counters on it.\n" +
			"When this creature dies, create a 1/1 colorless Thopter artifact creature token " +
			"with flying for each +1/+1 counter on this creature.\n" +
			"{1}, {T}: Put a +1/+1 counter on this creature.",
		tl: "artifact creature — construct", mc: "{X}{X}", cmc: 0, pwr: "0",
		mustHaveEffect:   "counter_add",
		mustHaveProduces: ResCounter,
	},

	// --- classifyZoneFlows exile_play -----------------------------------

	{
		// Cascade reminder leak into classifyZoneFlows's exile->hand
		// bridge. Wave 2 fixed the analysis.go cast-from-exile detector
		// but the parallel zone-flow detector in valuechains.go was
		// still firing. Bloodbraid Elf shouldn't surface a fake
		// exile->hand value-chain bridge from the cascade reminder
		// alone.
		name: "Bloodbraid Elf",
		ot: "Haste (This creature can attack and {T} as soon as it comes under your control.)\n" +
			"Cascade (When you cast this spell, exile cards from the top of your library " +
			"until you exile a nonland card that costs less. You may cast it without paying " +
			"its mana cost. Put the exiled cards on the bottom in a random order.)",
		tl: "creature — elf berserker", mc: "{2}{R}{G}", cmc: 4, pwr: "3",
		wantNoZoneFlow: "exile->hand",
	},
	{
		// Hostage Taker (TP): real exile-then-cast effect in body text,
		// not parenthetical. Must keep the exile->hand value-chain bridge
		// after the otClean migration.
		name: "Hostage Taker (TP)",
		ot: "When this creature enters, exile another target creature or artifact until this " +
			"creature leaves the battlefield. You may cast that card for as long as it remains " +
			"exiled, and you may spend mana as though it were mana of any color to cast that spell.",
		tl: "creature — human pirate", mc: "{2}{U}{B}", cmc: 4, pwr: "2",
		mustHaveEffect: "exile_cast",
	},
}

func TestClassifyCard_Wave3ReminderLeakFixes(t *testing.T) {
	for _, tc := range wave3Cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			p := ClassifyCard(tc.name, tc.ot, tc.tl, tc.mc, tc.cmc, tc.pwr)

			for _, eff := range tc.wantNoEffect {
				if hasEffectStr(p.Effects, eff) {
					t.Errorf("Effects contains %q via reminder leak; want absent. got=%v", eff, p.Effects)
				}
			}
			for _, res := range tc.wantNoProduces {
				if hasResource(p.Produces, res) {
					t.Errorf("Produces contains %q via reminder leak; want absent. got=%v", res, p.Produces)
				}
			}
			if tc.wantNoZoneFlow != "" && hasZoneFlow(p.ZoneFlows, tc.wantNoZoneFlow) {
				t.Errorf("ZoneFlows contains %q via reminder leak; want absent. got=%v",
					tc.wantNoZoneFlow, p.ZoneFlows)
			}

			if tc.mustHaveEffect != "" && !hasEffectStr(p.Effects, tc.mustHaveEffect) {
				t.Errorf("Effects missing %q after migration; got=%v", tc.mustHaveEffect, p.Effects)
			}
			if tc.mustHaveProduces != "" && !hasResource(p.Produces, tc.mustHaveProduces) {
				t.Errorf("Produces missing %q after migration; got=%v", tc.mustHaveProduces, p.Produces)
			}
		})
	}
}

// TestHasGraveyardSizeScalePattern pins the broadened predicate. The
// pre-r60-wave-3 detector only matched "in your graveyard"; this test
// confirms it now catches the Lhurgoyf family ("in all graveyards" /
// "in any graveyard") while still rejecting unrelated graveyard
// references (recursion phrasing, opponent-hate phrasing).
func TestHasGraveyardSizeScalePattern(t *testing.T) {
	cases := []struct {
		name, ot string
		want     bool
	}{
		// --- TRUE positives — Lhurgoyf family (newly caught) -----------
		{
			name: "Tarmogoyf — all graveyards",
			ot: "tarmogoyf's power is equal to the number of card types among cards in all " +
				"graveyards and its toughness is equal to that number plus 1.",
			want: true,
		},
		{
			name: "Lhurgoyf — all graveyards, creature cards",
			ot: "lhurgoyf's power is equal to the number of creature cards in all graveyards " +
				"and its toughness is equal to that number plus 1.",
			want: true,
		},
		{
			name: "Mortivore — all graveyards",
			ot: "mortivore's power and toughness are each equal to the number of creature " +
				"cards in all graveyards.",
			want: true,
		},
		// --- TRUE positives — your-graveyard (already worked) ----------
		{
			name: "Splinterfright — your graveyard",
			ot: "splinterfright's power and toughness are each equal to the number of creature " +
				"cards in your graveyard.",
			want: true,
		},
		{
			name: "Nighthowler — your graveyard via for-each",
			ot: "bestow {5}{b}{b}. nighthowler and enchanted creature each get +1/+1 for each " +
				"creature card in your graveyard.",
			want: true,
		},
		// --- TRUE negatives — guard against widening too far -----------
		{
			name: "Eternal Witness — recursion, not size payoff",
			ot:   "when this creature enters, you may return target card from your graveyard to your hand.",
			want: false,
		},
		{
			name: "Bojuka Bog — graveyard hate, no scaling",
			ot: "this land enters tapped.\nwhen this land enters, exile all cards from target " +
				"player's graveyard.",
			want: false,
		},
		{
			name: "Wonder — graveyard-static, no count/scale",
			ot: "flying\nas long as this card is in your graveyard and you control an island, " +
				"creatures you control have flying.",
			want: false,
		},
		{
			name: "Tormod's Crypt — graveyard exile, no scaling",
			ot:   "{t}, sacrifice this artifact: exile target player's graveyard.",
			want: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := hasGraveyardSizeScalePattern(c.ot)
			if got != c.want {
				t.Errorf("hasGraveyardSizeScalePattern(%q) = %v, want %v", c.ot, got, c.want)
			}
		})
	}
}

// TestCardIsReanimationEffect_NoLeakOnUnearth verifies the otClean
// migration on the canonical-reanimation-shapes detector. The unearth
// reminder shares "return this card from your graveyard to the
// battlefield" phrasing with the real-reanimation substring trio.
// Lord of Shatterskull Pass has unearth + leveler text; it should NOT
// classify as a reanimation effect on the strength of the unearth
// reminder alone (it has no actual "return target creature card from
// graveyard" effect of its own).
func TestCardIsReanimationEffect_NoLeakOnUnearth(t *testing.T) {
	cases := []struct {
		name, ot string
		want     bool
	}{
		{
			name: "Lord of Shatterskull Pass — unearth reminder only",
			ot: "lord of shatterskull pass enters with three level counters on it.\n" +
				"level up {r}\n" +
				"unearth {2}{r} ({2}{r}: return this card from your graveyard to the battlefield. " +
				"it gains haste. exile it at the beginning of the next end step or if it would " +
				"leave the battlefield. unearth only as a sorcery.)",
			want: false, // unearth self-return isn't a reanimation engine
		},
		{
			name: "Reanimate — real reanimation",
			ot: "put target creature card from a graveyard onto the battlefield under your control. " +
				"you lose life equal to its mana value.",
			want: true,
		},
		{
			name: "Eternal Witness — graveyard recursion to hand, not battlefield",
			ot:   "when this creature enters, you may return target card from your graveyard to your hand.",
			want: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := cardIsReanimationEffect(c.name, c.ot, false, "")
			if got != c.want {
				t.Errorf("cardIsReanimationEffect(%q) = %v, want %v", c.name, got, c.want)
			}
		})
	}
}
