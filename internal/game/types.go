// Package game implements the hexdek game state machine.
//
// State-tracking, not a rules engine: players (and the AI) announce
// intents via the WebSocket API, and the server records the resulting
// state changes. Complex interactions (the stack, layers, replacement
// effects, triggered abilities) are resolved by player declaration.
package game

import "time"

// Phase enumerates the standard MTG turn phases.
type Phase string

const (
	PhaseUntap   Phase = "untap"
	PhaseUpkeep  Phase = "upkeep"
	PhaseDraw    Phase = "draw"
	PhaseMain1   Phase = "main1"
	PhaseCombat  Phase = "combat"
	PhaseMain2   Phase = "main2"
	PhaseEnd     Phase = "end"
	PhaseCleanup Phase = "cleanup"
)

// PhaseOrder is the canonical sequence of phases per turn.
var PhaseOrder = []Phase{
	PhaseUntap, PhaseUpkeep, PhaseDraw, PhaseMain1,
	PhaseCombat, PhaseMain2, PhaseEnd, PhaseCleanup,
}

// Zone enumerates the zones a card can occupy.
type Zone string

const (
	ZoneLibrary     Zone = "library"
	ZoneHand        Zone = "hand"
	ZoneBattlefield Zone = "battlefield"
	ZoneGraveyard   Zone = "graveyard"
	ZoneExile       Zone = "exile"
	ZoneCommand     Zone = "command"
	ZoneStack       Zone = "stack"
)

// Card is a runtime card instance within a game. The card_data field
// stores the original deck card metadata as JSON so we can render it
// without re-querying the oracle.
//
// Power and Toughness mirror the printed (or chosen) base stats from
// the source deck JSON. The full §613 layers pipeline (counters,
// until-EOT modifications, anthem effects) is NOT modeled in this
// live-game MVP — see the gameengine package for that. Combat damage
// reads Power/Toughness directly with a 1/1 fallback when both are
// zero (a deck card was loaded without P/T metadata).
type Card struct {
	GameID       string `json:"game_id"`
	InstanceID   string `json:"instance_id"`
	Name         string `json:"name"`
	ManaCost     string `json:"mana_cost,omitempty"`
	CMC          int    `json:"cmc"`
	Power        int    `json:"power,omitempty"`
	Toughness    int    `json:"toughness,omitempty"`
	Types        []string `json:"types,omitempty"`
	Subtypes     []string `json:"subtypes,omitempty"`
	OwnerSeat    int    `json:"owner_seat"`
	Zone         Zone   `json:"zone"`
	ZonePosition int    `json:"zone_position"`
	Tapped       bool   `json:"tapped"`
	TappedForManaThisTurn bool `json:"tapped_for_mana_this_turn,omitempty"` // prevents tap → untap → retap free-mana exploit
	RevealedTo   string `json:"revealed_to,omitempty"` // comma-separated seat positions
}

// IsLand returns true if the card has the Land type.
func (c *Card) IsLand() bool { return hasType(c.Types, "Land") }

// IsCreature returns true if the card has the Creature type.
func (c *Card) IsCreature() bool { return hasType(c.Types, "Creature") }

// IsInstantOrSorcery returns true for non-permanent spell types.
func (c *Card) IsInstantOrSorcery() bool {
	return hasType(c.Types, "Instant") || hasType(c.Types, "Sorcery")
}

func hasType(types []string, want string) bool {
	for _, t := range types {
		if t == want {
			return true
		}
	}
	return false
}

// Player tracks the per-seat state for an active game.
type Player struct {
	GameID         string `json:"game_id"`
	SeatPosition   int    `json:"seat_position"`
	DeviceID       string `json:"device_id"`
	DeckID         string `json:"deck_id"`
	Life           int    `json:"life"`
	PoisonCounters int    `json:"poison_counters"`
	ManaPoolW      int    `json:"mana_pool_w"`
	ManaPoolU      int    `json:"mana_pool_u"`
	ManaPoolB      int    `json:"mana_pool_b"`
	ManaPoolR      int    `json:"mana_pool_r"`
	ManaPoolG      int    `json:"mana_pool_g"`
	ManaPoolC      int    `json:"mana_pool_c"`
	LandsPlayedTurn int   `json:"lands_played_turn"`
	// AttemptedEmptyDraw — CR §119.5 flag. Set by DrawCards when a player
	// is instructed to draw a card while their library is empty. The next
	// CheckGameEnd pass eliminates seats with this flag set. Once set,
	// stays set for the rest of the game (the loss is permanent per CR
	// §704.5b — there is no "undo your empty-library attempt" effect).
	AttemptedEmptyDraw bool `json:"attempted_empty_draw,omitempty"`
	// WinEffectTriggered — CR §104.2c hook. Set when a "you win the game"
	// effect resolves (Approach of the Second Sun's second cast, Felidar
	// Sovereign / Test of Endurance upkeep, Maze's End at 10 gates,
	// Hellkite Tyrant at 7+ stolen artifacts, Mayael's Aria, etc.). Read
	// by CheckGameEnd before evaluating loss conditions — a triggered
	// win effect overrides simultaneous loss conditions on the same seat
	// (CR §104.3a: if a win effect and a loss effect would both apply at
	// the same time to the same player, the win effect wins). Not wired
	// into card resolution yet in the MVP layer — the flag is the
	// architectural seam that card handlers will flip when the live
	// game gains those card implementations.
	WinEffectTriggered bool `json:"win_effect_triggered,omitempty"`
}

