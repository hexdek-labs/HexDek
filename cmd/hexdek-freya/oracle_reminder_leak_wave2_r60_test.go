package main

import (
	"testing"
)

// oracle_reminder_leak_wave2_r60_test.go — second wave of NLP-grade
// detector migrations. Wave 1 (PR #799) migrated 5 detector families
// (card draw / graveyard recursion / life gain / counter removal /
// damage effect) to otClean. This file pins 5 more wave-2 migrations:
//
//   - Cast-from-exile (`exile + you may cast/play`): cascade reminder
//     "(... You may cast it without paying its mana cost. ...)" tagged
//     every cascade card with exile_cast + ResCard. Suspend reminder
//     ("you may cast it without paying its mana cost") did the same.
//   - Impulse draw (`exile the top + may play/cast`): same cascade /
//     foretell reminder shape; pre-fix Bloodbraid Elf surfaced
//     impulse_draw despite being a one-shot keyword effect.
//   - Top-of-library manipulation (`top of + library`): cascade
//     reminder's "exile cards from the top of your library" tagged
//     every cascade card as a topdeck-manipulator.
//   - Counter add via "+1/+1 counter" (counter_add + ResCounter): riot
//     reminder "(This creature enters with your choice of a +1/+1
//     counter or haste.)" tagged every riot creature as a counter
//     producer, distorting +1/+1 counter archetype detection.
//   - Stat boost "gets +1/+1" mistakenly tagged as counter producer:
//     dropped from the ResCounter needles entirely. "gets +1/+1 until
//     end of turn" is Giant Growth, an anthem, or a prowess trigger —
//     not a +1/+1 counter producer. The misnomer was wrong on every
//     printing, not just keyword-reminder cards.
//
// Plus zone-flow-level migrations in classifyZoneFlows for the
// graveyard->battlefield and graveyard->hand bridges (unearth / dredge /
// recover reminders), which were the same reminder shapes leaking into
// the value-chain detector pipeline.

type wave2Case struct {
	name, ot, tl, mc, pwr string
	cmc                    int

	// Negative assertions — must be absent.
	wantNoEffect    []string // e.g. ["exile_cast", "impulse_draw"]
	wantNoProduces  []ResourceType
	wantNoZoneFlow  string // a "from->to" string like "graveyard->hand"

	// Positive sanity — must survive the migration.
	mustHaveEffect   string
	mustHaveProduces ResourceType
}

