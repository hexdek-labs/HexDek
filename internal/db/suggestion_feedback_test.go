package db

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func openSuggestionFeedbackDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if err := EnsureSuggestionFeedbackSchema(context.Background(), d); err != nil {
		t.Fatalf("EnsureSuggestionFeedbackSchema: %v", err)
	}
	return d
}

// TestEnsureSuggestionFeedbackSchema_NilDB verifies the defensive
// nil-db error.
func TestEnsureSuggestionFeedbackSchema_NilDB(t *testing.T) {
	if err := EnsureSuggestionFeedbackSchema(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil db")
	}
}

// TestEnsureSuggestionFeedbackSchema_Idempotent verifies repeat
// schema-apply doesn't error (server-startup safety).
func TestEnsureSuggestionFeedbackSchema_Idempotent(t *testing.T) {
	d := openSuggestionFeedbackDB(t)
	if err := EnsureSuggestionFeedbackSchema(context.Background(), d); err != nil {
		t.Fatalf("re-apply schema: %v", err)
	}
}

// TestUpsertSuggestionFeedback_NewRowCreatesWithGamesOne verifies
// the initial-row path: games_observed=1 + the win/validation/
// counter-signal fields carry over from the delta.
func TestUpsertSuggestionFeedback_NewRowCreatesWithGamesOne(t *testing.T) {
	d := openSuggestionFeedbackDB(t)
	ctx := context.Background()
	err := UpsertSuggestionFeedback(ctx, d, SuggestionFeedbackDelta{
		DeckKey:            "alice/krenko",
		SuggestionKind:     "add",
		SuggestionCategory: "interaction",
		CardName:           "Counterspell",
		SuggestionPriority: 8,
		Win:                1,
		Validated:          1,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	rows, err := ListSuggestionFeedbackByDeck(ctx, d, "alice/krenko")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.GamesObserved != 1 {
		t.Errorf("games_observed: want 1, got %d", r.GamesObserved)
	}
	if r.WinsObserved != 1 {
		t.Errorf("wins_observed: want 1, got %d", r.WinsObserved)
	}
	if r.Validations != 1 {
		t.Errorf("validations: want 1, got %d", r.Validations)
	}
	if r.CounterSignals != 0 {
		t.Errorf("counter_signals: want 0, got %d", r.CounterSignals)
	}
	if r.CardName != "Counterspell" {
		t.Errorf("card_name: want Counterspell, got %q", r.CardName)
	}
	if r.SuggestionPriority != 8 {
		t.Errorf("priority: want 8, got %d", r.SuggestionPriority)
	}
}

// TestUpsertSuggestionFeedback_RepeatIncrementsRunningCounts
// verifies the upsert path: a second delta for the same composite
// key adds to the existing counts rather than overwriting.
func TestUpsertSuggestionFeedback_RepeatIncrementsRunningCounts(t *testing.T) {
	d := openSuggestionFeedbackDB(t)
	ctx := context.Background()
	base := SuggestionFeedbackDelta{
		DeckKey:            "alice/krenko",
		SuggestionKind:     "add",
		SuggestionCategory: "interaction",
		CardName:           "Counterspell",
		SuggestionPriority: 8,
	}
	// Game 1: win, validated
	mustUpsertSF(t, d, withWinValidated(base, 1, 1, 0))
	// Game 2: loss, counter-signal
	mustUpsertSF(t, d, withWinValidated(base, 0, 0, 1))
	// Game 3: win, validated
	mustUpsertSF(t, d, withWinValidated(base, 1, 1, 0))

	rows, _ := ListSuggestionFeedbackByDeck(ctx, d, "alice/krenko")
	if len(rows) != 1 {
		t.Fatalf("want 1 row after 3 upserts, got %d", len(rows))
	}
	r := rows[0]
	if r.GamesObserved != 3 {
		t.Errorf("games_observed: want 3, got %d", r.GamesObserved)
	}
	if r.WinsObserved != 2 {
		t.Errorf("wins_observed: want 2, got %d", r.WinsObserved)
	}
	if r.Validations != 2 {
		t.Errorf("validations: want 2, got %d", r.Validations)
	}
	if r.CounterSignals != 1 {
		t.Errorf("counter_signals: want 1, got %d", r.CounterSignals)
	}
}

// TestUpsertSuggestionFeedback_DistinctSuggestionsOnSameDeckCoexist
// verifies the composite primary key correctly separates rows for
// different suggestions on the same deck.
func TestUpsertSuggestionFeedback_DistinctSuggestionsOnSameDeckCoexist(t *testing.T) {
	d := openSuggestionFeedbackDB(t)
	ctx := context.Background()
	mustUpsertSF(t, d, SuggestionFeedbackDelta{
		DeckKey: "alice/krenko", SuggestionKind: "add",
		CardName: "Counterspell", SuggestionCategory: "interaction",
	})
	mustUpsertSF(t, d, SuggestionFeedbackDelta{
		DeckKey: "alice/krenko", SuggestionKind: "add",
		CardName: "Swords to Plowshares", SuggestionCategory: "interaction",
	})
	mustUpsertSF(t, d, SuggestionFeedbackDelta{
		DeckKey: "alice/krenko", SuggestionKind: "cut",
		CardName: "Gilded Lotus", SuggestionCategory: "cuttable",
	})
	mustUpsertSF(t, d, SuggestionFeedbackDelta{
		DeckKey:         "alice/krenko",
		SuggestionKind:  "swap",
		CardName:        "Bountiful Promenade",
		CounterCardName: "Tranquil Cove",
		SuggestionCategory: "manabase",
	})

	rows, _ := ListSuggestionFeedbackByDeck(ctx, d, "alice/krenko")
	if len(rows) != 4 {
		t.Errorf("want 4 distinct rows, got %d", len(rows))
	}
}

// TestUpsertSuggestionFeedback_SwapKeyIncludesCounterCard verifies
// that two swap suggestions sharing the same add-card but with
// different cut-cards are tracked as separate rows.
func TestUpsertSuggestionFeedback_SwapKeyIncludesCounterCard(t *testing.T) {
	d := openSuggestionFeedbackDB(t)
	ctx := context.Background()
	// Two swap recommendations both adding Sacred Foundry but
	// targeting different cut-cards (Boros Guildgate vs
	// Wind-Scarred Crag).
	mustUpsertSF(t, d, SuggestionFeedbackDelta{
		DeckKey: "alice/voltron", SuggestionKind: "swap",
		CardName: "Sacred Foundry", CounterCardName: "Boros Guildgate",
		SuggestionCategory: "manabase",
	})
	mustUpsertSF(t, d, SuggestionFeedbackDelta{
		DeckKey: "alice/voltron", SuggestionKind: "swap",
		CardName: "Sacred Foundry", CounterCardName: "Wind-Scarred Crag",
		SuggestionCategory: "manabase",
	})
	rows, _ := ListSuggestionFeedbackByDeck(ctx, d, "alice/voltron")
	if len(rows) != 2 {
		t.Errorf("want 2 rows (distinct cut cards), got %d", len(rows))
	}
}

// TestListOverCalibratedSuggestions_FiltersAndOrders verifies the
// over-calibration analytics view: only rows with games_observed >=
// minGames AND counter_signals > validations are returned, ordered
// by (counter - validations) descending.
func TestListOverCalibratedSuggestions_FiltersAndOrders(t *testing.T) {
	d := openSuggestionFeedbackDB(t)
	ctx := context.Background()

	// Suggestion A: 10 games, 1 validation, 7 counter-signals (delta=6) — over-calibrated
	for i := 0; i < 10; i++ {
		delta := SuggestionFeedbackDelta{
			DeckKey: "u/a", SuggestionKind: "add", CardName: "Mana Crypt",
			SuggestionCategory: "ramp",
		}
		if i < 1 {
			delta.Validated = 1
		} else if i < 8 {
			delta.CounterSignal = 1
		}
		mustUpsertSF(t, d, delta)
	}
	// Suggestion B: 10 games, 6 validations, 1 counter-signal (delta=-5) — well-calibrated
	for i := 0; i < 10; i++ {
		delta := SuggestionFeedbackDelta{
			DeckKey: "u/b", SuggestionKind: "add", CardName: "Counterspell",
			SuggestionCategory: "interaction",
		}
		if i < 6 {
			delta.Validated = 1
		} else if i < 7 {
			delta.CounterSignal = 1
		}
		mustUpsertSF(t, d, delta)
	}
	// Suggestion C: 2 games, 2 counter-signals (delta=2) — over-calibrated but BELOW minGames=5
	for i := 0; i < 2; i++ {
		mustUpsertSF(t, d, SuggestionFeedbackDelta{
			DeckKey: "u/c", SuggestionKind: "add", CardName: "Force of Will",
			SuggestionCategory: "interaction", CounterSignal: 1,
		})
	}
	// Suggestion D: 8 games, 0 validations, 4 counter-signals (delta=4) — over-calibrated
	for i := 0; i < 8; i++ {
		delta := SuggestionFeedbackDelta{
			DeckKey: "u/d", SuggestionKind: "add", CardName: "Fierce Guardianship",
			SuggestionCategory: "interaction",
		}
		if i < 4 {
			delta.CounterSignal = 1
		}
		mustUpsertSF(t, d, delta)
	}

	got, err := ListOverCalibratedSuggestions(ctx, d, 5)
	if err != nil {
		t.Fatalf("list over-calibrated: %v", err)
	}
	// Expected: A (delta=6) and D (delta=4), in that order. B is well-
	// calibrated, C is under-min-games.
	if len(got) != 2 {
		t.Fatalf("want 2 over-calibrated rows, got %d", len(got))
	}
	if got[0].CardName != "Mana Crypt" {
		t.Errorf("first row: want Mana Crypt (delta=6), got %q", got[0].CardName)
	}
	if got[1].CardName != "Fierce Guardianship" {
		t.Errorf("second row: want Fierce Guardianship (delta=4), got %q", got[1].CardName)
	}
}

// TestBatchUpsertSuggestionFeedback_OneTransactionMultipleDeltas
// verifies the batch helper writes multiple deltas in one
// transaction.
func TestBatchUpsertSuggestionFeedback_OneTransactionMultipleDeltas(t *testing.T) {
	d := openSuggestionFeedbackDB(t)
	ctx := context.Background()
	deltas := []SuggestionFeedbackDelta{
		{DeckKey: "u/a", SuggestionKind: "add", CardName: "Sol Ring", SuggestionCategory: "ramp", Win: 1},
		{DeckKey: "u/a", SuggestionKind: "add", CardName: "Arcane Signet", SuggestionCategory: "ramp", Win: 1},
		{DeckKey: "u/a", SuggestionKind: "cut", CardName: "Filler", SuggestionCategory: "cuttable", Win: 1},
	}
	if err := BatchUpsertSuggestionFeedback(ctx, d, deltas); err != nil {
		t.Fatalf("batch: %v", err)
	}
	rows, _ := ListSuggestionFeedbackByDeck(ctx, d, "u/a")
	if len(rows) != 3 {
		t.Errorf("want 3 rows, got %d", len(rows))
	}
}

// TestBatchUpsertSuggestionFeedback_AtomicityOnInvalidDelta verifies
// that a single bad delta aborts the entire batch — no partial
// writes survive.
func TestBatchUpsertSuggestionFeedback_AtomicityOnInvalidDelta(t *testing.T) {
	d := openSuggestionFeedbackDB(t)
	ctx := context.Background()
	deltas := []SuggestionFeedbackDelta{
		{DeckKey: "u/a", SuggestionKind: "add", CardName: "Counterspell", SuggestionCategory: "interaction"},
		// Bad: SuggestionKind not in allowed set
		{DeckKey: "u/a", SuggestionKind: "bogus", CardName: "X"},
		{DeckKey: "u/a", SuggestionKind: "add", CardName: "Path to Exile", SuggestionCategory: "interaction"},
	}
	if err := BatchUpsertSuggestionFeedback(ctx, d, deltas); err == nil {
		t.Fatal("expected validation error for bogus kind")
	}
	rows, _ := ListSuggestionFeedbackByDeck(ctx, d, "u/a")
	if len(rows) != 0 {
		t.Errorf("transaction should have rolled back; want 0 rows, got %d", len(rows))
	}
}

// TestUpsertSuggestionFeedback_ValidationsClampNonNegative verifies
// that negative win/validation/counter values are clamped to 0 at
// the SQL layer.
func TestUpsertSuggestionFeedback_ValidationsClampNonNegative(t *testing.T) {
	d := openSuggestionFeedbackDB(t)
	ctx := context.Background()
	err := UpsertSuggestionFeedback(ctx, d, SuggestionFeedbackDelta{
		DeckKey:            "u/a",
		SuggestionKind:     "add",
		CardName:           "X",
		SuggestionCategory: "interaction",
		Win:                -5,
		Validated:          -3,
		CounterSignal:      -1,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	rows, _ := ListSuggestionFeedbackByDeck(ctx, d, "u/a")
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.WinsObserved != 0 || r.Validations != 0 || r.CounterSignals != 0 {
		t.Errorf("negative deltas not clamped: wins=%d validations=%d counter=%d",
			r.WinsObserved, r.Validations, r.CounterSignals)
	}
	// But games_observed still got the +1
	if r.GamesObserved != 1 {
		t.Errorf("games_observed: want 1, got %d", r.GamesObserved)
	}
}

// TestUpsertSuggestionFeedback_RejectsMalformedDeltas verifies the
// validation gate rejects rows missing the composite-key fields.
func TestUpsertSuggestionFeedback_RejectsMalformedDeltas(t *testing.T) {
	d := openSuggestionFeedbackDB(t)
	ctx := context.Background()
	cases := []struct {
		name  string
		delta SuggestionFeedbackDelta
	}{
		{"empty deck_key", SuggestionFeedbackDelta{SuggestionKind: "add", CardName: "X"}},
		{"empty card_name", SuggestionFeedbackDelta{DeckKey: "u/a", SuggestionKind: "add"}},
		{"empty kind", SuggestionFeedbackDelta{DeckKey: "u/a", CardName: "X"}},
		{"unknown kind", SuggestionFeedbackDelta{DeckKey: "u/a", SuggestionKind: "delete", CardName: "X"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := UpsertSuggestionFeedback(ctx, d, c.delta); err == nil {
				t.Errorf("%s: expected error, got nil", c.name)
			}
		})
	}
}

// TestListSuggestionFeedbackByDeck_EmptyDeckReturnsEmpty verifies a
// deck with no observations returns an empty slice (not an error).
func TestListSuggestionFeedbackByDeck_EmptyDeckReturnsEmpty(t *testing.T) {
	d := openSuggestionFeedbackDB(t)
	ctx := context.Background()
	rows, err := ListSuggestionFeedbackByDeck(ctx, d, "u/unknown")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("want 0 rows for unobserved deck, got %d", len(rows))
	}
}

// TestListSuggestionFeedbackByDeck_OrderingBySignalDesc verifies the
// per-deck list orders by (validations + counter_signals) desc.
func TestListSuggestionFeedbackByDeck_OrderingBySignalDesc(t *testing.T) {
	d := openSuggestionFeedbackDB(t)
	ctx := context.Background()
	// 3 rows on same deck with different total-signal counts.
	for i := 0; i < 5; i++ {
		mustUpsertSF(t, d, SuggestionFeedbackDelta{
			DeckKey: "u/a", SuggestionKind: "add", CardName: "LowSignal",
			SuggestionCategory: "interaction",
		})
	}
	for i := 0; i < 8; i++ {
		mustUpsertSF(t, d, SuggestionFeedbackDelta{
			DeckKey: "u/a", SuggestionKind: "add", CardName: "MidSignal",
			SuggestionCategory: "interaction", Validated: 1,
		})
	}
	for i := 0; i < 12; i++ {
		mustUpsertSF(t, d, SuggestionFeedbackDelta{
			DeckKey: "u/a", SuggestionKind: "add", CardName: "HighSignal",
			SuggestionCategory: "interaction", CounterSignal: 1,
		})
	}
	rows, _ := ListSuggestionFeedbackByDeck(ctx, d, "u/a")
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
	if rows[0].CardName != "HighSignal" {
		t.Errorf("first row by signal-desc: want HighSignal, got %q", rows[0].CardName)
	}
	if rows[2].CardName != "LowSignal" {
		t.Errorf("last row by signal-desc: want LowSignal, got %q", rows[2].CardName)
	}
}

// withWinValidated returns a copy of base with the increment fields
// set. Helper for tests that repeatedly upsert the same base
// delta with varying outcomes.
func withWinValidated(base SuggestionFeedbackDelta, win, validated, counter int) SuggestionFeedbackDelta {
	out := base
	out.Win = win
	out.Validated = validated
	out.CounterSignal = counter
	return out
}

func mustUpsertSF(t *testing.T, d *sql.DB, delta SuggestionFeedbackDelta) {
	t.Helper()
	if err := UpsertSuggestionFeedback(context.Background(), d, delta); err != nil {
		t.Fatalf("upsert (%s/%s): %v", delta.DeckKey, delta.CardName, err)
	}
}

// TestBuildSuggestionFeedbackDeltas_HappyPath verifies the canonical
// hat-side integration: 3 recommendations + 3 per-suggestion verdicts
// + a game outcome → 3 deltas, one per recommendation.
func TestBuildSuggestionFeedbackDeltas_HappyPath(t *testing.T) {
	suggestions := []SuggestionRef{
		{Kind: "add", Category: "interaction", CardName: "Counterspell", Priority: 8},
		{Kind: "cut", Category: "cuttable", CardName: "Gilded Lotus", Priority: 5},
		{Kind: "swap", Category: "manabase", CardName: "Bountiful Promenade",
			CounterCardName: "Tranquil Cove", Priority: 7},
	}
	verdicts := []GameOutcomeForSuggestion{
		{Validated: 1},      // counterspell-suggestion validated
		{CounterSignal: 1},  // cut-suggestion contradicted (the card mattered)
		{},                  // swap inconclusive
	}
	deltas, err := BuildSuggestionFeedbackDeltas("alice/krenko", suggestions, verdicts, true)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(deltas) != 3 {
		t.Fatalf("want 3 deltas, got %d", len(deltas))
	}

	// All three should carry Win=1
	for i, d := range deltas {
		if d.Win != 1 {
			t.Errorf("delta %d: want Win=1, got %d", i, d.Win)
		}
		if d.DeckKey != "alice/krenko" {
			t.Errorf("delta %d: deck_key not propagated", i)
		}
	}

	// First delta: Counterspell, validated
	if deltas[0].CardName != "Counterspell" || deltas[0].Validated != 1 || deltas[0].CounterSignal != 0 {
		t.Errorf("first delta wrong: %+v", deltas[0])
	}
	// Second: Gilded Lotus, counter-signal
	if deltas[1].CardName != "Gilded Lotus" || deltas[1].CounterSignal != 1 || deltas[1].Validated != 0 {
		t.Errorf("second delta wrong: %+v", deltas[1])
	}
	// Third: Swap, both flags 0
	if deltas[2].SuggestionKind != "swap" || deltas[2].CounterCardName != "Tranquil Cove" ||
		deltas[2].Validated != 0 || deltas[2].CounterSignal != 0 {
		t.Errorf("third delta wrong: %+v", deltas[2])
	}
}

// TestBuildSuggestionFeedbackDeltas_LossGameWinZero verifies that a
// losing game produces Win=0 across all deltas.
func TestBuildSuggestionFeedbackDeltas_LossGameWinZero(t *testing.T) {
	suggestions := []SuggestionRef{
		{Kind: "add", Category: "interaction", CardName: "Swords to Plowshares", Priority: 8},
	}
	verdicts := []GameOutcomeForSuggestion{{Validated: 1}}
	deltas, _ := BuildSuggestionFeedbackDeltas("alice/krenko", suggestions, verdicts, false)
	if deltas[0].Win != 0 {
		t.Errorf("lost game: want Win=0, got %d", deltas[0].Win)
	}
}

// TestBuildSuggestionFeedbackDeltas_LengthMismatchErrors verifies
// that a misaligned verdicts slice returns an error (catches the
// hat-side classifier-out-of-sync bug).
func TestBuildSuggestionFeedbackDeltas_LengthMismatchErrors(t *testing.T) {
	suggestions := []SuggestionRef{
		{Kind: "add", CardName: "X"},
		{Kind: "add", CardName: "Y"},
	}
	verdicts := []GameOutcomeForSuggestion{{Validated: 1}} // 1 verdict, 2 suggestions
	if _, err := BuildSuggestionFeedbackDeltas("u/a", suggestions, verdicts, true); err == nil {
		t.Error("want error on length mismatch, got nil")
	}
}

// TestBuildSuggestionFeedbackDeltas_EmptySuggestionsEmptyDeltas
// verifies the no-recommendations case: a well-tuned deck with 0
// suggestions produces 0 deltas, not nil and not an error.
func TestBuildSuggestionFeedbackDeltas_EmptySuggestionsEmptyDeltas(t *testing.T) {
	deltas, err := BuildSuggestionFeedbackDeltas("u/a", nil, nil, true)
	if err != nil {
		t.Errorf("want nil error, got %v", err)
	}
	if deltas == nil {
		t.Errorf("want empty slice (not nil)")
	}
	if len(deltas) != 0 {
		t.Errorf("want 0 deltas, got %d", len(deltas))
	}
}

// TestBuildSuggestionFeedbackDeltas_NegativeVerdictsClamped
// verifies that negative Validated/CounterSignal values are clamped
// to 0 at the helper layer (defense-in-depth alongside the SQL
// MAX(0, ...) clamp).
func TestBuildSuggestionFeedbackDeltas_NegativeVerdictsClamped(t *testing.T) {
	suggestions := []SuggestionRef{{Kind: "add", CardName: "X"}}
	verdicts := []GameOutcomeForSuggestion{{Validated: -1, CounterSignal: -3}}
	deltas, _ := BuildSuggestionFeedbackDeltas("u/a", suggestions, verdicts, true)
	if deltas[0].Validated != 0 || deltas[0].CounterSignal != 0 {
		t.Errorf("negatives not clamped: %+v", deltas[0])
	}
}

// TestBuildSuggestionFeedbackDeltas_EmptyDeckKeyErrors verifies the
// required deck_key gate.
func TestBuildSuggestionFeedbackDeltas_EmptyDeckKeyErrors(t *testing.T) {
	if _, err := BuildSuggestionFeedbackDeltas("", nil, nil, false); err == nil {
		t.Error("want error on empty deckKey")
	}
}

// TestBuildSuggestionFeedbackDeltas_EndToEndWithUpsert verifies the
// canonical full integration path: build deltas + upsert + read
// back.
func TestBuildSuggestionFeedbackDeltas_EndToEndWithUpsert(t *testing.T) {
	d := openSuggestionFeedbackDB(t)
	ctx := context.Background()

	suggestions := []SuggestionRef{
		{Kind: "add", Category: "interaction", CardName: "Counterspell", Priority: 8},
		{Kind: "add", Category: "wipes", CardName: "Cyclonic Rift", Priority: 7},
	}
	// Game 1: win, both validated
	deltas1, _ := BuildSuggestionFeedbackDeltas("alice/storm", suggestions,
		[]GameOutcomeForSuggestion{{Validated: 1}, {Validated: 1}}, true)
	if err := BatchUpsertSuggestionFeedback(ctx, d, deltas1); err != nil {
		t.Fatalf("upsert 1: %v", err)
	}
	// Game 2: loss, counterspell counter-signaled (deck won without
	// it, somehow — fake scenario), wipe still validated
	deltas2, _ := BuildSuggestionFeedbackDeltas("alice/storm", suggestions,
		[]GameOutcomeForSuggestion{{CounterSignal: 1}, {Validated: 1}}, false)
	if err := BatchUpsertSuggestionFeedback(ctx, d, deltas2); err != nil {
		t.Fatalf("upsert 2: %v", err)
	}

	rows, _ := ListSuggestionFeedbackByDeck(ctx, d, "alice/storm")
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	// Find Counterspell row
	var cs *SuggestionFeedback
	for i := range rows {
		if rows[i].CardName == "Counterspell" {
			cs = &rows[i]
		}
	}
	if cs == nil {
		t.Fatal("Counterspell row missing")
	}
	if cs.GamesObserved != 2 {
		t.Errorf("Counterspell games: want 2, got %d", cs.GamesObserved)
	}
	if cs.WinsObserved != 1 {
		t.Errorf("Counterspell wins: want 1, got %d", cs.WinsObserved)
	}
	if cs.Validations != 1 || cs.CounterSignals != 1 {
		t.Errorf("Counterspell signals: want 1/1, got %d/%d",
			cs.Validations, cs.CounterSignals)
	}
}