// LossCause identifies which CR §704.5 clause eliminated a seat. Returned
// by Player.CheckLossConditions so the engine-level aggregator can log
// the cause and downstream analytics (Heimdall, Loki invariants) can
// classify the elimination. The string values are stable — referenced
// in action-log payloads.
type LossCause string

const (
	// LossNone — the seat has met no CR §704.5 loss condition.
	LossNone LossCause = ""
	// LossLife — CR §704.5a: zero or less life.
	LossLife LossCause = "life_zero_or_less"
	// LossEmptyLibrary — CR §704.5b / §119.5: attempted to draw from an
	// empty library since the last SBA check.
	LossEmptyLibrary LossCause = "attempted_empty_library_draw"
	// LossPoison — CR §704.5c: ten or more poison counters.
	LossPoison LossCause = "ten_or_more_poison_counters"
	// LossCommanderDamage — CR §903.10a: 21+ combat damage from a single
	// commander. Reserved — the MVP layer doesn't track per-commander
	// damage yet; the constant is the architectural seam.
	LossCommanderDamage LossCause = "twenty_one_commander_damage"
	// LossEffect — CR §704.5g: "you lose the game" effects (Door to
	// Nothingness, Phage the Untouchable damage, Lich's Mirror, Demonic
	// Pact, Near-Death Experience, etc.). Reserved — not wired yet.
	LossEffect LossCause = "you_lose_the_game_effect"
)

// CheckLossConditions evaluates this seat's CR §704.5 loss clauses in
// isolation. Returns the FIRST matched cause (clauses are independent —
// a seat at -5 life with 10 poison loses to both, but only one cause is
// reported; the seat is still removed from play either way). Returns
// (LossNone, false) when the seat survives.
//
// This is a PURE method: no DB access, no game-state lookups. Callers
// (CheckGameEnd, future SBA pipeline) read whatever fields they want
// onto the Player and let this method classify.
//
// CR §704.5 ordering is "simultaneous" — the §704.5 pass evaluates all
// applicable clauses at once and removes every matching player. The
// engine-level aggregator (CheckGameEnd) is responsible for the
// simultaneous-removal semantics; this method just answers "is this
// one seat lost?".
func (p *Player) CheckLossConditions() (LossCause, bool) {
	if p == nil {
		return LossNone, false
	}
	// §704.5a — life total of 0 or less.
	if p.Life <= 0 {
		return LossLife, true
	}
	// §704.5b — empty-library draw flag (§119.5 mechanism).
	if p.AttemptedEmptyDraw {
		return LossEmptyLibrary, true
	}
	// §704.5c — ten or more poison counters.
	if p.PoisonCounters >= 10 {
		return LossPoison, true
	}
	// §903.10a (commander damage) and §704.5g (effect-loss) hooks land
	// here when their backing fields exist on Player.
	return LossNone, false
}

// TurnState describes the active turn.
type TurnState struct {
	GameID       string `json:"game_id"`
	ActiveSeat   int    `json:"active_seat"`
	Phase        Phase  `json:"phase"`
	PrioritySeat int    `json:"priority_seat"`
	TurnNumber   int    `json:"turn_number"`
}

// Game is the top-level container. Cards/Players/Turn are stored
// separately in SQLite and assembled when a snapshot is requested.
type Game struct {
	ID         string `json:"id"`
	PartyID    string `json:"party_id"`
	StartedAt  int64  `json:"started_at"`
	FinishedAt int64  `json:"finished_at,omitempty"`
	Winner     string `json:"winner_device_id,omitempty"`
}

// SnapshotForPlayer is the per-player view of the game state. Hidden
// information (other players' hands, library order) is REDACTED before
// returning.
type SnapshotForPlayer struct {
	Game       *Game                  `json:"game"`
	Turn       *TurnState             `json:"turn"`
	You        *Player                `json:"you"`
	Opponents  []*Player              `json:"opponents"`
	YourHand   []*Card                `json:"your_hand"`
	YourLib    int                    `json:"your_library_size"`
	YourGY     []*Card                `json:"your_graveyard"`
	YourExile  []*Card                `json:"your_exile"`
	Battlefield map[int][]*Card       `json:"battlefield_by_seat"` // seat → cards (visible to all)
	Command     map[int][]*Card       `json:"command_by_seat"`     // seat → commander zone (commanders are public in EDH)
	Graveyards  map[int][]*Card       `json:"graveyard_by_seat"`   // seat → graveyard (public)
	OppHandSizes map[int]int          `json:"opp_hand_sizes"`      // seat → hand size (counts only)
	OppLibSizes  map[int]int          `json:"opp_library_sizes"`
	GeneratedAt time.Time             `json:"generated_at"`
}
