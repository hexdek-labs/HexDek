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
	"fmt"
	"math/rand"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine/counters"
	"github.com/hexdek/hexdek/internal/gameengine/instanceid"
	"github.com/hexdek/hexdek/internal/judge"
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

	// Seed is the int64 value the caller used to construct gs.Rng. The
	// engine never reads this — it's an opaque audit trail captured for
	// the replay / anti-cheat pipeline (Phase 1: capture-only). Callers
	// who can't surface a seed (e.g. forwarded math/rand.Rand without a
	// known seed) should leave Seed=0 and consumers must treat 0 as
	// "unknown" rather than "seeded with 0".
	Seed int64

	// EventsLogged counts every LogEvent call for the lifetime of the
	// game, independent of EventPolicy. It is a deterministic
	// work-volume proxy: the tournament turn runner budgets each turn
	// by events logged instead of wall-clock time, so a pathological
	// turn fast-forwards identically on every host (seed-replay
	// determinism — r62).
	EventsLogged int64

	// Legality is the ride-along rules-legality validator (legality.go).
	// nil (the default) = off, zero behavior change — every hook is a
	// nil-receiver no-op. Opted into by runners that want live CR
	// legality auditing (loki -legality, tests). Deliberately NOT copied
	// by CloneForRollout: hypothetical rollout lines must not pollute
	// the violation stream.
	Legality *LegalityValidator

	// TempControlGrants holds until-end-of-turn control steals awaiting
	// cleanup-step reversal (r63 §108.3 owner-immutability work; see
	// control_revert.go).
	TempControlGrants []TempControlGrant

	// SeatOutcome is the per-seat win/loss self-checker (r63 phase 1,
	// owner design). nil (default) = off, zero overhead — same
	// ride-along pattern as Legality. Not copied by CloneForRollout.
	SeatOutcome *SeatOutcomeChecker

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

	// EventLog is the structured event log — the ONE event store
	// (consolidation step 3). Every resolver handler that mutates state
	// emits at least one Event via LogEvent; what this slice retains is
	// governed by EventPolicy. Tests assert against this slice; future
	// agents (Phase 10 policy) read it for credit assignment. Always in
	// chronological order under every policy.
	EventLog []Event

	// EventPolicy selects the EventLog retention policy (consolidation
	// step 3 — replaces the RetainEvents bool + lastEvent single-slot
	// dual path whose silent-drop split caused the goldilocks 1,795
	// keyword_dead failures):
	//   EventLogFull — retain everything (50k cap); audit / test mode.
	//   EventLogRing — bounded recent history: at least the most recent
	//                  EventRingSize events, never more than 2× that
	//                  (amortized compaction keeps EventLog a plain
	//                  chronological slice).
	//   EventLogNone — retain nothing; hats still observe every event.
	// The ZERO VALUE ("") retains everything: a struct-literal fixture
	// that forgets to set the policy gets full retention, so the
	// goldilocks silent-drop class cannot recur by omission.
	EventPolicy EventLogPolicy

	// EventRingSize bounds EventLog under EventLogRing. 0 means the
	// defaultEventRingSize. Ignored under other policies.
	EventRingSize int

	// Cards points at the loaded corpus. Resolver handlers reach in via
	// Cards.Get(name) when they need to spawn token cards or reveal cards.
	Cards *astload.Corpus

	// EffectTimestamp is a monotonically increasing counter assigned to
	// permanents as they enter the battlefield. §613 layer application
	// uses it to break ordering ties between effects of the same layer.
	EffectTimestamp int

	// IIDMinter owns the deterministic per-(game, seat) sequence
	// counters that drive InstanceID.seq5. Initialized in NewGameState;
	// callers that build a GameState by struct literal (older tests)
	// will see this nil — Mint helpers guard nil and produce empty IDs
	// in that case (legacy-mode backwards compat).
	// See docs/instanceid-system-v2-r60.md §3.
	IIDMinter *instanceid.Minter

	// IIDEnablerStack is the resolve-time stack of AbilityInstance
	// InstanceIDs for the currently-resolving spell/ability frames. Push
	// at ResolveStackTop / activation-resolve entry, pop on exit. Token
	// and copy mints during resolution call currentMintEnablerID(gs) to
	// stamp the top frame's ID as their EnablerInstanceID per
	// docs/instanceid-system-v2-r60.md §4 mint-path lineage requirement.
	// Empty when nothing is resolving (deck-load / setup-time mints get
	// empty enabler — that's the OG path, which doesn't require one).
	IIDEnablerStack []string

	// MintedInstanceIDs is the master set of every InstanceID ever minted
	// during this game (OG deck-load + TK token + CP copy + AB ability).
	// Populated by MintOG/Token/Copy + NewAbilityInstance. Used by the
	// Phase 4 ZoneConservation invariant per
	// docs/instanceid-system-v2-r60.md §13: the census of present IDs at
	// each game tick must equal (Minted) - (Ceased) - (RemovedFromGame).
	// Initialized in NewGameState; nil-safe (Phase 4 helpers lazy-init).
	MintedInstanceIDs map[string]struct{}

	// MintedInstanceIDNames is a Phase D diagnostic side-map: every
	// minted InstanceID maps to the Card.DisplayName at mint time.
	// Used by the ZoneConservation disappearance arm to include the
	// card name in violation messages so root-cause walks can identify
	// the offending engine path without re-running with diagnostic
	// instrumentation. Populated by RecordMintedInstanceID; never
	// cleared even on cease (cessation doesn't invalidate the name —
	// the post-mortem still needs to know what was minted).
	MintedInstanceIDNames map[string]string

	// MintedInstanceIDOwners records each minted card's owner at mint
	// time (r63 §108.3 owner-immutability invariant): ownership is
	// write-once, so any later Card.Owner divergence is engine
	// corruption. Tokens record their creator (§110.5a).
	MintedInstanceIDOwners map[string]int

	// CeasedInstanceIDs marks IDs no longer expected to appear in any
	// zone census. Populated at:
	//   - §707.10 — spell-copy cease at resolution (stack.go IsCopy arm)
	//   - §704.5d / §111.8 — token cessation on LTB (zone_change.go,
	//     destroyPermSBA, sacrificePermSBA, BouncePermanent, ExilePermanent)
	//   - §800.4a — owned-object removal when a seat leaves (multiplayer.go
	//     HandleSeatElimination)
	//   - §720.4 — Karn restart (rare)
	// Used in conjunction with MintedInstanceIDs by checkZoneConservation.
	CeasedInstanceIDs map[string]struct{}

	// ResolvingCards tracks the *Card of each stack item popped by
	// ResolveStackTop whose resolution is still executing — the
	// stack→destination-zone limbo window where the card is absent from
	// every conventional zone. Both checkZoneConservationByInstanceID and
	// collectPresentInstanceIDsForSweep count it as zone-presence: a
	// resolution that itself eliminates a seat or ends the game (mass
	// life-set, alt-win) reaches SweepOrphanedInstanceIDs MID-resolution
	// via CheckEnd → HandleSeatElimination, and without this the sweep
	// ceases the in-flight card's ID as an orphan — the subsequent
	// graveyard routing then reads as census fabrication forever after
	// (seed 7777 game 76, Biorhythm, r63). Pushed/popped LIFO by
	// ResolveStackTop; empty whenever the engine is at rest.
	ResolvingCards []*Card

	// TokenMintEvents is the Phase 5 audit-trail buffer per
	// docs/instanceid-system-v2-r60.md §6. Each FireCreateTokenEvent
	// resolution appends one TokenMintEvent recording the §616
	// replacement chain (Mondrak / Anointed / Doubling Season order),
	// the base-vs-final count, and every minted token's InstanceID.
	// Capped at 256 entries (oldest half dropped on overflow) so long
	// games + fuzzers don't accumulate unbounded memory.
	TokenMintEvents []TokenMintEvent

	// PendingTokenMintChain is a single-event sidecar populated by
	// FireCreateTokenEvent and consumed by resolveCreateToken /
	// resolveCreateTokenCopy when assembling the TokenMintEvent. Keeping
	// it on gs avoids changing FireCreateTokenEvent's return signature
	// (which has many callers). Cleared after every resolveCreateToken*
	// finishes folding it in.
	PendingTokenMintChain []ReplacementRef

	// SubsystemHooks is the InstanceID Phase 6 dormant-hook registry
	// per docs/instanceid-system-v2-r60.md §4.4 / §9. Populated by
	// RegisterSubsystemHooks at game-start with ten entries, one per
	// optional MTG subsystem (Day/Night, Monarch, Initiative, Ascend,
	// Dungeon, RingTempts, Energy, Experience, Foretell, CityBlessing).
	// Each hook stays Active=false until a card-zone-change event
	// matches its predicate, at which point CheckSubsystemActivation
	// flips Active=true and runs OnActivate. Once active, the subsystem
	// remains active for the rest of the game (sticky per CR).
	SubsystemHooks []*DormantHook

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

	// CounterThresholds is the counter-accumulator threshold registry
	// (R58). Per-card handlers register an effect that fires when a
	// source permanent's Counters[X] reaches a threshold at upkeep
	// evaluation. Helix Pinnacle (tower → 100 → win), Darksteel Reactor
	// (charge → 20 → win), Azor's Elocutors (filibuster → 5 → win),
	// Quest for Ula's Temple (quest → 3 → upkeep Kraken from hand) all
	// share this shape. See counter_threshold.go for the effect
	// struct + RegisterCounterThreshold /
	// UnregisterCounterThresholdsForPermanent /
	// EvaluateCounterThresholds APIs.
	CounterThresholds []*CounterThresholdEffect

	// ZoneCastPolicies is the §614 filter-driven zone-cast policy
	// registry (R55). Complements gs.ZoneCastGrants (per-*Card-pointer)
	// with predicate-driven multi-card grants — Aluren-style "any
	// player may cast creature spells with mana value 3 or less from
	// hand without paying their mana cost", Karn the Great Creator's
	// wishboard, Tinybones-style "cast from opponent's graveyard."
	// Populated by ETB hooks calling RegisterZoneCastPolicy; drained
	// on LTB via UnregisterZoneCastPoliciesForPermanent. Consulted by
	// FindZoneCastPolicy at cast-legality time. See
	// zone_cast_policy.go for the policy shape and semantics.
	ZoneCastPolicies []*ZoneCastPolicy

	// MiracleGrants is the granted-miracle registry (CR §702.94). A
	// permanent reading "Cards in your hand have miracle {N}" registers a
	// grant here via RegisterMiracleGrant; it is CONSULTED ONLY at draw
	// time (MaybeOpenMiracleWindow, the first draw of the turn), so granting
	// miracle to the whole hand still obeys §702.94a — only the single card
	// that is the FIRST card drawn this turn opens a castable window. See
	// miracle_grant.go.
	MiracleGrants []*MiracleGrant

	// DamageReplacements is the §614 damage-replacement registry
	// (R54). Populated by ETB hooks that call
	// RegisterDamageReplacement (Torbran +2, Sokrates dialogue,
	// Kuja Flare-Star double, Lightning stagger, Neriv ETB-this-turn
	// double, etc.); drained on LTB via
	// UnregisterDamageReplacementsForPermanent. Walked by
	// ApplyDamageReplacement, which is consulted by
	// applyCombatDamageToPlayer / applyCombatDamageToCreature
	// (combat.go) and DealDamage (state.go) before the actual damage
	// is applied. See docs/percard-stub-census-r53.md §"DealDamage
	// replacement hooks" for the 4-card slate this primitive unblocks.
	DamageReplacements []*DamageReplacement

	// ContinuousEffects is the §613 layer-system registry (Phase 8).
	// Every static ability that changes a copiable value (copy effects,
	// control change, text change, type-add/remove, color change, ability
	// add/remove, set/modify/switch P/T) lives here while its source is
	// on the battlefield. GetEffectiveCharacteristics walks this slice
	// in layer order (1→7e) to compute the current characteristics of a
	// permanent. Populated by RegisterContinuousEffect; drained on LTB
	// by UnregisterContinuousEffectsForPermanent.
	ContinuousEffects []*ContinuousEffect

	// SeatWardEffects is the r60 anthem-style ward registry (Hexing
	// Squelcher / Indomitable Might-class "all your creatures have
	// ward N" continuous effects). One entry per source permanent —
	// the per-target instance is materialized inside SeatWardCostsFor
	// at lookup time so a control change on the source naturally moves
	// which seat's creatures benefit. Populated by AddSeatWardCost;
	// drained on LTB via RemoveSeatWardCostsForSource. See
	// ward_seat_scope.go.
	SeatWardEffects []*SeatWardEntry

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

	// DelayedAbilityInstances is the InstanceID Phase 8 delayed-trigger
	// pool per docs/instanceid-system-v2-r60.md §11. Parallel to
	// DelayedTriggers (callback-based legacy surface): this pool holds
	// *AbilityInstance entries with DelayedUntil populated, preserving
	// SourceInstanceID lineage across the trigger's lifetime. The pool
	// walker is FireDelayedAbilityPool — called from event-emission
	// sites and phase boundaries — which matches each entry's
	// DelayedCondition against the current event and moves matching
	// abilities pool→stack. Source-independence per §112.7a: an
	// AbilityInstance in this pool keeps firing even after its source
	// permanent leaves play.
	DelayedAbilityInstances []*AbilityInstance

	// PendingExtraCombats is a FIFO queue of extra combat phases waiting
	// to be played out, in the order they were granted. Aggravated
	// Assault / Najeela / Seize the Day push vanilla entries (no
	// restriction, no OnBegin hook); Bumi Unleashed pushes a
	// "land_creatures_only" entry; Moraug pushes a vanilla entry with an
	// "untap all your creatures" OnBegin hook. take_turn's combat loop
	// pops the front, sets gs.CurrentCombatRestriction from that entry,
	// fires OnBegin if present, then runs a normal combat phase.
	//
	// Replaces the prior `int` counter — the counter form couldn't carry
	// per-combat metadata (restrictions, OnBegin hooks) which cards like
	// Bumi Unleashed and Moraug need. len(slice) replaces the old "> 0"
	// check; popping the front replaces the old "--".
	PendingExtraCombats []PendingExtraCombat

	// CurrentCombatRestriction is non-empty while the engine is in the
	// middle of running a restricted extra combat (e.g. "land_creatures_only"
	// for Bumi Unleashed). Read by DeclareAttackers when building the
	// legal attacker pool. Cleared at end_of_combat for that phase.
	CurrentCombatRestriction string

	// SpellsCastThisTurn is the GLOBAL cast counter read by Storm (CR
	// §702.40) — the number of spells cast this turn across ALL seats.
	// Incremented by CastSpell (and commander-zone casts) after the spell
	// successfully lands on the stack. Storm copies do NOT increment per
	// §707.10 (a copy isn't cast). Resets to 0 at each untap step.
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
	// is about to end — used by §730.2a (day↔night transition, which
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

	// ManaPoolExemptions registers permanents that prevent some or all
	// mana from being emptied at phase/step boundaries (CR §106.4 phase
	// drain exception list). Examples:
	//   - Omnath, Locus of Mana: green mana doesn't empty (controller-
	//     scoped, Colors={"G"}, Seat=controller)
	//   - Upwelling: all mana doesn't empty for every player (Seat=-1,
	//     Colors={"any"})
	// Populated by per_card OnETB hooks via RegisterManaPoolExemption;
	// cleared by UnregisterManaPoolExemptionForPerm at LTB. Read by
	// PoolExemptColors at DrainAllPools time (CR §106.4).
	ManaPoolExemptions []*ManaPoolExemption

	// ManaPoolColorlessConverters registers permanents whose controller's
	// would-be-lost unspent mana becomes colorless instead of emptying at
	// phase/step boundaries (Kruphix, God of Horizons; Horizon Stone). A
	// converter RECOLORS the drained mana to {C} and keeps it — distinct
	// from ManaPoolExemptions, which retain mana in its original color.
	// Populated by per_card OnETB via RegisterManaColorlessConverter,
	// cleared by UnregisterManaColorlessConverterForPerm at LTB, read by
	// PoolConvertsToColorless at DrainAllPools time (CR §106.4 / §616).
	ManaPoolColorlessConverters []*ManaPoolColorlessConverter

	// PoolDrivenPTEffects counts the live layer-7c continuous effects
	// whose P/T value is read from the mana pool (Omnath, Locus of Mana:
	// "+1/+1 for each unspent green mana"). When > 0, the mana mutators
	// (AddMana / DrainAllPools) refresh the characteristics cache so the
	// P/T tracks the pool. Gated so the common pool-CDA-free game pays
	// nothing. Incremented by RegisterPoolDrivenSelfBoostPT, decremented
	// by UnregisterPoolDrivenSelfBoostPT (CR §613.2 live-value reads).
	PoolDrivenPTEffects int

	// GraveyardFlashbackGrants is the registry of source-driven
	// "cards in your graveyard have flashback" continuous effects
	// (e.g. Iroh, Grand Lotus; Lier, Disciple of the Drowned). Unlike
	// ZoneCastGrants, these are not keyed by card pointer — they are
	// predicates evaluated dynamically against any card in the
	// controller's graveyard at flashback-cast time. Lifecycle is tied
	// to the source permanent's Timestamp; LTB clears matching grants
	// via ExpireGraveyardFlashbackGrantsBySource.
	GraveyardFlashbackGrants []*GraveyardFlashbackGrant

	// MayhemDiscards tracks which cards were discarded on which turn. CR
	// §702.187: a card with mayhem may be cast from its owner's graveyard
	// for its mayhem cost "if you discarded it this turn." Keyed by the
	// discarded card pointer; value is the turn number on which it was
	// discarded. DiscardCard writes here; CastMayhem checks
	// `gs.MayhemDiscards[card] == gs.Turn`. Cleared in end-of-turn cleanup
	// so the map doesn't grow unbounded.
	MayhemDiscards map[*Card]int

	// MadnessExile tracks cards exiled via the Madness §702.34a
	// discard replacement. Keyed by the exiled *Card pointer; value
	// captures the seat that discarded and the turn it happened on.
	// Written by OnDiscardMadness, read by CastWithMadness +
	// ResolveMadnessWindow, cleared when the window is consumed
	// (cast taken) or routed to graveyard (window declined).
	MadnessExile map[*Card]*MadnessWindow

	// PlotExile tracks cards exiled via the Plot activated ability (CR
	// §702.172 / Outlaws of Thunder Junction). Keyed by the exiled card
	// pointer; value is a *PlotMeta capturing the seat that activated
	// plot, the turn on which plot was activated, and the same value
	// echoed as ExiledAt for handler ergonomics. Written by
	// ActivatePlot; read by CastPlot to enforce the "on a later turn"
	// gate (gs.Turn > meta.Turn). Entries are removed when the plot
	// card is cast from exile; entries for cards that leave exile by
	// any other route (e.g. opponent's exile-zone wipe) become stale
	// and are cleared on the next CheckEnd / EndOfTurnCleanup sweep
	// that detects the card is no longer in any exile zone.
	PlotExile map[*Card]*PlotMeta

	// ParadigmExile tracks cards exiled with the Paradigm keyword ability
	// (Secrets of Strixhaven). At the beginning of the controller's first
	// main phase, the engine creates a copy of each paradigm card and casts
	// it without paying its mana cost. The original stays in exile
	// permanently. Keyed by seat index.
	ParadigmExile map[int][]*Card

	// BeholdRegistry tracks per-seat per-quality "beheld this turn"
	// counts. CR §701.4 (Bloomburrow Behold). Outer key: seat index.
	// Inner key: lowercased quality name (typically a creature subtype,
	// e.g. "dragon", "squirrel"). Value: how many times that quality has
	// been beheld this turn by that seat — most cards only key off >0
	// but the counter form lets handlers that care about repeated
	// beholds (e.g. "second time you behold a Dragon this turn") do so
	// without rewiring. Written by Behold, read by HasBeheld, cleared
	// by ClearBeholdRegistry at the start of each turn (UntapAll).
	BeholdRegistry map[int]map[string]int

	// CardFirstPlayed maps a card display name to the turn number on
	// which it first resolved as a spell during this game. Populated by
	// ResolveStackTop only for spell stack items (item.Card != nil and
	// item.Source == nil) and only on the first appearance per name —
	// re-casts (storm, recursion) don't overwrite. Heimdall reads this
	// at game end to feed the card_stats.avg_turn_played aggregate so
	// the analytics layer can answer "Sol Ring is played turn 1 in N%
	// of games it appears in" queries.
	CardFirstPlayed map[string]int

	// MulliganHistory captures the result of the London-mulligan procedure
	// per seat (CR §103.5). Index by seat; nil/empty when mulligans were
	// not run (engine-internal unit tests that hand-build a GameState).
	// Populated by tournament.RunLondonMulligan at the end of each seat's
	// mulligan resolution and read by Heimdall's ExtractMulliganStats so
	// downstream summaries can surface "seat 2 took 3 mulligans and kept
	// a 4-card hand" without requiring event-log retention.
	//
	// OpeningHandNames captures the FINAL post-bottom hand — the cards
	// actually entering turn 1 — not the pre-bottom hand. Snapshot only,
	// not used by the engine for any decisions.
	MulliganHistory []SeatMulliganStats

	// triggerBatchDepth and pendingTriggers implement CR §603.3b batched
	// trigger placement. When BeginTriggerBatch opens a batch (depth 0→1),
	// subsequent PushTriggeredAbility calls append to pendingTriggers
	// instead of push+resolve. EndTriggerBatch on the outermost frame
	// orders the batch APNAP + controller-choice via OrderTriggersAPNAP,
	// pushes via PushSimultaneousTriggers, then drains the stack through
	// priority/resolve until those items are gone. Re-entrant: nested
	// fire sites observe an open batch and just append. See trigger_batch.go.
	triggerBatchDepth int
	pendingTriggers   []*StackItem

	// announcedTargets exposes the currently-resolving StackItem's
	// announcement-time targets (CR §601.2c / §602.2b) to PickTarget so
	// resolution-time effect handlers honor the targets chosen — and
	// validated, warded, and §608.2b-checked — at announcement instead of
	// re-running the engine policy pick. Set/restored by ResolveStackTop
	// around each resolution frame; transient, never serialized or cloned.
	announcedTargets []Target
}

