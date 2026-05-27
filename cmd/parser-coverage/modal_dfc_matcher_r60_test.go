package main

import "testing"

// findMatcher finds the mechanic matcher func by name. The map is
// table-driven so an O(n) scan over <40 entries is fine in tests.
func findMatcher(t *testing.T, name string) mechanicMatcher {
	t.Helper()
	for _, def := range mechanicDefs {
		if def.name == name {
			return def.match
		}
	}
	t.Fatalf("mechanic %q not registered in mechanicDefs", name)
	return nil
}

// TestModalDFCMatcher_LayoutModalDFC pins the load-bearing fix: any
// card carrying layout="modal_dfc" must match, regardless of whether
// its substituted front-face oracle text mentions "transform" or "//".
// Pre-fix: 90 of 96 MDFCs silently missed the bucket because
// loadOracle's card_faces[0] substitution dropped the // separator
// and the "transform" keyword.
func TestModalDFCMatcher_LayoutModalDFC(t *testing.T) {
	match := findMatcher(t, "modal_dfc_transform")
	r := result{
		Name:       "Witch Enchanter // Witch-Blessed Meadow",
		Layout:     "modal_dfc",
		TypeLine:   "Creature — Human Warlock // Land",
		OracleText: "When this creature enters, destroy target artifact or enchantment an opponent controls.",
	}
	if !match(r) {
		t.Error("MDFC with layout=modal_dfc must match — load-bearing fix for the 90 silently-missed cards")
	}
}

// TestModalDFCMatcher_LayoutTransform covers the sibling DFC family
// (Innistrad werewolves, Garruk/Liliana DFC planeswalkers, Eldritch
// Moon meld pairs, etc.). These have layout="transform" and were
// caught pre-fix only when their front-face oracle text happened to
// mention "transform" (the keyword). Werewolves whose printed
// activation lives on the back face would have missed the bucket.
func TestModalDFCMatcher_LayoutTransform(t *testing.T) {
	match := findMatcher(t, "modal_dfc_transform")
	r := result{
		Name:       "Westvale Abbey // Ormendahl, Profane Prince",
		Layout:     "transform",
		TypeLine:   "Land // Legendary Creature — Demon",
		OracleText: "{T}: Add {C}. (back-face activation lives on the back face, not the front)",
	}
	if !match(r) {
		t.Error("DFC with layout=transform must match regardless of front-face text content")
	}
}

// TestModalDFCMatcher_TransformKeywordFallback defends the legacy
// keyword fallback. A hypothetical single-face card whose oracle text
// uses "transform" (e.g., one-shot effects that transform a permanent)
// must still match, since the field-driven check captures the named
// mechanic family rather than just the layout family.
func TestModalDFCMatcher_TransformKeywordFallback(t *testing.T) {
	match := findMatcher(t, "modal_dfc_transform")
	r := result{
		Name:       "Hypothetical Transform Sorcery",
		Layout:     "normal",
		TypeLine:   "Sorcery",
		OracleText: "Transform target werewolf you control.",
	}
	if !match(r) {
		t.Error("single-face card mentioning the transform keyword must still match (legacy fallback)")
	}
}

// TestModalDFCMatcher_RejectsUnrelated defends against the new layout
// check picking up adjacent multi-face layouts. Split cards
// (layout="split"), adventures (layout="adventure"), and ordinary
// single-face cards must NOT enter the bucket — they have their own
// matchers and inflating modal_dfc_transform's denominator would
// dilute the signal.
func TestModalDFCMatcher_RejectsUnrelated(t *testing.T) {
	match := findMatcher(t, "modal_dfc_transform")
	cases := []struct {
		name string
		r    result
	}{
		{"split_card", result{
			Name: "Fire // Ice", Layout: "split",
			TypeLine: "Instant // Instant", OracleText: "Fire deals 2 damage. // Tap target permanent.",
		}},
		{"adventure_card", result{
			Name: "Bonecrusher Giant // Stomp", Layout: "adventure",
			TypeLine: "Creature — Giant // Instant — Adventure",
			OracleText: "Whenever Bonecrusher Giant becomes the target of a spell, Bonecrusher Giant deals 2 damage to that spell's controller.",
		}},
		{"vanilla_creature", result{
			Name: "Grizzly Bears", Layout: "normal",
			TypeLine: "Creature — Bear", OracleText: "",
		}},
		{"instant_no_transform_keyword", result{
			Name: "Lightning Bolt", Layout: "normal",
			TypeLine: "Instant", OracleText: "Lightning Bolt deals 3 damage to any target.",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if match(tc.r) {
				t.Errorf("%s must NOT match modal_dfc_transform", tc.r.Name)
			}
		})
	}
}

// TestModalDFCMatcher_LayoutPropagatesThroughClassify pins the Layout
// field flowing from oracleEntry → result via classify(). Without
// this propagation the matcher's new layout check would always see
// "" and silently fall back to the legacy keyword path.
func TestModalDFCMatcher_LayoutPropagatesThroughClassify(t *testing.T) {
	corpus := buildEmptyAbilityCorpus(t, "Witch Enchanter // Witch-Blessed Meadow")
	e := oracleEntry{
		Name:       "Witch Enchanter // Witch-Blessed Meadow",
		Layout:     "modal_dfc",
		TypeLine:   "Creature — Human Warlock // Land",
		OracleText: "front face text",
	}
	r := classify(e, corpus)
	if r.Layout != "modal_dfc" {
		t.Errorf("classify must propagate Layout: want \"modal_dfc\", got %q", r.Layout)
	}
}
