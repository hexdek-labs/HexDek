// Package gameengine hosts the runtime state + effect resolver for the
// hexdek Go engine. It consumes the read-only AST produced by
// github.com/hexdek/hexdek/internal/gameast and mutates a
// GameState in response to effect resolutions.
//
// Scope (Phase 3):
//
//   - GameState / Seat / Card / Permanent / StackItem / Event types
//   - ResolveEffect(gs, src, effect) — dispatch on effect.Kind() for the
//     ~40 leaf + control-flow kinds emitted by scripts/mtg_ast.py
//   - PickTarget(gs, src, filter) — MVP target resolution
//
// Out of scope (defer to later phases):
//
//   - Combat damage assignment / blocker declaration   → Phase 4
//   - Priority passing + stack wiring                  → Phase 5
//   - State-based actions (§704)                       → Phase 6
//   - Replacement effects framework (§614)             → Phase 7
//   - Full §613 layer enforcement                      → Phase 8
//
// Thread-safety: a GameState value is single-goroutine. Tournament runs
// allocate one GameState per game; concurrency lives at the runner layer.
package gameengine

import (
	"math/rand"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameast"
)

// -----------------------------------------------------------------------------
// GameState
// -----------------------------------------------------------------------------

// GameState is the authoritative per-game state. Every resolver handler
// takes *GameState as its first mutation target.
type GameState struct {
	// Seats are players, indexed by seat number. Seats[0] is the starting
	// player for a two-player game; commander pods use len=4.
	Seats []*Seat

	// Rng is the per-game deterministic RNG. Must be seeded by the
	// constructor for reproducible tournament replays.
	Rng *rand.Rand

	// Turn bookkeeping. Turn is 1-indexed; Phase is "beginning"/"main"/
	// "combat"/"ending"; Step is the step within the phase ("untap",
	// "upkeep", "draw", "precombat_main", etc.); Active is the seat whose
	// turn it currently is.
	Turn   int
	Phase  string
	Step   string
	Active int

	// Stack holds spells and abilities waiting to resolve. Phase 5 will
	// wire priority around this; for Phase 3 the resolver only reads/marks
	// existing items (e.g., CounterSpell flipping Countered=true).
	Stack []*StackItem

	// EventLog is an append-only structured event stream. Every resolver
	// handler that mutates state emits at least one Event. Tests assert
	// against this slice; future agents (Phase 10 policy) read it for
	// credit assignment.
	EventLog []Event

	// RetainEvents controls whether LogEvent appends to EventLog.
	// When false (tournament non-audit mode), events are broadcast to
	// Hats but not accumulated, saving ~9GB/game of allocation pressure.
	RetainEvents bool

	// lastEvent holds the most recent event for Hat broadcast when
	// RetainEvents is false. Avoids slice growth while still letting
	// Hats observe gameplay.
	lastEvent Event

	// Cards points at the loaded corpus. Resolver handlers reach in via
	// Cards.Get(name) when they need to spawn token cards or reveal cards.
	Cards *astload.Corpus

	// EffectTimestamp is a monotonically increasing counter assigned to
	// permanents as they enter the battlefield. §613 layer application
	// uses it to break ordering ties between effects of the same layer.
	EffectTimestamp int

	// Flags is an open-ended map for one-off game-wide flags ("extra_turn
	// pending", "replacement effect seen", "eldrazi spawned this turn").
	// Resolvers write here when there isn't a dedicated field yet.
	Flags map[string]int

	// CommanderFormat gates §704.6c / §704.6d SBAs. When false, those
	// helpers short-circuit for zero runtime cost.
	CommanderFormat bool

	// Replacements is the §614 replacement-effects registry. Populated by
	// ETB hooks that call RegisterReplacementsForPermanent; drained on
	// LTB via UnregisterReplacementsForPermanent. FireEvent walks this
	// slice per event — the dispatcher pays O(n) per event, so prefer
	// inserting applicability predicates rather than registering wildcards.
	Replacements []*ReplacementEffect

	// ContinuousEffects is the §613 layer-system registry (Phase 8).
	// Every static ability that changes a copiable value (copy effects,
	// control change, text change, type-add/remove, color change, ability
	// add/remove, set/modify/switch P/T) lives here while its source is
	// on the battlefield. GetEffectiveCharacteristics walks this slice
	// in layer order (1→7e) to compute the current characteristics of a
	// permanent. Populated by RegisterContinuousEffect; drained on LTB
	// by UnregisterContinuousEffectsForPermanent.
	ContinuousEffects []*ContinuousEffect

	// charCache memoizes GetEffectiveCharacteristics. Key is the
	// Permanent pointer. Invalidated by charCacheEpoch — every mutation
	// that could change a characteristic bumps the epoch; a cache entry
	// is stale if its Epoch != the current epoch.
	charCache      map[*Permanent]*cachedCharacteristics
	charCacheEpoch uint64

	// painterColor is the chosen color for Painter's Servant. "" when no
	// Painter's Servant is on the battlefield. Set by
	// RegisterPaintersServant (Phase 8). Mirrors Python game.painter_color.
	PainterColor string

	// DelayedTriggers is the §603.7 delayed-trigger registry. Populated by
	// RegisterDelayedTrigger; drained at phase/step boundaries by
	// FireDelayedTriggers (turn.go). Mirrors Python game.delayed_triggers.
	DelayedTriggers []*DelayedTrigger

	// PendingExtraCombats mirrors Python game.pending_extra_combats — the
	// Aggravated Assault / Seize the Day / Moraug counter. Incremented by
	// ExtraCombat effects; take_turn's combat loop decrements + reruns.
	PendingExtraCombats int

	// SpellsCastThisTurn is the GLOBAL cast counter read by Storm (CR
	// §702.40) — the number of spells cast this turn across ALL seats.
	// Incremented by CastSpell (and commander-zone casts) after the spell
	// successfully lands on the stack. Storm copies do NOT increment per
	// §706.10 (a copy isn't cast). Resets to 0 at each untap step.
	//
	// Why GLOBAL: Storm's oracle text says "each other spell cast before
	// it this turn" — all players' spells count, not just the caster's.
	// A Counterspell cast during an opponent's turn still contributes to
	// the storm count on the caster's next turn (until the next untap).
	SpellsCastThisTurn int

	// CR §726 Day / Night designation. Begins as DayNightNeither per
	// §726.2 and transitions on specific boundaries (see dfc.go +
	// phases.go). Valid values: DayNightNeither, DayNightDay,
	// DayNightNight.
	DayNight string

	// Snapshot of spells cast by the active player during the turn that
	// is about to end — used by §726.3a (day↔night transition, which
	// runs at the START of the next turn so compares against "last
	// turn"). Captured by the tournament turn loop BEFORE rotating
	// active and consumed by EvaluateDayNightAtTurnStart().
	SpellsCastByActiveLastTurn int

	// PreventionShields is the §615 damage prevention shield registry.
	// Populated by AddPreventionShield; consumed by PreventDamageToPlayer
	// and PreventDamageToPermanent when damage would be dealt.
	PreventionShields []PreventionShield

	// ZoneCastGrants is the per-card zone-cast permission registry.
	// Populated by effects like Release to the Wind ("you may cast it
	// without paying its mana cost from exile") and Misthollow Griffin
	// ("you may cast this from exile"). Each entry maps a Card pointer
	// to a ZoneCastPermission describing how and from where it can be
	// cast. Consumed by the AI/Hat's cast-from-zone decision logic.
	ZoneCastGrants map[*Card]*ZoneCastPermission
}

