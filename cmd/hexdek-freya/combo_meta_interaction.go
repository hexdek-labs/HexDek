package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Combo-vs-meta interaction — per-combo exposure profile across the three
// canonical disruption axes a deck faces in a real pod:
//
//   1. STAX VULNERABILITY: cards in the meta that block the combo's
//      execution before it can fire. Drannith Magistrate (denies casts
//      from non-hand zones), Rule of Law / Eidolon of Rhetoric (one
//      spell per turn — kills multi-cast combos), Possibility Storm
//      (replaces the cast), counterspells (kills the resolve), Pithing
//      Needle (locks an activated combo key), Cursed Totem (creature
//      activated abilities off).
//
//   2. GRAVEYARD-HATE EXPOSURE: for combos that depend on the
//      graveyard. Rest in Peace / Leyline of the Void (continuous
//      exile-replace), Bojuka Bog (colorless one-shot), Faerie Macabre
//      (free instant exile of two cards), Grafdigger's Cage (blocks
//      cast-from-graveyard + cast-from-library).
//
//   3. REMOVAL TOLERANCE: how much spot removal the combo absorbs
//      before collapsing. Counts permanent pieces (creature / artifact
//      / enchantment / planeswalker / battle) that can be targeted by
//      spot removal, splits by whether each piece carries built-in
//      protection (hexproof / shroud / ward / indestructible / ward /
//      protection-from / can't-be-targeted / phase-out). The cheapest
//      kill cost is 1 removal targeting any unprotected piece; combos
//      with every piece protected require a protection-stripper plus a
//      removal (cost = 2).
//
// Per-combo entries are rolled up into a deck-level worst-case summary
// surfacing the single hoser that breaks the most combos, plus
// "combos that survive the canonical hate set" counts for stax and
// graveyard hate. Both are useful resilience signals — a deck with 3
// detected combos but only 1 survives RIP is a different shape than
// one where all 3 are graveyard-independent.
//
// Wiring: built in BuildDeckProfile after computeProtectionDensity (so
// the oracle scan can reuse the same protection-word list), attached
// to DeckProfile.ComboMetaInteraction (nil when no combos detected),
// surfaced under `combo_meta_interaction` in the unified-profile JSON
// and as a short text section after the combo-interaction matrix.
// ---------------------------------------------------------------------------

// ComboMetaVulnerability is the per-combo exposure profile. ComboIndex
// indexes into the deck's ComboInteractionMatrix.Combos slice when that
// matrix exists; -1 when the combo wasn't analyzed via the matrix path
// (1-combo decks where the matrix is nil but a single-combo vuln is
// still computed).
type ComboMetaVulnerability struct {
	ComboIndex int    // -1 when no ComboInteractionMatrix exists
	Label      string // combo display label
	Source     string // true_infinite / determined / graveyard_loop
	Cards      []string

	// Stax vulnerability. Score 0..3 (0 = immune, 3 = critical multi-vector
	// vulnerability). Hosers + Reasons are aligned slices.
	StaxScore   int
	StaxHosers  []string
	StaxReasons []string

	// Graveyard hate exposure. Score 0..3.
	GraveyardScore   int
	GraveyardHosers  []string
	GraveyardReasons []string

	// Removal tolerance. PermanentPieces is the count of pieces on the
	// battlefield (vulnerable to spot removal). ProtectedPieces is the
	// subset of those with built-in protection. RemovalRequiredToBreak
	// is the cheapest opponent spend to collapse the combo:
	//   - 1 when any unprotected permanent piece exists
	//   - 2 when every permanent piece is protected (need strip + remove)
	//   - 0 when the combo has no permanent pieces (instants/sorceries
	//     only — spot removal can't break it without counterspells)
	PermanentPieces       int
	ProtectedPieces       int
	UnprotectedPieceNames []string
	ProtectedPieceNames   []string
	RemovalRequiredToBreak int

	// DominantThreat names the highest-severity axis: "stax" /
	// "graveyard" / "removal" / "resilient" (when all scores are low and
	// removal cost is >= 2).
	DominantThreat string
}

