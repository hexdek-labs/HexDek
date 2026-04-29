package moxfield

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/hexdek/hexdek/internal/oracle"
)

// ResolveResult is the output of enriching a ParsedDecklist into our internal
// Deck format + any unresolved names (cards Scryfall couldn't find).
type ResolveResult struct {
	Deck       *Deck
	Unresolved []string
}

// Resolve turns a ParsedDecklist into a fully-typed internal Deck by looking
// up every card name via the oracle (Scryfall with SQLite cache).
//
// `name` is what to store as the deck's display name. If the parsed list only
// has one commander entry, `commanderName` is derived from it; caller can
// override by passing a non-empty value.
//
// Cards that Scryfall can't resolve are collected into Unresolved and OMITTED
// from the output deck — the caller decides whether to error out or proceed
// with a partial deck.
func Resolve(ctx context.Context, database *sql.DB, parsed *ParsedDecklist, name, commanderName, format string) (*ResolveResult, error) {
	if parsed == nil {
		return nil, fmt.Errorf("nil parsed decklist")
	}

	// Determine commander
	if commanderName == "" && len(parsed.Commander) > 0 {
		commanderName = parsed.Commander[0].Name
	}
	if commanderName == "" {
		return nil, fmt.Errorf("no commander found (add a `// Commander` section or pass commander_name)")
	}
	if format == "" {
		format = "commander"
	}
	if name == "" {
		name = commanderName
	}

	deck := &Deck{
		Name:      name,
		Format:    format,
		Commander: commanderName,
	}
	unresolved := []string{}

	// The commander itself goes into mainboard too, so StartGame can put it
	// in the command zone. We keep Quantity=1 and rely on engine to route.
	// Moxfield-style exports list commander separately from mainboard; we
	// merge them here.
	allEntries := make([]ParsedEntry, 0, len(parsed.Commander)+len(parsed.Mainboard))
	allEntries = append(allEntries, parsed.Commander...)
	allEntries = append(allEntries, parsed.Mainboard...)

	// Batch-resolve all unique card names up front. Scryfall's collection
	// endpoint handles up to 75 per request, so a 99-card deck typically
	// resolves in 1-2 API calls instead of 99 serial ones.
	uniqueNames := map[string]struct{}{}
	for _, e := range allEntries {
		uniqueNames[e.Name] = struct{}{}
	}
	nameList := make([]string, 0, len(uniqueNames))
	for n := range uniqueNames {
		nameList = append(nameList, n)
	}
	resolved := oracle.LookupMany(ctx, database, nameList)

	seen := map[string]int{}
	for _, entry := range allEntries {
		card, ok := resolved[strings.ToLower(strings.TrimSpace(entry.Name))]
		if !ok {
			unresolved = append(unresolved, entry.Name)
			continue
		}
		types, subtypes := splitTypeLine(card.TypeLine)
		colors := extractColors(card.ManaCost)

		// Dedupe: if the same card already added (e.g. commander also in list),
		// bump quantity instead of duplicating.
		if idx, ok := seen[strings.ToLower(card.Name)]; ok {
			deck.Mainboard[idx].Quantity += entry.Quantity
			continue
		}
		seen[strings.ToLower(card.Name)] = len(deck.Mainboard)
		deck.Mainboard = append(deck.Mainboard, &Card{
			Name:     card.Name,
			Quantity: entry.Quantity,
			ManaCost: card.ManaCost,
			CMC:      card.CMC,
			Types:    types,
			Subtypes: subtypes,
			Colors:   colors,
		})
	}

	if err := validate(deck); err != nil {
		return &ResolveResult{Deck: deck, Unresolved: unresolved}, fmt.Errorf("validate: %w", err)
	}
	return &ResolveResult{Deck: deck, Unresolved: unresolved}, nil
}

// splitTypeLine parses "Legendary Creature — Human Ninja" into
// types=["Legendary","Creature"], subtypes=["Human","Ninja"].
func splitTypeLine(typeLine string) (types, subtypes []string) {
	// Handle both em-dash and hyphen separators; Scryfall uses em-dash.
	normalized := strings.ReplaceAll(typeLine, "—", "-")
	parts := strings.SplitN(normalized, "-", 2)
	types = fields(parts[0])
	if len(parts) == 2 {
		subtypes = fields(parts[1])
	}
	return
}

func fields(s string) []string {
	out := []string{}
	for _, f := range strings.Fields(s) {
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// extractColors pulls unique WUBRG symbols from a mana cost string like
// "{2}{U}{B}" → ["U","B"]. Generic/X/numeric symbols are ignored.
// Hybrid symbols like {W/U} contribute both colors.
var symRE = regexp.MustCompile(`\{([^}]+)\}`)

func extractColors(manaCost string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, m := range symRE.FindAllStringSubmatch(manaCost, -1) {
		for _, part := range strings.Split(m[1], "/") {
			switch strings.ToUpper(part) {
			case "W", "U", "B", "R", "G":
				if !seen[strings.ToUpper(part)] {
					seen[strings.ToUpper(part)] = true
					out = append(out, strings.ToUpper(part))
				}
			}
		}
	}
	return out
}
