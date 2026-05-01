package hat

import (
	"math"
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// GameStateEvaluator scores a game state from one seat's perspective.
// Returns a value in [-1, +1] where +1 is winning and -1 is losing.
// Used by MCTS rollout evaluation and PokerHat re-evaluate.
type GameStateEvaluator struct {
	Weights  EvalWeights
	Strategy *StrategyProfile
}

// EvalResult holds the per-dimension breakdown alongside the final score.
type EvalResult struct {
	Score             float64
	BoardPresence     float64
	CardAdvantage     float64
	ManaAdvantage     float64
	LifeResource      float64
	ComboProximity    float64
	ThreatExposure    float64
	CommanderProgress float64
	GraveyardValue    float64
	DrainEngine       float64
}

// NewEvaluator constructs an evaluator from a strategy profile. If sp is
// nil, uses midrange defaults.
func NewEvaluator(sp *StrategyProfile) *GameStateEvaluator {
	e := &GameStateEvaluator{Strategy: sp}
	if sp != nil && sp.Weights != nil {
		e.Weights = *sp.Weights
	} else if sp != nil {
		e.Weights = DefaultWeightsForArchetype(sp.Archetype)
	} else {
		e.Weights = DefaultWeightsForArchetype(ArchetypeMidrange)
	}
	return e
}

// Evaluate returns a single score in [-1, +1].
func (e *GameStateEvaluator) Evaluate(gs *gameengine.GameState, seatIdx int) float64 {
	return e.EvaluateDetailed(gs, seatIdx).Score
}

// EvaluateDetailed returns the full per-dimension breakdown.
func (e *GameStateEvaluator) EvaluateDetailed(gs *gameengine.GameState, seatIdx int) EvalResult {
	seat := gs.Seats[seatIdx]

	if seat.Lost || seat.LeftGame {
		return EvalResult{Score: -1}
	}
	if seat.Won {
		return EvalResult{Score: 1}
	}

	var r EvalResult
	r.BoardPresence = e.scoreBoard(gs, seatIdx)
	r.CardAdvantage = e.scoreCards(gs, seatIdx)
	r.ManaAdvantage = e.scoreMana(gs, seatIdx)
	r.LifeResource = e.scoreLife(gs, seatIdx)
	r.ComboProximity = e.scoreCombo(gs, seatIdx)
	r.ThreatExposure = e.scoreThreat(gs, seatIdx)
	r.CommanderProgress = e.scoreCommander(gs, seatIdx)
	r.GraveyardValue = e.scoreGraveyard(gs, seatIdx)
	r.DrainEngine = e.scoreDrainEngine(gs, seatIdx)

	raw := e.Weights.BoardPresence*r.BoardPresence +
		e.Weights.CardAdvantage*r.CardAdvantage +
		e.Weights.ManaAdvantage*r.ManaAdvantage +
		e.Weights.LifeResource*r.LifeResource +
		e.Weights.ComboProximity*r.ComboProximity +
		e.Weights.ThreatExposure*r.ThreatExposure +
		e.Weights.CommanderProgress*r.CommanderProgress +
		e.Weights.GraveyardValue*r.GraveyardValue +
		e.Weights.DrainEngine*r.DrainEngine

	if e.Strategy != nil && e.Strategy.Weakness != nil {
		w := e.Strategy.Weakness
		if w.VulnerableToWipes > 0.3 {
			raw += r.ThreatExposure * w.VulnerableToWipes * 0.5
			raw += r.CardAdvantage * w.VulnerableToWipes * 0.3
		}
		if w.OverExtends > 0.3 && r.BoardPresence > 1.0 {
			raw -= (r.BoardPresence - 1.0) * w.OverExtends * 0.4
		}
		if w.SlowToClose > 0.3 {
			raw += r.ComboProximity * w.SlowToClose * 0.3
		}
	}

	r.Score = math.Tanh(raw / 5.0)
	return r
}

// -----------------------------------------------------------------------
// Dimension scorers — each returns an unbounded raw value. The weighted
// sum is tanh-compressed in EvaluateDetailed.
// -----------------------------------------------------------------------

// scoreBoard: total creature power relative to opponents' average.
func (e *GameStateEvaluator) scoreBoard(gs *gameengine.GameState, seatIdx int) float64 {
	myPow := float64(boardPower(gs.Seats[seatIdx]))

	var oppSum float64
	var oppN int
	for i, s := range gs.Seats {
		if i == seatIdx || s.Lost || s.LeftGame {
			continue
		}
		oppSum += float64(boardPower(s))
		oppN++
	}
	if oppN == 0 {
		if myPow > 0 {
			return 1
		}
		return 0
	}
	oppAvg := oppSum / float64(oppN)

	noncreatures := 0
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p != nil && !p.IsCreature() {
			noncreatures++
		}
	}

	return (myPow - oppAvg) / 10.0 + float64(noncreatures) * 0.1
}

