package hexapi

import (
	"encoding/json"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// handleSimilarDecks answers GET /api/decks/{owner}/{id}/similar with the
// top-N decks ranked by composite similarity to the target deck.
//
// Similarity (0-100+ composite, returned as `similarity`):
//
//	60 * jaccard(target.cards, other.cards)   // card overlap, set-size normalized
//	+ 25  if same commander_card
//	+ 15  if same archetype
//	+ 10  if bracket distance == 0
//	+  4  if bracket distance == 1            // adjacent brackets
//	+  5  if commander color identity matches // mono-W vs mono-W, Bant vs Bant, etc.
//
// Jaccard (|A∩B| / |A∪B|) replaces the old raw shared-card count so a
// 60-card overlap between two 99-card decks ranks above 60 shared cards
// out of 200 — the old metric biased toward large decks. Bracket adjacency
// gives partial credit for "one power tier off" decks instead of 0/all.
//
// Drop floor: similarity < 12 AND no commander/archetype/color match.
//
// The walk is O(n_decks * avg_deck_size); at the current scale (a few
// hundred decks, ~100 cards each) it's a single-digit-ms scan.
func (h *Handler) handleSimilarDecks(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}
	ownPath := findDeckFile(h.DecksDir, owner, id)
	if ownPath == "" {
		writeError(w, http.StatusNotFound, "deck not found")
		return
	}

	target := loadDeckSignature(h.DecksDir, owner, id, ownPath)
	if len(target.cards) == 0 {
		// No card list parsed — nothing to compare against.
		writeJSON(w, []map[string]any{})
		return
	}

	limit := 5
	if n := parseInt(r.URL.Query().Get("limit")); n > 0 && n <= 50 {
		limit = n
	}

	type scored struct {
		row   map[string]any
		score float64
	}
	var ranked []scored

	owners, err := os.ReadDir(h.DecksDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot read decks dir")
		return
	}
	for _, oent := range owners {
		if !oent.IsDir() {
			continue
		}
		ow := oent.Name()
		if ow == "freya" || ow == "benched" || ow == "test" || ow == "moxfield_300" || ow == ".versions" {
			continue
		}
		ownerDir := filepath.Join(h.DecksDir, ow)
		files, ferr := os.ReadDir(ownerDir)
		if ferr != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			n := f.Name()
			if !strings.HasSuffix(n, ".txt") && !strings.HasSuffix(n, ".json") {
				continue
			}
			otherID := strings.TrimSuffix(n, filepath.Ext(n))
			if ow == owner && otherID == id {
				continue // skip self
			}
			otherPath := filepath.Join(ownerDir, n)
			sig := loadDeckSignature(h.DecksDir, ow, otherID, otherPath)
			if len(sig.cards) == 0 {
				continue
			}
			s := computeSimilarity(target, sig)
			if s.dropped {
				continue
			}

			ranked = append(ranked, scored{
				row: map[string]any{
					"owner":            ow,
					"id":               otherID,
					"name":             sig.commanderName,
					"commander":        sig.commanderName,
					"commander_card":   sig.commanderCard,
					"bracket":          sig.bracket,
					"archetype":        sig.archetype,
					"shared_cards":     s.shared,
					"overlap_pct":      s.overlapPct,
					"same_commander":   s.sameCommander,
					"same_archetype":   s.sameArchetype,
					"same_bracket":     s.bracketDistance == 0 && target.bracket != "" && sig.bracket != "",
					"bracket_distance": s.bracketDistance,
					"same_colors":      s.sameColors,
					"score":            int(math.Round(s.score)), // back-compat field
					"similarity":       int(math.Round(s.score)),
				},
				score: s.score,
			})
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		// Tie-break: more shared cards wins, then alphabetical for stability.
		ai := ranked[i].row["shared_cards"].(int)
		bj := ranked[j].row["shared_cards"].(int)
		if ai != bj {
			return ai > bj
		}
		return ranked[i].row["id"].(string) < ranked[j].row["id"].(string)
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	out := make([]map[string]any, 0, len(ranked))
	for _, r := range ranked {
		out = append(out, r.row)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=120")
	_ = json.NewEncoder(w).Encode(out)
}

// similarityResult is the breakdown used to build the response row.
type similarityResult struct {
	shared          int
	overlapPct      int // 0-100, Jaccard * 100, rounded
	sameCommander   bool
	sameArchetype   bool
	bracketDistance int // -1 when either side unknown
	sameColors      bool
	score           float64
	dropped         bool
}

// computeSimilarity is pure — exposed for unit tests.
func computeSimilarity(target, other deckSig) similarityResult {
	res := similarityResult{bracketDistance: -1}

	for c := range target.cards {
		if other.cards[c] {
			res.shared++
		}
	}
	union := len(target.cards) + len(other.cards) - res.shared
	jaccard := 0.0
	if union > 0 {
		jaccard = float64(res.shared) / float64(union)
	}
	res.overlapPct = int(math.Round(jaccard * 100))

	score := 60.0 * jaccard

	if target.commanderCard != "" && other.commanderCard != "" &&
		strings.EqualFold(target.commanderCard, other.commanderCard) {
		res.sameCommander = true
		score += 25
	}
	if target.archetype != "" && other.archetype != "" &&
		strings.EqualFold(target.archetype, other.archetype) {
		res.sameArchetype = true
		score += 15
	}
	if tb, ok := parseBracket(target.bracket); ok {
		if ob, ok2 := parseBracket(other.bracket); ok2 {
			dist := tb - ob
			if dist < 0 {
				dist = -dist
			}
			res.bracketDistance = dist
			switch dist {
			case 0:
				score += 10
			case 1:
				score += 4
			}
		}
	}
	if len(target.colors) > 0 && colorIdentityEqual(target.colors, other.colors) {
		res.sameColors = true
		score += 5
	}

	res.score = score

	// Drop floor: nothing-in-common decks shouldn't pollute the list.
	if score < 12 && !res.sameCommander && !res.sameArchetype && !res.sameColors {
		res.dropped = true
	}
	return res
}

// parseBracket returns the integer bracket (1-5) parsed from the sig value.
// The filename parser uses "?" for unknown and "1".."5" otherwise; strategy.json
// writes the int directly via itoa.
func parseBracket(s string) (int, bool) {
	if s == "" || s == "?" {
		return 0, false
	}
	n := parseInt(s)
	if n <= 0 {
		return 0, false
	}
	return n, true
}

func colorIdentityEqual(a, b []string) bool {
	// Normalize first — uppercase, dedupe, sort — then compare.
	// (Comparing raw lengths early would mis-reject [W,W] vs [W].)
	norm := func(in []string) []string {
		seen := map[string]bool{}
		out := make([]string, 0, len(in))
		for _, c := range in {
			c = strings.ToUpper(strings.TrimSpace(c))
			if c == "" || seen[c] {
				continue
			}
			seen[c] = true
			out = append(out, c)
		}
		sort.Strings(out)
		return out
	}
	na, nb := norm(a), norm(b)
	if len(na) == 0 || len(na) != len(nb) {
		return false
	}
	for i := range na {
		if na[i] != nb[i] {
			return false
		}
	}
	return true
}

// deckSig is the minimal projection of a deck used for similarity scoring.
type deckSig struct {
	cards         map[string]bool // normalized lowercased card names (no commander)
	commanderName string
	commanderCard string
	bracket       string
	archetype     string
	colors        []string // commander color identity, e.g. ["W","U"]
}

// loadDeckSignature reads a deck file and pulls just the data we need
// for similarity scoring. Reads the Freya strategy.json sidecar when
// present for archetype + canonical bracket.
func loadDeckSignature(decksDir, owner, id, deckPath string) deckSig {
	sig := deckSig{cards: map[string]bool{}}

	commander, bracket, _ := parseDeckFilename(id)
	sig.commanderName = commander
	sig.bracket = bracket
	sig.commanderCard = extractCommander(deckPath)

	data, err := os.ReadFile(deckPath)
	if err != nil {
		return sig
	}
	var raw []map[string]any
	if strings.HasSuffix(deckPath, ".json") {
		raw = parseDeckJSON(data)
	} else {
		raw = parseDeckList(string(data))
	}
	for _, c := range raw {
		name, _ := c["name"].(string)
		if name == "" {
			continue
		}
		// Drop the commander tag prefix and any set-code parens.
		name = strings.TrimPrefix(name, "COMMANDER:")
		name = strings.TrimSpace(name)
		if idx := strings.Index(name, "("); idx > 0 {
			name = strings.TrimSpace(name[:idx])
		}
		// Skip the commander card itself — every deck shares its
		// commander with itself; same-commander gets its own bonus.
		if sig.commanderCard != "" && strings.EqualFold(name, sig.commanderCard) {
			continue
		}
		sig.cards[strings.ToLower(name)] = true
	}

	// Freya sidecar — for archetype, canonical bracket, color identity.
	strategyFile := filepath.Join(decksDir, owner, "freya", id+".strategy.json")
	if sd, err := os.ReadFile(strategyFile); err == nil {
		var strat struct {
			Archetype     string `json:"archetype"`
			Bracket       int    `json:"bracket"`
			ColorIdentity struct {
				CommanderColors []string `json:"commander_colors"`
			} `json:"color_identity"`
		}
		if json.Unmarshal(sd, &strat) == nil {
			if strat.Archetype != "" {
				sig.archetype = strat.Archetype
			}
			if strat.Bracket > 0 {
				sig.bracket = itoa(strat.Bracket)
			}
			if len(strat.ColorIdentity.CommanderColors) > 0 {
				sig.colors = strat.ColorIdentity.CommanderColors
			}
		}
	}
	return sig
}

func itoa(n int) string {
	// Tiny single-digit fast path so we don't pull in strconv just for bracket.
	if n >= 0 && n < 10 {
		return string(rune('0' + n))
	}
	// Fallback for >9 brackets (won't happen, but be safe).
	if n < 0 {
		return "-" + itoa(-n)
	}
	out := ""
	for n > 0 {
		out = string(rune('0'+(n%10))) + out
		n /= 10
	}
	return out
}