// Day/Night state constants (CR §726).
const (
	DayNightNeither = "neither"
	DayNightDay     = "day"
	DayNightNight   = "night"
)

// DelayedTrigger is one registered §603.7 delayed-trigger entry. Mirrors
// Python DelayedTrigger dataclass.
type DelayedTrigger struct {
	// TriggerAt is the phase/step boundary at which this trigger fires.
	// Canonical values:
	//   - "end_of_turn"        fires at end step
	//   - "next_end_step"      fires at next end step
	//   - "your_next_end_step" fires at controller's next end step
	//   - "next_upkeep"        fires at next upkeep
	//   - "your_next_upkeep"   fires at controller's next upkeep
	//   - "end_of_combat"      fires at end of combat step
	//   - "your_next_turn"     fires at controller's next untap
	//   - "on_event"           fires when ConditionFn returns true (event-based)
	TriggerAt string

	// ControllerSeat is the seat that controls the delayed trigger.
	ControllerSeat int

	// SourceCardName is for log attribution.
	SourceCardName string

	// CreatedTurn is the turn number on which the delayed trigger was
	// registered. Used to detect "next" (i.e. strictly later) semantics.
	CreatedTurn int

	// SourceTimestamp is the §613 timestamp of the source effect at
	// registration time. Used for fire-ordering (§603.7).
	SourceTimestamp int

	// EffectFn is invoked when the trigger fires.
	EffectFn func(gs *GameState)

	// Consumed is set to true after firing to mark the trigger for removal.
	Consumed bool

	// OneShot is true for "at the next time X happens" triggers that fire
	// once and then remove themselves. CR §603.7d. When OneShot is true,
	// Consumed is automatically set after the first firing.
	OneShot bool

	// ConditionFn is an optional predicate for event-based delayed triggers
	// (TriggerAt == "on_event"). Called by FireEventDelayedTriggers whenever
	// a game event occurs. The trigger fires when ConditionFn returns true.
	// Nil means no condition (phase-boundary triggers don't use this).
	ConditionFn func(gs *GameState, ev *Event) bool
}

// FireEventDelayedTriggers checks all event-based delayed triggers
// (TriggerAt == "on_event") against the given event. Any whose ConditionFn
// returns true are fired. Returns the number of triggers fired.
// Called from LogEvent or specific event-emission points.
func FireEventDelayedTriggers(gs *GameState, ev *Event) int {
	if gs == nil || ev == nil || len(gs.DelayedTriggers) == 0 {
		return 0
	}
	var toFire []*DelayedTrigger
	for _, dt := range gs.DelayedTriggers {
		if dt == nil || dt.Consumed {
			continue
		}
		if dt.TriggerAt != "on_event" {
			continue
		}
		if dt.ConditionFn == nil {
			continue
		}
		if dt.ConditionFn(gs, ev) {
			toFire = append(toFire, dt)
		}
	}
	fired := 0
	for _, dt := range toFire {
		dt.Consumed = true
		gs.LogEvent(Event{
			Kind:   "delayed_trigger_fires",
			Seat:   dt.ControllerSeat,
			Source: dt.SourceCardName,
			Details: map[string]interface{}{
				"trigger_at": dt.TriggerAt,
				"one_shot":   dt.OneShot,
				"rule":       "603.7d",
			},
		})
		if dt.EffectFn != nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						gs.LogEvent(Event{
							Kind:   "delayed_trigger_crashed",
							Source: dt.SourceCardName,
							Details: map[string]interface{}{
								"panic": r,
							},
						})
					}
				}()
				dt.EffectFn(gs)
			}()
		}
		fired++
	}
	// Clean up consumed triggers.
	if fired > 0 {
		kept := gs.DelayedTriggers[:0]
		for _, dt := range gs.DelayedTriggers {
			if dt != nil && !dt.Consumed {
				kept = append(kept, dt)
			}
		}
		gs.DelayedTriggers = kept
	}
	return fired
}