// ComboMetaInteraction is the deck-level rollup of per-combo
// vulnerabilities, plus headline "how many combos survive the
// canonical hate set" counts.
type ComboMetaInteraction struct {
	PerCombo []ComboMetaVulnerability

	// WorstStaxHoser is the stax piece that shuts down the most combos
	// in this deck (if any). Empty when no combo carries a stax hoser.
	// Count is the number of combos the hoser hits.
	WorstStaxHoser      string
	WorstStaxCount      int
	WorstGraveyardHoser string
	WorstGraveyardCount int

	// Survive*Count: combos with the relevant score == 0 (no vulnerability
	// from that axis at all). A 3-combo deck where 2 are graveyard-
	// independent reports SurviveGraveyardCount=2.
	SurviveStaxCount      int
	SurviveGraveyardCount int

	// FragileComboCount: combos whose RemovalRequiredToBreak == 1
	// (every combo with at least one unprotected permanent piece).
	// "Most combos are 1-removal-fragile" is the headline most decks
	// surface — protection density is rare.
	FragileComboCount int
}

// ---------------------------------------------------------------------------
// Hoser databases — meta cards mapped to the combo property they exploit.
//
// Each entry pairs a hoser name with a short reason and a severity
// (1..3). Detection runs by matching the combo's properties (graveyard-
// using, recursion-using, has activated ability, etc.) against the
// hoser's exploited property.
// ---------------------------------------------------------------------------

type metaHoser struct {
	Name     string
	Reason   string
	Severity int
}

// Stax hosers by combo property. Keys are property tags computed by
// staxPropertiesFor — every combo gets a set of tags, and each tag
// activates the matching hoser entries.
var staxHosersByProperty = map[string][]metaHoser{
	"casts_from_nonhand": {
		{"Drannith Magistrate", "denies casting from graveyard / exile / command zone", 3},
	},
	"multi_cast_per_turn": {
		{"Rule of Law", "locks every player to 1 spell per turn — collapses multi-cast combo chains", 3},
		{"Eidolon of Rhetoric", "same 1-spell-per-turn lock — also pings the caster", 3},
	},
	"recurs_spells_from_graveyard": {
		{"Possibility Storm", "every cast becomes a random reveal — combo lines that depend on a specific cast line collapse", 2},
	},
	"has_activated_ability": {
		{"Pithing Needle", "names a key combo piece — Walking Ballista, Isochron Scepter, Thassa's Oracle activations all stop", 2},
		{"Cursed Totem", "creature activated abilities off — kills any creature-tap or creature-pay combo", 2},
	},
	"depends_on_spell_resolution": {
		{"Counterspell", "any cheap counterspell answers the combo's keystone resolve", 1},
	},
}

// Graveyard hosers — applied to combos that use the graveyard at all.
// Severity drops between continuous (Leyline / RIP) and one-shot
// (Bojuka Bog) so the rollup reports the worst-case hoser per combo.
var graveyardHosers = []metaHoser{
	{"Rest in Peace", "continuous exile-replace — graveyard never fills, combo never starts", 3},
	{"Leyline of the Void", "free turn-zero exile-replace — graveyard never fills", 3},
	{"Grafdigger's Cage", "blocks cast-from-graveyard AND cast-from-library — kills Breach / Birthing Pod / Eldritch Evolution", 3},
	{"Bojuka Bog", "colorless one-shot graveyard exile — slots into any mana base", 2},
	{"Faerie Macabre", "free instant exile of two graveyard cards — kills the keystone mid-combo", 2},
	{"Soul-Guide Lantern", "{1} exile-all-graveyards activated artifact — replaces itself", 2},
}

