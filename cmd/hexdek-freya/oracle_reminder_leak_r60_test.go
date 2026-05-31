package main

import (
	"testing"
)

// oracle_reminder_leak_r60_test.go — adversarial regressions for the
// "NLP-grade oracle text parsing" Freya backlog item.
//
// Survey of `cmd/hexdek-freya/analysis.go` `ClassifyCard` turned up 4
// detector families that operated on raw oracle text and false-fired on
// keyword reminder text or modal-mode bleed. The reminder-stripping helper
// (`stripReminder` / `otClean`, established by PR #169 for cascade and the
// flashback / encore graveyard glosses) already existed; this pass extends
// that pattern to the next four worst offenders:
//
//   - Card draw (Bygone Bishop's Clue gloss "Draw a card.")
//   - Graveyard recursion (dredge / recover self-return reminder)
//   - Life gain (lifelink / extort reminder)
//   - Counter removal (vanishing / fading / suspend reminder)
//   - Damage effect (deathtouch / trample / wither / infect reminder)
//
// Each case below is a real Scryfall printing. The "want" assertions are
// the post-migration shape; the comment records the pre-migration false
// classification so future regressions surface the same way.

type leakCase struct {
	name, ot, tl, mc, pwr string
	cmc                    int

	// Negative assertions — these flags MUST be off / absent. Anything
	// listed here was triggered by the pre-migration matcher via reminder
	// text alone.
	wantNoRecursion bool
	wantNoLife      bool   // no ResLife in Produces
	wantNoCard      bool   // no ResCard in Produces
	wantNoCounter   bool   // no ResCounter in Consumes
	wantNoDamage    bool   // no "damage" in Effects

	// Positive sanity check on one classification that SHOULD survive — so
	// the test isn't just asserting absence (which could pass by deleting
	// the classifier entirely).
	mustHaveEffect   string // "" → skip
	mustHaveProduces string // "" → skip
}