// RegisterDelayedTrigger appends a delayed trigger to gs.DelayedTriggers.
// Mirrors Python register_delayed_trigger.
func (gs *GameState) RegisterDelayedTrigger(dt *DelayedTrigger) *DelayedTrigger {
	if gs == nil || dt == nil {
		return dt
	}
	if dt.SourceTimestamp == 0 {
		dt.SourceTimestamp = gs.NextTimestamp()
	}
	if dt.CreatedTurn == 0 {
		dt.CreatedTurn = gs.Turn
	}
	gs.DelayedTriggers = append(gs.DelayedTriggers, dt)
	gs.LogEvent(Event{
		Kind:   "delayed_trigger_registered",
		Seat:   dt.ControllerSeat,
		Source: dt.SourceCardName,
		Details: map[string]interface{}{
			"trigger_at": dt.TriggerAt,
			"rule":       "603.7",
		},
	})
	return dt
}

// NewGameState builds a fresh two-seat game. Caller is expected to
// populate libraries/hands/battlefields before calling ResolveEffect.
func NewGameState(seatCount int, rng *rand.Rand, corpus *astload.Corpus) *GameState {
	if seatCount < 1 {
		seatCount = 1
	}
	seats := make([]*Seat, seatCount)
	for i := 0; i < seatCount; i++ {
		seats[i] = newSeat(i)
	}
	return &GameState{
		Seats:        seats,
		Rng:          rng,
		Turn:         1,
		Phase:        "beginning",
		Step:         "untap",
		Active:       0,
		Cards:        corpus,
		Flags:        map[string]int{},
		EventLog:     make([]Event, 0, 64),
		RetainEvents: true,
		DayNight:     DayNightNeither,
	}
}

// NextTimestamp issues the next §613 layer timestamp and advances the
// counter. Monotonic; never re-used across a single game.
func (gs *GameState) NextTimestamp() int {
	gs.EffectTimestamp++
	return gs.EffectTimestamp
}

// LogEvent appends a structured event. Keeping this centralized means
// later phases can add instrumentation (timestamps, invariant checks) in
// one place.
//
// Phase 10: after the event is persisted, broadcast it to every seat's
// Hat (if any). Hats use this to drive archetype detection, mode
// transitions, and other adaptive behavior. The broadcast is best-effort
// — a nil-safe loop that tolerates seats without a Hat. Hats must not
// mutate the GameState from ObserveEvent (contract, not enforced) but
// they may update their OWN internal state.
func (gs *GameState) LogEvent(ev Event) {
	var evPtr *Event
	if gs.RetainEvents {
		gs.EventLog = append(gs.EventLog, ev)
		evPtr = &gs.EventLog[len(gs.EventLog)-1]
	} else {
		gs.lastEvent = ev
		evPtr = &gs.lastEvent
	}
	if len(gs.Seats) == 0 {
		return
	}
	for i, s := range gs.Seats {
		if s == nil || s.Hat == nil {
			continue
		}
		s.Hat.ObserveEvent(gs, i, evPtr)
	}
}

// Opponents returns the indices of every seat that is not `seat` and not
// already lost. Used by Choice/Damage/Discard targeting.
func (gs *GameState) Opponents(seat int) []int {
	out := make([]int, 0, len(gs.Seats))
	for i, s := range gs.Seats {
		if i == seat {
			continue
		}
		if s == nil || s.Lost || s.Won {
			continue
		}
		out = append(out, i)
	}
	return out
}

// -----------------------------------------------------------------------------
// Seat (player)
// -----------------------------------------------------------------------------

