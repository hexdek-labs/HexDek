package main

import (
	"strings"
	"testing"
)

// tutor_modal_complex_r60_test.go — regressions for the r60 tutor inference
// extension covering modal spells and complex wording that the substring
// matcher dropped or mis-classified before this PR.
//
// Each case is sourced from current Scryfall oracle text (lowercased, with
// the reminder-text parenthetical preserved verbatim so the reminder-strip
// helper is exercised end-to-end). The previous "before" behavior is
// recorded in the case comment so a future regressor can see which axis
// flipped if a case starts failing.

type modalTutorCase struct {
	name       string
	ot         string
	tl         string
	wantTutor  bool
	wantLand   bool
	wantWish   bool
	wantRestr  string // "" → don't assert
	wantDeliv  string // "" → don't assert
	whyBefore  string // documentation: what the pre-r60 matcher reported
}

var modalComplexCases = []modalTutorCase{
	// --- FIXES ----------------------------------------------------------

	{
		// Before: IsTutor=false. The substring "search your library" is
		// absent, and the reveal-until family didn't fire because Tainted
		// Pact uses "exile the top card" + "repeat this process" instead.
		// Result: the canonical cEDH black tutor for Thassa's Oracle was
		// silently dropped from every tutor count.
		name: "Tainted Pact",
		ot: "exile the top card of your library. you may put that card into your hand " +
			"unless it has the same name as another card exiled this way. repeat this " +
			"process until you put a card into your hand or you exile two cards with " +
			"the same name, whichever comes first.",
		tl:        "instant",
		wantTutor: true,
		whyBefore: "exile-until-unique pattern had no substring anchor",
	},
	{
		// Before: delivery="exile". The suspend reminder text says "exile
		// it with two time counters on it" and "you may cast it without
		// paying its mana cost", both of which the delivery scan picked
		// up as exile-delivery. Reminder stripping fixes this.
		name: "Profane Tutor",
		ot: "suspend 2—{1}{b} (rather than cast this card from your hand, pay {1}{b} " +
			"and exile it with two time counters on it. at the beginning of your upkeep, " +
			"remove a time counter. when the last is removed, you may cast it without " +
			"paying its mana cost.)\n" +
			"search your library for a card, put that card into your hand, then shuffle.",
		tl:        "sorcery",
		wantTutor: true,
		wantRestr: "any",
		wantDeliv: "hand",
		whyBefore: "suspend reminder text leaked 'exile it' + 'you may cast' into delivery",
	},
	{
		// Before: delivery="battlefield". The second bullet ("from your
		// hand onto the battlefield") leaked into the search-mode's
		// delivery answer. Modal scoping restricts the scan to the
		// library-search bullet.
		name: "Tooth and Nail",
		ot: "choose one —\n" +
			"• search your library for up to two creature cards, reveal them, " +
			"put them into your hand, then shuffle.\n" +
			"• put up to two creature cards from your hand onto the battlefield.\n" +
			"entwine {2}{g}{g} (choose both if you pay the entwine cost.)",
		tl:        "sorcery",
		wantTutor: true,
		wantRestr: "creature",
		wantDeliv: "hand",
		whyBefore: "second-mode 'onto the battlefield' bled into modal delivery",
	},
	{
		// Before: restriction="any". The dual-zone phrasing "library
		// and/or graveyard for a creature" prevented the type-bucket
		// substring match. Normalization rewrites it to the canonical
		// "search your library for a creature" form.
		name: "Finale of Devastation",
		ot: "search your library and/or graveyard for a creature card with mana value " +
			"x or less and put it onto the battlefield. if you search your library this " +
			"way, shuffle. if x is 10 or more, creatures you control get +x/+x and gain " +
			"haste until end of turn.",
		tl:        "sorcery",
		wantTutor: true,
		wantRestr: "cmc_variable",
		wantDeliv: "battlefield",
		whyBefore: "dual-zone 'library and/or graveyard for a creature' lost the type bucket",
	},
	{
		// Before: delivery="hand". "Put those cards on top in any order"
		// doesn't match any of the canonical "on top" phrasings — the
		// added "put those cards on top" needle catches it.
		name: "Insidious Dreams",
		ot: "as an additional cost to cast this spell, discard x cards.\n" +
			"search your library for x cards, then shuffle and put those cards on top " +
			"in any order.",
		tl:        "instant",
		wantTutor: true,
		wantDeliv: "top",
		whyBefore: "'put those cards on top in any order' wasn't in the top-delivery needle list",
	},
	{
		// Before: delivery="hand". "Third from the top" places the card
		// on the library, not in hand. New "Nth from the top" needles
		// route it correctly.
		name: "Long-Term Plans",
		ot:        "search your library for a card, then shuffle and put that card third from the top.",
		tl:        "instant",
		wantTutor: true,
		wantDeliv: "top",
		whyBefore: "'third from the top' wasn't in the top-delivery needle list",
	},

	// --- PINS (already worked, kept so future refactors don't regress) ----

	{
		// Plain "search your library for a card" buried behind an exile
		// preamble. The substring family always caught this, but worth
		// pinning so the exile-until matcher above doesn't accidentally
		// swallow it.
		name: "Demonic Bargain",
		ot: "exile the top thirteen cards of your library, then search your library " +
			"for a card. put that card into your hand, then shuffle.",
		tl:        "sorcery",
		wantTutor: true,
		wantRestr: "any",
		wantDeliv: "hand",
		whyBefore: "already worked — pinned to defend against exile-until widening",
	},
	{
		name: "Diabolic Intent",
		ot: "as an additional cost to cast this spell, sacrifice a creature.\n" +
			"search your library for a card, put that card into your hand, then shuffle.",
		tl:        "sorcery",
		wantTutor: true,
		wantRestr: "any",
		wantDeliv: "hand",
		whyBefore: "already worked — pinned for completeness alongside Demonic Bargain",
	},
	{
		// Reveal-until with "with the chosen name" — confirms the
		// reveal-until family still catches name-anchored variants now
		// that we also have an exile-until matcher in the same area.
		name: "Demonic Consultation",
		ot: "choose a card name. exile the top six cards of your library, then reveal " +
			"cards from the top of your library until you reveal a card with the chosen " +
			"name. put that card into your hand and exile all other cards revealed this way.",
		tl:        "instant",
		wantTutor: true,
		whyBefore: "already worked via reveal-until family",
	},
	{
		// Pure-reveal sibling of Demonic Consultation.
		name: "Spoils of the Vault",
		ot: "choose a card name. reveal cards from the top of your library until you " +
			"reveal a card with that name, then put that card into your hand. exile all " +
			"other cards revealed this way, and you lose 1 life for each of the exiled cards.",
		tl:        "instant",
		wantTutor: true,
		whyBefore: "already worked via reveal-until family",
	},
	{
		// Negative: artifact that exiles top-of-library face-down for a
		// future battlefield dump is NOT a tutor — no choice of card.
		// Pinned so the exile-until matcher's three-anchor guard stays
		// narrow enough to keep rejecting blind-exile effects.
		name: "Pyxis of Pandemonium",
		ot: "{t}: each player exiles the top card of their library face down.\n" +
			"{7}, {t}, sacrifice this artifact: each player turns face up all cards " +
			"they own exiled with this artifact, then puts all permanent cards among " +
			"them onto the battlefield.",
		tl:        "artifact",
		wantTutor: false,
		whyBefore: "already correctly false — pinned as a negative for exile-until widening",
	},
	{
		// Activated tutor on a permanent with a control-trade downside.
		// Wishclaw says "search your library for a card" — substring
		// matched it pre-r60. Pinned as the canonical activated-tutor.
		name: "Wishclaw Talisman",
		ot: "this artifact enters with three wish counters on it.\n" +
			"{1}, {t}, remove a wish counter from this artifact: search your library " +
			"for a card, put it into your hand, then shuffle. an opponent gains control " +
			"of this artifact. activate only during your turn.",
		tl:        "artifact",
		wantTutor: true,
		wantRestr: "any",
		wantDeliv: "hand",
		whyBefore: "already worked — pinned alongside Profane Tutor activated-pattern",
	},
}

