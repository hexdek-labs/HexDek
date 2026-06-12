// Package judge holds the canonical types for HexDek's unified
// validation/debug layer — consolidation step 1 of
// /tmp/fable-review/VALIDATION-CONSOLIDATION.md (greenlit 2026-06-12).
//
// The repo grew 13 ad-hoc validation surfaces, each with its own
// violation struct, event vocabulary, and loss-reason encoding. This
// package is the single home the later steps migrate onto:
//
//   - ValidationViolation — replaces gameengine.InvariantViolation,
//     hat.OracleViolation, loki's chaosViolation, and the violation
//     half of paritycheck.Divergence (steps 2+).
//   - Event — THE canonical event schema, promoted verbatim from
//     internal/paritycheck (which now aliases it).
//   - LossReason — the structured replacement for the freeform
//     Seat.LossReason string (lives ALONGSIDE the string until the
//     writers migrate).
//
// This package is intentionally a LEAF: stdlib imports only, so
// gameengine, hat, paritycheck, heimdall, analytics, tournament, and
// the cmd/ tools can all import it without cycles.
package judge

import "fmt"

// ---------------------------------------------------------------------------
// ValidationViolation — the one violation vocabulary.
// ---------------------------------------------------------------------------

// Severity levels. Values match hat.OracleViolation's existing vocabulary
// so Feynman output migrates without a value mapping.
const (
	SeverityCritical = "critical"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"
)

// Correctness dimensions (HEX-JUDGE.md): every Judge check answers one
// of five questions. Violations carry the dimension so reports and
// sinks can slice the stream by what KIND of wrongness occurred,
// orthogonally to which surface caught it.
const (
	DimensionLegality       = "legality"        // was each action allowed?
	DimensionConservation   = "conservation"    // objects conserved by identity?
	DimensionStateIntegrity = "state_integrity" // is state internally consistent?
	DimensionProgression    = "progression"     // does the game advance correctly?
	DimensionOutcome        = "outcome"         // did each effect produce the RIGHT result?
)

// Canonical Surface tags — which validator emitted the violation. One
// constant per emitting surface; offline drivers and inline checks share
// the same namespace so reports can group cross-surface.
const (
	SurfaceInvariants  = "invariants"   // gameengine.RunAllInvariants inline checks
	SurfaceLegality    = "legality"     // ride-along legality detector
	SurfaceFeynman     = "feynman"      // hat/feynman.go post-game oracle
	SurfaceLoki        = "loki"         // cmd/hexdek-loki chaos fuzzer
	SurfaceParity      = "parity"       // internal/paritycheck cross-engine diff
	SurfaceGoldilocks  = "goldilocks"   // cmd/hexdek-thor effect verification
	SurfaceSeatOutcome = "seat_outcome" // gameengine seat-outcome ride-along checker
	SurfaceOutcome     = "outcome"      // judge/outcome expected-delta interpreter
	SurfaceProgression = "progression"  // judge/progression trigger-correctness checker
)

// ValidationViolation is the canonical violation record. Every
// validation surface (inline invariants, ride-along legality, Feynman,
// Loki, parity, goldilocks) emits this shape so downstream reporting,
// deduplication, and triage speak one vocabulary.
//
// Field mapping from the legacy structs (migrated in steps 2+):
//
//	gameengine.InvariantViolation{Name, Message}    → {Surface: SurfaceInvariants, Name, Message}
//	hat.OracleViolation{Rule, Description, Seat,
//	                    Severity, Details}          → {Surface: SurfaceFeynman, Name: Rule,
//	                                                   Severity, Message: Description,
//	                                                   Context: Details + {"seat": Seat}}
//	loki chaosViolation{InvariantName, Message, …}  → {Surface: SurfaceLoki, Name, Message,
//	                                                   Context: {"game_idx", "seed", "turn", …}}
//	paritycheck.Divergence{Category, Detail, …}     → {Surface: SurfaceParity, Name: Category,
//	                                                   Message: Detail, Context: {"at_seq", …}}
type ValidationViolation struct {
	// Surface identifies the emitting validator (Surface* constants).
	Surface string `json:"surface"`

	// Name is the check or rule identifier — an invariant name
	// ("ZoneConservation"), a CR citation ("704.5f"), or a divergence
	// category ("event_field").
	Name string `json:"name"`

	// Severity is one of the Severity* constants. Surfaces that have no
	// severity notion default to SeverityCritical (a violation is a
	// violation until triaged otherwise).
	Severity string `json:"severity"`

	// Message is the human-readable description.
	Message string `json:"message"`

	// Dimension is the correctness dimension this violation belongs to
	// (Dimension* constants). Empty on legacy emissions that predate the
	// Judge fold; new checks must set it.
	Dimension string `json:"dimension,omitempty"`

	// Seat is the seat the violation attributes to: >= 1 for a specific
	// seat, -1 for explicitly game-global, 0 when unattributed (also
	// covers seat 0 — disambiguate via Context["seat"] when it matters).
	// Added in step 4: every engine surface (Feynman, legality,
	// seat-outcome) is seat-scoped, so the vocabulary carries it.
	Seat int `json:"seat,omitempty"`

	// Context carries surface-specific structured detail (turn, seed,
	// game index, instance IDs, …). Keys are snake_case by convention.
	// Nil when the surface has nothing to add.
	Context map[string]interface{} `json:"context,omitempty"`
}