// Seat holds per-player state. One Seat per player regardless of format.
type Seat struct {
	Idx int // seat index, 0-based

	// Life total / lose flags. Life starts at 20; commander format callers
	// should set it to 40 before the first ResolveEffect.
	Life           int
	Lost           bool
	Won            bool
	PoisonCounters int

	// StartingLife is the opening life total for this seat (CR §103.3 /
	// §903.7). Defaults to 20 on seat construction; SetupCommanderGame
	// writes 40. Kept so reset/replay logic and UI know the intended
	// starting point independent of current Life.
	StartingLife int

	// LeftGame is the §800.4a idempotency guard — true once the seat
	// has had its leave-the-game cleanup run. Mirrors Python
	// Seat._left_game.
	LeftGame bool

	// Zones. Library[0] is the top of the library.
	Library     []*Card
	Hand        []*Card
	Graveyard   []*Card
	Exile       []*Card
	Battlefield []*Permanent

	// ManaPool — legacy untyped generic pool. Kept as the LEGACY API
	// surface: historical call sites (and a large existing test suite)
	// treat it as a plain int counter (`seat.ManaPool = 10`,
	// `seat.ManaPool -= cost`). New code should prefer the TYPED
	// ColoredManaPool at `Seat.Mana` (see mana.go), and interact via
	// AddMana / PayGenericCost / DrainAllPools. When Mana is non-nil,
	// ManaPool is kept in sync as Mana.Total() after every typed op.
	ManaPool int

	// Mana is the typed five-color+colorless+any+restricted pool per
	// CR §106. Nil until the first AddMana / PayGenericCost call; the
	// bridge in EnsureTypedPool lazily materializes it. See mana.go.
	Mana *ColoredManaPool

	// CommanderNames holds commander card names for §903 support. Empty in
	// non-commander formats.
	CommanderNames []string

	// CommandZone holds commander cards currently in the command zone.
	// Populated by the §704.6d / §903.9a SBA path.
	CommandZone []*Card

	// CommanderDamage is per-commander combat damage taken, keyed by
	// (dealer_seat, commander_name). CR §704.6c checks for 21+ from a
	// single commander ("the same commander"). Partner support requires
	// the dealer-seat dimension because two commanders owned by the SAME
	// seat (partner pair) accumulate damage INDEPENDENTLY — a pilot who
	// ate 15 damage from Kraum and 15 from Tymna has NOT lost the game.
	//
	// Access pattern:
	//   seat.CommanderDamage[dealerSeat][commanderName] += dmg
	// Loss check at §704.6c:
	//   for each dealer, name → dmg >= 21 → seat loses.
	//
	// Mirrors Python Seat.commander_damage (dict[int, dict[str, int]]).
	CommanderDamage map[int]map[string]int

	// CommanderCastCounts is the §903.8 "each previous time ... from the
	// command zone" surcharge counter, keyed by commander card name. It
	// tracks the count of prior command-zone casts. Actual cost for the
	// next cast = base_cmc + 2 * CommanderCastCounts[name]. Partner pairs
	// keep TWO independent entries (one per commander name), so casting
	// Kraum three times doesn't tax Tymna.
	//
	// Named CommanderCastCounts (formerly CommanderTax) to mirror the
	// Python+Go partner spec in data/rules/FEATURE_GAP_LIST.md Tier 1 #5.
	// CommanderTax is retained as a transparent alias below for legacy
	// call sites and Python naming parity.
	CommanderCastCounts map[string]int

	// CommanderTax is an alias that mirrors CommanderCastCounts. Both
	// point at the SAME underlying map (set at newSeat / SetupCommander
	// time). Kept for source compatibility with existing tests + call
	// sites; prefer CommanderCastCounts in new code.
	CommanderTax map[string]int

	// LossReason is the human-readable reason the player lost. Set by the
	// SBA that caused the loss; empty when Lost==false.
	LossReason string

	// SBA704_5a_emitted prevents the 704.5a loss event from spamming the
	// stream each pass after a player is already lost. Mirrors Python's
	// `_sba_704_5a_emitted` attr.
	SBA704_5a_emitted bool

	// CommanderDamageNextSeq is the next EventLog index that §704.6c has
	// not yet scanned. Prevents double-counting across SBA passes.
	CommanderDamageNextSeq int

	// AttemptedEmptyDraw is set when Draw was called but the library was
	// empty. §704.5b will consume this in the SBA phase.
	AttemptedEmptyDraw bool

	// SpellsCastThisTurn is the per-seat count of spells THIS seat has
	// cast since its last untap. Resets at this seat's turn start, NOT
	// at every untap — an instant this seat casts on opponents' turns
	// still counts toward the seat's cast-count observability window.
	// Used by Storm-Kiln Artist / Young Pyromancer / Birgi / Monastery
	// Mentor / Niv-Mizzet Parun / Runaway Steam-Kin cast-trigger
	// observers (the "whenever YOU cast…" cards).
	SpellsCastThisTurn int

	// SpellsCastLastTurn is the previous-turn snapshot of
	// SpellsCastThisTurn. Some cards read "if you cast a spell last
	// turn…". Set by the untap step on this seat's own turn, before
	// zeroing SpellsCastThisTurn.
	SpellsCastLastTurn int

	// DescendedThisTurn is set to true the first time a permanent card
	// enters this seat's graveyard this turn (Ixalan descend mechanic).
	// Used by "if you've descended this turn" threshold checks. Reset at
	// this seat's own untap step. Writes are routed through MoveCard in
	// zone_move.go — direct zone-slice pokes do not set this flag.
	DescendedThisTurn bool

	// SkipUntapStep is true when the seat's untap step should be skipped
	// entirely (e.g. Stasis, Brine Elemental). CR §502.1: if the untap
	// step is skipped, permanents controlled by that player remain tapped.
	// Cleared at end of turn by the effect that set it, or by SBA/removal.
	SkipUntapStep bool

	// ControlledBy is the seat index of the player controlling this seat's
	// decisions (Mindslaver effect). When >= 0, all Hat calls for this
	// seat route to gs.Seats[ControlledBy].Hat instead. -1 (default) means
	// self-controlled.
	ControlledBy int

	// Companion holds the card designated as this seat's companion (CR
	// §702.139). Nil if no companion was declared. The companion starts
	// outside the game and can be moved to hand by paying {3}.
	Companion *Card

	// CompanionMoved is true once the companion has been moved to hand
	// (the 3-mana tax has been paid). Prevents paying twice.
	CompanionMoved bool

	// sbaSnapBuf is a reusable buffer for snapshotBattlefield to avoid
	// allocating a new slice on every SBA check pass (~5.7GB savings).
	sbaSnapBuf []*Permanent

	// Flags is an open-ended map for one-off per-seat flags, analogous to
	// Permanent.Flags. Used for transient player-level effects like
	// "protection from everything" (Teferi's Protection) or "your life
	// total can't change." Nil until first write.
	Flags map[string]int

	// Hat is the pluggable decision protocol for this seat (Phase 10).
	// Nil is valid — callers that want hat-driven decisions should set
	// this at seat construction or via NewGameStateWithHats. The engine
	// code paths that pre-date the Hat interface continue to call the
	// inline heuristic functions (pickAttackDefender, DeclareBlockers,
	// GetResponse, etc.); they also expose Hat-facing shims on GameState
	// that dispatch through `seat.Hat` when it is set.
	//
	// The engine MUST NEVER type-assert on Hat — that would defeat the
	// swappability contract. Treat the interface as opaque.
	Hat Hat
}

func newSeat(idx int) *Seat {
	// Shared tax/cast-counts map. Seat.CommanderCastCounts and Seat.CommanderTax
	// intentionally alias the same underlying map so legacy call sites can keep
	// using .CommanderTax while new code uses the spec-aligned name.
	castCounts := map[string]int{}
	return &Seat{
		Idx:                 idx,
		Life:                20,
		StartingLife:        20,
		ControlledBy:        -1,
		Library:             make([]*Card, 0, 60),
		Hand:                make([]*Card, 0, 10),
		Graveyard:           make([]*Card, 0, 16),
		Exile:               make([]*Card, 0, 8),
		Battlefield:         make([]*Permanent, 0, 16),
		CommandZone:         make([]*Card, 0, 2),
		CommanderDamage:     map[int]map[string]int{},
		CommanderCastCounts: castCounts,
		CommanderTax:        castCounts,
		Flags:               map[string]int{},
	}
}

