package analytics

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/seedcontract"
	"github.com/hexdek/hexdek/internal/validation"
)

// Consolidation step 2 — analytics consumes the structured loss
// category from the elimination event instead of substring-parsing the
// freeform reason, and stamps the canonical method alongside the
// finer-grained one.

// TestExtractKillRecords_StructuredCategory pins the structured path: a
// freeform reason that the substring sniffing CANNOT classify ("CR
// 704.6c" without the magic "21+ commander damage from" prefix) still
// classifies via loss_category.
func TestExtractKillRecords_StructuredCategory(t *testing.T) {
	commanders := []string{"Alesha", "Bob"}
	events := []gameengine.Event{
		{Kind: "damage", Seat: 0, Target: 1, Source: "Alesha, Who Smiles at Death", Amount: 21,
			Details: map[string]interface{}{"commander": true, "combat": true}},
		{Kind: "seat_eliminated", Seat: 1,
			Details: map[string]interface{}{
				"reason":           "eliminated (CR 704.6c)", // unparseable by the legacy substrings
				"loss_category":    validation.LossCategoryCommanderDamage,
				"loss_source_card": "Alesha, Who Smiles at Death",
				"turn":             9,
			}},
	}
	records := ExtractKillRecords(events, 2, commanders, 0, "g-s2")
	if len(records) != 1 {
		t.Fatalf("expected 1 kill via structured category, got %d", len(records))
	}
	r := records[0]
	if r.Method != "commander_damage" {
		t.Errorf("Method = %q, want commander_damage", r.Method)
	}
	if r.MethodCanonical != "commander" {
		t.Errorf("MethodCanonical = %q, want commander", r.MethodCanonical)
	}
	if r.LethalCard != "Alesha, Who Smiles at Death" {
		t.Errorf("LethalCard = %q", r.LethalCard)
	}
}

// TestExtractKillRecords_StructuredConcession pins that a structured
// concession produces no kill record even when the freeform reason
// doesn't contain the word "concession".
func TestExtractKillRecords_StructuredConcession(t *testing.T) {
	commanders := []string{"A", "B"}
	events := []gameengine.Event{
		{Kind: "damage", Seat: 0, Target: 1, Source: "Bolt", Amount: 3, Details: map[string]interface{}{}},
		{Kind: "seat_eliminated", Seat: 1,
			Details: map[string]interface{}{
				"reason":        "player left",
				"loss_category": validation.LossCategoryConcession,
			}},
	}
	if records := ExtractKillRecords(events, 2, commanders, 0, ""); len(records) != 0 {
		t.Errorf("structured concession should produce no kill record, got %d", len(records))
	}
}

// TestExtractKillRecords_MethodCanonicalStamped pins the canonical
// sibling on the legacy (string-parsed) path too.
func TestExtractKillRecords_MethodCanonicalStamped(t *testing.T) {
	commanders := []string{"A", "B"}
	events := []gameengine.Event{
		{Kind: "damage", Seat: 0, Target: 1, Source: "Lightning Bolt", Amount: 40,
			Details: map[string]interface{}{"combat": false}},
		{Kind: "seat_eliminated", Seat: 1,
			Details: map[string]interface{}{"reason": "life total 0"}},
	}
	records := ExtractKillRecords(events, 2, commanders, 0, "")
	if len(records) != 1 {
		t.Fatalf("expected 1, got %d", len(records))
	}
	if records[0].Method != "noncombat_damage" {
		t.Errorf("Method = %q, want noncombat_damage (legacy path unchanged)", records[0].Method)
	}
	if records[0].MethodCanonical != "combat" {
		t.Errorf("MethodCanonical = %q, want combat", records[0].MethodCanonical)
	}
	if got := seedcontract.CanonicalKillMethod(records[0].Method); got != records[0].MethodCanonical {
		t.Errorf("MethodCanonical must equal CanonicalKillMethod(Method); %q != %q", records[0].MethodCanonical, got)
	}
}
