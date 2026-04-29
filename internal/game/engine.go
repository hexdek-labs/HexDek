package game

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/mana"
	"github.com/hexdek/hexdek/internal/moxfield"
	"github.com/hexdek/hexdek/internal/shuffle"
)

// StartGameInput describes the players and decks for a new game.
type StartGameInput struct {
	PartyID string
	Players []StartGamePlayer
}

// StartGamePlayer is a single seat assignment.
type StartGamePlayer struct {
	SeatPosition int
	DeviceID     string
	DeckID       string
	StartingLife int // default 40 if 0
}

// StartGame initializes a new game from a party. Each player's deck is
// loaded, shuffled with crypto/rand, and the first 7 cards are dealt as
// the opening hand. Commander goes to the command zone. Turn state is
// initialized to seat 0, untap phase, turn 1.
func StartGame(ctx context.Context, database *sql.DB, in StartGameInput) (*Game, error) {
	if len(in.Players) < 2 {
		return nil, fmt.Errorf("game requires at least 2 players, got %d", len(in.Players))
	}

	// Generate shuffle seed (used as commitment hash for trustless shuffle)
	rawSeed := db.NewID(64)
	seedHashBytes := sha256.Sum256([]byte(rawSeed))
	seedHash := hex.EncodeToString(seedHashBytes[:])

	g, err := CreateGame(ctx, database, in.PartyID, seedHash)
	if err != nil {
		return nil, err
	}

	for _, sp := range in.Players {
		if sp.StartingLife == 0 {
			sp.StartingLife = 40
		}
		p := &Player{
			GameID:       g.ID,
			SeatPosition: sp.SeatPosition,
			DeviceID:     sp.DeviceID,
			DeckID:       sp.DeckID,
			Life:         sp.StartingLife,
		}
		if err := CreateGamePlayer(ctx, database, p); err != nil {
			return nil, fmt.Errorf("create player seat %d: %w", sp.SeatPosition, err)
		}

		// Load deck and assemble the library
		deckRecord, err := db.GetDeck(ctx, database, sp.DeckID)
		if err != nil {
			return nil, fmt.Errorf("load deck %q for seat %d: %w", sp.DeckID, sp.SeatPosition, err)
		}
		var deckJSON moxfield.Deck
		if err := json.Unmarshal([]byte(deckRecord.RawJSON), &deckJSON); err != nil {
			return nil, fmt.Errorf("parse deck JSON for seat %d: %w", sp.SeatPosition, err)
		}

		// Commander goes straight to command zone
		commanderCard := &Card{
			GameID:     g.ID,
			InstanceID: db.NewID(16),
			Name:       deckJSON.Commander,
			OwnerSeat:  sp.SeatPosition,
			Zone:       ZoneCommand,
		}
		// Try to populate commander mana cost from mainboard listing
		for _, m := range deckJSON.Mainboard {
			if m.Name == deckJSON.Commander {
				commanderCard.ManaCost = m.ManaCost
				commanderCard.CMC = m.CMC
				commanderCard.Types = m.Types
				commanderCard.Subtypes = m.Subtypes
				break
			}
		}
		if err := CreateGameCard(ctx, database, commanderCard); err != nil {
			return nil, fmt.Errorf("create commander for seat %d: %w", sp.SeatPosition, err)
		}

		// Build library, shuffle, insert
		library := deckJSON.ExpandLibrary()
		// Drop any commander instances from the library (they're in command zone)
		filtered := library[:0]
		for _, c := range library {
			if c.Name == deckJSON.Commander {
				continue
			}
			filtered = append(filtered, c)
		}
		if err := shuffle.Shuffle(filtered); err != nil {
			return nil, fmt.Errorf("shuffle for seat %d: %w", sp.SeatPosition, err)
		}

		for i, libCard := range filtered {
			zone := ZoneLibrary
			pos := i
			// First 7 cards go to hand as opening hand
			if i < 7 {
				zone = ZoneHand
				pos = i
			} else {
				pos = i - 7
			}
			gc := &Card{
				GameID:       g.ID,
				InstanceID:   db.NewID(16),
				Name:         libCard.Name,
				ManaCost:     libCard.ManaCost,
				CMC:          libCard.CMC,
				Types:        libCard.Types,
				Subtypes:     libCard.Subtypes,
				OwnerSeat:    sp.SeatPosition,
				Zone:         zone,
				ZonePosition: pos,
			}
			if err := CreateGameCard(ctx, database, gc); err != nil {
				return nil, fmt.Errorf("create card %d for seat %d: %w", i, sp.SeatPosition, err)
			}
		}
	}

	// Initial turn state: seat 0, untap phase
	turn := &TurnState{
		GameID:       g.ID,
		ActiveSeat:   0,
		Phase:        PhaseMain1, // skip untap/upkeep/draw on first turn
		PrioritySeat: 0,
		TurnNumber:   1,
	}
	if err := CreateTurnState(ctx, database, turn); err != nil {
		return nil, fmt.Errorf("create turn state: %w", err)
	}

	logPayload, _ := json.Marshal(map[string]any{
		"shuffle_seed_hash": seedHash,
		"player_count":      len(in.Players),
	})
	_ = AppendActionLog(ctx, database, g.ID, nil, "game_started", string(logPayload))

	return g, nil
}

