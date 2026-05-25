package main

import (
	"reflect"
	"strings"
	"testing"
)

// TestCleanForScan_DropInForInlineUse confirms CleanForScan returns the same
// result as the existing inline strings.ToLower(stripReminder(ot)) pattern
// used at analysis.go:211 — i.e. it is a true drop-in helper.
func TestCleanForScan_DropInForInlineUse(t *testing.T) {
	cases := []string{
		"Flying (This creature can't be blocked except by creatures with flying or reach.)",
		"Cascade (When you cast this spell, exile cards from the top of your library...)",
		"",
		"UPPERCASE text (with REMINDER) and a tail.",
	}
	for _, in := range cases {
		want := strings.ToLower(stripReminder(in))
		got := CleanForScan(in)
		if got != want {
			t.Errorf("CleanForScan(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSplitClauses_RealCards pins the splitter against shapes Freya
// actually sees. The contract: each clause is a sentence-or-shorter unit,
// cost/effect halves of activated abilities are split at the top-level
// colon, mana braces are preserved, newline-separated abilities are
// independent clauses.
func TestSplitClauses_RealCards(t *testing.T) {
	cases := []struct {
		name string
		ot   string
		want []string
	}{
		{
			name: "single sentence no period",
			ot:   "Lightning Bolt deals 3 damage to any target",
			want: []string{"Lightning Bolt deals 3 damage to any target"},
		},
		{
			name: "two sentences split on period",
			ot:   "Return target creature to its owner's hand. Then draw a card.",
			want: []string{
				"Return target creature to its owner's hand",
				"Then draw a card",
			},
		},
		{
			name: "newline-separated abilities",
			ot:   "Flying\nVigilance\nWhen this creature dies, draw a card.",
			want: []string{
				"Flying",
				"Vigilance",
				"When this creature dies, draw a card",
			},
		},
		{
			name: "activated ability splits at top-level colon",
			ot:   "{2}, {T}: Draw a card.",
			want: []string{
				"{2}, {T}",
				"Draw a card",
			},
		},
		{
			name: "mana braces stay intact",
			ot:   "Add {W}{U}{B}{R}{G}.",
			want: []string{"Add {W}{U}{B}{R}{G}"},
		},
		{
			name: "empty input returns nil",
			ot:   "",
			want: nil,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := SplitClauses(tc.ot)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("SplitClauses(%q)\n  got:  %#v\n  want: %#v", tc.ot, got, tc.want)
			}
		})
	}
}

// TestKeywordRegexCache_Stable confirms repeated lookups return the same
// compiled regex (cache hit) and produce identical match results — guards
// against accidental cache invalidation.
func TestKeywordRegexCache_Stable(t *testing.T) {
	r1 := keywordRegexFor("storm")
	r2 := keywordRegexFor("storm")
	if r1 != r2 {
		t.Error("keywordRegexFor should return the same *Regexp instance on cache hit")
	}
	if !r1.MatchString("storm count is two") {
		t.Error("storm should match as whole word")
	}
	if r1.MatchString("stormcrow swoops in") {
		t.Error("storm should not substring-match stormcrow")
	}
}

// --- The 10 worst FP/FN regressions. Each test pins one previously-known
// false-positive (or false-negative) case that the word-boundary rewrites
// now resolve. The negative-of-the-fix companion confirms the legitimate
// detection still fires.

// 1+2+3. Already pinned in oracletext_test.go (cascade, flashback,
// eternalize → EmptiesLibrary / graveyard_curate via otClean). Not
// re-asserted here.

// 4. Pre-fix the substring check on "storm" matched single-word compound
// names like "Stormtide Leviathan", "Stormchaser", "Stormrider Spirit",
// and any flavor mention of "stormcloud" / "stormfront" / "stormy" — all
// of which contain "storm" as an embedded substring but have no Storm
// keyword. The word-boundary rewrite stops the leak on closed compounds
// (note: two-word names like "Storm Crow" *do* legitimately whole-word
// match "storm" and are a separate, narrower problem — typically resolved
// by also requiring damage/copy keywords, which the existing companion
// guard already enforces).
func TestClassifyCard_StormtideNotStormFinisher(t *testing.T) {
	// Stormtide Leviathan's actual oracle — no Storm keyword.
	ot := "Islandwalk\nCreatures without flying or islandwalk can't attack."
	p := ClassifyCard("Stormtide Leviathan", ot, "Creature — Leviathan", "{5}{U}{U}{U}", 8, "8")
	if p.IsStormFinisher {
		t.Error("Stormtide Leviathan was flagged IsStormFinisher — 'storm' substring leaked from the creature name")
	}
}

// Companion case: a card whose flavor/quote contains "stormcloud" or
// "stormfront" pre-fix could also trip the same scan.
func TestClassifyCard_StormcloudFlavorNotStormFinisher(t *testing.T) {
	ot := "When this creature enters, it deals 1 damage to any target. The stormcloud above marks each strike."
	p := ClassifyCard("Cloud Striker", ot, "Creature — Elemental", "{1}{R}", 2, "1")
	if p.IsStormFinisher {
		t.Error("Cloud Striker flagged IsStormFinisher — 'stormcloud' substring leaked")
	}
}

// Negative-of-the-fix: a real storm spell still gets the tag.
func TestClassifyCard_GrapeshotStillStormFinisher(t *testing.T) {
	ot := "Grapeshot deals 1 damage to any target. Storm"
	p := ClassifyCard("Grapeshot", ot, "Sorcery", "{1}{R}", 2, "")
	if !p.IsStormFinisher {
		t.Error("Grapeshot lost IsStormFinisher after the word-boundary rewrite")
	}
}

// 5. Polymorph contains the substring "morph" — pre-fix hasMorphKeyword
// fired on Polymorphist's Jest and similar Poly* cards even though they
// have no morph mechanic.
func TestClassifyCard_PolymorphNotFacedownCreate(t *testing.T) {
	ot := "Until end of turn, each creature loses all abilities and becomes a 1/1 blue Frog creature."
	p := ClassifyCard("Polymorphist's Jest", ot, "Instant", "{2}{U}", 3, "")
	for _, e := range p.Effects {
		if e == "facedown_create" {
			t.Error("Polymorphist's Jest was flagged facedown_create — 'morph' substring leaked from 'Polymorphist'")
			break
		}
	}
}

// Negative-of-the-fix: a real morph creature still gets the tag.
func TestClassifyCard_WilltenderMorphStillFacedownCreate(t *testing.T) {
	ot := "Morph {3}{W} (You may cast this card face down as a 2/2 creature for {3}. Turn it face up any time for its morph cost.) When this creature is turned face up, target player can't cast spells this turn."
	p := ClassifyCard("Willbender", ot, "Creature — Human Wizard", "{1}{U}", 2, "1")
	found := false
	for _, e := range p.Effects {
		if e == "facedown_create" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Willbender (morph) lost facedown_create. Effects=%v", p.Effects)
	}
}

// 6. "Transformation" / "transformative" in oracle text pre-fix added the
// transform effect tag even when the card had no transform mechanic.
func TestClassifyCard_TransformationFlavorNotTransform(t *testing.T) {
	ot := "Target creature you control gains lifelink until end of turn. This is no transformation, merely a blessing."
	p := ClassifyCard("Spirit's Touch", ot, "Instant", "{W}", 1, "")
	for _, e := range p.Effects {
		if e == "transform" {
			t.Error("Spirit's Touch flagged transform — 'transformation' substring leaked")
			break
		}
	}
}

// Negative-of-the-fix: an actual transforming card still gets the tag.
func TestClassifyCard_WerewolfStillTransform(t *testing.T) {
	ot := "Daybound (If a player casts no spells during their own turn, it becomes night next turn.)\nAt the beginning of each upkeep, if it's night, transform this creature."
	p := ClassifyCard("Reckless Stormseeker", ot, "Creature — Human Werewolf", "{1}{R}", 2, "2")
	found := false
	for _, e := range p.Effects {
		if e == "transform" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Reckless Stormseeker (daybound) lost transform effect. Effects=%v", p.Effects)
	}
}

// 7. "Infectious" substring pre-fix flagged HasInfect on cards with no
// infect mechanic — e.g. Infectious Inquiry (a draw spell) or any card
// quoting "infection" / "infectious".
func TestClassifyCard_InfectiousInquiryNotHasInfect(t *testing.T) {
	ot := "Infectious Inquiry: target opponent reveals their hand. You draw a card."
	p := ClassifyCard("Infectious Inquiry", ot, "Instant", "{1}{B}", 2, "")
	if p.HasInfect {
		t.Error("Infectious Inquiry flagged HasInfect — 'infectious' substring leaked")
	}
}

// Negative-of-the-fix: a real infect grant still fires.
func TestClassifyCard_TaintedStrikeStillHasInfect(t *testing.T) {
	ot := "Target creature gains infect until end of turn."
	p := ClassifyCard("Tainted Strike", ot, "Instant", "{B}", 1, "")
	if !p.HasInfect {
		t.Error("Tainted Strike lost HasInfect after the word-boundary rewrite")
	}
}

// 8. A card whose oracle quotes "Millstone" by name pre-fix got the
// "mill" effect tag and was counted as a self-mill engine.
func TestClassifyCard_MillstoneNameQuoteNotMillEffect(t *testing.T) {
	// A flavor or theme card that mentions the artifact name without
	// having a mill ability of its own.
	ot := "Sacrifice this artifact: Search your library for a card named Millstone, reveal it, then shuffle."
	p := ClassifyCard("Antique Catalog", ot, "Artifact", "{2}", 2, "")
	for _, e := range p.Effects {
		if e == "mill" {
			t.Error("Antique Catalog flagged mill — 'Millstone' substring leaked")
			break
		}
	}
}

// Negative-of-the-fix: a real mill spell still fires.
func TestClassifyCard_GlimpseStillMill(t *testing.T) {
	ot := "Glimpse the Unthinkable: Target player mills ten cards."
	p := ClassifyCard("Glimpse the Unthinkable", ot, "Sorcery", "{U}{B}", 2, "")
	foundMill := false
	for _, e := range p.Effects {
		if e == "mill" {
			foundMill = true
			break
		}
	}
	if !foundMill {
		t.Errorf("Glimpse the Unthinkable lost mill effect. Effects=%v", p.Effects)
	}
}

// 9. Pre-fix "landfall" as a raw substring would match weird compounds,
// e.g. a card quoting "non-landfall" or a flavor passage with "landfalls".
// The implicit-landfall fallback ("whenever … land … enters") still picks
// up Tireless Tracker style triggers, so the keyword-only branch can be
// tightened without losing real positives.
func TestClassifyCard_NonLandfallFlavorNotLandfall(t *testing.T) {
	ot := "When this creature enters, draw a card. (This is not a landfalled trigger.)"
	p := ClassifyCard("Curious Spirit", ot, "Creature — Spirit", "{1}{U}", 2, "1")
	for _, trig := range p.Triggers {
		if trig == "landfall" {
			// After CleanForScan strips the parenthetical, the residual
			// is "draw a card" — no landfall trigger expected.
			t.Errorf("Curious Spirit flagged landfall trigger from non-keyword substring. Triggers=%v", p.Triggers)
			break
		}
	}
}

// Negative-of-the-fix: a real landfall card still fires.
func TestClassifyCard_LotusCobraStillLandfall(t *testing.T) {
	ot := "Landfall — Whenever a land you control enters, add one mana of any color."
	p := ClassifyCard("Lotus Cobra", ot, "Creature — Snake", "{1}{G}", 2, "2")
	found := false
	for _, trig := range p.Triggers {
		if trig == "landfall" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Lotus Cobra lost landfall trigger. Triggers=%v", p.Triggers)
	}
}

// 10. Modal-card awareness regression — splitModes already exists; this
// test pins the contract that modes are extracted in order and the
// preamble (slice index 0) is empty for a pure modal card, allowing
// downstream callers to dispatch per-mode rather than running every
// scanner against the full joined text. Documented here as a single
// integration anchor so future modal-aware classifiers have a regression
// to land against.
func TestSplitModes_PureModalNoPreamble(t *testing.T) {
	ot := "Choose one — • Counter target spell. • Draw a card."
	modes := splitModes(ot)
	if len(modes) != 3 {
		t.Fatalf("expected 3 elements (preamble + 2 modes), got %d: %#v", len(modes), modes)
	}
	if modes[0] != "" {
		t.Errorf("preamble should be empty for pure modal card, got %q", modes[0])
	}
	if modes[1] != "Counter target spell." {
		t.Errorf("mode 1 = %q", modes[1])
	}
	if modes[2] != "Draw a card." {
		t.Errorf("mode 2 = %q", modes[2])
	}
}
