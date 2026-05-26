package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// FreyaProfile holds the scalar Freya outputs that the snapshot layer
// persists per deck. It is the in-memory mirror of the
// `deck_freya_profile` table (see internal/db/schema.sql). The fields
// are intentionally a strict subset of Freya's full DeckProfile JSON —
// just what downstream analytics (e.g. bracket-vs-ELO correlation in
// docs/bracket-elo-distribution-r60.md) need to join against
// `showmatch_elo` without re-running Freya on the raw deck list.
type FreyaProfile struct {
	DeckKey            string
	Commander          string
	Owner              string
	PrimaryArchetype   string
	SecondaryArchetype string
	Bracket            int
	// SynergyPct is the 0.0-100.0 percentage form of Freya's
	// CommanderSynergy (which it stores as 0.0-1.0). Stored as percent
	// so ad-hoc SQL reads naturally.
	SynergyPct      float64
	PowerPercentile int
	// PowerTierCounts maps the S/A/B/C/D tiers to the number of cards
	// in each. Stable iteration order is the caller's responsibility;
	// the SQL column stores the JSON object verbatim.
	PowerTierCounts map[string]int
	// PrimaryRoles is the top-3 role list sorted by count descending.
	// Mirrors DeckProfile.TopRoles.
	PrimaryRoles []FreyaProfileRole
}

// FreyaProfileRole is one entry in PrimaryRoles. Both fields land in
// the JSON column as {"role": ..., "count": ...} so the SQL shape
// matches the in-memory shape.
type FreyaProfileRole struct {
	Role  string `json:"role"`
	Count int    `json:"count"`
}

// UpsertFreyaProfile writes p into deck_freya_profile. Existing rows
// for the same deck_key are overwritten — Freya re-runs replace the
// prior analysis rather than appending history (the showmatch_elo
// table is the moving rating; deck_freya_profile is "what does this
// deck currently look like").
func UpsertFreyaProfile(ctx context.Context, sqlDB *sql.DB, p FreyaProfile) error {
	if strings.TrimSpace(p.DeckKey) == "" {
		return fmt.Errorf("UpsertFreyaProfile: deck_key is required")
	}
	tierJSON, err := json.Marshal(p.normalizedTierCounts())
	if err != nil {
		return fmt.Errorf("marshal power_tier_counts: %w", err)
	}
	rolesJSON, err := json.Marshal(p.normalizedRoles())
	if err != nil {
		return fmt.Errorf("marshal primary_roles: %w", err)
	}
	_, err = sqlDB.ExecContext(ctx,
		`INSERT INTO deck_freya_profile (
			deck_key, commander, owner,
			primary_archetype, secondary_archetype,
			bracket, synergy_pct, power_percentile,
			power_tier_counts, primary_roles, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, unixepoch())
		 ON CONFLICT(deck_key) DO UPDATE SET
		   commander           = excluded.commander,
		   owner               = excluded.owner,
		   primary_archetype   = excluded.primary_archetype,
		   secondary_archetype = excluded.secondary_archetype,
		   bracket             = excluded.bracket,
		   synergy_pct         = excluded.synergy_pct,
		   power_percentile    = excluded.power_percentile,
		   power_tier_counts   = excluded.power_tier_counts,
		   primary_roles       = excluded.primary_roles,
		   updated_at          = excluded.updated_at`,
		p.DeckKey, p.Commander, p.Owner,
		p.PrimaryArchetype, p.SecondaryArchetype,
		p.Bracket, p.SynergyPct, p.PowerPercentile,
		string(tierJSON), string(rolesJSON))
	return err
}

// LoadFreyaProfile fetches one row by deck_key. Returns sql.ErrNoRows
// verbatim so callers can map missing-profile to a 404.
func LoadFreyaProfile(ctx context.Context, sqlDB *sql.DB, deckKey string) (FreyaProfile, error) {
	var (
		p             FreyaProfile
		tierJSON      string
		rolesJSON     string
	)
	err := sqlDB.QueryRowContext(ctx,
		`SELECT deck_key, commander, owner,
		        primary_archetype, secondary_archetype,
		        bracket, synergy_pct, power_percentile,
		        power_tier_counts, primary_roles
		 FROM deck_freya_profile WHERE deck_key = ?`, deckKey).Scan(
		&p.DeckKey, &p.Commander, &p.Owner,
		&p.PrimaryArchetype, &p.SecondaryArchetype,
		&p.Bracket, &p.SynergyPct, &p.PowerPercentile,
		&tierJSON, &rolesJSON)
	if err != nil {
		return p, err
	}
	if tierJSON != "" {
		_ = json.Unmarshal([]byte(tierJSON), &p.PowerTierCounts)
	}
	if rolesJSON != "" {
		_ = json.Unmarshal([]byte(rolesJSON), &p.PrimaryRoles)
	}
	return p, nil
}

