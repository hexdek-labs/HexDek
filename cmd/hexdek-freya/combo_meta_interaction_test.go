package main

import (
	"bytes"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Combo-vs-meta interaction tests. Each fixture builds a minimal
// FreyaReport with combos in the categorical-win buckets plus the
// corresponding CardProfile entries (for type-line + IsRecursion +
// Consumes inspection) and a stubOracleWithText DB (re-used from
// archetype_new_r60_test.go) to drive the protection-text scan.
//
// Tests pin: per-combo vulnerability detection across all three axes,
// removal-cost classification (1 for any unprotected piece / 2 when
// all are protected / 0 for instant-only combos), the dominant-threat
// tiebreak, and the deck-level worst-case rollup.
// ---------------------------------------------------------------------------

func metaReportWith(profiles []CardProfile, trueInfs, det, gy [][]string) *FreyaReport {
	r := &FreyaReport{Profiles: profiles}
	for _, cards := range trueInfs {
		r.TrueInfinites = append(r.TrueInfinites, ComboResult{
			Cards: append([]string(nil), cards...), LoopType: "true_infinite",
		})
	}
	for _, cards := range det {
		r.Determined = append(r.Determined, ComboResult{
			Cards: append([]string(nil), cards...), LoopType: "determined",
		})
	}
	for _, cards := range gy {
		r.GraveyardLoops = append(r.GraveyardLoops, ComboResult{
			Cards: append([]string(nil), cards...), LoopType: "synergy",
		})
	}
	return r
}

func findVulnByLabel(t *testing.T, m *ComboMetaInteraction, fragment string) *ComboMetaVulnerability {
	t.Helper()
	for i := range m.PerCombo {
		if strings.Contains(m.PerCombo[i].Label, fragment) {
			return &m.PerCombo[i]
		}
	}
	t.Fatalf("no combo found containing %q in matrix:\n%+v", fragment, m.PerCombo)
	return nil
}

// TestMetaVuln_GraveyardCombo_RIPCritical: a combo flagged as a
// graveyard_loop (or with an IsRecursion piece) should surface RIP /
// Leyline / Bojuka Bog as hosers with the dominant-threat tagged
// "graveyard".
func TestMetaVuln_GraveyardCombo_RIPCritical(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Animate Dead", TypeLine: "Enchantment — Aura", IsRecursion: true, RecursionDest: "battlefield"},
		{Name: "Worldgorger Dragon", TypeLine: "Creature — Dragon"},
		{Name: "Sun Titan", TypeLine: "Creature — Giant"}, // graveyard enabler
	}
	report := metaReportWith(profiles, nil, nil, [][]string{{"Animate Dead", "Worldgorger Dragon", "Sun Titan"}})

	oracle := stubOracleWithText(map[string]string{
		"animate dead":         "Enchant creature card in a graveyard. Return enchanted creature card to the battlefield...",
		"worldgorger dragon":   "When Worldgorger Dragon enters, exile all other permanents you control. When this leaves, return the exiled cards...",
		"sun titan":            "Vigilance. Whenever Sun Titan attacks or enters, you may return target permanent card with mana value 3 or less from your graveyard...",
	})

	m := BuildComboMetaInteraction(report, oracle)
	if m == nil {
		t.Fatal("expected non-nil ComboMetaInteraction")
	}
	if len(m.PerCombo) != 1 {
		t.Fatalf("expected 1 combo, got %d", len(m.PerCombo))
	}
	v := m.PerCombo[0]
	if v.GraveyardScore != 3 {
		t.Errorf("GraveyardScore: got %d, want 3", v.GraveyardScore)
	}
	wantHosers := []string{"Rest in Peace", "Leyline of the Void", "Bojuka Bog"}
	for _, want := range wantHosers {
		found := false
		for _, h := range v.GraveyardHosers {
			if h == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("GraveyardHosers missing %q; got %v", want, v.GraveyardHosers)
		}
	}
	if v.DominantThreat != "graveyard" {
		t.Errorf("DominantThreat: got %q, want \"graveyard\"", v.DominantThreat)
	}
	if m.WorstGraveyardHoser == "" {
		t.Errorf("WorstGraveyardHoser should be set")
	}
}

