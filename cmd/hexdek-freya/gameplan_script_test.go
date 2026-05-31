package main

import (
	"strings"
	"testing"
)

// TestBuildGameplanScript_ArchetypeCoverage drives buildGameplanScript
// against five archetype-distinct DeckProfiles and asserts each gets
// a populated script with the right archetype's flavor signals.
// Combo / Aggro / Reanimator / Voltron / Control are the canonical
// shapes a deckbuilder picks between — if the templates regress (a
// renamed archetype, a deleted entry), this test surfaces it.
func TestBuildGameplanScript_ArchetypeCoverage(t *testing.T) {
	cases := []struct {
		archetype string
		commander string
		// substrings the script must contain to prove the right
		// per-archetype template fired. Each case picks 2-3
		// archetype-distinctive phrases the template uses.
		mustContain []string
	}{
		{
			archetype: "Combo",
			commander: "Kraum, Ludevic's Opus",
			mustContain: []string{
				"Cast Kraum, Ludevic's Opus", // {commander} substitution
				"Assemble the win",
				"countermagic",
				"Combo piece exiled",
			},
		},
		{
			archetype: "Aggro",
			commander: "Krenko, Mob Boss",
			mustContain: []string{
				"1-drop creature",
				"Two-creature turn",
				"21 commander damage",
				"Boardwiped on turn 4",
			},
		},
		{
			archetype: "Reanimator",
			commander: "Meren of Clan Nel Toth",
			mustContain: []string{
				"self-mill",
				"Reanimate spell",
				"Graveyard hate",
				"Reanimate target exiled",
			},
		},
		{
			archetype: "Voltron",
			commander: "Uril, the Miststalker",
			mustContain: []string{
				"Cast Uril, the Miststalker", // {commander} substitution
				"hexproof/shroud",
				"21 commander damage",
				"Equipment / aura wiped",
			},
		},
		{
			archetype: "Control",
			commander: "Yshtola, Night's Blessed",
			mustContain: []string{
				"Cantrip",
				"Sweeper",
				"counterspell",
				"Mana-flooded",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.archetype, func(t *testing.T) {
			dp := &DeckProfile{
				PrimaryArchetype: c.archetype,
				PrimaryWinLine:   "Test Finisher",
			}
			report := &FreyaReport{Commander: c.commander}

			script := buildGameplanScript(dp, report)
			if script == nil {
				t.Fatalf("%s: buildGameplanScript returned nil", c.archetype)
			}
			if script.Archetype != c.archetype {
				t.Errorf("Archetype = %q, want %q", script.Archetype, c.archetype)
			}
			if script.Commander != c.commander {
				t.Errorf("Commander = %q, want %q", script.Commander, c.commander)
			}
			if len(script.TurnByTurn) < 4 {
				t.Errorf("%s: TurnByTurn = %d entries, want >= 4", c.archetype, len(script.TurnByTurn))
			}

			// Collapse the whole script into a single string for
			// substring assertions — the template can shift turn 3
			// vs. turn 4 between revisions without breaking the test.
			joined := flattenScript(script)
			for _, want := range c.mustContain {
				if !strings.Contains(joined, want) {
					t.Errorf("%s: script missing %q\n---\n%s", c.archetype, want, joined)
				}
			}
		})
	}
}

// TestBuildGameplanScript_UnknownArchetypeFallsBackToMidrange pins
// the defensive default. An unknown archetype string must NOT return
// nil — the deck still wants a play guide; we render the Midrange
// template as a sensible default.
func TestBuildGameplanScript_UnknownArchetypeFallsBackToMidrange(t *testing.T) {
	dp := &DeckProfile{
		PrimaryArchetype: "ThisIsNotARealArchetype",
	}
	report := &FreyaReport{Commander: "Test Cmdr"}

	script := buildGameplanScript(dp, report)
	if script == nil {
		t.Fatal("expected fallback script, got nil")
	}
	if script.Archetype != "ThisIsNotARealArchetype" {
		t.Errorf("Archetype field should preserve caller's value, got %q", script.Archetype)
	}
	if len(script.TurnByTurn) == 0 {
		t.Error("fallback should still have a turn-by-turn template")
	}
	// Midrange's distinctive line: T5 says "Pressure opponents".
	joined := flattenScript(script)
	if !strings.Contains(joined, "Pressure opponents") {
		t.Errorf("expected Midrange-fallback content; got:\n%s", joined)
	}
}

// TestBuildGameplanScript_NilDeckProfile pins the defensive entry —
// nil DeckProfile or empty PrimaryArchetype must return nil rather
// than panicking. Callers (the JSON / text renderers) already
// nil-check before rendering, so signaling absence via nil is the
// contract.
func TestBuildGameplanScript_NilDeckProfile(t *testing.T) {
	if got := buildGameplanScript(nil, nil); got != nil {
		t.Errorf("nil DeckProfile → expected nil script, got %+v", got)
	}
	if got := buildGameplanScript(&DeckProfile{PrimaryArchetype: ""}, nil); got != nil {
		t.Errorf("empty PrimaryArchetype → expected nil script, got %+v", got)
	}
}

// TestSubstituteHoles_FillsBoth verifies {commander} and {finisher}
// both interpolate when both are non-empty. Empty replacements leave
// the literal token alone (better than rendering "Cast .").
func TestSubstituteHoles_FillsBoth(t *testing.T) {
	cases := []struct {
		in, commander, finisher, want string
	}{
		{"Cast {commander}", "Atraxa", "", "Cast Atraxa"},
		{"Win via {finisher}", "", "Thoracle line", "Win via Thoracle line"},
		{"{commander} + {finisher}", "Krenko", "go-wide", "Krenko + go-wide"},
		{"Cast {commander}", "", "Thoracle", "Cast {commander}"}, // literal kept
		{"no holes here", "Krenko", "go-wide", "no holes here"},
	}
	for _, c := range cases {
		got := substituteHoles(c.in, c.commander, c.finisher)
		if got != c.want {
			t.Errorf("substituteHoles(%q, %q, %q) = %q, want %q",
				c.in, c.commander, c.finisher, got, c.want)
		}
	}
}

// TestRenderGameplanScript_TextLayout pins the text-section format.
// Every section (turn-by-turn, branching, degradation) must render
// with its expected header so a downstream consumer can grep for it.
func TestRenderGameplanScript_TextLayout(t *testing.T) {
	script := buildGameplanScript(&DeckProfile{
		PrimaryArchetype: "Combo",
		PrimaryWinLine:   "Thoracle + Consultation",
	}, &FreyaReport{Commander: "Tymna the Weaver"})

	var buf strings.Builder
	renderGameplanScript(func(s string) { buf.WriteString(s) }, script)
	out := buf.String()

	for _, want := range []string{
		"Turn-by-turn ideal sequence:",
		"T1:",
		"T3:",
		"T5:",
		"Branching decisions:",
		"IF ",
		"THEN ",
		"Graceful degradation:",
		"WHEN ",
		"Tymna the Weaver", // commander interpolation in T3 action
	} {
		if !strings.Contains(out, want) {
			t.Errorf("render missing %q\n---\n%s", want, out)
		}
	}
}

// TestRenderGameplanScript_NilSilent pins the defensive entry — nil
// script must NOT write anything. The text renderer is supposed to
// be callable unconditionally even when the deck has no archetype.
func TestRenderGameplanScript_NilSilent(t *testing.T) {
	var buf strings.Builder
	renderGameplanScript(func(s string) { buf.WriteString(s) }, nil)
	if buf.Len() != 0 {
		t.Errorf("nil script should write nothing; got %q", buf.String())
	}
}

func flattenScript(s *GameplanScript) string {
	var b strings.Builder
	for _, t := range s.TurnByTurn {
		b.WriteString(t.Action + " " + t.Note + "\n")
	}
	for _, d := range s.DecisionPoints {
		b.WriteString(d.Trigger + " " + d.Action + "\n")
	}
	for _, d := range s.DegradationPaths {
		b.WriteString(d.Setback + " " + d.Recover + "\n")
	}
	return b.String()
}
