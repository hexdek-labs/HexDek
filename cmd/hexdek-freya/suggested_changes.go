package main

import (
	"fmt"
	"sort"
	"strings"
)

// SuggestedChanges is the structured output of BuildSuggestedChanges
// — a prioritized list of specific add / cut / swap recommendations
// for a deck. Complementary to the existing CoachingTip system
// (cmd/hexdek-freya/coaching.go): coaching tips are human-prose
// category-level advice ("add 2 more interaction spells"); this
// struct surfaces the SAME analysis as machine-readable add/cut
// pairs with specific card names, so the Decks UI can render a
// "click to add" interaction and so external tooling (Moxfield
// auto-suggest, party-mode deck-upgrade flow) can consume the
// recommendations as data, not prose.
//
// Output prioritization (1-10 scale):
//
//	10  — critical mana base fixes (land count under floor, F-grade)
//	 9  — D-grade mana base + count-based interaction gaps below
//	      bracket floor
//	 8  — counterspell / wipe additions for archetypes that need
//	      them (control bracket-4+, combo any-bracket)
//	 7  — draw / ramp gap fills; mana base tap-land swap suggestions
//	 6  — protection adds for combo decks
//	 5  — cuttable-tier card removals (from existing CuttableCards
//	      analysis)
//	 4  — off-archetype card flags
//	 3  — flavor / synergy nudges
//
// Caller can sort or filter by Priority — the struct preserves the
// raw priority so UIs can render "top 3 changes" vs "all
// changes" without re-deriving.
type SuggestedChanges struct {
	// Adds: cards to add to the deck, grouped by category. Sorted
	// descending by Priority. Excludes cards already in the deck
	// (owned-card filter), filtered by deck color identity.
	Adds []SuggestedAdd

	// Cuts: cards already in the deck that Freya recommends
	// removing. Sourced from dp.CuttableCards (the synergy-tier
	// cuttable list). Sorted descending by Priority (= 100 - Power,
	// so D-tier cuts rank above C-tier).
	Cuts []SuggestedCut

	// Swaps: paired add+cut recommendations where the gap can be
	// closed by a specific 1-for-1 swap (most common: tap-land
	// upgrades like Tranquil Cove -> Bountiful Promenade). Sorted
	// descending by Priority.
	Swaps []SuggestedSwap

	// Summary: 1-2 sentence overview of the change set
	// ("3 adds, 2 cuts, 1 swap suggested. Highest priority: add
	// Counterspell — current 4 vs target 6 for Control archetype.")
	Summary string

	// TotalChanges is len(Adds) + len(Cuts) + len(Swaps). Surfaced
	// so UIs can short-circuit ("no suggestions") without iterating.
	TotalChanges int
}

// SuggestedAdd is one card recommendation. CardName references the
// buyGuideStaples catalog (cmd/hexdek-freya/buy_guide.go).
type SuggestedAdd struct {
	CardName string
	Category string // "interaction" / "ramp" / "draw" / "wipes" / "tutors" / "manabase"
	Priority int    // 1-10
	Reason   string // "current 4 counterspells, target 6 for Control archetype"
	GapSize  int    // current vs target gap that motivated this add
}

// SuggestedCut is one card to remove. CardName comes from the
// deck's existing CuttableCards (or off-archetype creature list).
type SuggestedCut struct {
	CardName string
	Category string // "cuttable" / "off_archetype" / "redundant_tutor"
	Priority int    // 1-10
	Reason   string
}

// SuggestedSwap pairs a specific add with a specific cut — the
// canonical "swap X for Y" recommendation. Most useful for the
// mana base case where "upgrade your tap-lands" is more actionable
// when the upgrade names a specific replacement.
type SuggestedSwap struct {
	AddCardName string
	CutCardName string
	Category    string // "manabase" / "interaction_upgrade" / "ramp_upgrade"
	Priority    int    // 1-10
	Reason      string // "Swap Tranquil Cove for Bountiful Promenade — same WU identity, enters untapped after early game"
}

