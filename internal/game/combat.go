package game

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/hexdek/hexdek/internal/db"
)

// AttackerSpec is a declaration that a single creature attacks a target player.
type AttackerSpec struct {
	InstanceID string `json:"instance_id"`
	TargetSeat int    `json:"target_seat"`
}

// BlockerSpec is a declaration that a blocker blocks a specific attacker.
type BlockerSpec struct {
	BlockerID  string `json:"blocker_id"`
	AttackerID string `json:"attacker_id"`
}

// DamageReport summarizes the damage dealt during combat resolution.
type DamageReport struct {
	PlayerDamage map[int]int    `json:"player_damage"` // seat → total damage taken
	CreatureDeaths []string     `json:"creature_deaths"` // instance IDs that died
	YurikoTriggers []YurikoHit  `json:"yuriko_triggers"` // ninjas (or commanders matching) that hit a player
}

// YurikoHit records a Yuriko-style trigger from combat damage to a player.
type YurikoHit struct {
	AttackerSeat  int    `json:"attacker_seat"`
	AttackerName  string `json:"attacker_name"`
	TargetSeat    int    `json:"target_seat"`
	RevealedCard  string `json:"revealed_card"`
	RevealedCMC   int    `json:"revealed_cmc"`
	Damage        int    `json:"damage"`
}

// DeclareAttackers records the attackers for the current combat. Called
// during the Combat phase by the active player. Validates that each
// attacker is a creature owned by the seat and not tapped.
func DeclareAttackers(ctx context.Context, database *sql.DB, gameID string, seat int, specs []AttackerSpec) error {
	turn, err := GetTurnState(ctx, database, gameID)
	if err != nil {
		return err
	}
	if turn.ActiveSeat != seat {
		return fmt.Errorf("only the active player can declare attackers")
	}
	if turn.Phase != PhaseCombat {
		return fmt.Errorf("must be in combat phase to declare attackers (current: %s)", turn.Phase)
	}

	// Clear any previous declarations
	_, _ = database.ExecContext(ctx, `DELETE FROM combat_attacker WHERE game_id = ?`, gameID)
	_, _ = database.ExecContext(ctx, `DELETE FROM combat_blocker WHERE game_id = ?`, gameID)

	now := db.Now()
	for _, spec := range specs {
		card, err := GetGameCard(ctx, database, gameID, spec.InstanceID)
		if err != nil {
			return fmt.Errorf("attacker %s: %w", spec.InstanceID, err)
		}
		if card.OwnerSeat != seat {
			return fmt.Errorf("attacker %s not owned by seat %d", card.Name, seat)
		}
		if card.Zone != ZoneBattlefield {
			return fmt.Errorf("attacker %s not on battlefield", card.Name)
		}
		if !card.IsCreature() {
			return fmt.Errorf("%s is not a creature", card.Name)
		}
		if card.Tapped {
			return fmt.Errorf("%s is tapped, can't attack", card.Name)
		}
		// Tap attacker (standard MTG rule)
		if err := SetCardTapped(ctx, database, gameID, card.InstanceID, true); err != nil {
			return err
		}
		_, err = database.ExecContext(ctx,
			`INSERT INTO combat_attacker (game_id, instance_id, target_seat, declared_at) VALUES (?, ?, ?, ?)`,
			gameID, spec.InstanceID, spec.TargetSeat, now)
		if err != nil {
			return fmt.Errorf("insert attacker: %w", err)
		}
	}

	logPayload, _ := json.Marshal(map[string]any{"attackers": specs})
	_ = AppendActionLog(ctx, database, gameID, &seat, "declare_attackers", string(logPayload))
	return nil
}

