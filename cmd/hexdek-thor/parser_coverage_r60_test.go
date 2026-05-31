package main

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// TestClassifyTrigger_ParserCoverageR60_GraveyardZoneChange pins the
// underscore-graveyard family that the parser emits as zone-change
// wrappers for "battlefield → graveyard". The substring catch above
// matches the prose forms "dies" / "is put into a graveyard" but
// misses the underscore slugs (self_put_into_graveyard_from_bf and
// friends) because they contain neither token. Each must route to
// the canonical creature_dies scaffold.
func TestClassifyTrigger_ParserCoverageR60_GraveyardZoneChange(t *testing.T) {
	cases := []string{
		"self_put_into_graveyard_from_bf",
		"ally_type_to_gy_from_bf",
		"type_to_gy_from_bf",
		"to_gy_from_bf",
		"opp_type_to_gy_from_bf",
		"ally_typed_to_gy",
		"tribal_to_gy_from_bf",
		"nontoken_type_to_gy",
		"opp_creature_to_gy",
		"self_to_gy",
		"self_die_or_ally_gy",
	}
	for _, ev := range cases {
		ev := ev
		t.Run(ev, func(t *testing.T) {
			got := classifyTrigger(&gameast.Trigger{Event: ev})
			if got != "creature_dies" {
				t.Errorf("classifyTrigger(%q): want creature_dies, got %q", ev, got)
			}
			if _, ok := triggerConditionActions["creature_dies"]; !ok {
				t.Fatalf("triggerConditionActions[creature_dies] missing")
			}
		})
	}
}

// TestClassifyTrigger_ParserCoverageR60_LongTail pins the era-4 long-tail
// slugs the parser emits for D&D / LOTR / Saga mechanics. Each must
// route to an existing scaffold whose primed world matches.
func TestClassifyTrigger_ParserCoverageR60_LongTail(t *testing.T) {
	cases := map[string]string{
		"specialize_from_zone":     "specialize_creature",
		"ring_tempts_you":          "when_you_do",
		"train":                    "when_you_do",
		"modified_creature_event":  "counters_put_on_self",
		"face_down_creature_event": "turned_face_up",
		"compound_tribe_enter":     "tribe_you_control_etb",
		"it_state_change":          "becomes_tapped_trigger",
	}
	for ev, wantSlug := range cases {
		ev, wantSlug := ev, wantSlug
		t.Run(ev, func(t *testing.T) {
			got := classifyTrigger(&gameast.Trigger{Event: ev})
			if got != wantSlug {
				t.Errorf("classifyTrigger(%q): want %q, got %q", ev, wantSlug, got)
			}
			if _, ok := triggerConditionActions[wantSlug]; !ok {
				t.Errorf("triggerConditionActions[%q] missing — routing slug is unregistered", wantSlug)
			}
		})
	}
}

// TestDetectConditionScaffold_ParserCoverageR60_NamedCounterThreshold pins
// the long-tail named-counter recognition. Each oracle phrase has a
// distinct printed counter type (release / dread / wreck / luck /
// arrowhead / echo / bounty / rad / phyresis); without these names in
// the recognizer the scaffold would fall back to the +1/+1 default and
// per_card handlers that read cs.subtype would receive the wrong name.
func TestDetectConditionScaffold_ParserCoverageR60_NamedCounterThreshold(t *testing.T) {
	cases := []struct {
		text    string
		subtype string
	}{
		{"it has thirteen or more release counters on it", "release"},
		{"there are three or more dread counters on it", "dread"},
		{"this enchantment has one or more wreck counters on it", "wreck"},
		{"this enchantment has ten or more luck counters on it", "luck"},
		{"it has three or more echo counters on it", "echo"},
		{"it has four or more bounty counters on it", "bounty"},
		{"it has six or more rad counters on it", "rad"},
		{"it has four or more phyresis counters on it", "phyresis"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.subtype, func(t *testing.T) {
			cond := &gameast.Condition{Kind: "if", Args: []any{tc.text}}
			got := detectConditionScaffold(cond)
			if got.kind != condScaffoldCountersOnSelfGE {
				t.Fatalf("kind: want condScaffoldCountersOnSelfGE, got %v (text=%q)", got.kind, tc.text)
			}
			if !strings.EqualFold(got.subtype, tc.subtype) {
				t.Errorf("subtype for %q: want %q, got %q", tc.text, tc.subtype, got.subtype)
			}
		})
	}
}

// TestDetectConditionScaffold_ParserCoverageR60_NamedCounter_Conditional
// confirms the same recognition works when the parser tags the kind as
// `conditional` (the as-long-as packaging) rather than the bare `if`.
// Both shapes go through the same raw-text matcher and must produce the
// same scaffold + named subtype.
func TestDetectConditionScaffold_ParserCoverageR60_NamedCounter_Conditional(t *testing.T) {
	cond := &gameast.Condition{
		Kind: "conditional",
		Args: []any{"as long as it has three or more dread counters on it, this creature has menace"},
	}
	got := detectConditionScaffold(cond)
	if got.kind != condScaffoldCountersOnSelfGE {
		t.Fatalf("kind: want condScaffoldCountersOnSelfGE, got %v", got.kind)
	}
	if got.subtype != "dread" {
		t.Errorf("subtype: want dread, got %q", got.subtype)
	}
}