// String renders the violation for logs/reports:
//
//	[severity] surface/Name: message
func (v ValidationViolation) String() string {
	if v.Seat > 0 {
		return fmt.Sprintf("[%s] %s/%s [seat %d]: %s", v.Severity, v.Surface, v.Name, v.Seat, v.Message)
	}
	return fmt.Sprintf("[%s] %s/%s: %s", v.Severity, v.Surface, v.Name, v.Message)
}

// ---------------------------------------------------------------------------
// Event — the canonical event schema (promoted from internal/paritycheck).
// ---------------------------------------------------------------------------

// Event is the canonical structured game event — promoted verbatim from
// internal/paritycheck (which now aliases this type), per the
// consolidation plan: "best-defined event vocabulary in the repo;
// everything else normalizes to it." Field set and JSON tags are
// wire-identical to the pre-promotion paritycheck.Event, pinned by
// TestEventWireFormatUnchanged.
type Event struct {
	// Seq is the monotonically-increasing event index within a game.
	Seq int `json:"seq"`

	// Turn is the turn number (1-indexed).
	Turn int `json:"turn"`

	// Phase is the CR §500.1 phase bucket (beginning / main / combat /
	// ending). Normalized to lowercase.
	Phase string `json:"phase"`

	// Step is the step within the phase (untap / upkeep / draw /
	// precombat_main / postcombat_main / declare_attackers / etc.).
	// Normalized to lowercase.
	Step string `json:"step"`

	// Seat is the seat index this event belongs to (-1 for global).
	Seat int `json:"seat"`

	// Kind is the event discriminator. Normalized to snake_case,
	// lowercase. Examples: turn_start, play_land, cast, stack_push,
	// stack_resolve, enter_battlefield, combat_damage, draw, discard,
	// die, exile, game_end.
	Kind string `json:"kind"`

	// Source is the source card name (or empty for seat-level events).
	Source string `json:"source,omitempty"`

	// Target is the target seat index (-1 for n/a).
	Target int `json:"target,omitempty"`

	// Amount is the numeric payload (damage dealt, cards drawn, life
	// gained, etc.). Zero when n/a.
	Amount int `json:"amount,omitempty"`

	// Rule is the CR citation (e.g. "601.2f", "704.5f"). Optional —
	// only the Python side fills this consistently.
	Rule string `json:"rule,omitempty"`
}

// ---------------------------------------------------------------------------
// LossReason — structured loss cause.
// ---------------------------------------------------------------------------

// Loss categories. Values are intentionally identical to
// gameengine.LossCause's stable strings (seat_loss_conditions_r60.go) so
// `string(cause)` round-trips into Category without a mapping table —
// validation cannot import gameengine (it would be a cycle), so the
// contract is value-equality, pinned by gameengine's
// TestLossCategoryValuesMatchLossCause.
const (
	LossCategoryLife            = "life_zero_or_less"            // CR §704.5a
	LossCategoryEmptyLibrary    = "attempted_empty_library_draw" // CR §704.5b
	LossCategoryPoison          = "ten_or_more_poison_counters"  // CR §704.5c
	LossCategoryCommanderDamage = "twenty_one_commander_damage"  // CR §704.6c / §903.10a
	LossCategoryEffect          = "you_lose_the_game_effect"     // CR §104.3e
	LossCategoryConcession      = "concession"                   // CR §104.3a
	LossCategoryLoopDraw        = "mandatory_loop_draw"          // CR §104.4b (SBA loop cap)
)

// KillClassification is heimdall.ClassifyKillFinal's structured result —
// the canonical kill method (seedcontract vocabulary: combat / commander
// / poison / mill / combo / timeout / draw / crash / concession) plus
// the final victim's structured loss detail. KillerSeat is -1 when no
// specific killer is attributable from final game state alone (the
// analytics event walk remains the per-victim killer-attribution
// authority).
type KillClassification struct {
	// Method is the canonical kill method (seedcontract vocabulary).
	Method string `json:"method"`

	// VictimSeat is the seat whose elimination ended the game (-1 when
	// no eliminated opponent exists — timeout/draw shapes).
	VictimSeat int `json:"victim_seat"`

	// KillerSeat is the seat credited with the kill, -1 when unknown.
	KillerSeat int `json:"killer_seat"`

	// Category / Rule / SourceCard mirror the victim's structured
	// LossReason when populated (empty for pre-LossDetail replays and
	// hand-built fixtures).
	Category   string `json:"category,omitempty"`
	Rule       string `json:"rule,omitempty"`
	SourceCard string `json:"source_card,omitempty"`
}

// LossReason is the structured loss cause — the canonical replacement
// for the freeform Seat.LossReason string ("21+ commander damage from
// X", "ten or more poison counters", …) whose substring-parsing the four
// kill classifiers each re-implemented. It lives ALONGSIDE the string on
// gameengine.Seat (field LossDetail) until the SBA writers migrate in a
// later step; nothing populates it yet.
type LossReason struct {
	// Category is one of the LossCategory* constants (value-identical
	// to gameengine.LossCause).
	Category string `json:"category"`

	// Rule is the CR citation that fired ("704.5a", "704.6c", "104.3e").
	Rule string `json:"rule,omitempty"`

	// SourceCard names the card responsible when one exists — the
	// commander for §704.6c, the effect source for §104.3e ("Door to
	// Nothingness"), empty for purely numeric clauses.
	SourceCard string `json:"source_card,omitempty"`
}

// String renders the loss reason for logs: "category (rule) [by SourceCard]".
func (r LossReason) String() string {
	s := r.Category
	if r.Rule != "" {
		s += " (" + r.Rule + ")"
	}
	if r.SourceCard != "" {
		s += " by " + r.SourceCard
	}
	return s
}
