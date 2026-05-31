// Command gen_spellbook reads a Commander Spellbook variants JSON (either the
// full upstream bulk dump or a curated seed file) and emits a generated Go
// source file containing the parsed, intra-deduped imports as compile-time
// literals.
//
// The generated file lives at cmd/hexdek-freya/spellbook_generated.go and is
// loaded automatically by Freya at startup (no --spellbook flag needed). The
// runtime --spellbook cache path still works and is merged on top of the
// generated baseline.
//
// Usage:
//
//	go run ./cmd/hexdek-freya/gen_spellbook \
//	    -in cmd/hexdek-freya/testdata/spellbook_generator_seed.json \
//	    -out cmd/hexdek-freya/spellbook_generated.go
//
// The parser logic mirrors ParseSpellbookJSON in cmd/hexdek-freya/spellbook_import.go.
// Kept deliberately self-contained so the generator does not depend on the
// hexdek-freya package (which cannot be imported by another main package).
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
)

type spellbookFile struct {
	Variants []spellbookVariant `json:"variants"`
}

type spellbookVariant struct {
	ID          string             `json:"id"`
	Uses        []spellbookUse     `json:"uses"`
	Produces    []spellbookProduce `json:"produces"`
	Description string             `json:"description"`
	Identity    string             `json:"identity"`
	Status      string             `json:"status"`
}

type spellbookUse struct {
	Card     spellbookCardRef `json:"card"`
	Quantity int              `json:"quantity"`
}

type spellbookCardRef struct {
	Name string `json:"name"`
}

type spellbookProduce struct {
	Feature spellbookFeature `json:"feature"`
}

type spellbookFeature struct {
	Name string `json:"name"`
}

type combo struct {
	Name        string
	Pieces      []string
	Type        string
	Mandatory   bool
	Description string
}

func statusOK(s string) bool {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "", "OK", "EXAMPLE":
		return true
	}
	return false
}

func inferType(produces []spellbookProduce) string {
	for _, p := range produces {
		n := strings.ToLower(p.Feature.Name)
		if strings.Contains(n, "win the game") || strings.Contains(n, "lose the game") {
			return "true_infinite"
		}
	}
	for _, p := range produces {
		n := strings.ToLower(p.Feature.Name)
		if strings.Contains(n, "infinite") {
			return "true_infinite"
		}
	}
	return "determined"
}

func canonicalKey(pieces []string) string {
	norm := make([]string, 0, len(pieces))
	for _, p := range pieces {
		n := strings.ToLower(strings.TrimSpace(p))
		if n == "" {
			continue
		}
		norm = append(norm, n)
	}
	sort.Strings(norm)
	return strings.Join(norm, "|")
}

func comboName(pieces []string, id string) string {
	if len(pieces) <= 3 {
		return strings.Join(pieces, " + ")
	}
	return fmt.Sprintf("%s + %d more (Spellbook %s)", pieces[0], len(pieces)-1, id)
}

func descriptionText(v spellbookVariant) string {
	desc := strings.TrimSpace(v.Description)
	var features []string
	for _, p := range v.Produces {
		if n := strings.TrimSpace(p.Feature.Name); n != "" {
			features = append(features, n)
		}
	}
	header := "Imported from Commander Spellbook"
	if v.ID != "" {
		header += " (" + v.ID + ")"
	}
	if len(features) > 0 {
		header += " — produces: " + strings.Join(features, ", ")
	}
	if desc == "" {
		return header
	}
	return header + ". " + desc
}

func parse(raw []byte) ([]combo, error) {
	var file spellbookFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, err
	}
	out := make([]combo, 0, len(file.Variants))
	seen := map[string]bool{}
	for _, v := range file.Variants {
		if !statusOK(v.Status) {
			continue
		}
		var pieces []string
		for _, u := range v.Uses {
			name := strings.TrimSpace(u.Card.Name)
			if name == "" {
				continue
			}
			pieces = append(pieces, name)
		}
		if len(pieces) < 2 {
			continue
		}
		key := canonicalKey(pieces)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, combo{
			Name:        comboName(pieces, v.ID),
			Pieces:      pieces,
			Type:        inferType(v.Produces),
			Mandatory:   true,
			Description: descriptionText(v),
		})
	}
	// Stable output order so generated diffs are reviewable.
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func main() {
	var in, out, sourceTag string
	flag.StringVar(&in, "in", "cmd/hexdek-freya/testdata/spellbook_generator_seed.json",
		"path to Commander Spellbook variants JSON")
	flag.StringVar(&out, "out", "cmd/hexdek-freya/spellbook_generated.go",
		"path to write generated Go file")
	flag.StringVar(&sourceTag, "source", "testdata/spellbook_generator_seed.json",
		"label recorded in the generated file's header comment")
	flag.Parse()

	raw, err := os.ReadFile(in)
	if err != nil {
		log.Fatalf("read %s: %v", in, err)
	}
	combos, err := parse(raw)
	if err != nil {
		log.Fatalf("parse %s: %v", in, err)
	}
	if len(combos) == 0 {
		log.Fatalf("no combos parsed from %s; refusing to overwrite %s with empty set", in, out)
	}

	src, err := render(combos, sourceTag, len(combos))
	if err != nil {
		log.Fatalf("render: %v", err)
	}

	if err := os.WriteFile(out, src, 0o644); err != nil {
		log.Fatalf("write %s: %v", out, err)
	}
	log.Printf("wrote %d combos to %s", len(combos), out)
}

func render(combos []combo, sourceTag string, count int) ([]byte, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "// Code generated by cmd/hexdek-freya/gen_spellbook; DO NOT EDIT.\n")
	fmt.Fprintf(&buf, "// Source: %s (%d combos)\n", sourceTag, count)
	fmt.Fprintf(&buf, "// Regenerate with: go run ./cmd/hexdek-freya/gen_spellbook -in <path> -out cmd/hexdek-freya/spellbook_generated.go\n\n")
	fmt.Fprintf(&buf, "package main\n\n")
	fmt.Fprintf(&buf, "// generatedSpellbookCombos is the deduped-on-card-set Commander Spellbook\n")
	fmt.Fprintf(&buf, "// import baseline shipped in the repo. Freya merges this on top of the\n")
	fmt.Fprintf(&buf, "// hand-curated KnownCombos via allKnownCombos(); the runtime --spellbook\n")
	fmt.Fprintf(&buf, "// cache layers further imports on top.\n")
	fmt.Fprintf(&buf, "var generatedSpellbookCombos = []KnownCombo{\n")
	for _, c := range combos {
		fmt.Fprintf(&buf, "\t{\n")
		fmt.Fprintf(&buf, "\t\tName: %s,\n", strconv.Quote(c.Name))
		fmt.Fprintf(&buf, "\t\tPieces: []string{")
		for i, p := range c.Pieces {
			if i > 0 {
				fmt.Fprintf(&buf, ", ")
			}
			fmt.Fprintf(&buf, "%s", strconv.Quote(p))
		}
		fmt.Fprintf(&buf, "},\n")
		fmt.Fprintf(&buf, "\t\tType: %s,\n", strconv.Quote(c.Type))
		fmt.Fprintf(&buf, "\t\tMandatory: %t,\n", c.Mandatory)
		fmt.Fprintf(&buf, "\t\tDescription: %s,\n", strconv.Quote(c.Description))
		fmt.Fprintf(&buf, "\t},\n")
	}
	fmt.Fprintf(&buf, "}\n")
	return format.Source(buf.Bytes())
}
