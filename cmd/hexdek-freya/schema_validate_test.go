package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestFreyaOutputSchema_Compiles is the smoke test that the embedded
// schema is itself valid JSON Schema. FreyaOutputSchema() panics on
// compile failure, so calling it here proves the schema can be loaded.
// Catches accidental syntax errors in the committed schema file.
func TestFreyaOutputSchema_Compiles(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("schema compilation panicked: %v", r)
		}
	}()
	s := FreyaOutputSchema()
	if s == nil {
		t.Fatal("FreyaOutputSchema returned nil")
	}
}

// TestValidateFreyaJSON_ValidPayloadPasses runs a minimally-valid
// payload through the validator. The payload mirrors the shape of
// real Freya output but uses literal values rather than rendering
// from a FreyaReport — this isolates the schema test from the
// rendering pipeline.
func TestValidateFreyaJSON_ValidPayloadPasses(t *testing.T) {
	body := `{
		"deck_name": "test_deck",
		"deck_path": "/data/decks/test_deck.txt",
		"commander": "Atraxa, Praetors' Voice",
		"total_cards": 100,
		"true_infinites": [],
		"determined_loops": [],
		"finishers": [],
		"synergies": [],
		"mana_curve": {"distribution": [0,8,18,15,10,7,3,2], "avg_cmc": 3.1, "curve_shape": "midrange", "land_count": 37, "nonland_count": 63},
		"color_balance": {"demand": {"W": 12, "U": 14, "B": 14, "G": 12}, "supply": {"W": 22, "U": 24, "B": 22, "G": 22}}
	}`

	errs, parseErr := ValidateFreyaJSON([]byte(body))
	if parseErr != nil {
		t.Fatalf("unexpected parse error: %v", parseErr)
	}
	if len(errs) > 0 {
		t.Errorf("expected zero validation errors, got %d:\n%s",
			len(errs), strings.Join(errs, "\n"))
	}
}

// TestValidateFreyaJSON_MissingRequiredFields verifies the schema
// catches dropped/renamed required fields — this is the drift
// detector. A payload missing `total_cards` should produce a
// validation error mentioning the required key.
func TestValidateFreyaJSON_MissingRequiredFields(t *testing.T) {
	body := `{
		"deck_name": "missing_total",
		"true_infinites": [],
		"determined_loops": [],
		"finishers": [],
		"synergies": [],
		"mana_curve": {"distribution": [0,8,18,15,10,7,3,2], "avg_cmc": 3.1},
		"color_balance": {"demand": {}, "supply": {}}
	}`
	errs, parseErr := ValidateFreyaJSON([]byte(body))
	if parseErr != nil {
		t.Fatalf("parse error: %v", parseErr)
	}
	if len(errs) == 0 {
		t.Fatal("expected at least one validation error for missing total_cards")
	}
	// At least one error should reference the missing required property.
	joined := strings.Join(errs, " | ")
	if !strings.Contains(strings.ToLower(joined), "total_cards") &&
		!strings.Contains(strings.ToLower(joined), "missing") &&
		!strings.Contains(strings.ToLower(joined), "required") {
		t.Errorf("validation errors did not mention the missing required field; got: %s", joined)
	}
}

// TestValidateFreyaJSON_WrongType verifies type-mismatch detection.
// total_cards is required to be an integer; passing a string should
// fire a validation error.
func TestValidateFreyaJSON_WrongType(t *testing.T) {
	body := `{
		"deck_name": "test_deck",
		"total_cards": "ninety-nine",
		"true_infinites": [],
		"determined_loops": [],
		"finishers": [],
		"synergies": [],
		"mana_curve": {"distribution": [0,8,18,15,10,7,3,2], "avg_cmc": 3.1},
		"color_balance": {"demand": {}, "supply": {}}
	}`
	errs, _ := ValidateFreyaJSON([]byte(body))
	if len(errs) == 0 {
		t.Errorf("expected validation error for total_cards as string, got none")
	}
}

