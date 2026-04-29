package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type WinLine struct {
	Pieces     []string
	Type       string // "infinite", "determined", "finisher", "combat", "commander_damage", "alt_wincon"
	Desc       string
	TutorPaths []TutorChain
}

type TutorChain struct {
	Tutor      string
	Finds      string
	Delivery   string // "hand", "top", "battlefield", "graveyard"
	Restricted string // "creature", "artifact", "enchantment", "instant_sorcery", "land", "any"
}

type WinLineAnalysis struct {
	WinLines             []WinLine
	TutorMap             []TutorInfo
	BackupPlans          []string
	SinglePoints         []string
	RedundancyMap        map[string]int
}

type TutorInfo struct {
	Name       string
	Restricted string
	Delivery   string
}

type tutorSpec struct {
	restricted string
	delivery   string
}

var knownTutors = map[string]tutorSpec{
	"demonic tutor":            {"any", "hand"},
	"vampiric tutor":           {"any", "top"},
	"imperial seal":            {"any", "top"},
	"grim tutor":               {"any", "hand"},
	"diabolic tutor":           {"any", "hand"},
	"diabolic intent":          {"any", "hand"},
	"beseech the mirror":       {"any", "hand"},
	"wishclaw talisman":        {"any", "hand"},
	"scheming symmetry":        {"any", "top"},
	"final parting":            {"any", "hand"},
	"razaketh, the foulblooded": {"any", "hand"},

	"enlightened tutor":        {"artifact_enchantment", "top"},
	"idyllic tutor":            {"enchantment", "hand"},
	"academy rector":           {"enchantment", "battlefield"},
	"zur the enchanter":        {"enchantment", "battlefield"},

	"worldly tutor":            {"creature", "top"},
	"sylvan tutor":             {"creature", "top"},
	"green sun's zenith":       {"creature", "battlefield"},
	"chord of calling":         {"creature", "battlefield"},
	"eldritch evolution":       {"creature", "battlefield"},
	"neoform":                  {"creature", "battlefield"},
	"birthing pod":             {"creature", "battlefield"},
	"survival of the fittest":  {"creature", "hand"},
	"fierce empath":            {"creature", "hand"},
	"fauna shaman":             {"creature", "hand"},
	"eladamri's call":          {"creature", "hand"},
	"finale of devastation":    {"creature", "battlefield"},
	"natural order":            {"creature", "battlefield"},
	"yisan, the wanderer bard": {"creature", "battlefield"},

	"mystical tutor":           {"instant_sorcery", "top"},
	"personal tutor":           {"sorcery", "top"},
	"merchant scroll":          {"instant", "hand"},
	"spellseeker":              {"instant_sorcery", "hand"},
	"solve the equation":       {"instant_sorcery", "hand"},

	"fabricate":                {"artifact", "hand"},
	"trinket mage":             {"artifact", "hand"},
	"trophy mage":              {"artifact", "hand"},
	"tribute mage":             {"artifact", "hand"},
	"treasure mage":            {"artifact", "hand"},
	"whir of invention":        {"artifact", "battlefield"},
	"tezzeret the seeker":      {"artifact", "battlefield"},
	"urza's saga":              {"artifact", "hand"},
	"inventors' fair":          {"artifact", "hand"},
	"transmute artifact":       {"artifact", "hand"},
	"muddle the mixture":       {"cmc2", "hand"},
	"drift of phantasms":       {"cmc3", "hand"},
	"perplex":                  {"cmc3", "hand"},
	"dimir infiltrator":        {"cmc2", "hand"},
	"shred memory":             {"cmc2", "hand"},
	"clutch of the undercity":  {"cmc4", "hand"},
	"brainspoil":               {"cmc5", "hand"},
	"netherborn altar":         {"cmc2", "hand"},
	"tolaria west":             {"cmc0", "hand"},

	"crop rotation":            {"land", "battlefield"},
	"expedition map":           {"land", "hand"},
	"sylvan scrying":           {"land", "hand"},
	"knight of the reliquary":  {"land", "battlefield"},
	"tempt with discovery":     {"land", "battlefield"},
	"hour of promise":          {"land", "battlefield"},
	"reap and sow":             {"land", "battlefield"},

	"entomb":                   {"any", "graveyard"},
	"buried alive":             {"creature", "graveyard"},
	"intuition":                {"any", "hand"},
	"gifts ungiven":            {"any", "hand"},
	"gamble":                   {"any", "hand"},

	"cultivate":                {"land", "hand"},
	"kodama's reach":           {"land", "hand"},
	"farseek":                  {"land", "battlefield"},
	"nature's lore":            {"land", "battlefield"},
	"three visits":             {"land", "battlefield"},
	"rampant growth":           {"land", "battlefield"},
	"sakura-tribe elder":       {"land", "battlefield"},
	"wood elves":               {"land", "battlefield"},
	"farhaven elf":             {"land", "battlefield"},
	"solemn simulacrum":        {"land", "battlefield"},
	"burnished hart":           {"land", "battlefield"},
	"explosive vegetation":     {"land", "battlefield"},
	"skyshroud claim":          {"land", "battlefield"},
	"migration path":           {"land", "battlefield"},
	"circuitous route":         {"land", "battlefield"},
	"primeval titan":           {"land", "battlefield"},
	"oracle of mul daya":       {"land", "battlefield"},
	"springbloom druid":        {"land", "battlefield"},

	"flooded strand":           {"land", "battlefield"},
	"polluted delta":           {"land", "battlefield"},
	"windswept heath":          {"land", "battlefield"},
	"wooded foothills":         {"land", "battlefield"},
	"bloodstained mire":        {"land", "battlefield"},
	"scalding tarn":            {"land", "battlefield"},
	"verdant catacombs":        {"land", "battlefield"},
	"misty rainforest":         {"land", "battlefield"},
	"arid mesa":                {"land", "battlefield"},
	"marsh flats":              {"land", "battlefield"},
	"prismatic vista":          {"land", "battlefield"},
	"fabled passage":           {"land", "battlefield"},
	"terramorphic expanse":     {"land", "battlefield"},
	"evolving wilds":           {"land", "battlefield"},
}