// scoreCards: hand size + library depth relative to average.
func (e *GameStateEvaluator) scoreCards(gs *gameengine.GameState, seatIdx int) float64 {
	seat := gs.Seats[seatIdx]
	myHand := float64(len(seat.Hand))
	myLib := float64(len(seat.Library))

	var oppHand, oppLib float64
	var oppN int
	for i, s := range gs.Seats {
		if i == seatIdx || s.Lost || s.LeftGame {
			continue
		}
		oppHand += float64(len(s.Hand))
		oppLib += float64(len(s.Library))
		oppN++
	}
	if oppN == 0 {
		return myHand / 7.0
	}
	avgHand := oppHand / float64(oppN)
	avgLib := oppLib / float64(oppN)

	return (myHand - avgHand) / 4.0 + (myLib - avgLib) / 40.0
}

// scoreMana: mana source count relative to average, plus color coverage.
func (e *GameStateEvaluator) scoreMana(gs *gameengine.GameState, seatIdx int) float64 {
	mySources := float64(countManaRocksAndLands(gs.Seats[seatIdx]))

	var oppSum float64
	var oppN int
	for i, s := range gs.Seats {
		if i == seatIdx || s.Lost || s.LeftGame {
			continue
		}
		oppSum += float64(countManaRocksAndLands(s))
		oppN++
	}

	var rawScore float64
	if oppN == 0 {
		rawScore = mySources / 5.0
	} else {
		rawScore = (mySources - oppSum/float64(oppN)) / 4.0
	}

	if e.Strategy == nil || e.Strategy.ColorDemand == nil {
		return rawScore
	}

	totalDemand := 0
	coveredDemand := 0
	for col, demand := range e.Strategy.ColorDemand {
		if demand < 2 {
			continue
		}
		totalDemand += demand
		if fieldColorSources(gs.Seats[seatIdx], col) > 0 {
			coveredDemand += demand
		}
	}
	if totalDemand > 0 {
		coverage := float64(coveredDemand) / float64(totalDemand)
		rawScore += (coverage - 0.5) * 0.8
	}

	return rawScore
}

// scoreLife: life as a resource. In commander, 40 is starting; < 10 is
// danger. Normalized relative to starting life. Life-payment decks
// (Bolas's Citadel, Necropotence, K'rrik) should not be penalized for
// spending life when they have payoffs on board.
func (e *GameStateEvaluator) scoreLife(gs *gameengine.GameState, seatIdx int) float64 {
	seat := gs.Seats[seatIdx]
	starting := float64(seat.StartingLife)
	if starting <= 0 {
		starting = 40
	}
	ratio := float64(seat.Life) / starting

	if seat.Life <= 0 {
		return -1
	}

	// Check for life-payment payoffs on the battlefield. When these are
	// active, low life is less dangerous because the life was SPENT for value.
	hasLifePayoff := false
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		ot := gameengine.OracleTextLower(p.Card)
		if strings.Contains(ot, "pay") && strings.Contains(ot, "life") &&
			(strings.Contains(ot, "draw") || strings.Contains(ot, "cast") || strings.Contains(ot, "search")) {
			hasLifePayoff = true
			break
		}
	}

	if seat.Life <= 10 {
		base := ratio - 0.5
		if hasLifePayoff {
			base *= 0.5
		}
		return base
	}
	base := (ratio - 0.5) * 0.5
	if hasLifePayoff && seat.Life > 20 {
		base += 0.1
	}
	return base
}