func TestClassifyTutor_ModalAndComplexWording(t *testing.T) {
	for _, tc := range modalComplexCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ot := strings.ToLower(tc.ot)
			tl := strings.ToLower(tc.tl)

			p := CardProfile{Name: tc.name}
			classifyTutorInto(&p, ot, tl, tc.name)

			if p.IsTutor != tc.wantTutor {
				t.Fatalf("IsTutor=%v want %v (was-before: %s)", p.IsTutor, tc.wantTutor, tc.whyBefore)
			}
			if p.IsLandTutor != tc.wantLand {
				t.Errorf("IsLandTutor=%v want %v", p.IsLandTutor, tc.wantLand)
			}
			if p.IsWishTutor != tc.wantWish {
				t.Errorf("IsWishTutor=%v want %v", p.IsWishTutor, tc.wantWish)
			}

			if tc.wantRestr == "" && tc.wantDeliv == "" {
				return
			}
			restr, deliv := inferTutorRestriction(ot, tl)
			if tc.wantRestr != "" && restr != tc.wantRestr {
				t.Errorf("restriction=%q want %q (was-before: %s)", restr, tc.wantRestr, tc.whyBefore)
			}
			if tc.wantDeliv != "" && deliv != tc.wantDeliv {
				t.Errorf("delivery=%q want %q (was-before: %s)", deliv, tc.wantDeliv, tc.whyBefore)
			}
		})
	}
}