// -----------------------------------------------------------------------------
// Card (a runtime card instance)
// -----------------------------------------------------------------------------

// Card is the lightweight runtime handle that points at an immutable
// CardAST and carries instance-specific metadata (owner, face-down flag,
// etc.). The engine never mutates CardAST pointers.
type Card struct {
	AST      *gameast.CardAST
	Name     string // cached for tokens / copies that may diverge from AST.Name
	Owner    int    // seat index of the owner (original owner, not controller)
	FaceDown bool

	// BasePT — set for creature tokens that don't have an AST. Phase 4
	// reads this during combat damage.
	BasePower     int
	BaseToughness int

	// Types cache — spelled out for tokens/copies. For real cards the
	// resolver computes on demand from AST.
	Types []string

	// Colors cache — e.g. ["R"], ["U","B"], or empty for colorless. For
	// tokens the caller populates at construction; for real cards the
	// value is populated by the corpus loader from the top-level colors
	// JSON field (Card.Colors lives on the RUNTIME Card, not on CardAST,
	// because colors can change via continuous effects — the AST-level
	// value is the characteristic-defining baseline). Cast-trigger
	// observers (Runaway Steam-Kin "whenever you cast a red spell") read
	// this.
	Colors []string

	// CMC — mana value of the card's printed mana cost. Used by cast
	// legality (mana payment) and storm copies (the copy inherits the
	// original's CMC for board-state tallies but pays nothing to exist).
	// 0 if unset; the cast path gracefully treats 0 as free.
	CMC int

	// TypeLine — the Scryfall-style printed type line ("Legendary
	// Creature — Human Wizard", "Instant", "Sorcery", "Artifact Token").
	// Used alongside Types for cast-trigger observers that filter on
	// "instant or sorcery" / "noncreature". Optional: Types is the
	// canonical source of truth; TypeLine is a convenience cache for
	// tokens/copies that want a human-readable string.
	TypeLine string

	// MDFC back-face data (CR §712.11). For modal double-faced cards, the
	// player chooses which face to cast from hand/command zone. These fields
	// hold the back face's characteristics so buildCastableList can offer
	// both faces. Non-MDFC cards leave these zero-valued.
	BackFaceCMC      int
	BackFaceName     string
	BackFaceTypes    []string
	BackFaceTypeLine string
	CastingBackFace  bool // transient: set by casting logic when back face chosen

	// IsCopy is true for card objects that were created as copies of other
	// cards (Fork, Twinflame, storm copies). CR §706.10: a copy of a spell
	// ceases to exist in any zone other than the stack. CR §704.5e: a copy
	// of a card in any zone other than the stack or battlefield ceases to
	// exist. SBA sba704_5e sweeps these from hand/graveyard/exile/library.
	IsCopy bool

	// OracleTextCache — lowercased oracle text, computed once on first
	// access via OracleTextLower. Avoids repeated string building +
	// ToLower in hot evaluator loops.
	OracleTextCache string
	oracleTextReady bool
}

func (c *Card) DeepCopy() *Card {
	if c == nil {
		return nil
	}
	cp := *c
	cp.Types = append([]string(nil), c.Types...)
	cp.Colors = append([]string(nil), c.Colors...)
	cp.BackFaceTypes = append([]string(nil), c.BackFaceTypes...)
	return &cp
}

// DisplayName returns the card's user-facing name, preferring the runtime
// override over the AST name.
func (c *Card) DisplayName() string {
	if c == nil {
		return "<nil>"
	}
	if c.Name != "" {
		return c.Name
	}
	if c.AST != nil {
		return c.AST.Name
	}
	return "<anonymous>"
}

// IsMDFC returns true if this card has a castable back face (modal DFC).
func (c *Card) IsMDFC() bool {
	return c != nil && c.BackFaceName != ""
}

// EffectiveCMC returns the mana value used for casting: BackFaceCMC
// when CastingBackFace is set, otherwise the front-face CMC.
func (c *Card) EffectiveCMC() int {
	if c != nil && c.CastingBackFace && c.BackFaceCMC > 0 {
		return c.BackFaceCMC
	}
	if c != nil {
		return c.CMC
	}
	return 0
}

// -----------------------------------------------------------------------------
// Permanent (on the battlefield)
// -----------------------------------------------------------------------------

