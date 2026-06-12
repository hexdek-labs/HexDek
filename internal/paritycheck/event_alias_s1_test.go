package paritycheck

import (
	"testing"

	"github.com/hexdek/hexdek/internal/judge"
)

// Consolidation step 1 pin: paritycheck.Event must remain a true ALIAS
// of judge.Event (one type, two names) — not a copied definition
// that could drift. Cross-assignment without conversion only compiles
// for identical types, so this test is primarily a compile-time pin.
func TestEventIsAliasOfValidationEvent(t *testing.T) {
	var pe Event = judge.Event{Kind: "turn_start", Seq: 1}
	var ve judge.Event = pe
	if ve.Kind != "turn_start" || ve.Seq != 1 {
		t.Errorf("alias round-trip lost data: %+v", ve)
	}
}
