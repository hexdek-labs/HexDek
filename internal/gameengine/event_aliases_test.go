package gameengine

import "testing"

// TestEventAlias_ETBFamily_RoutesToPermanentETB pins the three friendly
// per_card-only ETB aliases (creature_enters_battlefield,
// land_entered_battlefield, permanent_entered_battlefield) to the canonical
// engine event "permanent_etb" — fired via FireCardTrigger in
// etb_dispatch.go:94 and stack.go:1535. Pre-r60 these aliased to "etb" while
// the engine fired "permanent_etb", so per_card OnTrigger handlers using the
// friendly names silently never fired (Aura Shards / Field of the Dead /
// Sokka and Suki / Selvala). Per audit-percard-registry-r60.md §1.
func TestEventAlias_ETBFamily_RoutesToPermanentETB(t *testing.T) {
	cases := []struct {
		alias string
	}{
		{"creature_enters_battlefield"},
		{"land_entered_battlefield"},
		{"permanent_entered_battlefield"},
	}
	for _, tc := range cases {
		got := NormalizeEventSingle(tc.alias)
		if got != "permanent_etb" {
			t.Errorf("NormalizeEventSingle(%q) = %q, want %q", tc.alias, got, "permanent_etb")
		}
		if !EventEquals(tc.alias, "permanent_etb") {
			t.Errorf("EventEquals(%q, %q) = false, want true", tc.alias, "permanent_etb")
		}
		// Pre-fix counterfactual: these used to map to "etb". The new mapping
		// must NOT also match "etb" — otherwise observer ETB matching in
		// observer_triggers.go would double-fire on per_card-only events.
		if EventEquals(tc.alias, "etb") {
			t.Errorf("EventEquals(%q, %q) = true; per_card-only ETB aliases must NOT route to the AST observer 'etb' canonical", tc.alias, "etb")
		}
	}
}

// TestEventAlias_ASTObserverETB_StillMatches confirms the AST observer ETB
// aliases (parser-emitted Triggered.Event strings) still normalize to "etb"
// so observer_triggers.go's EventEquals(trig.Trigger.Event, "etb") match
// path keeps working. The alias-table fix above only repointed the THREE
// per_card-only names; the parser-emitted family must stay untouched.
func TestEventAlias_ASTObserverETB_StillMatches(t *testing.T) {
	parserEmitted := []string{
		"another_typed_enters",
		"tribe_you_control_etb",
		"ally_etb",
		"another_creature_enters",
		"another_typed_etb",
		"creature_etb_any",
		"artifact_etb",
		"land_etb_any",
		"permanent_enters",
	}
	for _, ev := range parserEmitted {
		if !EventEquals(ev, "etb") {
			t.Errorf("AST observer event %q should normalize to canonical %q, EventEquals = false", ev, "etb")
		}
	}
}
