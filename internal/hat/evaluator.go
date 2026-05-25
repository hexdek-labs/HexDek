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

	// PlanMultiplier is set by the hat's plan state machine each turn.
	// When non-nil, rescaleWeights multiplies each dimension weight by
	// the corresponding multiplier value. This is the mechanism for plan
	// biases (Execute boosts ComboProximity, Disrupt boosts ThreatExposure,
	// etc.) without permanently altering the archetype baseline.
	PlanMultiplier *EvalWeights
}

// EvalResult holds the per-dimension breakdown alongside the final score.
type EvalResult struct {
	Score              float64
	BoardPresence      float64
	CardAdvantage      float64
	ManaAdvantage      float64
	LifeResource       float64
	ComboProximity     float64
	ThreatExposure     float64
	CommanderProgress  float64
	GraveyardValue     float64
	DrainEngine        float64
	ArtifactSynergy    float64
	EnchantmentSynergy      float64
	OpponentGraveyardThreat float64
	PartnerSynergy     float64
	ActivationTempo    float64
	ToolboxBreadth        float64
	ThreatTrajectory      float64
	StackInteraction      float64
	PlaneswalkerProgress  float64
	ExileZoneAssets       float64
	StaxLockProgress      float64
}

func (r EvalResult) AsArray() [NumDimensions]float64 {
	return [NumDimensions]float64{
		r.BoardPresence, r.CardAdvantage, r.ManaAdvantage,
		r.LifeResource, r.ComboProximity, r.ThreatExposure,
		r.CommanderProgress, r.GraveyardValue, r.DrainEngine,
		r.ArtifactSynergy, r.EnchantmentSynergy, r.OpponentGraveyardThreat,
		r.PartnerSynergy, r.ActivationTempo, r.ToolboxBreadth,
		r.ThreatTrajectory, r.StackInteraction, r.PlaneswalkerProgress,
		r.ExileZoneAssets, r.StaxLockProgress,
	}
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
	r.ArtifactSynergy = e.scoreArtifactSynergy(gs, seatIdx)
	r.EnchantmentSynergy = e.scoreEnchantmentSynergy(gs, seatIdx)
	r.OpponentGraveyardThreat = e.scoreOpponentGraveyard(gs, seatIdx)
	r.PartnerSynergy = e.scorePartnerSynergy(gs, seatIdx)
	r.ActivationTempo = e.scoreActivationTempo(gs, seatIdx)
	r.ToolboxBreadth = e.scoreToolboxBreadth(gs, seatIdx)
	r.ThreatTrajectory = e.scoreThreatTrajectory(gs, seatIdx)
	r.StackInteraction = e.scoreStackInteraction(gs, seatIdx)
	r.PlaneswalkerProgress = e.scorePlaneswalkerProgress(gs, seatIdx)
	r.ExileZoneAssets = e.scoreExileAssets(gs, seatIdx)
	r.StaxLockProgress = e.scoreStaxLock(gs, seatIdx)

	w := e.rescaleWeights(gs, seatIdx)

	raw := w.BoardPresence*r.BoardPresence +
		w.CardAdvantage*r.CardAdvantage +
		w.ManaAdvantage*r.ManaAdvantage +
		w.LifeResource*r.LifeResource +
		w.ComboProximity*r.ComboProximity +
		w.ThreatExposure*r.ThreatExposure +
		w.CommanderProgress*r.CommanderProgress +
		w.GraveyardValue*r.GraveyardValue +
		w.DrainEngine*r.DrainEngine +
		w.ArtifactSynergy*r.ArtifactSynergy +
		w.EnchantmentSynergy*r.EnchantmentSynergy +
		w.OpponentGraveyardThreat*r.OpponentGraveyardThreat +
		w.PartnerSynergy*r.PartnerSynergy +
		w.ActivationTempo*r.ActivationTempo +
		w.ToolboxBreadth*r.ToolboxBreadth +
		w.ThreatTrajectory*r.ThreatTrajectory +
		w.StackInteraction*r.StackInteraction +
		w.PlaneswalkerProgress*r.PlaneswalkerProgress +
		w.ExileZoneAssets*r.ExileZoneAssets +
		w.StaxLockProgress*r.StaxLockProgress

	if e.Strategy != nil && e.Strategy.Weakness != nil {
		wk := e.Strategy.Weakness
		if wk.VulnerableToWipes > 0.3 {
			raw += r.ThreatExposure * wk.VulnerableToWipes * 0.5
			raw += r.CardAdvantage * wk.VulnerableToWipes * 0.3
		}
		if wk.OverExtends > 0.3 && r.BoardPresence > 1.0 {
			raw -= (r.BoardPresence - 1.0) * wk.OverExtends * 0.4
		}
		if wk.SlowToClose > 0.3 {
			raw += r.ComboProximity * wk.SlowToClose * 0.3
		}
		if wk.ManaScrew > 0.3 {
			raw += r.ManaAdvantage * wk.ManaScrew * 0.4
		}
		if wk.VulnerableToCounter > 0.3 {
			raw += r.ToolboxBreadth * wk.VulnerableToCounter * 0.3
			raw += r.ActivationTempo * wk.VulnerableToCounter * 0.2
		}
	}

	// Per-turn tempo bonus: read TurnCounters so storm/lifegain/go-wide
	// archetypes get rewarded for chaining cheap spells, stacking life
	// gains, or flooding the board within a single turn.
	raw += e.scoreTurnTempo(gs, seatIdx)

	r.Score = math.Tanh(raw / 5.0)
	return r
}

