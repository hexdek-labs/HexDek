package game

import (
	"context"
	"database/sql"
	"regexp"
	"strconv"
	"strings"
)

// LayerContext holds the per-combat continuous-effects context — the
// minimum CR §613 layer-7 substrate the live-game MVP needs to make
// "Glorious Anthem + Serra Angel = 5/5 attacker" come out right.
//
// Populated by BuildLayerContext at the top of ResolveCombat, threaded
// to combatPower / combatToughness. nil is safe — every layered read
// degrades to base P/T.
//
// What's modeled (matches gameengine's Permanent.Power in state.go:1213):
//   - Layer 7a: base (Card.Power / Card.Toughness, sourced from the
//     oracle cache at CreateGameCard time)
//   - Layer 7e — anthem-style continuous effects: "Creatures you control
//     get +N/+M" from cards on the controller's battlefield
//
// What's NOT modeled yet (future Card-schema extensions):
//   - Layer 7e — counters (+1/+1 / -1/-1): Card has no counter slots in
//     the SQLite schema today; once those land, anthemDelta sits next to
//     a counterDelta read with the same shape
//   - Layer 7d — set-P/T effects (Glorious Anthem-style buffs are 7e,
//     but "becomes a 4/4" is 7d and applies before 7e)
//   - Layer 7b — type-changing (Mistform / Conspiracy)
//   - Layer 7f — switch power-toughness (Sleeper Agent / Eight-and-a-Half-Tails)
type LayerContext struct {
	// BattlefieldBySeat is the full battlefield indexed by controller
	// seat. Used to enumerate anthem-bearers under the controller's
	// command — "Creatures you control" scopes to the seat that
	// controls the anthem source, NOT the seat that owns the buffed
	// creature.
	BattlefieldBySeat map[int][]*Card

	// OracleText maps lowercased card name → oracle text, populated
	// from the card_oracle cache. Anthems are detected by regex over
	// this text; cards with no cache entry contribute nothing (silent
	// degradation matches the P/T-hydration fallback).
	OracleText map[string]string
}

// BuildLayerContext snapshots every seat's battlefield + looks up oracle
// text for the unique card names. One DB hit per battlefield, one IN-list
// query against card_oracle.
//
// Returns nil + error on a DB failure. Callers may pass the nil to the
// layer-reads as a degraded fallback (defensive — combat shouldn't
// silently skip damage because the layer query failed).
func BuildLayerContext(ctx context.Context, database *sql.DB, gameID string) (*LayerContext, error) {
	players, err := ListGamePlayers(ctx, database, gameID)
	if err != nil {
		return nil, err
	}
	lc := &LayerContext{
		BattlefieldBySeat: make(map[int][]*Card, len(players)),
		OracleText:        make(map[string]string),
	}
	names := make(map[string]bool)
	for _, p := range players {
		bf, err := ListCardsInZone(ctx, database, gameID, p.SeatPosition, ZoneBattlefield)
		if err != nil {
			return nil, err
		}
		lc.BattlefieldBySeat[p.SeatPosition] = bf
		for _, c := range bf {
			if c != nil {
				names[strings.ToLower(strings.TrimSpace(c.Name))] = true
			}
		}
	}
	if len(names) == 0 {
		return lc, nil
	}
	keys := make([]string, 0, len(names))
	placeholders := make([]string, 0, len(names))
	args := make([]any, 0, len(names))
	for k := range names {
		keys = append(keys, k)
		placeholders = append(placeholders, "?")
		args = append(args, k)
	}
	q := "SELECT name, oracle_text FROM card_oracle WHERE name IN (" + strings.Join(placeholders, ",") + ")"
	rows, err := database.QueryContext(ctx, q, args...)
	if err != nil {
		// Cache miss is non-fatal — anthems just won't apply.
		return lc, nil
	}
	defer rows.Close()
	for rows.Next() {
		var name, text string
		if err := rows.Scan(&name, &text); err != nil {
			continue
		}
		lc.OracleText[name] = text
	}
	return lc, nil
}

