// Package ai runs an autopilot for seats marked is_ai=1. It plays lands,
// taps for mana, casts the most expensive affordable spell, attacks with
// all available creatures, and advances phases — enough for a human to
// playtest the full game loop against AI opponents.
package ai

import (
	"context"
	"database/sql"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/game"
	"github.com/hexdek/hexdek/internal/mana"
)

// BroadcastFn is called after each AI-driven advance so the WS layer can
// push fresh snapshots to connected clients. It's decoupled from this
// package so we don't take a dependency on internal/ws.
type BroadcastFn func(gameID string)

var (
	mu      sync.Mutex
	running = map[string]context.CancelFunc{} // gameID → cancel
)

// Start spawns a single autopilot goroutine for the game. Safe to call
// multiple times for the same game — only the first call takes effect.
func Start(parent context.Context, database *sql.DB, gameID string, numPlayers int, broadcast BroadcastFn) {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := running[gameID]; ok {
		return
	}

	aiSeats, err := aiSeatsForGame(parent, database, gameID)
	if err != nil {
		log.Printf("ai: list ai seats for %s: %v", gameID, err)
		return
	}
	if len(aiSeats) == 0 {
		return
	}

	ctx, cancel := context.WithCancel(parent)
	running[gameID] = cancel

	go run(ctx, database, gameID, aiSeats, numPlayers, broadcast)
	log.Printf("ai: autopilot started for game %s (ai seats: %v)", gameID, aiSeats)
}

// Stop halts autopilot for the game (called when the game finishes).
func Stop(gameID string) {
	mu.Lock()
	defer mu.Unlock()
	if cancel, ok := running[gameID]; ok {
		cancel()
		delete(running, gameID)
	}
}

func run(ctx context.Context, database *sql.DB, gameID string, aiSeats map[int]bool, numPlayers int, broadcast BroadcastFn) {
	ticker := time.NewTicker(700 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		turn, err := game.GetTurnState(ctx, database, gameID)
		if err != nil {
			log.Printf("ai: game %s turn state gone: %v", gameID, err)
			Stop(gameID)
			return
		}
		if !aiSeats[turn.ActiveSeat] {
			continue
		}

		seat := turn.ActiveSeat

		switch turn.Phase {
		case game.PhaseMain1, game.PhaseMain2:
			playLand(ctx, database, gameID, seat)
			tapAllLands(ctx, database, gameID, seat)
			castSpells(ctx, database, gameID, seat)
		case game.PhaseCombat:
			declareAttackers(ctx, database, gameID, seat, numPlayers)
			_, _ = game.ResolveCombat(ctx, database, gameID)
		}

		if _, err := game.AdvancePhase(ctx, database, gameID, numPlayers); err != nil {
			log.Printf("ai: advance phase game %s: %v", gameID, err)
			continue
		}
		if broadcast != nil {
			broadcast(gameID)
		}
	}
}

func playLand(ctx context.Context, database *sql.DB, gameID string, seat int) {
	hand, err := game.ListCardsInZone(ctx, database, gameID, seat, game.ZoneHand)
	if err != nil {
		return
	}
	for _, c := range hand {
		if c.IsLand() {
			if _, err := game.PlayLand(ctx, database, gameID, seat, c.InstanceID, false); err == nil {
				return
			}
		}
	}
}

func tapAllLands(ctx context.Context, database *sql.DB, gameID string, seat int) {
	bf, err := game.ListCardsInZone(ctx, database, gameID, seat, game.ZoneBattlefield)
	if err != nil {
		return
	}
	for _, c := range bf {
		if c.IsLand() && !c.Tapped {
			game.TapLandForMana(ctx, database, gameID, seat, c.InstanceID, "")
		}
	}
}

func castSpells(ctx context.Context, database *sql.DB, gameID string, seat int) {
	for {
		hand, err := game.ListCardsInZone(ctx, database, gameID, seat, game.ZoneHand)
		if err != nil || len(hand) == 0 {
			return
		}

		player, err := game.GetGamePlayer(ctx, database, gameID, seat)
		if err != nil {
			return
		}
		pool := mana.Pool{
			W: player.ManaPoolW, U: player.ManaPoolU, B: player.ManaPoolB,
			R: player.ManaPoolR, G: player.ManaPoolG, C: player.ManaPoolC,
		}
		if pool.Total() == 0 {
			return
		}

		sort.Slice(hand, func(i, j int) bool {
			return hand[i].CMC > hand[j].CMC
		})

		cast := false
		for _, c := range hand {
			if c.IsLand() {
				continue
			}
			cost, err := mana.Parse(c.ManaCost)
			if err != nil {
				continue
			}
			if pool.CanPay(cost, 0) {
				if _, err := game.CastSpell(ctx, database, gameID, seat, c.InstanceID, 0, false); err == nil {
					cast = true
					break
				}
			}
		}
		if !cast {
			return
		}
	}
}

func declareAttackers(ctx context.Context, database *sql.DB, gameID string, seat int, numPlayers int) {
	bf, err := game.ListCardsInZone(ctx, database, gameID, seat, game.ZoneBattlefield)
	if err != nil {
		return
	}

	target := -1
	for i := 0; i < numPlayers; i++ {
		if i != seat {
			target = i
			break
		}
	}
	if target < 0 {
		return
	}

	var specs []game.AttackerSpec
	for _, c := range bf {
		if c.IsCreature() && !c.Tapped {
			specs = append(specs, game.AttackerSpec{
				InstanceID: c.InstanceID,
				TargetSeat: target,
			})
		}
	}
	if len(specs) > 0 {
		_ = game.DeclareAttackers(ctx, database, gameID, seat, specs)
	}
}

func aiSeatsForGame(ctx context.Context, database *sql.DB, gameID string) (map[int]bool, error) {
	rows, err := database.QueryContext(ctx, `
		SELECT gp.seat_position
		FROM game_player gp
		JOIN game g ON g.id = gp.game_id
		JOIN party_member pm ON pm.party_id = g.party_id AND pm.device_id = gp.device_id
		WHERE gp.game_id = ? AND pm.is_ai = 1
	`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int]bool{}
	for rows.Next() {
		var seat int
		if err := rows.Scan(&seat); err != nil {
			return nil, err
		}
		out[seat] = true
	}
	return out, rows.Err()
}

var _ = db.Now
