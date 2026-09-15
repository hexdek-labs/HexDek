package hexapi

import (
	"encoding/json"
	"testing"
)

// annotateAnalysisFreshness decides what the deck page tells a user
// about whether their analysis is current. Getting it wrong in either
// direction is a user-visible lie: a false "out of date" nags people
// into pointless re-runs, a false "current" is the silent-staleness
// bug this change set exists to remove.
//
// CurrentFreyaVersion() is resolved once per process by probing the
// real binary, which test binaries don't ship — so in this package it
// returns "". That makes the no-version-known path the default here,
// and the tests are written to assert against the actual resolved
// value rather than assuming either outcome.

func decodeAnnotated(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("annotated output is not valid JSON: %v\n%s", err, raw)
	}
	return doc
}

func TestAnnotateAnalysisFreshness_PreservesExistingFields(t *testing.T) {
	// The annotation must be purely additive. Every consumer of this
	// endpoint — the deck page, the export modal, DeckCompare — reads
	// the original strategy.json keys, and a decode/re-encode round
	// trip is exactly where a field quietly goes missing.
	in := []byte(`{"archetype":"tokens","bracket":3,"card_roles":{"Sol Ring":"ramp"},` +
		`"cuttable_cards":["Orbs of Warding"],"keepable_hand_pct":63.5,"win_lines":[]}`)

	got := decodeAnnotated(t, annotateAnalysisFreshness(in))

	if got["archetype"] != "tokens" {
		t.Errorf("archetype = %v, want tokens", got["archetype"])
	}
	if got["bracket"] != float64(3) {
		t.Errorf("bracket = %v, want 3", got["bracket"])
	}
	if got["keepable_hand_pct"] != 63.5 {
		t.Errorf("keepable_hand_pct = %v, want 63.5", got["keepable_hand_pct"])
	}
	roles, ok := got["card_roles"].(map[string]any)
	if !ok || roles["Sol Ring"] != "ramp" {
		t.Errorf("card_roles lost or altered: %v", got["card_roles"])
	}
	if _, ok := got["win_lines"]; !ok {
		t.Error("win_lines key dropped — an empty array must survive the round trip")
	}
}

func TestAnnotateAnalysisFreshness_MalformedJSONPassesThrough(t *testing.T) {
	// A freshness annotation is never worth failing a read the user
	// would otherwise have gotten. A truncated or hand-edited
	// strategy.json should still reach the client exactly as stored.
	for _, raw := range [][]byte{
		[]byte(`{"archetype":"tokens"`), // truncated
		[]byte(`not json at all`),
		[]byte(``),
		[]byte(`null`), // decodes cleanly to a nil map
		[]byte(`[1,2,3]`),
	} {
		got := annotateAnalysisFreshness(raw)
		if string(got) != string(raw) {
			t.Errorf("input %q was modified to %q; malformed input must pass through untouched",
				raw, got)
		}
	}
}

func TestAnnotateFreshnessWith_NoClaimWhenCurrentVersionUnknown(t *testing.T) {
	// If we can't determine what Freya is running, we cannot know
	// whether a stored analysis is stale. Saying nothing is the only
	// honest option — asserting stale would nag every user on a server
	// whose freya binary is merely missing or mid-swap.
	in := []byte(`{"archetype":"tokens","freya_version":"r1-ancient"}`)
	out := annotateFreshnessWith(in, "")

	if string(out) != string(in) {
		t.Errorf("bytes were modified with no current version known:\n got %s\nwant %s", out, in)
	}
	got := decodeAnnotated(t, out)
	if _, present := got["stale"]; present {
		t.Errorf("stale was set to %v despite the current version being unknown", got["stale"])
	}
	if _, present := got["current_freya_version"]; present {
		t.Error("current_freya_version was set despite the probe returning nothing")
	}
}

func TestAnnotateFreshnessWith_StaleDecision(t *testing.T) {
	const current = "r60.2-a1b2c3d4e5f6"

	cases := []struct {
		name      string
		stored    string // "" means the key is absent entirely
		wantStale bool
		why       string
	}{
		{
			name: "same version is current", stored: current, wantStale: false,
			why: "the analysis was produced by the binary that is running; " +
				"nagging here would train users to ignore the banner",
		},
		{
			name: "older version is stale", stored: "r60.1-ffffffffffff", wantStale: true,
			why: "different build, therefore potentially different conclusions",
		},
		{
			name: "same base but different revision is stale", stored: "r60.2-ffffffffffff", wantStale: true,
			why: "the base token alone is NOT the identity — this is the whole " +
				"point of deriving the version from the build revision",
		},
		{
			name: "absent version is stale", stored: "", wantStale: true,
			why: "every run since 2026-09-15 stamps the field, so a file " +
				"without it was necessarily written by an earlier build — " +
				"a deduction, not a guess",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := map[string]any{"archetype": "tokens"}
			if tc.stored != "" {
				doc["freya_version"] = tc.stored
			}
			in, err := json.Marshal(doc)
			if err != nil {
				t.Fatalf("marshal fixture: %v", err)
			}

			got := decodeAnnotated(t, annotateFreshnessWith(in, current))

			if got["stale"] != tc.wantStale {
				t.Errorf("stale = %v, want %v\n  %s", got["stale"], tc.wantStale, tc.why)
			}
			if got["current_freya_version"] != current {
				t.Errorf("current_freya_version = %v, want %q — the frontend needs it "+
					"to name the running version in the banner",
					got["current_freya_version"], current)
			}
			if got["archetype"] != "tokens" {
				t.Errorf("archetype lost: %v", got["archetype"])
			}
		})
	}
}

func TestAnnotateFreshnessWith_NonStringStoredVersion(t *testing.T) {
	// A hand-edited or corrupt file could carry a non-string in that
	// key. The type assertion must not panic, and the result must be
	// "stale" rather than silently "current".
	in := []byte(`{"freya_version":42,"archetype":"tokens"}`)
	got := decodeAnnotated(t, annotateFreshnessWith(in, "r60.2-abc"))
	if got["stale"] != true {
		t.Errorf("stale = %v, want true for an unreadable version stamp", got["stale"])
	}
}

// TestCurrentFreyaVersion_StableAcrossCalls pins the caching contract.
// The value is consulted on every analysis read; if the sync.Once were
// dropped, each page view would fork a subprocess to learn a constant.
func TestCurrentFreyaVersion_StableAcrossCalls(t *testing.T) {
	first := CurrentFreyaVersion()
	for i := 0; i < 3; i++ {
		if got := CurrentFreyaVersion(); got != first {
			t.Fatalf("CurrentFreyaVersion() not stable: %q then %q", first, got)
		}
	}
}