// Day/Night state constants (CR §726).
const (
	DayNightNeither = "neither"
	DayNightDay     = "day"
	DayNightNight   = "night"
)

// SeatMulliganStats captures the per-seat outcome of CR §103.5 (the
// London mulligan procedure) for one game. Stored on
// GameState.MulliganHistory at the end of each seat's mulligan
// resolution by tournament.RunLondonMulligan. Read by Heimdall's
// ExtractMulliganStats to surface "how many mulligans + what kind of
// hand was kept" in GameSummary, independent of event-log retention.
//
// OpeningHand is the FINAL post-bottom hand — the cards entering
// turn 1 — not any intermediate draw. Name + Types are captured by
// value so the snapshot survives later zone changes / token reuse on
// the same *Card pointers, and downstream consumers (Heimdall's
// keepable evaluator) don't need to hold the original *Card.
type SeatMulliganStats struct {
	MulligansTaken int                 `json:"mulligans_taken"`
	OpeningHand    []MulliganHandEntry `json:"opening_hand,omitempty"`
}

// MulliganHandEntry is one card captured into a SeatMulliganStats
// snapshot. Types holds the runtime Card.Types slice (lowercase
// engine-canonical type strings: "land", "creature", "instant", etc.)
// so consumers can classify the card without re-resolving against the
// corpus.
type MulliganHandEntry struct {
	Name  string   `json:"name"`
	Types []string `json:"types,omitempty"`
}

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
		// CR §800.4a: a delayed trigger controlled by a player who has left
		// the game ceases to exist — it must not fire (HandleSeatElimination
		// purges these, but guard here too for triggers whose controller
		// leaves via a path that doesn't run the sweep, mirroring the
		// SeatHasLeftGame chokepoints on mana/trigger formation).
		if SeatHasLeftGame(gs, dt.ControllerSeat) {
			dt.Consumed = true
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

// NewGameStateSeeded constructs a GameState from an int64 seed,
// building the RNG and recording the seed for replay capture in one
// shot. Equivalent to:
//
//	gs := NewGameState(seatCount, rand.New(rand.NewSource(seed)), corpus)
//	gs.Seed = seed
//
// Prefer this helper at call sites that own the seed value (showmatch
// game runners, tournament drivers, replay seeders) so the seed
// doesn't get lost on its way to the persistence layer.
func NewGameStateSeeded(seatCount int, seed int64, corpus *astload.Corpus) *GameState {
	gs := NewGameState(seatCount, rand.New(rand.NewSource(seed)), corpus)
	gs.Seed = seed
	return gs
}

// strictCensusDefault flips the InstanceID Phase 4+ ZoneConservation
// strict-census disappearance check on/off for every newly-created
// GameState. Defaults to true post-gap-walk (PR — this branch): the
// 2.9M-hit disappearance arm surfaced by PR #755's first 25k sweep
// has been closed by the gap-walk's zone-purge + re-mint backstops
// (instanceid_gap_walk.go), so the strict arm now produces a clean
// signal at production-grade depths. Callers (mainly tests on
// struct-literal GameStates) can still flip back to off via
// SetStrictCensusDefault(false) if they want pre-Phase-4 behavior.
var strictCensusDefault = true

// SetStrictCensusDefault toggles whether NewGameState stamps
// gs.Flags["instanceid_strict_census"] = 1 on every freshly-built
// state. Default false; flipping to true enables the InstanceID Phase
// 4+ "card disappeared" check per docs/instanceid-system-v2-r60.md §13.
// Intended for Loki/CI sweep runs that want the strict view.
func SetStrictCensusDefault(on bool) {
	strictCensusDefault = on
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
	flags := map[string]int{}
	if strictCensusDefault {
		flags["instanceid_strict_census"] = 1
	}
	gs := &GameState{
		Seats:             seats,
		Rng:               rng,
		Turn:              1,
		Phase:             "beginning",
		Step:              "untap",
		Active:            0,
		Cards:             corpus,
		Flags:             flags,
		EventLog:          make([]Event, 0, 64),
		EventPolicy:       EventLogFull,
		DayNight:          DayNightNeither,
		IIDMinter:         instanceid.NewMinter(seatCount),
		MintedInstanceIDs: map[string]struct{}{},
		CeasedInstanceIDs: map[string]struct{}{},
	}
	RegisterSubsystemHooks(gs)
	return gs
}

// NextTimestamp issues the next §613 layer timestamp and advances the
// counter. Monotonic; never re-used across a single game.
func (gs *GameState) NextTimestamp() int {
	gs.EffectTimestamp++
	return gs.EffectTimestamp
}

// EventLogPolicy selects what LogEvent retains in gs.EventLog
// (consolidation step 3). The retention decision is the ONLY thing the
// policy changes: EventsLogged counts and the Hat broadcast happen for
// every event under every policy.
type EventLogPolicy string

const (
	// EventLogFull retains every event up to maxEventLog (50k), then
	// stops appending (first-50k-kept, matching the historical cap
	// behavior). The zero value "" behaves identically — full retention
	// is the safe default a fixture gets by omission.
	EventLogFull EventLogPolicy = "full"
	// EventLogRing retains a bounded recent window: at least the most
	// recent EventRingSize events (defaultEventRingSize when unset) and
	// never more than twice that, always in chronological order. Use
	// RecentEvents(n) for an exact-width tail.
	EventLogRing EventLogPolicy = "ring"
	// EventLogNone retains nothing — the replacement for the old
	// RetainEvents=false mode (tournament non-audit runs, rollout
	// clones), saving ~9GB/game of allocation pressure. Hats still
	// observe every event.
	EventLogNone EventLogPolicy = "none"
)

const (
	// maxEventLog caps EventLogFull retention.
	maxEventLog = 50000
	// defaultEventRingSize is the EventLogRing window when
	// gs.EventRingSize is 0.
	defaultEventRingSize = 512
)

// LogEvent records a structured event per gs.EventPolicy and broadcasts
// it to every seat's Hat. Keeping this centralized means later phases
// can add instrumentation (timestamps, invariant checks) in one place.
//
// Phase 10: after the event is persisted, broadcast it to every seat's
// Hat (if any). Hats use this to drive archetype detection, mode
// transitions, and other adaptive behavior. The broadcast is best-effort
// — a nil-safe loop that tolerates seats without a Hat. Hats must not
// mutate the GameState from ObserveEvent (contract, not enforced) and
// must not retain the *Event past the call (it may point at a stack
// copy that is not stored anywhere).
func (gs *GameState) LogEvent(ev Event) {
	gs.EventsLogged++
	evPtr := &ev
	switch gs.EventPolicy {
	case EventLogNone:
		// Retain nothing; hats observe the local copy.
	case EventLogRing:
		n := gs.EventRingSize
		if n <= 0 {
			n = defaultEventRingSize
		}
		// Amortized ring: append freely, and once the slice holds 2n
		// events compact the most recent n to the front. EventLog stays
		// a plain chronological slice under every policy, so direct
		// readers (range / index) never see wrapped order.
		if len(gs.EventLog) >= 2*n {
			copied := copy(gs.EventLog, gs.EventLog[len(gs.EventLog)-n+1:])
			gs.EventLog = gs.EventLog[:copied]
		}
		gs.EventLog = append(gs.EventLog, ev)
		evPtr = &gs.EventLog[len(gs.EventLog)-1]
	default: // EventLogFull and the zero value: full retention.
		if len(gs.EventLog) < maxEventLog {
			gs.EventLog = append(gs.EventLog, ev)
			evPtr = &gs.EventLog[len(gs.EventLog)-1]
		}
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

// RecentEvents returns the retained tail of the event log, at most n
// events, newest last. Convenience for consumers that only need recent
// history and shouldn't care which retention policy is active.
func (gs *GameState) RecentEvents(n int) []Event {
	if n <= 0 || len(gs.EventLog) == 0 {
		return nil
	}
	if n > len(gs.EventLog) {
		n = len(gs.EventLog)
	}
	return gs.EventLog[len(gs.EventLog)-n:]
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
// TurnCounters — centralized per-seat, per-turn tracking
// -----------------------------------------------------------------------------

// CastRecord captures metadata about a single spell cast this turn.
// Used by cards that care about "second instant or sorcery", "spell with
// mana value 5 or greater", "X spell", etc.
type CastRecord struct {
	CardName  string
	Types     []string // card types at time of cast
	ManaValue int      // total mana value (CMC) of the spell
	XCost     bool     // true if the spell had X in its mana cost
	XValue    int      // the actual X value paid (0 if !XCost)
}

// TurnCounters holds all per-turn counters for a single seat. Core engine
// functions (GainLife, drawOne, SacrificePermanent, etc.) increment these
// directly. Per_card handlers read them instead of rolling their own flags.
// Reset once per turn via Reset() in UntapAll.
type TurnCounters struct {
	LifeGained          int  // total life gained this turn (from GainLife)
	LifeLost            int  // total life lost this turn (from LoseLife + combat damage)
	DamageReceived      int  // total damage received this turn (combat + noncombat)
	LifePaid            int  // life paid as costs this turn (distinct from LifeLost per CR §118)
	CardsDrawn          int  // cards drawn this turn
	SpellsCast          int  // spells cast this turn
	CreaturesEntered    int  // creatures that entered the battlefield this turn
	ArtifactsEntered    int  // artifacts that entered the battlefield this turn
	EnchantmentsEntered int  // enchantments that entered the battlefield this turn
	TokensCreated       int  // tokens created this turn
	TreasuresCreated    int  // treasure tokens created this turn
	Sacrificed          int  // permanents sacrificed this turn
	PermanentsLeft      int  // permanents that left the battlefield this turn (dies + exile + bounce + sac)
	Discarded           int  // cards discarded this turn
	CommittedCrimes     int  // crimes committed this turn (CR §701.71 OTJ/MKM)
	Milled              int  // cards milled this turn
	LandsPlayed         int  // lands played this turn
	CreaturesDied       int  // creatures that died (went to GY from battlefield) this turn
	ExiledCards         int  // cards exiled this turn (from any zone)
	CastFromExile       int  // spells cast from exile this turn (cascade, impulse draw, etc.)
	Descended           bool // a permanent card entered graveyard this turn (Ixalan)
	// SpeedAdvancedThisTurn gates §702.178 / §702.179 — speed advances at
	// most once per turn regardless of how many damage events the player's
	// sources cause. Read+set by AdvanceSpeed in keywords_speed_counter.go.
	SpeedAdvancedThisTurn bool
	Attacked              bool         // this seat declared attackers this turn
	Casts                 []CastRecord // ordered log of every spell cast this turn

	// CombatDamageBy is the de-duplicated list of cards controlled by
	// this seat that dealt combat damage to ANY player (any opponent)
	// this turn. Populated by applyCombatDamageToPlayer in combat.go;
	// read by Prowl (§702.74), Freerunning (§702.169), and any other
	// "if you dealt combat damage to a player this turn with a [type]
	// creature" gate. One card appears at most once per turn even if
	// it dealt combat damage multiple times (e.g. double-strike,
	// extra-combat-phase). Cards that left the battlefield after
	// dealing damage still appear here — Prowl asks about historic
	// combat damage, not current board state.
	CombatDamageBy []*Card

	// ManaSpent counts the total mana paid by this seat this turn. CR
	// §702.190 (Expend) uses this counter — "Expend N — Triggered when
	// you've spent N or more mana of any color this turn." Bumped by
	// TrackManaSpentThisTurn in keywords_expend.go, which fires expend
	// triggers when the running total crosses each watcher's threshold.
	// Reset to 0 alongside the rest of TurnCounters at untap.
	ManaSpent int
}

// Reset zeroes all turn counters. Called once per turn at the untap step.
// Casts and CombatDamageBy slices are zero-length-reused so the backing
// arrays survive across turns (saves allocations in hot test loops + AI
// rollouts).
func (tc *TurnCounters) Reset() {
	*tc = TurnCounters{
		Casts:          tc.Casts[:0],
		CombatDamageBy: tc.CombatDamageBy[:0],
	}
}

// NthCastOfType returns the count of spells cast this turn that match
// any of the given types. E.g. NthCastOfType("instant","sorcery") returns
// how many instants or sorceries have been cast.
func (tc *TurnCounters) NthCastOfType(types ...string) int {
	n := 0
	for _, c := range tc.Casts {
		for _, wantType := range types {
			for _, hasType := range c.Types {
				if hasType == wantType {
					n++
					goto next
				}
			}
		}
	next:
	}
	return n
}

// HasXCast returns true if any spell cast this turn had an X cost.
func (tc *TurnCounters) HasXCast() bool {
	for _, c := range tc.Casts {
		if c.XCost {
			return true
		}
	}
	return false
}

// MaxManaValue returns the highest mana value among spells cast this turn.
func (tc *TurnCounters) MaxManaValue() int {
	max := 0
	for _, c := range tc.Casts {
		if c.ManaValue > max {
			max = c.ManaValue
		}
	}
	return max
}

// -----------------------------------------------------------------------------
// Seat (player)
// -----------------------------------------------------------------------------

// Seat holds per-player state. One Seat per player regardless of format.
type Seat struct {
	Idx int // seat index, 0-based

	// PlayerInstanceID is the Counter DB Phase 4 player identity stamp,
	// minted at seat construction as "p<idx>" (e.g. "p0", "p1"). Immutable
	// across the game — matches the InstanceID Phase 3 source-held linkage
	// convention so per-player counter placements carry a stable lineage
	// label distinct from object-level Permanent.InstanceID stamps.
	PlayerInstanceID string

	// Counters is the Phase 4 §122 player counter slice, mirroring
	// Permanent.CounterStacks. Holds non-legacy §122 player counter kinds
	// (e.g. shards, surveillance). Phase 4 keeps the high-volume poison /
	// experience / rad kinds on their legacy fields (PoisonCounters,
	// Flags["experience_counters"], Flags["rad_counters"]) and the seat
	// counters.Target adapter synthesizes a unified stack view across
	// both stores. Energy lives on its own §106.11 resource pool
	// (Flags["energy_counters"]) and is NOT a §122 counter — proliferate
	// never affects it.
	Counters []counters.CounterStack

	// Life total / lose flags. Life starts at 20; commander format callers
	// should set it to 40 before the first ResolveEffect.
	Life           int
	Lost           bool
	Won            bool
	PoisonCounters int

	// Speed is the Aetherdrift player-speed counter (CR §702.178 /
	// §702.179). Range 0..MaxSpeedCap (4). Persists across turns. The
	// once-per-turn advancement gate lives on Turn.SpeedAdvancedThisTurn
	// so it auto-resets in UntapAll. Read via SpeedOf / advanced via
	// AdvanceSpeed in keywords_speed_counter.go.
	Speed int

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

	// LossDetail is the structured loss cause (consolidation step 1) —
	// the canonical sibling of the freeform LossReason string above.
	// {Category, Rule, SourceCard} kills the downstream substring
	// parsing of LossReason that the four kill classifiers each
	// re-implement. ADDITIVE for now: nothing populates it yet; the SBA
	// writers migrate to stamping both fields in a later step, and
	// readers switch over once every writer does. nil ⇔ not populated
	// (which today is always).
	LossDetail *judge.LossReason

	// LostOrder is the 1-based elimination sequence: 1 = first player
	// eliminated, 2 = second, etc. Stamped by HandleSeatElimination
	// (which CheckEnd runs exactly once per newly-Lost seat, in
	// elimination order) — so it is populated regardless of
	// EventPolicy. 0 = never eliminated (or a fixture that bypassed
	// the §800.4a pipeline). heimdall.ClassifyKill keys the winner's
	// kill method off the opponent with the HIGHEST LostOrder — the
	// final elimination is how the winner closed the game (r62,
	// win_reason direction fix, reports 06/08).
	LostOrder int

	// SBA704_5a_emitted is the per-drop log-spam guard for §704.5a. Set
	// when the SBA either marks the player Lost or logs a loss_prevented
	// (Platinum Angel etc.) event. Cleared by sba704_5a once life is
	// positive again so the next drop produces a fresh audit entry.
	// Does NOT gate the SBA itself — the dying check uses `!Lost` so the
	// loss-prevention chain re-fires on every priority pass per CR §704.3.
	SBA704_5a_emitted bool

	// CommanderDamageNextSeq is the next EventLog index that §704.6c has
	// not yet scanned. Prevents double-counting across SBA passes.
	CommanderDamageNextSeq int

	// AttemptedEmptyDraw is set when Draw was called but the library was
	// empty. §704.5b will consume this in the SBA phase.
	AttemptedEmptyDraw bool

	// LostByEffect — CR §104.3e marker. Set by resolveLoseGame together
	// with Lost=true when this seat lost because a card effect (Demonic
	// Pact's final mode, Door to Nothingness, Phage the Untouchable's
	// damage clause, Lich's Mirror's failure fallback, etc.) said so —
	// as distinct from the numeric §704.5a/b/c/6c clauses. The
	// CheckLossConditions classifier reads this flag and returns
	// LossEffect when set, giving downstream consumers (Heimdall,
	// analytics, hat AI) a clean enum to switch on alongside the
	// human-readable LossReason string. Cleared (along with Lost) is a
	// bug — the two fields are expected to move in lockstep through
	// the resolveLoseGame canonical pipeline.
	LostByEffect bool

	// Turn is the centralized per-turn counter block. All core engine
	// functions (GainLife, drawOne, SacrificePermanent, etc.) increment
	// these counters directly. Per_card handlers read them instead of
	// rolling their own flag tracking. Reset once per turn in UntapAll.
	Turn TurnCounters

	// SpellsCastThisTurn is the per-seat count of spells THIS seat has
	// cast since its last untap. Resets at this seat's turn start, NOT
	// at every untap — an instant this seat casts on opponents' turns
	// still counts toward the seat's cast-count observability window.
	// Used by Storm-Kiln Artist / Young Pyromancer / Birgi / Monastery
	// Mentor / Niv-Mizzet Parun / Runaway Steam-Kin cast-trigger
	// observers (the "whenever YOU cast…" cards).
	// DEPRECATED: prefer seat.Turn.SpellsCast. Kept for compatibility
	// with existing per_card handlers during migration.
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
	// DEPRECATED: prefer seat.Turn.Descended. Kept for compatibility.
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

	// -----------------------------------------------------------------
	// InstanceID Phase 6 — lazy-init subsystem state (per
	// docs/instanceid-system-v2-r60.md §4.4 + Probe D audit at
	// docs/subsystem-activation-audit-r60.md). All zero/false by default;
	// flipped by CheckSubsystemActivation when the dormant hook for the
	// corresponding subsystem fires for the first time. Per-card
	// handlers read these typed fields for cheap hot-path checks rather
	// than re-scanning oracle text.
	// -----------------------------------------------------------------

	// DayNightActive is true once any card has caused the day/night
	// designation to exist for the game. Game-wide flag mirrored across
	// every seat at activation time (CR §726.3 persistence).
	DayNightActive bool

	// MonarchActive is true once any "become the monarch" effect has
	// resolved on this seat. Distinct from gs.Flags["monarch_seat"]
	// (the current holder) — MonarchActive is a per-seat sticky marker
	// that the monarch subsystem has been touched at least once by this
	// seat's permanents.
	MonarchActive bool

	// InitiativeHolder is true while this seat holds the initiative
	// (CR §702.156 / Initiative rules). Set on activation of any "take
	// the initiative" effect controlled by this seat; passed off via
	// combat-damage-to-player per the same rules.
	InitiativeHolder bool

	// AscendActive is true once any Ascend-keyword card under this
	// seat's control has entered any zone. Gates per-seat permanent
	// counting toward the city's blessing threshold.
	AscendActive bool

	// CurrentDungeon is the dungeon this seat is currently venturing
	// in (CR §309 Dungeons). Nil when the seat is between ventures.
	// The Phase 6 hook only seeds the dormant subsystem; per_card
	// venture handlers continue to populate this pointer when their
	// effect resolves.
	CurrentDungeon *Dungeon

	// RingTempts is the number of times the Ring has tempted this seat
	// (CR §702 / LTR Ring rules). 0 = the Ring has never tempted this
	// seat. Incremented by per_card temptation handlers; the Phase 6
	// hook only marks the subsystem live.
	RingTempts int

	// EnergyCounters mirrors the §106.11 energy pool. NOT a §122
	// counter — proliferate cannot add {E}. Mirrors the legacy
	// Flags["energy_counters"] storage so hot-path readers can use the
	// typed field. Helpers in energy.go remain the canonical add/spend
	// path; this field is read-side only for the activation registry.
	EnergyCounters int

	// XPCounters is the seat's experience-counter pool. Like
	// EnergyCounters, NOT a §122 counter — proliferate cannot grant XP
	// per CR. Read-side mirror for the activation registry.
	XPCounters int

	// ForetellExile is the per-seat foretell-exile bucket (CR §702.143).
	// Cards exiled via the foretell cost live here so the cast-from-
	// foretell path can find them without searching the general exile
	// zone. Populated lazily on first foretell activation; nil before
	// the subsystem wakes.
	ForetellExile []*Card

	// HasCityBlessing is the §702.131 city's-blessing flag. Once true,
	// stays true for the rest of the game. Set by the CityBlessing
	// activation hook when the seat reaches 10+ permanents with an
	// Ascend card present.
	HasCityBlessing bool

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
		PlayerInstanceID:    fmt.Sprintf("p%d", idx),
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

	// ManaCostString is the raw printed mana cost ("{2}{W}{W}{U}"). The
	// corpus loader populates this when available. Consumed by validators
	// that need pip-level inspection (CR §702.139 Jegantha's
	// "no card has more than one of the same mana symbol" check).
	// Empty for lands, tokens, and cards without a printed cost.
	ManaCostString string

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
	// cards (Fork, Twinflame, storm copies). CR §707.10: a copy of a spell
	// ceases to exist in any zone other than the stack. CR §704.5e: a copy
	// of a card in any zone other than the stack or battlefield ceases to
	// exist. SBA sba704_5e sweeps these from hand/graveyard/exile/library.
	IsCopy bool

	// ExiledByTimestamp is non-zero when this card was exiled "linked" to
	// a specific permanent (CR §406.7 — O-Ring, Fiend Hunter, Knowledge
	// Pool, Hostage Taker, etc.). The value is the exiling permanent's
	// Timestamp field, which uniquely identifies it within the game. When
	// the linked permanent leaves the battlefield, its LTB handler returns
	// all cards whose ExiledByTimestamp matches.
	ExiledByTimestamp int

	// OracleTextCache — lowercased oracle text, computed once on first
	// access via OracleTextLower. Avoids repeated string building +
	// ToLower in hot evaluator loops.
	OracleTextCache string
	oracleTextReady bool

	// TypeLineLowerCache — lowercased TypeLine, computed lazily.
	TypeLineLowerCache string
	typeLineLowerReady bool

	// ProducedColorsMask — bitmask of mana colors this card (typically a
	// land) can produce. Bits: W=1, U=2, B=4, R=8, G=16. Computed lazily
	// by hat.LandProducesColorsMask. Saves per-evaluation map allocation
	// and repeated strings.Contains across colors.
	ProducedColorsMask  uint8
	ProducedColorsReady bool

	// Meta — open-ended per-card runtime metadata. Used by mechanics
	// that need to mark a card object with provenance state that doesn't
	// fit a dedicated field. Currently populated by:
	//   - ConjureCard (keywords_conjure.go): Meta["conjured"] = true
	//     marks Alchemy-conjured cards so cards / effects keyed on
	//     "you cast a conjured spell" (or "you have a conjured card in
	//     your library") can identify them.
	// nil for cards built outside of any tagged provenance.
	Meta map[string]any

	// -----------------------------------------------------------------
	// InstanceID Phase 1 (docs/instanceid-system-v2-r60.md §4.1).
	// All zero-valued by default — backwards-compatible legacy mode
	// (empty InstanceID = not yet minted, treat per the v1 *Card path).
	// -----------------------------------------------------------------

	// InstanceID is the per-instance identity string. Format:
	// <prefix><seat><provenance><visibility><color><cmc><seq5>. Set at
	// mint time (deck-load for OG; token/copy/ability sites in Phase 2+).
	// Persists across zone changes per CR §400.7.
	InstanceID string

	// Provenance encodes WHAT this Card object is — OG (original print),
	// TK (token), CP (spell/perm copy), AB (ability instance). The
	// zero value is instanceid.ProvUnknown, which the engine treats as
	// "legacy / unminted" for backwards compatibility.
	Provenance instanceid.Provenance

	// Visibility is Visible by default; Hidden for face-down cards
	// (manifested 2/2, cloaked, morph, foretold pre-cast). Spectator
	// API uses this to gate characteristic leaks.
	Visibility instanceid.Visibility

	// SourceInstanceID is the InstanceID of the object this Card is a
	// copy of. Empty for OG. Required for CP. Optional for TK.
	SourceInstanceID string

	// EnablerInstanceID is the InstanceID of the ability or effect that
	// caused this Card to come into existence (Sai's thopter-mint ability
	// instance, Riku's copy-trigger instance, etc.). Empty for OG;
	// required for TK / CP / triggered-AB.
	EnablerInstanceID string

	// EnablerHistory is an append-only log of enabler IDs for re-copy
	// chains (Vesuvan Shapeshifter rewriting its copy target,
	// Lazav-shape "becomes a copy of" chains, Volrath rotational copy
	// stacks). The current EnablerInstanceID is the most recent entry;
	// EnablerHistory records the full lineage for replay / Heimdall
	// rendering.
	EnablerHistory []string

	// ActiveFace selects which face of a DFC / MDFC is currently active.
	// Default Front matches CR §712.6c (non-battlefield DFCs default to
	// front). Transform-style cards flip this; cast-as-back stamps Back
	// at cast time.
	ActiveFace instanceid.FaceIndex
}

func (c *Card) DeepCopy() *Card {
	if c == nil {
		return nil
	}
	cp := *c
	cp.Types = append([]string(nil), c.Types...)
	cp.Colors = append([]string(nil), c.Colors...)
	cp.BackFaceTypes = append([]string(nil), c.BackFaceTypes...)
	cp.EnablerHistory = append([]string(nil), c.EnablerHistory...)
	if c.Meta != nil {
		cp.Meta = make(map[string]any, len(c.Meta))
		for k, v := range c.Meta {
			cp.Meta[k] = v
		}
	}
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

	// CounterStacks is the InstanceID-aware counter representation
	// introduced by Counter DB Phase 1 (docs/counter-db-implementation-plan-r60.md
	// §3 + §4.1). Each stack carries the PlacedByInstanceID lineage of the
	// ability/effect that placed the counters and a PlacedAtTick game-clock
	// for forensic ordering. Phase 1 keeps this parallel to the existing
	// Counters map — Phase 2+ migrates consumers off the map and onto the
	// stack list. The §122.6 persistence invariant (counters survive Layer-4
	// type changes) pins on this field being immutable across type-strips.
	CounterStacks []counters.CounterStack

	// AttachedTo: for Auras/Equipment, the permanent this is attached to.
	// nil for unattached permanents.
	AttachedTo *Permanent

	// Modifications: buffs applied "until end of turn". Phase 8 will handle
	// the full §613 layer stack; here we just accumulate until-EOT entries.
	Modifications []Modification

	// GrantedAbilities: ability names granted "until end of turn".
	GrantedAbilities []string

	// AnimatedAddedTypes records the card-type tokens added by an
	// "becomes a creature until end of turn" animation (manlands like
	// Mutavault, the Restless cycle). The until-EOT P/T lives in
	// Modifications; these types are stripped at the §514.2 cleanup so a
	// land doesn't stay a creature after the turn ends. Empty when the
	// permanent is not currently animated.
	AnimatedAddedTypes []string

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

	// LinkedExile holds cards this permanent has exiled "until it leaves"
	// (CR §406.7). Oblivion Ring, Fiend Hunter, Knowledge Pool, Hostage
	// Taker, etc. append here at ETB; the LTB handler iterates this to
	// return cards to the appropriate zone. The matching Card has
	// ExiledByTimestamp set to this permanent's Timestamp.
	LinkedExile []*Card

	// -----------------------------------------------------------------
	// InstanceID Phase 3 (docs/instanceid-system-v2-r60.md §4.2 + §7).
	// Source-held exile linkage by InstanceID. Coexists with LinkedExile
	// (pointer-based) during the migration window — Phase 4 invariant
	// rewrite consumes both shapes. Zero-valued slice + LinkageNone tag
	// = legacy mode (LinkedExile / ExiledByTimestamp still authoritative).
	// -----------------------------------------------------------------

	// ExiledByMe is the InstanceID list of cards this permanent is
	// currently exiling under a source-held linkage. Per design v2 §7
	// and CR §400.7e, the source Permanent owns the linkage record —
	// no global table. LTB triggers walk this slice on any zone change
	// out of the battlefield (death, bounce, exile, commander-zone).
	ExiledByMe []string

	// LinkageKind tags how this permanent's exile linkage is to be
	// interpreted. LTBReturn = Banisher Priest / Oblivion Ring shape;
	// CastGrant = Etali / Mind's Desire (lifetime bound to AbilityInstance,
	// not to this Permanent); PermanentExile = Settle the Wreckage shape
	// (no return mechanism). LinkageNone for permanents not participating
	// in any exile linkage. Per design v2 §4.2.
	LinkageKind LinkageKind

	// Prepared is the §702.168 prepared state for Strixhaven DFCs.
	// While true, the controller may cast a copy of the card's spell face
	// (the back face). Casting the copy sets Prepared back to false.
	Prepared bool

	// -----------------------------------------------------------------
	// InstanceID Phase 5 (docs/instanceid-system-v2-r60.md §4.2 + §5).
	// Copy mechanism registry + provided-replacement audit fields.
	// All slices nil by default; presence-based opt-in per permanent.
	// -----------------------------------------------------------------

	// CopyMechanisms is the list of independently-triggered copy
	// capabilities on this permanent. Per Probe A, cards like Mirage
	// Mirror carry two entries (one upkeep-permanent, one activated-
	// temporary); Sakashima carries one with BypassesLegendRule = true.
	CopyMechanisms []CopyMechanism

	// CopiedTargetInstanceID is the InstanceID of the permanent this
	// permanent is currently a copy of (when CopiableSnapshot != nil).
	// Empty when this permanent is not currently copying anything.
	CopiedTargetInstanceID string

	// CopiableSnapshot is the §706.2 frozen-at-copy-moment snapshot of
	// the source's printed values. nil unless this permanent is acting
	// as a copy.
	CopiableSnapshot *CopiableCharacteristics

	// CopyHistory is an append-only audit log of every CopyMechanism
	// firing on this permanent. Replay tooling consumes this.
	CopyHistory []CopyEvent

	// BypassesLegendRule exempts this permanent from CR §704.5j legend-
	// rule SBA. Set by Sakashima at ETB ("Legend rule doesn't apply to
	// other permanents you control"). Per-card override per Probe A.
	BypassesLegendRule bool

	// AttachedTokenIDs tracks tokens this permanent has spawned as
	// attached copies (Helm of the Host's "create a token that's a copy
	// of equipped creature... attached to it"). Helm-style chain audit.
	AttachedTokenIDs []string

	// ProvidesReplacements lists the §614 replacement effects this
	// permanent registers when it's on the battlefield (Doubling Season,
	// Mondrak, Anointed Procession, Panharmonicon, etc.). Phase 5 audit
	// surface — the actual ReplacementEffect entries live on
	// gs.Replacements; this slice mirrors them for static introspection.
	ProvidesReplacements []ReplacementSpec

	// -----------------------------------------------------------------
	// InstanceID Phase 8 (docs/instanceid-system-v2-r60.md §8).
	// Unified Mutate + Meld merge shape. Empty / MergeNone when this
	// permanent is not a merged object.
	// -----------------------------------------------------------------

	// MergedCards is the ordered list of InstanceIDs of every card
	// currently merged into this permanent. For Mutate (§702.139) the
	// order is top-of-stack-first; for Meld (§712) it is exactly two
	// IDs. Unmerge on leave-play walks this slice to route each card
	// individually to its destination zone with its InstanceID preserved.
	MergedCards []string

	// MergeKind tags how this permanent was assembled. MergeNone for
	// ordinary permanents.
	MergeKind MergeKind

	// TopCard tracks the top of a mutate stack (§702.139a) — the
	// permanent's name, characteristics, and type line are taken from
	// this card while abilities accumulate from every MergedCards entry.
	// Unused for Meld (a melded result has its own combined card).
	TopCard *Card

	// MergedCardPtrs is the impl-side InstanceID → *Card resolution
	// table for cards currently merged into this permanent. Mutate +
	// Meld absorb their constituents from the battlefield without
	// routing them to any zone — without this map the unmerge walker
	// has no way to recover the *Card pointers for the non-surviving
	// stack members on leave-play. Keys are InstanceIDs; values are
	// the absorbed *Card. Populated by RecordMutateMerge /
	// RecordMeldMerge; drained by UnmergeOnLeavePlay.
	MergedCardPtrs map[string]*Card
}

// MergeKind enumerates the merged-card shapes per §8 of the InstanceID
// design v2.
type MergeKind int

const (
	MergeNone MergeKind = iota
	MergeMutate
	MergeMeld
)

// String renders MergeKind for logs / invariant errors.
func (k MergeKind) String() string {
	switch k {
	case MergeMutate:
		return "Mutate"
	case MergeMeld:
		return "Meld"
	}
	return "None"
}

// LinkageKind enumerates the three exile-linkage shapes per
// docs/instanceid-system-v2-r60.md §4.2 + §7. Stored on Permanent to
// tell the two-pronged ExileLinkageIntegrity invariant which validation
// path to apply.
//
//   - LinkageNone — this permanent isn't exiling anything (the default).
//   - LTBReturn — source Permanent owns the exile; LTB returns the cards.
//     Banisher Priest, Oblivion Ring, Detention Sphere, Faceless Butcher.
//     ExiledByMe MUST list the exile contents while the source is alive.
//   - CastGrant — cast-permission window bound to the AbilityInstance
//     lifetime, NOT the source Permanent's battlefield-lifetime. Etali,
//     Mind's Desire, Bolas's Citadel. The exiled cards remain in exile
//     after the cast window closes; the linkage is tracked on the
//     ZoneCastPermission's AbilityInstanceID, not on the source Permanent.
//   - PermanentExile — no return mechanism. Settle the Wreckage, disturb-
//     cast originals, foretold cards that never get cast. No back-reference
//     required; state is "in exile, indefinitely".
type LinkageKind int

const (
	LinkageNone LinkageKind = iota
	LTBReturn
	CastGrant
	PermanentExile
)

// String renders LinkageKind for logs / invariant errors.
func (k LinkageKind) String() string {
	switch k {
	case LTBReturn:
		return "LTBReturn"
	case CastGrant:
		return "CastGrant"
	case PermanentExile:
		return "PermanentExile"
	}
	return "LinkageNone"
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
	// CR §702.103a — while a bestow permanent is attached as an Aura it is
	// an enchantment — Aura and is NOT a creature (so it can't attack/block
	// and "destroy target creature" can't hit it). The flag is cleared on
	// falloff (BestowFalloff), reverting it to its printed creature type.
	if p != nil && p.Flags != nil && p.Flags["bestowed"] == 1 {
		return false
	}
	return p.hasType("creature")
}

// permIsBestowed reports whether p is currently attached as a bestow Aura.
func permIsBestowed(p *Permanent) bool {
	return p != nil && p.Flags != nil && p.Flags["bestowed"] == 1
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

// IsEnchantment — §205.3g (enchantment type). A bestowed permanent is an
// enchantment — Aura (CR §702.103a) regardless of its printed types.
func (p *Permanent) IsEnchantment() bool { return permIsBestowed(p) || p.hasType("enchantment") }

// IsBattle — §205.3g / §310 (battle type). See §704.5v.
func (p *Permanent) IsBattle() bool { return p.hasType("battle") }

// IsAura — §205.3h / §303 (Aura is an enchantment subtype). A bestowed
// permanent is an Aura while attached (CR §702.103a).
func (p *Permanent) IsAura() bool { return permIsBestowed(p) || p.hasType("aura") }

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
// a matching entry in GrantedAbilities. We conservatively check all
// surfaces, including the §122.1c keyword-counter grant via the Counter
// DB's CounterStacks lineage (Phase 2 wiring).
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
	if p.Counters["indestructible"] > 0 {
		return true
	}
	if p.HasKeywordCounter("indestructible") {
		return true
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

// ExileLinked moves a card to the owner's exile zone and links it to
// the exiling permanent (CR §406.7). When the permanent later leaves
// the battlefield, callers should use ReturnLinkedExile to return all
// linked cards. The card's ExiledByTimestamp is set to perm.Timestamp.
//
// Phase 3 (docs/instanceid-system-v2-r60.md §7): also stamps source-
// held InstanceID linkage. perm.ExiledByMe gains the card's InstanceID
// (when minted) and perm.LinkageKind is promoted to LTBReturn if it
// was LinkageNone. Permanents already tagged CastGrant/PermanentExile
// retain their tag — those callers route through their own primitives.
func ExileLinked(gs *GameState, perm *Permanent, card *Card, ownerSeat int, fromZone string) {
	if gs == nil || perm == nil || card == nil {
		return
	}
	// Move FIRST, stamp linkage only when the card actually lands in
	// exile. A §614 replacement can redirect the exile elsewhere — the
	// canonical case is §903.9b: a commander's owner may put it into the
	// command zone instead. Pre-fix the linkage was stamped before the
	// move, so a redirected commander left the source holding an
	// ExiledByMe entry for a card that was never in exile (seed-42 game
	// 486: Banisher Priest "exiles" seat 1's commander → command zone;
	// 24 ExileLinkageIntegrity hits until the Priest left, and the owner
	// re-cast the commander while the Priest still claimed it). Per
	// §406.7 the "until ~ leaves" link only forms for the card the
	// effect actually tracks in exile — a redirected commander escapes
	// the link permanently.
	res := MoveCard(gs, card, ownerSeat, fromZone, "exile", perm.Card.DisplayName()+"_exile_linked")
	if res.FinalZone != "exile" {
		gs.LogEvent(Event{
			Kind:   "exile_linked_redirected",
			Seat:   ownerSeat,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"card":             card.DisplayName(),
				"from_zone":        fromZone,
				"final_zone":       res.FinalZone,
				"source_timestamp": perm.Timestamp,
				"rule":             "406.7+903.9b",
				"reason":           "replacement_redirected_exile_no_linkage",
			},
		})
		return
	}
	card.ExiledByTimestamp = perm.Timestamp
	perm.LinkedExile = append(perm.LinkedExile, card)
	if card.InstanceID != "" {
		perm.ExiledByMe = append(perm.ExiledByMe, card.InstanceID)
	}
	if perm.LinkageKind == LinkageNone {
		perm.LinkageKind = LTBReturn
	}
	gs.LogEvent(Event{
		Kind:   "exile_linked_created",
		Seat:   ownerSeat,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"card":             card.DisplayName(),
			"from_zone":        fromZone,
			"source_timestamp": perm.Timestamp,
			"rule":             "406.7",
		},
	})
}

// ReleaseSourceLinkedExiles clears all §406.7 linked-exile bookkeeping
// for a permanent that is leaving via a path which can't safely route
// the exiled cards back to the battlefield — currently just
// HandleSeatElimination (CR §800.4a), which sweeps every leaving
// permanent in a single pass and would race with MoveCard if we tried
// to add returned cards to (still-being-cleared) battlefields.
//
// The exiled cards stay in their owners' exile zones, but their
// ExiledByTimestamp is reset and the source's LinkedExile / ExiledByMe
// slices are cleared. This satisfies the ExileLinkageIntegrity invariant
// (the dangling timestamp pointer goes away) at the cost of NOT routing
// the cards back to the battlefield per §406.7. Per §800.4a the
// leaving player's "until ~ leaves" effects end when the player leaves
// the game; the cards stuck in exile is a milder rules violation than
// the dangling-pointer leak.
//
// Engine-correctness TODO: route to owner's battlefield (or graveyard
// when owner is also leaving) via a deferred queue processed after the
// seat-elimination sweep completes. See ExileLinkageIntegrity Loki
// cluster (2026-05-30 r60 fresh-main).
func ReleaseSourceLinkedExiles(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if len(perm.LinkedExile) == 0 && len(perm.ExiledByMe) == 0 {
		return
	}
	releasedNames := make([]string, 0, len(perm.LinkedExile))
	for _, card := range perm.LinkedExile {
		if card == nil {
			continue
		}
		card.ExiledByTimestamp = 0
		releasedNames = append(releasedNames, card.DisplayName())
	}
	srcName := "<unknown>"
	if perm.Card != nil {
		srcName = perm.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "exile_linked_released_on_leave",
		Source: srcName,
		Amount: len(releasedNames),
		Details: map[string]interface{}{
			"cards":            releasedNames,
			"source_timestamp": perm.Timestamp,
			"rule":             "800.4a+406.7",
		},
	})
	perm.LinkedExile = nil
	perm.ExiledByMe = nil
}

// UnlinkExiledCard severs a card's §406.7 linked-exile bookkeeping from
// the card side: when a linked-exiled card leaves exile by some means
// OTHER than its source's LTB return (canonically the §704.6d/§903.9a
// SBA moving an exiled commander to the command zone — seed-42 game
// 486), the source must stop claiming it or ExileLinkageIntegrity
// reports the card as a broken linkage every census tick. Per §406.8
// semantics the "until ~ leaves" effect loses track of a card that
// changes zones — the link is consumed, not restored.
//
// Walks every battlefield for the permanent whose Timestamp matches
// card.ExiledByTimestamp, splices the card out of LinkedExile and its
// InstanceID out of ExiledByMe, and resets ExiledByTimestamp. No-op
// when the card carries no linkage stamp.
func UnlinkExiledCard(gs *GameState, card *Card) {
	if gs == nil || card == nil || card.ExiledByTimestamp == 0 {
		return
	}
	ts := card.ExiledByTimestamp
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Timestamp != ts {
				continue
			}
			kept := p.LinkedExile[:0]
			for _, lc := range p.LinkedExile {
				if lc != card {
					kept = append(kept, lc)
				}
			}
			p.LinkedExile = kept
			if card.InstanceID != "" {
				ids := p.ExiledByMe[:0]
				for _, iid := range p.ExiledByMe {
					if iid != card.InstanceID {
						ids = append(ids, iid)
					}
				}
				p.ExiledByMe = ids
			}
			srcName := "<unknown>"
			if p.Card != nil {
				srcName = p.Card.DisplayName()
			}
			gs.LogEvent(Event{
				Kind:   "exile_linked_unlinked",
				Seat:   card.Owner,
				Source: srcName,
				Details: map[string]interface{}{
					"card":             card.DisplayName(),
					"source_timestamp": ts,
					"rule":             "406.8",
					"reason":           "card_left_exile_by_other_means",
				},
			})
		}
	}
	card.ExiledByTimestamp = 0
}

// ReturnLinkedExile returns all cards linked to a permanent back to
// their owner's zone (typically "battlefield" for O-Ring effects or
// "hand" for bounce-exile effects). Clears the permanent's LinkedExile
// slice and resets each card's ExiledByTimestamp.
func ReturnLinkedExile(gs *GameState, perm *Permanent, toZone string) {
	if gs == nil || perm == nil || len(perm.LinkedExile) == 0 {
		return
	}
	returnedNames := make([]string, 0, len(perm.LinkedExile))
	for _, card := range perm.LinkedExile {
		if card == nil {
			continue
		}
		card.ExiledByTimestamp = 0
		returnedNames = append(returnedNames, card.DisplayName())
		MoveCard(gs, card, card.Owner, "exile", toZone, perm.Card.DisplayName()+"_ltb_return")
	}
	gs.LogEvent(Event{
		Kind:   "exile_linked_returned",
		Source: perm.Card.DisplayName(),
		Amount: len(returnedNames),
		Details: map[string]interface{}{
			"to_zone":          toZone,
			"cards":            returnedNames,
			"source_timestamp": perm.Timestamp,
			"rule":             "406.7",
		},
	})
	perm.LinkedExile = nil
	perm.ExiledByMe = nil
	// LinkageKind intentionally retained for forensic clarity — the
	// permanent has left play (or its slice was drained intentionally),
	// so the kind tag describes "what shape this permanent had" not
	// "what's currently exiled". Phase 4 invariants look at LinkageKind
	// + ExiledByMe length jointly, so an empty slice with a stale tag
	// is a valid "drained" state.
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
	// rather than CAST (CR §707.10). Storm copies, Twinflame copies,
	// Dualcaster Mage copies all set this. The distinction matters at
	// resolution time: a copy "ceases to exist" rather than going to
	// its owner's graveyard, because it isn't in any deck. A copy of a
	// permanent spell becomes a TOKEN copy of that permanent (§707.10f).
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

	// Ability is the AbilityInstance attached to this stack item when
	// Kind is "triggered" or "activated" — nil for spells. Carries the
	// AB-provenance InstanceID, source/enabler lineage, and TriggerMetadata
	// captured at push time. See docs/instanceid-system-v2-r60.md §4.3.
	// Empty (nil) is treated as legacy mode (Phase 1 backwards-compat).
	Ability *AbilityInstance

	// CleaveActive is true when the spell was cast for its cleave cost
	// (CR §702.158). Effect-resolution paths that need to distinguish
	// "bracketed" vs "brackets-removed" semantics read this; the actual
	// brackets-removed effect is already swapped onto Item.Effect at
	// cast time by CastWithCleave (keywords_cleave.go), so most
	// resolvers don't need to look at this — it's load-bearing for cards
	// that key off "if this spell's cleave cost was paid" (rare; mostly
	// for analytics + future per-card handlers).
	CleaveActive bool
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
//
// Phase D — universal token cessation chokepoint (CR §704.5d). When the
// removed perm is a TOKEN, its InstanceID is ceased and the *Card's
// InstanceID field cleared. This catches every token-removal path —
// canonical (Destroy / Exile / Sacrifice / Bounce already call
// markPermanentCeaseIfToken redundantly, which is idempotent) AND
// non-canonical (mutate-eats-other, gain-control swap, per_card raw
// removePermanent, etc.). Non-token perms are untouched: their *Card
// keeps its InstanceID stamped so graveyard/exile/hand references in
// the destination zone retain identity. Clearing the *Card.InstanceID
// on tokens lets re-add paths (blink, control-swap-with-re-wrap) pick
// up a fresh TK mint via EnsureTokenInstanceID when the perm re-enters
// — modeling §704.5d's "ceases to exist" + new-token-on-re-entry
// semantics correctly.
//
// Token-as-copy paths (Phantasmal Image, Spark Double, etc.) are
// already protected: the gap-walk's EnforceBattlefieldUniqueInstanceID
// re-mints those *Cards as TK before they hit the battlefield, so by
// the time this site sees them their ID is TK, not the original OG.
func (gs *GameState) removePermanent(p *Permanent) bool {
	if p == nil || p.Controller < 0 || p.Controller >= len(gs.Seats) {
		return false
	}
	bf := gs.Seats[p.Controller].Battlefield
	for i, q := range bf {
		if q == p {
			gs.Seats[p.Controller].Battlefield = append(bf[:i], bf[i+1:]...)
			if p.Card != nil && p.IsToken() && p.Card.InstanceID != "" {
				MarkInstanceIDCeased(gs, p.Card.InstanceID)
				p.Card.InstanceID = ""
			}
			return true
		}
	}
	return false
}

// isOwnerScopedZone reports whether a zone string names one of the
// owner-scoped private zones (hand, library, graveyard, exile, command
// zone). Per CR §402.1 / §403.1 / §404.1 / §406.1 / §408.1 (and §903.6
// / §903.9 for commander), a card placed in any of these zones lives
// in its OWNER's instance — never in some other player's. Battlefield
// + stack are excluded because they're shared (battlefield) or
// not-owned (stack).
func isOwnerScopedZone(zone string) bool {
	switch zone {
	case "hand", "library", "library_top", "library_bottom",
		"graveyard", "exile", "command_zone":
		return true
	}
	return false
}

// moveToZone appends c to the target seat's zone. Valid zones: "hand",
// "graveyard", "exile", "library_top", "library_bottom", "command_zone",
// "battlefield", "battlefield_tapped".
//
// CR §400.7 / §402-406: non-battlefield zones (hand, library, graveyard,
// exile, command zone) are OWNED BY THE CARD'S OWNER. A buggy caller
// passing controller-seat instead of owner-seat would route the card
// into a non-owner's private zone — the exact Etali r60 cluster shape
// (PR #685). This function defensively overrides `seat` to `c.Owner`
// for owner-scoped destinations and emits a `zone_owner_redirect`
// event so audits can count Etali-shape bug attempts.
// Battlefield destinations retain the caller-supplied `seat` because
// gain-control effects legitimately put cards on a non-owner's
// battlefield (CR §110.2a controller may differ from owner).
func (gs *GameState) moveToZone(seat int, c *Card, zone string) {
	if c == nil {
		return
	}
	// CR §800.4a chokepoint guard (r62, Phase-H OGVC fabrication class —
	// mirror of the MoveCard guard for direct callers): a card owned by
	// a player who has left the game cannot be moved into any zone. The
	// eliminated seat's slices keep their pointers for forensic clarity;
	// without this guard a selector that found one of those dead cards
	// re-materializes it in a live zone and the InstanceID census flags
	// fabrication ("present but not in (Minted - Ceased)").
	// Owner >= 0 (r63 seed-1337 game 2293 fix): the LeftGame guard MUST
	// cover seat 0. The old `Owner > 0` cutoff exempted every seat-0-owned
	// card from §800.4a, so when seat 0 was eliminated a reanimator could
	// re-materialize its ceased cards (The Cruelty of Gix → Flayer of
	// Loyalties). The owner-redirect check below keeps `> 0` (a separate
	// §400.7 concern with the zero-value-fixture caveat); only the
	// LeftGame refusal needs seat-0 coverage, and the LeftGame condition
	// itself is the discriminator against incidental zero-value fixtures.
	if c.Owner >= 0 && c.Owner < len(gs.Seats) {
		if os := gs.Seats[c.Owner]; os != nil && os.LeftGame {
			gs.LogEvent(Event{
				Kind:   "zone_move_refused",
				Seat:   seat,
				Source: c.DisplayName(),
				Details: map[string]interface{}{
					"rule":    "800.4a",
					"reason":  "owner_left_game",
					"to_zone": zone,
				},
			})
			return
		}
	}
	// Narrow trigger condition: only redirect when c.Owner is explicitly
	// set (Owner > 0). Zero-Owner cards bypass the check because Go's
	// int zero-value collides with "seat 0 owner" — and test fixtures
	// across the per_card suite construct cards via `&Card{Name: "X"}`
	// then place them in non-zero seats' zones, conventionally treating
	// the test's seat placement as ground truth. Real-game cards loaded
	// from decklists / spell-cast / library shuffles always have Owner
	// explicitly populated to the deck's owning seat, so the Etali bug
	// shape (Owner ∈ {1,2,3} card from opp library being routed to
	// Etali-controller-seat 0's exile) IS caught — the test convention
	// only hides Owner=0 cases, which are zero-value test artifacts
	// rather than real cross-seat bugs.
	if isOwnerScopedZone(zone) && c.Owner > 0 && c.Owner < len(gs.Seats) && c.Owner != seat {
		gs.LogEvent(Event{
			Kind:   "zone_owner_redirect",
			Seat:   c.Owner,
			Target: seat,
			Source: c.DisplayName(),
			Details: map[string]interface{}{
				"to_zone":     zone,
				"passed_seat": seat,
				"owner_seat":  c.Owner,
				"rule":        "400.7",
				"reason":      "owner_scoped_zone_redirect_to_card_owner",
			},
		})
		seat = c.Owner
	}
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	// CR §707.4 / §702.36: face-down cards are turned face-up when they
	// move to any zone other than the battlefield. Clear the FaceDown flag.
	// Cards entering the battlefield may remain face-down (Morph, Manifest).
	if c.FaceDown && zone != "battlefield" && zone != "battlefield_tapped" {
		c.FaceDown = false
	}
	// InstanceID gap-walk: any zone transition is a chance to detect
	// stale references to this card in OTHER zones (a previous move
	// that didn't sweep) or a DeepCopy-without-remint sibling sharing
	// this candidate's InstanceID. The helper purges same-pointer
	// occurrences (CR §400.7c — one zone at a time) and re-mints
	// different-pointer collisions (CR §706.10 — distinct minted IDs).
	EnforceBattlefieldUniqueInstanceID(gs, c, seat)
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
	case "library", "library_bottom":
		// Bare "library" = bottom of library (matches per_card callers
		// like nine_fingers_keene_to_bottom; runo_stromkirk does its
		// own top-of-library swap after placement so bottom is fine).
		// Without this arm, "library" fell through to the graveyard
		// default — cards going to "library" silently became graveyard
		// inhabitants. See docs/zone-accounting-analysis.md.
		if inSlice(s.Library) {
			return
		}
		s.Library = append(s.Library, c)
	case "library_top":
		if inSlice(s.Library) {
			return
		}
		s.Library = append([]*Card{c}, s.Library...)
	case "command_zone":
		if inSlice(s.CommandZone) {
			return
		}
		s.CommandZone = append(s.CommandZone, c)
	case "battlefield", "battlefield_tapped":
		// CR §110.1: A permanent is a card or token on the battlefield.
		// Wrap the Card in a Permanent and append to the seat's battlefield.
		// This case handles MoveCard(..., "battlefield", ...) calls from
		// ramp, cheat-into-play, and other non-cast paths that route
		// through FireZoneChange → moveToZone.
		//
		// Without this arm, "battlefield" fell through to the graveyard
		// default — cards going to "battlefield" silently became graveyard
		// inhabitants, causing ~80% of zone_accounting Feynman violations.
		// See docs/zone-accounting-analysis.md.
		for _, p := range s.Battlefield {
			if p.Card == c {
				return
			}
		}
		EnsureBattlefieldFrontFace(c)
		// CR 304.4 / 307.1: instants and sorceries cannot enter the
		// battlefield. If the card has no permanent type after face
		// correction, redirect to graveyard to avoid §205 SBA.
		if !CardCanEnterBattlefield(c) {
			s.Graveyard = append(s.Graveyard, c)
			return
		}
		tapped := zone == "battlefield_tapped"
		sick := false
		if cardHasType(c, "creature") {
			sick = !cardHasKeyword(c, "haste")
		}
		perm := &Permanent{
			Card:          c,
			Controller:    seat,
			Owner:         c.Owner,
			Tapped:        tapped,
			SummoningSick: sick,
			Timestamp:     gs.NextTimestamp(),
			Counters:      map[string]int{},
			Flags:         map[string]int{},
		}
		s.Battlefield = append(s.Battlefield, perm)
		RegisterReplacementsForPermanent(gs, perm)
		FirePermanentETBTriggers(gs, perm)
	default:
		if inSlice(s.Graveyard) {
			return
		}
		s.Graveyard = append(s.Graveyard, c)
	}
}

// GainLife adds life to a seat and fires the life_gained trigger so that
// Sanguine Bond, Aetherflux Reservoir, etc. see every life gain event.
// GainLife applies a life gain to `seat`, routing it through the CR §614
// would_gain_life replacement chain FIRST so EVERY lifegain source — combat
// lifelink, ETB drains, modal/triggered effects, the AST resolveGainLife path —
// is covered by lifegain doublers (Boon Reflection, Rhox Faithmender,
// Alhammarret's Archive — which multiply when stacked), additive deltas (Angel
// of Vitality), can't-gain-life, and opponent-gain→loss (Tainted Remedy). The
// post-replacement amount is what the life_gained trigger observers (Well of
// Lost Dreams, Archangel of Thune, Dawn of Hope) see — i.e. doublers apply
// BEFORE the payoff trigger reads the amount. Returns the life actually gained
// (0 if cancelled or non-positive).
//
// Pre-r63 the would_gain_life chain was consulted ONLY in resolveGainLife, so
// the 120+ bare GainLife() callers silently bypassed all §614 life
// replacements.
func GainLife(gs *GameState, seat, amount int, source string) int {
	if gs == nil || amount <= 0 || seat < 0 || seat >= len(gs.Seats) {
		return 0
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return 0
	}
	// §614 replacement chain (doublers / deltas / can't-gain / gain→loss).
	// No would_gain_life ApplyFn re-enters GainLife (the gain→loss handler
	// calls LoseLife), so this cannot recurse.
	modified, cancelled := FireGainLifeEvent(gs, seat, amount, nil)
	if cancelled || modified <= 0 {
		return 0
	}
	amount = modified

	s.Life += amount
	s.Turn.LifeGained += amount
	if s.Flags == nil {
		s.Flags = map[string]int{}
	}
	s.Flags["life_gained_this_turn"] += amount
	FireCardTrigger(gs, "life_gained", map[string]interface{}{
		"seat":   seat,
		"amount": amount,
		"source": source,
	})
	// r63 phase-3b: AST "whenever you gain life" triggers — the alias
	// mapped them to life_gained but only per_card handlers ever fired.
	FireLifeGainedASTTriggers(gs, seat)
	// Fire life_change trigger (positive amount = gain) for symmetry.
	FireCardTrigger(gs, "life_change", map[string]interface{}{
		"seat":   seat,
		"amount": amount,
		"source": source,
	})
	return amount
}

// LoseLife subtracts life, increments Turn.LifeLost, fires life_lost
// trigger, and dual-writes legacy flags. Mirrors GainLife.
func LoseLife(gs *GameState, seat, amount int, source string) {
	if gs == nil || amount <= 0 || seat < 0 || seat >= len(gs.Seats) {
		return
	}
	// Once the game has ended nothing further happens (CR §104.1-2): the
	// trigger dispatchers stop STARTING handlers on ended=1, but a handler
	// already mid-resolution when its own effects ended the game keeps
	// executing its remaining lines. Those trailing LoseLife calls were
	// draining the WINNER post-end (judge sweep round 2, seed 99 game 225:
	// an end-step Zurgo Stormrender chain killed the last opponent, the
	// game ended, and the rest of the chain pulled the winner to life=0 —
	// tripping SBACompleteness "704.5a missed" on a seat that can no
	// longer legally lose).
	if gs.Flags != nil && gs.Flags["ended"] == 1 {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}
	s.Life -= amount
	s.Turn.LifeLost += amount
	if s.Flags == nil {
		s.Flags = map[string]int{}
	}
	s.Flags["lost_life_this_turn"] += amount
	s.Flags["life_lost_this_turn"] += amount
	FireCardTrigger(gs, "life_lost", map[string]interface{}{
		"seat":   seat,
		"amount": amount,
		"source": source,
	})
	// Fire life_change trigger so Exquisite Blood can react to any life loss.
	FireCardTrigger(gs, "life_change", map[string]interface{}{
		"seat":   seat,
		"amount": -amount,
		"source": source,
	})
}

// DealDamage deals noncombat damage, incrementing DamageReceived and
// LifeLost. Use for "deals N damage to target player" effects.
func DealDamage(gs *GameState, seat, amount int, source string) {
	if gs == nil || amount <= 0 || seat < 0 || seat >= len(gs.Seats) {
		return
	}
	// Post-game-end guard — same rationale as LoseLife above (§104.1-2):
	// in-flight resolutions must not mutate life after the game ends.
	if gs.Flags != nil && gs.Flags["ended"] == 1 {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}
	// §614 damage replacement (R54) — consult any registered
	// "if a source you control would deal damage..." effects.
	// Torbran's red-source +2, Solphim's noncombat doubling, etc.
	// fire here. The replacement may zero the amount or fully
	// prevent (Sokrates-style dialogue conversion isn't reachable
	// from noncombat damage so its filter no-ops here).
	if len(gs.DamageReplacements) > 0 {
		ctx := &DamageContext{
			SourceName: source,
			TargetSeat: seat,
			Kind:       DamageNonCombatPlayer,
			Amount:     amount,
		}
		ApplyDamageReplacement(gs, ctx)
		if ctx.Prevented || ctx.Amount <= 0 {
			return
		}
		amount = ctx.Amount
	}
	// §615 prevention — DealDamage IS damage (it fires the damage + life_lost
	// triggers), so prevention shields and "prevent all damage" / protection
	// flags must apply. Pre-r63 this generic path (painlands, "deals N to
	// you" effects) skipped prevention entirely. No source permanent is
	// carried here, so color/quality-filtered shields can't match — that
	// limitation is inherent to the source-name-only signature, not this fix.
	amount = PreventDamageToPlayer(gs, seat, amount, nil)
	if amount <= 0 {
		return
	}
	s.Life -= amount
	s.Turn.LifeLost += amount
	s.Turn.DamageReceived += amount
	if s.Flags == nil {
		s.Flags = map[string]int{}
	}
	s.Flags["lost_life_this_turn"] += amount
	s.Flags["life_lost_this_turn"] += amount
	s.Flags["damage_taken_this_turn"] += amount
	FireCardTrigger(gs, "life_lost", map[string]interface{}{
		"seat":   seat,
		"amount": amount,
		"source": source,
	})
	// Fire life_change trigger so Exquisite Blood can react to noncombat damage.
	FireCardTrigger(gs, "life_change", map[string]interface{}{
		"seat":   seat,
		"amount": -amount,
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
	s.Turn.CardsDrawn++
	if s.Flags == nil {
		s.Flags = map[string]int{}
	}
	s.Flags["cards_drawn_this_turn"]++

	// CR §702.94a/b — Miracle. The dedicated miracle counter is reset to 0
	// for every seat at each turn start (UntapAll), so == 1 here means this
	// is the FIRST card this seat has drawn during the current turn — the
	// only draw that can open a miracle window (extra draws and non-draw
	// hand entry never reach this == 1 branch). MaybeOpenMiracleWindow
	// reveals a miracle card drawn first and grants the cast-for-miracle-cost
	// triggered ability.
	s.Flags["miracle_draws_this_turn"]++
	if s.Flags["miracle_draws_this_turn"] == 1 {
		MaybeOpenMiracleWindow(gs, seat, c)
	}

	// Fire "card_drawn" trigger for per-card handlers (Sheoldred,
	// Consecrated Sphinx, Smothering Tithe, Nekusar, Queza, Orcish
	// Bowmasters, The Watcher in the Water, The Council of Four,
	// Zimone and Dina, etc.). Recursion guard prevents infinite loops
	// when a draw-trigger handler itself draws cards (e.g. Consecrated
	// Sphinx drawing two in response to an opponent draw).
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	// CR §614.6 "except the first one they draw in each of their draw
	// steps" (Orcish Bowmasters, Notion Thief): the turn runner stamps
	// gs.Flags["_suppress_first_draw_trigger_seat"] = seat+1 before the
	// draw-step draw. Consume it HERE — the chokepoint every draw passes
	// through and where card_drawn fires — and surface it in the trigger
	// ctx. Pre-r62 the marker was consumed in FireDrawTriggerObservers,
	// which turn-step draws never reach, so it leaked onto the seat's
	// NEXT effect draw and suppressed the wrong trigger.
	isDrawStepDraw := false
	if v, ok := gs.Flags["_suppress_first_draw_trigger_seat"]; ok && v == seat+1 {
		isDrawStepDraw = true
		delete(gs.Flags, "_suppress_first_draw_trigger_seat")
	}
	gs.Flags["_card_drawn_depth"]++
	if gs.Flags["_card_drawn_depth"] <= 8 {
		FireCardTrigger(gs, "card_drawn", map[string]interface{}{
			"seat":              seat,
			"drawer_seat":       seat,
			"card":              c.DisplayName(),
			"nth_this_turn":     s.Flags["cards_drawn_this_turn"],
			"source":            "draw",
			"is_draw_step_draw": isDrawStepDraw,
		})
		// AST "whenever you draw a card" payoffs (the per_card registry above
		// covers only bespoke handlers; the AST walk reaches every parsed
		// draw trigger — r63 PROGRESSION saturation finding).
		FireDrawCardASTTriggers(gs, seat)
	}
	gs.Flags["_card_drawn_depth"]--

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
	s.Turn.Milled++
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
				"name":           p.Card.DisplayName(),
				"tapped":         p.Tapped,
				"summoning_sick": p.SummoningSick,
				"power":          p.Card.BasePower,
				"toughness":      p.Card.BaseToughness,
				"damage":         p.MarkedDamage,
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
