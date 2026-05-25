package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

// ---------------------------------------------------------------------------
// Structured cluster export — designed for downstream deck-builder
// integrations (Moxfield bridges, Discord-bot recommenders, hat
// strategy bridges) that need richer information than the
// display-oriented jsonCluster shape carries.
//
// Differences from the existing jsonCluster:
//
//   - members[] carries the FULL deduped list (not capped at 8) and
//     each entry includes its per-card role classification
//     ("producer" | "amplifier" | "payoff" | "both" | "unknown").
//     A deck-builder integration uses this to surface "your tokens
//     cluster has 6 producers and 0 payoffs — add Purphoros."
//
//   - score_breakdown decomposes the opaque integer Score into
//     pair_score + chain_bonus + the underlying role tallies, so
//     downstream tools can render "the +9 chain bonus comes from
//     3 complete producer→amplifier→payoff triangles."
//
//   - chain_complete + has_role_dichotomy expose structural facts
//     (the cluster has at least one full chain; the theme actually
//     has a producer/payoff dichotomy vs being a flat-count theme).
//
//   - schema_version pins the format so downstream parsers can
//     version-check rather than guess.
// ---------------------------------------------------------------------------

// ClusterExportSchemaVersion is bumped on any breaking change to the
// JSON shape below. Consumers should reject blobs whose version they
// don't recognize.
const ClusterExportSchemaVersion = "1.0"

// SynergyClusterExport is the top-level export blob, suitable for
// writing as a standalone .json file via the --clusters-out CLI flag
// or embedding inside the full Freya report.
type SynergyClusterExport struct {
	SchemaVersion string         `json:"schema_version"`
	DeckName      string         `json:"deck_name"`
	Commander     string         `json:"commander,omitempty"`
	GeneratedAt   string         `json:"generated_at"` // RFC3339
	Clusters      []ClusterEntry `json:"clusters"`
}

// ClusterEntry is one synergy cluster in the export blob.
type ClusterEntry struct {
	Theme            string                `json:"theme"`     // key (e.g. "tokens")
	Name             string                `json:"name"`      // display ("Token Engine")
	MemberCount      int                   `json:"member_count"`
	Score            int                   `json:"score"`
	HasRoleDichotomy bool                  `json:"has_role_dichotomy"` // false for landfall/spellcast/lifegain
	ChainComplete    bool                  `json:"chain_complete"`     // ≥1 P→A→Pf triangle present
	ScoreBreakdown   ClusterScoreBreakdown `json:"score_breakdown"`
	Members          []ClusterMember       `json:"members"`
}

// ClusterScoreBreakdown decomposes the opaque integer Score into its
// pair-score + chain-bonus components plus the underlying role
// tallies that drove them.
type ClusterScoreBreakdown struct {
	PairScore      int `json:"pair_score"`      // raw pairwise sum (before chain bonus)
	ChainBonus     int `json:"chain_bonus"`     // chain_count × 3
	ProducerCount  int `json:"producer_count"`  // counts include Both as both
	AmplifierCount int `json:"amplifier_count"`
	PayoffCount    int `json:"payoff_count"`
	UnknownCount   int `json:"unknown_count"`
	ChainCount     int `json:"chain_count"` // min(producers, amplifiers, payoffs)
}

// ClusterMember is one card's appearance in a cluster, with its
// per-theme role classification.
type ClusterMember struct {
	Name string `json:"name"`
	Role string `json:"role"` // "producer" | "amplifier" | "payoff" | "both" | "unknown"
}

// BuildClusterExport produces the rich structured export from a
// completed deck analysis. Re-derives per-card roles from the
// existing classifyClusterRole helper so we don't have to plumb
// extra state onto SynergyCluster at compute time.
//
// Returns nil if dp is nil or has no clusters. Always emits the
// schema_version + generated_at metadata even when clusters is empty
// (so downstream consumers can detect "this deck has no clusters"
// vs "the export ran but found nothing").
func BuildClusterExport(dp *DeckProfile, report *FreyaReport) *SynergyClusterExport {
	if dp == nil {
		return nil
	}
	export := &SynergyClusterExport{
		SchemaVersion: ClusterExportSchemaVersion,
		DeckName:      dp.DeckName,
		Commander:     dp.Commander,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Clusters:      []ClusterEntry{},
	}
	if len(dp.SynergyClusters) == 0 {
		return export
	}

	// Index nonland profiles by name for per-card role lookup. We need
	// the original CardProfile (with Triggers / Produces / IsOutlet /
	// IsBlinker / HasValueETB) to call classifyClusterRole.
	profileByName := map[string]CardProfile{}
	if report != nil {
		for _, p := range report.Profiles {
			if p.IsLand {
				continue
			}
			profileByName[p.Name] = p
		}
	}

	for _, sc := range dp.SynergyClusters {
		entry := buildClusterEntry(sc, profileByName)
		export.Clusters = append(export.Clusters, entry)
	}
	return export
}

