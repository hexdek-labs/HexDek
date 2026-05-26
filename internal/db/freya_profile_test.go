package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

// openFreyaProfileTestDB opens an in-memory SQLite with the full
// schema + applied migrations. Mirrors what Open() does at startup
// so the test exercises the same code paths as production.
// (Sibling openTestDB lives in party_r60_test.go; this file has a
// dedicated helper to add a t.Cleanup hook without disturbing the
// existing party tests.)
func openFreyaProfileTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

// Test_FreyaProfile_SchemaPresent confirms the migration / schema
// application created deck_freya_profile with the expected columns.
// If a future PR renames the table or drops a column, this fails
// loudly before any caller breaks at runtime.
func Test_FreyaProfile_SchemaPresent(t *testing.T) {
	d := openFreyaProfileTestDB(t)
	rows, err := d.Query(`PRAGMA table_info(deck_freya_profile)`)
	if err != nil {
		t.Fatalf("pragma: %v", err)
	}
	defer rows.Close()
	cols := map[string]string{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan pragma: %v", err)
		}
		cols[name] = ctype
	}
	want := map[string]string{
		"deck_key":            "TEXT",
		"commander":           "TEXT",
		"owner":               "TEXT",
		"primary_archetype":   "TEXT",
		"secondary_archetype": "TEXT",
		"bracket":             "INTEGER",
		"synergy_pct":         "REAL",
		"power_percentile":    "INTEGER",
		"power_tier_counts":   "TEXT",
		"primary_roles":       "TEXT",
		"updated_at":          "INTEGER",
	}
	for col, ty := range want {
		got, ok := cols[col]
		if !ok {
			t.Errorf("deck_freya_profile missing column %q", col)
			continue
		}
		if got != ty {
			t.Errorf("deck_freya_profile.%s: type=%q want %q", col, got, ty)
		}
	}
}

// Test_FreyaProfile_UpsertLoadRoundtrip confirms the in-memory struct
// survives a SQL round-trip including the JSON-encoded map + slice
// fields. Catches column-order drift between UpsertFreyaProfile and
// LoadFreyaProfile and a regression where the JSON marshaling stops
// matching the load-side unmarshal.
func Test_FreyaProfile_UpsertLoadRoundtrip(t *testing.T) {
	d := openFreyaProfileTestDB(t)
	ctx := context.Background()
	in := FreyaProfile{
		DeckKey:            "moxfield/edgar_markov_b2_alice_aBcDeFgH",
		Commander:          "Edgar Markov",
		Owner:              "alice",
		PrimaryArchetype:   "Aggro / Tribal",
		SecondaryArchetype: "Tokens",
		Bracket:            2,
		SynergyPct:         62.5,
		PowerPercentile:    47,
		PowerTierCounts:    map[string]int{"S": 2, "A": 8, "B": 35, "C": 24, "D": 6},
		PrimaryRoles: []FreyaProfileRole{
			{Role: "threat", Count: 22},
			{Role: "ramp", Count: 11},
			{Role: "removal", Count: 7},
		},
	}
	if err := UpsertFreyaProfile(ctx, d, in); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	out, err := LoadFreyaProfile(ctx, d, in.DeckKey)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if out.DeckKey != in.DeckKey || out.Commander != in.Commander ||
		out.Owner != in.Owner || out.PrimaryArchetype != in.PrimaryArchetype ||
		out.SecondaryArchetype != in.SecondaryArchetype ||
		out.Bracket != in.Bracket || out.SynergyPct != in.SynergyPct ||
		out.PowerPercentile != in.PowerPercentile {
		t.Errorf("scalar mismatch:\n  in  = %+v\n  out = %+v", in, out)
	}
	if !reflect.DeepEqual(out.PowerTierCounts, in.PowerTierCounts) {
		t.Errorf("PowerTierCounts mismatch:\n  in  = %v\n  out = %v",
			in.PowerTierCounts, out.PowerTierCounts)
	}
	if !reflect.DeepEqual(out.PrimaryRoles, in.PrimaryRoles) {
		t.Errorf("PrimaryRoles mismatch:\n  in  = %v\n  out = %v",
			in.PrimaryRoles, out.PrimaryRoles)
	}
}

