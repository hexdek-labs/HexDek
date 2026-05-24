package heimdall

// GameSeed stores the minimum data needed to deterministically replay a game.
// At 50M games/day, keep this struct small. CounterSnapshot is per-seat and
// optional — populated during observation mode, omitted from the fast path.
type GameSeed struct {
	RNGSeed         int64                    `json:"rng_seed"`
	DeckKeys        [4]string                `json:"deck_keys"`
	Winner          int                      `json:"winner"`
	Turns           int                      `json:"turns"`
	KillMethod      string                   `json:"kill_method"` // combat, commander, combo, mill, poison, timeout
	TurnCounters    [4]TurnCounterSnapshot   `json:"turn_counters,omitempty"`
}

// Observation is the lightweight per-game extraction. ~200 bytes (plus
// optional event slices when observation mode is on).
// Only populated during batch replay or live observation mode.
type Observation struct {
	Seed            GameSeed
	ParserGaps      []string        // card names that hit unhandled abilities
	DeadTriggers    []DeadTrigger   // registered but never fired
	ComboAttempted  bool            // did the deck attempt its Freya combo?
	ComboSucceeded  bool            // did the combo resolve to a win?
	ComboMissed     bool            // were combo pieces available but not used?
	CoTriggers      []CoTriggerPair // Huginn food: cards that synergized
	CausalPivot     *PivotEvent     // Tesla: the turn/action that decided the game
	CardFirstPlayed map[string]int  // card name → turn it first resolved as a spell

	// ZoneCastEvents captures grant lifecycle: registered, expired,
	// adventure_exiled, paradigm_exile_created. Used by Huginn to discover
	// which decks lean on cast-from-elsewhere patterns.
	ZoneCastEvents []ZoneCastEvent
	// ExileLinkEvents captures O-Ring style link/unlink for blink+exile
	// pattern discovery (Huginn) and parity-divergence audit (Muninn).
	ExileLinkEvents []ExileLinkEvent

	// CommanderZoneVisits captures per-commander zone-visit and cast
	// counts at game end. Useful for surfacing "this commander came
	// back N times" — a strong signal that the commander is being
	// answered repeatedly (removal-heavy meta or fragile commander) vs
	// "stayed on the battlefield all game" (sticky/protected board
	// presence). Populated per seat per commander card.
	CommanderZoneVisits []CommanderZoneVisit

	// RegretCards lists cards that a seat held but never cast — the
	// high-CMC stranded-in-hand signal Heimdall surfaces to flag
	// deck-construction or pilot-decision regret. Sorted by Seat
	// ascending then CMC descending (biggest regret per seat first).
	RegretCards []RegretCard
}