// TestValidateFreyaJSON_BadEnumValue verifies the curve_shape enum
// constraint fires when an out-of-band value sneaks in. Catches
// classifier bugs that produce unexpected shape labels.
func TestValidateFreyaJSON_BadEnumValue(t *testing.T) {
	body := `{
		"deck_name": "test_deck",
		"total_cards": 100,
		"true_infinites": [],
		"determined_loops": [],
		"finishers": [],
		"synergies": [],
		"mana_curve": {"distribution": [0,8,18,15,10,7,3,2], "avg_cmc": 3.1, "curve_shape": "lopsided"},
		"color_balance": {"demand": {}, "supply": {}}
	}`
	errs, _ := ValidateFreyaJSON([]byte(body))
	if len(errs) == 0 {
		t.Errorf("expected validation error for unknown curve_shape enum value")
	}
}

// TestValidateFreyaJSON_ManaCurveLengthEnforced verifies the
// fixed-length [8]int constraint on mana_curve.distribution. A 7-
// element distribution should be rejected.
func TestValidateFreyaJSON_ManaCurveLengthEnforced(t *testing.T) {
	body := `{
		"deck_name": "test_deck",
		"total_cards": 100,
		"true_infinites": [],
		"determined_loops": [],
		"finishers": [],
		"synergies": [],
		"mana_curve": {"distribution": [0,8,18,15,10,7,3], "avg_cmc": 3.1},
		"color_balance": {"demand": {}, "supply": {}}
	}`
	errs, _ := ValidateFreyaJSON([]byte(body))
	if len(errs) == 0 {
		t.Errorf("expected validation error for 7-element mana_curve.distribution (want exactly 8)")
	}
}

// TestValidateFreyaJSON_ComboValidEntryPasses verifies that a fully-
// shaped combo entry validates correctly. Specifically pins the
// loop_type enum + cards minItems=1 contract that the bracket scorer
// + win-line analyzer downstream consumers depend on.
func TestValidateFreyaJSON_ComboValidEntryPasses(t *testing.T) {
	body := `{
		"deck_name": "test_deck",
		"total_cards": 100,
		"true_infinites": [
			{
				"cards": ["Thassa's Oracle", "Demonic Consultation"],
				"loop_type": "true_infinite",
				"class": "library_exile_win",
				"description": "Exile library on Consultation; Thoracle wins on empty library.",
				"confirmed": true
			}
		],
		"determined_loops": [],
		"finishers": [],
		"synergies": [],
		"mana_curve": {"distribution": [0,8,18,15,10,7,3,2], "avg_cmc": 3.1},
		"color_balance": {"demand": {}, "supply": {}}
	}`
	errs, parseErr := ValidateFreyaJSON([]byte(body))
	if parseErr != nil {
		t.Fatalf("parse error: %v", parseErr)
	}
	if len(errs) > 0 {
		t.Errorf("valid combo entry rejected: %s", strings.Join(errs, "\n"))
	}
}

// TestValidateFreyaJSON_ComboBadLoopTypeRejected verifies an unknown
// loop_type fails the enum check. Without this, a classifier
// regression that emits "synergy_loop" instead of "synergy" would
// silently break every downstream consumer expecting the enum.
func TestValidateFreyaJSON_ComboBadLoopTypeRejected(t *testing.T) {
	body := `{
		"deck_name": "test_deck",
		"total_cards": 100,
		"true_infinites": [
			{
				"cards": ["Card A"],
				"loop_type": "unrecognized_loop_kind",
				"description": "test"
			}
		],
		"determined_loops": [],
		"finishers": [],
		"synergies": [],
		"mana_curve": {"distribution": [0,8,18,15,10,7,3,2], "avg_cmc": 3.1},
		"color_balance": {"demand": {}, "supply": {}}
	}`
	errs, _ := ValidateFreyaJSON([]byte(body))
	if len(errs) == 0 {
		t.Errorf("expected validation error for unknown loop_type")
	}
}

// TestValidateFreyaJSON_AdditionalPropertiesAllowed verifies the
// schema does NOT reject novel top-level keys. Freya routinely grows
// new fields (recent additions: pet_cards, tech_suggestions, etc.)
// and breaking the build on every additive change would defeat the
// schema's purpose. Drift = removing/renaming, not adding.
func TestValidateFreyaJSON_AdditionalPropertiesAllowed(t *testing.T) {
	body := `{
		"deck_name": "test_deck",
		"total_cards": 100,
		"true_infinites": [],
		"determined_loops": [],
		"finishers": [],
		"synergies": [],
		"mana_curve": {"distribution": [0,8,18,15,10,7,3,2], "avg_cmc": 3.1},
		"color_balance": {"demand": {}, "supply": {}},
		"some_brand_new_field_invented_after_schema_was_written": "okay",
		"another_new_thing": {"with": "structure"}
	}`
	errs, _ := ValidateFreyaJSON([]byte(body))
	if len(errs) > 0 {
		t.Errorf("schema rejected novel fields (should allow additive growth): %s",
			strings.Join(errs, "\n"))
	}
}