// buildClusterEntry handles one cluster: per-card role classification,
// pair-score + chain-bonus decomposition, structural facts.
func buildClusterEntry(sc SynergyCluster, profileByName map[string]CardProfile) ClusterEntry {
	// Source the full member list. Older callers may still hand us a
	// cluster with only Cards populated; fall back to that if AllMembers
	// is empty so the export degrades gracefully rather than reporting
	// zero members.
	memberNames := sc.AllMembers
	if len(memberNames) == 0 {
		memberNames = sc.Cards
	}

	members := make([]ClusterMember, 0, len(memberNames))
	roles := make([]clusterRole, 0, len(memberNames))
	hasDichotomy := false
	for _, name := range memberNames {
		role := clusterRoleUnknown
		if p, ok := profileByName[name]; ok {
			role = classifyClusterRole(p, sc.Theme)
		}
		if role != clusterRoleUnknown {
			hasDichotomy = true
		}
		roles = append(roles, role)
		members = append(members, ClusterMember{
			Name: name,
			Role: roleLabel(role),
		})
	}
	// Stable member order: alphabetical by name. The compute step
	// preserves theme-tagging order, but downstream consumers diffing
	// two cluster exports across runs want stability.
	sort.Slice(members, func(i, j int) bool {
		return members[i].Name < members[j].Name
	})

	bd := buildScoreBreakdown(roles)
	chainComplete := bd.ChainCount > 0

	return ClusterEntry{
		Theme:            sc.Theme,
		Name:             sc.Name,
		MemberCount:      sc.MemberCount,
		Score:            sc.Score,
		HasRoleDichotomy: hasDichotomy,
		ChainComplete:    chainComplete,
		ScoreBreakdown:   bd,
		Members:          members,
	}
}

// buildScoreBreakdown re-derives the pair-score + chain-bonus
// decomposition using the same algorithm as computeSynergyClusters.
// Kept in sync with the canonical scoring logic in advanced.go — if
// that algorithm changes, this needs the matching update.
func buildScoreBreakdown(roles []clusterRole) ClusterScoreBreakdown {
	var bd ClusterScoreBreakdown
	for _, r := range roles {
		switch r {
		case clusterRoleProducer:
			bd.ProducerCount++
		case clusterRoleAmplifier:
			bd.AmplifierCount++
		case clusterRolePayoff:
			bd.PayoffCount++
		case clusterRoleBoth:
			// Both counts as one producer AND one payoff for chain math
			// (matches computeSynergyClusters).
			bd.ProducerCount++
			bd.PayoffCount++
		case clusterRoleUnknown:
			bd.UnknownCount++
		}
	}
	// Pair-score: sum of rolesPairScore over every unordered pair.
	for i := 0; i < len(roles); i++ {
		for j := i + 1; j < len(roles); j++ {
			bd.PairScore += rolesPairScore(roles[i], roles[j])
		}
	}
	// Chain-bonus: min(producers, amplifiers, payoffs) × 3.
	bd.ChainCount = bd.ProducerCount
	if bd.AmplifierCount < bd.ChainCount {
		bd.ChainCount = bd.AmplifierCount
	}
	if bd.PayoffCount < bd.ChainCount {
		bd.ChainCount = bd.PayoffCount
	}
	bd.ChainBonus = bd.ChainCount * 3
	return bd
}

// roleLabel maps the internal clusterRole enum to the export's
// string vocabulary. Stable across schema versions — adding a new
// role would be a breaking change requiring a schema bump.
func roleLabel(r clusterRole) string {
	switch r {
	case clusterRoleProducer:
		return "producer"
	case clusterRoleAmplifier:
		return "amplifier"
	case clusterRolePayoff:
		return "payoff"
	case clusterRoleBoth:
		return "both"
	default:
		return "unknown"
	}
}

// WriteClusterExportJSON serializes the export to a file with
// pretty-printed indentation. Used by the --clusters-out CLI flag.
func WriteClusterExportJSON(export *SynergyClusterExport, path string) error {
	if export == nil {
		return fmt.Errorf("nil export")
	}
	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cluster export: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
