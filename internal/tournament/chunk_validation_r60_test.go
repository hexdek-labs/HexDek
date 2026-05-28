package tournament

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/deckparser"
	gameengine "github.com/hexdek/hexdek/internal/gameengine"
)

// chunk_validation_r60_test pins the input-validation guard branches in
// RunChunk. These execute before any engine code, so they're cheap to
// test without setting up a real game; the r60 audit flagged chunk.go
// at 0% coverage because the only existing call sites are integration
// runs that exercise the happy path.

// stubTournamentDeck builds a minimal deckparser.TournamentDeck
// satisfying the validation checks (non-nil, non-empty CommanderCards)
// so we can isolate the chunk-specific error branches without a real
// deck file on disk.
func stubTournamentDeck(name string) *deckparser.TournamentDeck {
	return &deckparser.TournamentDeck{
		CommanderName:  name,
		CommanderCards: []*gameengine.Card{{Name: name}},
		Library:        nil, // chunk validation doesn't read library
	}
}

// TestRunChunk_NSeatsZeroErrors pins the first guard: NSeats must be
// > 0. The error message format is part of the contract (consumers may
// match on it), so we assert the message prefix.
func TestRunChunk_NSeatsZeroErrors(t *testing.T) {
	out, err := RunChunk(ChunkConfig{NSeats: 0, NGames: 1, Decks: nil})
	if err == nil {
		t.Fatalf("expected NSeats=0 to error")
	}
	if out != nil {
		t.Errorf("expected nil outcomes on error, got %d", len(out))
	}
	if !strings.Contains(err.Error(), "NSeats must be > 0") {
		t.Errorf("wrong error message: %v", err)
	}
}

// TestRunChunk_NGamesZeroErrors pins the second guard.
func TestRunChunk_NGamesZeroErrors(t *testing.T) {
	_, err := RunChunk(ChunkConfig{NSeats: 2, NGames: 0})
	if err == nil || !strings.Contains(err.Error(), "NGames must be > 0") {
		t.Errorf("expected 'NGames must be > 0' error, got: %v", err)
	}
}

// TestRunChunk_DeckCountMismatchErrors pins the third guard: NSeats
// must equal len(Decks). The chunk runner doesn't rotate decks across
// seats the way Run does, so a length mismatch is unrecoverable.
func TestRunChunk_DeckCountMismatchErrors(t *testing.T) {
	_, err := RunChunk(ChunkConfig{
		NSeats: 4,
		NGames: 1,
		Decks:  []*deckparser.TournamentDeck{stubTournamentDeck("A"), stubTournamentDeck("B")},
	})
	if err == nil {
		t.Fatalf("expected deck-count mismatch error")
	}
	if !strings.Contains(err.Error(), "len(Decks)=2") || !strings.Contains(err.Error(), "NSeats=4") {
		t.Errorf("error should include both counts, got: %v", err)
	}
}

// TestRunChunk_HatFactoriesTooFewErrors pins the fourth guard:
// HatFactories must be 0, 1, or at least NSeats entries. 2 factories
// for 4 seats falls into the "default" case (not 0, not 1) and fails
// the `len < NSeats` check. (Surplus factories are allowed — the
// runner just takes the first NSeats — so this is the only path that
// triggers the error.)
func TestRunChunk_HatFactoriesTooFewErrors(t *testing.T) {
	stub := func() gameengine.Hat { return nil }
	_, err := RunChunk(ChunkConfig{
		NSeats: 4, NGames: 1,
		Decks: []*deckparser.TournamentDeck{
			stubTournamentDeck("A"), stubTournamentDeck("B"),
			stubTournamentDeck("C"), stubTournamentDeck("D"),
		},
		HatFactories: []HatFactory{stub, stub}, // 2 factories for 4 seats
	})
	if err == nil {
		t.Fatalf("expected HatFactories length error")
	}
	if !strings.Contains(err.Error(), "HatFactories must be 0, 1, or NSeats") {
		t.Errorf("wrong error: %v", err)
	}
}