// TestValidateFreyaJSON_EmptyInputErrors verifies the empty-input
// guard. Downstream callers passing nil/empty bytes should see a
// useful error rather than a silent pass.
func TestValidateFreyaJSON_EmptyInputErrors(t *testing.T) {
	_, err := ValidateFreyaJSON(nil)
	if err == nil {
		t.Error("expected error on nil input")
	}
	_, err = ValidateFreyaJSON([]byte{})
	if err == nil {
		t.Error("expected error on empty input")
	}
}

// TestValidateFreyaJSON_BadJSONErrors verifies malformed JSON produces
// a parse-error return (distinct from the validation-error slice).
func TestValidateFreyaJSON_BadJSONErrors(t *testing.T) {
	_, err := ValidateFreyaJSON([]byte("not json at all"))
	if err == nil {
		t.Error("expected parse error on malformed JSON input")
	}
}

// TestFreyaSchema_GeneratedOutputValidates is the CI-enforced drift
// detector. Marshals a fully-populated synthetic FreyaReport through
// the actual printJSON rendering pipeline and asserts the output
// passes the schema. Catches scenarios where:
//
//   - A required field is dropped from the jsonReport struct
//   - A field is renamed (json tag changed)
//   - A field's JSON type changes (int → string, etc.)
//   - The enum domain on loop_type shifts
//
// This is the test that fires under CI when code and schema diverge.
// Synthetic FreyaReport is built without oracle data (no live
// classifier runs) — just literal values populating every field
// the schema mentions.
func TestFreyaSchema_GeneratedOutputValidates(t *testing.T) {
	report := &FreyaReport{
		DeckName:   "schema_drift_probe",
		DeckPath:   "/tmp/schema_drift_probe.txt",
		Commander:  "Atraxa, Praetors' Voice",
		TotalCards: 100,
		TrueInfinites: []ComboResult{
			{
				Cards:       []string{"Thassa's Oracle", "Demonic Consultation"},
				LoopType:    "true_infinite",
				Class:       "library_exile_win",
				Description: "exile library + thoracle win",
				Confirmed:   true,
			},
		},
		Determined: []ComboResult{
			{
				Cards:       []string{"Card X", "Card Y"},
				LoopType:    "determined",
				Description: "determined loop sample",
			},
		},
		Finishers: []ComboResult{
			{Cards: []string{"Craterhoof Behemoth"}, LoopType: "finisher", Description: "lethal swing"},
		},
		Synergies: []ComboResult{
			{Cards: []string{"Smothering Tithe", "Anointed Procession"}, LoopType: "synergy", Description: "treasure doubling"},
		},
		ManaCurve:    [8]int{0, 8, 18, 15, 10, 7, 3, 2},
		AvgCMC:       3.1,
		LandCount:    37,
		NonlandCount: 63,
		CurveShape:   "midrange",
		ColorDemand:  map[string]int{"W": 12, "U": 14, "B": 14, "G": 12},
		ColorSupply:  map[string]int{"W": 22, "U": 24, "B": 22, "G": 22},
	}

	var buf bytes.Buffer
	printJSON(&buf, report)

	// First: confirm the marshaled output is itself parseable JSON.
	// A bad encoder would dump non-JSON; the schema validator only
	// catches schema-rule failures, so this sanity check covers the
	// marshalling layer separately.
	var probe any
	if err := json.Unmarshal(buf.Bytes(), &probe); err != nil {
		t.Fatalf("printJSON emitted non-JSON bytes: %v\n%s", err, buf.String())
	}

	// Now run the schema check. ANY validation error here is the
	// drift signal — CI fails until either the schema or the code is
	// updated to match.
	errs, parseErr := ValidateFreyaJSON(buf.Bytes())
	if parseErr != nil {
		t.Fatalf("schema validator returned a parse error on generated output: %v", parseErr)
	}
	if len(errs) > 0 {
		t.Errorf("generated Freya output FAILED schema validation — code/schema have drifted apart:\n  %s",
			strings.Join(errs, "\n  "))
	}
}
