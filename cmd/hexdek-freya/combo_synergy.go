package main

import (
	"fmt"
	"strings"
)

// Phase 3 — SYNERGY axis (commander-relative) + the Layer-B context re-tag.
//
// SYNERGY asks "does this card advance THIS commander's plan?" — it is the only
// commander-RELATIVE axis (COMBO and VALUE are deck-independent baselines). The
// fit score aggregates the signals Freya already computes, adapted to the data
// available at this layer (the deck's CardProfiles, including the commander's),
// so it works both inside AnalyzeDeck and on the full CLI path:
//
//   - commander themes derived from the commander's profile (sacrifice / tokens
//     / counters / graveyard / spellcast / lifegain / landfall / blink / tribal)
//   - per-card theme advancement (does this card feed a commander theme)
//   - tribal lord / payoff role
//   - symmetric table-wide effects (group sacrifice / group hug) — these are
//     synergy levers, NOT value (Phase 2 excluded them precisely so SYNERGY
//     can claim them here)
//
// Per Q3 the weights/thresholds are DERIVED (simple, legible) rather than fit by
// an elaborate calibration system.
//
// CONTEXT RE-TAG (Layer B, §5 — the capstone): a deck-independent VALUE baseline
// (Phase 2) gains a +SYNERGY tag when, in THIS shell, it advances the plan. The
// baseline never changes; only Layer B adds tags. VALUE and SYNERGY co-tag (an
// entry can be in both ValueEngines and SynergyPackages). COMBO stays its own
// axis (an enabler feeding a math payoff was already promoted to COMBO in P1).

// Derived synergy weights / threshold (Q3).
const (
	synergyThemeWeight     = 0.5 // a card feeding one commander theme
	synergyLordWeight      = 0.5 // a tribal lord / payoff
	synergySymmetricWeight = 0.3 // symmetric effect that also supports a theme
	synergyThreshold       = 0.5 // fit at/above this => synergy
)

// commanderThemesFromProfile derives the commander's plan themes from its
// CardProfile (the data available at this layer; the full oracle-text theme
// extraction in computeCommanderSynergy runs later and is unaffected). Returns
// nil for a themeless / goodstuff commander.
func commanderThemesFromProfile(c CardProfile) []string {
	var themes []string
	has := func(t string) bool { return profileHasTrigger(c, t) }
	if c.IsOutlet || has("dies") || has("sacrifice") || c.HasDeathDrain {
		themes = append(themes, "sacrifice")
	}
	if containsRes(c.Produces, ResToken) || c.IsMassTokens || c.MakesInfiniteTokens || has("token_created") {
		themes = append(themes, "tokens")
	}
	if has("counter_matters") || has("counter_placed") || c.CounterToDamage || c.DamageToCounter {
		themes = append(themes, "counters")
	}
	if c.IsRecursion || containsRes(c.Produces, ResGraveyard) ||
		containsRes(c.Produces, ResReanimate) || containsRes(c.Produces, ResGraveyardFill) {
		themes = append(themes, "graveyard")
	}
	if has("cast") {
		themes = append(themes, "spellcast")
	}
	if has("lifegain") || c.LifegainToDrain {
		themes = append(themes, "lifegain")
	}
	if has("landfall") {
		themes = append(themes, "landfall")
	}
	if c.IsBlinker {
		themes = append(themes, "blink")
	}
	// Tribal only when the commander is genuinely a tribal lord/payoff — a
	// commander merely BEING a creature with a subtype does not make the deck
	// tribal (that spuriously matches every creature in the 99).
	if c.IsTribalLord || c.IsTribalPayoff {
		themes = append(themes, "tribal")
	}
	return themes
}

// deckThemes returns the commander's plan themes for a report, found by looking
// up report.Commander in the deck's profiles. Empty when there is no commander
// or it carries no theme signal (goodstuff).
func deckThemes(report *FreyaReport, profByName map[string]CardProfile) []string {
	if report.Commander == "" {
		return nil
	}
	if c, ok := profByName[report.Commander]; ok {
		return commanderThemesFromProfile(c)
	}
	return nil
}

// cardAdvancesTheme reports whether a card feeds a given commander theme.
func cardAdvancesTheme(p CardProfile, theme string) bool {
	has := func(t string) bool { return profileHasTrigger(p, t) }
	switch theme {
	case "sacrifice":
		return p.IsOutlet || has("dies") || has("sacrifice") || p.HasDeathDrain
	case "tokens":
		return containsRes(p.Produces, ResToken) || p.IsMassTokens || p.MakesInfiniteTokens
	case "counters":
		return has("counter_matters") || has("counter_placed") ||
			p.CounterToDamage || p.DamageToCounter || containsRes(p.Produces, ResCounter)
	case "graveyard":
		return p.IsRecursion || containsRes(p.Produces, ResGraveyard) ||
			containsRes(p.Produces, ResReanimate) || containsRes(p.Produces, ResGraveyardFill)
	case "spellcast":
		tl := strings.ToLower(p.TypeLine)
		return strings.Contains(tl, "instant") || strings.Contains(tl, "sorcery") || has("cast")
	case "lifegain":
		return containsRes(p.Produces, ResLife) || has("lifegain") || p.LifegainToDrain
	case "landfall":
		return p.IsLand || p.IsLandTutor || has("landfall")
	case "blink":
		return p.IsBlinker || p.HasValueETB
	case "tribal":
		return p.IsTribalLord || p.IsTribalPayoff || len(p.CreatureTypes) > 0
	}
	return false
}

