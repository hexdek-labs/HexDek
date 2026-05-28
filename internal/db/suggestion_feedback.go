package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// suggestion_feedback — backend telemetry for validating Freya's
// SuggestedChanges output (PR #721) against observed game outcomes.
// When hat plays a deck and observes results, it can record whether
// each suggestion looked CORRECT (deck lost a winnable game due to
// the missing role the suggestion would have filled) or WRONG (deck
// won despite ignoring the suggestion). Over many games these signals
// reveal over-calibrated suggestions — a recommendation that the hat
// consistently counter-signals is one the suggestion engine should
// probably weaken.
//
// NOT user-facing. The Decks-screen still renders the raw Freya
// suggestions; this table is the analytics-only signal that
// downstream tuning can read to retune the suggestion thresholds.
//
// One row per (deck_key, suggestion_kind, card_name, counter_card_name).
// counter_card_name is only used for swaps — empty string for adds
// and cuts. The composite primary key lets the same deck carry
// multiple suggestion-feedback rows simultaneously (one per
// recommended add / cut / swap pair).
//
// games_observed is incremented every time a game is played with the
// deck while the suggestion is still in Freya's output. wins_observed
// tracks how many of those games the deck won (without taking the
// suggestion's advice). validations counts post-game hat classifications
// where the outcome supports the suggestion (deck lost on a game where
// the missing card would have mattered); counter_signals counts the
// opposite (deck won despite the gap).
//
// Hat-side classification policy (NOT pinned here — left to the hat
// package to decide per-game): a validation might be e.g. "the deck
// lost AND the loss correlated with the role-bucket the suggestion
// would have filled" (lost to a counterable spell with no counterspells
// in deck, lost to a wipe with no protection, etc). A counter-signal
// might be "deck won AND the suggestion-bucket was inert all game"
// (the deck won without ever needing a counterspell). The db layer
// just stores the running counts; the classification logic is the
// hat's call.
const suggestionFeedbackSchema = `
CREATE TABLE IF NOT EXISTS suggestion_feedback (
    deck_key            TEXT    NOT NULL,
    suggestion_kind     TEXT    NOT NULL,
    suggestion_category TEXT    NOT NULL,
    card_name           TEXT    NOT NULL,
    counter_card_name   TEXT    NOT NULL DEFAULT '',
    suggestion_priority INTEGER NOT NULL DEFAULT 0,
    games_observed      INTEGER NOT NULL DEFAULT 0,
    wins_observed       INTEGER NOT NULL DEFAULT 0,
    validations         INTEGER NOT NULL DEFAULT 0,
    counter_signals     INTEGER NOT NULL DEFAULT 0,
    updated_at          INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (deck_key, suggestion_kind, card_name, counter_card_name)
);
CREATE INDEX IF NOT EXISTS idx_suggestion_feedback_counter
    ON suggestion_feedback(counter_signals, validations);
CREATE INDEX IF NOT EXISTS idx_suggestion_feedback_deck
    ON suggestion_feedback(deck_key);
`

// EnsureSuggestionFeedbackSchema creates the suggestion_feedback
// table idempotently. Safe to call at server startup before any
// upsert runs.
func EnsureSuggestionFeedbackSchema(ctx context.Context, sqlDB *sql.DB) error {
	if sqlDB == nil {
		return errors.New("db: nil database for suggestion_feedback schema")
	}
	_, err := sqlDB.ExecContext(ctx, suggestionFeedbackSchema)
	return err
}

// SuggestionFeedback is the API row shape — what readers get back
// from ListSuggestionFeedbackByDeck / ListOverCalibratedSuggestions.
type SuggestionFeedback struct {
	DeckKey            string
	SuggestionKind     string // "add" | "cut" | "swap"
	SuggestionCategory string // "interaction" | "ramp" | ... | "cuttable" | "manabase"
	CardName           string
	CounterCardName    string // empty for adds/cuts; populated for swaps (the cut half)
	SuggestionPriority int
	GamesObserved      int
	WinsObserved       int
	Validations        int
	CounterSignals     int
	UpdatedAt          int64
}

// SuggestionFeedbackDelta is one game's increment for a single
// suggestion. Win / Validated / CounterSignal are 0 or 1.
//
// The caller is responsible for the classification (the hat's
// per-game post-mortem decides whether to mark a Validation or
// CounterSignal for each suggestion in Freya's output). The db
// layer just folds the increments into the running counts.
type SuggestionFeedbackDelta struct {
	DeckKey            string
	SuggestionKind     string
	SuggestionCategory string
	CardName           string
	CounterCardName    string
	SuggestionPriority int
	Win                int // 0 or 1 — was this game a win?
	Validated          int // 0 or 1 — did the outcome support the suggestion?
	CounterSignal      int // 0 or 1 — did the outcome contradict the suggestion?
}

