package hat

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r174 corpus differential.
//
// Runs the OLD expression and the NEW one over every card in a real
// Scryfall bulk dump and asserts zero disagreements. Skipped unless
// HEXDEK_R174_CORPUS points at a decompressed oracle-cards JSONL
// (Scryfall now ships gzipped JSONL, not a JSON array), because
// data/rules/oracle-cards.json is gitignored and absent on dev boxes.
//
//	curl -sL "$(curl -s https://api.scryfall.com/bulk-data |
//	    jq -r '.data[]|select(.type=="oracle_cards").jsonl_download_uri')" |
//	  gunzip > /tmp/oracle.jsonl
//	HEXDEK_R174_CORPUS=/tmp/oracle.jsonl go test ./internal/hat/ -run R174Corpus -v
//
// The needle set is every literal passed to typeLineContains in
// production, harvested by grep, plus the "tutor" probe in
// handQualityBonus.
var r174Needles = []string{
	"creature", "land", "artifact", "enchantment", "instant", "sorcery",
	"planeswalker", "commander", "equipment", "aura", "tutor",
}

type r174Card struct {
	Name     string `json:"name"`
	TypeLine string `json:"type_line"`
}

func r174LoadCorpus(t *testing.T) []r174Card {
	t.Helper()
	path := os.Getenv("HEXDEK_R174_CORPUS")
	if path == "" {
		t.Skip("HEXDEK_R174_CORPUS unset; see file header for how to fetch a bulk dump")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open corpus: %v", err)
	}
	defer f.Close()

	var out []r174Card
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 8<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line == "[" || line == "]" {
			continue
		}
		line = strings.TrimSuffix(line, ",")
		var c r174Card
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			continue
		}
		if c.Name == "" {
			continue
		}
		out = append(out, c)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan corpus: %v", err)
	}
	if len(out) < 10000 {
		t.Fatalf("corpus looks truncated: %d cards", len(out))
	}
	t.Logf("corpus: %d cards", len(out))
	return out
}

// parseTypesLikeDeckparser mirrors deckparser.parseTypes (unexported
// there) so the differential sees the exact type tokens the engine
// puts on Card.Types.
func parseTypesLikeDeckparser(typeLine string) []string {
	if typeLine == "" {
		return nil
	}
	normalized := strings.ReplaceAll(typeLine, "—", "-")
	out := []string{}
	for _, f := range strings.Fields(normalized) {
		f = strings.TrimSpace(f)
		if f == "" || f == "-" {
			continue
		}
		out = append(out, strings.ToLower(f))
	}
	return out
}

// Differential 1 — intByFoldedName vs m[strings.ToLower(name)] on both
// curated maps, over every real card name.
func TestR174Corpus_IntByFoldedName(t *testing.T) {
	cards := r174LoadCorpus(t)
	maps := []struct {
		label string
		m     map[string]int
	}{
		{"fastManaExtraYieldByName", fastManaExtraYieldByName},
		{"ritualNetByName", ritualNetByName},
	}
	disagree, hits := 0, 0
	for _, c := range cards {
		for _, mm := range maps {
			want := mm.m[strings.ToLower(c.Name)]
			got := intByFoldedName(mm.m, c.Name)
			if want != 0 {
				hits++
			}
			if got != want {
				disagree++
				if disagree <= 10 {
					t.Errorf("%s[%q]: new=%d old=%d", mm.label, c.Name, got, want)
				}
			}
		}
	}
	t.Logf("names checked=%d  map hits=%d  disagreements=%d", len(cards), hits, disagree)
	if disagree != 0 {
		t.Fatalf("%d disagreements", disagree)
	}
	if hits == 0 {
		t.Fatal("no card in the corpus hit either curated map — differential proved nothing")
	}
}

// Differential 2 — typeLineContains, new fold vs the old
// strings.Contains(strings.ToLower(t), strings.ToLower(substr)), over
// every real type line crossed with every production needle. Run
// against the lowercase tokens the engine builds AND against the raw
// mixed-case words, so the mixed-case branch is exercised too.
func TestR174Corpus_TypeLineContains(t *testing.T) {
	cards := r174LoadCorpus(t)

	oldTypeLineContains := func(card *gameengine.Card, substr string) bool {
		if card == nil {
			return false
		}
		ss := strings.ToLower(substr)
		for _, tt := range card.Types {
			if strings.Contains(strings.ToLower(tt), ss) {
				return true
			}
		}
		return false
	}

	disagree, trues, checks := 0, 0, 0
	for _, c := range cards {
		variants := [][]string{
			parseTypesLikeDeckparser(c.TypeLine),
			strings.Fields(strings.ReplaceAll(c.TypeLine, "—", "-")), // raw, mixed case
		}
		for _, types := range variants {
			card := &gameengine.Card{Name: c.Name, Types: types}
			for _, n := range r174Needles {
				checks++
				want := oldTypeLineContains(card, n)
				got := typeLineContains(card, n)
				if want {
					trues++
				}
				if got != want {
					disagree++
					if disagree <= 10 {
						t.Errorf("typeLineContains(%q=%v, %q): new=%v old=%v",
							c.Name, types, n, got, want)
					}
				}
			}
		}
	}
	t.Logf("checks=%d  true answers=%d  disagreements=%d", checks, trues, disagree)
	if disagree != 0 {
		t.Fatalf("%d disagreements", disagree)
	}
	if trues == 0 {
		t.Fatal("no needle ever matched — differential proved nothing")
	}
}

// Differential 3 — the handQualityBonus tutor name probe.
func TestR174Corpus_TutorNameProbe(t *testing.T) {
	cards := r174LoadCorpus(t)
	disagree, trues := 0, 0
	for _, c := range cards {
		want := strings.Contains(strings.ToLower(c.Name), "tutor")
		got := foldContainsASCII(c.Name, "tutor")
		if want {
			trues++
		}
		if got != want {
			disagree++
			if disagree <= 10 {
				t.Errorf("tutor probe %q: new=%v old=%v", c.Name, got, want)
			}
		}
	}
	t.Logf("names=%d  tutor matches=%d  disagreements=%d", len(cards), trues, disagree)
	if disagree != 0 {
		t.Fatalf("%d disagreements", disagree)
	}
	if trues == 0 {
		t.Fatal("no card name matched \"tutor\" — differential proved nothing")
	}
}

// Differential 4 — gameengine.TypeLineLower vs the inline
// strings.ToLower(c.TypeLine) cardHeuristic used, over every real
// type line, including a second call to prove the memo returns the
// same answer it computed.
func TestR174Corpus_TypeLineLower(t *testing.T) {
	cards := r174LoadCorpus(t)
	disagree := 0
	for _, c := range cards {
		card := &gameengine.Card{Name: c.Name, TypeLine: c.TypeLine}
		want := strings.ToLower(c.TypeLine)
		if got := gameengine.TypeLineLower(card); got != want {
			disagree++
			if disagree <= 10 {
				t.Errorf("TypeLineLower(%q): new=%q old=%q", c.TypeLine, got, want)
			}
		}
		if got := gameengine.TypeLineLower(card); got != want {
			disagree++
			if disagree <= 10 {
				t.Errorf("TypeLineLower(%q) second (memoised) call: new=%q old=%q",
					c.TypeLine, got, want)
			}
		}
	}
	t.Logf("type lines=%d  disagreements=%d", len(cards), disagree)
	if disagree != 0 {
		t.Fatalf("%d disagreements", disagree)
	}
}
