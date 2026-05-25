package heimdall

import (
	"sort"
)

// DeckArchive is the consolidated "everything we know about this
// deck" response for /api/decks/{owner}/{id}/archive. Combines:
//
//   - Identity: owner, deck_id, deck_key, current commander, current
//     archetype (when freya analysis is on file).
//   - Versions: every entry in the deck's version DAG, augmented with
//     per-version win/loss rollup (games bucketed into the version
//     that was HEAD at finished_at — same algorithm as
//     ComputeDeckVersionTrends).
//   - Games: aggregate totals plus the N most-recent games.
//   - ArchetypeEvolution: the timeline of archetype CHANGES across
//     versions, collapsing consecutive same-archetype versions into a
//     single step. Versions with no resolvable archetype are skipped
//     (don't appear as a "since" anchor) so the timeline only
//     records actual transitions.
//
// JSON shape is intentionally non-nil-empty for every slice so the
// frontend can iterate without nil-guards.
type DeckArchive struct {
	Owner              string                     `json:"owner"`
	DeckID             string                     `json:"deck_id"`
	DeckKey            string                     `json:"deck_key"`
	CurrentCommander   string                     `json:"current_commander,omitempty"`
	CurrentArchetype   string                     `json:"current_archetype,omitempty"`
	Versions           []DeckArchiveVersion       `json:"versions"`
	Games              DeckArchiveGames           `json:"games"`
	ArchetypeEvolution []DeckArchiveArchetypeStep `json:"archetype_evolution"`
}

// DeckArchiveVersion is one row of the deck's version history with
// per-version rollup. Archetype is the freya-classified archetype
// for that version, when a snapshot was preserved at the time
// (typically only the current version has this; older versions emit
// "" because historical strategy snapshots aren't persisted today).
type DeckArchiveVersion struct {
	Version   int     `json:"version"`
	Hash      string  `json:"hash,omitempty"`
	CreatedAt int64   `json:"created_at"`
	CardCount int     `json:"card_count"`
	CardDelta int     `json:"card_delta,omitempty"`
	Games     int     `json:"games"`
	Wins      int     `json:"wins"`
	Losses    int     `json:"losses"`
	Draws     int     `json:"draws"`
	WinRate   float64 `json:"win_rate"`
	Archetype string  `json:"archetype,omitempty"`
}

// DeckArchiveGames is the aggregate game-history block. Total /
// Wins / Losses / Draws sum across the deck's entire showmatch
// history. Recent is the trailing N games (default 20) in newest-
// first order.
type DeckArchiveGames struct {
	Total   int                     `json:"total"`
	Wins    int                     `json:"wins"`
	Losses  int                     `json:"losses"`
	Draws   int                     `json:"draws"`
	WinRate float64                 `json:"win_rate"`
	Recent  []DeckArchiveRecentGame `json:"recent"`
}

// DeckArchiveRecentGame is one of the trailing-N games on the
// archive's Recent list. Same shape as DeckVersionOutcome with an
// explicit JSON tag — kept separate so future fields specific to the
// archive surface (commander faced, ELO delta) can land here
// without polluting the version-trends pure-computation type.
type DeckArchiveRecentGame struct {
	FinishedAt int64 `json:"finished_at"`
	Won        bool  `json:"won"`
	Draw       bool  `json:"draw"`
}

// DeckArchiveArchetypeStep records one archetype transition: the
// archetype value, the version it started at, and the unix
// timestamp of that version's creation. The first step always
// records the deck's earliest known archetype; subsequent steps
// only appear when the archetype actually changed.
type DeckArchiveArchetypeStep struct {
	SinceVersion int    `json:"since_version"`
	SinceUnix    int64  `json:"since_unix"`
	Archetype    string `json:"archetype"`
}

// DeckArchiveInput is the assembled bundle of facts a hexapi handler
// hands to BuildDeckArchive. Keeping this as one struct lets the
// pure-computation signature stay small and tests stay readable.
type DeckArchiveInput struct {
	Owner            string
	DeckID           string
	CurrentCommander string
	CurrentArchetype string
	Lineage          []DeckArchiveLineageEntry
	Games            []DeckVersionOutcome
	RecentLimit      int // cap on DeckArchiveGames.Recent; <=0 means default 20
}

// DeckArchiveLineageEntry extends DeckVersionEntry with the
// archetype the deck was classified as at that version. Archetype is
// the empty string when no historical snapshot is available — the
// builder uses CurrentArchetype to fill in the LATEST version's
// archetype as a fallback when its lineage entry has an empty value.
type DeckArchiveLineageEntry struct {
	DeckVersionEntry
	Archetype string
}

