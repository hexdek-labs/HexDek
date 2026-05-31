package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sba_probe_test.go — pins the CR §704 SBA probe across the 9
// snapshot-detectable sub-rules. Each test writes a hand-rolled
// snapshot JSON to a temp dir, runs runSBAProbe, and inspects the
// structured report. Covers passing AND failing for each rule, the
// LeftGame skip, the PhasedOut skip, and the JSON output schema.

// writeSnap creates a temp dir with snapshot.json and returns its path.
func writeSnap(t *testing.T, snap SBASnapshot) string {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "snapshot.json")
	body, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// runProbe loads a snapshot path through runSBAProbe and returns the
// report. The caller-supplied snap is marshaled to disk so the full
// JSON I/O path runs (not just detectSBAViolations directly).
func runProbe(t *testing.T, snap SBASnapshot) *SBAReport {
	t.Helper()
	path := writeSnap(t, snap)
	rep, err := runSBAProbe(path, "")
	if err != nil {
		t.Fatalf("runSBAProbe: %v", err)
	}
	return rep
}

// hasViolationFor returns true if any violation in the report matches
// the given rule + (optionally) permanent name.
func hasViolationFor(rep *SBAReport, rule, permName string) bool {
	for _, v := range rep.Violations {
		if v.Rule == rule && (permName == "" || v.PermanentName == permName) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// §704.5a — life ≤ 0
// ---------------------------------------------------------------------------

func TestSBA_704_5a_Life_Zero_Violation(t *testing.T) {
	rep := runProbe(t, SBASnapshot{
		Seats: []SBASeat{{Idx: 0, Life: 0}, {Idx: 1, Life: 40}},
	})
	if rep.Valid {
		t.Fatalf("expected violation for life=0 seat 0; got valid")
	}
	if !hasViolationFor(rep, "704.5a", "") {
		t.Errorf("missing 704.5a violation: %+v", rep.Violations)
	}
}

func TestSBA_704_5a_Life_Negative_Violation(t *testing.T) {
	rep := runProbe(t, SBASnapshot{
		Seats: []SBASeat{{Idx: 0, Life: -7}},
	})
	if rep.ViolationsByRule["704.5a"] != 1 {
		t.Errorf("ViolationsByRule[704.5a] = %d, want 1", rep.ViolationsByRule["704.5a"])
	}
}

func TestSBA_704_5a_Life_ZeroButLost_NoViolation(t *testing.T) {
	// Seat at life=0 but already Lost=true — engine has resolved the
	// SBA. The probe must NOT fire again.
	rep := runProbe(t, SBASnapshot{
		Seats: []SBASeat{{Idx: 0, Life: 0, Lost: true}},
	})
	if hasViolationFor(rep, "704.5a", "") {
		t.Errorf("704.5a fired despite Lost=true: %+v", rep.Violations)
	}
}

func TestSBA_704_5a_LeftGame_NoViolation(t *testing.T) {
	// Seat with LeftGame=true (post-§800.4a cleanup) has all its
	// state preserved for audit; SBAs are by definition already
	// resolved.
	rep := runProbe(t, SBASnapshot{
		Seats: []SBASeat{{Idx: 0, Life: -5, LeftGame: true}},
	})
	if hasViolationFor(rep, "704.5a", "") {
		t.Errorf("704.5a fired despite LeftGame=true: %+v", rep.Violations)
	}
}

// ---------------------------------------------------------------------------
// §704.5c — 10+ poison counters
// ---------------------------------------------------------------------------

func TestSBA_704_5c_TenPoison_Violation(t *testing.T) {
	rep := runProbe(t, SBASnapshot{
		Seats: []SBASeat{{Idx: 0, Life: 30, PoisonCounters: 10}},
	})
	if !hasViolationFor(rep, "704.5c", "") {
		t.Errorf("missing 704.5c violation at exactly 10 poison: %+v", rep.Violations)
	}
}

func TestSBA_704_5c_NinePoison_NoViolation(t *testing.T) {
	rep := runProbe(t, SBASnapshot{
		Seats: []SBASeat{{Idx: 0, Life: 30, PoisonCounters: 9}},
	})
	if !rep.Valid {
		t.Errorf("expected valid at 9 poison (below threshold), got: %+v", rep.Violations)
	}
}

// ---------------------------------------------------------------------------
// §704.5f — creature with toughness ≤ 0
// ---------------------------------------------------------------------------

func TestSBA_704_5f_ZeroToughness_Violation(t *testing.T) {
	rep := runProbe(t, SBASnapshot{
		Seats: []SBASeat{{
			Idx: 0, Life: 20,
			Battlefield: []SBAPermanent{
				{Name: "0/0 Token", Types: []string{"creature"}, BasePower: 0, BaseToughness: 0},
			},
		}},
	})
	if !hasViolationFor(rep, "704.5f", "0/0 Token") {
		t.Errorf("missing 704.5f for 0-toughness creature: %+v", rep.Violations)
	}
}

func TestSBA_704_5f_NegativeToughnessViaCounter_Violation(t *testing.T) {
	// 2/2 creature with three -1/-1 counters → effective -1/-1, must die.
	rep := runProbe(t, SBASnapshot{
		Seats: []SBASeat{{
			Idx: 0, Life: 20,
			Battlefield: []SBAPermanent{
				{Name: "Bear", Types: []string{"creature"}, BasePower: 2, BaseToughness: 2,
					Counters: map[string]int{"-1/-1": 3}},
			},
		}},
	})
	if !hasViolationFor(rep, "704.5f", "Bear") {
		t.Errorf("missing 704.5f for 2/2 with three -1/-1 counters: %+v", rep.Violations)
	}
}

// ---------------------------------------------------------------------------
// §704.5g — marked damage ≥ toughness
// ---------------------------------------------------------------------------

func TestSBA_704_5g_LethalDamage_Violation(t *testing.T) {
	rep := runProbe(t, SBASnapshot{
		Seats: []SBASeat{{
			Idx: 0, Life: 20,
			Battlefield: []SBAPermanent{
				{Name: "Wounded Bear", Types: []string{"creature"}, BasePower: 2, BaseToughness: 2, MarkedDamage: 2},
			},
		}},
	})
	if !hasViolationFor(rep, "704.5g", "Wounded Bear") {
		t.Errorf("missing 704.5g for 2-damage 2-toughness creature: %+v", rep.Violations)
	}
}

func TestSBA_704_5g_NonLethalDamage_NoViolation(t *testing.T) {
	rep := runProbe(t, SBASnapshot{
		Seats: []SBASeat{{
			Idx: 0, Life: 20,
			Battlefield: []SBAPermanent{
				{Name: "Bruised Bear", Types: []string{"creature"}, BasePower: 2, BaseToughness: 3, MarkedDamage: 2},
			},
		}},
	})
	if !rep.Valid {
		t.Errorf("expected valid for 2-damage 3-toughness, got: %+v", rep.Violations)
	}
}

// ---------------------------------------------------------------------------
// §704.5h — planeswalker with 0 loyalty
// ---------------------------------------------------------------------------

func TestSBA_704_5h_ZeroLoyalty_Violation(t *testing.T) {
	rep := runProbe(t, SBASnapshot{
		Seats: []SBASeat{{
			Idx: 0, Life: 20,
			Battlefield: []SBAPermanent{
				{Name: "Spent Jace", Types: []string{"planeswalker"},
					Supertypes: []string{"legendary"}, Loyalty: 0},
			},
		}},
	})
	if !hasViolationFor(rep, "704.5h", "Spent Jace") {
		t.Errorf("missing 704.5h for 0-loyalty planeswalker: %+v", rep.Violations)
	}
}

// ---------------------------------------------------------------------------
// §704.5i — legend rule (same controller, same legendary name × 2)
// ---------------------------------------------------------------------------

func TestSBA_704_5i_DuplicateLegendaries_Violation(t *testing.T) {
	rep := runProbe(t, SBASnapshot{
		Seats: []SBASeat{{
			Idx: 0, Life: 20,
			Battlefield: []SBAPermanent{
				{Name: "Atraxa, Praetors' Voice", Types: []string{"creature"},
					Supertypes: []string{"legendary"}, BasePower: 4, BaseToughness: 4, Timestamp: 10},
				{Name: "Atraxa, Praetors' Voice", Types: []string{"creature"},
					Supertypes: []string{"legendary"}, BasePower: 4, BaseToughness: 4, Timestamp: 11},
			},
		}},
	})
	if !hasViolationFor(rep, "704.5i", "Atraxa, Praetors' Voice") {
		t.Errorf("missing 704.5i for duplicate legendaries: %+v", rep.Violations)
	}
}

func TestSBA_704_5i_DifferentNames_NoViolation(t *testing.T) {
	// Two legendaries with different names under one controller is fine.
	rep := runProbe(t, SBASnapshot{
		Seats: []SBASeat{{
			Idx: 0, Life: 20,
			Battlefield: []SBAPermanent{
				{Name: "Atraxa, Praetors' Voice", Types: []string{"creature"},
					Supertypes: []string{"legendary"}, BasePower: 4, BaseToughness: 4},
				{Name: "Edgar Markov", Types: []string{"creature"},
					Supertypes: []string{"legendary"}, BasePower: 4, BaseToughness: 4},
			},
		}},
	})
	if !rep.Valid {
		t.Errorf("expected valid: distinct legendary names are fine: %+v", rep.Violations)
	}
}

// ---------------------------------------------------------------------------
// §704.5j — world rule (cross-seat aggregation)
// ---------------------------------------------------------------------------

func TestSBA_704_5j_TwoWorlds_Violation(t *testing.T) {
	rep := runProbe(t, SBASnapshot{
		Seats: []SBASeat{
			{Idx: 0, Life: 20, Battlefield: []SBAPermanent{
				{Name: "In the Eye of Chaos", Types: []string{"enchantment"},
					Supertypes: []string{"world"}, Timestamp: 5},
			}},
			{Idx: 1, Life: 20, Battlefield: []SBAPermanent{
				{Name: "Nether Void", Types: []string{"enchantment"},
					Supertypes: []string{"world"}, Timestamp: 7},
			}},
		},
	})
	// World rule violations are emitted per-world-permanent (each "all but
	// newest goes" leaves multiple violations on the same call).
	if rep.ViolationsByRule["704.5j"] < 2 {
		t.Errorf("ViolationsByRule[704.5j] = %d, want ≥2 (one per offending world)",
			rep.ViolationsByRule["704.5j"])
	}
}

// ---------------------------------------------------------------------------
// §704.5p — un-paired +1/+1 and -1/-1 counters
// ---------------------------------------------------------------------------

func TestSBA_704_5p_BothCounterKinds_Violation(t *testing.T) {
	rep := runProbe(t, SBASnapshot{
		Seats: []SBASeat{{
			Idx: 0, Life: 20,
			Battlefield: []SBAPermanent{
				{Name: "Hangarback Walker", Types: []string{"creature"},
					BasePower: 0, BaseToughness: 0,
					Counters: map[string]int{"+1/+1": 3, "-1/-1": 2}},
			},
		}},
	})
	if !hasViolationFor(rep, "704.5p", "Hangarback Walker") {
		t.Errorf("missing 704.5p for both +1/+1 and -1/-1 counters: %+v", rep.Violations)
	}
}

// ---------------------------------------------------------------------------
// §704.6c — 21+ commander damage
// ---------------------------------------------------------------------------

func TestSBA_704_6c_TwentyOneDamage_Violation(t *testing.T) {
	rep := runProbe(t, SBASnapshot{
		Format: "commander",
		Seats: []SBASeat{
			{Idx: 0, Life: 30,
				CommanderDamage: map[string]map[string]int{
					"1": {"Edgar Markov": 21},
				}},
			{Idx: 1, Life: 30, CommandZone: []string{"Edgar Markov"}},
		},
	})
	if !hasViolationFor(rep, "704.6c", "") {
		t.Errorf("missing 704.6c at 21 damage: %+v", rep.Violations)
	}
}

func TestSBA_704_6c_TwentyDamage_NoViolation(t *testing.T) {
	rep := runProbe(t, SBASnapshot{
		Format: "commander",
		Seats: []SBASeat{
			{Idx: 0, Life: 30,
				CommanderDamage: map[string]map[string]int{
					"1": {"Kraum, Ludevic's Opus": 20},
				}},
		},
	})
	if !rep.Valid {
		t.Errorf("expected valid at exactly 20 commander damage (threshold is 21+); got: %+v", rep.Violations)
	}
}

// ---------------------------------------------------------------------------
// PhasedOut skip — §702.26
// ---------------------------------------------------------------------------

func TestSBA_PhasedOutCreature_SkipsAllChecks(t *testing.T) {
	// A phased-out 0/0 creature must NOT fire §704.5f (the creature is
	// "treated as though it doesn't exist" per CR §702.26). Same for
	// the duplicate-legendary check.
	rep := runProbe(t, SBASnapshot{
		Seats: []SBASeat{{
			Idx: 0, Life: 20,
			Battlefield: []SBAPermanent{
				{Name: "Ghostly Bear", Types: []string{"creature"},
					BasePower: 0, BaseToughness: 0, PhasedOut: true},
			},
		}},
	})
	if hasViolationFor(rep, "704.5f", "Ghostly Bear") {
		t.Errorf("704.5f fired despite PhasedOut=true: %+v", rep.Violations)
	}
}

// ---------------------------------------------------------------------------
// Multiple-rule snapshot — sanity that the probe emits every violation,
// not just the first one
// ---------------------------------------------------------------------------

func TestSBA_MultipleViolations_AllReported(t *testing.T) {
	rep := runProbe(t, SBASnapshot{
		Seats: []SBASeat{{
			Idx: 0, Life: 0, PoisonCounters: 12,
			Battlefield: []SBAPermanent{
				{Name: "Wisp", Types: []string{"creature"}, BaseToughness: 0},
				{Name: "Spent Jace", Types: []string{"planeswalker"}, Loyalty: 0,
					Supertypes: []string{"legendary"}},
			},
		}},
	})
	// Expect 704.5a + 704.5c + 704.5f + 704.5h.
	for _, want := range []string{"704.5a", "704.5c", "704.5f", "704.5h"} {
		if rep.ViolationsByRule[want] < 1 {
			t.Errorf("missing %s in multi-violation snapshot: %+v",
				want, rep.ViolationsByRule)
		}
	}
}

// ---------------------------------------------------------------------------
// JSON output schema lock
// ---------------------------------------------------------------------------

func TestSBA_ReportJSONShape(t *testing.T) {
	snap := SBASnapshot{
		Format: "commander",
		Turn:   5,
		Seats: []SBASeat{
			{Idx: 0, Life: -1},
		},
	}
	path := writeSnap(t, snap)
	tmpOut := filepath.Join(filepath.Dir(path), "report.json")
	if _, err := runSBAProbe(path, tmpOut); err != nil {
		t.Fatalf("runSBAProbe: %v", err)
	}
	raw, err := os.ReadFile(tmpOut)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		`"rule"`, `"snapshot_path"`, `"turn"`, `"seat_count"`,
		`"violations"`, `"violations_by_rule"`, `"valid"`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("missing top-level key %s in:\n%s", key, raw)
		}
	}
	for _, key := range []string{
		`"rule"`, `"rule_name"`, `"seat"`, `"detail"`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("missing violation field %s in:\n%s", key, raw)
		}
	}
	// Round-trip sanity.
	var rt SBAReport
	if err := json.Unmarshal(raw, &rt); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}
	if rt.Rule != "CR §704" {
		t.Errorf("Rule = %q, want CR §704", rt.Rule)
	}
}