// computeSynergyFit scores how well a card advances the commander's plan.
func computeSynergyFit(p CardProfile, themes []string) SynergyFit {
	score := 0.0
	var matched []string
	for _, t := range themes {
		if cardAdvancesTheme(p, t) {
			score += synergyThemeWeight
			matched = append(matched, t)
		}
	}
	role := "support"
	if p.IsTribalLord || p.IsTribalPayoff {
		score += synergyLordWeight
		role = "enabler"
	} else if p.IsOutlet || p.IsTutor || p.IsRecursion {
		role = "enabler"
	}
	if p.Symmetric && len(matched) > 0 {
		score += synergySymmetricWeight
	}
	if score > 1.0 {
		score = 1.0
	}
	return SynergyFit{Score: score, Themes: matched, Role: role, Symmetric: p.Symmetric}
}

// synergyQualifies decides whether a card carries the SYNERGY tag. A symmetric
// table-wide effect or a tribal lord/payoff always qualifies (they are
// definitionally synergy pieces); otherwise the fit score must clear the
// threshold (i.e. it advances at least one commander theme).
func synergyQualifies(fit SynergyFit, p CardProfile) bool {
	return fit.Score >= synergyThreshold || p.Symmetric || p.IsTribalLord || p.IsTribalPayoff
}

// populateSynergyPackages fills report.SynergyPackages and performs the Layer-B
// context re-tag (co-tagging VALUE entries that advance the plan with +SYNERGY).
// Idempotent; called from CategorizeReportCombos after the COMBO and VALUE
// passes. comboKeys / valueKeys are the canonical card-key sets already in
// report.Combos / report.ValueEngines.
func populateSynergyPackages(report *FreyaReport, comboKeys, valueKeys map[string]bool, profByName map[string]CardProfile) {
	report.SynergyPackages = nil

	themes := deckThemes(report, profByName)

	// Per-card synergy fit.
	fitByName := map[string]SynergyFit{}
	for _, p := range report.Profiles {
		fit := computeSynergyFit(p, themes)
		if synergyQualifies(fit, p) {
			fitByName[p.Name] = fit
		}
	}
	anyCardSynergy := func(names []string) bool {
		for _, n := range names {
			if _, ok := fitByName[n]; ok {
				return true
			}
		}
		return false
	}

	seen := map[string]bool{}
	add := func(c ComboResult, cats []string) {
		key := comboCardKey(c.Cards)
		if seen[key] {
			return
		}
		seen[key] = true
		c.Categories = cats
		c.Annotation = nil
		report.SynergyPackages = append(report.SynergyPackages, c)
	}

	// (a) Layer-B context re-tag: VALUE entries advancing the plan gain
	//     +SYNERGY and are mirrored into SynergyPackages (value+synergy
	//     co-tag). Mutates the VALUE entry's Categories in place.
	for i := range report.ValueEngines {
		ve := &report.ValueEngines[i]
		if !anyCardSynergy(ve.Cards) {
			continue
		}
		if !hasCategory(ve.Categories, CategorySynergy) {
			ve.Categories = append(ve.Categories, CategorySynergy)
		}
		add(*ve, []string{CategoryValue, CategorySynergy})
	}

	// (b) Pure per-card synergy pieces (lords, symmetric levers, theme
	//     enablers) that are NOT already a value or combo entry.
	for _, p := range report.Profiles {
		fit, ok := fitByName[p.Name]
		if !ok {
			continue
		}
		key := comboCardKey([]string{p.Name})
		if valueKeys[key] || comboKeys[key] || seen[key] {
			continue
		}
		add(ComboResult{
			Cards:       []string{p.Name},
			LoopType:    "synergy",
			Resources:   strings.Join(fit.Themes, "+"),
			Description: synergyDescription(p, fit, themes),
		}, []string{CategorySynergy})
	}

	// (c) Multi-card synergy interactions surfaced by the legacy detectors
	//     (lord pairs, theme packages) that advance the plan and aren't already
	//     a combo. Value pairs already co-tagged in (a) are skipped via `seen`.
	for _, c := range report.Synergies {
		if len(c.Cards) < 2 {
			continue
		}
		if comboKeys[comboCardKey(c.Cards)] {
			continue
		}
		if anyCardSynergy(c.Cards) {
			add(c, []string{CategorySynergy})
		}
	}
}

func synergyDescription(p CardProfile, fit SynergyFit, themes []string) string {
	switch {
	case len(fit.Themes) > 0:
		return fmt.Sprintf("%s — advances the commander's %s plan", p.Name, strings.Join(fit.Themes, "/"))
	case p.IsTribalLord || p.IsTribalPayoff:
		return fmt.Sprintf("%s — tribal lord/payoff synergy piece", p.Name)
	case p.Symmetric:
		return fmt.Sprintf("%s — symmetric table-wide effect (synergy lever)", p.Name)
	default:
		return fmt.Sprintf("%s — supports the deck's plan", p.Name)
	}
}

func hasCategory(cats []string, target string) bool {
	for _, c := range cats {
		if c == target {
			return true
		}
	}
	return false
}
