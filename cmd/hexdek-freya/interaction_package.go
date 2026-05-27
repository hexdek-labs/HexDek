package main

import (
	"sort"
	"strings"
)

// InteractionPackage scores how well a deck can defend its win lines through
// proactive and reactive interaction. The four contributing buckets capture
// distinct kinds of defense:
//
//   - Counterspells stop opposing interaction on the stack: the canonical
//     defense for any spell-based win line (Thoracle, Storm, Doomsday).
//   - OpponentInteraction lists removal that can target the opponent's OWN
//     interaction permanents — Krosan Grip on Rest in Peace, Boseiju on a
//     hate piece, Pyroblast on a counterspell, Stifle on a triggered
//     ability. These open the window for the win line to resolve.
//   - Protection covers proactive defense pieces: Veil of Summer / Silence
//     / Defense Grid (deny opponents their stack interaction) and
//     Heroic Intervention / Lightning Greaves / Mother of Runes (deny
//     opponents their removal once your pieces are in play).
//   - ProtectionTutors lists tutors in the deck whose restriction allows
//     fetching at least one of the deck's protection cards — Mystical
//     Tutor → Veil of Summer, Idyllic Tutor → Defense Grid, Demonic Tutor
//     → anything. Tutor support converts the deck's protection package
//     from "draw it naturally" to "find it the turn you go off."
//
// Per-winline DefendedBy is the union of Counterspells + Protection — the
// cards that directly defend that line resolving. OpponentInteraction and
// ProtectionTutors are deck-level signals that don't attach to specific
// lines (they enable defense rather than ARE defense).
//
// Score is in [0, 1]. Each bucket is normalized against a saturation point
// matched to "what a B4 cEDH deck typically runs" and averaged. A B2 precon
// with no counterspells, no protection, and a single Disenchant lands near
// 0.05; a Thoracle deck with Force / Pact / Veil / Silence / Defense Grid
// + Mystical Tutor lands above 0.85.
type InteractionPackage struct {
	Counterspells       []string // RoleCounterspell cards in the deck (sorted)
	OpponentInteraction []string // removal that can hit opp's stack/A/E/ability (sorted)
	Protection          []string // proactive protection effects (sorted)
	ProtectionTutors    []string // tutors that can fetch at least one Protection card (sorted)
	Score               float64  // 0..1 — see InteractionPackage doc comment
}

// protectionCards is the curated set of dedicated protection effects. Two
// flavors mixed: stack denial (Veil / Silence / Defense Grid / Grand
// Abolisher) and permanent/creature shielding (Heroic Intervention /
// Lightning Greaves / Mother of Runes). The two categories complement each
// other — stack denial protects you assembling the line, permanent
// shielding protects you resolving it.
//
// Curation policy: a card belongs here only if its FIRST-ORDER purpose is
// to defend you or your spells. General-utility creatures that incidentally
// grant protection (e.g. Sigarda, Host of Herons — passive hexproof, but
// primarily a threat) are out — those are counted via the existing
// protectionWords scan on per-card oracle text (computeProtectionDensity).
// This list is for DEDICATED protection slots.
//
// Keys lowercased to match the oracleDB lookup convention.
var protectionCards = map[string]bool{
	"veil of summer":         true,
	"silence":                true,
	"autumn's veil":          true,
	"defense grid":           true,
	"grand abolisher":        true,
	"conqueror's flail":      true,
	"city of solitude":       true,
	"dosan the falling leaf": true,
	"teferi, time raveler":   true,
	"allosaurus shepherd":    true,
	"cavern of souls":        true,
	"heroic intervention":    true,
	"tamiyo's safekeeping":   true,
	"vines of vastwood":      true,
	"mizzium skin":           true,
	"wrap in vigor":          true,
	"inspiring call":         true,
	"privileged position":    true,
	"sterling grove":         true,
	"sylvan safekeeper":      true,
	"blossoming defense":     true,
	"snakeskin veil":         true,
	"lightning greaves":      true,
	"swiftfoot boots":        true,
	"mother of runes":        true,
	"giver of runes":         true,
	"selfless spirit":        true,
	"boros charm":            true,
	"eerie interlude":        true,
	"ghostway":               true,
	"flawless maneuver":      true,
	"tyvar's stand":          true,
	"faith's reward":         true,
	"deflecting swat":        true,
	"fierce guardianship":    true,
	"rebuff the wicked":      true,
}