var wave2Cases = []wave2Case{
	// --- FIXES ----------------------------------------------------------

	{
		// Cascade reminder: "(... exile cards from the top of your library
		// until you exile a nonland card that costs less. You may cast it
		// without paying its mana cost. Put the exiled cards on the bottom
		// in a random order.)" — pre-fix tagged Bloodbraid as
		// Effects=[exile_cast, topdeck_manipulate, impulse_draw] +
		// Produces=[card]. Cascade is a one-shot keyword on cast, not a
		// recurring engine. Post-fix Bloodbraid has the clean profile of
		// a haste creature with no extra value-engine flags.
		name: "Bloodbraid Elf",
		ot: "Haste (This creature can attack and {T} as soon as it comes under your control.)\n" +
			"Cascade (When you cast this spell, exile cards from the top of your library " +
			"until you exile a nonland card that costs less. You may cast it without paying " +
			"its mana cost. Put the exiled cards on the bottom in a random order.)",
		tl: "creature — elf berserker", mc: "{2}{R}{G}", cmc: 4, pwr: "3",
		wantNoEffect:   []string{"exile_cast", "impulse_draw", "topdeck_manipulate"},
		wantNoProduces: []ResourceType{ResCard},
	},
	{
		// Riot reminder: "(This creature enters with your choice of a
		// +1/+1 counter or haste.)" — pre-fix tagged with
		// Effects=[counter_add] + Produces=[counter] from the parenthetical.
		// Real card body has "draw a card" via the cast-attack trigger;
		// that survives because the body text isn't parenthetical.
		name: "Burning-Tree Vandal",
		ot: "Riot (This creature enters with your choice of a +1/+1 counter or haste.)\n" +
			"Whenever this creature attacks, you may discard a card. If you do, draw a card.",
		tl: "creature — human warrior", mc: "{2}{R}", cmc: 3, pwr: "3",
		wantNoEffect:     []string{"counter_add"},
		wantNoProduces:   []ResourceType{ResCounter},
		mustHaveProduces: ResCard, // genuine "draw a card" body survives
	},
	{
		// Suspend reminder: "(Rather than cast this card from your hand,
		// you may pay {G} and exile it with two time counters on it. At
		// the beginning of your upkeep, remove a time counter. When the
		// last is removed, you may cast it without paying its mana cost.)"
		// — pre-fix tagged Search for Tomorrow with exile_cast +
		// Produces=[card]. The actual body is a land tutor; that
		// classification correctly survives the migration.
		name: "Search for Tomorrow",
		ot: "Search your library for a basic land card, put it onto the battlefield, then shuffle.\n" +
			"Suspend 2—{G} (Rather than cast this card from your hand, you may pay {G} and " +
			"exile it with two time counters on it. At the beginning of your upkeep, remove " +
			"a time counter. When the last is removed, you may cast it without paying its mana cost.)",
		tl: "sorcery", mc: "{2}{G}", cmc: 3, pwr: "",
		wantNoEffect:   []string{"exile_cast"},
		mustHaveEffect: "land_fetch", // genuine body land-tutor survives
	},
	{
		// Prowess reminder: "(Whenever you cast a noncreature spell, this
		// creature gets +1/+1 until end of turn.)" — pre-fix tagged Shu
		// Yun's Produces with ResCounter because of the "gets +1/+1"
		// needle. Stat boost ≠ counter; dropping the needle universally
		// fixes the misnomer.
		name: "Shu Yun, the Silent Tempest",
		ot: "Prowess (Whenever you cast a noncreature spell, this creature gets +1/+1 until end of turn.)\n" +
			"Whenever you cast a noncreature spell, you may pay {R/W}{R/W}. If you do, target " +
			"creature gains double strike until end of turn.",
		tl: "legendary creature — human monk", mc: "{2}{U}", cmc: 3, pwr: "2",
		wantNoProduces: []ResourceType{ResCounter},
	},
	{
		// Unearth reminder: "({2}: Return this card from your graveyard
		// to the battlefield. It gains haste. Exile it at the beginning
		// of the next end step or if it would leave the battlefield.
		// Unearth only as a sorcery.)" — pre-fix the zone-flow detector
		// classified this as a graveyard->battlefield bridge, distorting
		// value-chain analysis. Unearth self-return isn't a reanimator
		// engine; it's keyword-scoped self-recursion.
		name: "Lord of Shatterskull Pass (Unearth-test stub)",
		ot: "Lord of Shatterskull Pass enters with three level counters on it.\n" +
			"Level up {R}\n" +
			"Unearth {2}{R} ({2}{R}: Return this card from your graveyard to the battlefield. " +
			"It gains haste. Exile it at the beginning of the next end step or if it would " +
			"leave the battlefield. Unearth only as a sorcery.)",
		tl: "creature — minotaur warrior", mc: "{3}{R}{R}", cmc: 5, pwr: "3",
		wantNoZoneFlow: "graveyard->battlefield",
	},
	{
		// Dredge reminder zone-flow leak (companion to the wave-1 fix
		// for IsRecursion). The classifyZoneFlows graveyard->hand bridge
		// also leaked on dredge: every dredge creature surfaced as a
		// graveyard->hand value-chain bridge.
		name: "Stinkweed Imp (zone-flow regression)",
		ot: "Flying\n" +
			"Whenever this creature deals combat damage to a creature, destroy that creature.\n" +
			"Dredge 5 (If you would draw a card, you may mill five cards instead. If you do, " +
			"return this card from your graveyard to your hand.)",
		tl: "creature — imp", mc: "{B}", cmc: 1, pwr: "1",
		wantNoZoneFlow: "graveyard->hand",
	},

	// --- TRUE-POSITIVE PINS ---------------------------------------------

	{
		// Real cast-from-exile: body says "You may cast that card for as
		// long as it remains exiled". Must keep Effects=[exile_cast] +
		// Produces=[card] after the migration.
		name: "Hostage Taker",
		ot: "When this creature enters, exile another target creature or artifact until this " +
			"creature leaves the battlefield. You may cast that card for as long as it remains " +
			"exiled, and you may spend mana as though it were mana of any color to cast that spell.",
		tl: "creature — human pirate", mc: "{2}{U}{B}", cmc: 4, pwr: "2",
		mustHaveEffect:   "exile_cast",
		mustHaveProduces: ResCard,
	},
	{
		// Real impulse_draw + topdeck manipulation: body literally exiles
		// the top of each library and grants cast permission. Both flags
		// should survive otClean since the relevant text isn't
		// parenthetical.
		name: "Etali, Primal Storm",
		ot: "Whenever Etali attacks, exile the top card of each player's library, then you may " +
			"cast any number of spells from among those cards without paying their mana costs.",
		tl: "legendary creature — elder dinosaur", mc: "{4}{R}{R}", cmc: 6, pwr: "6",
		mustHaveEffect: "impulse_draw",
	},
	{
		// Real +1/+1 counter producer: body has "{1}, {T}: Put a +1/+1
		// counter on this creature" and the ETB clause "enters with X
		// +1/+1 counters on it". The migration must keep counter_add
		// classification on cards that actually produce counters via
		// non-reminder body text.
		name: "Hangarback Walker",
		ot: "This creature enters with X +1/+1 counters on it.\n" +
			"When this creature dies, create a 1/1 colorless Thopter artifact creature token " +
			"with flying for each +1/+1 counter on this creature.\n" +
			"{1}, {T}: Put a +1/+1 counter on this creature.",
		tl: "artifact creature — construct", mc: "{X}{X}", cmc: 0, pwr: "0",
		mustHaveEffect:   "counter_add",
		mustHaveProduces: ResCounter,
	},
	{
		// Real graveyard->hand recursion: Eternal Witness's body
		// explicitly returns target card from graveyard to hand. Must
		// survive the valuechains migration (companion to the wave 1
		// IsRecursion regression).
		name: "Eternal Witness",
		ot:   "When this creature enters, you may return target card from your graveyard to your hand.",
		tl:   "creature — human shaman", mc: "{1}{G}{G}", cmc: 3, pwr: "2",
		// No wantNoZoneFlow — this card SHOULD register graveyard->hand.
	},
}