func ComputeWinLines(report *FreyaReport, qtyProfiles []CardProfileQty, oracle *oracleDB) *WinLineAnalysis {
	wla := &WinLineAnalysis{
		RedundancyMap: map[string]int{},
	}

	profileByName := map[string]CardProfile{}
	typeByName := map[string]string{}
	for _, qp := range qtyProfiles {
		profileByName[qp.Profile.Name] = qp.Profile
		if oracle != nil {
			entry := oracle.lookup(qp.Profile.Name)
			if entry != nil {
				typeByName[qp.Profile.Name] = strings.ToLower(entry.TypeLine)
			}
		}
	}

	allTutors := buildTutorMap(qtyProfiles, oracle)
	wla.TutorMap = allTutors

	var winTutors []TutorInfo
	for _, t := range allTutors {
		if t.Restricted != "land" {
			winTutors = append(winTutors, t)
		}
	}

	addComboWinLines(wla, report.TrueInfinites, "infinite", winTutors, typeByName, oracle)
	addComboWinLines(wla, report.Determined, "determined", winTutors, typeByName, oracle)
	addComboWinLines(wla, report.Finishers, "finisher", winTutors, typeByName, oracle)
	addNonComboWinLines(wla, report, qtyProfiles, oracle, profileByName)

	sort.Slice(wla.WinLines, func(i, j int) bool {
		return winLinePriority(wla.WinLines[i].Type) < winLinePriority(wla.WinLines[j].Type)
	})

	computeBackupPlans(wla)
	computeRedundancy(wla, report, profileByName)
	computeSinglePoints(wla, profileByName)

	return wla
}

func winLinePriority(t string) int {
	switch t {
	case "infinite":
		return 0
	case "determined":
		return 1
	case "finisher":
		return 2
	case "alt_wincon":
		return 3
	case "combat":
		return 4
	case "commander_damage":
		return 5
	default:
		return 6
	}
}

func buildTutorMap(qtyProfiles []CardProfileQty, oracle *oracleDB) []TutorInfo {
	var tutors []TutorInfo
	seen := map[string]bool{}
	for _, qp := range qtyProfiles {
		if !qp.Profile.IsTutor || qp.Profile.IsLand {
			continue
		}
		name := qp.Profile.Name
		if seen[name] {
			continue
		}
		seen[name] = true

		nameLower := strings.ToLower(name)
		if spec, ok := knownTutors[nameLower]; ok {
			tutors = append(tutors, TutorInfo{
				Name:       name,
				Restricted: spec.restricted,
				Delivery:   spec.delivery,
			})
			continue
		}

		var ot, tl string
		if oracle != nil {
			entry := oracle.lookup(name)
			if entry != nil {
				ot = strings.ToLower(entry.OracleText)
				tl = strings.ToLower(entry.TypeLine)
				if ot == "" && len(entry.CardFaces) > 0 {
					ot = entry.CardFaces[0].OracleText
					tl = entry.CardFaces[0].TypeLine
					ot = strings.ToLower(ot)
					tl = strings.ToLower(tl)
				}
			}
		}

		restricted, delivery := inferTutorRestriction(ot, tl)
		tutors = append(tutors, TutorInfo{
			Name:       name,
			Restricted: restricted,
			Delivery:   delivery,
		})
	}
	return tutors
}