// UpsertSuggestionFeedback applies a single delta in one statement.
// Internally re-uses the running-count pattern from
// BatchUpsertSuggestionFeedback — prefer that helper for multi-row
// writes (one transaction vs N).
func UpsertSuggestionFeedback(ctx context.Context, sqlDB *sql.DB, d SuggestionFeedbackDelta) error {
	return BatchUpsertSuggestionFeedback(ctx, sqlDB, []SuggestionFeedbackDelta{d})
}

// BatchUpsertSuggestionFeedback applies deltas in a single
// transaction. Each delta:
//
//   - Increments games_observed by 1
//   - Increments wins_observed by d.Win
//   - Increments validations by d.Validated
//   - Increments counter_signals by d.CounterSignal
//
// All increments are non-negative; negative values are clamped to 0
// at the SQL layer to defend against malformed deltas (a sign error
// in the hat's post-game classifier shouldn't be able to corrupt the
// running totals).
//
// Validates that DeckKey / SuggestionKind / CardName are non-empty
// — required for the composite primary key — and that SuggestionKind
// is one of {"add", "cut", "swap"}. Other free-form fields
// (Category, Counter, Priority) get whatever the caller passes
// without validation; they're advisory metadata, not key components.
func BatchUpsertSuggestionFeedback(ctx context.Context, sqlDB *sql.DB, deltas []SuggestionFeedbackDelta) error {
	if len(deltas) == 0 {
		return nil
	}
	if sqlDB == nil {
		return errors.New("db: nil database")
	}
	now := time.Now().Unix()
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO suggestion_feedback (
			deck_key, suggestion_kind, suggestion_category,
			card_name, counter_card_name, suggestion_priority,
			games_observed, wins_observed, validations, counter_signals,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?)
		ON CONFLICT(deck_key, suggestion_kind, card_name, counter_card_name)
		DO UPDATE SET
			games_observed = games_observed + 1,
			wins_observed = wins_observed + MAX(0, excluded.wins_observed),
			validations = validations + MAX(0, excluded.validations),
			counter_signals = counter_signals + MAX(0, excluded.counter_signals),
			suggestion_category = excluded.suggestion_category,
			suggestion_priority = excluded.suggestion_priority,
			updated_at = excluded.updated_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, d := range deltas {
		if err := validateSuggestionFeedbackDelta(d); err != nil {
			return err
		}
		win := clampNonNeg(d.Win)
		validated := clampNonNeg(d.Validated)
		counter := clampNonNeg(d.CounterSignal)
		_, err := stmt.ExecContext(ctx,
			d.DeckKey, d.SuggestionKind, d.SuggestionCategory,
			d.CardName, d.CounterCardName, d.SuggestionPriority,
			win, validated, counter, now)
		if err != nil {
			return fmt.Errorf("upsert suggestion_feedback (%s/%s): %w",
				d.DeckKey, d.CardName, err)
		}
	}
	return tx.Commit()
}