// opponentInteractionCards is the curated set of removal/disruption that
// can answer an opponent's hate piece or counter their reactive spell.
// These open the window for the win line to resolve — distinct from
// general creature removal (Swords to Plowshares) which doesn't defend
// against the kinds of cards that stop combos.
//
// Includes: split-second / instant artifact-enchantment removal (Krosan
// Grip, Nature's Claim, Force of Vigor); land/channel disruption (Boseiju
// Who Endures); color-hosing counters (Pyroblast, Red Elemental Blast);
// activated/triggered-ability counters (Stifle, Trickbind, Disallow);
// activation-locks (Pithing Needle, Phyrexian Revoker — name a hate-piece
// activation and shut it down).
var opponentInteractionCards = map[string]bool{
	"krosan grip":          true,
	"force of vigor":       true,
	"nature's claim":       true,
	"disenchant":           true,
	"naturalize":           true,
	"return to nature":     true,
	"wear // tear":         true,
	"aura shards":          true,
	"reclamation sage":     true,
	"bane of progress":     true,
	"vandalblast":          true,
	"shatterstorm":         true,
	"by force":             true,
	"boseiju, who endures": true,
	"pyroblast":            true,
	"red elemental blast":  true,
	"hydroblast":           true,
	"blue elemental blast": true,
	"stifle":               true,
	"trickbind":            true,
	"disallow":             true,
	"squelch":              true,
	"tale's end":           true,
	"pithing needle":       true,
	"phyrexian revoker":    true,
	"sorcerous spyglass":   true,
	"null rod":             true,
	"stony silence":        true,
	"collector ouphe":      true,
}