// TestMetaVuln_StormCombo_RuleOfLawCritical: storm-shaped combos (3+
// pieces OR IsStormFinisher) trigger Rule of Law / Eidolon as
// critical stax hosers.
func TestMetaVuln_StormCombo_RuleOfLawCritical(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Aetherflux Reservoir", TypeLine: "Artifact", IsStormFinisher: true, IsManaPayoff: true, Consumes: []ResourceType{ResMana}},
		{Name: "Bonus Round", TypeLine: "Instant"},
		{Name: "Sol Ring", TypeLine: "Artifact", Produces: []ResourceType{ResMana}, Consumes: []ResourceType{ResMana}},
	}
	report := metaReportWith(profiles, nil,
		[][]string{{"Aetherflux Reservoir", "Bonus Round", "Sol Ring"}}, nil)

	oracle := stubOracleWithText(map[string]string{
		"aetherflux reservoir": "Whenever you cast a spell, you gain 1 life for each spell you've cast this turn. Pay 50 life: Aetherflux Reservoir deals 50 damage to any target.",
		"bonus round":          "Until end of turn, whenever a player casts an instant or sorcery spell, that player copies it.",
		"sol ring":             "{T}: Add {C}{C}.",
	})

	m := BuildComboMetaInteraction(report, oracle)
	if m == nil {
		t.Fatal("expected non-nil")
	}
	v := m.PerCombo[0]
	if v.StaxScore != 3 {
		t.Errorf("StaxScore: got %d, want 3", v.StaxScore)
	}
	foundRuleOfLaw := false
	for _, h := range v.StaxHosers {
		if h == "Rule of Law" {
			foundRuleOfLaw = true
		}
	}
	if !foundRuleOfLaw {
		t.Errorf("expected Rule of Law in StaxHosers, got %v", v.StaxHosers)
	}
	if m.WorstStaxHoser == "" {
		t.Errorf("WorstStaxHoser should be set")
	}
}

// TestMetaVuln_RemovalFragileCombo: combo with all unprotected
// permanent pieces — RemovalRequiredToBreak == 1, classified fragile.
func TestMetaVuln_RemovalFragileCombo(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Heliod, Sun-Crowned", TypeLine: "Legendary Enchantment Creature — God"},
		{Name: "Walking Ballista", TypeLine: "Artifact Creature — Construct"},
	}
	report := metaReportWith(profiles, [][]string{{"Heliod, Sun-Crowned", "Walking Ballista"}}, nil, nil)
	oracle := stubOracleWithText(map[string]string{
		"heliod, sun-crowned": "Indestructible. Whenever you gain life, put a +1/+1 counter on target creature or enchantment.",
		"walking ballista":    "{X}{X}: Walking Ballista enters with X +1/+1 counters on it. {4}: Put a +1/+1 counter on this. Remove a +1/+1 counter from this: It deals 1 damage to any target.",
	})

	m := BuildComboMetaInteraction(report, oracle)
	v := m.PerCombo[0]
	if v.PermanentPieces != 2 {
		t.Errorf("PermanentPieces: got %d, want 2", v.PermanentPieces)
	}
	if v.ProtectedPieces != 1 {
		t.Errorf("ProtectedPieces: got %d, want 1 (Heliod is indestructible)", v.ProtectedPieces)
	}
	if v.RemovalRequiredToBreak != 1 {
		t.Errorf("RemovalRequiredToBreak: got %d, want 1 (Ballista is unprotected)", v.RemovalRequiredToBreak)
	}
	if m.FragileComboCount != 1 {
		t.Errorf("FragileComboCount: got %d, want 1", m.FragileComboCount)
	}
}

// TestMetaVuln_AllProtectedCombo: every permanent piece has built-in
// protection → RemovalRequiredToBreak = 2 (strip + remove).
func TestMetaVuln_AllProtectedCombo(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Indestructible One", TypeLine: "Creature — Avatar"},
		{Name: "Hexproof Two", TypeLine: "Creature — Wizard"},
	}
	report := metaReportWith(profiles, [][]string{{"Indestructible One", "Hexproof Two"}}, nil, nil)
	oracle := stubOracleWithText(map[string]string{
		"indestructible one": "Indestructible. This creature can't be destroyed by damage or destroy effects.",
		"hexproof two":       "Hexproof. This creature can't be the target of spells or abilities your opponents control.",
	})

	m := BuildComboMetaInteraction(report, oracle)
	v := m.PerCombo[0]
	if v.PermanentPieces != 2 {
		t.Errorf("PermanentPieces: got %d, want 2", v.PermanentPieces)
	}
	if v.ProtectedPieces != 2 {
		t.Errorf("ProtectedPieces: got %d, want 2", v.ProtectedPieces)
	}
	if v.RemovalRequiredToBreak != 2 {
		t.Errorf("RemovalRequiredToBreak: got %d, want 2 (need protection strip + removal)",
			v.RemovalRequiredToBreak)
	}
	if m.FragileComboCount != 0 {
		t.Errorf("FragileComboCount: got %d, want 0", m.FragileComboCount)
	}
}