// scoreTurnTempo reads the engine's per-seat TurnCounters
// (Seat.Turn.SpellsCast / LifeGained / CreaturesEntered) and returns an
// archetype-aware bonus. Storm/Combo decks get bonus per spell chained,
// Lifegain decks get bonus per life-gain trigger this turn, Aggro and
// "go-wide" tokens decks get bonus per creature that entered this turn.
//
// All bonuses are small and additive into the pre-tanh raw score so they
// nudge the evaluator without overriding the dimension-weighted base.
func (e *GameStateEvaluator) scoreTurnTempo(gs *gameengine.GameState, seatIdx int) float64 {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return 0
	}
	tc := seat.Turn

	// Default per-counter weights — tuned to match the archetype-agnostic
	// "good thing happened this turn" intuition without dominating.
	spellW := 0.05    // ~1/20th of a dimension per spell cast this turn
	lifeW := 0.01     // 0.01 per life gained
	creatureW := 0.05 // 0.05 per creature entered

	// Archetype amplifier: when we know what the deck wants to do, push
	// the matching counter harder so the hat sees positional momentum.
	if e.Strategy != nil {
		switch strings.ToLower(e.Strategy.Archetype) {
		case "combo", "storm":
			spellW = 0.12
		case "aggro":
			creatureW = 0.10
		case "tokens", "go-wide", "gowide":
			creatureW = 0.15
		case "lifegain":
			lifeW = 0.04
		}

		// CommanderThemes give a softer secondary signal — a deck that
		// isn't archetype:Combo but reads as "spellslinger" still benefits
		// from chaining spells.
		for _, theme := range e.Strategy.CommanderThemes {
			switch strings.ToLower(theme) {
			case "spellslinger", "storm":
				if spellW < 0.10 {
					spellW = 0.10
				}
			case "tokens", "go-wide":
				if creatureW < 0.10 {
					creatureW = 0.10
				}
			case "lifegain":
				if lifeW < 0.03 {
					lifeW = 0.03
				}
			}
		}
	}

	bonus := float64(tc.SpellsCast)*spellW +
		float64(tc.LifeGained)*lifeW +
		float64(tc.CreaturesEntered)*creatureW

	// Cap the per-turn bonus so a runaway storm count can't dominate
	// the entire eval (~one full dimension's weight worth).
	if bonus > 1.5 {
		bonus = 1.5
	}
	return bonus
}

// -----------------------------------------------------------------------
// Dimension scorers — each returns an unbounded raw value. The weighted
// sum is tanh-compressed in EvaluateDetailed.
// -----------------------------------------------------------------------