// taplandUpgradeMap is the curated tap-land → untapped-dual upgrade
// table. Keyed by lowercased tap-land name; value is the
// recommended replacement (typically a Battlebond duo land —
// untapped if you have an opponent, which is always true in
// 4-player Commander). All pairs are color-identity-preserving so
// the caller doesn't need to re-check colors.
//
// Selection criteria: tap-lands that show up in WotC precons + the
// canonical "obvious upgrade" pair from the budget-EDH community.
// Curated to ~20 entries — not exhaustive, but the most-common
// precon offenders are covered.
var taplandUpgradeMap = map[string]string{
	// WU
	"tranquil cove":             "Bountiful Promenade",
	"azorius guildgate":         "Hallowed Fountain",
	"meandering river":          "Mystic Gate",
	// UB
	"dismal backwater":          "Morphic Pool",
	"dimir guildgate":           "Watery Grave",
	"submerged boneyard":        "Sunken Hollow",
	// BR
	"bloodfell caves":           "Luxury Suite",
	"rakdos guildgate":          "Blood Crypt",
	"foreboding ruins":          "Blood Crypt",
	// RG
	"rugged highlands":          "Spire Garden",
	"gruul guildgate":           "Stomping Ground",
	"timber gorge":              "Cinder Glade",
	// GW
	"blossoming sands":          "Bountiful Promenade",
	"selesnya guildgate":        "Temple Garden",
	"tranquil expanse":          "Temple Garden",
	// WB
	"scoured barrens":           "Shineshadow Snarl",
	"orzhov guildgate":          "Godless Shrine",
	"forsaken sanctuary":        "Concealed Courtyard",
	// UR
	"swiftwater cliffs":         "Training Center",
	"izzet guildgate":           "Steam Vents",
	"highland lake":             "Spirebluff Canal",
	// BG
	"jungle hollow":             "Undergrowth Stadium",
	"golgari guildgate":         "Overgrown Tomb",
	"woodland cemetery":         "Overgrown Tomb",
	// RW
	"wind-scarred crag":         "Sacred Foundry",
	"boros guildgate":           "Sacred Foundry",
	"stone quarry":              "Inspiring Vantage",
	// UG
	"thornwood falls":           "Hinterland Harbor",
	"simic guildgate":           "Breeding Pool",
	"woodland stream":           "Yavimaya Coast",
}