// scoreCombo: how close we are to assembling a combo. 1.0 = all pieces
// in hand/battlefield, scaled down by missing pieces.
func (e *GameStateEvaluator) scoreCombo(gs *gameengine.GameState, seatIdx int) float64 {
	if e.Strategy == nil || len(e.Strategy.ComboPieces) == 0 {
		return e.scoreComboHardcoded(gs, seatIdx)
	}

	seat := gs.Seats[seatIdx]
	available := make(map[string]bool)
	for _, c := range seat.Hand {
		if c != nil {
			available[c.DisplayName()] = true
		}
	}
	for _, p := range seat.Battlefield {
		if p != nil && p.Card != nil {
			available[p.Card.DisplayName()] = true
		}
	}

	bestRatio := 0.0
	for _, cp := range e.Strategy.ComboPieces {
		if len(cp.Pieces) == 0 {
			continue
		}
		found := 0
		for _, piece := range cp.Pieces {
			if available[piece] {
				found++
			}
		}
		ratio := float64(found) / float64(len(cp.Pieces))
		if ratio > bestRatio {
			bestRatio = ratio
		}
	}

	if bestRatio >= 1.0 {
		return 2.0
	}
	return bestRatio * 1.5
}

func (e *GameStateEvaluator) scoreComboHardcoded(gs *gameengine.GameState, seatIdx int) float64 {
	seat := gs.Seats[seatIdx]
	available := make(map[string]bool)
	for _, c := range seat.Hand {
		if c != nil {
			available[c.DisplayName()] = true
		}
	}
	for _, p := range seat.Battlefield {
		if p != nil && p.Card != nil {
			available[p.Card.DisplayName()] = true
		}
	}

	for _, kc := range knownCombos {
		if available[kc.piece1] && available[kc.piece2] {
			return 2.0
		}
		if available[kc.piece1] || available[kc.piece2] {
			return 0.75
		}
	}
	return 0
}

// scoreThreat: negative score based on how threatened we are by
// opponents' boards. Average opponent board power relative to our life.
func (e *GameStateEvaluator) scoreThreat(gs *gameengine.GameState, seatIdx int) float64 {
	seat := gs.Seats[seatIdx]
	if seat.Life <= 0 {
		return -1
	}

	var maxOppPow float64
	dangerousPermanents := 0.0
	for i, s := range gs.Seats {
		if i == seatIdx || s.Lost || s.LeftGame {
			continue
		}
		bp := float64(boardPower(s))
		if bp > maxOppPow {
			maxOppPow = bp
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			ot := gameengine.OracleTextLower(p.Card)
			if strings.Contains(ot, "whenever a creature enters") && strings.Contains(ot, "damage") {
				dangerousPermanents += 0.3
			}
			if strings.Contains(ot, "whenever a creature dies") && strings.Contains(ot, "loses") {
				dangerousPermanents += 0.25
			}
			if strings.Contains(ot, "each opponent") && strings.Contains(ot, "loses") {
				dangerousPermanents += 0.2
			}
			if strings.Contains(ot, "double") && strings.Contains(ot, "damage") {
				dangerousPermanents += 0.3
			}
		}
	}

	// Vulnerability-aware threat: if an opponent controls a card we're
	// vulnerable to (from Freya threat assessment), increase threat.
	hoserPenalty := 0.0
	if e.Strategy != nil && len(e.Strategy.VulnerableTo) > 0 {
		for i, s := range gs.Seats {
			if i == seatIdx || s.Lost || s.LeftGame {
				continue
			}
			for _, p := range s.Battlefield {
				if p == nil || p.Card == nil {
					continue
				}
				name := strings.ToLower(p.Card.DisplayName())
				for _, hoser := range e.Strategy.VulnerableTo {
					if strings.EqualFold(name, hoser) {
						hoserPenalty += 0.4
					}
				}
			}
		}
		if hoserPenalty > 1.0 {
			hoserPenalty = 1.0
		}
	}

	lethalRatio := maxOppPow / float64(seat.Life)
	if lethalRatio >= 1.0 {
		return -1.0
	}
	return -lethalRatio*0.8 - dangerousPermanents*0.3 - hoserPenalty
}

