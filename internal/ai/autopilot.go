// Package ai runs a dead-simple autopilot for seats marked is_ai=1. It
// polls turn state and, whenever an AI-owned seat has active priority, it
// advances the phase. That's enough to let a single human test the full
// game loop without a second player, and enough to keep the game flowing
// past AI turns in a mixed party.
//
// This is intentionally not "smart" — no card play, no combat decisions.
// Future work: add a behavior policy that actually plays lands, casts
// spells, attacks when favorable, etc. For now it just holds the clock.
package ai

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/game"
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

	// Figure out which seats are AI-controlled.
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
	// Small delay between advances so humans can see the snapshots tick
	// through the AI's turn.
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
			// Game was deleted or DB went away — exit loop.
			log.Printf("ai: game %s turn state gone: %v", gameID, err)
			Stop(gameID)
			return
		}
		if !aiSeats[turn.ActiveSeat] {
			continue
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

func aiSeatsForGame(ctx context.Context, database *sql.DB, gameID string) (map[int]bool, error) {
	// Join game_player → party_member to find seats flagged is_ai=1.
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

// Keep the db package referenced so imports don't get trimmed by goimports
// during build if this file is the only consumer — harmless var.
var _ = db.Now