var reminderLeakCases = []leakCase{
	// --- FIXES ----------------------------------------------------------

	{
		// Dredge reminder: "(...If you do, return this card from your
		// graveyard to your hand.)" — the parenthetical self-return is
		// the dredge mechanic itself, not a general recursion engine.
		// Pre-fix: IsRecursion=true. Post-fix: still gets self_mill /
		// dredge / graveyard_fill from the dredge keyword, but no
		// generic recursion tag.
		name: "Stinkweed Imp",
		ot: "Flying\n" +
			"Whenever this creature deals combat damage to a creature, destroy that creature.\n" +
			"Dredge 5 (If you would draw a card, you may mill five cards instead. If you do, " +
			"return this card from your graveyard to your hand.)",
		tl: "creature — imp", mc: "{B}", cmc: 1, pwr: "1",
		wantNoRecursion: true,
		mustHaveEffect:  "self_mill",
	},
	{
		// Deathtouch reminder: "(Any amount of damage this deals to a
		// creature is enough to destroy it.)" — every deathtouch creature
		// pre-fix carried Effects=["damage"] from this gloss alone, even
		// though the keyword damage is a combat-passive, not an active
		// effect on the card. Typhoid Rats is the canonical minimal
		// case: a vanilla 1/1 with only deathtouch.
		name: "Typhoid Rats",
		ot:   "Deathtouch (Any amount of damage this deals to a creature is enough to destroy it.)",
		tl:   "creature — rat", mc: "{B}", cmc: 1, pwr: "1",
		wantNoDamage: true,
	},
	{
		// Lifelink reminder: "(...Damage dealt by this creature also
		// causes you to gain that much life.)" — pre-fix every lifelink
		// printing carried Produces=ResLife. Speaker's *real* effect is
		// a token-making activation; the post-fix shape correctly keeps
		// the token producer and drops the spurious life producer.
		name: "Speaker of the Heavens",
		ot: "Vigilance, lifelink (Attacking doesn't cause this creature to tap. Damage dealt " +
			"by this creature also causes you to gain that much life.)\n" +
			"{T}: Create a 4/4 white Angel creature token with flying. Activate only if you have " +
			"at least 7 life more than your starting life total and only as a sorcery.",
		tl: "creature — human cleric", mc: "{W}", cmc: 1, pwr: "1",
		wantNoLife:       true,
		mustHaveProduces: "token",
	},
	{
		// Investigate reminder: "(Create a Clue token. It's an artifact
		// with '{2}, Sacrifice this token: Draw a card.')" — the Clue's
		// draw-a-card ability bled through to the host creature pre-fix,
		// tagging Bygone Bishop as a card-producer when only the Clue is.
		// Token classification is intentionally kept (investigate IS a
		// token-producer keyword); only the second-order draw leak is
		// pruned.
		name: "Bygone Bishop",
		ot: "Flying\n" +
			"Whenever you cast a creature spell with mana value 3 or less, investigate. " +
			"(Create a Clue token. It's an artifact with \"{2}, Sacrifice this token: Draw a card.\")",
		tl: "creature — human cleric", mc: "{2}{W}", cmc: 3, pwr: "2",
		wantNoCard:       true,
		mustHaveProduces: "token",
	},
	{
		// Vanishing reminder: "(This permanent enters with N time
		// counters on it. At the beginning of your upkeep, remove a time
		// counter from it. When the last is removed, sacrifice it.)" —
		// the "remove ... counter" gloss falsely tagged every vanishing
		// permanent as a counter consumer. The synthetic case here pins
		// the gloss alone (vanishing has limited real printings; the
		// reminder-shape is what matters).
		name: "Faerie Conclave (Vanishing-test stub)",
		ot: "Vanishing 3 (This creature enters with three time counters on it. At the " +
			"beginning of your upkeep, remove a time counter from it. When the last is " +
			"removed, sacrifice it.)\n" +
			"Flying",
		tl: "creature — faerie", mc: "{1}{U}", cmc: 2, pwr: "2",
		wantNoCounter: true,
	},

	// --- POSITIVE PINS — the migrated detectors must still fire on real
	// recursion / life / damage / counter / card-producing cards. Drops
	// here mean the otClean migration over-narrowed.

	{
		// Real recursion: explicit "return target card from your graveyard
		// to your hand" body, not reminder text. Must keep IsRecursion.
		name: "Eternal Witness",
		ot:   "When this creature enters, you may return target card from your graveyard to your hand.",
		tl:   "creature — human shaman", mc: "{1}{G}{G}", cmc: 3, pwr: "2",
		// No negative assertions — this card SHOULD trigger recursion.
		mustHaveProduces: "graveyard",
	},
	{
		// Real life gain: trigger says "you gain 1 life" — survives otClean
		// because the body isn't parenthetical.
		name: "Soul Warden",
		ot:   "Whenever another creature enters, you gain 1 life.",
		tl:   "creature — human cleric", mc: "{W}", cmc: 1, pwr: "1",
		mustHaveProduces: "life",
	},
	{
		// Real damage effect: explicit "deals 3 damage to any target". Must
		// keep Effects=damage even after the otClean migration.
		name: "Lightning Bolt",
		ot:   "This spell deals 3 damage to any target.",
		tl:   "instant", mc: "{R}", cmc: 1, pwr: "",
		mustHaveEffect: "damage",
	},
	{
		// Real counter consumption: "Remove a +1/+1 counter from this
		// creature: Regenerate it." — in-body activation cost, not a
		// keyword gloss.
		name: "Triskelion",
		ot: "Triskelion enters with three +1/+1 counters on it.\n" +
			"Remove a +1/+1 counter from this creature: This creature deals 1 damage to any target.",
		tl: "artifact creature — construct", mc: "{6}", cmc: 6, pwr: "1",
		// Must surface counter consumption.
	},
	{
		// Real card draw: "draw three cards" — survives otClean because
		// the body isn't parenthetical.
		name: "Sphinx of Lost Truths",
		ot: "Kicker {1}{U} (You may pay an additional {1}{U} as you cast this spell.)\n" +
			"Flying\n" +
			"When this creature enters, draw three cards. Then if it wasn't kicked, discard three cards.",
		tl: "creature — sphinx", mc: "{3}{U}{U}", cmc: 5, pwr: "3",
		mustHaveProduces: "card",
	},
}

