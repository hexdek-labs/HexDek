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
	Name        string
	Steps       []ValueChainStep
	BridgeCards []string
	Depth       int
	WeakestLink int
	Redundancy  string // "HIGH", "MEDIUM", "LOW"
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

	return &ValueChain{
		Name:        tmpl.Name,
		Steps:       steps,
		BridgeCards: bridges,
		Depth:       len(steps),
		WeakestLink: weakest,
		Redundancy:  redundancy,
	}
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
		sb.WriteString(fmt.Sprintf("  Depth: %d | Weakest link: Step %d - %s (%d pieces) | Redundancy: %s\n\n",
			chain.Depth, chain.WeakestLink+1, weakLabel, weakCount, chain.Redundancy))

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
		sb.WriteString(fmt.Sprintf("**Depth:** %d | **Weakest link:** Step %d - %s (%d pieces) | **Redundancy:** %s\n\n",
			chain.Depth, chain.WeakestLink+1, weakLabel, weakCount, chain.Redundancy))

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
	Name        string               `json:"name"`
	Steps       []jsonValueChainStep `json:"steps"`
	BridgeCards []string             `json:"bridge_cards,omitempty"`
	Depth       int                  `json:"depth"`
	WeakestLink int                  `json:"weakest_link"`
	Redundancy  string               `json:"redundancy"`
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
		out[i] = jsonValueChain{
			Name:        c.Name,
			Steps:       steps,
			BridgeCards: c.BridgeCards,
			Depth:       c.Depth,
			WeakestLink: c.WeakestLink,
			Redundancy:  c.Redundancy,
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
