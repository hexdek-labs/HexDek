package astload

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r140 — the exporter binds a "this ability triggers only once each turn"
// clause to the Triggered ability it limits instead of emitting a detached
// sibling Static whose "this ability" has no referent. decodeTriggered is
// field-by-field, so the field has to be read explicitly or the exporter is
// writing into a store nothing loads.
//
// The fixture is the real shape the exporter produces, read off the r140
// export of the live corpus: Kambal's class, a Triggered carrying
// timing_restriction and NO sibling Static.
const r140Row = `{"name":"R140 Limited Trigger","ast":{"__ast_type__":"CardAST",` +
	`"name":"R140 Limited Trigger","abilities":[` +
	`{"__ast_type__":"Triggered","trigger":{"__ast_type__":"Trigger","event":"attacks"},` +
	`"effect":null,"intervening_if":null,"timing_restriction":"once_per_turn",` +
	`"raw":"whenever this creature attacks, create a treasure token"}],` +
	`"parse_errors":[],"fully_parsed":true,"faces":[]}}`

const r140Unrestricted = `{"name":"R140 Open Trigger","ast":{"__ast_type__":"CardAST",` +
	`"name":"R140 Open Trigger","abilities":[` +
	`{"__ast_type__":"Triggered","trigger":{"__ast_type__":"Trigger","event":"attacks"},` +
	`"effect":null,"intervening_if":null,"timing_restriction":null,` +
	`"raw":"whenever this creature attacks, draw a card"}],` +
	`"parse_errors":[],"fully_parsed":true,"faces":[]}}`

func r140Load(t *testing.T, rows ...string) *Corpus {
	t.Helper()
	path := filepath.Join(t.TempDir(), "r140.jsonl")
	var body string
	for _, r := range rows {
		body += r + "\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return c
}

func r140Trigger(t *testing.T, c *Corpus, name string) *gameast.Triggered {
	t.Helper()
	card, ok := c.Get(name)
	if !ok || card == nil {
		t.Fatalf("card %q not in corpus", name)
	}
	for _, ab := range card.Abilities {
		if tr, ok := ab.(*gameast.Triggered); ok {
			return tr
		}
	}
	t.Fatalf("card %q has no Triggered ability", name)
	return nil
}

func TestR140_TriggeredTimingRestrictionSurvivesTheLoader(t *testing.T) {
	c := r140Load(t, r140Row)
	tr := r140Trigger(t, c, "R140 Limited Trigger")
	if tr.TimingRestriction != "once_per_turn" {
		t.Fatalf("the restriction was dropped on load: got %q, want %q "+
			"— the exporter would be writing into a store nothing reads",
			tr.TimingRestriction, "once_per_turn")
	}
}

func TestR140_AnUnrestrictedTriggerStaysUnrestricted(t *testing.T) {
	// The other direction: the loader must not invent a restriction, and a
	// null must arrive as empty rather than as some placeholder.
	c := r140Load(t, r140Unrestricted)
	tr := r140Trigger(t, c, "R140 Open Trigger")
	if tr.TimingRestriction != "" {
		t.Fatalf("an unrestricted trigger came back restricted: %q", tr.TimingRestriction)
	}
}

func TestR140_TheRestrictionIsNotASiblingStaticAnyMore(t *testing.T) {
	// Pins the shape, not just the field: a bound restriction means the
	// detached Static is gone. If a later exporter change emitted both, the
	// engine would be at risk of applying the limiter twice.
	c := r140Load(t, r140Row)
	card, ok := c.Get("R140 Limited Trigger")
	if !ok || card == nil {
		t.Fatal("card missing")
	}
	for _, ab := range card.Abilities {
		st, ok := ab.(*gameast.Static)
		if !ok || st.Modification == nil {
			continue
		}
		if st.Modification.ModKind == "timing_restriction" {
			t.Fatalf("a detached timing_restriction Static survived alongside "+
				"the bound field: %+v", st.Modification)
		}
	}
}