// Permanent is a battlefield object. One Permanent per Card on the
// battlefield; a token has Card.AST==nil and uses BasePower/Toughness.
type Permanent struct {
	Card       *Card
	Controller int // seat index (CR §108.4 — may change via Gilded Drake, Threaten, etc.)

	// Owner is the seat that owns this permanent per CR §108.3 —
	// permanent and distinct from Controller. A Gilded Drake control
	// swap flips Controller but leaves Owner alone; §903.9b keys its
	// commander-return replacement off OWNER, not controller, which is
	// why it must survive control changes. Defaults to Controller at
	// ETB when callers leave it zero-valued; RegisterContinuousEffects-
	// ForPermanent / SetupCommanderGame backfill.
	Owner int

	Tapped        bool
	SummoningSick bool

	// PhasedOut is the §702.26 phasing flag. Phased-out permanents are
	// treated as though they do not exist — they can't be targeted, don't
	// trigger, and aren't counted. Auto-phase-in happens at the controller's
	// untap step before untapping (CR §502.1).
	PhasedOut bool

	// DoesNotUntap is true for permanents with "doesn't untap during your
	// untap step" (e.g. Mana Vault, Grim Monolith, Winter Orb targets).
	// CR §502.2 — UntapAll skips permanents with this flag set.
	DoesNotUntap bool

	// Timestamp is the §613 layer timestamp assigned at ETB. Required for
	// breaking ties when two layered effects conflict.
	Timestamp int

	// Counters: "+1/+1" -> N, "-1/-1" -> N, "loyalty" -> N, "charge" -> N.
	// Empty map if no counters; callers may nil-check and lazy-init.
	Counters map[string]int

	// AttachedTo: for Auras/Equipment, the permanent this is attached to.
	// nil for unattached permanents.
	AttachedTo *Permanent

	// Modifications: buffs applied "until end of turn". Phase 8 will handle
	// the full §613 layer stack; here we just accumulate until-EOT entries.
	Modifications []Modification

	// GrantedAbilities: ability names granted "until end of turn".
	GrantedAbilities []string

	// MarkedDamage: damage on the creature this turn (wiped at end of turn).
	MarkedDamage int

	// Flags: open-ended runtime flags ("cannot_be_countered", "hexproof",
	// "prevented"). Resolvers write here.
	Flags map[string]int

	// OriginalCard retains the card pointer that was on this permanent
	// before a clone/copy ETB handler swapped perm.Card to a DeepCopy.
	// Zone conservation uses this to account for the orphaned pointer.
	OriginalCard *Card

	// SaddlersThisTurn — for Mounts (CR §702.171). Records the permanents
	// that contributed power to saddling this mount this turn. Populated by
	// ActivateSaddle, cleared at end-of-turn cleanup. Used by triggers like
	// "The Gitrog, Ravenous Ride" that reference creatures that saddled the
	// mount this turn.
	SaddlersThisTurn []*Permanent

	// DFC / transform state (CR §712). Transformed is false while the
	// FRONT face is active (default at ETB per §712.2), true once
	// Transform has flipped the permanent to the BACK face. Every
	// transform event toggles this.
	Transformed bool
	// FrontFaceAST / BackFaceAST are the CardAST for each face of a
	// DFC. They're populated at ETB (from DFCFaceCache on the card, or
	// from a per-Permanent preload path). Both are nil for non-DFC
	// permanents. Transform swaps which one perm.Card.AST points at.
	FrontFaceAST *gameast.CardAST
	BackFaceAST  *gameast.CardAST
	// FrontFaceName / BackFaceName — the human-readable names of each
	// face. The front-face name is what players type in a deckfile;
	// the back-face name lives only on the oracle card.
	FrontFaceName string
	BackFaceName  string
}

// Modification is a runtime +X/+Y style buff with a duration tag.
type Modification struct {
	Power     int
	Toughness int
	Duration  string // "until_end_of_turn" / "permanent" / "until_your_next_turn"
	// Source timestamp — used by Phase 8 layer ordering.
	Timestamp int
}

// Power returns the permanent's current power, applying counters and
// until-EOT modifications on top of the base AST power. Full §613 layers
// are Phase 8 territory; this is an intentional MVP approximation.
func (p *Permanent) Power() int {
	if p == nil {
		return 0
	}
	// Face-down creatures are always 2/2 (CR §707.2).
	if p.Card != nil && p.Card.FaceDown {
		return 2
	}
	if p.Flags != nil && p.Flags["face_down"] == 1 {
		return 2
	}
	base := 0
	if p.Card != nil {
		base = p.Card.BasePower
	}
	// +1/+1 counters add, -1/-1 counters subtract.
	if p.Counters != nil {
		base += p.Counters["+1/+1"]
		base -= p.Counters["-1/-1"]
	}
	for _, m := range p.Modifications {
		base += m.Power
	}
	return base
}

// Toughness returns the current toughness.
func (p *Permanent) Toughness() int {
	if p == nil {
		return 0
	}
	// Face-down creatures are always 2/2 (CR §707.2).
	if p.Card != nil && p.Card.FaceDown {
		return 2
	}
	if p.Flags != nil && p.Flags["face_down"] == 1 {
		return 2
	}
	base := 0
	if p.Card != nil {
		base = p.Card.BaseToughness
	}
	if p.Counters != nil {
		base += p.Counters["+1/+1"]
		base -= p.Counters["-1/-1"]
	}
	for _, m := range p.Modifications {
		base += m.Toughness
	}
	return base
}

// IsCreature returns true if this permanent has the "creature" type.
// MVP: reads from Card.Types. A future Phase 8 layers pass will fold in
// type-add/type-remove effects.
// Per CR §702.176: while impending (has time counters), the permanent
// is NOT a creature even if it has the creature type.
func (p *Permanent) IsCreature() bool {
	if p != nil && p.Flags != nil && p.Flags["not_creature_while_impending"] == 1 {
		return false
	}
	return p.hasType("creature")
}

// hasType is the shared type-line predicate used by the 704.5 type checks.
// Case-insensitive match against Card.Types, which stores lowercased tokens
// per the resolver's ETB convention. A nil permanent or card returns false.
func (p *Permanent) hasType(t string) bool {
	if p == nil || p.Card == nil {
		return false
	}
	for _, x := range p.Card.Types {
		if x == t {
			return true
		}
	}
	return false
}

// IsPlaneswalker — §306.1 (planeswalker type).
func (p *Permanent) IsPlaneswalker() bool { return p.hasType("planeswalker") }

// IsLegendary — §205.4b (legendary supertype). Read from Card.Types since
// the resolver folds supertypes into that slice at ETB.
func (p *Permanent) IsLegendary() bool { return p.hasType("legendary") }

// IsWorld — §205.4f (world supertype). See §704.5k.
func (p *Permanent) IsWorld() bool { return p.hasType("world") }

// IsLand — §205.3g (land type).
func (p *Permanent) IsLand() bool { return p.hasType("land") }

// IsArtifact — §205.3g (artifact type).
func (p *Permanent) IsArtifact() bool { return p.hasType("artifact") }

// IsEnchantment — §205.3g (enchantment type).
func (p *Permanent) IsEnchantment() bool { return p.hasType("enchantment") }

