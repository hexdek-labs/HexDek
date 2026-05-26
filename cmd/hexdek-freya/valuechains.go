package main

import (
	"fmt"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Zone flow model -- describes how a card moves resources between zones.
// ---------------------------------------------------------------------------

type ZoneFlow struct {
	From     string // "library", "hand", "graveyard", "battlefield", "exile"
	To       string
	Resource string // "land", "creature", "card", "mana", "any"
}

// ---------------------------------------------------------------------------
// Value chain model -- a multi-step resource pipeline across zones.
// ---------------------------------------------------------------------------

type ValueChainStep struct {
	Label string   // "FILL", "RECOVER", "TRIGGER", "PAYOFF", etc.
	Flow  ZoneFlow
	Cards []string
}

type ValueChain struct {
	Name           string
	Steps          []ValueChainStep
	BridgeCards    []string
	Depth          int
	WeakestLink    int
	Redundancy     string // "HIGH", "MEDIUM", "LOW"
	RecursionDepth string // "infinite" (has loop-back), "deep" (3+ recursion pieces), "shallow" (1-2), "none"
	Rationale      *ValueChainRationale
}

// ValueChainRationale explains why an engine was identified for this deck.
type ValueChainRationale struct {
	Trigger    string   // what cards/interactions trigger the engine
	HowItWorks string   // 1-2 sentence description of engine mechanics
	KeyPieces  []string // anchor cards (bridges + weakest-link pieces)
}

// ---------------------------------------------------------------------------
// classifyZoneFlows -- determine zone transitions from oracle text and profile.
// ---------------------------------------------------------------------------

func classifyZoneFlows(ot, tl string, p *CardProfile) []ZoneFlow {
	var flows []ZoneFlow

	// Self-mill: library -> graveyard
	if strings.Contains(ot, "mill") && !strings.Contains(ot, "target opponent") &&
		!strings.Contains(ot, "target player") {
		flows = append(flows, ZoneFlow{From: "library", To: "graveyard", Resource: "card"})
	}
	// Surveil: library -> graveyard (selective)
	if strings.Contains(ot, "surveil") {
		flows = append(flows, ZoneFlow{From: "library", To: "graveyard", Resource: "card"})
	}
	// Dredge: library -> graveyard
	if strings.Contains(ot, "dredge") {
		flows = append(flows, ZoneFlow{From: "library", To: "graveyard", Resource: "card"})
	}
	// Discard: hand -> graveyard
	if containsAny(ot, "discard a card", "discard two", "discard your hand",
		"discard a creature", "discard a land") {
		flows = append(flows, ZoneFlow{From: "hand", To: "graveyard", Resource: "card"})
	}

	// Reanimate: graveyard -> battlefield
	if strings.Contains(ot, "return") && strings.Contains(ot, "graveyard") &&
		strings.Contains(ot, "battlefield") {
		resource := "creature"
		if strings.Contains(ot, "land") {
			resource = "land"
		} else if strings.Contains(ot, "permanent") || strings.Contains(ot, "card") {
			resource = "any"
		}
		flows = append(flows, ZoneFlow{From: "graveyard", To: "battlefield", Resource: resource})
	}

	// Recursion to hand: graveyard -> hand
	if strings.Contains(ot, "return") && strings.Contains(ot, "graveyard") &&
		(strings.Contains(ot, "to your hand") || strings.Contains(ot, "to its owner's hand")) {
		flows = append(flows, ZoneFlow{From: "graveyard", To: "hand", Resource: "card"})
	}

	// Library search to battlefield (tutors to field)
	if strings.Contains(ot, "search") && strings.Contains(ot, "library") &&
		strings.Contains(ot, "battlefield") {
		resource := "any"
		if strings.Contains(ot, "land") {
			resource = "land"
		} else if strings.Contains(ot, "creature") {
			resource = "creature"
		}
		flows = append(flows, ZoneFlow{From: "library", To: "battlefield", Resource: resource})
	}

	// Library search to hand (tutor)
	if strings.Contains(ot, "search") && strings.Contains(ot, "library") &&
		(strings.Contains(ot, "hand") || strings.Contains(ot, "reveal")) &&
		!strings.Contains(ot, "battlefield") {
		flows = append(flows, ZoneFlow{From: "library", To: "hand", Resource: "card"})
	}

	// Token creation: nothing -> battlefield
	if strings.Contains(ot, "create") && strings.Contains(ot, "token") {
		resource := "creature"
		if strings.Contains(ot, "treasure") {
			resource = "mana"
		}
		flows = append(flows, ZoneFlow{From: "hand", To: "battlefield", Resource: resource})
	}

	// Sacrifice: battlefield -> graveyard
	if p.IsOutlet {
		flows = append(flows, ZoneFlow{From: "battlefield", To: "graveyard", Resource: "creature"})
	}

	// Landfall trigger: land ETB -> value (special: triggers from land entering)
	if profileHasTrigger(*p, "landfall") {
		flows = append(flows, ZoneFlow{From: "battlefield", To: "battlefield", Resource: "land"})
	}

	// ETB triggers: creature entering -> value
	if profileHasTrigger(*p, "etb") {
		flows = append(flows, ZoneFlow{From: "battlefield", To: "battlefield", Resource: "creature"})
	}

	// Death triggers: creature dying -> value
	if profileHasTrigger(*p, "dies") || profileHasTrigger(*p, "sacrifice") {
		flows = append(flows, ZoneFlow{From: "graveyard", To: "battlefield", Resource: "creature"})
	}

	// Blink: battlefield -> exile -> battlefield
	if p.IsBlinker {
		flows = append(flows, ZoneFlow{From: "battlefield", To: "battlefield", Resource: "creature"})
	}

	// Cost reduction / free cast: hand -> battlefield (cheap)
	if containsAny(ot, "costs {1} less", "costs {2} less", "costs {3} less",
		"cost {1} less", "cost {2} less", "without paying") {
		flows = append(flows, ZoneFlow{From: "hand", To: "battlefield", Resource: "any"})
	}

	// Exile play: exile -> hand/battlefield
	if strings.Contains(ot, "exile") && containsAny(ot, "you may cast", "you may play") {
		flows = append(flows, ZoneFlow{From: "exile", To: "hand", Resource: "card"})
	}

	// Cast triggers: hand -> stack -> value (enchantress draw, storm payoff, magecraft)
	if profileHasTrigger(*p, "cast") {
		flows = append(flows, ZoneFlow{From: "hand", To: "battlefield", Resource: "spell"})
	}

	// Counter placement: battlefield -> battlefield (counters as resource)
	if profileHasEffect(*p, "counter_add") || profileHasEffect(*p, "proliferate") {
		flows = append(flows, ZoneFlow{From: "battlefield", To: "battlefield", Resource: "counter"})
	}

	// Counter payoff: battlefield state -> value (cards that care about counters existing)
	if profileHasTrigger(*p, "counter_matters") || profileHasTrigger(*p, "counter_placed") {
		flows = append(flows, ZoneFlow{From: "battlefield", To: "battlefield", Resource: "counter_payoff"})
	}

	// Artifact sacrifice: battlefield -> graveyard (artifact-specific)
	if p.IsOutlet && strings.Contains(tl, "artifact") {
		flows = append(flows, ZoneFlow{From: "battlefield", To: "graveyard", Resource: "artifact"})
	}

	// Cycling source: cards with cycling keyword discard themselves
	// (hand → graveyard) to draw a card. The cycling cost is real mana;
	// the value lives in the trigger landscape (Astral Drift / Drake
	// Haven / Lightning Rift / Ominous Seas / Faith of the Devoted /
	// New Perspectives — every cycling deck runs ≥3 of these).
	//
	// Detection looks for the keyword "cycling" followed by either a
	// cost in braces ({2}) or a typeline (basiccycling) — both real
	// printings. The word "cycle" alone isn't enough (some cards just
	// REFERENCE cycling without having it). Reminder text inside
	// parens is already stripped upstream when ot comes from CleanForScan
	// callers; for raw-ot callers the substring "cycling {" is still
	// the right anchor because the reminder gloss says "{2}, Discard
	// this card: Draw a card." not "cycling {2}".
	if strings.Contains(ot, "cycling {") ||
		strings.Contains(ot, "basic landcycling") ||
		strings.Contains(ot, "typecycling") {
		flows = append(flows, ZoneFlow{From: "hand", To: "graveyard", Resource: "cycle_source"})
	}

	// Cycling payoff: cards that trigger when ANY card is cycled —
	// Astral Drift / Drake Haven / Lightning Rift / Faith of the Devoted
	// (whenever you cycle); Astral Slide / Decree of Annihilation
	// (whenever a player cycles); Ominous Seas (whenever you cycle or
	// discard).
	//
	// CRITICAL: "when you cycle this card" is a SELF-trigger on the
	// card's own cycling (Krosan Tusker, Eternal Dragon, Decree of
	// Justice) — that's a CYCLE_SOURCE behavior, not a battlefield-
	// resident payoff. We must NOT match it here. Hence the explicit
	// "whenever" / "you've cycled" anchors, plus a "when you cycle a"
	// (article-anchored, excludes "this card") fallback.
	cyclePayoff := strings.Contains(ot, "whenever you cycle") ||
		strings.Contains(ot, "whenever a player cycles") ||
		strings.Contains(ot, "when you cycle a ") ||
		strings.Contains(ot, "when you cycle or discard") ||
		strings.Contains(ot, "whenever you cycle or discard") ||
		strings.Contains(ot, "you've cycled")
	if cyclePayoff {
		flows = append(flows, ZoneFlow{From: "battlefield", To: "battlefield", Resource: "cycle_payoff"})
	}

	return flows
}

// ---------------------------------------------------------------------------
// Chain template definitions -- patterns we look for.
// ---------------------------------------------------------------------------

type chainTemplate struct {
	Name  string
	Steps []chainStepPattern
}

type chainStepPattern struct {
	Label    string
	From     string
	To       string
	Resource string // "" = any resource
}

var chainTemplates = []chainTemplate{
	{
		Name: "Landfall Engine",
		Steps: []chainStepPattern{
			{Label: "FILL", From: "library", To: "graveyard", Resource: ""},
			{Label: "RECOVER", From: "graveyard", To: "battlefield", Resource: "land"},
			{Label: "TRIGGER", From: "battlefield", To: "battlefield", Resource: "land"},
		},
	},
	{
		Name: "Aristocrats Engine",
		Steps: []chainStepPattern{
			{Label: "GENERATE", From: "hand", To: "battlefield", Resource: "creature"},
			{Label: "SACRIFICE", From: "battlefield", To: "graveyard", Resource: "creature"},
			{Label: "DRAIN", From: "graveyard", To: "battlefield", Resource: "creature"},
		},
	},
	{
		Name: "Reanimator Engine",
		Steps: []chainStepPattern{
			{Label: "FILL", From: "library", To: "graveyard", Resource: ""},
			{Label: "REANIMATE", From: "graveyard", To: "battlefield", Resource: "creature"},
			{Label: "VALUE", From: "battlefield", To: "battlefield", Resource: "creature"},
		},
	},
	{
		Name: "Graveyard Value Engine",
		Steps: []chainStepPattern{
			{Label: "FILL", From: "library", To: "graveyard", Resource: ""},
			{Label: "RECOVER", From: "graveyard", To: "hand", Resource: ""},
			{Label: "DEPLOY", From: "hand", To: "battlefield", Resource: ""},
		},
	},
	{
		Name: "Spellslinger Engine",
		Steps: []chainStepPattern{
			{Label: "CANTRIP", From: "library", To: "hand", Resource: "card"},
			{Label: "DISCOUNT", From: "hand", To: "battlefield", Resource: "any"},
		},
	},
	{
		Name: "Blink Value Engine",
		Steps: []chainStepPattern{
			{Label: "DEPLOY", From: "hand", To: "battlefield", Resource: "creature"},
			{Label: "BLINK", From: "battlefield", To: "battlefield", Resource: "creature"},
		},
	},
	{
		Name: "Storm Engine",
		Steps: []chainStepPattern{
			{Label: "DISCOUNT", From: "hand", To: "battlefield", Resource: "any"},
			{Label: "CANTRIP", From: "library", To: "hand", Resource: "card"},
			{Label: "PAYOFF", From: "hand", To: "battlefield", Resource: "spell"},
		},
	},
	{
		Name: "Artifact Engine",
		Steps: []chainStepPattern{
			{Label: "DEPLOY", From: "hand", To: "battlefield", Resource: "creature"},
			{Label: "SACRIFICE", From: "battlefield", To: "graveyard", Resource: "artifact"},
			{Label: "RECOVER", From: "graveyard", To: "battlefield", Resource: ""},
		},
	},
	{
		Name: "Enchantress Engine",
		Steps: []chainStepPattern{
			{Label: "DRAW", From: "hand", To: "battlefield", Resource: "spell"},
			{Label: "REFUEL", From: "library", To: "hand", Resource: "card"},
		},
	},
	{
		Name: "Counters Matter Engine",
		Steps: []chainStepPattern{
			{Label: "PLACE", From: "battlefield", To: "battlefield", Resource: "counter"},
			{Label: "PAYOFF", From: "battlefield", To: "battlefield", Resource: "counter_payoff"},
		},
	},
	{
		// Cycling Engine (R60). The chain is a tight 2-step pipeline:
		// hand → graveyard via cycling cost, then battlefield-resident
		// payoff observes the cycle event. Drake Haven / Astral Drift /
		// Lightning Rift / Ominous Seas / Faith of the Devoted / New
		// Perspectives are the canonical payoffs; any card with a
		// cycling cost feeds the source step.
		//
		// Like Spellslinger and Storm engines, the deck wins through
		// REPEATED execution of the chain — one Astral Drift trigger
		// is a Cloudshift; a chain of 10 cycling triggers in one turn
		// is a board reset + lethal swing. The redundancy rating in
		// matchChainTemplate captures how many of each step a deck
		// runs (a 5-source/2-payoff deck is MEDIUM-redundancy;
		// 12-source/4-payoff is HIGH).
		Name: "Cycling Engine",
		Steps: []chainStepPattern{
			{Label: "CYCLE", From: "hand", To: "graveyard", Resource: "cycle_source"},
			{Label: "PAYOFF", From: "battlefield", To: "battlefield", Resource: "cycle_payoff"},
		},
	},
}

// ---------------------------------------------------------------------------
// DetectValueChains -- find multi-step resource pipelines in a deck.
// ---------------------------------------------------------------------------

func DetectValueChains(profiles []CardProfile) []ValueChain {
	var chains []ValueChain

	for _, tmpl := range chainTemplates {
		chain := matchChainTemplate(tmpl, profiles)
		if chain == nil {
			continue
		}
		chains = append(chains, *chain)
	}

	sort.Slice(chains, func(i, j int) bool {
		ti := 0
		for _, s := range chains[i].Steps {
			ti += len(s.Cards)
		}
		tj := 0
		for _, s := range chains[j].Steps {
			tj += len(s.Cards)
		}
		return ti > tj
	})

	return chains
}

func matchChainTemplate(tmpl chainTemplate, profiles []CardProfile) *ValueChain {
	steps := make([]ValueChainStep, len(tmpl.Steps))
	for i, pat := range tmpl.Steps {
		steps[i] = ValueChainStep{
			Label: pat.Label,
			Flow: ZoneFlow{
				From:     pat.From,
				To:       pat.To,
				Resource: pat.Resource,
			},
		}
	}

	// Find cards that match each step.
	for _, p := range profiles {
		if p.IsLand {
			continue
		}
		for si, pat := range tmpl.Steps {
			if cardMatchesStep(p, pat) {
				steps[si].Cards = append(steps[si].Cards, p.Name)
			}
		}
	}

	// Every step must have at least 1 card.
	totalPieces := 0
	for _, s := range steps {
		if len(s.Cards) == 0 {
			return nil
		}
		totalPieces += len(s.Cards)
	}

	// Require meaningful depth: at least 3 total pieces for a 2-step chain,
	// 4 for a 3-step chain.
	minPieces := len(steps) + 1
	if totalPieces < minPieces {
		return nil
	}

	// Find weakest link.
	weakest := 0
	weakestCount := len(steps[0].Cards)
	for i, s := range steps {
		if len(s.Cards) < weakestCount {
			weakest = i
			weakestCount = len(s.Cards)
		}
	}

	// Redundancy rating.
	redundancy := "LOW"
	if weakestCount >= 6 {
		redundancy = "HIGH"
	} else if weakestCount >= 3 {
		redundancy = "MEDIUM"
	}

	// Find bridge cards (cards that appear in 2+ steps).
	cardStepCount := map[string]int{}
	for _, s := range steps {
		for _, name := range s.Cards {
			cardStepCount[name]++
		}
	}
	var bridges []string
	for name, count := range cardStepCount {
		if count >= 2 {
			bridges = append(bridges, name)
		}
	}
	sort.Strings(bridges)

	// Deduplicate card lists within each step.
	for i := range steps {
		steps[i].Cards = uniqueStrings(steps[i].Cards)
	}

	// Recursion depth: check if the chain forms a loop (last step's To == first step's From).
	recursionDepth := "none"
	lastStep := tmpl.Steps[len(tmpl.Steps)-1]
	firstStep := tmpl.Steps[0]
	if lastStep.To == firstStep.From {
		recursionDepth = "infinite"
	} else if len(bridges) >= 3 {
		recursionDepth = "deep"
	} else if len(bridges) >= 1 {
		recursionDepth = "shallow"
	}

	chain := &ValueChain{
		Name:           tmpl.Name,
		Steps:          steps,
		BridgeCards:    bridges,
		Depth:          len(steps),
		WeakestLink:    weakest,
		Redundancy:     redundancy,
		RecursionDepth: recursionDepth,
	}
	chain.Rationale = buildValueChainRationale(chain)
	return chain
}

// engineRationaleByName describes what triggers each engine and how it produces value.
// Keys match chainTemplate.Name verbatim.
var engineRationaleByName = map[string]struct {
	Trigger    string
	HowItWorks string
}{
	"Landfall Engine": {
		Trigger:    "lands entering the battlefield from any zone (drops, ramp, recursion)",
		HowItWorks: "Mill or ramp lands into the graveyard, recur them to the battlefield, and bank the landfall trigger each time. Repeat for compounding value per turn.",
	},
	"Aristocrats Engine": {
		Trigger:    "creatures dying or being sacrificed",
		HowItWorks: "Generate creature bodies (tokens or recursion), feed them to a sacrifice outlet, and collect death/drain payoffs. The graveyard refills the loop via reanimation.",
	},
	"Reanimator Engine": {
		Trigger:    "fat creatures hitting the graveyard ahead of their natural mana cost",
		HowItWorks: "Self-mill or discard a high-impact creature, then cheat it into play with reanimation. Pay off the early threat with combat, ETB, or attack triggers.",
	},
	"Graveyard Value Engine": {
		Trigger:    "cards leaving hand or library and landing in the graveyard",
		HowItWorks: "Use mill/discard/loot to load the graveyard, recur cards back to hand, and recast them. Each rotation generates incremental tempo and card advantage.",
	},
	"Spellslinger Engine": {
		Trigger:    "casting noncreature spells",
		HowItWorks: "Cantrips refill the hand for cheap; cost reducers and rituals turn surplus mana into more spells. The chain wins through prowess, storm, or magecraft payoffs.",
	},
	"Blink Value Engine": {
		Trigger:    "creatures with ETB triggers entering the battlefield",
		HowItWorks: "Deploy ETB creatures, then exile-and-return them with blink effects. Each flicker re-fires the ETB, so a few high-value bodies snowball into virtual card advantage.",
	},
	"Storm Engine": {
		Trigger:    "casting many spells in a single turn",
		HowItWorks: "Cost reducers and rituals make spells effectively free; cantrips chain into more spells; a storm payoff converts the spell count into a lethal trigger.",
	},
	"Artifact Engine": {
		Trigger:    "artifacts entering the battlefield, being sacrificed, or being recurred",
		HowItWorks: "Cheap artifacts drop, get sacrificed for value (mana, draw, damage), and return from the graveyard. Density of triggers turns each cycle into compounding card advantage.",
	},
	"Enchantress Engine": {
		Trigger:    "enchantments entering the battlefield",
		HowItWorks: "Each enchantment played triggers a draw from one of several enchantress effects, refilling the hand and powering the next enchantment. The chain self-sustains as long as the engine creature survives.",
	},
	"Counters Matter Engine": {
		Trigger:    "+1/+1 (or other) counters being placed",
		HowItWorks: "Place counters via spells, ETBs, or proliferate; a payoff card converts counter density into draws, damage, or scaling threats. Proliferate accelerates the loop without spending more cards.",
	},
	"Cycling Engine": {
		Trigger:    "cycling a card from hand (discard for {2} or its cycling cost)",
		HowItWorks: "Pay the cycling cost to discard a card and draw one. Each cycle fires every cycling-payoff permanent in play: Astral Drift exiles + returns a creature, Drake Haven makes a 2/2 flier, Lightning Rift deals 2 damage, Ominous Seas adds a counter, Faith of the Devoted drains 2. A deck running 10-15 cyclers + 3-4 payoffs converts every cycle into a board-wipe-plus-tempo turn.",
	},
}

func buildValueChainRationale(chain *ValueChain) *ValueChainRationale {
	r := &ValueChainRationale{}
	if base, ok := engineRationaleByName[chain.Name]; ok {
		r.Trigger = base.Trigger
		r.HowItWorks = base.HowItWorks
	} else {
		r.Trigger = "deck-specific resource flow detected"
		r.HowItWorks = fmt.Sprintf("Multi-step pipeline (%d steps) across deck zones.", chain.Depth)
	}

	// Key pieces: bridges first (they span multiple steps), then the strongest
	// step's first card so a reader can see at least one anchor per chain.
	seen := map[string]bool{}
	for _, b := range chain.BridgeCards {
		if !seen[b] {
			r.KeyPieces = append(r.KeyPieces, b)
			seen[b] = true
		}
	}
	if len(r.KeyPieces) < 3 {
		// Add the first card of each step to give callers a representative
		// example until we hit 3 anchors.
		for _, step := range chain.Steps {
			for _, c := range step.Cards {
				if !seen[c] {
					r.KeyPieces = append(r.KeyPieces, c)
					seen[c] = true
					break
				}
			}
			if len(r.KeyPieces) >= 3 {
				break
			}
		}
	}
	return r
}

func cardMatchesStep(p CardProfile, pat chainStepPattern) bool {
	for _, flow := range p.ZoneFlows {
		if flow.From != pat.From || flow.To != pat.To {
			continue
		}
		if pat.Resource != "" && flow.Resource != pat.Resource && flow.Resource != "any" {
			continue
		}
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Bridge card annotation -- show which steps a bridge spans.
// ---------------------------------------------------------------------------

func bridgeAnnotation(name string, steps []ValueChainStep) string {
	var labels []string
	for _, s := range steps {
		for _, c := range s.Cards {
			if c == name {
				labels = append(labels, s.Label)
				break
			}
		}
	}
	return strings.Join(labels, "+")
}

// ---------------------------------------------------------------------------
// Report rendering -- value chains in all 3 formats.
// ---------------------------------------------------------------------------

func renderValueChainsText(chains []ValueChain) string {
	if len(chains) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("VALUE CHAINS\n")
	sb.WriteString("============\n")

	for _, chain := range chains {
		totalPieces := 0
		for _, s := range chain.Steps {
			totalPieces += len(s.Cards)
		}
		sb.WriteString(fmt.Sprintf("%s (%d steps, %d pieces)\n", chain.Name, chain.Depth, totalPieces))
		weakLabel := chain.Steps[chain.WeakestLink].Label
		weakCount := len(chain.Steps[chain.WeakestLink].Cards)
		sb.WriteString(fmt.Sprintf("  Depth: %d | Weakest link: Step %d - %s (%d pieces) | Redundancy: %s | Recursion: %s\n\n",
			chain.Depth, chain.WeakestLink+1, weakLabel, weakCount, chain.Redundancy, chain.RecursionDepth))

		for _, step := range chain.Steps {
			flow := fmt.Sprintf("%s→%s", capitalize(step.Flow.From), capitalize(step.Flow.To))
			cards := strings.Join(step.Cards, ", ")
			if len(cards) > 80 {
				cards = cards[:77] + "..."
			}
			sb.WriteString(fmt.Sprintf("  [%-8s] %s: %s\n", step.Label, flow, cards))
		}

		if len(chain.BridgeCards) > 0 {
			var annotated []string
			for _, b := range chain.BridgeCards {
				annotated = append(annotated, fmt.Sprintf("%s (%s)", b, bridgeAnnotation(b, chain.Steps)))
			}
			sb.WriteString(fmt.Sprintf("\n  Bridge cards: %s\n", strings.Join(annotated, ", ")))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func renderValueChainsMarkdown(chains []ValueChain) string {
	if len(chains) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Value Chains\n\n")

	for _, chain := range chains {
		totalPieces := 0
		for _, s := range chain.Steps {
			totalPieces += len(s.Cards)
		}
		sb.WriteString(fmt.Sprintf("### %s (%d steps, %d pieces)\n\n", chain.Name, chain.Depth, totalPieces))
		weakLabel := chain.Steps[chain.WeakestLink].Label
		weakCount := len(chain.Steps[chain.WeakestLink].Cards)
		sb.WriteString(fmt.Sprintf("**Depth:** %d | **Weakest link:** Step %d - %s (%d pieces) | **Redundancy:** %s | **Recursion:** %s\n\n",
			chain.Depth, chain.WeakestLink+1, weakLabel, weakCount, chain.Redundancy, chain.RecursionDepth))

		for _, step := range chain.Steps {
			flow := fmt.Sprintf("%s→%s", capitalize(step.Flow.From), capitalize(step.Flow.To))
			sb.WriteString(fmt.Sprintf("- **[%s]** %s: %s\n", step.Label, flow, strings.Join(step.Cards, ", ")))
		}
		sb.WriteString("\n")

		if len(chain.BridgeCards) > 0 {
			var annotated []string
			for _, b := range chain.BridgeCards {
				annotated = append(annotated, fmt.Sprintf("**%s** (%s)", b, bridgeAnnotation(b, chain.Steps)))
			}
			sb.WriteString(fmt.Sprintf("**Bridge cards:** %s\n\n", strings.Join(annotated, ", ")))
		}
	}

	return sb.String()
}

// ---------------------------------------------------------------------------
// JSON types for value chains.
// ---------------------------------------------------------------------------

type jsonValueChain struct {
	Name           string                   `json:"name"`
	Steps          []jsonValueChainStep     `json:"steps"`
	BridgeCards    []string                 `json:"bridge_cards,omitempty"`
	Depth          int                      `json:"depth"`
	WeakestLink    int                      `json:"weakest_link"`
	Redundancy     string                   `json:"redundancy"`
	RecursionDepth string                   `json:"recursion_depth"`
	Rationale      *jsonValueChainRationale `json:"rationale,omitempty"`
}

type jsonValueChainRationale struct {
	Trigger    string   `json:"trigger,omitempty"`
	HowItWorks string   `json:"how_it_works,omitempty"`
	KeyPieces  []string `json:"key_pieces,omitempty"`
}

type jsonValueChainStep struct {
	Label    string   `json:"label"`
	From     string   `json:"from"`
	To       string   `json:"to"`
	Resource string   `json:"resource,omitempty"`
	Cards    []string `json:"cards"`
}

func buildJSONValueChains(chains []ValueChain) []jsonValueChain {
	if len(chains) == 0 {
		return nil
	}
	out := make([]jsonValueChain, len(chains))
	for i, c := range chains {
		steps := make([]jsonValueChainStep, len(c.Steps))
		for j, s := range c.Steps {
			steps[j] = jsonValueChainStep{
				Label:    s.Label,
				From:     s.Flow.From,
				To:       s.Flow.To,
				Resource: s.Flow.Resource,
				Cards:    s.Cards,
			}
		}
		var rat *jsonValueChainRationale
		if c.Rationale != nil {
			rat = &jsonValueChainRationale{
				Trigger:    c.Rationale.Trigger,
				HowItWorks: c.Rationale.HowItWorks,
				KeyPieces:  c.Rationale.KeyPieces,
			}
		}
		out[i] = jsonValueChain{
			Name:           c.Name,
			Steps:          steps,
			BridgeCards:    c.BridgeCards,
			Depth:          c.Depth,
			WeakestLink:    c.WeakestLink,
			Redundancy:     c.Redundancy,
			RecursionDepth: c.RecursionDepth,
			Rationale:      rat,
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