// TestStripReminderText pins the reminder-strip helper across the wording
// shapes that show up in the wild — single-sentence, multi-sentence,
// nested-paren (rare but appears in Un-set / silver-border printings),
// and unbalanced (defensive — the helper must not deadlock).
func TestStripReminderText(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"foo (bar) baz", "foo  baz"},
		{"a (b. c. d.) e", "a  e"},
		{"x (y (z) y) x", "x  x"},
		{"unbalanced (text without close", "unbalanced "},
		{"close without open) more", "close without open) more"},
		{"no parens at all", "no parens at all"},
	}
	for _, c := range cases {
		got := stripReminderText(c.in)
		if got != c.want {
			t.Errorf("stripReminderText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestScopeToSearchSegment pins the modal-scoping helper. Bulleted
// oracles get narrowed to the search-bearing bullets; non-modal text
// passes through untouched; bulleted text with no search-bearing bullet
// also passes through untouched (so the caller can still apply default
// delivery inference instead of degrading to "hand" silently).
func TestScopeToSearchSegment(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{
			name: "no bullets",
			in:   "search your library for a card.",
			want: "search your library for a card.",
		},
		{
			name: "modal with search in mode 1 only",
			in: "choose one — • search your library for a creature, put it into your hand. " +
				"• put a creature from your hand onto the battlefield.",
			want: " search your library for a creature, put it into your hand. ",
		},
		{
			name: "modal with no search anywhere",
			in:   "choose one — • destroy target creature. • exile target creature.",
			want: "choose one — • destroy target creature. • exile target creature.",
		},
		{
			name: "modal with reveal-until in one mode",
			in: "choose one — • counter target spell. • reveal cards from the top of your " +
				"library until you reveal an instant, put it into your hand.",
			want: " reveal cards from the top of your library until you reveal an instant, " +
				"put it into your hand.",
		},
	}
	for _, c := range cases {
		got := scopeToSearchSegment(c.in)
		if got != c.want {
			t.Errorf("%s: scopeToSearchSegment = %q, want %q", c.name, got, c.want)
		}
	}
}