const deckArchiveDefaultRecent = 20

// BuildDeckArchive assembles a DeckArchive from the input bundle. It
// reuses ComputeDeckVersionTrends for the per-version rollup so the
// archive's per-version winrate numbers are bit-identical to the
// /api/decks/{owner}/{id}/version-trends endpoint. The function is
// pure; all DB / file IO happens in the calling hexapi handler.
func BuildDeckArchive(in DeckArchiveInput) DeckArchive {
	limit := in.RecentLimit
	if limit <= 0 {
		limit = deckArchiveDefaultRecent
	}

	out := DeckArchive{
		Owner:              in.Owner,
		DeckID:             in.DeckID,
		DeckKey:            in.Owner + "/" + in.DeckID,
		CurrentCommander:   in.CurrentCommander,
		CurrentArchetype:   in.CurrentArchetype,
		Versions:           []DeckArchiveVersion{},
		ArchetypeEvolution: []DeckArchiveArchetypeStep{},
		Games: DeckArchiveGames{
			Recent: []DeckArchiveRecentGame{},
		},
	}

	// Aggregate game totals across the full history.
	for _, g := range in.Games {
		out.Games.Total++
		switch {
		case g.Won:
			out.Games.Wins++
		case g.Draw:
			out.Games.Draws++
		default:
			out.Games.Losses++
		}
	}
	if out.Games.Total > 0 {
		out.Games.WinRate = float64(out.Games.Wins) / float64(out.Games.Total)
	}

	// Recent games: sort by FinishedAt desc, take first N.
	sortedGames := make([]DeckVersionOutcome, len(in.Games))
	copy(sortedGames, in.Games)
	sort.SliceStable(sortedGames, func(i, j int) bool {
		return sortedGames[i].FinishedAt > sortedGames[j].FinishedAt
	})
	for i := 0; i < len(sortedGames) && i < limit; i++ {
		g := sortedGames[i]
		out.Games.Recent = append(out.Games.Recent, DeckArchiveRecentGame{
			FinishedAt: g.FinishedAt,
			Won:        g.Won,
			Draw:       g.Draw,
		})
	}

	// Per-version rollup via the existing pure-computation path. We
	// stripe lineage → DeckVersionEntry first, then re-attach per-
	// version archetype onto the winrate rows.
	lineageBase := make([]DeckVersionEntry, len(in.Lineage))
	archByVersion := make(map[int]string, len(in.Lineage))
	for i, e := range in.Lineage {
		lineageBase[i] = e.DeckVersionEntry
		if e.Archetype != "" {
			archByVersion[e.Version] = e.Archetype
		}
	}
	trends := ComputeDeckVersionTrends(in.Owner, in.DeckID, lineageBase, in.Games)

	// Determine the latest known version so we can fall back to
	// CurrentArchetype for it when no historical snapshot was saved.
	latestVersion := -1
	for _, t := range trends.Versions {
		if t.Version > latestVersion {
			latestVersion = t.Version
		}
	}

	out.Versions = make([]DeckArchiveVersion, 0, len(trends.Versions))
	for _, v := range trends.Versions {
		arch := archByVersion[v.Version]
		if arch == "" && v.Version == latestVersion && in.CurrentArchetype != "" {
			arch = in.CurrentArchetype
		}
		out.Versions = append(out.Versions, DeckArchiveVersion{
			Version:   v.Version,
			Hash:      v.Hash,
			CreatedAt: v.CreatedAt,
			CardCount: v.CardCount,
			CardDelta: v.CardDelta,
			Games:     v.Games,
			Wins:      v.Wins,
			Losses:    v.Losses,
			Draws:     v.Draws,
			WinRate:   v.WinRate,
			Archetype: arch,
		})
	}

	// Archetype evolution: walk versions in chronological order
	// (Version asc — same order ComputeDeckVersionTrends returns)
	// and emit a step whenever the archetype changes vs the previous
	// step. Versions with empty archetype don't anchor a step but
	// don't terminate the current step either.
	prevArch := ""
	for _, v := range out.Versions {
		if v.Archetype == "" {
			continue
		}
		if v.Archetype == prevArch {
			continue
		}
		out.ArchetypeEvolution = append(out.ArchetypeEvolution, DeckArchiveArchetypeStep{
			SinceVersion: v.Version,
			SinceUnix:    v.CreatedAt,
			Archetype:    v.Archetype,
		})
		prevArch = v.Archetype
	}

	return out
}