// scoreBoard: total creature power relative to opponents' average.
func (e *GameStateEvaluator) scoreBoard(gs *gameengine.GameState, seatIdx int) float64 {
	myPow := float64(boardPower(gs, gs.Seats[seatIdx]))

	var oppSum float64
	var oppN int
	for i, s := range gs.Seats {
		if i == seatIdx || s.Lost || s.LeftGame {
			continue
		}
		oppSum += float64(boardPower(gs, s))
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

// scoreCards: hand size + library depth + persistent draw engines on
// battlefield + castable-from-exile cards, all relative to opponent
// average. The engine + castable-exile terms capture virtual card
// advantage that pure hand-count misses (a 4-card hand behind a Rhystic
// Study + Phyrexian Arena is a different game than a 4-card hand
// behind nothing).
func (e *GameStateEvaluator) scoreCards(gs *gameengine.GameState, seatIdx int) float64 {
	seat := gs.Seats[seatIdx]
	myHand := float64(len(seat.Hand))
	myLib := float64(len(seat.Library))
	myEngines := drawEngineCredit(seat)
	myCastExile := float64(castableExileCount(gs, seatIdx))

	var oppHand, oppLib, oppEngines, oppCastExile float64
	var oppN int
	for i, s := range gs.Seats {
		if i == seatIdx || s.Lost || s.LeftGame {
			continue
		}
		oppHand += float64(len(s.Hand))
		oppLib += float64(len(s.Library))
		oppEngines += drawEngineCredit(s)
		oppCastExile += float64(castableExileCount(gs, i))
		oppN++
	}
	if oppN == 0 {
		return myHand/7.0 + myEngines*0.4 + myCastExile*0.3
	}
	avgHand := oppHand / float64(oppN)
	avgLib := oppLib / float64(oppN)
	avgEngines := oppEngines / float64(oppN)
	avgCastExile := oppCastExile / float64(oppN)

	return (myHand-avgHand)/4.0 +
		(myLib-avgLib)/40.0 +
		(myEngines-avgEngines)*0.4 +
		(myCastExile-avgCastExile)*0.3
}

// drawEngineCredit estimates the seat's virtual cards-per-turn from
// persistent draw engines on its battlefield (Rhystic Study, Phyrexian
// Arena, Mystic Remora, Esper Sentinel, Sylvan Library, Howling Mine,
// Mind's Eye, ...). Each detected engine contributes 1.0. Capped at 4
// to bound stax/wheel-board outliers.
func drawEngineCredit(seat *gameengine.Seat) float64 {
	if seat == nil {
		return 0
	}
	n := 0
	for _, p := range seat.Battlefield {
		if !isPersistentDrawEngine(p) {
			continue
		}
		n++
		if n >= 4 {
			break
		}
	}
	return float64(n)
}

// isPersistentDrawEngine returns true when perm's oracle text indicates
// a recurring controller-facing card-draw payoff. The three matched
// shapes — "additional card", upkeep-cadenced draw, and whenever-X you
// (may) draw a card — cover the bulk of Commander draw enchantments,
// artifacts, and Esper-Sentinel-style creatures. One-shot ETB drawers
// (Elvish Visionary's "when ~ enters, you draw a card") are
// intentionally excluded — they require the "whenever" / upkeep cue.
func isPersistentDrawEngine(p *gameengine.Permanent) bool {
	if p == nil || p.Card == nil {
		return false
	}
	ot := gameengine.OracleTextLower(p.Card)
	if ot == "" {
		return false
	}
	if strings.Contains(ot, "additional card") {
		return true
	}
	if strings.Contains(ot, "at the beginning of your upkeep") &&
		(strings.Contains(ot, "you draw") || strings.Contains(ot, "draw a card")) {
		return true
	}
	if strings.Contains(ot, "whenever") &&
		(strings.Contains(ot, "you draw a card") || strings.Contains(ot, "you may draw a card")) {
		return true
	}
	return false
}

// castableExileCount counts cards in seat's exile zone that the seat
// can still cast as a spell via a ZoneCastGrants entry whose Zone is
// "exile" and whose RequireController matches (or is -1 = owner-cast,
// which we honor when the card's owner is this seat). Covers foretell,
// plot, suspend-resolves-to-cast, Misthollow Griffin / Squee, and "may
// cast from exile" grants such as Light Up the Stage, Prosper, Faldorn.
func castableExileCount(gs *gameengine.GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || len(seat.Exile) == 0 || gs.ZoneCastGrants == nil {
		return 0
	}
	n := 0
	for _, c := range seat.Exile {
		if c == nil {
			continue
		}
		perm, ok := gs.ZoneCastGrants[c]
		if !ok || perm == nil || perm.Zone != "exile" {
			continue
		}
		if perm.RequireController == seatIdx ||
			(perm.RequireController == -1 && c.Owner == seatIdx) {
			n++
		}
	}
	return n
}

// scoreMana: mana source count relative to average, plus color coverage.
func (e *GameStateEvaluator) scoreMana(gs *gameengine.GameState, seatIdx int) float64 {
	mySources := float64(CountManaRocksAndLands(gs.Seats[seatIdx]))

	var oppSum float64
	var oppN int
	for i, s := range gs.Seats {
		if i == seatIdx || s.Lost || s.LeftGame {
			continue
		}
		oppSum += float64(CountManaRocksAndLands(s))
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
//
// When the deck has a primary combo class (>=2 ComboPlans share a
// class), on-class plans score their piece-presence ratio at full
// weight; off-class plans are dampened to 0.7x. The damping keeps
// off-class plans relevant (a complete off-class plan still beats a
// half-assembled on-class plan) without letting an opportunistic
// off-class piece pull the deck off its primary line.
//
// Plans with empty Class always score at full weight — Class is a
// post-hoc tag and the scorer must remain useful for unclassified
// or legacy strategy.json files.
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

	primaryClass := e.Strategy.PrimaryComboClass()
	const offClassDamping = 0.7

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
		// Off-class damping: when the deck has a primary class and this
		// plan is in a different class, scale the contribution. Plans
		// without a Class tag (legacy / unclassified) always score full
		// weight.
		if primaryClass != "" && cp.Class != "" && cp.Class != primaryClass {
			ratio *= offClassDamping
		}
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
		bp := float64(boardPower(gs, s))
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

	// Poison threat: high poison counters are an existential threat
	// regardless of combat board state. 10 = lethal per §704.5c.
	poisonPenalty := 0.0
	if seat.PoisonCounters >= 9 {
		poisonPenalty = 0.8 // one more counter = death
	} else if seat.PoisonCounters >= 7 {
		poisonPenalty = 0.5
	} else if seat.PoisonCounters >= 5 {
		poisonPenalty = 0.2
	}
	// Check if any opponent controls infect/toxic creatures — amplifies
	// the urgency when we already have poison counters.
	if seat.PoisonCounters > 0 {
		for i, s := range gs.Seats {
			if i == seatIdx || s.Lost || s.LeftGame {
				continue
			}
			for _, p := range s.Battlefield {
				if p == nil {
					continue
				}
				if p.HasKeyword("infect") || p.HasKeyword("toxic") {
					poisonPenalty += 0.15
					break // one infect source per opponent is enough
				}
			}
		}
		if poisonPenalty > 1.0 {
			poisonPenalty = 1.0
		}
	}

	// Mill threat: low library is dangerous if opponents have mill effects.
	millPenalty := 0.0
	myLibSize := len(seat.Library)
	if myLibSize < 15 {
		for i, s := range gs.Seats {
			if i == seatIdx || s.Lost || s.LeftGame {
				continue
			}
			for _, p := range s.Battlefield {
				if p == nil || p.Card == nil {
					continue
				}
				ot := gameengine.OracleTextLower(p.Card)
				if (strings.Contains(ot, "mill") || strings.Contains(ot, "cards from the top of") ||
					(strings.Contains(ot, "library") && strings.Contains(ot, "graveyard"))) &&
					(strings.Contains(ot, "opponent") || strings.Contains(ot, "target player") || strings.Contains(ot, "each player")) {
					if myLibSize < 5 {
						millPenalty = 0.8
					} else if myLibSize < 10 {
						millPenalty = 0.4
					} else {
						millPenalty = 0.2
					}
					break
				}
			}
			if millPenalty > 0 {
				break
			}
		}
	}

	// Commander damage proximity: §704.6c lethal at 21.
	cmdrPenalty := 0.0
	if gs.CommanderFormat && seat.CommanderDamage != nil {
		for _, cmdMap := range seat.CommanderDamage {
			for _, dmg := range cmdMap {
				if dmg >= 18 {
					cmdrPenalty = 0.6
				} else if dmg >= 15 && cmdrPenalty < 0.3 {
					cmdrPenalty = 0.3
				}
			}
		}
	}

	return -lethalRatio*0.8 - dangerousPermanents*0.3 - hoserPenalty - poisonPenalty - millPenalty - cmdrPenalty
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

// scoreArtifactSynergy: counts artifacts on battlefield, treasure tokens,
// and artifact-matters payoffs on the commander.
func (e *GameStateEvaluator) scoreArtifactSynergy(gs *gameengine.GameState, seatIdx int) float64 {
	seat := gs.Seats[seatIdx]

	artifactCount := 0
	treasureCount := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.IsArtifact() {
			artifactCount++
		}
		for _, t := range p.Card.Types {
			if t == "treasure" {
				treasureCount++
				break
			}
		}
	}

	commanderBonus := 0.0
	for _, c := range seat.CommandZone {
		if c == nil {
			continue
		}
		ot := gameengine.OracleTextLower(c)
		if strings.Contains(ot, "artifact") &&
			(strings.Contains(ot, "whenever") || strings.Contains(ot, "for each") ||
				strings.Contains(ot, "control") || strings.Contains(ot, "cast")) {
			commanderBonus = 0.6
			break
		}
	}
	if commanderBonus == 0 {
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			isCommander := false
			for _, cn := range seat.CommanderNames {
				if p.Card.DisplayName() == cn {
					isCommander = true
					break
				}
			}
			if !isCommander {
				continue
			}
			ot := gameengine.OracleTextLower(p.Card)
			if strings.Contains(ot, "artifact") &&
				(strings.Contains(ot, "whenever") || strings.Contains(ot, "for each") ||
					strings.Contains(ot, "control") || strings.Contains(ot, "cast")) {
				commanderBonus = 0.6
			}
			break
		}
	}

	return float64(artifactCount)*0.1 + float64(treasureCount)*0.15 + commanderBonus
}

// scoreEnchantmentSynergy: counts enchantments on battlefield and
// enchantress-style draw engines (whenever you cast an enchantment, draw).
func (e *GameStateEvaluator) scoreEnchantmentSynergy(gs *gameengine.GameState, seatIdx int) float64 {
	seat := gs.Seats[seatIdx]

	enchantmentCount := 0
	enchantressEngines := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.IsEnchantment() {
			enchantmentCount++
		}
		ot := gameengine.OracleTextLower(p.Card)
		if strings.Contains(ot, "whenever you cast an enchantment") &&
			strings.Contains(ot, "draw") {
			enchantressEngines++
		}
	}

	score := float64(enchantmentCount) * 0.12
	score += float64(enchantressEngines) * 0.5
	if enchantressEngines > 0 && enchantmentCount > 3 {
		score *= 1.0 + math.Min(float64(enchantressEngines), 3)*0.2
	}
	return score
}

// scoreOpponentGraveyard: negative signal representing danger from opponents'
// graveyards. Detects reanimation targets, flashback/escape spells, and
// high-value creatures that could be cheated back into play.
func (e *GameStateEvaluator) scoreOpponentGraveyard(gs *gameengine.GameState, seatIdx int) float64 {
	threat := 0.0
	for i, s := range gs.Seats {
		if i == seatIdx || s.Lost || s.LeftGame {
			continue
		}
		for _, c := range s.Graveyard {
			if c == nil {
				continue
			}
			ot := gameengine.OracleTextLower(c)

			// High-CMC creatures are prime reanimation targets.
			isCreature := false
			for _, t := range c.Types {
				if t == "creature" {
					isCreature = true
					break
				}
			}
			if isCreature && c.CMC >= 5 {
				threat -= 0.20
			} else if isCreature && c.CMC >= 3 {
				threat -= 0.08
			}

			// Spells with flashback/escape/unearth/retrace can be re-cast.
			if strings.Contains(ot, "flashback") || strings.Contains(ot, "escape") ||
				strings.Contains(ot, "unearth") || strings.Contains(ot, "retrace") {
				threat -= 0.15
			}
		}

		// Check if opponent has reanimation enablers on battlefield.
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			ot := gameengine.OracleTextLower(p.Card)
			if (strings.Contains(ot, "return") || strings.Contains(ot, "put")) &&
				strings.Contains(ot, "graveyard") &&
				strings.Contains(ot, "battlefield") {
				threat -= 0.30
			}
		}

		// Commanders known for graveyard abuse amplify the threat.
		for _, cn := range s.CommanderNames {
			cnLower := strings.ToLower(cn)
			if strings.Contains(cnLower, "meren") ||
				strings.Contains(cnLower, "muldrotha") ||
				strings.Contains(cnLower, "karador") ||
				strings.Contains(cnLower, "chainer") ||
				strings.Contains(cnLower, "araumi") ||
				strings.Contains(cnLower, "nethroi") ||
				strings.Contains(cnLower, "sefris") {
				threat -= 0.40
				break
			}
		}
	}
	return threat
}