func inferTutorRestriction(ot, tl string) (string, string) {
	restricted := "any"
	delivery := "hand"

	if strings.Contains(ot, "transmute") {
		for cmc := 0; cmc <= 9; cmc++ {
			if strings.Contains(ot, fmt.Sprintf("mana value %d", cmc)) ||
				strings.Contains(ot, fmt.Sprintf("converted mana cost %d", cmc)) {
				return fmt.Sprintf("cmc%d", cmc), "hand"
			}
		}
	}

	if strings.Contains(ot, "search your library for a land") ||
		strings.Contains(ot, "search your library for a basic land") ||
		strings.Contains(ot, "search your library for up to two basic land") ||
		strings.Contains(ot, "search your library for a plains") ||
		strings.Contains(ot, "search your library for a forest") ||
		strings.Contains(ot, "search your library for a card with a basic land type") ||
		(strings.Contains(ot, "search your library") && strings.Contains(ot, "land card") &&
			!strings.Contains(ot, "nonland")) {
		restricted = "land"
	} else if strings.Contains(ot, "search your library for a creature") {
		restricted = "creature"
	} else if strings.Contains(ot, "search your library for an artifact") {
		restricted = "artifact"
	} else if strings.Contains(ot, "search your library for an enchantment") {
		restricted = "enchantment"
	} else if strings.Contains(ot, "search your library for an instant") ||
		strings.Contains(ot, "search your library for a sorcery") {
		restricted = "instant_sorcery"
	}

	if strings.Contains(ot, "put it on top") || strings.Contains(ot, "on top of your library") {
		delivery = "top"
	} else if strings.Contains(ot, "onto the battlefield") || strings.Contains(ot, "put it onto the battlefield") ||
		strings.Contains(ot, "put that card onto the battlefield") {
		delivery = "battlefield"
	} else if strings.Contains(ot, "into your graveyard") || strings.Contains(ot, "put it into your graveyard") {
		delivery = "graveyard"
	}

	return restricted, delivery
}

func tutorCanFind(tutor TutorInfo, cardName string, typeByName map[string]string, oracle *oracleDB) bool {
	if tutor.Restricted == "any" {
		return true
	}
	tl := typeByName[cardName]
	if tl == "" && oracle != nil {
		entry := oracle.lookup(cardName)
		if entry != nil {
			tl = strings.ToLower(entry.TypeLine)
		}
	}

	switch tutor.Restricted {
	case "creature":
		return strings.Contains(tl, "creature")
	case "artifact":
		return strings.Contains(tl, "artifact")
	case "enchantment":
		return strings.Contains(tl, "enchantment")
	case "artifact_enchantment":
		return strings.Contains(tl, "artifact") || strings.Contains(tl, "enchantment")
	case "instant_sorcery":
		return strings.Contains(tl, "instant") || strings.Contains(tl, "sorcery")
	case "instant":
		return strings.Contains(tl, "instant")
	case "sorcery":
		return strings.Contains(tl, "sorcery")
	case "land":
		return strings.Contains(tl, "land")
	case "cmc0", "cmc1", "cmc2", "cmc3", "cmc4", "cmc5", "cmc6", "cmc7":
		target, _ := strconv.Atoi(tutor.Restricted[3:])
		if oracle != nil {
			entry := oracle.lookup(cardName)
			if entry != nil {
				return int(entry.CMC) == target
			}
		}
		return false
	}
	return false
}

func addComboWinLines(wla *WinLineAnalysis, combos []ComboResult, lineType string, tutors []TutorInfo, typeByName map[string]string, oracle *oracleDB) {
	for _, combo := range combos {
		wl := WinLine{
			Pieces: combo.Cards,
			Type:   lineType,
			Desc:   combo.Description,
		}

		for _, piece := range combo.Cards {
			for _, tutor := range tutors {
				if tutorCanFind(tutor, piece, typeByName, oracle) {
					wl.TutorPaths = append(wl.TutorPaths, TutorChain{
						Tutor:      tutor.Name,
						Finds:      piece,
						Delivery:   tutor.Delivery,
						Restricted: tutor.Restricted,
					})
				}
			}
		}

		wla.WinLines = append(wla.WinLines, wl)
	}
}

