package moxfield

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/hexdek/hexdek/internal/judge"
	"github.com/hexdek/hexdek/internal/oracle"
)

// FormatViolation describes a single card whose Scryfall format
// legality reports a status (banned / not_legal / restricted) that
// prevents the card from being legal in the deck's declared format.
type FormatViolation struct {
	CardName string // canonical Scryfall card name
	Format   string // normalized format slug ("commander", "brawl", …)
	Status   string // Scryfall's status: "banned" / "not_legal" / "restricted"
	Reason   string // short human-readable explanation
}

func (v FormatViolation) String() string {
	return fmt.Sprintf("%s: %s (%s)", v.CardName, v.Reason, v.Status)
}

// FormatReport is the aggregate result of validating a decklist against
// a single format. Violations is sorted by CardName for deterministic
// output. SkippedUnknown lists cards whose legality couldn't be
// determined (no Scryfall data, or a pre-migration cache hit with an
// empty legalities column). Callers should surface SkippedUnknown so a
// user understands the validation was incomplete.
type FormatReport struct {
	Format         string
	Violations     []FormatViolation
	SkippedUnknown []string
}

// Stale-ban overrides (Gifts Ungiven etc.) are consulted via the single
// canonical judge.IsUnbannedOverride helper so the import path and the
// deck-analysis/display path share ONE correction source. Originally a
// local map here (#996); lifted to internal/judge (r63) so a recently-
// unbanned card reads LEGAL everywhere from one edit.

// IsClean returns true when zero violations were found. Unknown-skip
// entries don't count as clean — they're a different signal.
func (r *FormatReport) IsClean() bool { return len(r.Violations) == 0 }

// ValidateFormat checks each card against the Scryfall legality for
// the given format. Format strings should match Moxfield/Scryfall
// slugs ("commander", "brawl", "modern", "legacy", "vintage", "pauper",
// "pioneer", "standard", "pioneer", "historic", "alchemy",
// "premodern", "duel", "oathbreaker", "predh", …). Format is lower-
// cased before lookup.
//
// Treatment of statuses:
//   - "legal" / "" (legal-default): no violation.
//   - "banned" / "not_legal": always violations.
//   - "restricted": flagged AT MOST ONCE per card name (we don't yet
//     enforce per-card count; Vintage is the only format that uses
//     restricted, and only there does multi-copy matter).
//
// Cards passed in via `cards` whose Legalities map is nil or doesn't
// have a key for the requested format are added to SkippedUnknown
// rather than silently treated as legal — we'd rather tell the user
// "we don't know" than falsely greenlight a deck.
func ValidateFormat(format string, cards []*oracle.Card) *FormatReport {
	f := strings.ToLower(strings.TrimSpace(format))
	r := &FormatReport{Format: f}
	if f == "" {
		// Empty format: nothing to validate against. Return all cards
		// as unknown-skipped so the caller surfaces this clearly.
		for _, c := range cards {
			if c == nil {
				continue
			}
			r.SkippedUnknown = append(r.SkippedUnknown, c.Name)
		}
		sort.Strings(r.SkippedUnknown)
		return r
	}
	seenViolation := map[string]bool{}
	for _, c := range cards {
		if c == nil || strings.TrimSpace(c.Name) == "" {
			continue
		}
		if c.Legalities == nil {
			r.SkippedUnknown = append(r.SkippedUnknown, c.Name)
			continue
		}
		status, present := c.Legalities[f]
		if !present {
			r.SkippedUnknown = append(r.SkippedUnknown, c.Name)
			continue
		}
		// Stale-legality override: some cached Scryfall entries predate
		// recent Commander unbans (e.g. Gifts Ungiven) and still read
		// "banned". The shared canonical override (judge.IsUnbannedOverride)
		// reads known-unbanned cards as legal until the legality cache is
		// refreshed. Temporary — delete the override entries after refresh.
		if judge.IsUnbannedOverride(f, c.Name) {
			continue
		}
		switch status {
		case "legal", "":
			// fine
		case "banned", "not_legal", "restricted":
			if seenViolation[c.Name] {
				continue
			}
			seenViolation[c.Name] = true
			r.Violations = append(r.Violations, FormatViolation{
				CardName: c.Name,
				Format:   f,
				Status:   status,
				Reason:   reasonFor(status, f),
			})
		default:
			// Unknown status string — treat as a violation so a future
			// Scryfall status change doesn't silently mask issues.
			if seenViolation[c.Name] {
				continue
			}
			seenViolation[c.Name] = true
			r.Violations = append(r.Violations, FormatViolation{
				CardName: c.Name,
				Format:   f,
				Status:   status,
				Reason:   fmt.Sprintf("unknown legality status %q in %s", status, f),
			})
		}
	}
	sort.Slice(r.Violations, func(i, j int) bool {
		return r.Violations[i].CardName < r.Violations[j].CardName
	})
	sort.Strings(r.SkippedUnknown)
	return r
}

func reasonFor(status, format string) string {
	switch status {
	case "banned":
		return "banned in " + format
	case "not_legal":
		return "not legal in " + format
	case "restricted":
		return "restricted in " + format + " (max 1 copy)"
	default:
		return status + " in " + format
	}
}

// ValidateFormatFromURL is the high-level convenience used by the
// CLI: given a Moxfield URL it routes through the existing fetch
// cache, resolves each unique commander+mainboard card via the oracle
// (Scryfall), and runs ValidateFormat against the deck's declared
// format. Returns nil on URL parse failure or a fetch error so the
// caller can fall back to a "validation skipped" notice.
//
// Callers that already have an *oracle.Card slice (e.g., post-Resolve)
// should call ValidateFormat directly to avoid the extra Scryfall fetch.
func ValidateFormatFromURL(ctx context.Context, database *sql.DB, url string) *FormatReport {
	deckID := ExtractDeckID(url)
	if deckID == "" {
		return nil
	}
	data, err := fetchDeckRaw(deckID)
	if err != nil {
		return nil
	}
	return validateFormatFromAPIResponse(ctx, database, data)
}

// validateFormatFromAPIResponse is the unexported core that does the
// actual oracle resolution + validation. Exists separately from
// ValidateFormatFromURL so tests in this package can drive it with a
// hand-built apiResponse without needing the deck-fetch layer.
func validateFormatFromAPIResponse(ctx context.Context, database *sql.DB, data *apiResponse) *FormatReport {
	if data == nil {
		return &FormatReport{}
	}
	uniqueNames := map[string]bool{}
	add := func(entries map[string]apiCardEntry) {
		for _, e := range entries {
			if e.Card.Name == "" || e.Quantity <= 0 {
				continue
			}
			uniqueNames[e.Card.Name] = true
		}
	}
	add(data.commanders())
	add(data.mainboard())
	// Sideboard / companion / maybeboard / considering intentionally
	// excluded: they're not part of the playable Commander deck and
	// Moxfield often parks experimental / off-format cards there.
	names := make([]string, 0, len(uniqueNames))
	for n := range uniqueNames {
		names = append(names, n)
	}
	resolved := oracle.LookupMany(ctx, database, names)
	// LookupMany may index the same *Card under multiple keys (the
	// fuzzy-fallback path stores both the query name and the canonical
	// name). Dedupe by pointer so a DFC like "Wedding Announcement"
	// doesn't get its legality checked twice.
	seen := map[*oracle.Card]bool{}
	cards := make([]*oracle.Card, 0, len(resolved))
	for _, c := range resolved {
		if c == nil || seen[c] {
			continue
		}
		seen[c] = true
		cards = append(cards, c)
	}
	return ValidateFormat(data.Format, cards)
}