// scorePartnerSynergy: value of having two commanders (partner pair) and
// their on-field interaction. Returns 0 for non-partner decks.
func (e *GameStateEvaluator) scorePartnerSynergy(gs *gameengine.GameState, seatIdx int) float64 {
	if !gs.CommanderFormat {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if len(seat.CommanderNames) < 2 {
		return 0
	}

	score := 0.0

	cmdsOnField := 0
	var cmdCards []*gameengine.Card
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, cn := range seat.CommanderNames {
			if p.Card.DisplayName() == cn {
				cmdsOnField++
				cmdCards = append(cmdCards, p.Card)
			}
		}
	}
	if cmdsOnField >= 2 {
		score += 0.6
	} else if cmdsOnField == 1 {
		score += 0.2
	}

	// Color coverage from the partner pair.
	colorSet := make(map[string]bool)
	for _, cn := range seat.CommanderNames {
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if p.Card.DisplayName() == cn {
				for _, c := range p.Card.Colors {
					colorSet[c] = true
				}
			}
		}
		for _, c := range seat.CommandZone {
			if c == nil {
				continue
			}
			if c.DisplayName() == cn {
				for _, col := range c.Colors {
					colorSet[col] = true
				}
			}
		}
	}
	if len(colorSet) >= 4 {
		score += 0.35
	} else if len(colorSet) >= 3 {
		score += 0.25
	} else if len(colorSet) >= 2 {
		score += 0.15
	}

	// Complementary abilities: detect if the two commanders cover
	// different strategic roles (one draws, one attacks, etc.).
	if len(cmdCards) >= 2 {
		roles := 0
		for _, c := range cmdCards {
			ot := gameengine.OracleTextLower(c)
			if strings.Contains(ot, "draw") {
				roles |= 1
			}
			if c.BasePower >= 4 || strings.Contains(ot, "combat") || strings.Contains(ot, "attack") {
				roles |= 2
			}
			if strings.Contains(ot, "search") || strings.Contains(ot, "tutor") {
				roles |= 4
			}
			if strings.Contains(ot, "destroy") || strings.Contains(ot, "exile") || strings.Contains(ot, "counter") {
				roles |= 8
			}
		}
		// Count distinct role bits — more roles covered = better complementarity.
		bits := 0
		for v := roles; v > 0; v &= v - 1 {
			bits++
		}
		if bits >= 3 {
			score += 0.3
		} else if bits >= 2 {
			score += 0.15
		}
	}

	// Tax penalty: partners dying repeatedly is bad.
	totalTax := 0
	for _, cn := range seat.CommanderNames {
		totalTax += seat.CommanderCastCounts[cn]
	}
	if totalTax >= 4 {
		score -= 0.2
	}

	return score
}