func addNonComboWinLines(wla *WinLineAnalysis, report *FreyaReport, qtyProfiles []CardProfileQty, oracle *oracleDB, profileByName map[string]CardProfile) {
	for _, qp := range qtyProfiles {
		if qp.Profile.IsLand {
			continue
		}
		p := qp.Profile

		var ot string
		if oracle != nil {
			entry := oracle.lookup(p.Name)
			if entry != nil {
				ot = strings.ToLower(entry.OracleText)
				if ot == "" && len(entry.CardFaces) > 0 {
					ot = strings.ToLower(entry.CardFaces[0].OracleText)
				}
			}
		}

		if p.IsWinCon && !alreadyInWinLines(wla, p.Name) {
			wla.WinLines = append(wla.WinLines, WinLine{
				Pieces: []string{p.Name},
				Type:   "alt_wincon",
				Desc:   "alternate win condition",
			})
		}
	}

	if report.Commander != "" {
		wla.WinLines = append(wla.WinLines, WinLine{
			Pieces: []string{report.Commander},
			Type:   "commander_damage",
			Desc:   "21 commander damage",
		})
	}

	threatCount := 0
	pumpCount := 0
	for _, qp := range qtyProfiles {
		if qp.Profile.IsLand {
			continue
		}
		tl := strings.ToLower(qp.Profile.TypeLine)
		if strings.Contains(tl, "creature") && qp.Profile.CMC >= 3 {
			threatCount += qp.Qty
		}
		var ot string
		if oracle != nil {
			entry := oracle.lookup(qp.Profile.Name)
			if entry != nil {
				ot = strings.ToLower(entry.OracleText)
			}
		}
		if containsAny(ot, "creatures you control get", "all creatures get", "overrun", "trample") {
			pumpCount += qp.Qty
		}
	}
	if threatCount >= 10 || pumpCount >= 2 {
		wla.WinLines = append(wla.WinLines, WinLine{
			Pieces: []string{fmt.Sprintf("%d threats + %d pumps", threatCount, pumpCount)},
			Type:   "combat",
			Desc:   "combat damage with creature pressure",
		})
	}
}

func alreadyInWinLines(wla *WinLineAnalysis, name string) bool {
	for _, wl := range wla.WinLines {
		for _, p := range wl.Pieces {
			if p == name {
				return true
			}
		}
	}
	return false
}

func computeBackupPlans(wla *WinLineAnalysis) {
	if len(wla.WinLines) <= 1 {
		return
	}

	tutorCoverage := map[int]int{}
	for i, wl := range wla.WinLines {
		tutorCoverage[i] = len(wl.TutorPaths)
	}

	bestIdx := 0
	bestCount := 0
	for i, count := range tutorCoverage {
		if count > bestCount {
			bestCount = count
			bestIdx = i
		}
	}

	for i, wl := range wla.WinLines {
		if i == bestIdx {
			continue
		}
		label := strings.Join(wl.Pieces, " + ")
		if wl.Type == "combat" || wl.Type == "commander_damage" {
			wla.BackupPlans = append(wla.BackupPlans,
				fmt.Sprintf("Backup: %s (%s)", label, wl.Type))
		} else {
			paths := tutorCoverage[i]
			if paths > 0 {
				wla.BackupPlans = append(wla.BackupPlans,
					fmt.Sprintf("Backup: %s (%d tutor paths)", label, paths))
			} else {
				wla.BackupPlans = append(wla.BackupPlans,
					fmt.Sprintf("Backup: %s (no tutor access — must draw naturally)", label))
			}
		}
	}
}

func computeRedundancy(wla *WinLineAnalysis, report *FreyaReport, profileByName map[string]CardProfile) {
	for _, p := range profileByName {
		if p.IsLand {
			continue
		}
		if p.IsWinCon {
			wla.RedundancyMap["win_condition"]++
		}
		if p.IsOutlet {
			wla.RedundancyMap["sacrifice_outlet"]++
		}
		if p.IsTutor {
			wla.RedundancyMap["tutor"]++
		}
		if p.IsMassWipe {
			wla.RedundancyMap["board_wipe"]++
		}
		for _, r := range p.Produces {
			if r == ResCard {
				wla.RedundancyMap["draw_engine"]++
				break
			}
		}
		for _, r := range p.Produces {
			if r == ResMana {
				wla.RedundancyMap["mana_source"]++
				break
			}
		}
	}
}

func computeSinglePoints(wla *WinLineAnalysis, profileByName map[string]CardProfile) {
	for _, wl := range wla.WinLines {
		if wl.Type == "combat" || wl.Type == "commander_damage" {
			continue
		}
		for _, piece := range wl.Pieces {
			if len(wl.TutorPaths) == 0 {
				continue
			}
			hasTutor := false
			for _, tp := range wl.TutorPaths {
				if tp.Finds == piece {
					hasTutor = true
					break
				}
			}
			if !hasTutor {
				wla.SinglePoints = append(wla.SinglePoints,
					fmt.Sprintf("%s is needed for [%s] but no tutor in the deck can find it",
						piece, strings.Join(wl.Pieces, " + ")))
			}
		}
	}

	wla.SinglePoints = uniqueStrings(wla.SinglePoints)

	if wla.RedundancyMap["win_condition"] == 1 {
		wla.SinglePoints = append(wla.SinglePoints,
			"only 1 card tagged as win condition — if exiled, no backup win con")
	}
	if wla.RedundancyMap["sacrifice_outlet"] == 1 {
		wla.SinglePoints = append(wla.SinglePoints,
			"only 1 sacrifice outlet — if removed, combo lines that need sac are dead")
	}
}
