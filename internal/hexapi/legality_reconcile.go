package hexapi

import (
	"encoding/json"
	"strings"

	"github.com/hexdek/hexdek/internal/judge"
)

// reconcileStaleBans patches a stored Freya analysis so its legality reflects
// the CURRENT banlist, not the banlist at analysis time.
//
// The deck page serves the stored strategy.json verdict; it is never
// recomputed on read. A deck analyzed before a Commander unban (e.g. Gifts
// Ungiven, unbanned 2024-09) therefore keeps a frozen "illegal" verdict
// forever even though the current engine reads the card legal. This re-checks
// every card the stored report flagged as banned against the canonical unban
// override (judge.IsUnbannedOverride) and drops any that are no longer banned,
// then recomputes banned_cards.valid, the top-level valid, and prunes the
// matching error/warning strings.
//
// It only ever REMOVES a stale ban — never adds one — so it cannot introduce a
// false ILLEGAL. A card newly banned AFTER analysis is still caught on the
// deck's next re-analysis (the deterministic checks — count / color identity /
// singleton / commander — do not go stale from text that hasn't changed).
//
// Pass-through on any decode failure or when nothing is flagged, mirroring
// annotateAnalysisFreshness: a reconciliation is never worth failing a read.
func reconcileStaleBans(raw []byte) []byte {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil || doc == nil {
		return raw
	}
	leg, ok := doc["legality"].(map[string]any)
	if !ok {
		return raw
	}
	bc, ok := leg["banned_cards"].(map[string]any)
	if !ok {
		return raw
	}
	found, _ := bc["banned_found"].([]any)
	if len(found) == 0 {
		return raw // nothing flagged banned — nothing to reconcile
	}

	var remaining []any
	var dropped []string
	for _, f := range found {
		name, _ := f.(string)
		if name != "" && judge.IsUnbannedOverride("commander", name) {
			dropped = append(dropped, name) // now legal — drop the stale ban
			continue
		}
		remaining = append(remaining, f)
	}
	if len(dropped) == 0 {
		return raw // every flagged card is still genuinely banned
	}

	if len(remaining) == 0 {
		bc["valid"] = true
		delete(bc, "banned_found")
	} else {
		bc["banned_found"] = remaining
	}
	leg["banned_cards"] = bc

	// Top-level valid is the AND of every sub-check's validity.
	leg["valid"] = legalitySubCheckValid(leg, "card_count") &&
		legalitySubCheckValid(leg, "color_identity") &&
		legalitySubCheckValid(leg, "singleton") &&
		legalitySubCheckValid(leg, "banned_cards") &&
		legalitySubCheckValid(leg, "commander")

	// Drop any error/warning line that names a now-legal card so the
	// violations panel doesn't keep showing the stale ban.
	if pruned := pruneStringsMentioning(leg["errors"], dropped); pruned != nil || leg["errors"] != nil {
		leg["errors"] = pruned
	}
	if pruned := pruneStringsMentioning(leg["warnings"], dropped); pruned != nil || leg["warnings"] != nil {
		leg["warnings"] = pruned
	}

	doc["legality"] = leg
	out, err := json.Marshal(doc)
	if err != nil {
		return raw
	}
	return out
}

// legalitySubCheckValid reads legality[key]["valid"]; a missing sub-check or a
// non-bool valid is treated as passing (never invents a failure).
func legalitySubCheckValid(leg map[string]any, key string) bool {
	m, ok := leg[key].(map[string]any)
	if !ok {
		return true
	}
	v, ok := m["valid"].(bool)
	if !ok {
		return true
	}
	return v
}

// pruneStringsMentioning returns the string slice with any entry that
// case-insensitively contains one of names removed. Returns nil when the input
// isn't a slice or the result is empty (so the key serializes as absent).
func pruneStringsMentioning(arr any, names []string) []any {
	list, ok := arr.([]any)
	if !ok {
		return nil
	}
	var out []any
	for _, e := range list {
		s, _ := e.(string)
		drop := false
		low := strings.ToLower(s)
		for _, n := range names {
			if s != "" && strings.Contains(low, strings.ToLower(n)) {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