func TestClassifyCard_Wave2ReminderLeakFixes(t *testing.T) {
	for _, tc := range wave2Cases {
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

// hasZoneFlow scans the ZoneFlows slice for a flow whose From->To pair
// matches the given "from->to" string. Used by the zone-flow regressions
// for unearth / dredge / recover keyword-reminder leaks.
func hasZoneFlow(flows []ZoneFlow, key string) bool {
	for _, f := range flows {
		if f.From+"->"+f.To == key {
			return true
		}
	}
	return false
}

// TestGetsPlusOneSlashOne_NoLongerCountsAsCounter pins the dropped
// "gets +1/+1" needle. Pre-r60-wave-2 the ResCounter detector listed
// "gets +1/+1" alongside real counter needles like "put a +1/+1
// counter" — a misnomer that classified Giant Growth, anthems, and
// every prowess trigger as a counter producer. This test pins that
// the needle drop is permanent.
func TestGetsPlusOneSlashOne_NoLongerCountsAsCounter(t *testing.T) {
	cases := []struct {
		name, ot string
	}{
		{"Giant Growth", "Target creature gets +3/+3 until end of turn."},
		{"Glorious Anthem", "Creatures you control get +1/+1."},
		{"Prowess (synthetic body)", "Whenever you cast a noncreature spell, this creature gets +1/+1 until end of turn."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := ClassifyCard(c.name, c.ot, "instant", "{G}", 1, "")
			if hasResource(p.Produces, ResCounter) {
				t.Errorf("%s: Produces contains ResCounter despite no real counter; got=%v",
					c.name, p.Produces)
			}
		})
	}
}