// Test_FreyaProfile_Upsert_ReplacesRow confirms ON CONFLICT(deck_key)
// overwrites the prior row rather than appending. This is the
// "re-runs replace the prior analysis" contract from the table
// comment in schema.sql.
func Test_FreyaProfile_Upsert_ReplacesRow(t *testing.T) {
	d := openFreyaProfileTestDB(t)
	ctx := context.Background()
	first := FreyaProfile{
		DeckKey: "moxfield/test_b2_x", Commander: "Atraxa",
		PrimaryArchetype: "Counters", Bracket: 2, SynergyPct: 30,
		PowerTierCounts: map[string]int{"S": 1, "A": 5},
	}
	second := FreyaProfile{
		DeckKey: "moxfield/test_b2_x", Commander: "Atraxa",
		PrimaryArchetype: "Superfriends", Bracket: 3, SynergyPct: 55,
		PowerTierCounts: map[string]int{"S": 3, "A": 12},
	}
	if err := UpsertFreyaProfile(ctx, d, first); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := UpsertFreyaProfile(ctx, d, second); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	out, err := LoadFreyaProfile(ctx, d, second.DeckKey)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if out.PrimaryArchetype != "Superfriends" || out.Bracket != 3 || out.SynergyPct != 55 {
		t.Errorf("second upsert did not replace first: %+v", out)
	}
	// And there's exactly one row, not two.
	var n int
	if err := d.QueryRow(`SELECT COUNT(*) FROM deck_freya_profile`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("row count after two upserts: got %d want 1", n)
	}
}

func Test_FreyaProfile_Load_ReturnsErrNoRows(t *testing.T) {
	d := openFreyaProfileTestDB(t)
	_, err := LoadFreyaProfile(context.Background(), d, "moxfield/does_not_exist")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("missing row: got err=%v want sql.ErrNoRows", err)
	}
}

func Test_FreyaProfile_Upsert_RejectsEmptyKey(t *testing.T) {
	d := openFreyaProfileTestDB(t)
	err := UpsertFreyaProfile(context.Background(), d,
		FreyaProfile{DeckKey: "   ", Commander: "X"})
	if err == nil {
		t.Error("upsert with empty deck_key: want error, got nil")
	}
}

func Test_FreyaProfile_LoadAll_OrderByUpdatedDesc(t *testing.T) {
	d := openFreyaProfileTestDB(t)
	ctx := context.Background()
	for _, k := range []string{"a", "b", "c"} {
		if err := UpsertFreyaProfile(ctx, d, FreyaProfile{
			DeckKey: "moxfield/" + k, Commander: k,
			PrimaryArchetype: "Combo", Bracket: 4,
		}); err != nil {
			t.Fatalf("upsert %s: %v", k, err)
		}
		// Bump updated_at so the ORDER BY is observable. SQLite's
		// unixepoch() has 1-second resolution; we can't beat that
		// inside a tight loop, so force monotonic updates by hand.
		_, err := d.ExecContext(ctx,
			`UPDATE deck_freya_profile SET updated_at = updated_at + ?
			 WHERE deck_key = ?`, len(k), "moxfield/"+k)
		if err != nil {
			t.Fatalf("bump updated_at: %v", err)
		}
	}
	all, err := LoadAllFreyaProfiles(ctx, d)
	if err != nil {
		t.Fatalf("load all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("load all returned %d rows, want 3", len(all))
	}
	// Decreasing updated_at: most-recently-bumped first.
	for i := 1; i < len(all); i++ {
		// Can't inspect updated_at directly through the struct (it's
		// not exposed), so re-query each row for its timestamp.
		var a, b int64
		_ = d.QueryRow(`SELECT updated_at FROM deck_freya_profile WHERE deck_key = ?`,
			all[i-1].DeckKey).Scan(&a)
		_ = d.QueryRow(`SELECT updated_at FROM deck_freya_profile WHERE deck_key = ?`,
			all[i].DeckKey).Scan(&b)
		if a < b {
			t.Errorf("LoadAllFreyaProfiles not ordered DESC: position %d updated_at=%d < %d at %d",
				i-1, a, b, i)
		}
	}
}

// Test_FreyaProfile_FromJSON_FullBlob exercises the parser on a JSON
// payload shaped exactly like Freya's --format json output. Catches
// regressions in the field-name mapping
// (CommanderSynergy → synergy_pct etc.) and the 0-1 → 0-100 rescaling
// of synergy.
func Test_FreyaProfile_FromJSON_FullBlob(t *testing.T) {
	blob := []byte(`{
		"deck_name": "edgar_markov_b2_alice",
		"commander": "Edgar Markov",
		"primary_archetype": "Aggro / Tribal",
		"secondary_archetype": "Tokens",
		"bracket": 2,
		"commander_synergy": 0.625,
		"power_percentile": 47,
		"power_tier_counts": {"S": 2, "A": 8, "B": 35, "C": 24, "D": 6},
		"top_roles": [
			{"role": "threat", "count": 22},
			{"role": "ramp", "count": 11},
			{"role": "removal", "count": 7}
		],
		"some_other_field": "ignored"
	}`)
	got, err := FreyaProfileFromJSON(blob, "moxfield/edgar_b2_alice", "alice", "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := FreyaProfile{
		DeckKey:            "moxfield/edgar_b2_alice",
		Commander:          "Edgar Markov",
		Owner:              "alice",
		PrimaryArchetype:   "Aggro / Tribal",
		SecondaryArchetype: "Tokens",
		Bracket:            2,
		SynergyPct:         62.5,
		PowerPercentile:    47,
		PowerTierCounts:    map[string]int{"S": 2, "A": 8, "B": 35, "C": 24, "D": 6},
		PrimaryRoles: []FreyaProfileRole{
			{Role: "threat", Count: 22},
			{Role: "ramp", Count: 11},
			{Role: "removal", Count: 7},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FromJSON mismatch:\n  got  = %+v\n  want = %+v", got, want)
	}
}

// Test_FreyaProfile_FromJSON_CommanderOverride confirms the
// commanderOverride argument takes precedence over the embedded
// `commander` field. The runFreya callsite doesn't strictly need
// this today, but it's the kind of thing that protects against
// future drift between the file-system commander_name (canonical)
// and the JSON's commander (a freya-derived label).
func Test_FreyaProfile_FromJSON_CommanderOverride(t *testing.T) {
	blob := []byte(`{"commander": "Auto-detected", "primary_archetype": "X"}`)
	got, err := FreyaProfileFromJSON(blob, "k", "o", "Real Name")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Commander != "Real Name" {
		t.Errorf("override: got %q want %q", got.Commander, "Real Name")
	}
}

// Test_FreyaProfile_FromJSON_SynergyClamp confirms out-of-range
// synergy values get clamped to [0, 100] rather than silently
// producing impossible rows. Defends against a future Freya version
// that emits an absolute card-count instead of a fraction.
func Test_FreyaProfile_FromJSON_SynergyClamp(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  float64
		want float64
	}{
		{"normal_fraction", 0.5, 50},
		{"below_zero", -0.1, 0},
		{"above_one", 1.5, 100},
		{"zero", 0, 0},
		{"one", 1, 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			blob, _ := json.Marshal(map[string]any{
				"commander_synergy": tc.raw,
			})
			got, err := FreyaProfileFromJSON(blob, "k", "o", "")
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got.SynergyPct != tc.want {
				t.Errorf("clamp(%v): got %v want %v", tc.raw, got.SynergyPct, tc.want)
			}
		})
	}
}