// scoreActivationTempo: value of untapped activated abilities on the
// battlefield. Higher when the seat has repeatable engines ready.
func (e *GameStateEvaluator) scoreActivationTempo(gs *gameengine.GameState, seatIdx int) float64 {
	seat := gs.Seats[seatIdx]
	score := 0.0

	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		ot := gameengine.OracleTextLower(p.Card)
		if !strings.Contains(ot, ":") {
			continue
		}
		// Skip pure mana abilities.
		if isManaOnlyAbility(ot) {
			continue
		}

		value := 0.3
		// No tap required = repeatable within a turn.
		if !strings.Contains(ot, "{t}:") && !strings.Contains(ot, ", tap:") && !strings.Contains(ot, "tap an untapped") {
			value = 0.4
		}
		// High-impact activations.
		if strings.Contains(ot, "draw") || strings.Contains(ot, "destroy") ||
			strings.Contains(ot, "exile") || strings.Contains(ot, "create") ||
			strings.Contains(ot, "damage") {
			value += 0.15
		}

		if p.Tapped {
			score += value * 0.3
		} else {
			score += value
		}
	}

	// Compare to opponents.
	var oppAvg float64
	var oppN int
	for i, s := range gs.Seats {
		if i == seatIdx || s.Lost || s.LeftGame {
			continue
		}
		oppN++
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			ot := gameengine.OracleTextLower(p.Card)
			if strings.Contains(ot, ":") && !isManaOnlyAbility(ot) && !p.Tapped {
				oppAvg += 0.3
			}
		}
	}
	if oppN > 0 {
		oppAvg /= float64(oppN)
		score += (score - oppAvg) * 0.2
	}

	return score
}