// LoadAllFreyaProfiles returns every row, ordered by updated_at
// descending. The result set is bounded by the deck corpus size
// (~1,300 rows in the r60 snapshot), so no paging is needed for the
// analytics callers that use this.
func LoadAllFreyaProfiles(ctx context.Context, sqlDB *sql.DB) ([]FreyaProfile, error) {
	rows, err := sqlDB.QueryContext(ctx,
		`SELECT deck_key, commander, owner,
		        primary_archetype, secondary_archetype,
		        bracket, synergy_pct, power_percentile,
		        power_tier_counts, primary_roles
		 FROM deck_freya_profile ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FreyaProfile
	for rows.Next() {
		var (
			p         FreyaProfile
			tierJSON  string
			rolesJSON string
		)
		if err := rows.Scan(
			&p.DeckKey, &p.Commander, &p.Owner,
			&p.PrimaryArchetype, &p.SecondaryArchetype,
			&p.Bracket, &p.SynergyPct, &p.PowerPercentile,
			&tierJSON, &rolesJSON); err != nil {
			return nil, err
		}
		if tierJSON != "" {
			_ = json.Unmarshal([]byte(tierJSON), &p.PowerTierCounts)
		}
		if rolesJSON != "" {
			_ = json.Unmarshal([]byte(rolesJSON), &p.PrimaryRoles)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// FreyaProfileFromJSON extracts the scalars we persist from Freya's
// full DeckProfile JSON (the output of `hexdek-freya --deck X --format
// json`). deckKey + owner are caller-supplied because the JSON itself
// doesn't carry the moxfield-style deck_key; the file-system layout
// owns that identity. commander defaults to the JSON's commander field
// but is overridden if caller passes a non-empty commanderOverride.
//
// Returns a partially-populated profile when individual fields are
// missing — Freya routinely omits zero-valued optional fields via
// `omitempty` tags, so the absence of a field is "this deck has none"
// rather than "the JSON is broken". An outright parse error is the
// only hard failure.
func FreyaProfileFromJSON(blob []byte, deckKey, owner, commanderOverride string) (FreyaProfile, error) {
	// Minimal struct mirrors cmd/hexdek-freya/report.go's
	// jsonDeckProfile but pulls only the fields we persist. Decoupled
	// from the freya package so internal/db doesn't take a heavy
	// dependency on the freya CLI for snapshot reads.
	var raw struct {
		Commander          string         `json:"commander"`
		PrimaryArchetype   string         `json:"primary_archetype"`
		SecondaryArchetype string         `json:"secondary_archetype"`
		Bracket            int            `json:"bracket"`
		CommanderSynergy   float64        `json:"commander_synergy"`
		PowerPercentile    int            `json:"power_percentile"`
		PowerTierCounts    map[string]int `json:"power_tier_counts"`
		TopRoles           []struct {
			Role  string `json:"role"`
			Count int    `json:"count"`
		} `json:"top_roles"`
	}
	if err := json.Unmarshal(blob, &raw); err != nil {
		return FreyaProfile{}, fmt.Errorf("parse freya json: %w", err)
	}
	commander := strings.TrimSpace(commanderOverride)
	if commander == "" {
		commander = strings.TrimSpace(raw.Commander)
	}
	roles := make([]FreyaProfileRole, 0, len(raw.TopRoles))
	for _, r := range raw.TopRoles {
		roles = append(roles, FreyaProfileRole{Role: r.Role, Count: r.Count})
	}
	return FreyaProfile{
		DeckKey:            strings.TrimSpace(deckKey),
		Commander:          commander,
		Owner:              strings.TrimSpace(owner),
		PrimaryArchetype:   strings.TrimSpace(raw.PrimaryArchetype),
		SecondaryArchetype: strings.TrimSpace(raw.SecondaryArchetype),
		Bracket:            raw.Bracket,
		// Freya stores CommanderSynergy as 0.0-1.0; the column persists
		// the percentage form so ad-hoc SQL reads naturally. Clamp to
		// [0, 100] defensively in case a future Freya version emits a
		// value outside the canonical range.
		SynergyPct:      clampPct(raw.CommanderSynergy * 100.0),
		PowerPercentile: raw.PowerPercentile,
		PowerTierCounts: raw.PowerTierCounts,
		PrimaryRoles:    roles,
	}, nil
}

func clampPct(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// normalizedTierCounts returns a non-nil map. SQLite's TEXT column
// requires a string and an empty-tier-count deck must still round-trip
// as `{}` rather than the JSON `null` that a nil map serializes to.
func (p FreyaProfile) normalizedTierCounts() map[string]int {
	if p.PowerTierCounts == nil {
		return map[string]int{}
	}
	return p.PowerTierCounts
}

// normalizedRoles returns a non-nil slice for the same reason as
// normalizedTierCounts — `[]` rather than `null` in the column.
func (p FreyaProfile) normalizedRoles() []FreyaProfileRole {
	if p.PrimaryRoles == nil {
		return []FreyaProfileRole{}
	}
	return p.PrimaryRoles
}
