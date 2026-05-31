package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// commander_check_test.go — pins the CR §903 commander legality probe.
// Each test case writes a synthetic oracle JSON + deck text pair to a
// temp dir, runs runCommanderCheck, and inspects the structured
// report. Covers §903.5a (legendary creature), §903.5b ("can be your
// commander" text), §903.6 (Scryfall commander-format legality),
// §903.4 (color identity vs deck), and the banned-list scan.
//
// The oracle JSON is hand-rolled so the tests don't depend on the
// 163MB real corpus — each fixture declares only the cards relevant
// to the case. This is deliberately at the integration level (full
// runCommanderCheck pipeline, real JSON round-trip) so a regression
// in any of parseDeckFile / loadOracleCmdrDB / classifyCommander /
// classifyFormatLegal / validateDeckColorIdentity / validateBannedCards
// surfaces immediately.

// writeFixture creates a temp dir containing oracle.json + deck.txt
// with the supplied content, and returns the two paths.
func writeFixture(t *testing.T, oracleJSON, deckText string) (oraclePath, deckPath string) {
	t.Helper()
	tmp := t.TempDir()
	oraclePath = filepath.Join(tmp, "oracle.json")
	deckPath = filepath.Join(tmp, "deck.txt")
	if err := os.WriteFile(oraclePath, []byte(oracleJSON), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(deckPath, []byte(deckText), 0644); err != nil {
		t.Fatal(err)
	}
	return
}

// runCheck is the common entry point for tests.
func runCheck(t *testing.T, oracleJSON, deckText string) *CommanderReport {
	t.Helper()
	oraclePath, deckPath := writeFixture(t, oracleJSON, deckText)
	rep, err := runCommanderCheck(deckPath, oraclePath, "")
	if err != nil {
		t.Fatalf("runCommanderCheck: %v", err)
	}
	return rep
}

// ---------------------------------------------------------------------------
// CR §903.5a — legendary creature commander, monocolor — happy path
// ---------------------------------------------------------------------------

func TestCommander_LegendaryCreature_MonoColor_Valid(t *testing.T) {
	oracle := `[
		{"name":"Krenko, Mob Boss","type_line":"Legendary Creature — Goblin Warrior","oracle_text":"Tap: Create X 1/1 red Goblin creature tokens.","color_identity":["R"],"legalities":{"commander":"legal"}},
		{"name":"Sol Ring","type_line":"Artifact","oracle_text":"Tap: Add {C}{C}.","color_identity":[],"legalities":{"commander":"legal"}},
		{"name":"Lightning Bolt","type_line":"Instant","oracle_text":"Bolt deals 3 damage.","color_identity":["R"],"legalities":{"commander":"legal"}}
	]`
	deck := "COMMANDER: Krenko, Mob Boss\n1 Sol Ring\n1 Lightning Bolt\n"
	rep := runCheck(t, oracle, deck)
	if !rep.Valid {
		t.Fatalf("expected Valid=true, got false; report: %+v", rep)
	}
	if !rep.Checks.CommanderLegality.Valid {
		t.Errorf("CommanderLegality.Valid = false")
	}
	if rep.Checks.CommanderLegality.PerCmdr[0].Rule != "CR §903.5a" {
		t.Errorf("Rule = %q, want CR §903.5a", rep.Checks.CommanderLegality.PerCmdr[0].Rule)
	}
	if len(rep.CommanderColorIdentity) != 1 || rep.CommanderColorIdentity[0] != "R" {
		t.Errorf("CommanderColorIdentity = %v, want [R]", rep.CommanderColorIdentity)
	}
}

// ---------------------------------------------------------------------------
// CR §903.5b — planeswalker commander with "can be your commander"
// ---------------------------------------------------------------------------

func TestCommander_PlaneswalkerWithDesignation_Valid(t *testing.T) {
	oracle := `[
		{"name":"Saheeli, the Gifted","type_line":"Legendary Planeswalker — Saheeli","oracle_text":"Saheeli, the Gifted can be your commander.\n+1: Create a 1/1 colorless Servo artifact creature token.","color_identity":["R","U"],"legalities":{"commander":"legal"}},
		{"name":"Sol Ring","type_line":"Artifact","oracle_text":"","color_identity":[],"legalities":{"commander":"legal"}}
	]`
	deck := "COMMANDER: Saheeli, the Gifted\n1 Sol Ring\n"
	rep := runCheck(t, oracle, deck)
	if !rep.Valid {
		t.Fatalf("expected Valid=true, got false; report: %+v", rep)
	}
	if rep.Checks.CommanderLegality.PerCmdr[0].Rule != "CR §903.5b" {
		t.Errorf("Rule = %q, want CR §903.5b", rep.Checks.CommanderLegality.PerCmdr[0].Rule)
	}
}

// ---------------------------------------------------------------------------
// Failing: planeswalker WITHOUT "can be your commander"
// ---------------------------------------------------------------------------

func TestCommander_PlainPlaneswalker_Invalid(t *testing.T) {
	oracle := `[
		{"name":"Jace, the Mind Sculptor","type_line":"Legendary Planeswalker — Jace","oracle_text":"+2: Look at the top card of target player's library.","color_identity":["U"],"legalities":{"commander":"banned"}}
	]`
	deck := "COMMANDER: Jace, the Mind Sculptor\n"
	rep := runCheck(t, oracle, deck)
	if rep.Valid {
		t.Fatal("expected Valid=false for plain planeswalker (no commander designation + banned)")
	}
	v := rep.Checks.CommanderLegality.PerCmdr[0]
	if v.Valid {
		t.Errorf("CommanderLegality verdict.Valid = true, want false")
	}
	if !strings.Contains(v.Reason, "not a legendary creature") {
		t.Errorf("reason = %q, want substring 'not a legendary creature'", v.Reason)
	}
	// Also format-legal fails — Jace TMS is banned in Commander.
	if rep.Checks.CommanderFormatLegal.Valid {
		t.Errorf("CommanderFormatLegal.Valid = true, want false")
	}
	if rep.Checks.CommanderFormatLegal.PerCmdr[0].ScryfallStatus != "banned" {
		t.Errorf("ScryfallStatus = %q, want banned", rep.Checks.CommanderFormatLegal.PerCmdr[0].ScryfallStatus)
	}
}

// ---------------------------------------------------------------------------
// Failing: non-legendary creature commander
// ---------------------------------------------------------------------------

func TestCommander_NonLegendaryCreature_Invalid(t *testing.T) {
	oracle := `[
		{"name":"Grizzly Bears","type_line":"Creature — Bear","oracle_text":"","color_identity":["G"],"legalities":{"commander":"legal"}}
	]`
	deck := "COMMANDER: Grizzly Bears\n"
	rep := runCheck(t, oracle, deck)
	if rep.Valid {
		t.Fatal("expected Valid=false for non-legendary creature commander")
	}
	if rep.Checks.CommanderLegality.PerCmdr[0].Valid {
		t.Errorf("verdict.Valid = true, want false")
	}
}

// ---------------------------------------------------------------------------
// Failing: commander not found in oracle
// ---------------------------------------------------------------------------

func TestCommander_NotFoundInOracle_Invalid(t *testing.T) {
	oracle := `[
		{"name":"Sol Ring","type_line":"Artifact","oracle_text":"","color_identity":[],"legalities":{"commander":"legal"}}
	]`
	deck := "COMMANDER: Mxyzptlk, the Imp\n1 Sol Ring\n"
	rep := runCheck(t, oracle, deck)
	if rep.Valid {
		t.Fatal("expected Valid=false for unknown commander")
	}
	v := rep.Checks.CommanderLegality.PerCmdr[0]
	if !strings.Contains(v.Reason, "not found in oracle") {
		t.Errorf("reason = %q, want 'not found in oracle'", v.Reason)
	}
}

// ---------------------------------------------------------------------------
// CR §903.4 — color identity violation
// ---------------------------------------------------------------------------

func TestCommander_ColorIdentityViolation_Invalid(t *testing.T) {
	oracle := `[
		{"name":"Krenko, Mob Boss","type_line":"Legendary Creature — Goblin Warrior","oracle_text":"","color_identity":["R"],"legalities":{"commander":"legal"}},
		{"name":"Sol Ring","type_line":"Artifact","oracle_text":"","color_identity":[],"legalities":{"commander":"legal"}},
		{"name":"Counterspell","type_line":"Instant","oracle_text":"Counter target spell.","color_identity":["U"],"legalities":{"commander":"legal"}}
	]`
	deck := "COMMANDER: Krenko, Mob Boss\n1 Sol Ring\n1 Counterspell\n"
	rep := runCheck(t, oracle, deck)
	if rep.Valid {
		t.Fatal("expected Valid=false: Counterspell (U) in a mono-R Krenko deck")
	}
	if rep.Checks.ColorIdentity.Valid {
		t.Errorf("ColorIdentity.Valid = true, want false")
	}
	if len(rep.Checks.ColorIdentity.Violations) != 1 {
		t.Fatalf("Violations = %d, want 1", len(rep.Checks.ColorIdentity.Violations))
	}
	v := rep.Checks.ColorIdentity.Violations[0]
	if v.CardName != "Counterspell" {
		t.Errorf("CardName = %q, want Counterspell", v.CardName)
	}
	if len(v.OutOfIdentity) != 1 || v.OutOfIdentity[0] != "U" {
		t.Errorf("OutOfIdentity = %v, want [U]", v.OutOfIdentity)
	}
}

// ---------------------------------------------------------------------------
// CR §903.4 — partner color-identity UNION across two commanders
// ---------------------------------------------------------------------------

func TestCommander_PartnerColorIdentityUnion_Valid(t *testing.T) {
	oracle := `[
		{"name":"Akiri, Line-Slinger","type_line":"Legendary Creature — Kor Soldier","oracle_text":"Partner","color_identity":["R","W"],"legalities":{"commander":"legal"}},
		{"name":"Silas Renn, Seeker Adept","type_line":"Legendary Creature — Human Artificer","oracle_text":"Partner","color_identity":["U","B"],"legalities":{"commander":"legal"}},
		{"name":"Esper Sentinel","type_line":"Artifact Creature — Construct","oracle_text":"","color_identity":["W"],"legalities":{"commander":"legal"}},
		{"name":"Goblin Welder","type_line":"Creature — Goblin Artificer","oracle_text":"","color_identity":["R"],"legalities":{"commander":"legal"}},
		{"name":"Notion Thief","type_line":"Creature — Human Rogue","oracle_text":"","color_identity":["U","B"],"legalities":{"commander":"legal"}}
	]`
	deck := "COMMANDER: Akiri, Line-Slinger\nCOMMANDER: Silas Renn, Seeker Adept\n1 Esper Sentinel\n1 Goblin Welder\n1 Notion Thief\n"
	rep := runCheck(t, oracle, deck)
	if !rep.Valid {
		t.Fatalf("expected Valid=true: union {W,U,B,R} covers every deck card; got %+v", rep.Checks.ColorIdentity)
	}
	// Allowed identity should be {W,U,B,R} in WUBRG order.
	wantAllowed := []string{"W", "U", "B", "R"}
	if !equalStringSlice(rep.CommanderColorIdentity, wantAllowed) {
		t.Errorf("CommanderColorIdentity = %v, want %v", rep.CommanderColorIdentity, wantAllowed)
	}
}

// ---------------------------------------------------------------------------
// Banned list — banned commander
// ---------------------------------------------------------------------------

func TestCommander_BannedCommander_Invalid(t *testing.T) {
	oracle := `[
		{"name":"Golos, Tireless Pilgrim","type_line":"Legendary Artifact Creature — Scout","oracle_text":"When Golos enters, you may search...","color_identity":["W","U","B","R","G"],"legalities":{"commander":"banned"}},
		{"name":"Sol Ring","type_line":"Artifact","oracle_text":"","color_identity":[],"legalities":{"commander":"legal"}}
	]`
	deck := "COMMANDER: Golos, Tireless Pilgrim\n1 Sol Ring\n"
	rep := runCheck(t, oracle, deck)
	if rep.Valid {
		t.Fatal("expected Valid=false: Golos is banned as a commander")
	}
	if rep.Checks.BannedCards.Valid {
		t.Errorf("BannedCards.Valid = true, want false")
	}
	if len(rep.Checks.BannedCards.Violations) != 1 {
		t.Fatalf("Violations = %d, want 1", len(rep.Checks.BannedCards.Violations))
	}
	v := rep.Checks.BannedCards.Violations[0]
	if v.Where != "commander" {
		t.Errorf("Where = %q, want commander", v.Where)
	}
}

// ---------------------------------------------------------------------------
// Banned list — banned mainboard card under a legal commander
// ---------------------------------------------------------------------------

func TestCommander_BannedMainboardCard_Invalid(t *testing.T) {
	oracle := `[
		{"name":"Krenko, Mob Boss","type_line":"Legendary Creature — Goblin Warrior","oracle_text":"","color_identity":["R"],"legalities":{"commander":"legal"}},
		{"name":"Mana Crypt","type_line":"Artifact","oracle_text":"At the beginning of your upkeep...","color_identity":[],"legalities":{"commander":"banned"}},
		{"name":"Sol Ring","type_line":"Artifact","oracle_text":"","color_identity":[],"legalities":{"commander":"legal"}}
	]`
	deck := "COMMANDER: Krenko, Mob Boss\n1 Mana Crypt\n1 Sol Ring\n"
	rep := runCheck(t, oracle, deck)
	if rep.Valid {
		t.Fatal("expected Valid=false: Mana Crypt is on the banned list")
	}
	if rep.Checks.BannedCards.Valid {
		t.Errorf("BannedCards.Valid = true, want false")
	}
	if len(rep.Checks.BannedCards.Violations) != 1 {
		t.Fatalf("Violations = %d, want 1: %+v", len(rep.Checks.BannedCards.Violations), rep.Checks.BannedCards.Violations)
	}
	v := rep.Checks.BannedCards.Violations[0]
	if v.Where != "mainboard" || !strings.EqualFold(v.CardName, "Mana Crypt") {
		t.Errorf("violation = %+v, want Mana Crypt in mainboard", v)
	}
}

// ---------------------------------------------------------------------------
// No COMMANDER: directive in deck file
// ---------------------------------------------------------------------------

func TestCommander_NoCommanderDirective_Invalid(t *testing.T) {
	oracle := `[
		{"name":"Sol Ring","type_line":"Artifact","oracle_text":"","color_identity":[],"legalities":{"commander":"legal"}}
	]`
	// Note: no COMMANDER: line at all
	deck := "# Some deck\n1 Sol Ring\n"
	rep := runCheck(t, oracle, deck)
	if rep.Valid {
		t.Fatal("expected Valid=false when no COMMANDER: directive present")
	}
	if len(rep.Commanders) != 0 {
		t.Errorf("Commanders = %v, want empty", rep.Commanders)
	}
	if rep.Checks.CommanderLegality.PerCmdr[0].Reason != "no COMMANDER: directive found in deck file" {
		t.Errorf("unexpected reason: %q", rep.Checks.CommanderLegality.PerCmdr[0].Reason)
	}
}

// ---------------------------------------------------------------------------
// Art-series / non-playable layout shadowing — caught while smoke-testing
// Edgar Markov against the real oracle corpus. Scryfall ships an
// `art_series` entry named "Edgar Markov // Edgar Markov" with
// type_line "Card // Card"; when indexed by face name, that overwrote
// the real Legendary Creature entry and the probe reported Edgar as
// "not a legendary creature." Fix: skip layout in
// {art_series, emblem, token, double_faced_token, scheme, plane,
// phenomenon, vanguard}. This test pins it by ordering the art_series
// entry FIRST in the fixture (no protection from the fix → indexes
// would shadow even if the real entry came later).
// ---------------------------------------------------------------------------

func TestCommander_ArtSeriesShadowing_RealEntryWins(t *testing.T) {
	oracle := `[
		{"name":"Edgar Markov // Edgar Markov","layout":"art_series","type_line":"Card // Card","oracle_text":"","color_identity":[],"legalities":{"commander":"not_legal"}},
		{"name":"Edgar Markov","layout":"normal","type_line":"Legendary Creature — Vampire Knight","oracle_text":"Whenever a Vampire enters, put a +1/+1 counter on it.","color_identity":["W","B","R"],"legalities":{"commander":"legal"}}
	]`
	deck := "COMMANDER: Edgar Markov\n"
	rep := runCheck(t, oracle, deck)
	if !rep.Valid {
		t.Fatalf("expected Valid=true (the art_series shadow must NOT win): %+v", rep)
	}
	v := rep.Checks.CommanderLegality.PerCmdr[0]
	if !v.Valid {
		t.Errorf("CommanderLegality verdict.Valid = false; reason=%q (art_series shadow won)", v.Reason)
	}
	if v.Rule != "CR §903.5a" {
		t.Errorf("Rule = %q, want CR §903.5a", v.Rule)
	}
	wantCI := []string{"W", "B", "R"}
	if !equalStringSlice(rep.CommanderColorIdentity, wantCI) {
		t.Errorf("CommanderColorIdentity = %v, want %v (art_series CI shadowed real CI?)",
			rep.CommanderColorIdentity, wantCI)
	}
}

// ---------------------------------------------------------------------------
// JSON output schema lock
// ---------------------------------------------------------------------------

func TestCommander_ReportJSONShape(t *testing.T) {
	oracle := `[
		{"name":"Krenko, Mob Boss","type_line":"Legendary Creature — Goblin Warrior","oracle_text":"","color_identity":["R"],"legalities":{"commander":"legal"}}
	]`
	deck := "COMMANDER: Krenko, Mob Boss\n"
	oraclePath, deckPath := writeFixture(t, oracle, deck)
	tmpOut := filepath.Join(filepath.Dir(deckPath), "report.json")
	if _, err := runCommanderCheck(deckPath, oraclePath, tmpOut); err != nil {
		t.Fatalf("runCommanderCheck: %v", err)
	}
	raw, err := os.ReadFile(tmpOut)
	if err != nil {
		t.Fatal(err)
	}
	// Required top-level keys.
	for _, key := range []string{
		`"rule"`, `"deck_path"`, `"commanders"`, `"commander_color_identity"`,
		`"deck_card_count"`, `"checks"`, `"valid"`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("missing top-level key %s in:\n%s", key, raw)
		}
	}
	// Required check sections.
	for _, key := range []string{
		`"commander_legality"`, `"commander_format_legal"`,
		`"color_identity"`, `"banned_cards"`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("missing check section %s in:\n%s", key, raw)
		}
	}
	// Round-trip sanity.
	var rt CommanderReport
	if err := json.Unmarshal(raw, &rt); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}
	if rt.Rule != "CR §903" {
		t.Errorf("Rule = %q, want CR §903", rt.Rule)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