// BuildSuggestedChanges produces a SuggestedChanges rollup for a
// deck. Reads existing analysis (bracket-scaled minimum gaps,
// CuttableCards, mana base grade) and pairs them with specific
// card recommendations from buyGuideStaples + taplandUpgradeMap.
//
// Returns a non-nil zero SuggestedChanges{} on nil inputs rather
// than nil — the UI can always render the Summary (which will say
// "no suggestions") without nil-checking. TotalChanges = 0 is the
// "no changes" signal.
func BuildSuggestedChanges(dp *DeckProfile, report *FreyaReport) *SuggestedChanges {
	out := &SuggestedChanges{}
	if dp == nil || report == nil {
		out.Summary = "Insufficient analysis data — no suggestions available."
		return out
	}

	bracket := dp.Bracket
	if bracket == 0 {
		bracket = 3
	}
	arch := strings.ToLower(dp.PrimaryArchetype)

	// Build the owned-cards set so adds don't recommend what's
	// already there.
	owned := map[string]bool{}
	for _, p := range report.Profiles {
		owned[strings.ToLower(p.Name)] = true
	}

	// ── Interaction gap → counterspell + removal adds ──
	interactionCount := report.RemovalCount
	if report.Roles != nil {
		interactionCount += report.Roles.RoleCounts[RoleCounterspell]
	}
	interactionTarget := interactionTargetFor(bracket, arch)
	if interactionCount < interactionTarget {
		gap := interactionTarget - interactionCount
		archLabel := dp.PrimaryArchetype
		if archLabel == "" {
			archLabel = fmt.Sprintf("bracket-%d", bracket)
		}
		reason := fmt.Sprintf("current %d interaction spells, target %d for %s archetype",
			interactionCount, interactionTarget, archLabel)
		out.Adds = append(out.Adds,
			pickStapleAdds("interaction", dp.ColorIdentity, owned, gap, 8, reason)...)
	}

	// ── No wipes → add a wipe ──
	if report.Roles != nil && report.Roles.RoleCounts[RoleBoardWipe] == 0 {
		prio := 7
		if bracket >= 4 {
			prio = 8
		}
		out.Adds = append(out.Adds,
			pickStapleAdds("wipes", dp.ColorIdentity, owned, 1, prio,
				"zero board wipes — table can't recover from a runaway opponent")...)
	}

	// ── Draw gap ──
	drawTarget := drawTargetFor(bracket)
	if dp.DrawCount < drawTarget {
		gap := drawTarget - dp.DrawCount
		reason := fmt.Sprintf("current %d draw sources, target %d for bracket-%d",
			dp.DrawCount, drawTarget, bracket)
		out.Adds = append(out.Adds,
			pickStapleAdds("draw", dp.ColorIdentity, owned, gap, 7, reason)...)
	}

	// ── Ramp gap ──
	rampTarget := rampTargetFor(bracket)
	if dp.RampCount < rampTarget {
		gap := rampTarget - dp.RampCount
		reason := fmt.Sprintf("current %d ramp pieces, target %d for bracket-%d",
			dp.RampCount, rampTarget, bracket)
		out.Adds = append(out.Adds,
			pickStapleAdds("ramp", dp.ColorIdentity, owned, gap, 7, reason)...)
	}

	// ── Tutor gap (combo / storm bracket 4+) ──
	if (strings.Contains(arch, "combo") || strings.Contains(arch, "storm")) && bracket >= 4 {
		tutorTarget := 5
		if bracket >= 5 {
			tutorTarget = 8
		}
		if report.NonLandTutorCount < tutorTarget {
			gap := tutorTarget - report.NonLandTutorCount
			reason := fmt.Sprintf("current %d nonland tutors, target %d for %s at bracket-%d",
				report.NonLandTutorCount, tutorTarget, dp.PrimaryArchetype, bracket)
			out.Adds = append(out.Adds,
				pickStapleAdds("tutors", dp.ColorIdentity, owned, gap, 7, reason)...)
		}
	}

	// ── Tap-land swaps (mana base C/D/F-grade only) ──
	if dp.ManaBaseGrade == "C" || dp.ManaBaseGrade == "D" || dp.ManaBaseGrade == "F" {
		swapPrio := 7
		if dp.ManaBaseGrade == "F" {
			swapPrio = 10
		} else if dp.ManaBaseGrade == "D" {
			swapPrio = 9
		}
		for _, p := range report.Profiles {
			lc := strings.ToLower(p.Name)
			if upgrade, ok := taplandUpgradeMap[lc]; ok {
				if owned[strings.ToLower(upgrade)] {
					continue
				}
				out.Swaps = append(out.Swaps, SuggestedSwap{
					AddCardName: upgrade,
					CutCardName: p.Name,
					Category:    "manabase",
					Priority:    swapPrio,
					Reason: fmt.Sprintf(
						"Mana base %s-grade — swap %s for %s (color-identity preserving, enters untapped in multiplayer)",
						dp.ManaBaseGrade, p.Name, upgrade),
				})
			}
		}
	}

	// ── Cuts: sourced from existing CuttableCards analysis ──
	for _, cc := range dp.CuttableCards {
		prio := 5
		// Map the existing power-tier to suggested-cut priority.
		// D-tier cuts above C-tier; high-priority for dead slots.
		switch cc.PowerTier {
		case "D":
			prio = 5
		case "C":
			prio = 4
		}
		reason := cc.Reason
		if reason == "" {
			reason = "Synergy tier classified this as cuttable"
		}
		out.Cuts = append(out.Cuts, SuggestedCut{
			CardName: cc.Name,
			Category: "cuttable",
			Priority: prio,
			Reason:   reason,
		})
	}

	// Sort all three slices descending by Priority. Stable so ties
	// preserve insertion order (insertion order tracks the gap-
	// detection order in BuildSuggestedChanges).
	sort.SliceStable(out.Adds, func(i, j int) bool {
		return out.Adds[i].Priority > out.Adds[j].Priority
	})
	sort.SliceStable(out.Cuts, func(i, j int) bool {
		return out.Cuts[i].Priority > out.Cuts[j].Priority
	})
	sort.SliceStable(out.Swaps, func(i, j int) bool {
		return out.Swaps[i].Priority > out.Swaps[j].Priority
	})

	out.TotalChanges = len(out.Adds) + len(out.Cuts) + len(out.Swaps)
	out.Summary = buildChangesSummary(out)
	return out
}

