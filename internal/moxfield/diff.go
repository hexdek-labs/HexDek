package moxfield

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ----- deck version diff -----
//
// DiffSnapshots compares two raw Moxfield API responses (typically
// pulled from snapshot files via LoadSnapshot) and reports what
// changed between them at the deck level: the per-board card adds /
// removes / quantity changes, plus the deck-level metadata fields
// (name, format, tags). Used by the CLI on re-import to surface
// upstream edits to the user — "the deck owner swapped Mana Crypt
// for Sol Ring and added a Combo tag since you last imported."

// CardLine is a single card-quantity tuple from a board.
type CardLine struct {
	Name     string
	Quantity int
}

// QuantityChange is a card whose quantity differs between the two
// sides of a diff. Old=2, New=3 means the player went from 2 copies
// to 3 (only meaningful for non-singleton formats like Modern; in
// Commander, every card is qty 1 except basic lands).
type QuantityChange struct {
	Name   string
	OldQty int
	NewQty int
}

// BoardDiff captures the per-board changes for a single board name
// (commanders, mainboard, sideboard, companions, maybeboard,
// considering). Slices are deterministically sorted by Name.
type BoardDiff struct {
	BoardName string
	Added     []CardLine       // present in new, absent in old
	Removed   []CardLine       // present in old, absent in new
	Changed   []QuantityChange // present in both with different qty
}

// IsEmpty returns true when no cards changed in this board.
func (b *BoardDiff) IsEmpty() bool {
	return len(b.Added) == 0 && len(b.Removed) == 0 && len(b.Changed) == 0
}

// DeckDiff is the top-level result of comparing two snapshots.
type DeckDiff struct {
	DeckID string

	NameChanged bool
	NameOld     string
	NameNew     string

	FormatChanged bool
	FormatOld     string
	FormatNew     string

	TagsAdded   []string
	TagsRemoved []string

	// Boards is keyed by board name. Only populated for boards that
	// had at least one card change OR that appear in either side
	// (a board missing from both sides is omitted).
	Boards map[string]*BoardDiff
}

// IsEmpty returns true when neither metadata nor any board has any
// changes — the two snapshots are functionally identical.
func (d *DeckDiff) IsEmpty() bool {
	if d == nil {
		return true
	}
	if d.NameChanged || d.FormatChanged {
		return false
	}
	if len(d.TagsAdded) > 0 || len(d.TagsRemoved) > 0 {
		return false
	}
	for _, b := range d.Boards {
		if !b.IsEmpty() {
			return false
		}
	}
	return true
}

// DiffSnapshots parses two raw Moxfield API JSON responses and
// returns the structured difference. Order matters: `oldRaw` is the
// baseline (prior import), `newRaw` is the newly-observed state.
// Added/Removed in board diffs are framed from new's perspective
// ("added by Moxfield owner since old", "removed since old").
//
// An error is returned only when one of the JSON bodies is
// unparseable. An empty rawJSON is treated as "deck did not exist"
// (every card in the other side counts as added or removed).
func DiffSnapshots(oldRaw, newRaw []byte) (*DeckDiff, error) {
	var oldResp, newResp apiResponse
	if len(oldRaw) > 0 {
		if err := json.Unmarshal(oldRaw, &oldResp); err != nil {
			return nil, fmt.Errorf("diff: parse oldRaw: %w", err)
		}
	}
	if len(newRaw) > 0 {
		if err := json.Unmarshal(newRaw, &newResp); err != nil {
			return nil, fmt.Errorf("diff: parse newRaw: %w", err)
		}
	}
	return diffApiResponses(&oldResp, &newResp), nil
}

// diffApiResponses is the inner core that pure-functions the diff
// against the already-parsed apiResponse pair. Exposed for in-package
// tests that want to skip the JSON round-trip.
func diffApiResponses(oldResp, newResp *apiResponse) *DeckDiff {
	d := &DeckDiff{
		Boards: map[string]*BoardDiff{},
	}

	// --- metadata ---
	d.NameOld = oldResp.Name
	d.NameNew = newResp.Name
	d.NameChanged = oldResp.Name != newResp.Name

	d.FormatOld = oldResp.Format
	d.FormatNew = newResp.Format
	d.FormatChanged = oldResp.Format != newResp.Format

	d.TagsAdded, d.TagsRemoved = diffStringSets(oldResp.deckTags(), newResp.deckTags())

	// --- per-board card changes ---
	type boardPair struct {
		name string
		old  map[string]apiCardEntry
		new  map[string]apiCardEntry
	}
	pairs := []boardPair{
		{"commanders", oldResp.commanders(), newResp.commanders()},
		{"mainboard", oldResp.mainboard(), newResp.mainboard()},
		{"sideboard", oldResp.sideboard(), newResp.sideboard()},
		{"companions", oldResp.companions(), newResp.companions()},
		{"maybeboard", oldResp.maybeboard(), newResp.maybeboard()},
		{"considering", oldResp.considering(), newResp.considering()},
	}
	for _, p := range pairs {
		bd := diffBoard(p.name, p.old, p.new)
		// Skip boards that don't exist in either side.
		if bd == nil {
			continue
		}
		d.Boards[p.name] = bd
	}
	return d
}

