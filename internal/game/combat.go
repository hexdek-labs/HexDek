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

// CheckGameEnd evaluates win conditions: any player at life <= 0 is out,
// any player drawing from empty library is out, any player at 10+ poison
// is out. If exactly one player remains, finish the game with that winner.
func CheckGameEnd(ctx context.Context, database *sql.DB, gameID string) error {
	players, err := ListGamePlayers(ctx, database, gameID)
	if err != nil {
		return err
	}
	type alive struct {
		seat     int
		deviceID string
	}
	var aliveList []alive
	for _, p := range players {
		if p.Life <= 0 {
			continue
		}
		if p.PoisonCounters >= 10 {
			continue
		}
		// Library size check
		libCount, _ := CountCardsInZone(ctx, database, gameID, p.SeatPosition, ZoneLibrary)
		if libCount == 0 {
			// Drawing from empty = lose at next draw (we'll mark as eliminated
			// only if a draw is attempted; for now, library 0 doesn't auto-lose)
			// keep in alive list
		}
		aliveList = append(aliveList, alive{seat: p.SeatPosition, deviceID: p.DeviceID})
	}
	if len(aliveList) == 1 {
		return FinishGame(ctx, database, gameID, aliveList[0].deviceID)
	}
	if len(aliveList) == 0 {
		// Draw — no winner
		return FinishGame(ctx, database, gameID, "")
	}
	return nil
}

// creaturePower extracts a creature's power from its types/subtypes
// metadata. For MVP we use a stub: return 1 if no power is specified.
// Real implementation would parse from oracle data.
func creaturePower(card *Card) int {
	// Placeholder: until we wire Scryfall power/toughness into the deck JSON,
	// every creature is 1/1. Yuriko-tribal decks live and die on Yuriko's
	// reveal trigger, not on raw P/T anyway, so this is fine for MVP.
	return 1
}

func creatureToughness(card *Card) int {
	return 1
}