// isManaOnlyAbility returns true if the oracle text's colon-ability is
// purely a mana-producing ability (e.g., "{T}: Add {G}").
func isManaOnlyAbility(ot string) bool {
	return (strings.Contains(ot, ": add {") || strings.Contains(ot, ": add one")) &&
		!strings.Contains(ot, "draw") && !strings.Contains(ot, "destroy") &&
		!strings.Contains(ot, "exile") && !strings.Contains(ot, "damage") &&
		!strings.Contains(ot, "create") && !strings.Contains(ot, "counter") &&
		!strings.Contains(ot, "return")
}

// scoreToolboxBreadth: diversity of available lines — tutors in hand,
// modal spells, MDFC flexibility, and non-mana activations on board.
func (e *GameStateEvaluator) scoreToolboxBreadth(gs *gameengine.GameState, seatIdx int) float64 {
	seat := gs.Seats[seatIdx]
	score := 0.0

	tutorsInHand := 0
	modalInHand := 0
	mdfcInHand := 0

	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		ot := gameengine.OracleTextLower(c)

		if strings.Contains(ot, "search your library") {
			tutorsInHand++
		}
		if strings.Contains(ot, "choose one") || strings.Contains(ot, "choose two") ||
			strings.Contains(ot, "choose three") {
			modalInHand++
		}
		if c.IsMDFC() {
			mdfcInHand++
		}
	}

	// Non-mana activations on battlefield.
	activationsOnBoard := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		ot := gameengine.OracleTextLower(p.Card)
		if strings.Contains(ot, ":") && !isManaOnlyAbility(ot) {
			activationsOnBoard++
		}
	}

	score += float64(tutorsInHand) * 0.4
	score += float64(modalInHand) * 0.25
	score += float64(mdfcInHand) * 0.15
	score += math.Min(float64(activationsOnBoard), 6) * 0.15

	// Tutor targets defined = tutors are even more valuable.
	if e.Strategy != nil && len(e.Strategy.TutorTargets) > 0 && tutorsInHand > 0 {
		score += 0.2
	}

	return score
}

// scoreThreatTrajectory: forward-looking threat assessment. Projects each
// opponent's next-turn power using hand size, mana availability, and
// recent spell cadence rather than just current board state.
func (e *GameStateEvaluator) scoreThreatTrajectory(gs *gameengine.GameState, seatIdx int) float64 {
	seat := gs.Seats[seatIdx]
	if seat.Life <= 0 {
		return -1
	}

	threat := 0.0
	for i, s := range gs.Seats {
		if i == seatIdx || s.Lost || s.LeftGame {
			continue
		}

		bp := float64(boardPower(gs, s))
		handCards := float64(len(s.Hand))
		manaSources := float64(CountManaRocksAndLands(s))

		// Deployment potential: each unplayed card with available mana
		// is ~2 power waiting to deploy. Cap by mana availability.
		deployable := math.Min(handCards, manaSources/2.5)
		projectedPower := bp + deployable*1.5

		// Spell cadence: opponents chaining spells are ramping up.
		cadenceBonus := 0.0
		if s.SpellsCastThisTurn > 2 {
			cadenceBonus = float64(s.SpellsCastThisTurn-2) * 0.15
		}

		// Project threat as ratio to our life total.
		if seat.Life > 0 {
			rawThreat := (projectedPower + cadenceBonus*5) / float64(seat.Life)
			threat -= rawThreat * 0.3
		}
	}

	if threat < -2.0 {
		threat = -2.0
	}
	return threat
}

func (e *GameStateEvaluator) scoreStackInteraction(gs *gameengine.GameState, seatIdx int) float64 {
	seat := gs.Seats[seatIdx]
	avail := gameengine.AvailableManaEstimate(gs, seat)

	var score float64
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		isInstant := false
		for _, t := range c.Types {
			if t == "instant" {
				isInstant = true
				break
			}
		}
		ot := gameengine.OracleTextLower(c)
		hasFlash := !isInstant && strings.Contains(ot, "flash")
		if !isInstant && !hasFlash {
			continue
		}

		cmc := gameengine.ManaCostOf(c)
		castable := cmc <= avail

		value := 0.0
		switch {
		case strings.Contains(ot, "counter target"):
			value = 1.0
		case strings.Contains(ot, "destroy target") || strings.Contains(ot, "exile target"):
			value = 0.8
		case strings.Contains(ot, "return target") && strings.Contains(ot, "owner"):
			value = 0.6
		case isInstant && (strings.Contains(ot, "damage to") || strings.Contains(ot, "-x/-x") || strings.Contains(ot, "gets -")):
			value = 0.5
		case hasFlash:
			value = 0.3
		case isInstant:
			value = 0.15
		}
		if value == 0 {
			continue
		}
		if castable {
			score += value
		} else {
			score += value * 0.3
		}
	}
	return score / 2.0
}

