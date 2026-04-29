package per_card

import (
	"math/rand"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerThassasOracle wires up Thassa's Oracle.
//
// Oracle text:
//
//	When Thassa's Oracle enters the battlefield, look at the top X cards
//	of your library, where X is your devotion to blue. Put up to one of
//	them on top of your library and the rest on the bottom of your
//	library in a random order. Then if your library has X or fewer
//	cards in it, you win the game.
//
// Rules notes:
//   - Devotion is CR §700.5: the number of {U} mana symbols among your
//     permanents' mana costs. Oracle herself has {U}{U} — one from her
//     own cost still counts because she IS on the battlefield at the
//     time her ETB trigger resolves (CR §603.6a — triggered abilities
//     evaluate when they resolve, not when they trigger).
//   - The win check is "if your library has X or fewer cards" — includes
//     the case where X ≥ library size. Zero-card library + 1 devotion
//     wins trivially; that's the Thoracle-Consultation/Pact combo line.
//   - This handler intentionally ignores the look/reorder step — see the
//     Python reference. The reorder is unobservable in MVP (no hidden
//     information model) and matters only for Laboratory Maniac timing,
//     not for the win check itself.
func registerThassasOracle(r *Registry) {
	r.OnETB("Thassa's Oracle", thassasOracleETB)
}

func thassasOracleETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "thassas_oracle_etb_win_check"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	// Count devotion to blue. Include Oracle herself (she's on the
	// battlefield by the time the ETB trigger resolves). If the caller
	// set "pip:U" on Thassa's Oracle card (test fixtures do), that's
	// included here; otherwise the count falls through to whatever other
	// blue permanents are in play.
	devotion := countDevotion(s, "U")
	librarySize := len(s.Library)

	// Random reorder of the bottom pile — faithful to oracle text for
	// tests that want to observe library-state changes. We only touch
	// the top `devotion` cards (or the whole library if smaller). Keep
	// up to one on top (the first one, as a deterministic MVP choice);
	// shuffle the rest to the bottom. The engine's gs.Rng seeds this.
	thassasOracleReorder(gs, s, devotion)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         seat,
		"devotion":     devotion,
		"library_size": librarySize,
	})

	if librarySize <= devotion {
		// Auto-win. This is THE Thassa's Oracle moment — library ≤
		// devotion means every Thoracle-consultation game ends here.
		emitWin(gs, seat, slug, perm.Card.DisplayName(),
			"Thassa's Oracle — library ≤ devotion to blue")
	}
}

// thassasOracleReorder implements the look-top-X-and-reorder clause.
// MVP: keep the first card on top, shuffle the rest to the bottom in a
// random order. Observable side effect: the library ordering changes.
// A real client would expose the choice; we pick deterministically.
func thassasOracleReorder(gs *gameengine.GameState, s *gameengine.Seat, devotion int) {
	if s == nil || devotion <= 0 || len(s.Library) == 0 {
		return
	}
	look := devotion
	if look > len(s.Library) {
		look = len(s.Library)
	}
	if look <= 1 {
		// Nothing to reorder meaningfully.
		return
	}
	// Keep first card on top; the rest (look-1 of them) go to the bottom
	// in a shuffled order.
	top := s.Library[0]
	tail := append([]*gameengine.Card(nil), s.Library[1:look]...)
	rest := append([]*gameengine.Card(nil), s.Library[look:]...)
	// Shuffle tail deterministically.
	rng := gs.Rng
	if rng == nil {
		rng = rand.New(rand.NewSource(1))
	}
	rng.Shuffle(len(tail), func(i, j int) { tail[i], tail[j] = tail[j], tail[i] })
	// New library: [top] + rest + shuffled tail (the tail is put "on the
	// bottom in a random order" per oracle text).
	newLib := make([]*gameengine.Card, 0, len(s.Library))
	newLib = append(newLib, top)
	newLib = append(newLib, rest...)
	newLib = append(newLib, tail...)
	s.Library = newLib
}