// TestMetaVuln_InstantSorceryCombo: combo with no permanent pieces
// (purely instants/sorceries) has RemovalRequiredToBreak = 0 — spot
// removal cannot break it (would need counterspells).
func TestMetaVuln_InstantSorceryCombo(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Demonic Consultation", TypeLine: "Instant"},
		{Name: "Tainted Pact", TypeLine: "Instant"},
	}
	report := metaReportWith(profiles, [][]string{{"Demonic Consultation", "Tainted Pact"}}, nil, nil)
	oracle := stubOracleWithText(map[string]string{
		"demonic consultation": "Choose a card name. Exile the top six cards of your library, then exile all cards from your library with that name.",
		"tainted pact":         "Exile the top card of your library. You may put that card into your hand unless it has the same name as another card exiled this way...",
	})

	m := BuildComboMetaInteraction(report, oracle)
	v := m.PerCombo[0]
	if v.PermanentPieces != 0 {
		t.Errorf("PermanentPieces: got %d, want 0", v.PermanentPieces)
	}
	if v.RemovalRequiredToBreak != 0 {
		t.Errorf("RemovalRequiredToBreak: got %d, want 0 (instants can't be spot-removed)",
			v.RemovalRequiredToBreak)
	}
}

// TestMetaVuln_ResilientCombo: low-vulnerability combo with all
// pieces protected and no stax/graveyard exposure → DominantThreat
// = "resilient".
func TestMetaVuln_ResilientCombo(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Indestructo A", TypeLine: "Creature — Beast"},
		{Name: "Indestructo B", TypeLine: "Creature — Beast"},
	}
	// 2-piece combo, no recursion, no storm, no activated abilities,
	// no graveyard use. Removal cost 2 since both protected.
	report := metaReportWith(profiles, [][]string{{"Indestructo A", "Indestructo B"}}, nil, nil)
	oracle := stubOracleWithText(map[string]string{
		"indestructo a": "Indestructible.",
		"indestructo b": "Indestructible.",
	})
	m := BuildComboMetaInteraction(report, oracle)
	v := m.PerCombo[0]
	if v.DominantThreat != "resilient" {
		t.Errorf("DominantThreat: got %q, want \"resilient\" (no stax/gy vuln, 2-removal cost)",
			v.DominantThreat)
	}
}

// TestMetaVuln_WorstHoserRollup: 3 graveyard combos → WorstGraveyardHoser
// has count = 3.
func TestMetaVuln_WorstHoserRollup(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Recur1", TypeLine: "Creature", IsRecursion: true, RecursionDest: "battlefield"},
		{Name: "Target1", TypeLine: "Creature"},
		{Name: "Recur2", TypeLine: "Enchantment", IsRecursion: true, RecursionDest: "battlefield"},
		{Name: "Target2", TypeLine: "Creature"},
		{Name: "Recur3", TypeLine: "Creature", IsRecursion: true, RecursionDest: "battlefield"},
		{Name: "Target3", TypeLine: "Creature"},
	}
	report := metaReportWith(profiles, nil, nil, [][]string{
		{"Recur1", "Target1"},
		{"Recur2", "Target2"},
		{"Recur3", "Target3"},
	})
	m := BuildComboMetaInteraction(report, nil)
	if m.WorstGraveyardCount != 3 {
		t.Errorf("WorstGraveyardCount: got %d, want 3", m.WorstGraveyardCount)
	}
	if m.WorstGraveyardHoser == "" {
		t.Errorf("WorstGraveyardHoser should be set")
	}
	if m.SurviveGraveyardCount != 0 {
		t.Errorf("SurviveGraveyardCount: got %d, want 0 (all 3 use graveyard)",
			m.SurviveGraveyardCount)
	}
}

// TestMetaVuln_SurviveCounts: mixed deck — some combos use graveyard,
// some don't. SurviveGraveyardCount counts the non-graveyard ones.
func TestMetaVuln_SurviveCounts(t *testing.T) {
	profiles := []CardProfile{
		{Name: "GyA", TypeLine: "Creature", IsRecursion: true},
		{Name: "GyB", TypeLine: "Creature"},
		{Name: "NonA", TypeLine: "Creature"},
		{Name: "NonB", TypeLine: "Creature"},
	}
	report := metaReportWith(profiles, nil, nil, [][]string{
		{"GyA", "GyB"},   // graveyard combo via IsRecursion piece (will count)
	})
	// Add a non-graveyard combo into TrueInfinites separately:
	report.TrueInfinites = append(report.TrueInfinites, ComboResult{
		Cards: []string{"NonA", "NonB"}, LoopType: "true_infinite",
	})
	m := BuildComboMetaInteraction(report, nil)
	if len(m.PerCombo) != 2 {
		t.Fatalf("expected 2 combos, got %d", len(m.PerCombo))
	}
	if m.SurviveGraveyardCount != 1 {
		t.Errorf("SurviveGraveyardCount: got %d, want 1 (NonA+NonB independent)",
			m.SurviveGraveyardCount)
	}
}