// staxPropertiesFor returns the set of stax-property tags a combo
// exhibits. Pure property-of-the-combo detection — no oracle text scan
// per piece, just combo-shape inspection.
func staxPropertiesFor(cards []CardProfile, source string) map[string]bool {
	out := map[string]bool{}
	if len(cards) == 0 {
		return out
	}

	// casts_from_nonhand: the combo plays a card from graveyard /
	// exile / command zone. Source=="graveyard_loop" trivially
	// matches. Otherwise look for IsRecursion (returns from graveyard,
	// typically to battlefield via a cast trigger) or known-shape
	// cards.
	if source == "graveyard_loop" {
		out["casts_from_nonhand"] = true
	}
	for _, c := range cards {
		if c.IsRecursion {
			out["casts_from_nonhand"] = true
		}
		// Known recast engines that cast from graveyard / library /
		// exile. These are the cards Drannith Magistrate specifically
		// shuts down.
		nameLower := strings.ToLower(c.Name)
		for _, anchor := range []string{
			"underworld breach", "yawgmoth's will", "past in flames",
			"mizzix's mastery", "echo of eons", "doomsday",
			"birthing pod", "eldritch evolution", "neoform",
			"natural order", "ad nauseam", "demonic consultation",
			"tainted pact",
		} {
			if strings.Contains(nameLower, anchor) {
				out["casts_from_nonhand"] = true
			}
		}
	}

	// multi_cast_per_turn: combos with 3+ pieces typically chain casts
	// in a single turn. Storm finishers (Aetherflux / Tendrils /
	// Grapeshot) are unconditionally multi-cast.
	if len(cards) >= 3 {
		out["multi_cast_per_turn"] = true
	}
	for _, c := range cards {
		if c.IsStormFinisher {
			out["multi_cast_per_turn"] = true
		}
	}

	// recurs_spells_from_graveyard: more specific than casts_from_nonhand.
	// Detects Possibility-Storm-style risk. Same anchors as the
	// graveyard-recur set above, plus IsRecursion with RecursionDest=="battlefield".
	for _, c := range cards {
		if c.IsRecursion {
			out["recurs_spells_from_graveyard"] = true
		}
	}

	// has_activated_ability: heuristic — any piece with non-empty
	// Consumes that is mana, OR known activated-ability combo pieces.
	for _, c := range cards {
		if containsRes(c.Consumes, ResMana) {
			out["has_activated_ability"] = true
		}
		nameLower := strings.ToLower(c.Name)
		for _, anchor := range []string{
			"walking ballista", "triskelion", "isochron scepter",
			"staff of domination", "metalworker", "ashnod's altar",
			"krark-clan ironworks", "phyrexian altar", "grim monolith",
			"basalt monolith", "rings of brighthearth", "thassa's oracle",
		} {
			if strings.Contains(nameLower, anchor) {
				out["has_activated_ability"] = true
			}
		}
	}

	// depends_on_spell_resolution: every combo's keystone is at least
	// one spell cast. Always true except for purely-triggered combos
	// already on the battlefield (Sanguine Bond + Exquisite Blood
	// post-resolution). We mark it true universally — counterspells
	// answer any keystone-cast cast moment.
	out["depends_on_spell_resolution"] = true

	return out
}