// Test_FreyaProfile_FromJSON_EmptyFields confirms missing optional
// fields produce zero-valued struct members rather than parse errors
// or nil-map crashes downstream. Freya's `omitempty` tags make this
// the common case for low-power decks.
func Test_FreyaProfile_FromJSON_EmptyFields(t *testing.T) {
	got, err := FreyaProfileFromJSON([]byte(`{"primary_archetype": "Bogles"}`),
		"k", "o", "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.PrimaryArchetype != "Bogles" {
		t.Errorf("archetype: got %q want Bogles", got.PrimaryArchetype)
	}
	if got.PowerTierCounts != nil {
		// Parse phase preserves nil; normalization to {} happens at
		// upsert time. Both shapes are valid here.
		if len(got.PowerTierCounts) != 0 {
			t.Errorf("PowerTierCounts: got %v want empty", got.PowerTierCounts)
		}
	}
	if len(got.PrimaryRoles) != 0 {
		t.Errorf("PrimaryRoles: got %v want empty", got.PrimaryRoles)
	}
}

// Test_FreyaProfile_Upsert_EmptyMapSerializedAsObject confirms
// PowerTierCounts=nil round-trips as `{}` in the column (not the JSON
// `null` that a nil map serializes to). The normalizedTierCounts
// helper is the implementation; this test pins the contract so a
// future refactor that drops the helper trips a CI failure.
func Test_FreyaProfile_Upsert_EmptyMapSerializedAsObject(t *testing.T) {
	d := openFreyaProfileTestDB(t)
	ctx := context.Background()
	in := FreyaProfile{DeckKey: "moxfield/empty_b2_x", Commander: "Test"}
	if err := UpsertFreyaProfile(ctx, d, in); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	var tierJSON, rolesJSON string
	if err := d.QueryRowContext(ctx,
		`SELECT power_tier_counts, primary_roles FROM deck_freya_profile WHERE deck_key = ?`,
		in.DeckKey).Scan(&tierJSON, &rolesJSON); err != nil {
		t.Fatalf("read raw columns: %v", err)
	}
	if tierJSON != "{}" {
		t.Errorf("power_tier_counts raw column: got %q want %q", tierJSON, "{}")
	}
	if rolesJSON != "[]" {
		t.Errorf("primary_roles raw column: got %q want %q", rolesJSON, "[]")
	}
}

// Test_FreyaProfile_FromJSON_ParseError confirms a junk payload
// returns an error rather than a zero-valued struct (which would
// silently overwrite a real prior row with garbage on the next
// upsert).
func Test_FreyaProfile_FromJSON_ParseError(t *testing.T) {
	_, err := FreyaProfileFromJSON([]byte(`not json {`), "k", "o", "")
	if err == nil {
		t.Error("parse junk: want error, got nil")
	}
}