// TestMetaVuln_NilOnNoCombo: report with no combos → nil.
func TestMetaVuln_NilOnNoCombo(t *testing.T) {
	if m := BuildComboMetaInteraction(&FreyaReport{}, nil); m != nil {
		t.Errorf("expected nil for combo-less report, got %+v", m)
	}
}

// TestMetaVuln_NilOnNilReport: defensive nil-input.
func TestMetaVuln_NilOnNilReport(t *testing.T) {
	if m := BuildComboMetaInteraction(nil, nil); m != nil {
		t.Errorf("expected nil for nil report, got %+v", m)
	}
}

// TestMetaVuln_DrannithFiresOnRecursion: a TrueInfinite combo that
// includes an IsRecursion piece (Worldgorger / Animate Dead family)
// triggers Drannith Magistrate (casts_from_nonhand) AND the graveyard
// hoser family.
func TestMetaVuln_DrannithFiresOnRecursion(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Animate Dead", TypeLine: "Enchantment — Aura", IsRecursion: true, RecursionDest: "battlefield"},
		{Name: "Worldgorger Dragon", TypeLine: "Creature — Dragon"},
	}
	report := metaReportWith(profiles,
		[][]string{{"Animate Dead", "Worldgorger Dragon"}}, nil, nil)
	m := BuildComboMetaInteraction(report, nil)
	v := m.PerCombo[0]
	foundDrannith := false
	for _, h := range v.StaxHosers {
		if h == "Drannith Magistrate" {
			foundDrannith = true
		}
	}
	if !foundDrannith {
		t.Errorf("expected Drannith Magistrate in StaxHosers (recursion-using combo), got %v",
			v.StaxHosers)
	}
}

// TestPrintComboMetaInteraction_RendersHeaders: text output contains
// the canonical headers and per-combo lines.
func TestPrintComboMetaInteraction_RendersHeaders(t *testing.T) {
	profiles := []CardProfile{
		{Name: "GA", TypeLine: "Creature", IsRecursion: true},
		{Name: "GB", TypeLine: "Creature"},
	}
	report := metaReportWith(profiles, nil, nil, [][]string{{"GA", "GB"}})
	m := BuildComboMetaInteraction(report, nil)
	var buf bytes.Buffer
	printComboMetaInteraction(&buf, m)
	out := buf.String()
	musts := []string{
		"COMBO vs META",
		"Worst graveyard hoser:",
		"Survives stax:",
		"Survives graveyard hate:",
		"1-removal-fragile:",
		"dominant threat:",
		"graveyard (sev",
		"removal: ",
	}
	for _, s := range musts {
		if !strings.Contains(out, s) {
			t.Errorf("text output missing %q\nfull output:\n%s", s, out)
		}
	}
}

// TestPrintComboMetaInteraction_NilNoOp: nil input renders nothing.
func TestPrintComboMetaInteraction_NilNoOp(t *testing.T) {
	var buf bytes.Buffer
	printComboMetaInteraction(&buf, nil)
	if buf.Len() != 0 {
		t.Errorf("expected zero output for nil, got: %s", buf.String())
	}
}

// TestMetaVuln_NamedAnchorTriggersGraveyard: an Underworld Breach
// piece (named anchor only — no IsRecursion flag) still classifies as
// graveyard-using and surfaces the hate set.
func TestMetaVuln_NamedAnchorTriggersGraveyard(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Underworld Breach", TypeLine: "Enchantment"},
		{Name: "Brain Freeze", TypeLine: "Instant"},
		{Name: "Lion's Eye Diamond", TypeLine: "Artifact"},
	}
	report := metaReportWith(profiles,
		[][]string{{"Underworld Breach", "Brain Freeze", "Lion's Eye Diamond"}}, nil, nil)
	m := BuildComboMetaInteraction(report, nil)
	v := m.PerCombo[0]
	if v.GraveyardScore == 0 {
		t.Errorf("expected GraveyardScore > 0 (Underworld Breach is a named anchor); got 0\nhosers: %v",
			v.GraveyardHosers)
	}
}
