// find_high_freq_uncovered: cross-references parser-coverage's uncovered
// cards (MISSING/EMPTY_AST/PARTIAL) with deck-frequency counts across
// data/decks/**/*.txt, printing the top N highest-frequency uncovered
// cards. One-off script used to seed the parser-coverage backlog;
// kept in scripts/ for reproducibility but not built by default.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/astload"
)

type oracleEntry struct {
	Name       string `json:"name"`
	Layout     string `json:"layout"`
	TypeLine   string `json:"type_line"`
	SetName    string `json:"set_name"`
	OracleText string `json:"oracle_text"`
	CardFaces  []struct {
		OracleText string `json:"oracle_text"`
	} `json:"card_faces"`
}

func main() {
	oraclePath := flag.String("oracle", "data/rules/oracle-cards.json", "")
	astPath := flag.String("ast", "data/rules/ast_dataset.jsonl", "")
	decksRoot := flag.String("decks", "data/decks", "")
	topN := flag.Int("top", 20, "")
	flag.Parse()

	corpus, err := astload.Load(*astPath)
	if err != nil {
		log.Fatalf("astload: %v", err)
	}

	f, err := os.Open(*oraclePath)
	if err != nil {
		log.Fatalf("open oracle: %v", err)
	}
	var raw []oracleEntry
	if err := json.NewDecoder(f).Decode(&raw); err != nil {
		log.Fatalf("decode oracle: %v", err)
	}
	f.Close()

	uncovered := map[string]string{} // name → class
	seen := map[string]bool{}
	unSets := map[string]bool{"Unstable": true, "Unhinged": true, "Unglued": true, "Unsanctioned": true, "Unfinity": true}
	for _, e := range raw {
		if e.Name == "" || seen[e.Name] || unSets[e.SetName] || e.Layout == "token" {
			continue
		}
		seen[e.Name] = true
		text := strings.TrimSpace(e.OracleText)
		if text == "" && len(e.CardFaces) > 0 {
			text = strings.TrimSpace(e.CardFaces[0].OracleText)
		}
		isVanilla := text == "" || (strings.Contains(strings.ToLower(e.TypeLine), "basic") && strings.Contains(strings.ToLower(e.TypeLine), "land"))
		card, ok := corpus.Get(e.Name)
		if !ok {
			uncovered[e.Name] = "MISSING"
			continue
		}
		if !card.FullyParsed || len(card.ParseErrors) > 0 {
			uncovered[e.Name] = "PARTIAL"
			continue
		}
		if len(card.Abilities) == 0 && !isVanilla {
			uncovered[e.Name] = "EMPTY_AST"
		}
	}

	freq := map[string]int{}
	deckCount := 0
	err = filepath.Walk(*decksRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".txt") {
			return nil
		}
		deckCount++
		df, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer df.Close()
		s := bufio.NewScanner(df)
		for s.Scan() {
			line := strings.TrimSpace(s.Text())
			if line == "" {
				continue
			}
			name := ""
			if strings.HasPrefix(line, "COMMANDER:") {
				name = strings.TrimSpace(strings.TrimPrefix(line, "COMMANDER:"))
			} else {
				parts := strings.SplitN(line, " ", 2)
				if len(parts) == 2 {
					if _, err := strconv.Atoi(parts[0]); err == nil {
						name = strings.TrimSpace(parts[1])
					}
				}
			}
			if name == "" {
				continue
			}
			if _, isUnc := uncovered[name]; isUnc {
				freq[name]++
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalf("walk decks: %v", err)
	}

	type entry struct {
		Name  string
		Class string
		Count int
	}
	var entries []entry
	for n, c := range freq {
		entries = append(entries, entry{n, uncovered[n], c})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Count != entries[j].Count {
			return entries[i].Count > entries[j].Count
		}
		return entries[i].Name < entries[j].Name
	})

	fmt.Printf("# scanned %d decks, %d uncovered cards in oracle\n", deckCount, len(uncovered))
	fmt.Printf("# top %d uncovered cards by deck-frequency\n", *topN)
	fmt.Println("count\tclass\tname")
	for i, e := range entries {
		if i >= *topN {
			break
		}
		fmt.Printf("%d\t%s\t%s\n", e.Count, e.Class, e.Name)
	}
}
