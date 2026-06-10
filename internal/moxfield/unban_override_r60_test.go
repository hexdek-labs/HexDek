package moxfield

import (
	"testing"

	"github.com/hexdek/hexdek/internal/oracle"
)

// unban_override_r60_test.go — pins the stale-legality override (r60) that
// keeps recently-unbanned cards whose cached Scryfall legality still reads
// "banned" (e.g. Gifts Ungiven, unbanned in Commander 2024) from being
// flagged as violations on import.

func TestValidateFormat_GiftsUngivenOverride(t *testing.T) {
	cards := []*oracle.Card{
		{Name: "Sol Ring", Legalities: leg("commander", "legal")},
		{Name: "Gifts Ungiven", Legalities: leg("commander", "banned")}, // stale cache
	}
	r := ValidateFormat("commander", cards)
	if !r.IsClean() {
		t.Fatalf("Gifts Ungiven should be overridden to legal in commander, got %d violation(s): %+v",
			len(r.Violations), r.Violations)
	}
}

func TestValidateFormat_OverrideCaseAndSpaceInsensitive(t *testing.T) {
	cards := []*oracle.Card{
		{Name: "  GIFTS UNGIVEN ", Legalities: leg("commander", "banned")},
	}
	if r := ValidateFormat("commander", cards); !r.IsClean() {
		t.Fatalf("override should be case/space-insensitive, got %+v", r.Violations)
	}
}

func TestValidateFormat_OverrideIsFormatScoped(t *testing.T) {
	// The override map is commander-only. A "banned" status in a different
	// format must still flag — the synthetic modern-banned status here proves
	// the override doesn't leak across formats.
	cards := []*oracle.Card{
		{Name: "Gifts Ungiven", Legalities: leg("modern", "banned")},
	}
	if r := ValidateFormat("modern", cards); r.IsClean() {
		t.Fatalf("commander-scoped override must not apply to modern; expected a violation")
	}
}
