package per_card

import (
	"reflect"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Regression tests for the dev/muninn-handlers-201-220 wave: dispatch-layer
// fallback so existing handlers fire on cascade/copy/token-renamed variants
// (cascade.go renames StackItem.Card.Name to "X (cascade)", Urza copies
// become "X (Urza copy)", Miirym tokens become "X (Miirym Token)", etc.).
// Before the fix, those variant names silently bypassed every fire*
// dispatcher except the partial " // " fallback in fireETB/fireTrigger.

func TestLookupCandidates_StripsTrailingParenthetical(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"Necromancy (cascade)", []string{"necromancy (cascade)", "necromancy"}},
		{"Phoenix Fleet Airship (Urza copy)", []string{"phoenix fleet airship (urza copy)", "phoenix fleet airship"}},
		{"Tiamat (Miirym Token)", []string{"tiamat (miirym token)", "tiamat"}},
		{"Claim Jumper (Restore-Relic token)", []string{"claim jumper (restorerelic token)", "claim jumper"}},
		// Stacked rename: iteratively strip every trailing parenthetical so
		// double-renamed sources (e.g. an Urza copy of an Urza copy) reach
		// the base handler in a single dispatch pass.
		{"Crown of Gondor (Urza copy) (Urza copy)", []string{"crown of gondor (urza copy) (urza copy)", "crown of gondor (urza copy)", "crown of gondor"}},
		// Pure base name yields a single key.
		{"Lightning Bolt", []string{"lightning bolt"}},
		// DFC alone keeps the historical " // " front-face fallback.
		// normalizeName preserves "//" (only punctuation in []rune is stripped).
		{"Curious Homunculus // Voracious Reader", []string{"curious homunculus // voracious reader", "curious homunculus"}},
		// Cascade-fired DFC: strip paren, then split DFC.
		{"Eccentric Pestfinder // Turn Stones (cascade)", []string{"eccentric pestfinder // turn stones (cascade)", "eccentric pestfinder // turn stones", "eccentric pestfinder"}},
	}
	for _, tc := range cases {
		got := lookupCandidates(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("lookupCandidates(%q):\n  got  %q\n  want %q", tc.in, got, tc.want)
		}
	}
}

// Regression for dev/lookup-candidates-token-paren-r60: paren-less " Token"
// suffix on direct-mint token names was bypassing every fire* dispatcher
// (Muninn classification doc flagged 7 affected snippets). The fallback
// chain must strip a trailing " token" so the base-name handler fires.
func TestLookupCandidates_StripsTrailingTokenSuffix(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"Claim Jumper Token", []string{"claim jumper token", "claim jumper"}},
		{"Sand Scout Token", []string{"sand scout token", "sand scout"}},
		{"Gau Feral Youth Token", []string{"gau feral youth token", "gau feral youth"}},
		{"Phyrexian Myr Token", []string{"phyrexian myr token", "phyrexian myr"}},
		{"Kodama of the East Tree Token", []string{"kodama of the east tree token", "kodama of the east tree"}},
		{"Wistfulness Token", []string{"wistfulness token", "wistfulness"}},
		{"Myr Token", []string{"myr token", "myr"}},
		{"Rankle and Torbran Token", []string{"rankle and torbran token", "rankle and torbran"}},
		// Token suffix combined with a trailing paren rider — the paren-
		// strip pass handles the rider first, then the token-suffix pass
		// strips " token" off both the original and the paren-stripped
		// form. Result includes both the rendered name AND the bare base.
		{"Myr Token (cascade)", []string{"myr token (cascade)", "myr token", "myr"}},
	}
	for _, tc := range cases {
		got := lookupCandidates(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("lookupCandidates(%q):\n  got  %q\n  want %q", tc.in, got, tc.want)
		}
	}
}

// Regression: stacked-rename strip must terminate cleanly when the inner
// layer is itself a base name (no paren). The base must appear, and the
// loop must not infinite-loop on degenerate inputs.
func TestLookupCandidates_StackedParenStripTerminates(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		// Three layers — Urza copying an Urza copy of an Urza copy.
		{"Bear (Urza copy) (Urza copy) (Urza copy)", []string{
			"bear (urza copy) (urza copy) (urza copy)",
			"bear (urza copy) (urza copy)",
			"bear (urza copy)",
			"bear",
		}},
		// Degenerate "(...)" with nothing in front shouldn't grow output.
		{"(Urza copy)", []string{"(urza copy)"}},
	}
	for _, tc := range cases {
		got := lookupCandidates(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("lookupCandidates(%q):\n  got  %q\n  want %q", tc.in, got, tc.want)
		}
	}
}

// Cascade-renamed ETB must reach the front-face handler.
func TestFireETB_DispatchesThroughCascadeRename(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	fired := 0
	Global().OnETB("Wave201CascadeETB", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
		fired++
	})

	gs := newGame(t, 2)
	perm := addPerm(gs, 0, "Wave201CascadeETB (cascade)", "creature")

	gameengine.InvokeETBHook(gs, perm)

	if fired != 1 {
		t.Errorf("expected cascade-renamed ETB to dispatch via fallback, fired=%d", fired)
	}
}

// Urza copy must reach the front-face handler.
func TestFireETB_DispatchesThroughUrzaCopyRename(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	fired := 0
	Global().OnETB("Wave201UrzaCopy", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
		fired++
	})

	gs := newGame(t, 2)
	perm := addPerm(gs, 0, "Wave201UrzaCopy (Urza copy)", "artifact")

	gameengine.InvokeETBHook(gs, perm)

	if fired != 1 {
		t.Errorf("expected Urza-copy ETB to dispatch via fallback, fired=%d", fired)
	}
}

// Cascade-renamed Trigger must reach the front-face handler.
func TestFireTrigger_DispatchesThroughCascadeRename(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	fired := 0
	Global().OnTrigger("Wave201CascadeTrigger", "end_step", func(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
		fired++
	})

	gs := newGame(t, 2)
	addPerm(gs, 0, "Wave201CascadeTrigger (cascade)", "enchantment")

	gameengine.FireCardTrigger(gs, "end_step", map[string]interface{}{
		"controller_seat": 0,
	})

	if fired != 1 {
		t.Errorf("expected cascade-renamed trigger to dispatch via fallback, fired=%d", fired)
	}
}

// Pure base-name lookups still hit the direct key (no fallback regression).
func TestFireETB_DirectNameStillDispatches(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	fired := 0
	Global().OnETB("Wave201Direct", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
		fired++
	})

	gs := newGame(t, 2)
	perm := addPerm(gs, 0, "Wave201Direct", "creature")

	gameengine.InvokeETBHook(gs, perm)

	if fired != 1 {
		t.Errorf("expected direct-name ETB to dispatch, fired=%d", fired)
	}
}