// diffBoard computes Added / Removed / Changed for a single board.
// Returns nil if both old and new are empty (board never existed).
func diffBoard(name string, oldEntries, newEntries map[string]apiCardEntry) *BoardDiff {
	if len(oldEntries) == 0 && len(newEntries) == 0 {
		return nil
	}
	oldQty := flattenBoardToNameQty(oldEntries)
	newQty := flattenBoardToNameQty(newEntries)
	bd := &BoardDiff{BoardName: name}
	// Added (in new, not in old)
	for n, q := range newQty {
		if _, ok := oldQty[n]; !ok {
			bd.Added = append(bd.Added, CardLine{Name: n, Quantity: q})
		}
	}
	// Removed (in old, not in new)
	for n, q := range oldQty {
		if _, ok := newQty[n]; !ok {
			bd.Removed = append(bd.Removed, CardLine{Name: n, Quantity: q})
		}
	}
	// Quantity-changed (in both, different qty)
	for n, oq := range oldQty {
		if nq, ok := newQty[n]; ok && nq != oq {
			bd.Changed = append(bd.Changed, QuantityChange{Name: n, OldQty: oq, NewQty: nq})
		}
	}
	sort.Slice(bd.Added, func(i, j int) bool { return bd.Added[i].Name < bd.Added[j].Name })
	sort.Slice(bd.Removed, func(i, j int) bool { return bd.Removed[i].Name < bd.Removed[j].Name })
	sort.Slice(bd.Changed, func(i, j int) bool { return bd.Changed[i].Name < bd.Changed[j].Name })
	return bd
}

// flattenBoardToNameQty collapses a Moxfield board's entries to
// name→quantity, skipping empty names + zero/negative quantities and
// summing duplicates (defensive — Moxfield typically dedupes, but
// older exports or custom card edits can introduce dupes by entry).
func flattenBoardToNameQty(entries map[string]apiCardEntry) map[string]int {
	out := map[string]int{}
	for _, e := range entries {
		if e.Card.Name == "" || e.Quantity <= 0 {
			continue
		}
		out[e.Card.Name] += e.Quantity
	}
	return out
}

// diffStringSets returns (added_to_new, removed_from_old) given two
// sorted string slices. Comparison is case-sensitive — case
// differences in tags ("Combo" vs "combo") count as a change, which
// matches the deckTags() output that already case-dedupes.
func diffStringSets(oldSet, newSet []string) (added, removed []string) {
	oldMap := map[string]bool{}
	for _, s := range oldSet {
		oldMap[s] = true
	}
	newMap := map[string]bool{}
	for _, s := range newSet {
		newMap[s] = true
	}
	for s := range newMap {
		if !oldMap[s] {
			added = append(added, s)
		}
	}
	for s := range oldMap {
		if !newMap[s] {
			removed = append(removed, s)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

// String returns a human-readable formatted diff suitable for CLI
// output. Boards with no changes are omitted from the body. An
// IsEmpty diff renders as "(no changes)".
func (d *DeckDiff) String() string {
	if d.IsEmpty() {
		return "(no changes)"
	}
	var sb strings.Builder
	if d.NameChanged {
		fmt.Fprintf(&sb, "  name:   %q → %q\n", d.NameOld, d.NameNew)
	}
	if d.FormatChanged {
		fmt.Fprintf(&sb, "  format: %q → %q\n", d.FormatOld, d.FormatNew)
	}
	if len(d.TagsAdded) > 0 {
		fmt.Fprintf(&sb, "  tags +: %s\n", strings.Join(d.TagsAdded, ", "))
	}
	if len(d.TagsRemoved) > 0 {
		fmt.Fprintf(&sb, "  tags -: %s\n", strings.Join(d.TagsRemoved, ", "))
	}
	boardOrder := []string{"commanders", "mainboard", "sideboard", "companions", "maybeboard", "considering"}
	for _, name := range boardOrder {
		bd, ok := d.Boards[name]
		if !ok || bd.IsEmpty() {
			continue
		}
		fmt.Fprintf(&sb, "  %s:\n", name)
		for _, c := range bd.Added {
			fmt.Fprintf(&sb, "    + %d %s\n", c.Quantity, c.Name)
		}
		for _, c := range bd.Removed {
			fmt.Fprintf(&sb, "    - %d %s\n", c.Quantity, c.Name)
		}
		for _, c := range bd.Changed {
			fmt.Fprintf(&sb, "    ~ %s: %d → %d\n", c.Name, c.OldQty, c.NewQty)
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

// DiffLatestSnapshots is a CLI convenience: loads the two most-
// recent snapshots for a deck and returns their diff. Returns
// (nil, nil) if fewer than 2 snapshots exist — first imports have
// nothing to diff against.
func DiffLatestSnapshots(deckID string) (*DeckDiff, error) {
	snaps, err := ListSnapshots(deckID)
	if err != nil {
		return nil, err
	}
	if len(snaps) < 2 {
		return nil, nil
	}
	newRaw, err := LoadSnapshot(snaps[0].Path)
	if err != nil {
		return nil, fmt.Errorf("diff: load new snapshot: %w", err)
	}
	oldRaw, err := LoadSnapshot(snaps[1].Path)
	if err != nil {
		return nil, fmt.Errorf("diff: load old snapshot: %w", err)
	}
	d, err := DiffSnapshots(oldRaw, newRaw)
	if err != nil {
		return nil, err
	}
	if d != nil {
		d.DeckID = deckID
	}
	return d, nil
}