// scoreCommander: commander combat damage dealt + commander zone status.
func (e *GameStateEvaluator) scoreCommander(gs *gameengine.GameState, seatIdx int) float64 {
	if !gs.CommanderFormat {
		return 0
	}

	seat := gs.Seats[seatIdx]
	score := 0.0

	cmdOnField := false
	synergyBonus := 0.3
	if e.Strategy != nil && e.Strategy.CommanderSynergy > 0.5 {
		synergyBonus = 0.3 + e.Strategy.CommanderSynergy*0.3
	}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, cn := range seat.CommanderNames {
			if p.Card.DisplayName() == cn {
				cmdOnField = true
				score += synergyBonus
			}
		}
	}
	if !cmdOnField && len(seat.CommandZone) > 0 {
		tax := 0
		for _, cn := range seat.CommanderNames {
			tax += seat.CommanderCastCounts[cn]
		}
		taxPenalty := 0.15
		if e.Strategy != nil && e.Strategy.CommanderSynergy > 0.5 {
			taxPenalty = 0.20
		}
		score -= float64(tax) * taxPenalty
	}

	for i, opp := range gs.Seats {
		if i == seatIdx || opp.Lost || opp.LeftGame {
			continue
		}
		if opp.CommanderDamage == nil {
			continue
		}
		if dmgMap, ok := opp.CommanderDamage[seatIdx]; ok {
			for _, dmg := range dmgMap {
				score += float64(dmg) / 21.0
			}
		}
	}

	return score
}

// scoreGraveyard: value of graveyard contents. Creature recursion
// potential + spell flashback potential. Enhanced for self-mill
// strategies where graveyard size itself is the win condition.
func (e *GameStateEvaluator) scoreGraveyard(gs *gameengine.GameState, seatIdx int) float64 {
	seat := gs.Seats[seatIdx]
	if len(seat.Graveyard) == 0 {
		return 0
	}

	value := 0.0
	landCount := 0
	creatureCount := 0
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		ot := gameengine.OracleTextLower(c)
		isCreature := false
		for _, t := range c.Types {
			switch t {
			case "creature":
				isCreature = true
				creatureCount++
			case "land":
				landCount++
			}
		}
		if isCreature {
			value += 0.15
		}
		if strings.Contains(ot, "flashback") || strings.Contains(ot, "escape") ||
			strings.Contains(ot, "unearth") || strings.Contains(ot, "retrace") {
			value += 0.25
		}
	}

	// Self-mill bonus: check if commander or battlefield permanents
	// care about graveyard size (Uurg, Splinterfright, Lhurgoyf,
	// Jarad, Sidisi, etc.)
	selfMillPayoff := false
	for _, cn := range seat.CommanderNames {
		cnLower := strings.ToLower(cn)
		if strings.Contains(cnLower, "uurg") ||
			strings.Contains(cnLower, "sidisi") ||
			strings.Contains(cnLower, "muldrotha") ||
			strings.Contains(cnLower, "karador") ||
			strings.Contains(cnLower, "gitrog") ||
			strings.Contains(cnLower, "slogurk") {
			selfMillPayoff = true
			break
		}
	}
	if !selfMillPayoff {
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			ot := gameengine.OracleTextLower(p.Card)
			if (strings.Contains(ot, "equal to") || strings.Contains(ot, "for each")) &&
				(strings.Contains(ot, "graveyard") || strings.Contains(ot, "creature card")) {
				selfMillPayoff = true
				break
			}
		}
	}
	if selfMillPayoff {
		gySize := float64(len(seat.Graveyard))
		value += gySize * 0.08
		value += float64(landCount) * 0.05
		value += float64(creatureCount) * 0.06
	}

	if e.Strategy != nil && (e.Strategy.Archetype == ArchetypeSelfmill || e.Strategy.Archetype == ArchetypeReanimator) {
		value *= 1.3
	}

	var oppAvg float64
	var oppN int
	for i, s := range gs.Seats {
		if i == seatIdx || s.Lost || s.LeftGame {
			continue
		}
		oppN++
		oppAvg += float64(len(s.Graveyard))
	}
	if oppN > 0 {
		oppAvg /= float64(oppN)
	}
	value += (float64(len(seat.Graveyard)) - oppAvg) * 0.05

	return value
}