func (e *GameStateEvaluator) scorePlaneswalkerProgress(gs *gameengine.GameState, seatIdx int) float64 {
	seat := gs.Seats[seatIdx]
	var score float64
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsPlaneswalker() {
			continue
		}
		loyalty := 0
		if p.Counters != nil {
			loyalty = p.Counters["loyalty"]
		}
		if loyalty <= 0 {
			continue
		}
		score += float64(loyalty) / 6.0
	}
	oppPW := 0.0
	for i, s := range gs.Seats {
		if i == seatIdx || s.Lost || s.LeftGame {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.IsPlaneswalker() {
				loy := 0
				if p.Counters != nil {
					loy = p.Counters["loyalty"]
				}
				oppPW += float64(loy) / 6.0
			}
		}
	}
	oppN := 0
	for i, s := range gs.Seats {
		if i != seatIdx && !s.Lost && !s.LeftGame {
			oppN++
		}
	}
	if oppN > 0 {
		score -= oppPW / float64(oppN) * 0.5
	}
	return score
}

func (e *GameStateEvaluator) scoreExileAssets(gs *gameengine.GameState, seatIdx int) float64 {
	seat := gs.Seats[seatIdx]
	if len(seat.Exile) == 0 {
		return 0
	}
	hasEnabler := false
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		ot := gameengine.OracleTextLower(p.Card)
		if (strings.Contains(ot, "exile") && (strings.Contains(ot, "you may play") || strings.Contains(ot, "you may cast"))) ||
			strings.Contains(ot, "play cards from exile") ||
			strings.Contains(ot, "cast spells from exile") ||
			strings.Contains(ot, "play lands from exile") {
			hasEnabler = true
			break
		}
	}
	selfPlayable := 0
	for _, c := range seat.Exile {
		if c == nil {
			continue
		}
		ot := gameengine.OracleTextLower(c)
		if strings.Contains(ot, "foretell") || strings.Contains(ot, "adventure") ||
			strings.Contains(ot, "suspend") {
			selfPlayable++
		}
	}
	score := float64(selfPlayable) / 3.0
	if hasEnabler {
		score += float64(len(seat.Exile)) / 5.0
	}
	return score
}

func (e *GameStateEvaluator) scoreStaxLock(gs *gameengine.GameState, seatIdx int) float64 {
	seat := gs.Seats[seatIdx]
	var score float64
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		ot := gameengine.OracleTextLower(p.Card)
		value := 0.0
		switch {
		case strings.Contains(ot, "nonland permanents") && strings.Contains(ot, "don't untap"):
			value = 1.5
		case strings.Contains(ot, "can't cast") || (strings.Contains(ot, "each opponent") && strings.Contains(ot, "can't")):
			value = 1.2
		case strings.Contains(ot, "additional") && strings.Contains(ot, "cost") && strings.Contains(ot, "pay"):
			value = 1.0
		case strings.Contains(ot, "enters the battlefield tapped") && strings.Contains(ot, "opponents"):
			value = 0.8
		case strings.Contains(ot, "whenever") && strings.Contains(ot, "opponent") && (strings.Contains(ot, "loses") || strings.Contains(ot, "damage")):
			value = 0.6
		case strings.Contains(ot, "sacrifice") && strings.Contains(ot, "each") && strings.Contains(ot, "opponent"):
			value = 0.8
		}
		if value > 0 {
			score += value
		}
	}
	return score / 2.0
}