// ZoneCastEvent records a zone-cast grant lifecycle moment or an
// adventure/paradigm exile event. Kind is one of:
//   - "zone_cast_grant_registered"
//   - "zone_cast_grant_expired"
//   - "adventure_exiled"
//   - "paradigm_exile_created"
type ZoneCastEvent struct {
	Kind     string `json:"kind"`
	Card     string `json:"card"`
	Source   string `json:"source,omitempty"`
	Zone     string `json:"zone,omitempty"`
	Keyword  string `json:"keyword,omitempty"`
	Duration string `json:"duration,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Seat     int    `json:"seat"`
	Turn     int    `json:"turn,omitempty"`
}

// ExileLinkEvent records an O-Ring style link or unlink. Kind is one of:
//   - "exile_linked_created"
//   - "exile_linked_returned"
type ExileLinkEvent struct {
	Kind            string   `json:"kind"`
	Source          string   `json:"source"`
	Cards           []string `json:"cards,omitempty"`     // for returned events
	Card            string   `json:"card,omitempty"`      // for created events
	FromZone        string   `json:"from_zone,omitempty"` // for created
	ToZone          string   `json:"to_zone,omitempty"`   // for returned
	Seat            int      `json:"seat"`
	SourceTimestamp int      `json:"source_timestamp,omitempty"`
	Turn            int      `json:"turn,omitempty"`
}

// TurnCounterSnapshot is a per-seat aggregate of TurnCounters at game end.
// Mirrors the engine's gameengine.TurnCounters but omits per-cast records
// (those would blow up the GameSeed). Captures the 21 numeric/boolean fields
// downstream tools care about.
type TurnCounterSnapshot struct {
	LifeGained          int  `json:"life_gained"`
	LifeLost            int  `json:"life_lost"`
	DamageReceived      int  `json:"damage_received"`
	LifePaid            int  `json:"life_paid"`
	CardsDrawn          int  `json:"cards_drawn"`
	SpellsCast          int  `json:"spells_cast"`
	CreaturesEntered    int  `json:"creatures_entered"`
	ArtifactsEntered    int  `json:"artifacts_entered"`
	EnchantmentsEntered int  `json:"enchantments_entered"`
	TokensCreated       int  `json:"tokens_created"`
	TreasuresCreated    int  `json:"treasures_created"`
	Sacrificed          int  `json:"sacrificed"`
	PermanentsLeft      int  `json:"permanents_left"`
	Discarded           int  `json:"discarded"`
	Milled              int  `json:"milled"`
	LandsPlayed         int  `json:"lands_played"`
	CreaturesDied       int  `json:"creatures_died"`
	ExiledCards         int  `json:"exiled_cards"`
	CastFromExile       int  `json:"cast_from_exile"`
	Descended           bool `json:"descended"`
	Attacked            bool `json:"attacked"`
}

// DeadTrigger records a trigger that was registered but never fired during the game.
type DeadTrigger struct {
	CardName    string
	TriggerType string // etb, ltb, dies, cast, etc.
}

// CoTriggerPair records two cards that synergized within a turn window.
type CoTriggerPair struct {
	CardA       string
	CardB       string
	ImpactScore float64
	TurnWindow  int
}

// PivotEvent records the turn/action that decided the game outcome.
type PivotEvent struct {
	Turn   int
	Action string
	Seat   int
}

// CommanderZoneVisit records command-zone activity for one commander
// (one seat). Visits derives from CastCount + InZoneAtEnd: each cast
// from the command zone is one departure, and §903.9a redirects on
// death/exile are the only way a commander re-enters the command
// zone in standard play, so:
//
//	Visits = CastCount + (1 if InZoneAtEnd else 0)
//
// This assumes the standard §903 setup where every commander starts
// in the command zone. Edge cases where a commander is moved out of
// the command zone by means other than casting (e.g., stolen pre-
// cast) will under-count by 1; flagged but not corrected here since
// those paths are vanishingly rare and reporting CastCount + flag
// lets downstream consumers re-derive.
type CommanderZoneVisit struct {
	Seat          int    `json:"seat"`
	CommanderName string `json:"commander_name"`
	CastCount     int    `json:"cast_count"`
	InZoneAtEnd   bool   `json:"in_zone_at_end"`
	Visits        int    `json:"visits"`
}

// RegretCard records one card a seat held but never deployed. The
// archetypal signal is "I was sitting on a 7-mana bomb the whole
// game and never cast it" — a deck-construction or game-plan flag
// worth surfacing per-game.
//
// Reason discriminates how the regret was identified:
//
//   - "stranded_in_hand": card was in the seat's hand at game end
//     with CMC ≥ 1 (i.e., a real spell, not a 0-cost rider or land).
//     Derivable from end-state alone; no event log required.
//
// Fizzle / no-legal-target / redundant detection requires
// RetainEvents and is deferred — the fast replay path runs with
// events disabled. When that surface is added, additional Reason
// values ("fizzled", "no_legal_target", "redundant_etb") will
// extend this type rather than introducing a parallel struct.
type RegretCard struct {
	Seat     int    `json:"seat"`
	CardName string `json:"card_name"`
	CMC      int    `json:"cmc"`
	Reason   string `json:"reason"`
}

// HealthPulse is the periodic telemetry snapshot sent to GA4.
type HealthPulse struct {
	GamesPlayed   int
	ParserGaps    int
	Crashes       int
	DeadTriggers  int
	TopGapCards   []string
	EngineVersion string
}