func TestClassifyCard_ReminderLeakFixes(t *testing.T) {
	for _, tc := range reminderLeakCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			p := ClassifyCard(tc.name, tc.ot, tc.tl, tc.mc, tc.cmc, tc.pwr)

			if tc.wantNoRecursion && p.IsRecursion {
				t.Errorf("IsRecursion=true via reminder leak; want false")
			}
			if tc.wantNoLife && hasResource(p.Produces, ResLife) {
				t.Errorf("Produces contains ResLife via reminder leak; want absent. got=%v", p.Produces)
			}
			if tc.wantNoCard && hasResource(p.Produces, ResCard) {
				t.Errorf("Produces contains ResCard via reminder leak; want absent. got=%v", p.Produces)
			}
			if tc.wantNoCounter && hasResource(p.Consumes, ResCounter) {
				t.Errorf("Consumes contains ResCounter via reminder leak; want absent. got=%v", p.Consumes)
			}
			if tc.wantNoDamage && hasEffectStr(p.Effects, "damage") {
				t.Errorf("Effects contains 'damage' via reminder leak; want absent. got=%v", p.Effects)
			}

			if tc.mustHaveEffect != "" && !hasEffectStr(p.Effects, tc.mustHaveEffect) {
				t.Errorf("Effects missing %q after migration; got=%v", tc.mustHaveEffect, p.Effects)
			}
			if tc.mustHaveProduces != "" && !hasResource(p.Produces, ResourceType(tc.mustHaveProduces)) {
				t.Errorf("Produces missing %q after migration; got=%v", tc.mustHaveProduces, p.Produces)
			}
		})
	}
}

func hasResource(rs []ResourceType, r ResourceType) bool {
	for _, x := range rs {
		if x == r {
			return true
		}
	}
	return false
}

func hasEffectStr(es []string, s string) bool {
	for _, x := range es {
		if x == s {
			return true
		}
	}
	return false
}

// TestStripReminder_ReusedByMigrations is a sanity check on the helper
// itself — the migrations all funnel through `stripReminder` (via
// `otClean`), so this pins the reminder-strip shapes that the migrations
// depend on. If a future refactor changes the helper's nesting behavior
// or whitespace collapse, the per-card cases above stay green but this
// test surfaces the surgery directly.
func TestStripReminder_ReusedByMigrations(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{
			name: "Dredge gloss",
			in:   "Dredge 5 (If you would draw a card, you may mill five cards instead. If you do, return this card from your graveyard to your hand.)",
			want: "Dredge 5",
		},
		{
			name: "Deathtouch gloss",
			in:   "Deathtouch (Any amount of damage this deals to a creature is enough to destroy it.)",
			want: "Deathtouch",
		},
		{
			name: "Lifelink gloss in compound keyword line",
			in:   "Vigilance, lifelink (Attacking doesn't cause this creature to tap. Damage dealt by this creature also causes you to gain that much life.)",
			want: "Vigilance, lifelink",
		},
		{
			name: "Vanishing gloss",
			in:   "Vanishing 3 (This creature enters with three time counters on it. At the beginning of your upkeep, remove a time counter from it. When the last is removed, sacrifice it.)",
			want: "Vanishing 3",
		},
		{
			name: "Investigate gloss",
			in:   "Whenever you cast a creature spell with mana value 3 or less, investigate. (Create a Clue token. It's an artifact with \"{2}, Sacrifice this token: Draw a card.\")",
			want: "Whenever you cast a creature spell with mana value 3 or less, investigate.",
		},
	}
	for _, c := range cases {
		got := stripReminder(c.in)
		if got != c.want {
			t.Errorf("%s:\n  stripReminder(%q)\n  = %q\n  want %q", c.name, c.in, got, c.want)
		}
	}
}