// DeclareBlockers records blocking assignments for one defending player.
func DeclareBlockers(ctx context.Context, database *sql.DB, gameID string, defenderSeat int, specs []BlockerSpec) error {
	turn, err := GetTurnState(ctx, database, gameID)
	if err != nil {
		return err
	}
	if turn.Phase != PhaseCombat {
		return fmt.Errorf("must be in combat phase to declare blockers (current: %s)", turn.Phase)
	}
	if turn.ActiveSeat == defenderSeat {
		return fmt.Errorf("active player cannot block their own attackers")
	}

	now := db.Now()
	for _, spec := range specs {
		blocker, err := GetGameCard(ctx, database, gameID, spec.BlockerID)
		if err != nil {
			return fmt.Errorf("blocker %s: %w", spec.BlockerID, err)
		}
		if blocker.OwnerSeat != defenderSeat {
			return fmt.Errorf("blocker %s not owned by seat %d", blocker.Name, defenderSeat)
		}
		if blocker.Zone != ZoneBattlefield {
			return fmt.Errorf("blocker %s not on battlefield", blocker.Name)
		}
		if !blocker.IsCreature() {
			return fmt.Errorf("%s is not a creature", blocker.Name)
		}
		if blocker.Tapped {
			return fmt.Errorf("%s is tapped, can't block", blocker.Name)
		}
		// Validate the attacker exists in current combat
		var found int
		_ = database.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM combat_attacker WHERE game_id = ? AND instance_id = ?`,
			gameID, spec.AttackerID).Scan(&found)
		if found == 0 {
			return fmt.Errorf("attacker %s not in current combat", spec.AttackerID)
		}
		_, err = database.ExecContext(ctx,
			`INSERT INTO combat_blocker (game_id, blocker_id, attacker_id, declared_at) VALUES (?, ?, ?, ?)`,
			gameID, spec.BlockerID, spec.AttackerID, now)
		if err != nil {
			return fmt.Errorf("insert blocker: %w", err)
		}
	}

	logPayload, _ := json.Marshal(map[string]any{"blockers": specs})
	_ = AppendActionLog(ctx, database, gameID, &defenderSeat, "declare_blockers", string(logPayload))
	return nil
}

// ResolveCombat applies combat damage. For every attacker:
//   - if blocked, attacker and blockers exchange power as damage
//     (attacker's full power split however; for MVP we just kill any
//     blocker if attacker.power >= blocker.toughness, AND kill attacker
//     if blockers.totalPower >= attacker.toughness)
//   - if unblocked, attacker deals damage to defending player
//     (and triggers Yuriko reveal if attacker is a Ninja)
// Returns a damage report.
func ResolveCombat(ctx context.Context, database *sql.DB, gameID string) (*DamageReport, error) {
	turn, err := GetTurnState(ctx, database, gameID)
	if err != nil {
		return nil, err
	}
	if turn.Phase != PhaseCombat {
		return nil, fmt.Errorf("not in combat phase (current: %s)", turn.Phase)
	}

	report := &DamageReport{
		PlayerDamage:   map[int]int{},
		CreatureDeaths: []string{},
		YurikoTriggers: []YurikoHit{},
	}

	// Fetch all attackers
	rows, err := database.QueryContext(ctx,
		`SELECT instance_id, target_seat FROM combat_attacker WHERE game_id = ?`, gameID)
	if err != nil {
		return nil, err
	}
	type attackInfo struct {
		ID         string
		TargetSeat int
	}
	var attackers []attackInfo
	for rows.Next() {
		var a attackInfo
		_ = rows.Scan(&a.ID, &a.TargetSeat)
		attackers = append(attackers, a)
	}
	rows.Close()

	for _, atk := range attackers {
		card, err := GetGameCard(ctx, database, gameID, atk.ID)
		if err != nil {
			continue
		}
		// Find blockers for this attacker
		blockerRows, err := database.QueryContext(ctx,
			`SELECT blocker_id FROM combat_blocker WHERE game_id = ? AND attacker_id = ?`, gameID, atk.ID)
		if err != nil {
			continue
		}
		var blockerIDs []string
		for blockerRows.Next() {
			var bid string
			_ = blockerRows.Scan(&bid)
			blockerIDs = append(blockerIDs, bid)
		}
		blockerRows.Close()

		attackerPower := creaturePower(card)
		attackerToughness := creatureToughness(card)

		if len(blockerIDs) == 0 {
			// Unblocked: damage to defending player
			target, err := GetGamePlayer(ctx, database, gameID, atk.TargetSeat)
			if err == nil && attackerPower > 0 {
				target.Life -= attackerPower
				_ = UpdateGamePlayer(ctx, database, target)
				report.PlayerDamage[atk.TargetSeat] += attackerPower
			}
			// Yuriko reveal trigger if attacker is a Ninja
			if hasType(card.Subtypes, "Ninja") {
				revealed, dmg, err := YurikoReveal(ctx, database, gameID, card.OwnerSeat, atk.TargetSeat)
				if err == nil {
					report.YurikoTriggers = append(report.YurikoTriggers, YurikoHit{
						AttackerSeat: card.OwnerSeat,
						AttackerName: card.Name,
						TargetSeat:   atk.TargetSeat,
						RevealedCard: revealed.Name,
						RevealedCMC:  revealed.CMC,
						Damage:       dmg,
					})
				}
			}
			continue
		}

		// Blocked: attacker deals damage to blockers, blockers deal damage to attacker
		blockerTotalPower := 0
		blockerTotalToughness := 0
		for _, bid := range blockerIDs {
			b, err := GetGameCard(ctx, database, gameID, bid)
			if err != nil {
				continue
			}
			bp := creaturePower(b)
			bt := creatureToughness(b)
			blockerTotalPower += bp
			blockerTotalToughness += bt
			// Attacker deals damage to first blocker that can absorb it; then
			// remaining damage spills to next blocker. For MVP simplification,
			// just check if attacker.power >= blocker.toughness for each.
			if attackerPower >= bt {
				_ = MoveCard(ctx, database, gameID, b.InstanceID, ZoneGraveyard, 0)
				report.CreatureDeaths = append(report.CreatureDeaths, b.InstanceID)
				attackerPower -= bt
			}
		}
		// If blockers' total power >= attacker.toughness, attacker dies
		if blockerTotalPower >= attackerToughness {
			_ = MoveCard(ctx, database, gameID, card.InstanceID, ZoneGraveyard, 0)
			report.CreatureDeaths = append(report.CreatureDeaths, card.InstanceID)
		}
	}

	// Clear combat tracking
	_, _ = database.ExecContext(ctx, `DELETE FROM combat_attacker WHERE game_id = ?`, gameID)
	_, _ = database.ExecContext(ctx, `DELETE FROM combat_blocker WHERE game_id = ?`, gameID)

	logPayload, _ := json.Marshal(report)
	_ = AppendActionLog(ctx, database, gameID, nil, "resolve_combat", string(logPayload))

	// Check game-end after combat damage
	_ = CheckGameEnd(ctx, database, gameID)

	return report, nil
}

// CheckGameEnd is the engine-level master aggregator across CR §104.2.
// Two-level structure:
//
//  1. Per-seat: Player.CheckLossConditions() classifies each seat against
//     its own §704.5 clauses (life, empty-library, poison, …) in
//     isolation. Pure method — no DB access, easy to unit-test.
//  2. Engine-level (here): collect every seat's loss result, then apply
//     the multiplayer-resolution clauses.
//
// Resolution order:
//
//   - §104.2c (FIRST): any seat with WinEffectTriggered=true wins
//     immediately, even if that same seat would otherwise meet a §704.5
//     loss condition (CR §104.3a — win effects take precedence over
//     simultaneous loss effects on the same player). When multiple seats
//     trigger a win effect in the same window, the lowest seat index
//     wins (deterministic tie-break; matches turn-order priority).
//   - §104.2a: all-but-one seat lost → the surviving seat wins.
//   - §104.2b: all remaining seats lost simultaneously → the game ends
//     as a draw (Winner="").
//
// A no-op return (nil with no FinishGame call) means the game is still
// in progress. Callers must invoke this after any state change that
// could resolve a loss/win condition (combat damage, spell resolution,
// SBA pass, priority pass).
func CheckGameEnd(ctx context.Context, database *sql.DB, gameID string) error {
	players, err := ListGamePlayers(ctx, database, gameID)
	if err != nil {
		return err
	}
	// §104.2c — "you win the game" effects override loss effects on the
	// same seat (CR §104.3a). Scan first, in seat order, so a triggered
	// winner is finalized before any §704.5 elimination is applied.
	for _, p := range players {
		if p == nil || !p.WinEffectTriggered {
			continue
		}
		return FinishGame(ctx, database, gameID, p.DeviceID)
	}
	type alive struct {
		seat     int
		deviceID string
	}
	var aliveList []alive
	for _, p := range players {
		if _, lost := p.CheckLossConditions(); lost {
			continue
		}
		aliveList = append(aliveList, alive{seat: p.SeatPosition, deviceID: p.DeviceID})
	}
	if len(aliveList) == 1 {
		// §104.2a — last seat standing wins.
		return FinishGame(ctx, database, gameID, aliveList[0].deviceID)
	}
	if len(aliveList) == 0 {
		// §104.2b — simultaneous elimination → draw.
		return FinishGame(ctx, database, gameID, "")
	}
	return nil
}

// creaturePower returns the creature's combat power.
//
// Reads Card.Power directly (populated by CreateGameCard from the deck
// JSON's `power` field, hydrated via marshalCardData / hydrateCardData).
// Falls back to 1 when both Power and Toughness are zero — that's the
// "deck JSON omitted P/T metadata" case, where the 1/1 default
// preserves the pre-r60 MVP behavior. A legitimate 0/0 creature
// (Tarmogoyf with no types, Spellskite-as-printed) is indistinguishable
// from "missing" in the current schema and will also receive the 1/1
// fallback — acceptable for the MVP since the live-game test
// endpoints don't run those cards. Full §613 layers (counters,
// until-EOT modifications, anthems) are NOT modeled here — that's the
// gameengine package; if/when those land in the live game, this is
// the function that needs to consume them.
func creaturePower(card *Card) int {
	if card == nil {
		return 0
	}
	if card.Power == 0 && card.Toughness == 0 {
		return 1
	}
	return card.Power
}

// creatureToughness returns the creature's combat toughness. See
// creaturePower for the fallback semantics.
func creatureToughness(card *Card) int {
	if card == nil {
		return 0
	}
	if card.Power == 0 && card.Toughness == 0 {
		return 1
	}
	return card.Toughness
}
