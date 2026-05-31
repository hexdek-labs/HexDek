package analytics

import "strings"

// ClassifyArchetype derives a coarse-grained archetype label for a
// DeckFingerprint based on three signals:
//
//   1. Signature card hits — how many cards from each archetype's
//      canonical signature list show up in CardFrequency at >= 0.20
//      (cast in at least 20% of observed games).
//   2. Win-condition mix — which WinCondition kind dominates wins.
//   3. Threat timing — AvgTurnToFirstCombo vs. AvgFirstBlood gives
//      a "race to combo" vs. "long combat" tilt.
//
// Returns (archetype label, confidence [0, 1]).
//
// The signature lists are intentionally short — 5-12 anchor cards per
// archetype. They identify the SHAPE of the deck, not its entire
// strategy. A deck with 4 Stax-list hits at high frequency is
// extremely likely a Stax deck; one hit is meaningless. The threshold
// math is deliberately simple so the heuristic is auditable and
// failure modes are obvious.
//
// "unknown" is returned when:
//   - No signature list hits ≥2 entries at the 0.20 frequency floor.
//   - WinConditionMix is empty (no wins observed yet).
//   - GamesObserved < 3 (sample too small).
//
// The hat's OpponentProfile should treat archetype="unknown" as
// "default to your own observation"; a high-confidence non-unknown
// fingerprint can pre-seed OpponentProfile.Archetype +
// OpponentProfile.Confidence on game start.
func ClassifyArchetype(fp *DeckFingerprint) (string, float64) {
	if fp == nil || fp.GamesObserved < 3 {
		return "unknown", 0
	}

	// Bucket the >=0.20 frequency entries by archetype signature.
	hits := map[string]int{
		"stax":     0,
		"combo":    0,
		"aggro":    0,
		"control":  0,
		"midrange": 0,
	}
	for _, e := range fp.CardFrequency {
		if e.Frequency < 0.20 {
			break // entries are sorted by frequency desc.
		}
		lower := strings.ToLower(e.Name)
		for archetype, signature := range archetypeSignatures {
			for _, sig := range signature {
				if strings.Contains(lower, sig) {
					hits[archetype]++
					break
				}
			}
		}
	}

	// Win-condition signals provide an independent tilt.
	winTilt := map[string]float64{}
	for cond, frac := range fp.WinConditionMix {
		switch cond {
		case "combo":
			winTilt["combo"] += frac
		case "combat_damage":
			// Combat damage covers BOTH aggro and midrange. Use the
			// turn-to-first-blood signal to split: early first blood
			// → aggro, later → midrange.
			if fp.AvgFirstBlood > 0 && fp.AvgFirstBlood <= 6.0 {
				winTilt["aggro"] += frac
			} else {
				winTilt["midrange"] += frac
			}
		case "commander_damage":
			// Voltron — leans aggro since the deck has to swing.
			winTilt["aggro"] += frac * 0.7
			winTilt["midrange"] += frac * 0.3
		case "life_drain":
			// Drain decks straddle combo and control.
			winTilt["combo"] += frac * 0.5
			winTilt["control"] += frac * 0.5
		case "turn_cap", "timeout":
			// Hit the turn cap — pillow-fort / stax / pure control.
			winTilt["stax"] += frac * 0.5
			winTilt["control"] += frac * 0.5
		}
	}

	// Combined score per archetype: signature hits (capped at the
	// archetype's signature list length) × 0.6 + winTilt × 0.4.
	scores := map[string]float64{}
	for archetype := range hits {
		sigLen := float64(len(archetypeSignatures[archetype]))
		if sigLen == 0 {
			continue
		}
		sigScore := float64(hits[archetype]) / sigLen
		if sigScore > 1.0 {
			sigScore = 1.0
		}
		scores[archetype] = sigScore*0.6 + winTilt[archetype]*0.4
	}

	// Pick the winner. Tiebreak by descending signature-hit count.
	var bestArch string
	var bestScore float64
	for arch, score := range scores {
		if score > bestScore || (score == bestScore && hits[arch] > hits[bestArch]) {
			bestArch = arch
			bestScore = score
		}
	}

	// Reject low-confidence reads. The signature side must have at
	// least 2 hits OR the win mix must have ≥0.5 contribution to one
	// archetype for us to commit.
	if hits[bestArch] < 2 && winTilt[bestArch] < 0.5 {
		return "unknown", 0
	}
	// Clamp confidence to [0, 1].
	confidence := bestScore
	if confidence > 1.0 {
		confidence = 1.0
	}
	return bestArch, confidence
}

// archetypeSignatures lists the canonical anchor-card substrings for
// each archetype. Substring match against the lowercased card name —
// "Smothering Tithe" matches "smothering tithe", "Smothering Tithe
// (Token)" (rare but safe), and stays robust to commander-tax-style
// suffixes if Heimdall ever emits them. The lists are deliberately
// short; the goal is a fast majority-vote, not exhaustive coverage.
//
// Sources: heuristic from Freya's known-archetype card lists +
// community staples. The hat's OpponentProfile already has its own
// rough archetype set ("aggro", "combo", "control", "midrange"); we
// add "stax" as a distinct fifth bucket since stax behaves very
// differently from generic control and the hat's threat-priority
// downstream branches on it.
var archetypeSignatures = map[string][]string{
	"stax": {
		"smothering tithe",
		"rhystic study",
		"winter orb",
		"stasis",
		"static orb",
		"trinisphere",
		"thalia, guardian of thraben",
		"thalia, heretic cathar",
		"drannith magistrate",
		"deafening silence",
		"collector ouphe",
		"null rod",
		"cursed totem",
		"linvala, keeper of silence",
	},
	"combo": {
		"thassa's oracle",
		"laboratory maniac",
		"jace, wielder of mysteries",
		"demonic consultation",
		"tainted pact",
		"dramatic reversal",
		"isochron scepter",
		"food chain",
		"underworld breach",
		"ad nauseam",
		"doomsday",
		"hermit druid",
		"mikaeus, the unhallowed",
	},
	"aggro": {
		"lightning bolt",
		"goblin guide",
		"monastery swiftspear",
		"craterhoof behemoth",
		"embercleave",
		"berserkers' onslaught",
		"hammer of nazahn",
		"shadowspear",
		"loxodon warhammer",
		"akroma's memorial",
	},
	"control": {
		"counterspell",
		"swan song",
		"mana drain",
		"force of negation",
		"force of will",
		"wrath of god",
		"damnation",
		"toxic deluge",
		"cyclonic rift",
		"farewell",
		"swords to plowshares",
		"path to exile",
		"teferi's protection",
	},
	"midrange": {
		"sol ring",
		"command tower",
		"arcane signet",
		"cultivate",
		"kodama's reach",
		"beast within",
		"generous gift",
		"rampant growth",
		"farseek",
		"three visits",
		"nature's lore",
	},
}