// rescaleWeights adjusts evaluator dimension weights based on game state:
// game stage (early/mid/late) and relative board position (ahead/behind).
func (e *GameStateEvaluator) rescaleWeights(gs *gameengine.GameState, seatIdx int) EvalWeights {
	w := e.Weights

	turn := 1
	if gs != nil {
		turn = gs.Turn
	}

	// Game stage: 0 = early, 0.5 = mid, 1 = late.
	var stage float64
	if turn <= 5 {
		stage = 0
	} else if turn <= 12 {
		stage = float64(turn-5) / 14.0
	} else {
		stage = math.Min(1.0, 0.5+float64(turn-12)/20.0)
	}

	// Position signal: board power comparison.
	seat := gs.Seats[seatIdx]
	myPow := float64(boardPower(gs, seat))
	var oppPowSum float64
	var oppN int
	for i, s := range gs.Seats {
		if i == seatIdx || s.Lost || s.LeftGame {
			continue
		}
		oppPowSum += float64(boardPower(gs, s))
		oppN++
	}
	positionSignal := 0.0
	if oppN > 0 {
		oppPowAvg := oppPowSum / float64(oppN)
		total := myPow + oppPowAvg
		if total > 0 {
			positionSignal = (myPow - oppPowAvg) / total
		}
	}

	// Early game: ramp and card draw matter more.
	earlyFactor := math.Max(0, 1-stage*2)
	w.ManaAdvantage *= 1.0 + earlyFactor*0.3
	w.CardAdvantage *= 1.0 + earlyFactor*0.2
	w.PartnerSynergy *= 1.0 + earlyFactor*0.15

	// Late game: closing power matters more.
	lateFactor := math.Max(0, stage*2-1)
	w.ComboProximity *= 1.0 + lateFactor*0.3
	w.ThreatExposure *= 1.0 + lateFactor*0.2
	w.BoardPresence *= 1.0 + lateFactor*0.15
	w.DrainEngine *= 1.0 + lateFactor*0.25
	w.GraveyardValue *= 1.0 + lateFactor*0.2
	w.CommanderProgress *= 1.0 + lateFactor*0.15
	w.ThreatTrajectory *= 1.0 + lateFactor*0.15
	w.OpponentGraveyardThreat *= 1.0 + lateFactor*0.2
	w.StackInteraction *= 1.0 + lateFactor*0.25
	// R60 cross-cutting rebalance: life as a resource matters more once
	// the game drags past turn 12 — everyone's taken combat damage, drain
	// engines threaten lethal sooner, and the gap between "we have 30
	// life" and "we have 10 life" is now the difference between
	// surviving a swing and dying. Pre-R60 LifeResource was only bumped
	// when AHEAD, missing the case where a long late game made life
	// scarce for all seats.
	w.LifeResource *= 1.0 + lateFactor*0.15

	// Mid-game: activated abilities, synergy engines, and lock pieces peak.
	midFactor := 1.0 - math.Abs(stage-0.5)*2
	w.ActivationTempo *= 1.0 + midFactor*0.2
	w.ArtifactSynergy *= 1.0 + midFactor*0.15
	w.EnchantmentSynergy *= 1.0 + midFactor*0.15
	w.PlaneswalkerProgress *= 1.0 + midFactor*0.2
	w.StaxLockProgress *= 1.0 + midFactor*0.25

	// Behind: need to find answers or combos.
	if positionSignal < -0.3 {
		behindFactor := math.Min(1.0, (-positionSignal-0.3)*2)
		w.ComboProximity *= 1.0 + behindFactor*0.4
		w.ThreatExposure *= 1.0 + behindFactor*0.3
		w.ToolboxBreadth *= 1.0 + behindFactor*0.3
		w.DrainEngine *= 1.0 + behindFactor*0.2
		w.GraveyardValue *= 1.0 + behindFactor*0.15
		w.StackInteraction *= 1.0 + behindFactor*0.25
		w.StaxLockProgress *= 1.0 + behindFactor*0.15
	}

	// Ahead: consolidate advantage.
	if positionSignal > 0.3 {
		aheadFactor := math.Min(1.0, (positionSignal-0.3)*2)
		// R60 cross-cutting rebalance: a board-ahead deck should
		// consolidate by adding MORE board (extending the lead), not
		// just by adding more cards. Pre-R60 the ahead branch bumped
		// CardAdvantage/ManaAdvantage/LifeResource but not BoardPresence
		// — which made the evaluator treat "ahead on board" the same as
		// "behind on board" for the purpose of valuing the next creature.
		w.BoardPresence *= 1.0 + aheadFactor*0.2
		w.CardAdvantage *= 1.0 + aheadFactor*0.3
		w.ManaAdvantage *= 1.0 + aheadFactor*0.2
		w.LifeResource *= 1.0 + aheadFactor*0.2
		w.ActivationTempo *= 1.0 + aheadFactor*0.15
		w.ArtifactSynergy *= 1.0 + aheadFactor*0.1
		w.EnchantmentSynergy *= 1.0 + aheadFactor*0.1
	}

	// Plan bias: apply plan-state multipliers when set by the hat's
	// state machine. These layer ON TOP of stage/position adjustments.
	if e.PlanMultiplier != nil {
		pm := e.PlanMultiplier
		w.BoardPresence *= pm.BoardPresence
		w.CardAdvantage *= pm.CardAdvantage
		w.ManaAdvantage *= pm.ManaAdvantage
		w.LifeResource *= pm.LifeResource
		w.ComboProximity *= pm.ComboProximity
		w.ThreatExposure *= pm.ThreatExposure
		w.CommanderProgress *= pm.CommanderProgress
		w.GraveyardValue *= pm.GraveyardValue
		w.DrainEngine *= pm.DrainEngine
		w.ArtifactSynergy *= pm.ArtifactSynergy
		w.EnchantmentSynergy *= pm.EnchantmentSynergy
		w.OpponentGraveyardThreat *= pm.OpponentGraveyardThreat
		w.PartnerSynergy *= pm.PartnerSynergy
		w.ActivationTempo *= pm.ActivationTempo
		w.ToolboxBreadth *= pm.ToolboxBreadth
		w.ThreatTrajectory *= pm.ThreatTrajectory
		w.StackInteraction *= pm.StackInteraction
		w.PlaneswalkerProgress *= pm.PlaneswalkerProgress
		w.ExileZoneAssets *= pm.ExileZoneAssets
		w.StaxLockProgress *= pm.StaxLockProgress
	}

	return w
}