// IsBattle — §205.3g / §310 (battle type). See §704.5v.
func (p *Permanent) IsBattle() bool { return p.hasType("battle") }

// IsAura — §205.3h / §303 (Aura is an enchantment subtype).
func (p *Permanent) IsAura() bool { return p.hasType("aura") }

// IsEquipment — §301.5 (Equipment artifact subtype). See §704.5n.
func (p *Permanent) IsEquipment() bool { return p.hasType("equipment") }

// IsFortification — §301.5a (Fortification artifact subtype).
func (p *Permanent) IsFortification() bool { return p.hasType("fortification") }

// IsSaga — §714 (enchantment subtype). See §704.5s.
func (p *Permanent) IsSaga() bool { return p.hasType("saga") }

// IsRole — Enchantment – Aura – Role. See §704.5y.
func (p *Permanent) IsRole() bool { return p.hasType("role") }

// IsToken — the resolver tags tokens with "token" in Card.Types when it
// spawns them via CreateToken. See §704.5d.
func (p *Permanent) IsToken() bool { return p.hasType("token") }

// IsIndestructible — §702.12 indestructible keyword. The Phase 3 runtime
// represents keyword grants via Permanent.Flags["indestructible"] > 0 or
// a matching entry in GrantedAbilities. We conservatively check both.
func (p *Permanent) IsIndestructible() bool {
	if p == nil {
		return false
	}
	if p.Flags != nil && (p.Flags["indestructible"] > 0 || p.Flags["kw:indestructible"] > 0) {
		return true
	}
	for _, a := range p.GrantedAbilities {
		if a == "indestructible" {
			return true
		}
	}
	return false
}

// AddCounter atomically adds N counters of the given kind. Negative N
// removes counters (floor 0).
func (p *Permanent) AddCounter(kind string, n int) {
	if p.Counters == nil {
		p.Counters = map[string]int{}
	}
	p.Counters[kind] += n
	if p.Counters[kind] < 0 {
		p.Counters[kind] = 0
	}
}

// -----------------------------------------------------------------------------
// StackItem (a spell or ability waiting to resolve)
// -----------------------------------------------------------------------------

// StackItem is an entry on the stack. Phase 5 will add proper
// priority/response handling; Phase 3 just needs the struct so
// CounterSpell can mark Countered=true.
type StackItem struct {
	ID         int
	Controller int
	Card       *Card          // for spells
	Source     *Permanent     // for abilities
	Effect     gameast.Effect // the effect to resolve on pop
	Targets    []Target       // resolved targets
	Countered  bool

	// Kind discriminates the stack item type for resolution dispatch.
	// Valid values:
	//   ""            — legacy: spell (Card != nil, Source == nil) or
	//                   triggered ability (Source != nil)
	//   "spell"       — explicit spell cast
	//   "activated"   — activated ability (CR §602.1d) pushed onto the
	//                   stack so opponents can respond before resolution
	//   "triggered"   — triggered ability (CR §603.3)
	// The empty string is backward-compatible: ResolveStackTop infers
	// spell vs triggered from Card/Source fields when Kind is empty.
	Kind string

	// AbilityIdx is the 0-based index into Source.Card.AST.Abilities
	// for activated-ability stack items. Used by ResolveStackTop to
	// dispatch to InvokeActivatedHook with the correct ability index.
	AbilityIdx int

	// IsCopy is true for spells that were put on the stack as COPIES
	// rather than CAST (CR §706.10). Storm copies, Twinflame copies,
	// Dualcaster Mage copies all set this. The distinction matters at
	// resolution time: a copy "ceases to exist" rather than going to
	// its owner's graveyard, because it isn't in any deck. A copy of a
	// permanent spell becomes a TOKEN copy of that permanent (§706.10a).
	IsCopy bool

	// CastZone is the zone the spell was cast from. Empty string (or
	// "hand") means cast from hand (the default). Other values:
	//   - "graveyard" (flashback, escape, Underworld Breach)
	//   - "exile"     (Misthollow Griffin, Squee the Immortal)
	//   - "library"   (Bolas's Citadel, Future Sight)
	//   - "command_zone" (commander cast)
	// Used by ResolveStackTop to determine post-resolution destination
	// (e.g. flashback → exile instead of graveyard).
	CastZone string

	// CostMeta carries cost-payment metadata from CastSpellWithCosts /
	// CastFromZone. Read by ResolveStackTop and resolvePermanentSpellETB
	// to drive downstream effects like evoke sacrifice triggers and
	// flashback exile-instead-of-graveyard.
	CostMeta map[string]interface{}

	// ChosenX is the value of X chosen by the caster when casting a spell
	// with X in its mana cost (CR §107.3). Stored on the stack item so
	// resolution can reference it (Walking Ballista ETB, Fireball damage,
	// etc.). Zero when the spell has no X in its cost.
	ChosenX int
}

// -----------------------------------------------------------------------------
// Event (structured log entry)
// -----------------------------------------------------------------------------

// Event is a single entry in the game log. Kind is the event
// discriminator (matches the effect kind when applicable).
type Event struct {
	Kind    string
	Seat    int    // primary actor, -1 if not applicable
	Target  int    // target seat if applicable, -1 otherwise
	Source  string // source card name for the event
	Amount  int    // numeric payload (damage dealt, cards drawn, etc.)
	Details map[string]interface{}
}

// -----------------------------------------------------------------------------
// Target — resolved target for an effect
// -----------------------------------------------------------------------------

// TargetKind discriminates Target.
type TargetKind int

const (
	TargetKindNone TargetKind = iota
	TargetKindSeat
	TargetKindPermanent
	TargetKindStackItem
	TargetKindCard // a card in a zone other than the battlefield (hand/graveyard/exile)
)

