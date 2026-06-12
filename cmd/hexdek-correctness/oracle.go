package main

// oracle.go — Scryfall oracle-cards.json → gameengine.ChaosCorpus.
// Copied faithfully from cmd/hexdek-loki (package-main code is not
// importable); the filtering and defaults must stay identical so this
// sweep samples the same card population as the loki baselines.

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

type oracleEntry struct {
	Name          string   `json:"name"`
	TypeLine      string   `json:"type_line"`
	SetName       string   `json:"set_name"`
	ManaCost      string   `json:"mana_cost"`
	CMC           float64  `json:"cmc"`
	Colors        []string `json:"colors"`
	ColorIdentity []string `json:"color_identity"`
	Power         string   `json:"power"`
	Toughness     string   `json:"toughness"`
	OracleText    string   `json:"oracle_text"`
	Loyalty       string   `json:"loyalty"`
	Defense       string   `json:"defense"`
	CardFaces     []struct {
		Name      string   `json:"name"`
		TypeLine  string   `json:"type_line"`
		ManaCost  string   `json:"mana_cost"`
		Colors    []string `json:"colors"`
		Power     string   `json:"power"`
		Toughness string   `json:"toughness"`
	} `json:"card_faces"`
}

func loadOracleCorpus(path string) (*gameengine.ChaosCorpus, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open oracle %s: %w", path, err)
	}
	defer f.Close()

	var entries []oracleEntry
	if err := json.NewDecoder(f).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode oracle: %w", err)
	}

	// Un-sets excluded per project directive (7174n1c 2026-04-17).
	unSets := map[string]bool{
		"Unstable": true, "Unhinged": true, "Unglued": true,
		"Unsanctioned": true, "Unfinity": true,
	}

	cards := make([]*gameengine.ChaosCard, 0, len(entries))
	for _, e := range entries {
		if e.Name == "" {
			continue
		}
		if unSets[e.SetName] {
			continue
		}
		typeLine := e.TypeLine
		if typeLine == "" && len(e.CardFaces) > 0 {
			typeLine = e.CardFaces[0].TypeLine
		}

		tlLower := strings.ToLower(typeLine)
		types := parseTypesSimple(typeLine)

		// Scryfall token entries are not deck cards.
		if strings.Contains(tlLower, "token") {
			continue
		}
		// Memorabilia / art-series inserts carry the literal type_line
		// "Card" — not playable objects.
		if tlLower == "card" || strings.HasPrefix(tlLower, "card //") {
			continue
		}

		isLegendary := strings.Contains(tlLower, "legendary")
		isCreature := strings.Contains(tlLower, "creature")
		isLand := strings.Contains(tlLower, "land")

		basicNames := map[string]bool{
			"Plains": true, "Island": true, "Swamp": true,
			"Mountain": true, "Forest": true, "Wastes": true,
		}
		isBasicLand := isLand && (strings.Contains(tlLower, "basic") || basicNames[e.Name])

		pw, pwOK := atoiSafe(e.Power)
		tg, tgOK := atoiSafe(e.Toughness)
		if pw == 0 && tg == 0 && len(e.CardFaces) > 0 {
			pw, pwOK = atoiSafe(e.CardFaces[0].Power)
			tg, tgOK = atoiSafe(e.CardFaces[0].Toughness)
		}
		if tg == 0 {
			if loy, ok := atoiSafe(e.Loyalty); ok {
				tg = loy
			}
		}
		if tg == 0 {
			if def, ok := atoiSafe(e.Defense); ok {
				tg = def
			}
		}

		// ETB-choice / enters-with-counters 0/0 defaults so the cards
		// survive SBA §704.5f on chaos boards.
		if isCreature && pw == 0 && tg == 0 {
			otLower := strings.ToLower(e.OracleText)
			isPTStar := !pwOK || !tgOK
			isETBChoice := (strings.Contains(otLower, "as this creature enters") ||
				strings.Contains(otLower, "as it enters")) &&
				(strings.Contains(otLower, "choose") ||
					strings.Contains(otLower, "becomes your choice"))
			isETBCounters := strings.Contains(otLower, "enters with") &&
				strings.Contains(otLower, "+1/+1 counter")

			if isPTStar && isETBChoice {
				pw = 3
				tg = 3
			} else if isETBCounters {
				pw = 3
				tg = 3
			}
		}

		cards = append(cards, &gameengine.ChaosCard{
			Name:          e.Name,
			TypeLine:      typeLine,
			Types:         types,
			ManaCost:      e.ManaCost,
			CMC:           int(e.CMC + 0.5),
			Colors:        e.Colors,
			ColorIdentity: e.ColorIdentity,
			Power:         pw,
			Toughness:     tg,
			IsLegendary:   isLegendary,
			IsCreature:    isCreature,
			IsLand:        isLand,
			IsBasicLand:   isBasicLand,
		})
	}

	return gameengine.NewChaosCorpus(cards), nil
}

func parseTypesSimple(typeLine string) []string {
	if typeLine == "" {
		return nil
	}
	normalized := strings.ReplaceAll(typeLine, "—", "-")
	var out []string
	for _, f := range strings.Fields(normalized) {
		f = strings.TrimSpace(f)
		if f == "" || f == "-" {
			continue
		}
		out = append(out, strings.ToLower(f))
	}
	return out
}

func atoiSafe(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "*" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}