// computeInteractionPackage builds dp.InteractionPackage and back-fills
// each report.WinLines.WinLines[i].DefendedBy so consumers see which
// interaction cards specifically defend each detected line.
//
// Mutates report.WinLines.WinLines in place. Safe to call with nil
// WinLines / Roles — produces an empty package with score 0.
func computeInteractionPackage(dp *DeckProfile, report *FreyaReport, oracle *oracleDB) {
	if dp == nil || report == nil {
		return
	}

	pkg := InteractionPackage{}

	// Counterspells — pulled straight from RoleCounterspell tagging so we
	// pick up the full curated counter list (Counterspell, Force of Will,
	// Mana Drain, Mental Misstep, Swan Song, etc.) without re-deriving it.
	if report.Roles != nil {
		for _, a := range report.Roles.Assignments {
			for _, r := range a.Roles {
				if r == RoleCounterspell {
					pkg.Counterspells = append(pkg.Counterspells, a.Name)
					break
				}
			}
		}
	}

	// Protection + OpponentInteraction — name-table lookups over the
	// deck's card list. Profiles iterates the 99 (sans duplicates from
	// the qty layer); ensures Lightning Greaves only counts once even if
	// listed twice somehow.
	seenProt := map[string]bool{}
	seenOpp := map[string]bool{}
	for _, p := range report.Profiles {
		key := strings.ToLower(p.Name)
		if protectionCards[key] && !seenProt[p.Name] {
			pkg.Protection = append(pkg.Protection, p.Name)
			seenProt[p.Name] = true
		}
		if opponentInteractionCards[key] && !seenOpp[p.Name] {
			pkg.OpponentInteraction = append(pkg.OpponentInteraction, p.Name)
			seenOpp[p.Name] = true
		}
	}

	// ProtectionTutors — only meaningful when the deck actually has
	// protection cards for the tutor to find. We reuse the WinLines tutor
	// classification (TutorInfo with restricted bucket) so the matching
	// rules stay consistent with win-line tutor coverage.
	if len(pkg.Protection) > 0 && report.WinLines != nil {
		typeByName := map[string]string{}
		for _, p := range report.Profiles {
			if p.TypeLine != "" {
				typeByName[p.Name] = strings.ToLower(p.TypeLine)
			}
		}
		if oracle != nil {
			for _, p := range report.Profiles {
				entry := oracle.lookup(p.Name)
				if entry != nil && entry.TypeLine != "" {
					typeByName[p.Name] = strings.ToLower(entry.TypeLine)
				}
			}
		}
		seenTutor := map[string]bool{}
		for _, t := range report.WinLines.TutorMap {
			if seenTutor[t.Name] {
				continue
			}
			// Skip land tutors — they never find protection.
			if t.Restricted == "land" || t.Restricted == "basic_land" {
				continue
			}
			for _, prot := range pkg.Protection {
				if tutorCanFind(t, prot, typeByName, oracle) {
					pkg.ProtectionTutors = append(pkg.ProtectionTutors, t.Name)
					seenTutor[t.Name] = true
					break
				}
			}
		}
	}

	sort.Strings(pkg.Counterspells)
	sort.Strings(pkg.OpponentInteraction)
	sort.Strings(pkg.Protection)
	sort.Strings(pkg.ProtectionTutors)

	pkg.Score = interactionPackageScore(pkg)
	dp.InteractionPackage = pkg

	// Per-winline DefendedBy: counterspells + protection union. Cards from
	// OpponentInteraction and ProtectionTutors are deck-level enablers —
	// they don't attach to a specific resolving line. Capped at
	// defendedByCap entries; alphabetized.
	if report.WinLines != nil {
		defenders := uniqueStrings(append(append([]string(nil), pkg.Counterspells...), pkg.Protection...))
		sort.Strings(defenders)
		if len(defenders) > defendedByCap {
			defenders = defenders[:defendedByCap]
		}
		for i := range report.WinLines.WinLines {
			if len(defenders) == 0 {
				report.WinLines.WinLines[i].DefendedBy = nil
				continue
			}
			report.WinLines.WinLines[i].DefendedBy = append([]string(nil), defenders...)
		}
	}
}

// defendedByCap bounds the per-winline DefendedBy list. Tuned so the JSON
// payload stays compact (an 80-card "protection deck" with 8 counterspells
// + 12 protection cards is realistic — listing all 20 against every win
// line would be noise).
const defendedByCap = 12

// interactionPackageScore normalizes each bucket against a saturation
// point and averages. Saturation points reflect "what a B4 deck typically
// runs":
//
//	Counterspells       ≥ 4 → full credit  (Thoracle deck minimum)
//	OpponentInteraction ≥ 3 → full credit  (Krosan/Boseiju/Pyroblast)
//	Protection          ≥ 3 → full credit  (Veil + Silence + Greaves)
//	ProtectionTutors    ≥ 2 → full credit  (Mystical + Demonic)
//
// Equal weights — no bucket dominates. A deck strong in only one
// category still scores meaningfully (max 0.25); a deck balanced across
// all four scores 1.0.
func interactionPackageScore(pkg InteractionPackage) float64 {
	norm := func(count, sat int) float64 {
		if count >= sat {
			return 1.0
		}
		return float64(count) / float64(sat)
	}
	counters := norm(len(pkg.Counterspells), 4)
	oppInt := norm(len(pkg.OpponentInteraction), 3)
	prot := norm(len(pkg.Protection), 3)
	tutors := norm(len(pkg.ProtectionTutors), 2)
	return (counters + oppInt + prot + tutors) / 4.0
}

// uniqueStrings is defined in advanced.go — reused here.