// Target wraps a resolved target. Exactly one of Seat/Permanent/Stack/Card
// is populated.
type Target struct {
	Kind      TargetKind
	Seat      int        // seat index, or -1
	Permanent *Permanent // non-nil if Kind == Permanent
	Stack     *StackItem
	Card      *Card
	Zone      string // for Card: "hand"/"graveyard"/"exile"/"library"
}

// SeatTarget returns (seatIdx, ok).
func (t Target) SeatTarget() (int, bool) {
	if t.Kind == TargetKindSeat {
		return t.Seat, true
	}
	return -1, false
}

// -----------------------------------------------------------------------------
// Utility: move-card-between-zones helpers
// -----------------------------------------------------------------------------

// removePermanent removes p from its controller's battlefield, returning true
// if found. Caller is responsible for placing p's Card in a destination zone.
func (gs *GameState) removePermanent(p *Permanent) bool {
	if p == nil || p.Controller < 0 || p.Controller >= len(gs.Seats) {
		return false
	}
	bf := gs.Seats[p.Controller].Battlefield
	for i, q := range bf {
		if q == p {
			gs.Seats[p.Controller].Battlefield = append(bf[:i], bf[i+1:]...)
			return true
		}
	}
	return false
}

// moveToZone appends c to the target seat's zone. Valid zones: "hand",
// "graveyard", "exile", "library_top", "library_bottom".
func (gs *GameState) moveToZone(seat int, c *Card, zone string) {
	if seat < 0 || seat >= len(gs.Seats) || c == nil {
		return
	}
	// CR §707.4 / §702.36: face-down cards are turned face-up when they
	// move to any zone other than the battlefield. Clear the FaceDown flag.
	if c.FaceDown {
		c.FaceDown = false
	}
	s := gs.Seats[seat]
	inSlice := func(slice []*Card) bool {
		for _, existing := range slice {
			if existing == c {
				return true
			}
		}
		return false
	}
	switch zone {
	case "hand":
		if inSlice(s.Hand) {
			return
		}
		s.Hand = append(s.Hand, c)
	case "graveyard":
		if inSlice(s.Graveyard) {
			return
		}
		s.Graveyard = append(s.Graveyard, c)
	case "exile":
		if inSlice(s.Exile) {
			return
		}
		s.Exile = append(s.Exile, c)
	case "library_top":
		s.Library = append([]*Card{c}, s.Library...)
	case "library_bottom":
		s.Library = append(s.Library, c)
	default:
		if inSlice(s.Graveyard) {
			return
		}
		s.Graveyard = append(s.Graveyard, c)
	}
}

// GainLife adds life to a seat and fires the life_gained trigger so that
// Sanguine Bond, Aetherflux Reservoir, etc. see every life gain event.
func GainLife(gs *GameState, seat, amount int, source string) {
	if gs == nil || amount <= 0 || seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}
	s.Life += amount
	FireCardTrigger(gs, "life_gained", map[string]interface{}{
		"seat":   seat,
		"amount": amount,
		"source": source,
	})
}

// drawOne pulls the top card of seat's library into its hand. Sets
// AttemptedEmptyDraw if the library is empty (SBA consumer for §704.5b).
// Returns (card, drew) where drew is false on empty library.
func (gs *GameState) drawOne(seat int) (*Card, bool) {
	if seat < 0 || seat >= len(gs.Seats) {
		return nil, false
	}
	s := gs.Seats[seat]
	if len(s.Library) == 0 {
		s.AttemptedEmptyDraw = true
		return nil, false
	}
	c := s.Library[0]
	MoveCard(gs, c, seat, "library", "hand", "draw")
	return c, true
}

// millOne pulls the top card of seat's library into its graveyard.
// Returns (card, milled) where milled is false on empty library.
func (gs *GameState) millOne(seat int) (*Card, bool) {
	if seat < 0 || seat >= len(gs.Seats) {
		return nil, false
	}
	s := gs.Seats[seat]
	if len(s.Library) == 0 {
		return nil, false
	}
	c := s.Library[0]
	MoveCard(gs, c, seat, "library", "graveyard", "mill")
	return c, true
}

// Snapshot emits a full game-state "state" event so downstream viewers /
// parity probes can resync. Mirrors Python Game.snapshot(). Called at
// turn end, game start, and game over.
func (gs *GameState) Snapshot() {
	if gs == nil {
		return
	}
	seats := make([]map[string]interface{}, 0, len(gs.Seats))
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		graveyardNames := make([]string, 0, len(s.Graveyard))
		for _, c := range s.Graveyard {
			if c != nil {
				graveyardNames = append(graveyardNames, c.DisplayName())
			}
		}
		bfPerms := make([]map[string]interface{}, 0, len(s.Battlefield))
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			pm := map[string]interface{}{
				"name":          p.Card.DisplayName(),
				"tapped":        p.Tapped,
				"summoning_sick": p.SummoningSick,
				"power":         p.Card.BasePower,
				"toughness":     p.Card.BaseToughness,
				"damage":        p.MarkedDamage,
			}
			bfPerms = append(bfPerms, pm)
		}
		seatState := map[string]interface{}{
			"idx":         s.Idx,
			"life":        s.Life,
			"hand":        len(s.Hand),
			"library":     len(s.Library),
			"graveyard":   graveyardNames,
			"battlefield": bfPerms,
			"mana_pool":   s.ManaPool,
			"lost":        s.Lost,
		}
		seats = append(seats, seatState)
	}
	details := map[string]interface{}{
		"seats": seats,
		"turn":  gs.Turn,
		"phase": gs.Phase,
		"step":  gs.Step,
	}
	// Back-compat: old viewers read seat_0/seat_1 directly for 2-player.
	if len(seats) == 2 {
		details["seat_0"] = seats[0]
		details["seat_1"] = seats[1]
	}
	gs.LogEvent(Event{
		Kind:    "state",
		Seat:    gs.Active,
		Details: details,
	})
}