// pickStapleAdds picks up to `gap` cards from the named buy-guide
// staple category, filtered by color identity and excluding cards
// already in the deck. Returns SuggestedAdd entries with the shared
// reason + priority. The caller's responsibility to compute the
// gap; this function just renders the picks.
//
// Sort order on the returned slice matches the buyGuideStaples
// catalog order — which is hand-curated by BaselineImpact descending,
// so the highest-impact card available to the deck's color identity
// comes first.
func pickStapleAdds(category string, colorIdentity []string, owned map[string]bool, gap int, priority int, reason string) []SuggestedAdd {
	if gap <= 0 {
		return nil
	}
	staples, ok := buyGuideStaples[category]
	if !ok {
		return nil
	}
	// Sort by BaselineImpact descending so the strongest picks
	// come first (the catalog stores them in this order already
	// but be defensive in case of future edits).
	picks := make([]stapleEntry, len(staples))
	copy(picks, staples)
	sort.SliceStable(picks, func(i, j int) bool {
		return picks[i].BaselineImpact > picks[j].BaselineImpact
	})

	var out []SuggestedAdd
	for _, s := range picks {
		if len(out) >= gap {
			break
		}
		if owned[strings.ToLower(s.Name)] {
			continue
		}
		if !colorIdentityAllows(colorIdentity, s.Colors) {
			continue
		}
		out = append(out, SuggestedAdd{
			CardName: s.Name,
			Category: category,
			Priority: priority,
			Reason:   reason,
			GapSize:  gap,
		})
	}
	return out
}

// interactionTargetFor returns the bracket+archetype-scaled
// interaction floor. Mirrors the threshold logic in
// computeCoachingTips with an extra Control-archetype lift (Control
// decks want more interaction than the bracket-floor implies — the
// archetype IS interaction).
func interactionTargetFor(bracket int, arch string) int {
	target := 8
	switch {
	case bracket >= 5:
		target = 12
	case bracket >= 4:
		target = 10
	}
	if strings.Contains(arch, "control") && target < 12 {
		target = 12
	}
	return target
}

// drawTargetFor scales draw count by bracket. Matches the floor
// used by computeCoachingTips.
func drawTargetFor(bracket int) int {
	switch {
	case bracket >= 5:
		return 11
	case bracket >= 4:
		return 9
	default:
		return 7
	}
}

// rampTargetFor scales ramp count by bracket. Matches the floor
// used by computeCoachingTips.
func rampTargetFor(bracket int) int {
	if bracket >= 4 {
		return 10
	}
	return 8
}

// buildChangesSummary renders the 1-2 sentence overview. Format:
//
//	"3 adds, 2 cuts, 1 swap suggested. Highest priority: add
//	Counterspell (current 4 vs target 6 for Control archetype)."
func buildChangesSummary(s *SuggestedChanges) string {
	if s == nil || s.TotalChanges == 0 {
		return "Deck looks well-tuned — no specific changes suggested."
	}
	count := fmt.Sprintf("%d add%s, %d cut%s, %d swap%s suggested.",
		len(s.Adds), plural(len(s.Adds)),
		len(s.Cuts), plural(len(s.Cuts)),
		len(s.Swaps), plural(len(s.Swaps)))

	// Identify the single highest-priority recommendation across
	// all three slices for the "highest priority" callout.
	type prioritized struct {
		kind     string
		priority int
		text     string
	}
	var candidates []prioritized
	if len(s.Adds) > 0 {
		candidates = append(candidates, prioritized{
			kind: "add", priority: s.Adds[0].Priority,
			text: fmt.Sprintf("add %s (%s)", s.Adds[0].CardName, s.Adds[0].Reason),
		})
	}
	if len(s.Cuts) > 0 {
		candidates = append(candidates, prioritized{
			kind: "cut", priority: s.Cuts[0].Priority,
			text: fmt.Sprintf("cut %s (%s)", s.Cuts[0].CardName, s.Cuts[0].Reason),
		})
	}
	if len(s.Swaps) > 0 {
		candidates = append(candidates, prioritized{
			kind: "swap", priority: s.Swaps[0].Priority,
			text: fmt.Sprintf("swap %s for %s", s.Swaps[0].CutCardName, s.Swaps[0].AddCardName),
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].priority > candidates[j].priority
	})
	if len(candidates) > 0 {
		return count + " Highest priority: " + candidates[0].text + "."
	}
	return count
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