// AdvancePhase moves the turn to the next phase (and to next player at
// cleanup boundary). Returns the new turn state.
func AdvancePhase(ctx context.Context, database *sql.DB, gameID string, numPlayers int) (*TurnState, error) {
	turn, err := GetTurnState(ctx, database, gameID)
	if err != nil {
		return nil, err
	}

	// Find current phase index
	idx := -1
	for i, p := range PhaseOrder {
		if p == turn.Phase {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil, fmt.Errorf("unknown current phase %q", turn.Phase)
	}

	if turn.Phase == PhaseCleanup {
		// End-of-turn → advance to next player, reset to untap
		turn.ActiveSeat = (turn.ActiveSeat + 1) % numPlayers
		turn.PrioritySeat = turn.ActiveSeat
		turn.Phase = PhaseUntap
		turn.TurnNumber++
		// Untap all permanents for new active player
		if err := untapAll(ctx, database, gameID, turn.ActiveSeat); err != nil {
			return nil, err
		}
		// Reset land-played counter for new active player
		if err := resetLandsPlayed(ctx, database, gameID, turn.ActiveSeat); err != nil {
			return nil, err
		}
	} else {
		// Advance to next phase in normal sequence
		turn.Phase = PhaseOrder[idx+1]
	}

	// Empty mana pools on phase change (MTG rule)
	players, err := ListGamePlayers(ctx, database, gameID)
	if err != nil {
		return nil, err
	}
	for _, p := range players {
		p.ManaPoolW, p.ManaPoolU, p.ManaPoolB = 0, 0, 0
		p.ManaPoolR, p.ManaPoolG, p.ManaPoolC = 0, 0, 0
		_ = UpdateGamePlayer(ctx, database, p)
	}

	if err := UpdateTurnState(ctx, database, turn); err != nil {
		return nil, err
	}

	// Auto-draw on draw phase (skip turn 1 first player per MTG convention — handled by initial Main1 phase)
	if turn.Phase == PhaseDraw {
		if _, err := DrawCards(ctx, database, gameID, turn.ActiveSeat, 1, true); err != nil {
			return nil, fmt.Errorf("auto-draw at draw phase: %w", err)
		}
	}

	logPayload, _ := json.Marshal(map[string]any{
		"new_phase":       string(turn.Phase),
		"new_active_seat": turn.ActiveSeat,
		"turn_number":     turn.TurnNumber,
	})
	seat := turn.ActiveSeat
	_ = AppendActionLog(ctx, database, gameID, &seat, "advance_phase", string(logPayload))

	return turn, nil
}

// DrawCards moves N cards from the top of a player's library to their hand.
// Returns the cards drawn (visible to the drawer only).
//
// Without override, manual draws (button-driven, not engine-driven from a
// phase change or card ability) require an active draw step OR a non-active
// state — so the player can simulate "draw a card" effects but can't farm
// extra cards mid-turn. The auto-draw on phase change (in AdvancePhase) does
// not go through this gate.
func DrawCards(ctx context.Context, database *sql.DB, gameID string, seat int, n int, override bool) ([]*Card, error) {
	if !override {
		turn, err := GetTurnState(ctx, database, gameID)
		if err != nil {
			return nil, err
		}
		if turn.ActiveSeat == seat && turn.Phase != PhaseDraw {
			return nil, fmt.Errorf("manual draw only allowed in draw step or off-turn (current=%s) — pass override to force (e.g. card ability)", turn.Phase)
		}
	}
	library, err := ListCardsInZone(ctx, database, gameID, seat, ZoneLibrary)
	if err != nil {
		return nil, err
	}
	if n > len(library) {
		// Drawing from empty library is a loss condition; for MVP we just
		// draw what's available and let the caller detect the empty state.
		n = len(library)
	}
	handCount, err := CountCardsInZone(ctx, database, gameID, seat, ZoneHand)
	if err != nil {
		return nil, err
	}

	drawn := make([]*Card, 0, n)
	for i := 0; i < n; i++ {
		c := library[i]
		c.Zone = ZoneHand
		c.ZonePosition = handCount + i
		if err := MoveCard(ctx, database, gameID, c.InstanceID, ZoneHand, c.ZonePosition); err != nil {
			return nil, err
		}
		drawn = append(drawn, c)
	}

	// Re-position remaining library cards
	for i, c := range library[n:] {
		if err := MoveCard(ctx, database, gameID, c.InstanceID, ZoneLibrary, i); err != nil {
			return nil, err
		}
	}

	logPayload, _ := json.Marshal(map[string]any{"count": n})
	_ = AppendActionLog(ctx, database, gameID, &seat, "draw_cards", string(logPayload))
	return drawn, nil
}

// PlayLand moves a land card from hand to battlefield. Enforces the
// one-land-per-turn rule and the active-seat rule unless override is true.
//
// override exists because MTG has cards that grant extra land drops
// (Azusa, Exploration, etc.) and instant-speed land plays (Quicksand,
// Crucible-with-fetchland on opp turn) — until we model those, the
// player can manually override.
func PlayLand(ctx context.Context, database *sql.DB, gameID string, seat int, instanceID string, override bool) (*Card, error) {
	card, err := GetGameCard(ctx, database, gameID, instanceID)
	if err != nil {
		return nil, err
	}
	if card.OwnerSeat != seat {
		return nil, fmt.Errorf("card %q not owned by seat %d", instanceID, seat)
	}
	if card.Zone != ZoneHand {
		return nil, fmt.Errorf("card %q not in hand (in %q)", card.Name, card.Zone)
	}
	if !card.IsLand() {
		return nil, fmt.Errorf("card %q is not a land", card.Name)
	}

	if !override {
		// Must be your turn (lands are sorcery-speed by default)
		turn, err := GetTurnState(ctx, database, gameID)
		if err != nil {
			return nil, err
		}
		if turn.ActiveSeat != seat {
			return nil, fmt.Errorf("can only play lands on your own turn (active=seat %d) — pass override to force", turn.ActiveSeat)
		}
		// Must be a main phase
		if turn.Phase != PhaseMain1 && turn.Phase != PhaseMain2 {
			return nil, fmt.Errorf("can only play lands in main1/main2 (current=%s) — pass override to force", turn.Phase)
		}
		// One land per turn
		p, err := GetGamePlayer(ctx, database, gameID, seat)
		if err != nil {
			return nil, err
		}
		if p.LandsPlayedTurn >= 1 {
			return nil, fmt.Errorf("already played a land this turn — pass override to force (e.g. Azusa, Exploration)")
		}
	}

	bfCount, err := CountCardsInZone(ctx, database, gameID, seat, ZoneBattlefield)
	if err != nil {
		return nil, err
	}
	if err := MoveCard(ctx, database, gameID, instanceID, ZoneBattlefield, bfCount); err != nil {
		return nil, err
	}
	card.Zone = ZoneBattlefield
	card.ZonePosition = bfCount

	// Bump lands_played_turn
	if p, err := GetGamePlayer(ctx, database, gameID, seat); err == nil {
		p.LandsPlayedTurn++
		_ = UpdateGamePlayer(ctx, database, p)
	}

	logPayload, _ := json.Marshal(map[string]any{"card_name": card.Name, "instance_id": instanceID, "override": override})
	_ = AppendActionLog(ctx, database, gameID, &seat, "play_land", string(logPayload))
	return card, nil
}

// TapLandForMana taps a land and adds the appropriate mana to the player's pool.
// The color is inferred from the land's subtype (Island=U, Swamp=B, etc.).
// For multi-color duals we let the player specify which color they want via
// the chosenColor parameter.
func TapLandForMana(ctx context.Context, database *sql.DB, gameID string, seat int, instanceID string, chosenColor string) (string, error) {
	card, err := GetGameCard(ctx, database, gameID, instanceID)
	if err != nil {
		return "", err
	}
	if card.OwnerSeat != seat {
		return "", fmt.Errorf("card %q not owned by seat %d", instanceID, seat)
	}
	if card.Zone != ZoneBattlefield {
		return "", fmt.Errorf("card %q not on battlefield", card.Name)
	}
	if !card.IsLand() {
		return "", fmt.Errorf("card %q is not a land", card.Name)
	}
	if card.Tapped {
		return "", fmt.Errorf("card %q is already tapped", card.Name)
	}
	if card.TappedForManaThisTurn {
		// Closes the tap → untap → retap free-mana exploit. Players who legitimately
		// untap-and-retap (e.g. via Aphetto Alchemist, Wilderness Reclamation) can
		// override by clearing the flag manually.
		return "", fmt.Errorf("%q already produced mana this turn (untapping then retapping does NOT refresh that)", card.Name)
	}

	color := chosenColor
	if color == "" {
		color = inferLandColor(card)
	}
	if color == "" {
		// Non-basic, non-typed land (Maze of Ith, Strip Mine, etc.). Default to
		// colorless rather than erroring — the player can always specify a color
		// for true mana-producing utility lands via the picker UI.
		color = "C"
	}

	if err := SetCardTapped(ctx, database, gameID, instanceID, true); err != nil {
		return "", err
	}
	if err := SetTappedForManaThisTurn(ctx, database, gameID, instanceID, true); err != nil {
		return "", err
	}

	player, err := GetGamePlayer(ctx, database, gameID, seat)
	if err != nil {
		return "", err
	}
	switch strings.ToUpper(color) {
	case "W":
		player.ManaPoolW++
	case "U":
		player.ManaPoolU++
	case "B":
		player.ManaPoolB++
	case "R":
		player.ManaPoolR++
	case "G":
		player.ManaPoolG++
	case "C":
		player.ManaPoolC++
	default:
		return "", fmt.Errorf("unknown color %q", color)
	}
	if err := UpdateGamePlayer(ctx, database, player); err != nil {
		return "", err
	}

	logPayload, _ := json.Marshal(map[string]any{
		"land_name": card.Name,
		"color":     color,
	})
	_ = AppendActionLog(ctx, database, gameID, &seat, "tap_land", string(logPayload))
	return color, nil
}

// CastSpell validates the player's mana pool against the spell's cost,
// pays the cost, and moves the card to its appropriate destination
// (battlefield for permanents, graveyard for instants/sorceries).
//
// Without override, sorcery-speed spells (anything that isn't Instant or
// Flash) require it to be your turn AND a main phase. Instants can be
// cast any time (we don't yet model priority/the stack so the gate is
// permissive — server allows, players resolve socially).
func CastSpell(ctx context.Context, database *sql.DB, gameID string, seat int, instanceID string, xValue int, override bool) (*Card, error) {
	card, err := GetGameCard(ctx, database, gameID, instanceID)
	if err != nil {
		return nil, err
	}
	if card.OwnerSeat != seat {
		return nil, fmt.Errorf("card %q not owned by seat %d", instanceID, seat)
	}
	if card.Zone != ZoneHand && card.Zone != ZoneCommand {
		return nil, fmt.Errorf("card %q not castable from %q", card.Name, card.Zone)
	}

	if !override {
		isInstant := hasType(card.Types, "Instant") || hasType(card.Subtypes, "Flash")
		if !isInstant {
			turn, err := GetTurnState(ctx, database, gameID)
			if err != nil {
				return nil, err
			}
			if turn.ActiveSeat != seat {
				return nil, fmt.Errorf("can only cast sorcery-speed spells on your turn — pass override to force")
			}
			if turn.Phase != PhaseMain1 && turn.Phase != PhaseMain2 {
				return nil, fmt.Errorf("can only cast sorcery-speed spells in main1/main2 (current=%s) — pass override to force", turn.Phase)
			}
		}
	}

	cost, err := mana.Parse(card.ManaCost)
	if err != nil {
		return nil, fmt.Errorf("parse mana cost %q: %w", card.ManaCost, err)
	}
	player, err := GetGamePlayer(ctx, database, gameID, seat)
	if err != nil {
		return nil, err
	}
	pool := mana.Pool{
		W: player.ManaPoolW, U: player.ManaPoolU, B: player.ManaPoolB,
		R: player.ManaPoolR, G: player.ManaPoolG, C: player.ManaPoolC,
	}
	if !pool.CanPay(cost, xValue) {
		return nil, fmt.Errorf("insufficient mana to cast %s (cost: %s, pool: %s)", card.Name, cost.String(), pool.String())
	}
	if err := pool.Pay(cost, xValue); err != nil {
		return nil, err
	}
	player.ManaPoolW, player.ManaPoolU, player.ManaPoolB = pool.W, pool.U, pool.B
	player.ManaPoolR, player.ManaPoolG, player.ManaPoolC = pool.R, pool.G, pool.C
	if err := UpdateGamePlayer(ctx, database, player); err != nil {
		return nil, err
	}

	// Determine destination
	destZone := ZoneBattlefield
	if card.IsInstantOrSorcery() {
		destZone = ZoneGraveyard
	}
	destCount, err := CountCardsInZone(ctx, database, gameID, seat, destZone)
	if err != nil {
		return nil, err
	}
	if err := MoveCard(ctx, database, gameID, instanceID, destZone, destCount); err != nil {
		return nil, err
	}
	card.Zone = destZone
	card.ZonePosition = destCount

	logPayload, _ := json.Marshal(map[string]any{
		"card_name":   card.Name,
		"instance_id": instanceID,
		"destination": string(destZone),
		"x_value":     xValue,
	})
	_ = AppendActionLog(ctx, database, gameID, &seat, "cast_spell", string(logPayload))
	return card, nil
}

// YurikoReveal is the deck's signature trigger: when a ninja deals combat
// damage to an opponent, reveal the top card of the attacker's library;
// the opponent loses life equal to its mana value.
//
// MVP simplification: caller passes targetSeat and the system reveals + applies.
func YurikoReveal(ctx context.Context, database *sql.DB, gameID string, attackerSeat int, targetSeat int) (revealedCard *Card, damage int, err error) {
	library, err := ListCardsInZone(ctx, database, gameID, attackerSeat, ZoneLibrary)
	if err != nil {
		return nil, 0, err
	}
	if len(library) == 0 {
		return nil, 0, fmt.Errorf("library empty — no reveal possible")
	}
	top := library[0]
	damage = top.CMC

	target, err := GetGamePlayer(ctx, database, gameID, targetSeat)
	if err != nil {
		return top, damage, err
	}
	target.Life -= damage
	if err := UpdateGamePlayer(ctx, database, target); err != nil {
		return top, damage, err
	}

	logPayload, _ := json.Marshal(map[string]any{
		"attacker_seat":  attackerSeat,
		"target_seat":    targetSeat,
		"revealed_card":  top.Name,
		"revealed_cmc":   top.CMC,
		"damage":         damage,
	})
	_ = AppendActionLog(ctx, database, gameID, &attackerSeat, "yuriko_reveal", string(logPayload))
	return top, damage, nil
}

// Snapshot returns a per-player view of the game with hidden info redacted.
func Snapshot(ctx context.Context, database *sql.DB, gameID string, viewerSeat int) (*SnapshotForPlayer, error) {
	g, err := GetGame(ctx, database, gameID)
	if err != nil {
		return nil, err
	}
	turn, err := GetTurnState(ctx, database, gameID)
	if err != nil {
		return nil, err
	}
	players, err := ListGamePlayers(ctx, database, gameID)
	if err != nil {
		return nil, err
	}

	snap := &SnapshotForPlayer{
		Game:         g,
		Turn:         turn,
		Battlefield:  make(map[int][]*Card),
		Command:      make(map[int][]*Card),
		Graveyards:   make(map[int][]*Card),
		OppHandSizes: make(map[int]int),
		OppLibSizes:  make(map[int]int),
		GeneratedAt:  time.Now(),
	}
	for _, p := range players {
		bf, err := ListCardsInZone(ctx, database, gameID, p.SeatPosition, ZoneBattlefield)
		if err != nil {
			return nil, err
		}
		snap.Battlefield[p.SeatPosition] = bf
		// Commander zone is public in EDH — every player can see every commander.
		cmd, _ := ListCardsInZone(ctx, database, gameID, p.SeatPosition, ZoneCommand)
		snap.Command[p.SeatPosition] = cmd
		// Graveyard is also public information.
		gy, _ := ListCardsInZone(ctx, database, gameID, p.SeatPosition, ZoneGraveyard)
		snap.Graveyards[p.SeatPosition] = gy

		if p.SeatPosition == viewerSeat {
			snap.You = p
			snap.YourHand, _ = ListCardsInZone(ctx, database, gameID, p.SeatPosition, ZoneHand)
			snap.YourGY = gy
			snap.YourExile, _ = ListCardsInZone(ctx, database, gameID, p.SeatPosition, ZoneExile)
			snap.YourLib, _ = CountCardsInZone(ctx, database, gameID, p.SeatPosition, ZoneLibrary)
		} else {
			snap.Opponents = append(snap.Opponents, p)
			snap.OppHandSizes[p.SeatPosition], _ = CountCardsInZone(ctx, database, gameID, p.SeatPosition, ZoneHand)
			snap.OppLibSizes[p.SeatPosition], _ = CountCardsInZone(ctx, database, gameID, p.SeatPosition, ZoneLibrary)
		}
	}
	return snap, nil
}

// SetTapped is the player-driven tap/untap action for any permanent on the
// battlefield. Lands tapped via this path do NOT add mana — use TapLandForMana
// for that. This is for combat, ability activation, manual untap, etc.
func SetTapped(ctx context.Context, database *sql.DB, gameID string, seat int, instanceID string, tapped bool) error {
	card, err := GetGameCard(ctx, database, gameID, instanceID)
	if err != nil {
		return err
	}
	if card.OwnerSeat != seat {
		return fmt.Errorf("card %q not owned by seat %d", instanceID, seat)
	}
	if card.Zone != ZoneBattlefield {
		return fmt.Errorf("card %q not on battlefield", card.Name)
	}
	if err := SetCardTapped(ctx, database, gameID, instanceID, tapped); err != nil {
		return err
	}
	action := "untap_card"
	if tapped {
		action = "tap_card"
	}
	logPayload, _ := json.Marshal(map[string]any{
		"card_name":   card.Name,
		"instance_id": instanceID,
	})
	_ = AppendActionLog(ctx, database, gameID, &seat, action, string(logPayload))
	return nil
}

// UntapAllForSeat untaps every permanent on the seat's battlefield. Same as
// the auto-untap that runs at the untap step, but callable on demand.
func UntapAllForSeat(ctx context.Context, database *sql.DB, gameID string, seat int) error {
	if err := untapAll(ctx, database, gameID, seat); err != nil {
		return err
	}
	logPayload, _ := json.Marshal(map[string]any{"seat": seat})
	_ = AppendActionLog(ctx, database, gameID, &seat, "untap_all", string(logPayload))
	return nil
}

// AdjustLife is a manual life-change for the seat. Used by the +/- buttons in
// the UI for combat damage, life-gain effects, commander damage, etc. when we
// don't have a card-driven path yet.
func AdjustLife(ctx context.Context, database *sql.DB, gameID string, seat int, delta int) error {
	p, err := GetGamePlayer(ctx, database, gameID, seat)
	if err != nil {
		return err
	}
	p.Life += delta
	if err := UpdateGamePlayer(ctx, database, p); err != nil {
		return err
	}
	logPayload, _ := json.Marshal(map[string]any{
		"delta":    delta,
		"new_life": p.Life,
	})
	_ = AppendActionLog(ctx, database, gameID, &seat, "adjust_life", string(logPayload))
	return nil
}

// ----- internals -----

func untapAll(ctx context.Context, database *sql.DB, gameID string, seat int) error {
	bf, err := ListCardsInZone(ctx, database, gameID, seat, ZoneBattlefield)
	if err != nil {
		return err
	}
	for _, c := range bf {
		if c.Tapped {
			_ = SetCardTapped(ctx, database, gameID, c.InstanceID, false)
		}
	}
	// Clear the per-turn mana-tap flag in bulk — at the start of your turn,
	// every land you control is fresh to tap for mana again.
	_ = ClearTappedForManaForSeat(ctx, database, gameID, seat)
	return nil
}

func resetLandsPlayed(ctx context.Context, database *sql.DB, gameID string, seat int) error {
	p, err := GetGamePlayer(ctx, database, gameID, seat)
	if err != nil {
		return err
	}
	p.LandsPlayedTurn = 0
	return UpdateGamePlayer(ctx, database, p)
}

// inferLandColor returns the dominant color for a basic land or
// returns empty string if the land has multiple colors and the player
// must choose.
func inferLandColor(card *Card) string {
	for _, sub := range card.Subtypes {
		switch sub {
		case "Plains":
			return "W"
		case "Island":
			return "U"
		case "Swamp":
			return "B"
		case "Mountain":
			return "R"
		case "Forest":
			return "G"
		}
	}
	// Multi-color or non-basic: caller must specify
	return ""
}

// itoa is a stdlib alias kept here so future phase-name handling can grow
// without circular imports.
var _ = strconv.Itoa
