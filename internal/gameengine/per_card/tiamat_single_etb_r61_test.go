package per_card

import (
	"fmt"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Regressions for r61 review 03 bug C-1: Tiamat was registered with TWO
// full ETB tutor handlers (tiamat.go via registerDefaults, plus
// custom_tiamat.go via zz_handler_coverage_2_register.go). fireETB runs
// every handler under the normalized key, so a cast Tiamat tutored up
// to TEN Dragons — two stacked 5-card searches — instead of five.
// custom_tiamat.go is now a documented no-op; these tests pin both the
// registry shape and the dispatch-level behavior so a re-introduced
// duplicate fails loudly.

// TestTiamat_ExactlyOneETBHandlerRegistered walks the live registry and
// asserts Tiamat has exactly one ETB handler. Direct-call handler tests
// can never catch a duplicate registration; only the registry itself
// can.
func TestTiamat_ExactlyOneETBHandlerRegistered(t *testing.T) {
	reg := Global()
	reg.mu.RLock()
	n := len(reg.etb[normalizeName("Tiamat")])
	reg.mu.RUnlock()
	if n != 1 {
		t.Fatalf("Tiamat must have exactly 1 ETB handler registered, got %d — "+
			"a duplicate registration makes a cast Tiamat tutor 5 Dragons per handler (r61 C-1)", n)
	}
}

// TestTiamat_CastTutorsAtMostFiveDragons_Dispatch fires the composite
// ETB dispatch path with a library of 12 distinct Dragons and asserts
// exactly five reach hand. Under the C-1 double registration this
// yielded 10.
func TestTiamat_CastTutorsAtMostFiveDragons_Dispatch(t *testing.T) {
	gs := newGame(t, 2)
	for i := 0; i < 12; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library, &gameengine.Card{
			Name:  fmt.Sprintf("Test Dragon %02d", i),
			Owner: 0,
			Types: []string{"creature", "dragon"},
		})
	}

	tiamat := addPerm(gs, 0, "Tiamat", "creature", "dragon")
	tiamat.Flags["was_cast"] = 1 // intervening-if: "if you cast it"
	fireETB(gs, tiamat)

	if got := len(gs.Seats[0].Hand); got != 5 {
		t.Fatalf("cast Tiamat must tutor exactly 5 Dragons from a 12-Dragon library, got %d in hand (10 = duplicate ETB registration, r61 C-1)", got)
	}
	if got := len(gs.Seats[0].Library); got != 7 {
		t.Fatalf("library should retain 7 of 12 Dragons after the tutor, got %d", got)
	}
}
