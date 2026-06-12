package astload

import (
	"strings"
	"testing"
)

// r61.1 — DatasetManifest header line (freshness stamp). The export
// script writes it as the first JSONL row; LoadReader must decode it
// onto Corpus.Manifest and NOT count it as a card or emit a warning.
func TestLoadReader_DatasetManifest(t *testing.T) {
	const data = `{"__ast_type__": "DatasetManifest", "schema": 1, "export_unix": 1781250876, "export_date": "2026-06-11T23:34:36-0700", "parser_git_sha": "abc123", "parser_source_hash": "84a4a58359795ca3deadbeef", "entry_count": 1}
{"name": "Test Card", "oracle_text": "Flying", "type_line": "Creature", "mana_cost": "{1}", "cmc": 1.0, "colors": [], "ast": {"__ast_type__": "CardAST", "name": "Test Card", "abilities": [{"__ast_type__": "Keyword", "name": "flying", "args": [], "raw": "flying"}], "parse_errors": [], "fully_parsed": true}}
`
	c, err := LoadReader(strings.NewReader(data))
	if err != nil {
		t.Fatalf("LoadReader: %v", err)
	}
	if c.Manifest == nil {
		t.Fatalf("Manifest not decoded from header line")
	}
	if c.Manifest.Schema != 1 || c.Manifest.EntryCount != 1 {
		t.Errorf("Manifest fields wrong: %+v", c.Manifest)
	}
	if c.Manifest.ParserSourceHash != "84a4a58359795ca3deadbeef" {
		t.Errorf("ParserSourceHash = %q", c.Manifest.ParserSourceHash)
	}
	if c.CardCount != 1 {
		t.Errorf("manifest line must not count as a card; CardCount = %d", c.CardCount)
	}
	if len(c.ParseWarnings) != 0 {
		t.Errorf("manifest line must not warn: %v", c.ParseWarnings)
	}
	if _, ok := c.Get("Test Card"); !ok {
		t.Errorf("card after manifest line not loaded")
	}
}

// Pre-manifest datasets (no header line) must keep loading with
// Manifest == nil — the loader is backward compatible both ways.
func TestLoadReader_NoManifest(t *testing.T) {
	const data = `{"name": "Test Card", "oracle_text": "", "type_line": "Instant", "mana_cost": "{0}", "cmc": 0.0, "colors": [], "ast": {"__ast_type__": "CardAST", "name": "Test Card", "abilities": [], "parse_errors": [], "fully_parsed": true}}
`
	c, err := LoadReader(strings.NewReader(data))
	if err != nil {
		t.Fatalf("LoadReader: %v", err)
	}
	if c.Manifest != nil {
		t.Errorf("expected nil Manifest for pre-manifest dataset, got %+v", c.Manifest)
	}
	if c.CardCount != 1 {
		t.Errorf("CardCount = %d, want 1", c.CardCount)
	}
}
