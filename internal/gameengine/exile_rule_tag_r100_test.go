package gameengine

import "testing"

// r100 — the resolve event's "rule" field for an exile-on-resolve used
// to be the literal "702.34a" for every item that reached the branch.
// That is flashback's rule, and it was emitted as event DATA on cards
// with no flashback: Force of Negation, Mindbreak Trap, Transcendent
// Dragon, Smirking Spelljacker, River Song's Diary, Kess, Maestros
// Ascendancy, Feather, Eldritch Evolution, and Emry's / Wrexial's
// grants, all of which exile because of their own printed text.
//
// Each number below was read from MagicCompRules-20260819.txt for this
// table rather than copied from the code being changed.
func TestExileRuleTagR100_PerSource(t *testing.T) {
	cases := []struct {
		name    string
		keyword string
		want    string
	}{
		{"flashback carries its own carveout", "flashback", "702.34a"},
		{"aftermath carries its own carveout", "aftermath", "702.127a"},
		{"jump-start carries its own carveout", "jump-start", "702.133a"},

		// A card's own printed "instead" — the per_card zone-cast
		// grants stamp their handler's name here, not a CR keyword.
		{"Emry's grant is card text", "emry_artifact_cast", crReplacementInstead},
		{"Wrexial's grant is card text", "wrexial_free_cast", crReplacementInstead},
		{"Mizzix's grant is card text", "mizzixs_mastery_grant", crReplacementInstead},
		{"no keyword at all (counter-and-exile)", "", crReplacementInstead},

		// Keywords that must NEVER reach the branch. If one does, the
		// honest answer is still the generic replacement rule — never
		// flashback's number, which is what made this wrong.
		{"escape has no carveout (r96)", "escape", crReplacementInstead},
		{"mayhem has no carveout (r99)", "mayhem", crReplacementInstead},
		{"omen has no carveout (r99)", "omen", crReplacementInstead},
	}
	for _, tc := range cases {
		item := &StackItem{CostMeta: map[string]interface{}{}}
		if tc.keyword != "" {
			item.CostMeta["zone_cast_keyword"] = tc.keyword
		}
		if got := exileOnResolveRule(item); got != tc.want {
			t.Errorf("%s: keyword %q -> rule %q, want %q", tc.name, tc.keyword, got, tc.want)
		}
	}
	// Nil-safety: the off-board path can be reached with a bare item.
	if got := exileOnResolveRule(nil); got != crReplacementInstead {
		t.Errorf("nil item -> %q, want %q", got, crReplacementInstead)
	}
	if got := exileOnResolveRule(&StackItem{}); got != crReplacementInstead {
		t.Errorf("item with no CostMeta -> %q, want %q", got, crReplacementInstead)
	}
}

// The off-board path must carry the same per-source tag, not just the
// resolve path — they are different code and r96's escape defect lived
// in exactly that gap.
func TestExileRuleTagR100_OffboardPathAgrees(t *testing.T) {
	for _, kw := range []string{"flashback", "aftermath", "jump-start", "emry_artifact_cast"} {
		item := &StackItem{CostMeta: map[string]interface{}{
			"zone_cast_keyword": kw,
			"exile_on_resolve":  true,
		}}
		zone, rule := postStackZoneForOffboard(item, "graveyard", "701.6a")
		if zone != "exile" {
			t.Errorf("%s: off-board zone = %q, want exile", kw, zone)
		}
		if want := exileOnResolveRule(item); rule != want {
			t.Errorf("%s: off-board rule = %q, want %q (same as the resolve path)", kw, rule, want)
		}
	}
	// A spell with no exile routing keeps the caller's default and is
	// untouched by the new tag — the over-suppression guard.
	plain := &StackItem{CostMeta: map[string]interface{}{}}
	if zone, rule := postStackZoneForOffboard(plain, "graveyard", "701.6a"); zone != "graveyard" || rule != "701.6a" {
		t.Errorf("plain item routed to (%q, %q), want (graveyard, 701.6a)", zone, rule)
	}
}