// usesGraveyard returns true if the combo depends on the graveyard for
// its execution. Source=="graveyard_loop" is a strict yes; otherwise
// scan piece flags + names.
func usesGraveyard(cards []CardProfile, source string) bool {
	if source == "graveyard_loop" {
		return true
	}
	for _, c := range cards {
		if c.IsRecursion {
			return true
		}
		if containsRes(c.Produces, ResGraveyard) || containsRes(c.Produces, ResReanimate) ||
			containsRes(c.Produces, ResGraveyardFill) ||
			containsRes(c.Consumes, ResGraveyard) || containsRes(c.Consumes, ResReanimate) {
			return true
		}
		nameLower := strings.ToLower(c.Name)
		for _, anchor := range []string{
			"underworld breach", "yawgmoth's will", "past in flames",
			"animate dead", "necromancy", "dance of the dead",
			"reanimate", "worldgorger dragon", "karmic guide",
			"reveillark", "sun titan", "muldrotha", "meren",
			"sheoldred", "kokusho", "phyrexian reclamation",
			"yawgmoth, thran physician", "lurrus", "doomsday",
		} {
			if strings.Contains(nameLower, anchor) {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Per-combo builder
// ---------------------------------------------------------------------------

// BuildComboMetaInteraction computes the per-combo and deck-level
// vulnerability profile. Returns nil when the report has no combos in
// the TrueInfinites + Determined + GraveyardLoops bucket.
func BuildComboMetaInteraction(report *FreyaReport, oracle *oracleDB) *ComboMetaInteraction {
	if report == nil {
		return nil
	}

	type entry struct {
		Index  int
		Label  string
		Cards  []string
		Source string
	}
	var entries []entry
	addAll := func(combos []ComboResult, source string) {
		for _, c := range combos {
			if len(c.Cards) == 0 {
				continue
			}
			cards := append([]string(nil), c.Cards...)
			sort.Strings(cards)
			entries = append(entries, entry{
				Label:  strings.Join(cards, " + "),
				Cards:  cards,
				Source: source,
			})
		}
	}
	addAll(report.TrueInfinites, "true_infinite")
	addAll(report.Determined, "determined")
	addAll(report.GraveyardLoops, "graveyard_loop")

	if len(entries) == 0 {
		return nil
	}

	// If a ComboInteractionMatrix exists, align per-combo indices to
	// the matrix's Combos slice (same dedup ordering: source asc,
	// label asc). Otherwise emit indices in entry-add order.
	if report.ComboInteraction != nil {
		sort.SliceStable(entries, func(i, j int) bool {
			if entries[i].Source != entries[j].Source {
				return entries[i].Source < entries[j].Source
			}
			return entries[i].Label < entries[j].Label
		})
		seen := map[string]bool{}
		deduped := entries[:0]
		for _, e := range entries {
			key := e.Source + "|" + e.Label
			if seen[key] {
				continue
			}
			seen[key] = true
			deduped = append(deduped, e)
		}
		entries = deduped
		for i := range entries {
			entries[i].Index = i
		}
	} else {
		for i := range entries {
			entries[i].Index = i
		}
	}

	profileByName := profileMapByName(report.Profiles)

	out := &ComboMetaInteraction{}
	staxHoserCount := map[string]int{}
	gyHoserCount := map[string]int{}

	for _, e := range entries {
		// Hydrate per-piece CardProfile from the deck (missing entries
		// fall back to a minimal stub).
		var pieces []CardProfile
		for _, name := range e.Cards {
			if p, ok := profileByName[name]; ok {
				pieces = append(pieces, p)
			} else {
				pieces = append(pieces, CardProfile{Name: name})
			}
		}

		vuln := ComboMetaVulnerability{
			ComboIndex: e.Index,
			Label:      e.Label,
			Source:     e.Source,
			Cards:      e.Cards,
		}

		// Stax exposure.
		props := staxPropertiesFor(pieces, e.Source)
		// Iterate property tags in sorted order so hoser lists are
		// deterministic across runs.
		var propTags []string
		for tag := range props {
			propTags = append(propTags, tag)
		}
		sort.Strings(propTags)
		staxSeen := map[string]bool{}
		for _, tag := range propTags {
			for _, h := range staxHosersByProperty[tag] {
				if staxSeen[h.Name] {
					continue
				}
				staxSeen[h.Name] = true
				vuln.StaxHosers = append(vuln.StaxHosers, h.Name)
				vuln.StaxReasons = append(vuln.StaxReasons, h.Reason)
				if h.Severity > vuln.StaxScore {
					vuln.StaxScore = h.Severity
				}
				staxHoserCount[h.Name]++
			}
		}

		// Graveyard exposure.
		if usesGraveyard(pieces, e.Source) {
			for _, h := range graveyardHosers {
				vuln.GraveyardHosers = append(vuln.GraveyardHosers, h.Name)
				vuln.GraveyardReasons = append(vuln.GraveyardReasons, h.Reason)
				if h.Severity > vuln.GraveyardScore {
					vuln.GraveyardScore = h.Severity
				}
				gyHoserCount[h.Name]++
			}
		}

		// Removal tolerance: classify each piece by permanent-ness +
		// built-in protection (via oracle scan, mirroring
		// computeProtectionDensity).
		for _, p := range pieces {
			if !isComboPiecePermanent(p) {
				continue
			}
			vuln.PermanentPieces++
			if hasBuiltInProtection(p, oracle) {
				vuln.ProtectedPieces++
				vuln.ProtectedPieceNames = append(vuln.ProtectedPieceNames, p.Name)
			} else {
				vuln.UnprotectedPieceNames = append(vuln.UnprotectedPieceNames, p.Name)
			}
		}
		switch {
		case vuln.PermanentPieces == 0:
			vuln.RemovalRequiredToBreak = 0
		case vuln.PermanentPieces == vuln.ProtectedPieces:
			vuln.RemovalRequiredToBreak = 2
		default:
			vuln.RemovalRequiredToBreak = 1
		}

		// Dominant threat — highest-severity axis. Tie-breaks: stax >
		// graveyard > removal so the more strategic locks read first.
		switch {
		case vuln.GraveyardScore >= 3:
			vuln.DominantThreat = "graveyard"
		case vuln.StaxScore >= 3:
			vuln.DominantThreat = "stax"
		case vuln.GraveyardScore == 2:
			vuln.DominantThreat = "graveyard"
		case vuln.StaxScore == 2:
			vuln.DominantThreat = "stax"
		case vuln.RemovalRequiredToBreak == 1:
			vuln.DominantThreat = "removal"
		default:
			vuln.DominantThreat = "resilient"
		}

		if vuln.StaxScore == 0 {
			out.SurviveStaxCount++
		}
		if vuln.GraveyardScore == 0 {
			out.SurviveGraveyardCount++
		}
		if vuln.RemovalRequiredToBreak == 1 {
			out.FragileComboCount++
		}

		out.PerCombo = append(out.PerCombo, vuln)
	}

	// Deck-level worst-case hoser: max-count entry per axis. Tie-break
	// by name ascending for determinism.
	worstName := func(counts map[string]int) (string, int) {
		var best string
		bestN := 0
		var names []string
		for n := range counts {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			if counts[n] > bestN {
				bestN = counts[n]
				best = n
			}
		}
		return best, bestN
	}
	out.WorstStaxHoser, out.WorstStaxCount = worstName(staxHoserCount)
	out.WorstGraveyardHoser, out.WorstGraveyardCount = worstName(gyHoserCount)

	return out
}

// isComboPiecePermanent classifies a combo piece as a permanent
// (vulnerable to spot removal). Reads TypeLine — instants/sorceries
// fall through as non-permanent. Empty type line (oracle miss) treated
// as permanent by default since most combo pieces are permanents.
func isComboPiecePermanent(p CardProfile) bool {
	tl := strings.ToLower(p.TypeLine)
	if tl == "" {
		return true
	}
	if strings.Contains(tl, "instant") || strings.Contains(tl, "sorcery") {
		return false
	}
	return true
}

// hasBuiltInProtection scans a piece's oracle text for the canonical
// protection phrases (mirroring computeProtectionDensity in deckprofile.go).
// Returns false when the oracle is unavailable for the card — conservative
// "no protection" so removal-cost defaults to 1 for unknown pieces.
func hasBuiltInProtection(p CardProfile, oracle *oracleDB) bool {
	if oracle == nil {
		return false
	}
	entry := oracle.lookup(p.Name)
	if entry == nil {
		return false
	}
	ot := strings.ToLower(entry.OracleText)
	if ot == "" && len(entry.CardFaces) > 0 {
		ot = strings.ToLower(entry.CardFaces[0].OracleText)
	}
	for _, word := range []string{
		"hexproof", "shroud", "indestructible", "ward",
		"protection from", "can't be the target",
		"can't be countered", "can't be destroyed",
		"phase out", "phases out",
	} {
		if strings.Contains(ot, word) {
			return true
		}
	}
	return false
}

// printComboMetaInteraction renders the human-readable section. No-op
// when m is nil. Format follows the existing [CYA] convention.
func printComboMetaInteraction(w io.Writer, m *ComboMetaInteraction) {
	if m == nil || len(m.PerCombo) == 0 {
		return
	}
	fmt.Fprintf(w, "[CYA] COMBO vs META -- %d combo(s) profiled\n", len(m.PerCombo))
	if m.WorstStaxHoser != "" {
		fmt.Fprintf(w, "  Worst stax hoser: %s (breaks %d combo(s))\n",
			m.WorstStaxHoser, m.WorstStaxCount)
	}
	if m.WorstGraveyardHoser != "" {
		fmt.Fprintf(w, "  Worst graveyard hoser: %s (breaks %d combo(s))\n",
			m.WorstGraveyardHoser, m.WorstGraveyardCount)
	}
	fmt.Fprintf(w, "  Survives stax: %d/%d  |  Survives graveyard hate: %d/%d  |  1-removal-fragile: %d/%d\n",
		m.SurviveStaxCount, len(m.PerCombo),
		m.SurviveGraveyardCount, len(m.PerCombo),
		m.FragileComboCount, len(m.PerCombo))
	for _, v := range m.PerCombo {
		fmt.Fprintf(w, "  [%d] %s — dominant threat: %s\n", v.ComboIndex, v.Label, v.DominantThreat)
		if v.StaxScore > 0 {
			fmt.Fprintf(w, "      stax (sev %d): %s\n",
				v.StaxScore, strings.Join(v.StaxHosers, ", "))
		}
		if v.GraveyardScore > 0 {
			fmt.Fprintf(w, "      graveyard (sev %d): %s\n",
				v.GraveyardScore, strings.Join(v.GraveyardHosers, ", "))
		}
		fmt.Fprintf(w, "      removal: %d perm piece(s), %d protected, %d removal(s) to break\n",
			v.PermanentPieces, v.ProtectedPieces, v.RemovalRequiredToBreak)
	}
	fmt.Fprintf(w, "\n")
}

// comboMetaInteractionToJSON projects the rollup into the JSON shape
// (defined in report.go). Returns nil when the input is nil so the JSON
// field is omitted via the omitempty tag.
func comboMetaInteractionToJSON(m *ComboMetaInteraction) *jsonComboMetaInteraction {
	if m == nil {
		return nil
	}
	perCombo := make([]jsonComboMetaVuln, len(m.PerCombo))
	for i, v := range m.PerCombo {
		perCombo[i] = jsonComboMetaVuln{
			ComboIndex:             v.ComboIndex,
			Label:                  v.Label,
			Source:                 v.Source,
			Cards:                  v.Cards,
			StaxScore:              v.StaxScore,
			StaxHosers:             v.StaxHosers,
			StaxReasons:            v.StaxReasons,
			GraveyardScore:         v.GraveyardScore,
			GraveyardHosers:        v.GraveyardHosers,
			GraveyardReasons:       v.GraveyardReasons,
			PermanentPieces:        v.PermanentPieces,
			ProtectedPieces:        v.ProtectedPieces,
			UnprotectedPieceNames:  v.UnprotectedPieceNames,
			ProtectedPieceNames:    v.ProtectedPieceNames,
			RemovalRequiredToBreak: v.RemovalRequiredToBreak,
			DominantThreat:         v.DominantThreat,
		}
	}
	return &jsonComboMetaInteraction{
		PerCombo:              perCombo,
		WorstStaxHoser:        m.WorstStaxHoser,
		WorstStaxCount:        m.WorstStaxCount,
		WorstGraveyardHoser:   m.WorstGraveyardHoser,
		WorstGraveyardCount:   m.WorstGraveyardCount,
		SurviveStaxCount:      m.SurviveStaxCount,
		SurviveGraveyardCount: m.SurviveGraveyardCount,
		FragileComboCount:     m.FragileComboCount,
	}
}