// ListSuggestionFeedbackByDeck returns every suggestion-feedback row
// for one deck, ordered by validations + counter_signals descending
// then by card_name ascending (so the highest-signal suggestions
// surface first in the analytics view). Returns an empty slice when
// the deck has no recorded feedback yet — a fresh deck or one whose
// hat plays haven't recorded any classifications.
func ListSuggestionFeedbackByDeck(ctx context.Context, sqlDB *sql.DB, deckKey string) ([]SuggestionFeedback, error) {
	if sqlDB == nil {
		return nil, errors.New("db: nil database")
	}
	if strings.TrimSpace(deckKey) == "" {
		return nil, errors.New("db: deck_key required")
	}
	rows, err := sqlDB.QueryContext(ctx, `
		SELECT deck_key, suggestion_kind, suggestion_category,
		       card_name, counter_card_name, suggestion_priority,
		       games_observed, wins_observed, validations, counter_signals,
		       updated_at
		FROM suggestion_feedback
		WHERE deck_key = ?
		ORDER BY (validations + counter_signals) DESC, card_name ASC`,
		deckKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSuggestionFeedbackRows(rows)
}

// ListOverCalibratedSuggestions returns rows where counter_signals
// substantially exceeds validations — the "the hat keeps showing this
// suggestion is wrong" tail. Filters to rows with games_observed >=
// minGames so a single-game outlier doesn't dominate the list.
//
// Ordering: (counter_signals - validations) descending, so the
// strongest counter-signals come first. Useful for the "which
// suggestions should we tune down?" analytics view.
func ListOverCalibratedSuggestions(ctx context.Context, sqlDB *sql.DB, minGames int) ([]SuggestionFeedback, error) {
	if sqlDB == nil {
		return nil, errors.New("db: nil database")
	}
	if minGames < 1 {
		minGames = 1
	}
	rows, err := sqlDB.QueryContext(ctx, `
		SELECT deck_key, suggestion_kind, suggestion_category,
		       card_name, counter_card_name, suggestion_priority,
		       games_observed, wins_observed, validations, counter_signals,
		       updated_at
		FROM suggestion_feedback
		WHERE games_observed >= ?
		  AND counter_signals > validations
		ORDER BY (counter_signals - validations) DESC, card_name ASC`,
		minGames)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSuggestionFeedbackRows(rows)
}

func scanSuggestionFeedbackRows(rows *sql.Rows) ([]SuggestionFeedback, error) {
	var out []SuggestionFeedback
	for rows.Next() {
		var r SuggestionFeedback
		if err := rows.Scan(
			&r.DeckKey, &r.SuggestionKind, &r.SuggestionCategory,
			&r.CardName, &r.CounterCardName, &r.SuggestionPriority,
			&r.GamesObserved, &r.WinsObserved, &r.Validations, &r.CounterSignals,
			&r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func validateSuggestionFeedbackDelta(d SuggestionFeedbackDelta) error {
	if strings.TrimSpace(d.DeckKey) == "" {
		return errors.New("db: SuggestionFeedbackDelta.DeckKey required")
	}
	if strings.TrimSpace(d.CardName) == "" {
		return errors.New("db: SuggestionFeedbackDelta.CardName required")
	}
	switch d.SuggestionKind {
	case "add", "cut", "swap":
		return nil
	}
	return fmt.Errorf("db: SuggestionFeedbackDelta.SuggestionKind must be add/cut/swap, got %q",
		d.SuggestionKind)
}

func clampNonNeg(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// SuggestionRef is the minimal hat-consumable view of one Freya
// suggestion (an add / cut / swap recommendation). Lives in the db
// package as the integration bridge: the hat reads
// id.suggestions.json (per PR #720), maps each entry to a
// SuggestionRef, and uses BuildSuggestionFeedbackDeltas to convert
// the deck's full suggestion set + the per-game outcome into a
// batch of SuggestionFeedbackDelta.
//
// Decoupled from cmd/hexdek-freya's SuggestedChanges struct (which
// lives in package main and can't be imported) so the hat can
// translate via JSON unmarshal without a package-main dependency.
type SuggestionRef struct {
	Kind            string // "add" | "cut" | "swap"
	Category        string
	CardName        string
	CounterCardName string // empty for adds/cuts
	Priority        int
}

// GameOutcomeForSuggestion captures the per-suggestion verdict the
// hat's post-game classifier delivers. The hat decides whether the
// outcome supports the suggestion (Validated=1), contradicts it
// (CounterSignal=1), or neither (both 0 — game was inconclusive
// for this specific recommendation). Both flags can be 0
// simultaneously; ONLY ONE should ever be 1 (the same game can't
// both validate and counter-signal the same recommendation).
//
// Win flags whether the seat playing the deck won the game,
// independent of suggestion-specific verdicts.
type GameOutcomeForSuggestion struct {
	Win           int // 0 or 1
	Validated     int // 0 or 1
	CounterSignal int // 0 or 1
}

// BuildSuggestionFeedbackDeltas maps a deck's full suggestion list
// plus a single game's per-suggestion verdicts into a batch of
// SuggestionFeedbackDelta ready for BatchUpsertSuggestionFeedback.
//
// The verdicts slice must be the same length as suggestions —
// one verdict per suggestion, in the same order. Mismatched
// lengths return an error rather than silently truncating; this
// catches the hat-side classifier bug where the verdict slice
// gets out of sync with the recommendation list.
//
// Win is the same across all returned deltas (the deck either won
// the game or didn't); only Validated / CounterSignal vary per
// suggestion. This is the canonical "one game played, N
// recommendations evaluated" shape.
//
// Returns an empty slice (not nil) when suggestions is empty — the
// caller can pass the result to BatchUpsertSuggestionFeedback
// without nil-checking.
func BuildSuggestionFeedbackDeltas(
	deckKey string,
	suggestions []SuggestionRef,
	verdicts []GameOutcomeForSuggestion,
	gameWon bool,
) ([]SuggestionFeedbackDelta, error) {
	if strings.TrimSpace(deckKey) == "" {
		return nil, errors.New("db: BuildSuggestionFeedbackDeltas: deckKey required")
	}
	if len(suggestions) != len(verdicts) {
		return nil, fmt.Errorf("db: BuildSuggestionFeedbackDeltas: %d suggestions but %d verdicts",
			len(suggestions), len(verdicts))
	}
	win := 0
	if gameWon {
		win = 1
	}
	out := make([]SuggestionFeedbackDelta, 0, len(suggestions))
	for i, s := range suggestions {
		v := verdicts[i]
		out = append(out, SuggestionFeedbackDelta{
			DeckKey:            deckKey,
			SuggestionKind:     s.Kind,
			SuggestionCategory: s.Category,
			CardName:           s.CardName,
			CounterCardName:    s.CounterCardName,
			SuggestionPriority: s.Priority,
			Win:                win,
			Validated:          clampNonNeg(v.Validated),
			CounterSignal:      clampNonNeg(v.CounterSignal),
		})
	}
	return out, nil
}
