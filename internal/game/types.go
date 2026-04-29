// Package game implements the mtgsquad game state machine.
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
type Card struct {
	GameID       string `json:"game_id"`
	InstanceID   string `json:"instance_id"`
	Name         string `json:"name"`
	ManaCost     string `json:"mana_cost,omitempty"`
	CMC          int    `json:"cmc"`
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
