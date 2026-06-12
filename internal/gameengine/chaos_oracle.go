package gameengine

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// chaos_oracle.go — the Scryfall bulk-oracle → ChaosCorpus loader,
// PROMOTED from cmd/hexdek-loki (r63 Judge CI gate) so every driver
// (loki, hexdek-judge --run) shares the one corpus-quality-filtered
// implementation: un-set exclusion, token/memorabilia filtering, DFC
// front-face fallbacks, planeswalker loyalty-as-toughness, and the
// ETB-choice / enters-with-counters 0/0 P/T defaults. loki's
// loadOracleCorpus is now a delegate to this function.

// chaosOracleEntry is the slice of a Scryfall oracle entry the chaos
// corpus needs (moved verbatim from loki's oracleEntry).
type chaosOracleEntry struct {
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

func LoadChaosCorpusFromOracleJSON(path string) (*ChaosCorpus, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open oracle %s: %w", path, err)
	}
	defer f.Close()

	var entries []chaosOracleEntry
	if err := json.NewDecoder(f).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode oracle: %w", err)
	}

	// Un-sets excluded per project directive (7174n1c 2026-04-17).
	// These sets contain mechanics (widgets, augment, host, contraptions)
	// the engine doesn't handle, producing false-positive violations.
	unSets := map[string]bool{
		"Unstable": true, "Unhinged": true, "Unglued": true,
		"Unsanctioned": true, "Unfinity": true,
	}

	cards := make([]*ChaosCard, 0, len(entries))
	for _, e := range entries {
		if e.Name == "" {
			continue
		}
		if unSets[e.SetName] {
			continue
		}
		// Mystery Booster playtest cards are excluded for the same
		// reason as the un-sets: deliberately rules-breaking designs
		// (e.g. Visitor from Planet Q's printed type line is "Instant
		// Creature") that no format legalizes and the engine cannot
		// represent — 121 cards, 0 Commander-legal, and the one that
		// reached a chaos battlefield was the r63 correctness baseline's
		// last permanent_types hit.
		if strings.HasPrefix(e.SetName, "Mystery Booster Playtest Cards") {
			continue
		}
		typeLine := e.TypeLine
		if typeLine == "" && len(e.CardFaces) > 0 {
			typeLine = e.CardFaces[0].TypeLine
		}

		tlLower := strings.ToLower(typeLine)
		types := parseTypesSimple(typeLine)

		// Scryfall token entries are not deck cards — they're battlefield
		// instances minted by resolveCreateToken. Excluding them keeps the
		// chaos deck generator from seeding them as library/hand cards,
		// which would let SBA 704.5d sweep them (correctly per CR §704.5d)
		// but also flag the disappearance against the InstanceID census
		// for cards that should never have been minted as OG. Filtering
		// at the corpus level prevents the noise at the source.
		if strings.Contains(tlLower, "token") {
			continue
		}

		// Memorabilia / art-series / minigame inserts carry the literal
		// type_line "Card" (or "Card // Card") — they are not playable
		// objects. The fj22 "Knights" insert (empty mana cost, oracle
		// text "(Theme color: {W})") leaked into chaos decks this way
		// and got "cast" for announced-0 (r63, seed 42 game 482).
		// 2,650 such entries exist in the oracle corpus; none belong in
		// a deck pool.
		if tlLower == "card" || strings.HasPrefix(tlLower, "card //") {
			continue
		}

		isLegendary := strings.Contains(tlLower, "legendary")
		isCreature := strings.Contains(tlLower, "creature")
		isLand := strings.Contains(tlLower, "land")

		// Basic land detection.
		basicNames := map[string]bool{
			"Plains": true, "Island": true, "Swamp": true,
			"Mountain": true, "Forest": true, "Wastes": true,
		}
		isBasicLand := isLand && (strings.Contains(tlLower, "basic") || basicNames[e.Name])

		// Parse P/T.
		pw, pwOK := atoiSafe(e.Power)
		tg, tgOK := atoiSafe(e.Toughness)
		if pw == 0 && tg == 0 && len(e.CardFaces) > 0 {
			pw, pwOK = atoiSafe(e.CardFaces[0].Power)
			tg, tgOK = atoiSafe(e.CardFaces[0].Toughness)
		}
		// Planeswalker loyalty as toughness surrogate.
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

		// ETB-choice default: cards like Primal Plasma, Primal Clay,
		// Aquamorph Entity etc. have */* P/T with "As ~ enters, choose"
		// text. Without ETB resolution they'd be 0/0 and die to SBA
		// 704.5f. Similarly, 0/0 creatures that "enter with +1/+1
		// counters" (Marath, Verazol) need a baseline. Apply safe
		// defaults at corpus-load time so every downstream consumer
		// (chaos games, nightmare boards) inherits the fix.
		if isCreature && pw == 0 && tg == 0 {
			otLower := strings.ToLower(e.OracleText)
			isPTStar := !pwOK || !tgOK // "*" fails atoiSafe
			isETBChoice := (strings.Contains(otLower, "as this creature enters") ||
				strings.Contains(otLower, "as it enters")) &&
				(strings.Contains(otLower, "choose") ||
					strings.Contains(otLower, "becomes your choice"))
			isETBCounters := strings.Contains(otLower, "enters with") &&
				strings.Contains(otLower, "+1/+1 counter")

			if isPTStar && isETBChoice {
				// Pick the balanced middle form (most of these offer 3/3).
				pw = 3
				tg = 3
			} else if isETBCounters {
				// Give a baseline so they survive SBAs.
				pw = 3
				tg = 3
			}
		}

		card := &ChaosCard{
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
		}
		cards = append(cards, card)
	}

	return NewChaosCorpus(cards), nil
}

func parseTypesSimple(typeLine string) []string {
	if typeLine == "" {
		return nil
	}
	normalized := strings.ReplaceAll(typeLine, "\u2014", "-")
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
	n := 0
	neg := false
	i := 0
	if s[0] == '-' {
		neg = true
		i = 1
	} else if s[0] == '+' {
		i = 1
	}
	if i >= len(s) {
		return 0, false
	}
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	if neg {
		n = -n
	}
	return n, true
}