// scoreDrainEngine: scores the presence and strength of aristocrats-style
// drain infrastructure. Detects death-trigger payoffs (Blood Artist,
// Zulaport Cutthroat, Dina), sacrifice outlets, and available fodder.
// Returns higher values when more drain pieces are assembled.
func (e *GameStateEvaluator) scoreDrainEngine(gs *gameengine.GameState, seatIdx int) float64 {
	seat := gs.Seats[seatIdx]

	drainPayoffs := 0.0
	sacOutlets := 0.0
	fodder := 0.0
	lifeGainPayoffs := 0.0

	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		ot := gameengine.OracleTextLower(p.Card)

		// Death-trigger drain payoffs: Blood Artist, Zulaport Cutthroat,
		// Bastion of Remembrance, Vindictive Vampire, Syr Konrad, etc.
		if strings.Contains(ot, "whenever") &&
			(strings.Contains(ot, "creature dies") || strings.Contains(ot, "a creature dies")) &&
			(strings.Contains(ot, "loses") || strings.Contains(ot, "drain") ||
				strings.Contains(ot, "each opponent") || strings.Contains(ot, "life")) {
			drainPayoffs += 1.0
		}

		// Lifegain-trigger drain: Dina, Vito, Marauding Blight-Priest
		if strings.Contains(ot, "whenever you gain life") &&
			(strings.Contains(ot, "loses") || strings.Contains(ot, "each opponent")) {
			lifeGainPayoffs += 1.0
		}

		// ETB/LTB drain: Wayward Servant, Corpse Knight
		if strings.Contains(ot, "whenever") &&
			strings.Contains(ot, "enters") &&
			(strings.Contains(ot, "loses") || strings.Contains(ot, "each opponent")) {
			drainPayoffs += 0.7
		}

		// Sacrifice outlets (free or cheap)
		if (strings.Contains(ot, "sacrifice a creature") || strings.Contains(ot, "sacrifice another") ||
			strings.Contains(ot, "sacrifice an artifact") || strings.Contains(ot, "sacrifice a permanent")) &&
			!strings.Contains(ot, "when") {
			sacOutlets += 1.0
		}

		// Fodder: tokens and small creatures with death triggers
		if p.IsToken() && !p.IsCreature() {
			fodder += 0.3 // Treasure, Food, Clue tokens
		}
		if p.IsCreature() {
			if p.IsToken() {
				fodder += 0.5
			} else if strings.Contains(ot, "when") && strings.Contains(ot, "dies") {
				fodder += 0.8
			}
		}
	}

	if drainPayoffs == 0 && lifeGainPayoffs == 0 {
		return 0
	}

	// Score based on how many drain engine pieces are assembled.
	// Having payoff + outlet + fodder is much more valuable than just payoff.
	score := drainPayoffs * 0.5
	score += lifeGainPayoffs * 0.4

	if sacOutlets > 0 {
		score += sacOutlets * 0.4
		// Payoff + outlet synergy multiplier
		score *= 1.0 + math.Min(sacOutlets, 2)*0.3
	}

	if fodder > 0 {
		score += fodder * 0.2
		// Full engine assembled: payoff + outlet + fodder
		if sacOutlets > 0 {
			score *= 1.0 + math.Min(fodder, 4)*0.15
		}
	}

	// Opponent life totals factor: drain is more valuable when opponents
	// are already low.
	avgOppLife := 0.0
	oppN := 0
	for i, s := range gs.Seats {
		if i == seatIdx || s.Lost || s.LeftGame {
			continue
		}
		avgOppLife += float64(s.Life)
		oppN++
	}
	if oppN > 0 {
		avgOppLife /= float64(oppN)
		if avgOppLife < 20 {
			score *= 1.0 + (20-avgOppLife)/20*0.5
		}
	}

	return score
}
