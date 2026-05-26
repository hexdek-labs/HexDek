package main

import (
	"strings"
	"testing"
)

// oracletext_nlp_r60_test.go — regressions for the r60 deeper-NLP
// extensions in oracletext.go. Four new helpers + one verification:
//
//   - SplitUnless        — separate "primary effect" from " unless …"
//   - IsSoftRemoval      — combined: removal verb + unless-tax = soft
//   - SplitTriggerEffect — structural Whenever/When/At split
//   - HasDependentTrigger — "when that creature dies" nested-trigger
//                           detection
//   - modal coverage     — verify the existing modeHeaderRE handles
//                          "Choose one", "Choose two", "Choose three",
//                          "Choose four", "Choose one or more", and
//                          "Choose any number"

// -----------------------------------------------------------------------------
// SplitUnless
// -----------------------------------------------------------------------------

func TestSplitUnless_RealCards(t *testing.T) {
	cases := []struct {
		name    string
		clause  string
		primary string
		cond    string
	}{
		{
			name:    "Mana Leak — counter unless pay {3}",
			clause:  "counter target spell unless its controller pays {3}",
			primary: "counter target spell",
			cond:    "its controller pays {3}",
		},
		{
			name:    "Spell Pierce — counter noncreature unless pay {2}",
			clause:  "counter target noncreature spell unless its controller pays {2}",
			primary: "counter target noncreature spell",
			cond:    "its controller pays {2}",
		},
		{
			name:    "Diabolic Edict-tax — sacrifice a creature unless pay 3 life",
			clause:  "target player sacrifices a creature unless they pay 3 life",
			primary: "target player sacrifices a creature",
			cond:    "they pay 3 life",
		},
		{
			name:    "Doom Blade — no unless, returns clause unchanged",
			clause:  "destroy target nonblack creature",
			primary: "destroy target nonblack creature",
			cond:    "",
		},
		{
			name:    "Counterspell — no unless",
			clause:  "counter target spell",
			primary: "counter target spell",
			cond:    "",
		},
		{
			name:    "Trailing period stripped from condition",
			clause:  "destroy target creature unless its controller pays {3}.",
			primary: "destroy target creature",
			cond:    "its controller pays {3}",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gotP, gotC := SplitUnless(tc.clause)
			if gotP != tc.primary {
				t.Errorf("primary: got %q, want %q", gotP, tc.primary)
			}
			if gotC != tc.cond {
				t.Errorf("condition: got %q, want %q", gotC, tc.cond)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// IsSoftRemoval
// -----------------------------------------------------------------------------

func TestIsSoftRemoval_RealCards(t *testing.T) {
	cases := []struct {
		name   string
		clause string
		want   bool
	}{
		{"Mana Leak (soft counter)",
			"counter target spell unless its controller pays {3}", true},
		{"Spell Pierce (soft counter)",
			"counter target noncreature spell unless its controller pays {2}", true},
		{"Tax-edict (soft sac via opponent)",
			"destroy target creature unless its controller pays {3}", true},
		{"Counterspell (hard counter — no unless)",
			"counter target spell", false},
		{"Doom Blade (hard removal — no unless)",
			"destroy target nonblack creature", false},
		{"Lightning Bolt (no removal verb)",
			"this spell deals 3 damage to any target", false},
		{
			name: "Draw unless — wrong primary verb",
			// Hypothetical "draw a card unless an opponent pays {1}" —
			// not a removal effect; should NOT classify as soft removal
			// even though there's an unless-tax.
			clause: "draw a card unless an opponent pays {1}",
			want:   false,
		},
		{
			name: "Destroy unless it's a token — irrelevant condition",
			// "Destroy target creature unless it's a token" — the unless
			// branch is a type-check, not a pay-around. Should NOT
			// classify as soft removal.
			clause: "destroy target creature unless it's a token",
			want:   false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := IsSoftRemoval(tc.clause)
			if got != tc.want {
				t.Errorf("IsSoftRemoval(%q) = %v, want %v", tc.clause, got, tc.want)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// SplitTriggerEffect
// -----------------------------------------------------------------------------

func TestSplitTriggerEffect_RealCards(t *testing.T) {
	cases := []struct {
		name      string
		clause    string
		wantKind  string
		wantTrig  string
		wantIf    string
		wantEff   string
	}{
		{
			name:     "Reassembling Skeleton-style — Whenever a creature dies, draw a card",
			clause:   "whenever a creature you control dies, draw a card.",
			wantKind: "whenever",
			wantTrig: "a creature you control dies",
			wantIf:   "",
			wantEff:  "draw a card.",
		},
		{
			name:     "Intervening-if — Whenever X, if Y, Z",
			clause:   "whenever this creature attacks, if you control a goblin, deal 2 damage to any target.",
			wantKind: "whenever",
			wantTrig: "this creature attacks",
			wantIf:   "you control a goblin",
			wantEff:  "deal 2 damage to any target.",
		},
		{
			name:     "At-style — at the beginning of upkeep",
			clause:   "at the beginning of your upkeep, draw a card.",
			wantKind: "at",
			wantTrig: "the beginning of your upkeep",
			wantIf:   "",
			wantEff:  "draw a card.",
		},
		{
			name:     "When-style — ETB",
			clause:   "when this creature enters, search your library for a creature card.",
			wantKind: "when",
			wantTrig: "this creature enters",
			wantIf:   "",
			wantEff:  "search your library for a creature card.",
		},
		{
			name:     "Non-trigger clause returns empty",
			clause:   "destroy target creature.",
			wantKind: "",
			wantTrig: "",
			wantIf:   "",
			wantEff:  "",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			kind, trig, ifc, eff := SplitTriggerEffect(tc.clause)
			if kind != tc.wantKind {
				t.Errorf("kind: got %q, want %q", kind, tc.wantKind)
			}
			if trig != tc.wantTrig {
				t.Errorf("trigger: got %q, want %q", trig, tc.wantTrig)
			}
			if ifc != tc.wantIf {
				t.Errorf("ifCond: got %q, want %q", ifc, tc.wantIf)
			}
			if eff != tc.wantEff {
				t.Errorf("effect: got %q, want %q", eff, tc.wantEff)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// HasDependentTrigger
// -----------------------------------------------------------------------------

func TestHasDependentTrigger_RealCards(t *testing.T) {
	cases := []struct {
		name   string
		effect string
		want   bool
	}{
		{
			name:   "Etrata, the Silencer — when that creature is no longer exiled",
			effect: "exile target creature that player controls. when that creature dies, you draw a card.",
			want:   true,
		},
		{
			name: "Cabal Therapist — names a card; whenever that player discards",
			effect: "name a nonland card. target player reveals their hand. whenever that " +
				"player discards a card with that name, you gain 1 life.",
			want: true,
		},
		{
			name:   "Doom Blade — no dependent trigger",
			effect: "destroy target nonblack creature.",
			want:   false,
		},
		{
			name:   "Wrath of God — global wipe, no dependent",
			effect: "destroy all creatures. they can't be regenerated.",
			want:   false,
		},
		{
			name:   "Whenever (top-level, not nested) — not dependent",
			effect: "whenever you cast a noncreature spell, this creature gets +1/+0 until end of turn.",
			want:   false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := HasDependentTrigger(tc.effect)
			if got != tc.want {
				t.Errorf("HasDependentTrigger = %v, want %v", got, tc.want)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Modal coverage — verify the existing modeHeaderRE handles all real
// modal-card flavors. (Verification rather than extension — Magic's
// modal vocabulary is bounded: one/two/three/four/one or more/any number.)
// -----------------------------------------------------------------------------

func TestSplitModes_AllModalFlavors(t *testing.T) {
	cases := []struct {
		name string
		ot   string
		nModes int
	}{
		{"Choose one", "Choose one — • A. • B. • C.", 3},
		{"Choose two", "Choose two — • A. • B. • C. • D.", 4},
		{"Choose three", "Choose three — • A. • B. • C. • D.", 4},
		{"Choose four", "Choose four — • A. • B. • C. • D. • E.", 5},
		{"Choose one or more", "Choose one or more — • A. • B. • C.", 3},
		{"Choose any number", "Choose any number — • A. • B.", 2},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			modes := splitModes(tc.ot)
			// modes[0] is the always-empty preamble; modes[1:] are bullets.
			gotN := len(modes) - 1
			if gotN != tc.nModes {
				t.Errorf("splitModes(%q): want %d modes, got %d (%v)",
					tc.name, tc.nModes, gotN, modes)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Integration: SplitUnless × SplitTriggerEffect
// -----------------------------------------------------------------------------

// A real card like Mausoleum Wanderer combines a trigger with an unless
// clause: "Whenever you cast a Spirit spell, counter target spell unless
// its controller pays {X}". The two helpers should compose cleanly.
func TestNLPIntegration_TriggerWithUnlessEffect(t *testing.T) {
	clause := "whenever you cast a spirit spell, counter target spell unless its controller pays {x}."
	kind, trig, _, eff := SplitTriggerEffect(clause)
	if kind != "whenever" {
		t.Fatalf("trigger kind: got %q, want whenever", kind)
	}
	if !strings.Contains(trig, "cast a spirit spell") {
		t.Errorf("trigger should contain 'cast a spirit spell'; got %q", trig)
	}
	// Now run SplitUnless on the effect.
	primary, cond := SplitUnless(eff)
	if !strings.Contains(primary, "counter target spell") {
		t.Errorf("primary effect should contain 'counter target spell'; got %q", primary)
	}
	if !strings.Contains(cond, "pays {x}") {
		t.Errorf("unless-condition should contain 'pays {x}'; got %q", cond)
	}
	// And IsSoftRemoval should fire on the effect — Mausoleum Wanderer
	// triggers a soft counter.
	if !IsSoftRemoval(eff) {
		t.Errorf("trigger effect is a soft counter (Mausoleum Wanderer); IsSoftRemoval should fire")
	}
}