// anthemRE matches the canonical anthem text shape: "Creatures you
// control get +N/+M". Variants the regex is intentionally permissive
// about: "Other creatures you control" (Honor of the Pure-style — the
// source excludes itself but the +N/+M still applies to its controller's
// other creatures); buff with optional whitespace; integer-only deltas
// (variable-anthem effects like "+X/+0 where X is the number of cats
// you control" aren't matched — those are 7d set-P/T or scalar count
// dependent and need a richer parse).
//
// NOT matched (deliberately): global pumps that hit every creature
// regardless of controller ("Creatures get +1/+1" — Door of Destinies
// when tribal is set, etc.). Those are rare enough in the MVP card
// pool that we'd add them as a separate pattern with a global-scope
// flag rather than confusing the controller-scoped path.
var anthemRE = regexp.MustCompile(`(?i)(?:other\s+)?creatures\s+you\s+control\s+get\s+\+(\d+)/\+(\d+)`)

// parseAnthemFromText returns the +P/+T delta on the FIRST anthem clause
// in the text. Multi-anthem cards (rare — usually one phase-scoped + one
// permanent) would need a multi-match scan; deferred.
func parseAnthemFromText(text string) (dp, dt int) {
	if text == "" {
		return 0, 0
	}
	m := anthemRE.FindStringSubmatch(text)
	if len(m) < 3 {
		return 0, 0
	}
	dp, _ = strconv.Atoi(m[1])
	dt, _ = strconv.Atoi(m[2])
	return dp, dt
}

// anthemDelta sums all anthem effects active on the controllerSeat's
// battlefield. Returns the combined +P/+T delta to apply to every
// creature that seat controls.
//
// "Other creatures you control" semantics (Honor of the Pure-style):
// the source excludes itself, but we apply the buff to every OTHER
// creature the seat controls. For the MVP we treat all anthem-bearers
// uniformly — the delta applies to the queried creature regardless of
// whether the bearer is "creatures you control" or "OTHER creatures
// you control" — UNLESS the queried creature IS the anthem-bearer (in
// which case the "other" qualifier excludes it). The buffedInstanceID
// arg handles that exclusion.
func anthemDelta(lc *LayerContext, controllerSeat int, buffedInstanceID string) (dp, dt int) {
	if lc == nil {
		return 0, 0
	}
	bf := lc.BattlefieldBySeat[controllerSeat]
	for _, c := range bf {
		if c == nil {
			continue
		}
		text := lc.OracleText[strings.ToLower(strings.TrimSpace(c.Name))]
		ap, at := parseAnthemFromText(text)
		if ap == 0 && at == 0 {
			continue
		}
		if anthemExcludesSelf(text) && c.InstanceID == buffedInstanceID {
			continue
		}
		dp += ap
		dt += at
	}
	return dp, dt
}

// anthemExcludesSelf reports whether the anthem text scopes to "OTHER
// creatures you control" — in which case the bearer doesn't buff itself.
func anthemExcludesSelf(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "other creatures you control get")
}

// combatPower returns the creature's effective power for combat damage
// resolution, applying base + (future counters/modifications) + battlefield
// anthems. nil layer-context degrades to base — the 1/1 fallback still
// applies for missing-P/T cards.
func combatPower(card *Card, lc *LayerContext) int {
	if card == nil {
		return 0
	}
	base := creaturePower(card)
	if lc == nil {
		return base
	}
	dp, _ := anthemDelta(lc, card.OwnerSeat, card.InstanceID)
	out := base + dp
	if out < 0 {
		return 0
	}
	return out
}

// combatToughness mirrors combatPower for toughness — base + anthem delta.
func combatToughness(card *Card, lc *LayerContext) int {
	if card == nil {
		return 0
	}
	base := creatureToughness(card)
	if lc == nil {
		return base
	}
	_, dt := anthemDelta(lc, card.OwnerSeat, card.InstanceID)
	out := base + dt
	if out < 0 {
		return 0
	}
	return out
}
