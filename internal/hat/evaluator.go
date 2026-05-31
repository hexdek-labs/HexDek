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
// scoreBoard returns this seat's relative BoardPresence score: power
// differential vs opponent average plus noncreature density.
//
// r60 sweep — two targeted improvements (see effectiveBoardPower in
// poker.go + liveCreatureCount):
//
//  1. Use effectiveBoardPower instead of raw boardPower so tapped +
//     summoning-sick (no-haste) creatures are correctly de-rated. A
//     fully-committed board that can't defend should score lower than
//     a fresh untapped board with equal raw power.
//
//  2. Width bonus: per-creature-count differential vs opponent average,
//     weighted at 0.05 per body. Captures the value of multiple bodies
//     independent of total power — 6 untapped 1/1 tokens have the same
//     raw power as one untapped 6/6 but score modestly higher because
//     each token can independently block, sac, crew, equip, or feed
//     ETB-doubler triggers. Cap implicit at oppN-relative averaging.
//     0.05 per body keeps the bonus small (4-body differential = 0.2
//     score points, vs ~1.0 for a 10-power swing) so width doesn't
//     drown out the dominant power term.
func (e *GameStateEvaluator) scoreBoard(gs *gameengine.GameState, seatIdx int) float64 {
	mySeat := gs.Seats[seatIdx]
	myPow := float64(effectiveBoardPower(gs, mySeat))
	myCreatures := float64(liveCreatureCount(mySeat))

	var oppSum float64
	var oppCreaturesSum float64
	var oppN int
	for i, s := range gs.Seats {
		if i == seatIdx || s.Lost || s.LeftGame {
			continue
		}
		oppSum += float64(effectiveBoardPower(gs, s))
		oppCreaturesSum += float64(liveCreatureCount(s))
		oppN++
	}
	if oppN == 0 {
		if myPow > 0 {
			return 1
		}
		return 0
	}
	oppAvg := oppSum / float64(oppN)
	oppCreaturesAvg := oppCreaturesSum / float64(oppN)

	noncreatures := 0
	for _, p := range mySeat.Battlefield {
		if p != nil && !p.IsCreature() {
			noncreatures++
		}
	}

	// R60: commander-centric commander-on-field bonus. When Freya flags
	// the deck as commander-centric (Voltron / engine commander / >=45%
	// commander synergy / commander oracle text with >=2 engine phrases)
	// AND our commander is on the battlefield, the commander's
	// strategic value EXCEEDS its raw power: it carries the gameplan,
	// it carries commander damage toward §704.6c lethal, and its
	// removal collapses the position. Pre-r60 scoreBoard ignored
	// Strategy entirely — a Voltron Sram on board scored identically
	// to a vanilla 2/2. Mirrors the synergyBonus floor in scoreCommander
	// for the BoardPresence axis. +0.30 per commander permanent, capped
	// at 0.60 (partner pair); gated on CommanderFormat so non-EDH
	// games (where Strategy can still be set) don't fire.
	commanderBonus := 0.0
	if e.Strategy != nil && e.Strategy.IsCommanderCentric && gs.CommanderFormat {
		cmdrCount := 0
		for _, p := range mySeat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			for _, cn := range mySeat.CommanderNames {
				if p.Card.DisplayName() == cn {
					cmdrCount++
					break
				}
			}
		}
		commanderBonus = float64(cmdrCount) * 0.30
		if commanderBonus > 0.60 {
			commanderBonus = 0.60
		}
	}

	return (myPow-oppAvg)/10.0 +
		float64(noncreatures)*0.1 +
		(myCreatures-oppCreaturesAvg)*0.05 +
		commanderBonus
}

// scoreCards: hand size + library depth + persistent draw engines on
// battlefield + castable-from-exile cards + opponent-triggered value
// engines, all relative to opponent average. The engine + castable-
// exile + opp-trigger-value terms capture virtual card advantage that
// pure hand-count misses (a 4-card hand behind a Rhystic Study +
// Phyrexian Arena + Smothering Tithe is a different game than a
// 4-card hand behind nothing).
//
// The opp-trigger-value term — the Rhystic Study / Smothering Tithe
// class — is the long-tail companion to drawEngineCredit. Where
// drawEngineCredit captures a flat per-engine rate that assumes a
// representative pod, this term scales with the actual living-opp
// count: 0.4 per engine per living opponent. A Smothering Tithe at a
// 4-player table with 3 live opponents is +1.2 here; the same card
// when the table has been whittled to a 1v1 is only +0.4. Engines
// that are also draw engines (Rhystic, Mystic Remora, Esper Sentinel)
// stack the bonus on top of their flat 1.5 rate-class credit —
// intentional, since drawEngineCredit is calibrated against a typical
// pod and this term tunes for the actual count. Treasure / mana
// engines that are NOT draw engines (Smothering Tithe, Trouble in
// Pairs) get nonzero credit here for the first time.
func (e *GameStateEvaluator) scoreCards(gs *gameengine.GameState, seatIdx int) float64 {
	seat := gs.Seats[seatIdx]
	myHand := float64(len(seat.Hand))
	myLib := float64(len(seat.Library))
	myEngines := drawEngineCredit(seat)
	myCastExile := float64(castableExileCount(gs, seatIdx))
	myOppValue := float64(opponentTriggerValueCount(seat))
	myQuality := handQualityBonus(seat)

	var oppHand, oppLib, oppEngines, oppCastExile, oppOppValue, oppQuality float64
	var oppN int
	for i, s := range gs.Seats {
		if i == seatIdx || s.Lost || s.LeftGame {
			continue
		}
		oppHand += float64(len(s.Hand))
		oppLib += float64(len(s.Library))
		oppEngines += drawEngineCredit(s)
		oppCastExile += float64(castableExileCount(gs, i))
		oppOppValue += float64(opponentTriggerValueCount(s))
		oppQuality += handQualityBonus(s)
		oppN++
	}
	if oppN == 0 {
		return myHand/7.0 + myEngines*0.4 + myCastExile*0.3 + myQuality
	}
	avgHand := oppHand / float64(oppN)
	avgLib := oppLib / float64(oppN)
	avgEngines := oppEngines / float64(oppN)
	avgCastExile := oppCastExile / float64(oppN)
	avgOppValue := oppOppValue / float64(oppN)
	avgQuality := oppQuality / float64(oppN)

	return (myHand-avgHand)/4.0 +
		(myLib-avgLib)/40.0 +
		(myEngines-avgEngines)*0.4 +
		(myCastExile-avgCastExile)*0.3 +
		(myOppValue-avgOppValue)*0.4*float64(oppN) +
		(myQuality - avgQuality)
}

// handQualityBonus collapses three hand-quality signals into a single
// delta that scoreCards applies symmetrically (my - opp avg). Pre-r60
// scoreCards was hand-COUNT only — a 5-card hand of dead high-CMC
// spells scored identically to a 5-card hand with 2 cantrips + 1
// tutor + 2 castable threats. The signals:
//
//   - Tutor-in-hand quality bonus (+0.15 per tutor, cap +0.45). A
//     tutor is a virtual draw of the BEST card in the deck for the
//     current state — the existing scoreCombo path already gives
//     them combo-piece credit, but scoreCards was blind. 3-tutor
//     saturation mirrors the combo-side cap.
//   - Cantrip-in-hand quality bonus (+0.08 per cantrip, cap +0.32).
//     Brainstorm / Ponder / Preordain / Opt / Sleight of Hand —
//     cheap (CMC <= 2) instants/sorceries with "draw a card" oracle.
//     Replaces-itself + filters, so each is roughly 0.4 of a real
//     card; the cap caps at 4 cantrips saturated.
//   - Hand-flood dead-card penalty (-0.08 per dead, cap -0.40). A
//     card whose CMC > current lands + 3 won't cast for 3+ turns,
//     so it's marginally usable at best; treat it as <1 card.
//     Captures the "I have 6 cards but 4 are 8-drops on a 3-land
//     board" flooded scenario the raw count credited at full value.
//
// Symmetrically applied to opp hands too — in determinized rollouts
// opp hands are randomized but still walkable, and the dimension
// scoring already trusts that opp hand contents are visible to the
// evaluator (myHand vs oppHand delta needs both to be readable).
func handQualityBonus(seat *gameengine.Seat) float64 {
	if seat == nil || len(seat.Hand) == 0 {
		return 0
	}
	// Land count drives the dead-card threshold.
	lands := 0
	for _, p := range seat.Battlefield {
		if p != nil && p.IsLand() && !p.PhasedOut {
			lands++
		}
	}
	deadThreshold := lands + 3

	tutorCount := 0
	cantripCount := 0
	deadCount := 0
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		isLand := false
		for _, t := range c.Types {
			if t == "land" {
				isLand = true
				break
			}
		}
		if isLand {
			// Lands count as "dead" beyond a buffer when the seat is
			// flooded — 2+ lands in hand AND 5+ on field is real flood.
			if lands >= 5 {
				deadCount++
			}
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc > deadThreshold {
			deadCount++
		}
		ot := gameengine.OracleTextLower(c)
		// Cantrip: CMC <= 2 instant/sorcery with "draw a card".
		if cmc <= 2 && (typeLineContains(c, "instant") || typeLineContains(c, "sorcery")) &&
			strings.Contains(ot, "draw a card") {
			cantripCount++
			continue // a cantrip-tutor would double-count; cantrip wins
		}
		// Tutor: "search your library for" OR name contains "tutor".
		if strings.Contains(ot, "search your library for") ||
			strings.Contains(strings.ToLower(c.DisplayName()), "tutor") {
			tutorCount++
		}
	}

	tutorBonus := 0.15 * float64(tutorCount)
	if tutorBonus > 0.45 {
		tutorBonus = 0.45
	}
	cantripBonus := 0.08 * float64(cantripCount)
	if cantripBonus > 0.32 {
		cantripBonus = 0.32
	}
	deadPenalty := 0.08 * float64(deadCount)
	if deadPenalty > 0.40 {
		deadPenalty = 0.40
	}
	return tutorBonus + cantripBonus - deadPenalty
}

// drawEngineCredit estimates the seat's virtual cards-per-turn from
// persistent draw engines on its battlefield (Rhystic Study, Phyrexian
// Arena, Mystic Remora, Esper Sentinel, Sylvan Library, Howling Mine,
// Mind's Eye, ...). Each engine contributes a rate-class weight via
// drawEngineRate. Capped at 4.0 total to bound stax/wheel-board
// outliers.
//
// r60 retune: pre-r60 every engine contributed 1.0 uniformly. Rhystic
// Study (which fires 3-6 times per turn cycle in a 4-player pod) was
// treated identically to Phyrexian Arena (1 fire per turn cycle) and
// Howling Mine (1 fire per turn cycle that ALSO helps opponents).
// Differentiating these rates produces strictly more honest
// CardAdvantage scoring without changing the dimension's relative
// weight. See drawEngineRate for the rate-class table.
func drawEngineCredit(seat *gameengine.Seat) float64 {
	if seat == nil {
		return 0
	}
	const cap = 4.0
	total := 0.0
	for _, p := range seat.Battlefield {
		w := drawEngineRate(p)
		if w == 0 {
			continue
		}
		total += w
		if total >= cap {
			return cap
		}
	}
	return total
}

// drawEngineRate returns the per-engine weight, classifying by oracle-
// text shape:
//
//	0.0   — not a persistent draw engine
//	0.6   — symmetric draw ("each player draws") — Howling Mine, Temple
//	        Bell, Font of Mythos, Dictate of Kruphix. The owner gets
//	        the cards but so does every opponent — a 4-player table
//	        diffuses 75% of the value to other seats. Weighting these
//	        at full credit double-counted shared cards as if they were
//	        purely our advantage.
//	1.0   — upkeep-cadenced / passive controller-only draw — Phyrexian
//	        Arena, Sylvan Library, Mind's Eye. One fire per turn cycle
//	        for one card. The standard pre-r60 baseline.
//	1.5   — opponent-action-triggered draw — Rhystic Study, Mystic
//	        Remora, Esper Sentinel. Each opponent's spell triggers a
//	        draw window; in a 4-player pod with 3 opponents casting
//	        2-3 spells each per turn cycle, these engines fire 4-9
//	        times per turn. Even derated for the unless-pay clause +
//	        Remora's cumulative-upkeep churn, the throughput is ~1.5x
//	        the upkeep-cadenced baseline.
//
// r60 follow-up: two new rate classes for virtual CA that pure
// "you draw a card" detection missed:
//
//	0.8   — impulse-draw / exile-cast engines — recurring "exile the
//	        top card of your library; you may play/cast it" (Outpost
//	        Siege, Wild-Magic Sorcerer, Etali Primal Storm, Prosper
//	        Tome-Bound, Faldorn). Each fires ~1x per cycle and adds a
//	        castable card; rated below the 1.0 upkeep baseline because
//	        the cast is conditional on mana availability and spell
//	        relevance (typical real-world hit rate ~80%). The result
//	        AFTER firing is also picked up by castableExileCount, but
//	        scoring only the result undercounts the engine on the turn
//	        it lands (no exile yet) and a seat that has popped its
//	        impulse card every turn shows no engine value at all.
//	0.3   — persistent scry / surveil engines — recurring "scry N" /
//	        "surveil N" triggers (Path of Ancestry, Ledger Shredder).
//	        Filters draw quality rather than adding a card; weighted
//	        well below a true engine since it's pseudo-CA, but >0
//	        because a 30%+ improvement in next-draw quality is a real
//	        cards-per-turn equivalent over a game.
//
// The classifier is conservative: a card needs to match the rate-class
// pattern explicitly to upgrade; ambiguous shapes (e.g. "whenever a
// creature you control dies, draw a card") stay at the 1.0 baseline
// since their rate depends on board state.
func drawEngineRate(p *gameengine.Permanent) float64 {
	if p == nil || p.Card == nil {
		return 0
	}
	ot := gameengine.OracleTextLower(p.Card)
	if ot == "" {
		return 0
	}
	// Most-specific gates first: impulse and scry/surveil engines
	// don't say "you draw a card" so they fall through
	// isPersistentDrawEngine entirely. Check them before the
	// draw-engine gate so they register.
	if isImpulseDrawEngine(ot) {
		return 0.8
	}
	if isPersistentScryEngine(ot) {
		return 0.3
	}
	if !isPersistentDrawEngine(p) {
		return 0
	}
	// Symmetric (each-player-draws) effects diffuse their value across
	// the table. Detect before the opp-trigger gate so a "each player
	// may draw" doesn't accidentally upgrade. Multiple phrasings:
	//   - "each player draws a card" (Temple Bell)
	//   - "each player may draw" (rare modal)
	//   - "each player's draw step" (Howling Mine, Font of Mythos,
	//     Dictate of Kruphix — table-wide draw-step enabler with "that
	//     player draws an additional card" follow-up)
	if strings.Contains(ot, "each player draws") ||
		strings.Contains(ot, "each player may draw") ||
		strings.Contains(ot, "each player's draw step") {
		return 0.6
	}
	// Opponent-action triggers fire ~3-6 times per turn cycle in a 4p
	// pod vs the upkeep-cadenced 1 fire / cycle baseline. The "unless
	// that player pays {N}" Rhystic / Remora clause is intentionally
	// not parsed out — the 1.5 weight already accounts for the partial
	// pay-through rate.
	if strings.Contains(ot, "whenever an opponent casts") ||
		strings.Contains(ot, "whenever an opponent cycles") ||
		strings.Contains(ot, "whenever a player other than you") {
		return 1.5
	}
	return 1.0
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

// isImpulseDrawEngine returns true when ot describes a recurring
// "exile the top card of your library; you may play/cast it" effect.
// Matches Outpost Siege (Khans mode, upkeep), Wild-Magic Sorcerer
// (attack-trigger), Etali Primal Storm (attack-trigger, each player's
// library), Prosper Tome-Bound (end-step), Faldorn (combat-trigger).
//
// Both the recurring cue ("whenever" / "at the beginning") AND an
// exile-then-cast pair are required — one-shot impulse spells like
// "Light Up the Stage" or static "you may play the top card" (Future
// Sight, Vizier of the Menagerie, Augur of Autumn) intentionally
// don't register as recurring exile-cast engines.
func isImpulseDrawEngine(ot string) bool {
	if !strings.Contains(ot, "whenever") && !strings.Contains(ot, "at the beginning") {
		return false
	}
	// Exile cue must reference a library (your / each player's /
	// their). "exile the top card" of anything else (e.g. a stack
	// item, an opponent's hand) is a different effect family.
	hasExile := strings.Contains(ot, "exile the top card")
	hasLibrary := strings.Contains(ot, "your library") ||
		strings.Contains(ot, "each player's library") ||
		strings.Contains(ot, "their library")
	if !hasExile || !hasLibrary {
		return false
	}
	// Cast-permission cue — both "play" and "cast" variants, with
	// the leading subject phrasing ("you may play", "you may cast")
	// to avoid matching effects that exile and merely look at the
	// card (Bonders' Enclave-style top-deck inspection).
	return strings.Contains(ot, "you may play") ||
		strings.Contains(ot, "you may cast") ||
		strings.Contains(ot, "play that card") ||
		strings.Contains(ot, "cast that card") ||
		strings.Contains(ot, "play it this turn") ||
		strings.Contains(ot, "cast it this turn")
}

// isPersistentScryEngine returns true when ot describes a recurring
// scry or surveil trigger. Matches Path of Ancestry ("whenever you
// cast a creature spell that shares a creature type with your
// commander, scry 1"), Ledger Shredder ("whenever you cast the second
// spell each turn, surveil 2"), and upkeep-cadenced scry sources.
//
// Recurring cue ("whenever" / "at the beginning") is required to
// exclude activated-cost scry sources (Crystal Ball, Soothsaying)
// whose throughput depends on mana spend and is already accounted for
// implicitly in ManaAdvantage opportunity-cost. One-shot ETB scry on
// creature spells (Omenspeaker, Augury Owl) also doesn't register —
// they're not persistent.
func isPersistentScryEngine(ot string) bool {
	if !strings.Contains(ot, "whenever") && !strings.Contains(ot, "at the beginning") {
		return false
	}
	return strings.Contains(ot, "scry ") || strings.Contains(ot, "surveil ")
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

// opponentTriggerValueCount counts nonland permanents on seat's
// battlefield that match the Rhystic Study / Smothering Tithe class:
// passive triggers that fire on an opponent action ("whenever an
// opponent casts / draws / attacks / cycles / sacrifices...") and
// produce a free resource for the controller (a card, a treasure, a
// token, or mana). The count is capped at 4 — mirrors the
// drawEngineCredit cap so multi-engine stax/value boards don't blow
// out the dimension.
//
// Scoring scales by living-opponent count in scoreCards (0.4 per
// engine per opp). A Smothering Tithe at a 4-player table with 3
// live opps is worth +1.2 in CA units; the same card at a 1v1 is
// only +0.4. Captures the "Rhystic in cEDH vs Rhystic in a duel"
// asymmetry the flat draw-engine rate-class table can't express.
func opponentTriggerValueCount(seat *gameengine.Seat) int {
	if seat == nil {
		return 0
	}
	const cap = 4
	n := 0
	for _, p := range seat.Battlefield {
		if !isOpponentTriggeredValueEngine(p) {
			continue
		}
		n++
		if n >= cap {
			return cap
		}
	}
	return n
}

// isOpponentTriggeredValueEngine returns true when p is a nonland
// permanent whose oracle text describes a recurring trigger on an
// opponent action that gives the controller a free resource.
//
// Anchor cards in the class:
//   - Rhystic Study: opp casts → you may draw
//   - Mystic Remora: opp casts noncreature → you draw
//   - Esper Sentinel: opp casts first noncreature → you draw
//   - Smothering Tithe: opp draws → you create a treasure
//   - Trouble in Pairs: opp's second action → you draw + treasure
//
// Excluded by design:
//   - Lands (skipped — different class)
//   - Symmetric "each player" effects (Howling Mine — already
//     discounted in drawEngineRate; not opp-asymmetric)
//   - Own-upkeep value engines (Phyrexian Arena, Black Market
//     Connections — fire on YOUR upkeep, not opp action)
//   - Damage-only opp-triggers (Underworld Dreams — punishes opp
//     draws without giving you a resource; that's ThreatExposure
//     not CardAdvantage)
func isOpponentTriggeredValueEngine(p *gameengine.Permanent) bool {
	if p == nil || p.Card == nil {
		return false
	}
	// Skip lands. The class is scoped to nonland permanents per
	// the design — lands don't typically generate per-opp-action
	// resources at a meaningful rate, and the opp-cast trigger
	// pattern on a land (Bountiful Promenade etc.) is structurally
	// different.
	for _, t := range p.Card.Types {
		if t == "land" {
			return false
		}
	}
	ot := gameengine.OracleTextLower(p.Card)
	if ot == "" {
		return false
	}
	// Recurring opp-action trigger cue. Both phrasings — "whenever
	// an opponent" and "whenever a player other than you" — are
	// standard for this class.
	hasOppTrigger := strings.Contains(ot, "whenever an opponent") ||
		strings.Contains(ot, "whenever a player other than you")
	if !hasOppTrigger {
		return false
	}
	// Free-resource consequence. The four resource families that
	// qualify as long-tail CA: card draw, treasure tokens, generic
	// token creation, and direct mana addition.
	if strings.Contains(ot, "you draw a card") ||
		strings.Contains(ot, "you may draw a card") ||
		strings.Contains(ot, "you draw a") ||
		strings.Contains(ot, "create a treasure") ||
		strings.Contains(ot, "treasure token") ||
		strings.Contains(ot, "you create a") ||
		strings.Contains(ot, "add {") {
		return true
	}
	return false
}

// scoreMana: mana source count relative to average, plus color coverage.
func (e *GameStateEvaluator) scoreMana(gs *gameengine.GameState, seatIdx int) float64 {
	// R60: fold fast-mana extra-yield into both my and opp source totals.
	// CountManaRocksAndLands credits each mana permanent at 1; that flat
	// rate treats Sol Ring (taps for {2}) and Mana Vault (taps for {3})
	// identically to a Plains. fastManaExtraYield returns ONLY the extra
	// mana above that baseline so the existing count semantics survive
	// while the asymmetric yield surfaces in the count delta. Applied
	// symmetrically to opp totals so a Sol Ring on each side of the
	// table reads neutral, not us-down.
	mySources := float64(CountManaRocksAndLands(gs.Seats[seatIdx]) + fastManaExtraYield(gs.Seats[seatIdx]))

	var oppSum float64
	var oppN int
	for i, s := range gs.Seats {
		if i == seatIdx || s.Lost || s.LeftGame {
			continue
		}
		oppSum += float64(CountManaRocksAndLands(s) + fastManaExtraYield(s))
		oppN++
	}

	var rawScore float64
	if oppN == 0 {
		rawScore = mySources / 5.0
	} else {
		rawScore = (mySources - oppSum/float64(oppN)) / 4.0
	}

	// R60: ramp-in-hand future-source preview. Pre-r60 a hand with 3
	// Cultivates scored identically to a hand with 3 vanilla 3-drops on
	// the mana axis — the future-source pipeline was invisible. Credit
	// each land-search ramp spell, mana dork in hand, and mana rock in
	// hand at 0.5 fractional sources, capped at +2.0 (4 ramp pieces is
	// the bottleneck before land-drop saturation matters more than
	// ramp). Divided by 4 to match the per-source weighting on the raw
	// count delta above.
	rawScore += rampInHandFutureSources(gs.Seats[seatIdx]) / 4.0

	// r60: response-mana bonus on opponent turns. When it's NOT our turn,
	// only untapped sources are usable for instants / counters / activated
	// removal. A deck with 8 sources all tapped is worse off at end-of-
	// opponent-turn than one with 6 untapped — the same total raw count
	// hides a real tempo asymmetry. Add a small bonus proportional to the
	// untapped-source ratio when responding. Capped at +0.25 so it doesn't
	// override the primary count delta. On our own turn the bonus is 0:
	// tapped sources will untap on the next upkeep, so the distinction is
	// noise within the turn.
	if gs.Active != seatIdx && mySources > 0 {
		untapped := float64(CountUntappedManaSources(gs.Seats[seatIdx]))
		rawScore += 0.25 * (untapped / mySources)
	}

	// r60: in-hand color-screw penalty. Distinct from the deck-level
	// ColorDemand coverage check below — that measures STRATEGIC need
	// (what the deck wants across a game), this measures TACTICAL
	// playability of cards in our hand right now. A mono-blue deck
	// whose hand holds a {UU} spell with 0 untapped Islands is screwed
	// even though its strategic color demand is fully covered. The
	// penalty runs BEFORE the early-return below so it fires even when
	// no Strategy is loaded (the deck-level coverage check is a no-op
	// in that case, but the hand-level screw signal still applies).
	rawScore += handColorScrewPenalty(gs.Seats[seatIdx])

	// r60: ritual-fuel bonus. Dark Ritual / Cabal Ritual / Seething
	// Song / Pyretic Ritual / Rite of Flame / Infernal Plunge produce
	// net positive mana for ONE turn but only if there's a spell to
	// chain into that we couldn't otherwise cast. A ritual in hand
	// with no chain target is dead this turn — we don't credit it.
	// This is conditional fast mana: gated on the existence of a
	// castable target whose CMC exceeds the seat's current untapped
	// source ceiling but fits inside (ceiling + ritual net).
	rawScore += ritualFuelBonus(gs.Seats[seatIdx])

	if e.Strategy == nil || e.Strategy.ColorDemand == nil {
		return rawScore
	}

	// r60: depth-weighted color coverage. The previous binary check
	// (`fieldColorSources(...) > 0`) gave a deck with 1 black source the
	// same credit as one with 4 — but a {BBBB}-pip demand profile needs
	// actual depth, not just presence. Switch to a sqrt-decay weight: 1
	// source = 0.5 credit, 2 sources = 0.71, 4 sources = 1.0, more = 1.0.
	// Preserves the existing average-credit semantics (0.5 baseline) so
	// the (coverage - 0.5) * 0.8 final step still nets to 0 at "one
	// source per demanded color" — only DEEP coverage shifts the score.
	totalDemand := 0
	coveredDemand := 0.0
	for col, demand := range e.Strategy.ColorDemand {
		if demand < 2 {
			continue
		}
		totalDemand += demand
		coveredDemand += float64(demand) * colorDepthWeight(fieldColorSources(gs.Seats[seatIdx], col))
	}
	if totalDemand > 0 {
		coverage := coveredDemand / float64(totalDemand)
		rawScore += (coverage - 0.5) * 0.8
	}

	return rawScore
}

// handColorScrewPenalty returns a non-positive penalty when the seat's
// hand contains spells with colored-pip requirements that the seat's
// UNTAPPED battlefield can't satisfy this turn.
//
// Demand model: for each playable hand card (skip lands and cards with
// no ManaCostString), parse the Pure pip array and accumulate the MAX
// per-color requirement across all hand cards. Max-per-color (rather
// than sum) is the right binding constraint because spells cast
// serially — one stuck {UUU} Cryptic Command is the binding case, not
// three different {U}-cost spells (which can be cast across turns).
//
// Hybrid pips are intentionally ignored ({R/G} can be paid by either
// color; counting them as binding double-counts a flexible requirement).
//
// Penalty: -0.15 per shortfall pip per color, summed across colors,
// clamped at -1.0. The -0.15 magnitude is calibrated below the
// count-delta weight ((mySources - oppAvg)/4 = ~0.25 per source) so
// color screw is felt but not catastrophic — a one-pip shortfall on
// one color is roughly equivalent to having half a source fewer.
func handColorScrewPenalty(seat *gameengine.Seat) float64 {
	if seat == nil || len(seat.Hand) == 0 {
		return 0
	}
	// handDemand[colorBit] = max pure-pip requirement of that color
	// across any single playable hand card. Indexed by W/U/B/R/G slots.
	var handDemand [6]int
	const (
		slotW = 0 // matches gameengine.manaSlot* layout
		slotU = 1
		slotB = 2
		slotR = 3
		slotG = 4
		// slotC (colorless requirement, Eldrazi) intentionally skipped —
		// untapped lands of any color satisfy generic mana, and true {C}
		// requirements are rare enough that adding the special case
		// inflates the penalty surface for a tiny minority of cards.
	)
	for _, card := range seat.Hand {
		if card == nil || card.ManaCostString == "" {
			continue
		}
		// Skip lands: they don't cast.
		isLand := false
		for _, t := range card.Types {
			if t == "land" {
				isLand = true
				break
			}
		}
		if isLand {
			continue
		}
		req := gameengine.ParseCostRequirements(card.ManaCostString)
		for slot := 0; slot < 5; slot++ {
			if req.Pure[slot] > handDemand[slot] {
				handDemand[slot] = req.Pure[slot]
			}
		}
	}
	// Map each demanded color slot to its untapped source count.
	slotColor := [5]string{"W", "U", "B", "R", "G"}
	penalty := 0.0
	for slot := 0; slot < 5; slot++ {
		demand := handDemand[slot]
		if demand <= 0 {
			continue
		}
		untapped := untappedFieldColorSources(seat, slotColor[slot])
		if untapped < demand {
			shortfall := demand - untapped
			penalty -= 0.15 * float64(shortfall)
		}
	}
	if penalty < -1.0 {
		penalty = -1.0
	}
	return penalty
}

// colorDepthWeight maps a per-color source count to a [0..1] credit
// using a sqrt-decay so the first 1-2 sources contribute the most and
// the curve asymptotes at 4 sources. Tuned so 1 source ≈ 0.5 (parity
// with the pre-r60 binary coverage) and 4+ ≈ 1.0.
//
//	count | weight
//	------|-------
//	  0   | 0.00
//	  1   | 0.50
//	  2   | 0.71
//	  3   | 0.87
//	  4+  | 1.00
func colorDepthWeight(n int) float64 {
	if n <= 0 {
		return 0
	}
	if n >= 4 {
		return 1.0
	}
	// sqrt-based decay: 0.5 at 1, 0.71 at 2, 0.87 at 3, 1.0 at 4.
	return math.Sqrt(float64(n)) * 0.5
}

// ritualNetByName lists the canonical mono-color rituals whose net
// mana production is positive and unconditional (no graveyard /
// threshold / X-cost gating). The map value is the net mana the
// ritual produces standalone: CMC of the cost subtracted from the
// total mana added to the pool.
//
//	Dark Ritual      {B}    → {B}{B}{B}             net +2
//	Cabal Ritual     {1}{B} → {B}{B}{B}             net +1 (threshold +5 skipped)
//	Pyretic Ritual   {1}{R} → {R}{R}{R}             net +1
//	Rite of Flame    {R}    → {R}{R}                net +1 (graveyard scaling skipped)
//	Seething Song    {2}{R} → {R}{R}{R}{R}{R}       net +2
//	Infernal Plunge  {R}    → {R}{R}{R} (sac creat) net +2 (sac cost is not mana)
//
// Desperate Ritual ({2}{R} → {R}{R}{R}) is intentionally excluded —
// standalone net is 0; its value lives in Arcane splice chains the
// eval can't model. Other rituals (Manamorphose, Pulse of the Forge,
// Rain of Filth, Lotus Petal) are excluded because they're either
// mana-neutral cantrips or rely on board state (lands you control,
// X-cost) the eval would need to model separately.
var ritualNetByName = map[string]int{
	"dark ritual":     2,
	"cabal ritual":    1,
	"pyretic ritual":  1,
	"rite of flame":   1,
	"seething song":   2,
	"infernal plunge": 2,
}

// ritualNetMana returns the net mana a ritual card in hand provides
// when cast standalone. 0 for non-rituals or cards we don't model.
// Lowercase-name lookup so localized printings (foreign-language
// cards with same English name field) and case variants both match.
func ritualNetMana(c *gameengine.Card) int {
	if c == nil {
		return 0
	}
	return ritualNetByName[strings.ToLower(c.DisplayName())]
}

// ritualFuelBonus returns a small positive eval bonus when the seat
// holds at least one ritual AND at least one castable spell whose
// CMC can ONLY be paid by chaining the ritual's burst mana. A ritual
// in hand with no chain target is dead this turn — we don't credit
// it (the user can't capitalize on the fast mana, so the eval
// shouldn't treat it as having tempo value).
//
// Chain-target test:
//   - playableCap = CountUntappedManaSources(seat) (color-agnostic
//     ceiling on what we can cast right now using only untapped
//     lands/cheap rocks)
//   - totalRitualNet = sum of net mana across all rituals in hand
//   - eligible target = any non-ritual hand spell whose CMC is
//     strictly GREATER than playableCap (we couldn't cast it without
//     the ritual) and AT MOST playableCap + totalRitualNet (the
//     ritual lets us cast it). Lands are skipped (they don't "cast"
//     in the mana-pool sense).
//
// Bonus magnitude: 0.10 per net mana from the rituals, capped at
// +0.4 so a hand stuffed with rituals doesn't dominate the eval
// relative to the count delta (~0.25 per source). The 0.10 weight
// is calibrated symmetrically against the color-screw -0.15 — a
// ritual is conditionally good (must have chain target) while
// color screw is unconditionally bad, so the bonus per pip is
// smaller than the penalty per pip.
func ritualFuelBonus(seat *gameengine.Seat) float64 {
	if seat == nil || len(seat.Hand) == 0 {
		return 0
	}
	totalRitualNet := 0
	for _, c := range seat.Hand {
		if n := ritualNetMana(c); n > 0 {
			totalRitualNet += n
		}
	}
	if totalRitualNet == 0 {
		return 0
	}
	playableCap := CountUntappedManaSources(seat)
	ceiling := playableCap + totalRitualNet
	// Find any non-ritual hand spell whose CMC needs the ritual mana.
	hasChainTarget := false
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		if ritualNetMana(c) > 0 {
			continue // skip the rituals themselves
		}
		// Skip lands — they don't "cast" with mana.
		isLand := false
		for _, t := range c.Types {
			if t == "land" {
				isLand = true
				break
			}
		}
		if isLand {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc > playableCap && cmc <= ceiling {
			hasChainTarget = true
			break
		}
	}
	if !hasChainTarget {
		return 0
	}
	bonus := 0.10 * float64(totalRitualNet)
	if bonus > 0.4 {
		bonus = 0.4
	}
	return bonus
}

// CountUntappedManaSources counts the seat's currently-untapped mana-
// producing permanents — used by scoreMana to weight response mana on
// opponent turns. Mirrors CountManaRocksAndLands (lands + cheap mana
// artifacts) but only counts sources with `Tapped == false`.
// fastManaExtraYieldByName lists fast-mana permanents whose per-tap
// yield exceeds a basic land's 1 mana. The map value is the EXTRA
// mana per turn above the baseline 1 that CountManaRocksAndLands
// already credits — so Sol Ring (taps for {2}) returns 1, Mana Vault
// (taps for {3}) returns 2. Drives the asymmetric fast-mana bonus
// in scoreMana — pre-r60 a Sol Ring counted identically to a Plain.
//
// Mana Vault / Grim Monolith / Basalt Monolith all produce {3} per
// activation; even with the don't-untap drawback they cycle once per
// game-turn-pair in practice. Conservative +2 credit reflects this.
//
// Moxen intentionally excluded — they tap for 1 mana like a land;
// their timing advantage (zero-cost, no land drop) is already
// captured by raw battlefield count on early turns.
//
// Black Lotus excluded — one-shot sac for +3 burst is a different
// shape (one-time tempo spike, not steady mana), covered separately
// by the existing ritualFuelBonus model when chained.
var fastManaExtraYieldByName = map[string]int{
	"sol ring":         1,
	"mana crypt":       1,
	"mana vault":       2,
	"grim monolith":    2,
	"basalt monolith":  2,
	"thran dynamo":     1, // {4} cost, taps for {3} → +2 yield but high cost; conservative +1
	"worn powerstone":  1, // ETB tapped, then taps for {2}; net +1
	"gilded lotus":     2, // {5}, taps for {3} of any one color; +2 yield
}

// fastManaExtraYield returns the sum of per-turn extra mana above
// baseline-1 contributed by fast-mana permanents on this seat's
// battlefield. Uses the curated name map; phased-out permanents and
// permanents that don't untap on the next upkeep (e.g. Winter Orb
// targets) are skipped at the call site by reading the seat directly.
// CountManaRocksAndLands already credits each of these at 1; this
// helper returns ONLY the extra above that 1.
func fastManaExtraYield(seat *gameengine.Seat) int {
	if seat == nil {
		return 0
	}
	total := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || p.PhasedOut {
			continue
		}
		if extra, ok := fastManaExtraYieldByName[strings.ToLower(p.Card.DisplayName())]; ok {
			total += extra
		}
	}
	return total
}

// rampInHandFutureSources estimates fractional future mana sources
// based on cards in hand. Ramp spells (land-search-and-put or direct-
// add) and mana rocks in hand will deploy next turn or two, so they
// represent real future mana advantage the raw battlefield count
// misses. Pre-r60, a hand with 3 Cultivates scored identically to a
// hand with 3 dead non-ramp cards.
//
// Scoring: 0.5 per ramp spell or in-hand mana rock, capped at 2.0
// (4 ramp pieces in hand is the saturation point — past that the
// extra cards are bottlenecked by land drops, not future-source
// potential). The 0.5 fractional weight accounts for the cast cost
// (we have to spend a turn casting the ramp) and the asymmetric
// timing relative to deployed sources.
//
// Recognized shapes:
//   - "search your library for a/two land" + ("battlefield" / "into
//     your hand") → Cultivate, Kodama's Reach, Rampant Growth, Three
//     Visits, Farseek, Skyshroud Claim, Nature's Lore, Wood Elves
//     (creature with ETB ramp)
//   - "add" + "mana" creature/instant ramp shapes (Llanowar Elves,
//     Birds of Paradise, Arbor Elf — these are dorks waiting in hand)
//   - mana rocks in hand: CMC <= 3 artifact + (mana | {t}) oracle
//     pattern (matches CountManaRocksAndLands's battlefield rule)
func rampInHandFutureSources(seat *gameengine.Seat) float64 {
	if seat == nil || len(seat.Hand) == 0 {
		return 0
	}
	rampCount := 0
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		// Skip lands — they're already in the "next land drop" pipeline,
		// not ramp.
		isLand := false
		for _, t := range c.Types {
			if t == "land" {
				isLand = true
				break
			}
		}
		if isLand {
			continue
		}
		ot := gameengine.OracleTextLower(c)
		// Land-search ramp.
		if strings.Contains(ot, "search your library") &&
			(strings.Contains(ot, "land card") || strings.Contains(ot, "basic land") ||
				strings.Contains(ot, "forest") || strings.Contains(ot, "plains") ||
				strings.Contains(ot, "island") || strings.Contains(ot, "swamp") ||
				strings.Contains(ot, "mountain")) &&
			(strings.Contains(ot, "battlefield") || strings.Contains(ot, "onto the battlefield")) {
			rampCount++
			continue
		}
		// Mana dork / instant ramp ("add ... mana" oracle).
		if strings.Contains(ot, "add") &&
			(strings.Contains(ot, "{w}") || strings.Contains(ot, "{u}") ||
				strings.Contains(ot, "{b}") || strings.Contains(ot, "{r}") ||
				strings.Contains(ot, "{g}") || strings.Contains(ot, "{c}") ||
				strings.Contains(ot, "mana of any color")) {
			rampCount++
			continue
		}
		// Mana rock in hand (artifact, low CMC, mana-producing).
		isArtifact := false
		for _, t := range c.Types {
			if t == "artifact" {
				isArtifact = true
				break
			}
		}
		if isArtifact && gameengine.ManaCostOf(c) <= 3 &&
			(strings.Contains(ot, "mana") || strings.Contains(ot, "{t}")) {
			rampCount++
		}
	}
	bonus := 0.5 * float64(rampCount)
	if bonus > 2.0 {
		bonus = 2.0
	}
	return bonus
}

func CountUntappedManaSources(seat *gameengine.Seat) int {
	if seat == nil {
		return 0
	}
	n := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || p.Tapped {
			continue
		}
		if p.IsLand() {
			n++
			continue
		}
		if typeLineContains(p.Card, "artifact") && gameengine.ManaCostOf(p.Card) <= 3 {
			ot := gameengine.OracleTextLower(p.Card)
			if strings.Contains(ot, "mana") || strings.Contains(ot, "{t}") {
				n++
			}
		}
	}
	return n
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

	// R60: effective-clock awareness via commander damage. Pre-r60 the
	// dimension read seat.Life only, but in commander our practical
	// clock is min(seat.Life, 21 - max_cmdr_dmg_taken). At life=12
	// with 14 commander damage from one source, we have only 7
	// commander-damage-points to live — that's a much shorter clock
	// than the raw life suggests. Only narrows the clock when commander
	// damage is in the relevant band (>=14, matching the Voltron-side
	// "two-thirds" threshold from scoreCommander), and only when the
	// derived clock is actually shorter than raw life — a commander
	// damage of 5 doesn't shorten a life=12 clock.
	effectiveLife := seat.Life
	if gs.CommanderFormat && seat.CommanderDamage != nil {
		maxCmdrDmg := 0
		for _, cmdMap := range seat.CommanderDamage {
			for _, dmg := range cmdMap {
				if dmg > maxCmdrDmg {
					maxCmdrDmg = dmg
				}
			}
		}
		if maxCmdrDmg >= 14 {
			cmdrClock := 21 - maxCmdrDmg
			if cmdrClock < effectiveLife {
				effectiveLife = cmdrClock
			}
		}
	}
	ratio := float64(effectiveLife) / starting

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
		if isLifeAsResourcePayoff(ot) {
			hasLifePayoff = true
			break
		}
	}
	// R60: pay-life tutors / draw engines in hand also count as life
	// payoffs, but only when the seat has enough life to actually pay
	// the activation cost. Vampiric Tutor / Imperial Seal cost 2 life;
	// Necropotence pays X life for cards; Bargain-style draws pay life.
	// Threshold of 4 life keeps the payoff valid for back-to-back use
	// (2 life × 2 tutors) and gates out cases where the card in hand
	// is functionally unusable. Non-land filter avoids land-side
	// payoffs (handled separately by the painland tax below).
	if !hasLifePayoff && seat.Life >= 4 {
		for _, c := range seat.Hand {
			if c == nil {
				continue
			}
			isLand := false
			for _, t := range c.Types {
				if t == "land" {
					isLand = true
					break
				}
			}
			if isLand {
				continue
			}
			ot := gameengine.OracleTextLower(c)
			if isLifeAsResourcePayoff(ot) {
				hasLifePayoff = true
				break
			}
		}
	}

	// R60r9: convex danger curve. Pre-tune the danger band (life ≤ 10) was
	// linear in ratio, so life=1 (one shock = death) scored only ~2x worse
	// than life=10 — under-weighting proximity-to-zero. Added quadratic
	// (10 - life)² penalty so each life point lost below 10 hurts more
	// than the last. Mid-band (>10) preserved as-is to avoid retuning the
	// established "calm zone" semantics; the convexity is targeted at
	// shock-range death proximity.
	var base float64
	// R60: use effectiveLife (clock-aware: min of raw life and 21-max-cmdr-dmg)
	// for the band check too — pre-r60 a seat at life=20 with 18 commander
	// damage from one source had effectiveLife=3 but went to the mid-band
	// because seat.Life > 10, missing the convex shock penalty entirely.
	if effectiveLife <= 10 {
		base = ratio - 0.5
		danger := float64(10-effectiveLife) / 10.0 // 0..1
		base -= danger * danger * 0.5
		if hasLifePayoff {
			base *= 0.5
		}
	} else {
		base = (ratio - 0.5) * 0.5
		if hasLifePayoff && effectiveLife > 20 {
			base += 0.1
		}
	}

	// R60r9: lifegain on the battlefield is a forward-looking recovery
	// signal that softens the danger penalty. Soul Warden / Soul's
	// Attendant / Heliod / Trelasarra-style passive triggers and lifelink
	// attackers recoup life over time, so the current life total is more
	// durable than the raw number suggests. Each active engine reduces
	// the negative penalty by 10% (capped at 30% — three engines isn't
	// infinite life, and we still respect the underlying clock). Only
	// applied when base is negative; engines don't add positive score,
	// they only dampen the deficit.
	if base < 0 {
		engines := 0
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if p.HasKeyword("lifelink") && p.IsCreature() && gs.PowerOf(p) >= 2 {
				engines++
				continue
			}
			ot := gameengine.OracleTextLower(p.Card)
			if strings.Contains(ot, "gain") && strings.Contains(ot, "life") &&
				(strings.Contains(ot, "whenever") || strings.Contains(ot, "at the beginning")) {
				engines++
			}
		}
		if engines > 3 {
			engines = 3
		}
		if engines > 0 {
			base *= 1.0 - 0.1*float64(engines)
		}
	}

	// R60: painland mana-base tax. City of Brass, Mana Confluence,
	// Ancient Tomb, original painlands (Karplusan Forest etc.), and
	// shocklands chip self-damage on tap (or up-front on ETB) and
	// create an ongoing soft tax on top of opponent pressure. Detect on
	// own battlefield via the canonical "damage to you" suffix (City of
	// Brass / Ancient Tomb / painlands) plus the "pay … life" + "add"
	// activated-cost shape (Mana Confluence). Two penalties stack:
	//
	//   (1) low-life amplifier: when base is already negative, each
	//       painland multiplies the deficit by 5% (capped at 5 = 25%).
	//       The mana base is materially more dangerous in the shock
	//       range — every tap is another step toward zero.
	//
	//   (2) compounding-turn erosion (R60 follow-up to #560): even at
	//       healthy life, a painland in play is a debt that accrues
	//       every turn. By turn 30 with 4 painlands the seat has
	//       structurally taken roughly 12+ damage from its own lands
	//       and its forward life buffer is more fragile than a basics
	//       deck at the same total. Modeled as painlands * turn / 400,
	//       clamped at -0.15. At turn 30 with 4 painlands: 4*30/400 =
	//       0.30 → clamped to -0.15. At turn 5 with 2 painlands:
	//       2*5/400 = 0.025. Fires regardless of base sign so the
	//       hat treats painlands as a deck-quality deficit, not just
	//       a shock-range hazard.
	painlands := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsLand() {
			continue
		}
		ot := gameengine.OracleTextLower(p.Card)
		if strings.Contains(ot, "damage to you") ||
			(strings.Contains(ot, "pay") && strings.Contains(ot, "life") && strings.Contains(ot, "add")) {
			painlands++
		}
	}
	cappedPainlands := painlands
	if cappedPainlands > 5 {
		cappedPainlands = 5
	}
	if base < 0 && cappedPainlands > 0 {
		base *= 1.0 + 0.05*float64(cappedPainlands)
	}
	if cappedPainlands > 0 && gs.Turn > 0 {
		erosion := float64(cappedPainlands) * float64(gs.Turn) / 400.0
		if erosion > 0.15 {
			erosion = 0.15
		}
		base -= erosion
	}

	// R60: inevitability amplifier. Low-life states are dramatically more
	// dangerous when an opponent is also winning the BOARD — they have
	// the resources to convert our low-life position into a kill (alpha
	// strike, drain trigger flood, follow-up burn). Conversely, low life
	// vs a stalled-out table is just a soft clock. Pre-r60 the dimension
	// scored seat.Life=4 identically regardless of whether opponents had
	// 20-power boards or 2-power boards. Fires only in the shock band
	// (effectiveLife <= 10) so it doesn't inflate mid-band life into
	// false danger; scales with the gap between our defensive presence
	// and the strongest opponent's effective offense. Capped at 0.30
	// so a single huge-board opponent doesn't flip the dimension to
	// lethal on top of the existing convex curve.
	if effectiveLife <= 10 && !hasLifePayoff {
		var maxOppPow float64
		for i, s := range gs.Seats {
			if i == seatIdx || s == nil || s.Lost || s.LeftGame {
				continue
			}
			if bp := effectiveOffensivePower(gs, s); bp > maxOppPow {
				maxOppPow = bp
			}
		}
		myPow := effectiveOffensivePower(gs, seat)
		if maxOppPow >= 4 && maxOppPow > myPow*1.5 {
			gap := maxOppPow - myPow
			// Scale by gap and by how deep into the shock band we are.
			depthFactor := float64(10-effectiveLife) / 10.0
			if depthFactor < 0 {
				depthFactor = 0
			}
			inevitability := gap * 0.02 * (0.5 + depthFactor)
			if inevitability > 0.30 {
				inevitability = 0.30
			}
			base -= inevitability
		}
	}

	// R60 round 5: fold an opponent-pressure component into LifeResource so
	// aggressive archetypes (whose LifeResource weight is high) actually
	// value lowering opponents' life totals — not just preserving their
	// own. Pressure is the strongest opponent's life ratio inverted, capped
	// at +0.5 contribution so it can't dominate the own-life term. For
	// 4-player commander with one opp at 12/40 life: ratio=0.30, pressure
	// = (1 - 0.30) * 0.5 = +0.35 (lethal-clock signal). For all opps at
	// starting life: pressure = 0 (no signal).
	pressure := 0.0
	for i, s := range gs.Seats {
		if i == seatIdx || s == nil || s.Lost || s.LeftGame {
			continue
		}
		oppStarting := float64(s.StartingLife)
		if oppStarting <= 0 {
			oppStarting = 40
		}
		oppLife := float64(s.Life)
		if oppLife <= 0 {
			oppLife = 0
		}
		oppRatio := oppLife / oppStarting
		// Take the WEAKEST (lowest life ratio) opponent — that's the one
		// closest to elimination, which is where the win clock is.
		oppPressure := (1.0 - oppRatio) * 0.5
		if oppPressure > pressure {
			pressure = oppPressure
		}
	}
	if pressure < 0 {
		pressure = 0
	}
	if pressure > 0.5 {
		pressure = 0.5
	}
	base += pressure
	if base < -1 {
		base = -1
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

	// r60: piece-availability is now weighted, not boolean. Hand and
	// battlefield pieces score full credit (1.0); graveyard pieces
	// score half credit (0.5) to model recurrable softness — a piece
	// in graveyard is reachable via reanimate / Sun-Titan-class /
	// Karador / Past in Flames / Regrowth, all common in combo decks
	// that loop dies-triggers or play long-game value. Conservatively
	// dampened because not every deck can recur, but the signal isn't
	// zero: a Worldgorger Dragon in the graveyard with no Animate Dead
	// in hand is still strictly more reachable than a Worldgorger
	// Dragon in the library.
	//
	// R60r2: when a graveyard-recursion ENGINE is actually in play
	// (Underworld Breach / Yawgmoth's Will / Past in Flames / Muldrotha
	// / Karador / Sun Titan / Snapcaster), graveyard pieces aren't
	// "theoretically reachable" — they're at-hand reach this turn or
	// next. Upgrade the graveyard weight to 0.9 (just under 1.0 because
	// the recursion still costs mana / a card-slot). The pre-r60r2
	// 0.5 flat regardless of recursion-engine presence was the deeper
	// gap surfaced by the survey — a Worldgorger Dragon in graveyard
	// with Animate Dead ALREADY IN PLAY scored identically to one with
	// no animate effect anywhere.
	graveyardWeight := 0.5
	if seatHasComboGraveyardRecursion(seat) {
		graveyardWeight = 0.9
	}
	available := make(map[string]float64, len(seat.Hand)+len(seat.Battlefield)+len(seat.Graveyard))
	for _, c := range seat.Hand {
		if c != nil {
			available[c.DisplayName()] = 1.0
		}
	}
	for _, p := range seat.Battlefield {
		if p != nil && p.Card != nil {
			available[p.Card.DisplayName()] = 1.0
		}
	}
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		// Don't downgrade a hand/battlefield piece if it ALSO appears
		// in graveyard (rare — copy effects / token returns); keep the
		// strictly stronger zone's weight.
		if available[c.DisplayName()] < graveyardWeight {
			available[c.DisplayName()] = graveyardWeight
		}
	}

	// r60: tutors in hand count as "soft pieces" — each tutor fills
	// one missing slot per plan, mirroring the actual EDH reality that
	// holding Demonic Tutor + 1 piece is functionally similar to
	// holding 2 pieces (cast the tutor, fetch the missing piece, play
	// next turn).
	//
	// r60-cedh-tuning: the tutor credit is capped at (realPiecesFound +
	// 1) rather than the original flat cap of 1. The flat cap missed
	// the canonical cEDH reach pattern — 1 anchoring piece in hand
	// plus 2 tutors functionally closes a 3-piece combo (turn N cast
	// tutor 1 for piece 2, turn N+1 cast tutor 2 for piece 3, deploy
	// piece 1 along the way). The +1 anchor floor preserves the
	// existing guard: a tutor-only hand with no real pieces still
	// claims at most 1 soft-piece, so the score stays well under
	// completion. See TestScoreCombo_TutorCappedAtOnePerPlan +
	// TestScoreCombo_MultiTutorMultipleMissingSlots.
	tutorsInHand := seatTutorsInHand(seat)

	primaryClass := e.Strategy.PrimaryComboClass()
	const offClassDamping = 0.7

	bestRatio := 0.0
	var bestPlan *ComboPlan
	for i := range e.Strategy.ComboPieces {
		cp := &e.Strategy.ComboPieces[i]
		if len(cp.Pieces) == 0 {
			continue
		}
		foundWeight := 0.0
		missing := 0
		realPiecesFound := 0
		for _, piece := range cp.Pieces {
			if w := available[piece]; w > 0 {
				foundWeight += w
				realPiecesFound++
			} else {
				missing++
			}
		}
		// Tutor credit: each tutor in hand fills one missing slot, capped
		// by the number of missing slots AND by (realPiecesFound + 1).
		// The realPiecesFound+1 cap is the no-real-piece guard — a
		// tutor-only hand can never claim more than 1 soft-piece, so a
		// hand of 3 tutors in a 2-piece combo still scores 0.75 (the
		// pre-tuning contract). One real piece anchors the cap up to 2;
		// two real pieces anchor it up to 3, etc. — letting the multi-
		// tutor reach scale with the hand's actual progress.
		if tutorsInHand > 0 && missing > 0 {
			tutorCredit := tutorsInHand
			if tutorCredit > missing {
				tutorCredit = missing
			}
			if cap := realPiecesFound + 1; tutorCredit > cap {
				tutorCredit = cap
			}
			foundWeight += float64(tutorCredit)
		}
		// foundWeight can exceed len(pieces) when graveyard + hand +
		// tutor stack up (shouldn't happen in practice but defensive).
		if foundWeight > float64(len(cp.Pieces)) {
			foundWeight = float64(len(cp.Pieces))
		}
		ratio := foundWeight / float64(len(cp.Pieces))
		// Off-class damping: when the deck has a primary class and this
		// plan is in a different class, scale the contribution. Plans
		// without a Class tag (legacy / unclassified) always score full
		// weight.
		if primaryClass != "" && cp.Class != "" && cp.Class != primaryClass {
			ratio *= offClassDamping
		}
		if ratio > bestRatio {
			bestRatio = ratio
			bestPlan = cp
		}
	}

	// R60r2: cost-reducer bonus. Goblin Electromancer / Helm of Awakening
	// / Birgi / Jhoira / Will of the Jeskai-class permanents bring combo
	// payoffs into cast-range a turn earlier. The base case +0.08 fires
	// when (a) a cost-reducer is on this seat's battlefield AND (b) we've
	// made meaningful progress (bestRatio > 0.4) so the bonus doesn't
	// fire when the combo is still 3+ pieces away.
	//
	// R60r3: CMC-after-reduction castable-this-turn tier. Parse the
	// reduction amount from each reducer's oracle text ("cost {N} less"
	// → N, default 1) and sum across reducers. For the best plan,
	// compute the effective cost to deploy its IN-HAND pieces:
	//   effectiveCost = max(0, sum(face CMC) - reduction × numHandPieces)
	// Compare to seat's land count. If effectiveCost <= availableLands,
	// the combo is CASTABLE THIS TURN — upgrade the bonus from +0.08
	// to +0.15. The pre-r60r3 path treated "Helm of Awakening + 2-mana
	// combo piece in hand + 2 lands" identically to "Helm of Awakening
	// + 6-mana combo piece in hand + 2 lands" — both got +0.08, even
	// though the first is closing this turn and the second isn't.
	//
	// Pieces missing from all zones don't contribute to effectiveCost
	// (we can't know their CMC) — the signal credits castability of
	// what we ALREADY HAVE; the missing-piece coverage stays a
	// tutor-credit / graveyard-credit problem.
	if bestRatio > 0.4 && seatHasComboCostReducer(seat) {
		bonus := 0.08
		if bestPlan != nil {
			reduction := seatCostReductionAmount(seat)
			handPieceCMCs := 0
			handPieceCount := 0
			for _, c := range seat.Hand {
				if c == nil {
					continue
				}
				name := c.DisplayName()
				for _, piece := range bestPlan.Pieces {
					if piece == name {
						handPieceCMCs += gameengine.ManaCostOf(c)
						handPieceCount++
						break
					}
				}
			}
			if handPieceCount > 0 {
				effectiveCost := handPieceCMCs - reduction*handPieceCount
				if effectiveCost < 0 {
					effectiveCost = 0
				}
				if effectiveCost <= seatAvailableLands(seat) {
					bonus = 0.15
				}
			}
		}
		bestRatio += bonus
		if bestRatio > 1.0 {
			bestRatio = 1.0
		}
	}

	// Per-state emergent-synergy bump (wave-2 freya-hat integration audit,
	// 2026-05-30). Companion to applyEmergentSynergyBoost in
	// strategy_loader.go — that pass adds a global Weights.ComboProximity
	// multiplier to ALL game states whenever the deck has Huginn-discovered
	// synergies, regardless of whether the synergy pieces are actually
	// visible right now. This per-state pass targets the lift to states
	// where the synergy pieces are currently in hand / battlefield (or
	// graveyard when a recursion engine is in play, via the same
	// `available` map the combo-piece loop already populated) so the
	// evaluator differentiates "synergy pieces in hand, combo about to
	// land" from "synergy pieces still in deck, combo theoretical".
	//
	// Magnitudes (+0.04 / +0.08, cap +0.20) deliberately smaller than
	// the load-time bump (+0.10 / +0.20, cap +0.50): the load-time bump
	// is a static prior on the whole deck, while this bump is a state-
	// specific reward, so summing both produces a meaningful spread
	// between visible-synergy and theoretical-synergy states without
	// double-counting the prior. See TestEmergentSynergyBump in
	// emergent_synergy_state_r60_test.go.
	bestRatio += emergentSynergyBump(e.Strategy, available)
	if bestRatio > 1.0 {
		bestRatio = 1.0
	}

	if bestRatio >= 1.0 {
		return 2.0
	}
	return bestRatio * 1.5
}

// emergentSynergyBump returns a small additive ratio bump for each
// EmergentSynergy whose `Cards` are all visible in the `available`
// map (the hand/battlefield/recursable-graveyard view scoreCombo
// computes). Tier-2 contributes +0.04, Tier-3 contributes +0.08, sum
// capped at +0.20. Returns 0 when sp / available is nil, when there
// are no synergies, or when no synergy has all its pieces visible.
// Tested via TestEmergentSynergyBump in
// emergent_synergy_state_r60_test.go.
func emergentSynergyBump(sp *StrategyProfile, available map[string]float64) float64 {
	if sp == nil || len(sp.EmergentSynergies) == 0 || available == nil {
		return 0
	}
	bump := 0.0
	for _, es := range sp.EmergentSynergies {
		if len(es.Cards) == 0 {
			continue
		}
		allVisible := true
		for _, name := range es.Cards {
			if available[name] <= 0 {
				allVisible = false
				break
			}
		}
		if !allVisible {
			continue
		}
		switch es.Tier {
		case 2:
			bump += 0.04
		case 3:
			bump += 0.08
		}
	}
	if bump > 0.20 {
		bump = 0.20
	}
	return bump
}

// protectionThreatScalar returns the multiplier to apply to the
// ThreatExposure weight based on the deck's protection density —
// the ratio of RoleCombo / RoleThreat pieces with built-in protection
// (hexproof, shroud, indestructible, ward, "protection from", "can't
// be the target", "can't be countered", "can't be destroyed", phase
// out) to total key pieces. Mapping:
//
//   - ProtectionRatio == -1 (deck has fewer than 3 key pieces, ratio is
//     noise): 1.0 (no adjustment) — midrange decks shouldn't get
//     threat-weight scaling from a non-combo-shape signal.
//   - ProtectionRatio >= 0.50 (half or more key pieces protected):
//     0.85 — combo piece survives the average removal spell, eval
//     shouldn't over-react to incoming threats.
//   - ProtectionRatio <= 0.25 (mostly unprotected combo shell):
//     1.15 — single Path / Pongify resets the line, eval SHOULD
//     react to incoming threats (prioritize protection + disruption).
//   - 0.25 < ratio < 0.50 (middle band): 1.0 — no adjustment, the
//     deck has neither robust protection nor critical fragility.
//
// Magnitudes (±15%) deliberately smaller than stage/position scaling
// (±20-40%) so this signal refines rather than dominates. Returns 1.0
// for nil sp (legacy / no-strategy evaluator).
//
// Added in wave-3 freya-hat integration audit (2026-05-30). Tested
// in protection_threat_scalar_r60_test.go.
func protectionThreatScalar(sp *StrategyProfile) float64 {
	if sp == nil {
		return 1.0
	}
	ratio := sp.ProtectionRatio()
	switch {
	case ratio < 0: // -1 sentinel: too few key pieces, no signal
		return 1.0
	case ratio >= 0.50:
		return 0.85
	case ratio <= 0.25:
		return 1.15
	}
	return 1.0
}

// seatHasComboGraveyardRecursion reports whether the seat has an active
// graveyard-recursion engine that lets combo pieces in graveyard be
// effectively at-hand reach. Broader than seatHasReanimatorEngine which
// only covers creature reanimators (Meren / Muldrotha / Karador
// commanders + battlefield "return target creature card") — combo decks
// frequently care about INSTANT/SORCERY recursion (Underworld Breach,
// Past in Flames, Yawgmoth's Will, Mizzix's Mastery) which the
// creature-only check misses.
//
// Detection covers three shapes:
//   - existing creature-recursion path (delegate to seatHasReanimatorEngine)
//   - battlefield permanents whose oracle text says "may cast" /
//     "may play" + "graveyard" — Underworld Breach, Yawgmoth's Will,
//     Past in Flames, Lurrus, Codie, Mizzix's Mastery (foretell), etc.
//   - battlefield permanents with "snapcaster" / "anchor" / "snapping"
//     patterns — for now, name-anchored to keep the heuristic tight.
func seatHasComboGraveyardRecursion(seat *gameengine.Seat) bool {
	if seat == nil {
		return false
	}
	if seatHasReanimatorEngine(seat) {
		return true
	}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		ot := gameengine.OracleTextLower(p.Card)
		// "may cast a card from your graveyard" / "may play cards
		// from your graveyard" — Underworld Breach, Yawgmoth's Will,
		// Bolas's Citadel-style. Conservative: requires BOTH "graveyard"
		// AND a cast/play verb so we don't false-fire on exile/discard
		// effects.
		if strings.Contains(ot, "graveyard") &&
			(strings.Contains(ot, "may cast") || strings.Contains(ot, "may play") ||
				strings.Contains(ot, "you may cast") || strings.Contains(ot, "you may play") ||
				strings.Contains(ot, "have flashback") || strings.Contains(ot, "gain flashback")) {
			return true
		}
		// Snapcaster-style: "target instant or sorcery card in your
		// graveyard gains flashback" — already covered by "have/gain
		// flashback" above; name-anchor as defense in depth.
		name := strings.ToLower(p.Card.DisplayName())
		if strings.Contains(name, "snapcaster") || strings.Contains(name, "underworld breach") ||
			strings.Contains(name, "yawgmoth's will") || strings.Contains(name, "past in flames") {
			return true
		}
	}
	return false
}

// seatHasComboCostReducer reports whether the seat controls a battlefield
// permanent that reduces spell costs. Targets the canonical EDH cost-
// reducer set: Goblin Electromancer (instant/sorcery -1), Will of the
// Jeskai-style global reducers, Helm of Awakening (-1 all), Birgi (treasure
// on noncreature cast), Jhoira (free draw on historic), Heartless
// Hidetsugu-class.
//
// Oracle-text detection: "cost {N} less to cast" / "spells you cast cost
// {N} less" / "noncreature spells you cast cost {N} less" / "instant and
// sorcery spells you cast cost {N} less". Substring match on "cost" +
// "less" anchored to a cast/spell context is the canonical shape.
// Defense-in-depth name match for a few canonical cards whose oracle
// templating doesn't fit the pattern.
// seatCostReductionAmount returns the total mana reduction this seat's
// battlefield permanents apply to its spells. Sums each reducer's "{N}
// less" template across the battlefield. Conservative: defaults to 1
// per reducer when the oracle text lacks a parseable `{N}` (the
// canonical cards — Goblin Electromancer / Helm of Awakening / Baral /
// Curious Homunculus — all read "cost {1} less"). Caps at 4 to prevent
// theoretical-only stacks (a board with 5+ reducers is a degenerate
// state where mana cost no longer constrains).
//
// Used by scoreCombo to compute effective CMC of combo pieces in hand
// after reducer effects — turns a 3-mana piece into a 2-mana piece when
// Helm of Awakening is in play, which then determines whether the combo
// is castable this turn.
func seatCostReductionAmount(seat *gameengine.Seat) int {
	if seat == nil {
		return 0
	}
	total := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		ot := gameengine.OracleTextLower(p.Card)
		isReducer := strings.Contains(ot, "cost") && strings.Contains(ot, "less") &&
			(strings.Contains(ot, "you cast") || strings.Contains(ot, "to cast") ||
				strings.Contains(ot, "spells"))
		if !isReducer {
			// Name-anchor fallback for canonical reducers whose oracle
			// templating may not survive normalization through tests.
			name := strings.ToLower(p.Card.DisplayName())
			if strings.Contains(name, "electromancer") || strings.Contains(name, "helm of awakening") ||
				strings.Contains(name, "birgi") || strings.Contains(name, "jhoira") {
				total += 1
			}
			continue
		}
		// Try to extract {N} from "cost {N} less". Walk the oracle text
		// for "{n} less" patterns; first numeric brace before "less" wins.
		n := parseReductionN(ot)
		if n <= 0 {
			n = 1
		}
		total += n
	}
	if total > 4 {
		total = 4
	}
	return total
}

// parseReductionN scans `ot` for the canonical "cost {N} less" template
// and returns N. Returns 0 if no parseable `{N}` precedes "less". Only
// matches single-digit reductions (no card prints "cost {10} less").
func parseReductionN(ot string) int {
	lessIdx := strings.Index(ot, " less")
	if lessIdx < 0 {
		return 0
	}
	// Walk backwards from "less" looking for the nearest "{N}" brace.
	prefix := ot[:lessIdx]
	close := strings.LastIndex(prefix, "}")
	if close < 0 {
		return 0
	}
	open := strings.LastIndex(prefix[:close], "{")
	if open < 0 {
		return 0
	}
	sym := prefix[open+1 : close]
	if len(sym) == 1 && sym[0] >= '0' && sym[0] <= '9' {
		return int(sym[0] - '0')
	}
	return 0
}

// seatAvailableLands returns the count of land permanents on this seat's
// battlefield. Used by scoreCombo as a mana-availability proxy when
// deciding whether the cost-reducer bonus should upgrade from +0.08
// (reducer present) to +0.15 (combo castable THIS TURN after reduction).
//
// Proxy intentionally simple: counts all lands regardless of tap state
// (a tapped land will untap next turn, and the signal is forward-looking
// "how castable is the combo from here"). Mana rocks are not counted —
// keeping the proxy land-only avoids double-counting reducers that also
// produce mana (Jhoira) and matches the rough mental model EDH players
// use when budgeting turns.
func seatAvailableLands(seat *gameengine.Seat) int {
	if seat == nil {
		return 0
	}
	n := 0
	for _, p := range seat.Battlefield {
		if p != nil && p.IsLand() {
			n++
		}
	}
	return n
}

func seatHasComboCostReducer(seat *gameengine.Seat) bool {
	if seat == nil {
		return false
	}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		ot := gameengine.OracleTextLower(p.Card)
		// Canonical "X cost {N} less" template — Goblin Electromancer,
		// Helm of Awakening, Will of the Jeskai (delayed), Curious
		// Homunculus, Baral Chief of Compliance, etc.
		if strings.Contains(ot, "cost") && strings.Contains(ot, "less") &&
			(strings.Contains(ot, "you cast") || strings.Contains(ot, "to cast") ||
				strings.Contains(ot, "spells")) {
			return true
		}
		// Name-anchored defense in depth for templating outliers.
		name := strings.ToLower(p.Card.DisplayName())
		if strings.Contains(name, "electromancer") || strings.Contains(name, "helm of awakening") ||
			strings.Contains(name, "birgi") || strings.Contains(name, "jhoira") ||
			strings.Contains(name, "urza, lord high artificer") {
			return true
		}
	}
	return false
}

// seatTutorsInHand counts the number of unconditional or library-search
// tutors currently in seat's hand. Used by scoreCombo's tutor-credit
// path to model "1 mana = 1 deployed piece next turn." Matches three
// shapes:
//
//   - "search your library for a card" (Demonic Tutor, Vampiric Tutor,
//     Imperial Seal, Personal Tutor — broad and narrow)
//   - "search your library for an [instant|sorcery|creature|...] card"
//     (Mystical Tutor, Worldly Tutor, Eladamri's Call, etc.)
//   - "tutor" in card name (defense-in-depth for cards whose oracle
//     text encodes the search via a non-standard phrasing)
//
// Excludes purely-creature-targeting tutors (Survival of the Fittest's
// activated tutor effect lives on the battlefield, not the hand; a
// resolved fetch land doesn't count). Excludes "transmute" / "cycle"
// / "scry" — those are weaker than canonical tutors and warrant a
// separate credit path if added.
func seatTutorsInHand(seat *gameengine.Seat) int {
	if seat == nil {
		return 0
	}
	n := 0
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		ot := gameengine.OracleTextLower(c)
		if ot == "" {
			// Name-based fallback for cards we can't read oracle text for.
			name := strings.ToLower(c.DisplayName())
			if strings.Contains(name, "tutor") {
				n++
			}
			continue
		}
		if strings.Contains(ot, "search your library for") {
			n++
			continue
		}
		// Defense in depth: name match catches cards whose oracle text
		// shape we missed (unusual templating).
		name := strings.ToLower(c.DisplayName())
		if strings.Contains(name, "tutor") {
			n++
		}
	}
	return n
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
	hardestToAnswer := 0.0
	dangerousPermanents := 0.0
	for i, s := range gs.Seats {
		if i == seatIdx || s.Lost || s.LeftGame {
			continue
		}
		bp := effectiveOffensivePower(gs, s)
		if bp > maxOppPow {
			maxOppPow = bp
		}
		if h := hardToAnswerScore(gs, s); h > hardestToAnswer {
			hardestToAnswer = h
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

	// Hard-to-answer multiplier: threats with indestructible / hexproof /
	// ward / protection compound the lethal clock because they survive
	// generic removal — we burn into specific answers (exile, board wipe,
	// -X/-X) and may not have one. Adds up to +20% to the offensive
	// pressure score, capped so a single huge protected creature never
	// flips the dimension to lethal on its own.
	hardToAnswerPenalty := hardestToAnswer * lethalRatio * 0.2

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

	// R60: stack-pending pressure. The dimension previously never read
	// gs.Stack — a Murder targeting our Atraxa, a Counterspell aimed at
	// our wincon cast, or a Wrath of God on the stack all contributed
	// 0.0 to threat. That's the most concretely-missing signal:
	// information that's about to RESOLVE against us next priority pass.
	// Walk every stack item NOT controlled by this seat, score by who/
	// what it targets:
	//
	//   - direct seat target (Lightning Bolt at me, Mind Twist at me):
	//     0.30 per item — the effect lands on me regardless of board.
	//   - permanent target where the permanent is controlled by us:
	//     0.20 per item — generic removal; we might still respond.
	//   - stack-item target where the target stack item is one of OUR
	//     own spells (counterspell aimed at our cast): 0.35 per item —
	//     losing a spell mid-cast is more painful than losing a
	//     permanent because we already paid the mana.
	//
	// Cap total stack pressure at 0.80 so a flood of low-value triggers
	// can't flip the dimension to lethal on its own. Triggered abilities
	// (Kind=="triggered") still count — a Blood Artist drain triggered
	// against us is real threat — but copy items (CR §707.10) are
	// skipped because the original already counted.
	stackPressure := 0.0
	if len(gs.Stack) > 0 {
		for _, item := range gs.Stack {
			if item == nil || item.IsCopy {
				continue
			}
			if item.Controller == seatIdx {
				continue
			}
			for _, t := range item.Targets {
				switch t.Kind {
				case gameengine.TargetKindSeat:
					if t.Seat == seatIdx {
						stackPressure += 0.30
					}
				case gameengine.TargetKindPermanent:
					if t.Permanent != nil && t.Permanent.Controller == seatIdx {
						stackPressure += 0.20
					}
				case gameengine.TargetKindStackItem:
					if t.Stack != nil && t.Stack.Controller == seatIdx {
						stackPressure += 0.35
					}
				}
			}
		}
		if stackPressure > 0.80 {
			stackPressure = 0.80
		}
	}

	// R60: single-removal concentration penalty. wipeMagnet (below) is
	// the OVER-committed side ("a sweeper takes my whole army") — its
	// mirror is the UNDER-committed side: a single keystone creature
	// carrying most of our offensive power is one Murder away from
	// collapsing the position. Walk own creatures, compute max-power
	// share of total power; if the top creature owns ≥50% of our
	// offense AND total offense is non-trivial (>4 to gate out the 1/1
	// token early game), credit the share above 0.5 at 1.0 per unit,
	// capped at 0.50 so a one-creature board (share=1.0) reads exactly
	// the cap. Distinct from hardestToAnswer (THEIR scary creature
	// surviving OUR removal) — this is OUR keystone exposure to THEIR
	// spot removal.
	removalConcentration := 0.0
	{
		var totalPow, maxPow float64
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil || p.PhasedOut || !p.IsCreature() {
				continue
			}
			pow := float64(p.Power())
			if pow <= 0 {
				continue
			}
			totalPow += pow
			if pow > maxPow {
				maxPow = pow
			}
		}
		if totalPow > 4 && maxPow > 0 {
			share := maxPow / totalPow
			if share >= 0.5 {
				removalConcentration = share - 0.5
				if removalConcentration > 0.50 {
					removalConcentration = 0.50
				}
			}
		}
	}

	// R60: untap-step window — defensive-shell exposure. When the
	// active player isn't us, our tapped creatures can't block. If our
	// best-toughness creature is tapped AND noticeably bigger than the
	// best untapped one, the defensive shell is compromised for this
	// priority window. Gated on a non-trivial opponent offensive power
	// (>=4) so the signal doesn't fire over a single 2/2 we can ignore.
	// Reads gs.Active (the seat whose turn it is) — pre-r60 the
	// dimension was completely turn-agnostic, so a tapped Avenger of
	// Zendikar in opponent's combat step scored identically to the
	// same creature untapped on our own turn.
	untapWindow := 0.0
	if gs.Active != seatIdx && maxOppPow >= 4 {
		var bestUntappedTough, bestTappedTough int
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil || p.PhasedOut || !p.IsCreature() {
				continue
			}
			t := p.Toughness()
			if t <= 0 {
				continue
			}
			if p.Tapped {
				if t > bestTappedTough {
					bestTappedTough = t
				}
			} else {
				if t > bestUntappedTough {
					bestUntappedTough = t
				}
			}
		}
		if bestTappedTough > bestUntappedTough+2 {
			diff := bestTappedTough - bestUntappedTough
			untapWindow = float64(diff) * 0.05
			if untapWindow > 0.30 {
				untapWindow = 0.30
			}
		}
	}

	// R60: interaction-mana starvation during opponent priority. With
	// no untapped lands during an opponent's turn, every spell they
	// cast resolves unopposed — we can't pay for a Counterspell, a
	// Swords to Plowshares, a Heroic Intervention, etc. Hand contents
	// are unknown post-determinization, so we score on the WINDOW
	// (untapped-mana availability) regardless of what we hold. Gated
	// on opponent threat (>=4 max opp pow) so a peaceful early opp
	// turn doesn't trigger.
	interactionStarvation := 0.0
	if gs.Active != seatIdx && maxOppPow >= 4 {
		untappedLands := 0
		for _, p := range seat.Battlefield {
			if p == nil || p.PhasedOut || !p.IsLand() {
				continue
			}
			if !p.Tapped {
				untappedLands++
			}
		}
		switch untappedLands {
		case 0:
			interactionStarvation = 0.20
		case 1:
			interactionStarvation = 0.08
		}
	}

	// R60: wipe-magnet penalty. lethalRatio (maxOpp/life) is naturally
	// LOW when we're leading on board — opponents have small boards
	// because we're dominating. But that exact state is when the table
	// most wants to wipe us. Compare our effective offensive power to
	// the strongest opponent's; if we're commanding the board by a
	// wide margin, surface that as a removal-attractiveness penalty.
	// Gated on a non-trivial own-board floor (myBoard > 6) so the
	// early-game tie-state doesn't trigger.
	wipeMagnet := 0.0
	myBoard := effectiveOffensivePower(gs, seat)
	if myBoard > 6 && maxOppPow > 0 {
		ratio := myBoard / maxOppPow
		switch {
		case ratio >= 2.0:
			wipeMagnet = 0.30
		case ratio >= 1.5:
			wipeMagnet = 0.15
		}
	}

	return -lethalRatio*0.8 - dangerousPermanents*0.3 - hoserPenalty - poisonPenalty - millPenalty - cmdrPenalty - hardToAnswerPenalty - stackPressure - wipeMagnet - removalConcentration - untapWindow - interactionStarvation
}

// effectiveOffensivePower returns an evasion-weighted, summoning-sickness-
// discounted offensive-power figure for a seat. Used by scoreThreat in
// place of raw boardPower so that "the opponent has 12 power on board"
// reflects how much of that power can actually pressure us NEXT combat:
//
//   - Flying / shadow / horsemanship / unblockable: 1.5x weight — ground
//     blockers don't apply.
//   - Trample: 1.25x weight — chump-blocking only partially mitigates.
//   - Menace: 1.20x weight — single-blocker chumps don't work.
//   - Summoning sick & no haste: 0.6x weight — the creature can attack
//     NEXT turn, not this one, so there's a turn of warning for the
//     defender to find an answer. Planeswalkers and lands are unaffected
//     (a PW's loyalty ticks immediately on the turn it ETBs).
//
// Multipliers stack multiplicatively (a flying trampler is 1.5 * 1.25
// = 1.875x). Planeswalker loyalty proxy carries over from boardPower.
func effectiveOffensivePower(gs *gameengine.GameState, seat *gameengine.Seat) float64 {
	if seat == nil {
		return 0
	}
	total := 0.0
	for _, p := range seat.Battlefield {
		if p == nil {
			continue
		}
		if p.IsCreature() {
			pw := gs.PowerOf(p)
			if pw <= 0 {
				continue
			}
			mult := 1.0
			if p.HasKeyword("flying") || p.HasKeyword("shadow") ||
				p.HasKeyword("horsemanship") || p.HasKeyword("unblockable") {
				mult *= 1.5
			}
			if p.HasKeyword("trample") {
				mult *= 1.25
			}
			if p.HasKeyword("menace") {
				mult *= 1.20
			}
			if p.SummoningSick && !p.HasKeyword("haste") {
				mult *= 0.6
			}
			total += float64(pw) * mult
			continue
		}
		if p.IsPlaneswalker() {
			loy := 0
			if p.Counters != nil {
				loy = p.Counters["loyalty"]
			}
			if loy < 0 {
				loy = 0
			}
			total += float64(loy + 2)
		}
	}
	return total
}

// hardToAnswerScore returns 0..1 reflecting how much of seat's offensive
// pressure rides on creatures that resist single-target removal.
// Indestructible / hexproof / shroud / ward / protection each count; the
// raw count is normalized so 3+ hard-to-answer attackers saturates at 1.0.
// Threshold gates on power >= 3 so a 1/1 ward-bearing utility creature
// doesn't move the dimension.
func hardToAnswerScore(gs *gameengine.GameState, seat *gameengine.Seat) float64 {
	if seat == nil {
		return 0
	}
	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if gs.PowerOf(p) < 3 {
			continue
		}
		if p.HasKeyword("indestructible") || p.HasKeyword("hexproof") ||
			p.HasKeyword("shroud") || p.HasKeyword("ward") ||
			p.HasKeyword("protection") {
			count++
		}
	}
	score := float64(count) / 3.0
	if score > 1.0 {
		score = 1.0
	}
	return score
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
	if e.Strategy != nil {
		switch {
		case e.Strategy.IsCommanderCentric:
			// R60: Freya-flagged commander-centric decks (Voltron / engine
			// commanders / ≥45% commander synergy / ≥2 commander engine
			// phrases) — floor the on-field bonus at 0.55 so an
			// engine-commander deck with CommanderSynergy < 0.5 (where the
			// raw-synergy gate below would not fire) still registers the
			// commander resolving as load-bearing. Matches the way
			// commanderUrgency already routes around the same flag in
			// the cast-loop retry path (yggdrasil.go:7933).
			synergyBonus = 0.55 + e.Strategy.CommanderSynergy*0.3
		case e.Strategy.CommanderSynergy > 0.5:
			synergyBonus = 0.3 + e.Strategy.CommanderSynergy*0.3
		}
	}
	// commanderPerms: pointers to every commander permanent on this seat's
	// battlefield. Captured during the cmdOnField scan so the voltron
	// signal below can read counters / Modifications / attachment chain
	// without re-walking the battlefield.
	var commanderPerms []*gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, cn := range seat.CommanderNames {
			if p.Card.DisplayName() == cn {
				cmdOnField = true
				score += synergyBonus
				commanderPerms = append(commanderPerms, p)
			}
		}
	}
	// R60: voltron setup signal. Pre-r60 the dimension treated a vanilla
	// commander on field identically to a commander with 3 equipment + 2
	// auras + 4 +1/+1 counters attached — every voltron-shape investment
	// was invisible until the commander damage actually landed. Two
	// additive signals close that gap WITHOUT double-counting the
	// concentrated-damage signal below (which credits damage DEALT, not
	// the setup that produces it):
	//
	//   1. Equipment/Aura attachments — count the equipment + aura
	//      permanents AttachedTo the commander. Per-piece bonus tuned so
	//      a fully-decked commander (Sword of Fire and Ice + Embercleave
	//      + Sword of Light and Shadow, attached) reads ~+0.36 and a
	//      Uril-style aura pile (Ethereal Armor + Daybreak Coronet +
	//      Eldrazi Conscription) reads ~+0.30. Equipment > aura because
	//      auras die with the commander (CR §704.5n) so they're a
	//      riskier investment; the lower per-piece weight reflects that.
	//      Both signals saturate at 4 pieces — past 4 the commander is
	//      already a wincon and additional pieces are noise.
	//
	//   2. Power-above-base — commander.Power() folds +1/+1 counters AND
	//      Modifications AND attached-equipment buffs. We credit the
	//      DELTA against BasePower (so a 2/2 Sram with +6 power from
	//      equipment reads +0.40) at 0.05 per point, capped at 8 points
	//      = +0.40. This captures counter-voltron (Sek'Kuar / Animar /
	//      Hamza) which the attachment-count signal misses entirely, and
	//      catches modal-aura buffs (Eldrazi Conscription's +10/+10) that
	//      the per-piece count would underweight.
	//
	// Voltron archetype gets the standard 1.5x synergy multiplier on the
	// combined voltron block — for an archetype whose entire gameplan is
	// "make the commander unblockable / lethal," the signal needs the
	// same weight ramp the existing CommanderSynergy path already applies
	// to commander-on-field.
	if cmdOnField {
		var equipCount, auraCount int
		for _, p := range seat.Battlefield {
			if p == nil || p.AttachedTo == nil {
				continue
			}
			attached := false
			for _, cp := range commanderPerms {
				if p.AttachedTo == cp {
					attached = true
					break
				}
			}
			if !attached {
				continue
			}
			switch {
			case p.IsEquipment():
				equipCount++
			case p.IsAura():
				auraCount++
			}
		}
		voltron := 0.0
		if equipCount > 4 {
			equipCount = 4
		}
		if auraCount > 4 {
			auraCount = 4
		}
		voltron += float64(equipCount) * 0.12
		voltron += float64(auraCount) * 0.10
		// Power-above-base, summed across all commander permanents (Krenko
		// + Goblin Lackey edge-case partner pairs, etc.).
		powerDelta := 0
		for _, cp := range commanderPerms {
			if cp.Card == nil {
				continue
			}
			d := cp.Power() - cp.Card.BasePower
			if d > 0 {
				powerDelta += d
			}
		}
		if powerDelta > 8 {
			powerDelta = 8
		}
		voltron += float64(powerDelta) * 0.05
		if e.Strategy != nil && e.Strategy.Archetype == ArchetypeVoltron {
			voltron *= 1.5
		}
		score += voltron
	}
	if !cmdOnField && len(seat.CommandZone) > 0 {
		tax := 0
		for _, cn := range seat.CommanderNames {
			tax += seat.CommanderCastCounts[cn]
		}
		taxPenalty := 0.15
		if e.Strategy != nil {
			switch {
			case e.Strategy.IsCommanderCentric:
				// R60: commander-centric decks NEED to land the commander —
				// every additional tax point is a turn of stalled gameplan,
				// so the per-tax penalty escalates further than the
				// synergy>0.5 path. Caps at 0.28 per tax point pre-
				// castability scalar (so 4 tax + uncastable amplifies to
				// 0.28 * 4 * 1.6 = 1.79 vs the pre-r60 0.15 * 4 = 0.60).
				taxPenalty = 0.28
			case e.Strategy.CommanderSynergy > 0.5:
				taxPenalty = 0.20
			}
		}

		// R60r10: castability-weighted tax. The naive linear -tax * 0.15
		// didn't distinguish "5 CMC commander + 4 tax = uncastable on a 6-
		// land board" from "the same commander on a 14-land board where
		// tax barely matters." Compute commander total cost vs land count
		// as a rough mana-ceiling proxy and inflate or soften accordingly.
		cmcCommander := 0
		for _, c := range seat.CommandZone {
			if c == nil {
				continue
			}
			for _, cn := range seat.CommanderNames {
				if c.DisplayName() == cn {
					if cmc := gameengine.ManaCostOf(c); cmc > cmcCommander {
						cmcCommander = cmc
					}
				}
			}
		}
		landCount := 0
		for _, p := range seat.Battlefield {
			if p != nil && p.IsLand() {
				landCount++
			}
		}
		castabilityFactor := 1.0
		totalCost := cmcCommander + tax
		if totalCost > 0 && landCount > 0 {
			ratio := float64(totalCost) / float64(landCount)
			switch {
			case ratio > 1.5: // can't cast for 2+ turns even with a land drop
				castabilityFactor = 1.6
			case ratio > 1.0: // turn-or-two-out
				castabilityFactor = 1.2
			case ratio < 0.5: // abundantly affordable, tax matters less
				castabilityFactor = 0.6
			}
		} else if totalCost > 0 && landCount == 0 {
			// No mana, commander stuck — strongest amplification.
			castabilityFactor = 1.6
		}

		score -= float64(tax) * taxPenalty * castabilityFactor
	}

	// R60r10: concentrated commander damage signal. Voltron WINS by
	// stacking damage on a single opponent past 21 (CR §704.6c is per-
	// opponent, per-source). Pre-tune summed all opp damage equally —
	// "5 on each of 4 opps" scored the same as "20 on one opp = 1 swing
	// from lethal." Reward concentration with a convex curve on the
	// max-per-opponent value; spread damage drops to an ambient-pressure
	// fraction.
	var maxDmg, sumDmg int
	for i, opp := range gs.Seats {
		if i == seatIdx || opp.Lost || opp.LeftGame {
			continue
		}
		if opp.CommanderDamage == nil {
			continue
		}
		if dmgMap, ok := opp.CommanderDamage[seatIdx]; ok {
			perOpp := 0
			for _, dmg := range dmgMap {
				perOpp += dmg
			}
			if perOpp > maxDmg {
				maxDmg = perOpp
			}
			sumDmg += perOpp
		}
	}
	if maxDmg > 0 {
		p := float64(maxDmg) / 21.0
		if p > 1.0 {
			p = 1.0
		}
		// Convex (p²) + small linear floor (0.2*p) so early damage still
		// registers but approaching-lethal dominates. At 21: 1.0 + 0.2 =
		// 1.2 (was 1.0 linear). At 10: 0.227 + 0.095 = 0.32 (was 0.48
		// linear) — early voltron is correctly understated; the swing
		// matters when you're CLOSE.
		score += p*p + p*0.2
	}
	if spread := sumDmg - maxDmg; spread > 0 {
		// Ambient pressure — "I'm on everyone's commander-damage clock"
		// is a real signal but not a wincon by itself.
		score += float64(spread) / 21.0 * 0.15
	}

	// R60: discrete partial-Voltron thresholds on the max-per-opponent
	// commander damage. The convex p² + 0.2p curve above produces a
	// smooth ramp, but human Voltron play treats commander-damage clocks
	// as discrete signposts: 7 (first knock — opp is now aware of the
	// commander as a damage threat), 14 (two-thirds — opp must answer
	// the commander or die in two combats), 18 (one normal swing
	// from §704.6c lethal). Adding a flat bonus per threshold crossed
	// sharpens the dimension's response to "concentrate damage on the
	// closest-to-dead opponent" — pre-r60, the delta from 13 to 14
	// commander damage was the same shape as 4 to 5, so MCTS couldn't
	// distinguish a strategic threshold cross from incremental progress.
	// Bonuses stack additively (18 fires all three) so the saturation
	// from 0.91 (convex curve at 18) → 1.0+ encodes the qualitative
	// "I am one swing from a win" jump.
	switch {
	case maxDmg >= 18:
		score += 0.08 + 0.12 + 0.15 // 7-floor + 14-floor + 18-floor = 0.35
	case maxDmg >= 14:
		score += 0.08 + 0.12 // 7-floor + 14-floor = 0.20
	case maxDmg >= 7:
		score += 0.08
	}

	return score
}

// scoreGraveyard: value of graveyard contents. Models four overlapping
// graveyard-as-resource modes that hat decisions need to weigh:
//
//  1. Recursion-target value (flashback / escape / unearth / disturb /
//     aftermath / embalm / eternalize / encore / jump-start) scaled by
//     the card's CMC — a 4-mana flashback bomb is worth more than a
//     1-mana flashback cantrip, but the bonus saturates so a long
//     graveyard of 8+ recursion cards doesn't blow out the dimension.
//
//  2. Reanimator-target ramp — high-CMC creatures in graveyard are
//     worth a lot more when there's a reanimate engine on board
//     (Animate Dead, Reanimate, Necromancy, Living Death, Karador,
//     Meren, Muldrotha, Chainer, Sevinne's Reclamation, etc.). Without
//     an engine the creatures score the same flat 0.15 as before
//     (recursion is only theoretical); with one, each creature scales
//     by min(0.40, cmc*0.08) on top of the flat baseline so a
//     Griselbrand-in-graveyard + Animate Dead-on-board reads ~0.79,
//     not 0.15.
//
//  3. Delirium gate (CR §702.108 — 4+ distinct card types in your
//     graveyard) — adds a flat threshold bonus once active, scaled
//     1.5x when the seat has a known delirium payoff (commander or
//     battlefield permanent with oracle "delirium" reference). Card
//     types tracked per CR §300.1: creature / instant / sorcery /
//     artifact / enchantment / planeswalker / land / battle / tribal.
//
//  4. Threshold gate (legacy 7+ cards in your graveyard) — same shape
//     as delirium but on graveyard size, scaled 1.5x when the seat
//     has a known threshold payoff. Cheaper to hit than delirium so
//     the base bonus is smaller, but it stacks with delirium for
//     decks that care about both.
//
// Self-mill payoff scaling (Uurg / Sidisi / Muldrotha / Splinterfright)
// and the cross-opponent delta are preserved from the pre-r60
// implementation — they were the only signals before this overhaul
// and are well-tuned at their current weights.
func (e *GameStateEvaluator) scoreGraveyard(gs *gameengine.GameState, seatIdx int) float64 {
	seat := gs.Seats[seatIdx]
	if len(seat.Graveyard) == 0 {
		return 0
	}

	hasReanimator := seatHasReanimatorEngine(seat)

	value := 0.0
	landCount := 0
	creatureCount := 0
	creatureCMCs := make([]int, 0, len(seat.Graveyard))
	recursionValue := 0.0
	typeSet := map[string]bool{}
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
				typeSet["creature"] = true
			case "land":
				landCount++
				typeSet["land"] = true
			case "instant", "sorcery", "artifact", "enchantment", "planeswalker", "battle", "tribal":
				typeSet[t] = true
			}
		}
		if isCreature {
			value += 0.15
			creatureCMCs = append(creatureCMCs, gameengine.ManaCostOf(c))
		}
		// Recursion-keyword bonus — scaled by CMC so a 4-mana flashback
		// bomb is worth ~0.30 vs a 1-mana cantrip at ~0.15. Floor at
		// 0.10 (so even free spells like Jump-Start on a 0-cost card
		// register), ceiling at 0.35 (a 6-mana flashback Cataclysm is
		// not 6x as valuable as a 1-mana cantrip).
		if hasGraveyardCastKeyword(ot) {
			cmc := gameengine.ManaCostOf(c)
			rv := 0.10 + float64(cmc)*0.05
			if rv > 0.35 {
				rv = 0.35
			}
			recursionValue += rv
		}
	}
	value += recursionValue

	// Reanimator-target ramp. Only fires when a reanimate engine is
	// already on board — otherwise the high-CMC creatures are
	// theoretical reanimation targets, valued at the base 0.15 above.
	if hasReanimator {
		for _, cmc := range creatureCMCs {
			ramp := float64(cmc) * 0.08
			if ramp > 0.40 {
				ramp = 0.40
			}
			value += ramp
		}
	}

	// Delirium (CR §702.108) — 4+ distinct card types in your graveyard.
	if len(typeSet) >= 4 {
		base := 0.40
		if seatHasDeliriumPayoff(seat) {
			base *= 1.5
		}
		value += base
	}

	// Threshold (legacy) — 7+ cards in your graveyard.
	if len(seat.Graveyard) >= 7 {
		base := 0.30
		if seatHasThresholdPayoff(seat) {
			base *= 1.5
		}
		value += base
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

	// R60: spell-recursion enabler bonus. seatHasReanimatorEngine above
	// catches the creature-reanimation surface (Meren / Muldrotha /
	// Animate Dead family) but ignores the parallel spell-recursion
	// surface — Snapcaster Mage / Past in Flames / Underworld Breach /
	// Mizzix's Mastery / Yawgmoth's Will / Lurrus / Sun Titan — which
	// makes non-keyword instants/sorceries in graveyard into recurring
	// resources. Add a per-card bonus for graveyard cards that DIDN'T
	// already score via the recursion-keyword path so we don't double-
	// count printed-flashback cards. Capped so a 30-card graveyard with
	// one enabler doesn't dominate the dimension.
	if seatHasSpellRecursionEnabler(seat) {
		instSorcCount := 0
		for _, c := range seat.Graveyard {
			if c == nil {
				continue
			}
			ot := gameengine.OracleTextLower(c)
			if hasGraveyardCastKeyword(ot) {
				continue
			}
			for _, t := range c.Types {
				if t == "instant" || t == "sorcery" {
					instSorcCount++
					break
				}
			}
		}
		bonus := float64(instSorcCount) * 0.10
		if bonus > 0.50 {
			bonus = 0.50
		}
		value += bonus
	}

	// R60: land-recursion enabler bonus. Crucible of Worlds / Ramunap
	// Excavator / Lord Windgrace / The Gitrog Monster make lands in the
	// graveyard into playable tempo. Pre-r60, lands only scored under
	// selfMillPayoff (+0.05 each), but a Tron or Lands deck running
	// Crucible isn't necessarily self-mill — fetchlands fuel the
	// graveyard organically. Gated on !selfMillPayoff so we don't
	// double-credit the Gitrog overlap case.
	if !selfMillPayoff && seatHasLandRecursionEnabler(seat) {
		landBonus := float64(landCount) * 0.08
		if landBonus > 0.50 {
			landBonus = 0.50
		}
		value += landBonus
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

// hasGraveyardCastKeyword reports whether the lower-cased oracle text
// references a keyword that lets the card be cast / activated from the
// graveyard. Covers the pre-r60 quartet (flashback / escape / unearth
// / retrace) plus the keywords added by post-AKH sets: disturb (MID
// — reversible token graveyard cast), aftermath (AKH — second-half
// graveyard cast), embalm / eternalize (AMN — token-creature
// graveyard activations), encore (CMR — triple token attack), and
// jump-start (GRN — discard-a-card-and-cast).
// isLifeAsResourcePayoff reports whether the lower-cased oracle text
// describes a card that USES the controller's life as a strategic
// resource — i.e. the deck WANTS to be at low life because the card
// converts life to value or removes the mana-cost gate. Drives
// scoreLife's hasLifePayoff dampener: when one of these is present,
// the shock-range life penalty is halved because the position
// represents intentional life-spending, not impending death.
//
// Recognized shapes:
//   - "pay" + "life" + value verb (draw / cast / search / exile / put /
//     destroy) — Necropotence (exile cards via pay-life), Greed (pay 2:
//     draw), Vampiric/Imperial tutors (pay 2: search), Yawgmoth (pay 2:
//     put -1/-1 counter), Vilis (draw triggers off life loss with pay
//     activations). Pre-r60 the gate was draw/cast/search only — missed
//     Yawgmoth and the broader exile/destroy/put utility shapes.
//   - "cost" + "life" + "more" — K'rrik shape ("spells you cast cost
//     2 life more to cast"). Mana → life conversion makes raw life
//     the real mana pool, so low life = no spells.
//   - "rather than pay" + "mana cost" — Bolas's Citadel ("pay life
//     equal to its mana value rather than pay its mana cost"). Same
//     mana → life conversion shape with different oracle wording.
//   - "ad nauseam" / "lose life equal to" — Ad Nauseam / Dark
//     Confidant family. These are passive life-drain engines that
//     ALSO count as life-as-resource cards: the deck CHOSE to run
//     them knowing the trade. The dimension dampens the penalty
//     because the life loss is bought for cards.
//
// Deliberately EXCLUDES "you may pay X life: that player/target loses"
// style cards (drain-on-opponent that requires paying life) — those
// aren't deck-side life-as-resource engines; they're attack lines.
func isLifeAsResourcePayoff(oracleLower string) bool {
	if !strings.Contains(oracleLower, "life") {
		return false
	}
	// K'rrik shape — mana cost converted to life cost.
	if strings.Contains(oracleLower, "cost") && strings.Contains(oracleLower, "life") &&
		strings.Contains(oracleLower, "more") {
		return true
	}
	// Bolas's Citadel shape — pay life equal to MV instead of mana.
	if strings.Contains(oracleLower, "rather than pay") &&
		(strings.Contains(oracleLower, "mana cost") || strings.Contains(oracleLower, "mana value")) {
		return true
	}
	// Ad Nauseam family — explicit "life equal to" + reveal/draw access.
	if strings.Contains(oracleLower, "lose life equal to") &&
		(strings.Contains(oracleLower, "reveal") || strings.Contains(oracleLower, "into your hand")) {
		return true
	}
	// Broad pay-life-for-value: pay + life + a value verb.
	if strings.Contains(oracleLower, "pay") &&
		(strings.Contains(oracleLower, "draw") || strings.Contains(oracleLower, "cast") ||
			strings.Contains(oracleLower, "search") || strings.Contains(oracleLower, "exile") ||
			strings.Contains(oracleLower, "destroy") || strings.Contains(oracleLower, "put")) {
		// Defensive exclude: "pay X life: target player/opponent loses Y life"
		// (drain attack lines, not self-resource engines).
		if strings.Contains(oracleLower, "target player") || strings.Contains(oracleLower, "each opponent") {
			// Only exclude if there's no self-value verb adjacent — drain
			// effects don't draw/search for the controller.
			if !strings.Contains(oracleLower, "draw a card") && !strings.Contains(oracleLower, "search your library") {
				return false
			}
		}
		return true
	}
	return false
}

func hasGraveyardCastKeyword(oracleLower string) bool {
	return strings.Contains(oracleLower, "flashback") ||
		strings.Contains(oracleLower, "escape") ||
		strings.Contains(oracleLower, "unearth") ||
		strings.Contains(oracleLower, "retrace") ||
		strings.Contains(oracleLower, "disturb") ||
		strings.Contains(oracleLower, "aftermath") ||
		strings.Contains(oracleLower, "embalm") ||
		strings.Contains(oracleLower, "eternalize") ||
		strings.Contains(oracleLower, "encore") ||
		strings.Contains(oracleLower, "jump-start")
}

// seatHasReanimatorEngine reports whether the seat has an active
// reanimator engine on its battlefield — i.e. some permanent whose
// oracle text suggests it can return creatures from any graveyard to
// the battlefield. Matches the pattern used by scoreOpponentGraveyard
// for the threat side (where the same check downgrades the position),
// kept colocated here so the two evaluator paths stay consistent.
//
// Recognized engines: explicit reanimator commanders (Meren, Muldrotha,
// Karador, Chainer, Araumi, Nethroi, Sefris, Reya) plus generic
// oracle-text engines (Animate Dead / Reanimate / Necromancy / Living
// Death class — "return ... creature card ... from ... graveyard ...
// to the battlefield"). Spell-form reanimators in hand don't count;
// the ramp only fires when an engine is actually deployed.
func seatHasReanimatorEngine(seat *gameengine.Seat) bool {
	if seat == nil {
		return false
	}
	for _, cn := range seat.CommanderNames {
		cnLower := strings.ToLower(cn)
		if strings.Contains(cnLower, "meren") ||
			strings.Contains(cnLower, "muldrotha") ||
			strings.Contains(cnLower, "karador") ||
			strings.Contains(cnLower, "chainer") ||
			strings.Contains(cnLower, "araumi") ||
			strings.Contains(cnLower, "nethroi") ||
			strings.Contains(cnLower, "sefris") ||
			strings.Contains(cnLower, "reya") {
			return true
		}
	}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		ot := gameengine.OracleTextLower(p.Card)
		if strings.Contains(ot, "graveyard") &&
			strings.Contains(ot, "battlefield") &&
			(strings.Contains(ot, "return") || strings.Contains(ot, "put")) &&
			(strings.Contains(ot, "creature card") || strings.Contains(ot, "creature from") ||
				strings.Contains(ot, "target creature")) {
			return true
		}
	}
	return false
}

// seatHasSpellRecursionEnabler reports whether the seat has a
// battlefield permanent that recurs noncreature cards (typically
// instants/sorceries) from its graveyard. Distinct from
// seatHasReanimatorEngine (creature-side) — this is the parallel
// instant/sorcery surface that makes non-keyword spells in graveyard
// real recurring resources. Matches:
//   - Snapcaster Mage shape: "instant or sorcery card" + "your graveyard"
//   - Past in Flames / Underworld Breach / Yawgmoth's Will / Mizzix's
//     Mastery shape: "(cast|play) ... from your graveyard" + a non-land
//     scoping word
//   - Lurrus / Sun Titan shape: "return ... permanent ... from your
//     graveyard ... to the battlefield"
func seatHasSpellRecursionEnabler(seat *gameengine.Seat) bool {
	if seat == nil {
		return false
	}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		ot := gameengine.OracleTextLower(p.Card)
		if !strings.Contains(ot, "graveyard") {
			continue
		}
		if (strings.Contains(ot, "instant or sorcery card") ||
			strings.Contains(ot, "instant and sorcery card")) &&
			(strings.Contains(ot, "from your graveyard") || strings.Contains(ot, "in your graveyard")) {
			return true
		}
		if (strings.Contains(ot, "cast") || strings.Contains(ot, "play")) &&
			strings.Contains(ot, "from your graveyard") &&
			(strings.Contains(ot, "instant") || strings.Contains(ot, "sorcery") ||
				strings.Contains(ot, "nonland") || strings.Contains(ot, "noncreature") ||
				strings.Contains(ot, "permanent")) {
			return true
		}
		if strings.Contains(ot, "return") &&
			(strings.Contains(ot, "permanent card") || strings.Contains(ot, "permanent from")) &&
			strings.Contains(ot, "from your graveyard") &&
			strings.Contains(ot, "battlefield") {
			return true
		}
	}
	return false
}

// seatHasLandRecursionEnabler reports whether the seat has a battlefield
// permanent that lets it play lands from its graveyard. Matches
// Crucible of Worlds, Ramunap Excavator, Lord Windgrace, The Gitrog
// Monster's lands-matter clause, etc. Drives the r60 land-from-
// graveyard bonus so a Tron / Lands shell running Crucible without a
// self-mill engine still values its graveyard lands.
func seatHasLandRecursionEnabler(seat *gameengine.Seat) bool {
	if seat == nil {
		return false
	}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		ot := gameengine.OracleTextLower(p.Card)
		if strings.Contains(ot, "play lands from your graveyard") {
			return true
		}
		if strings.Contains(ot, "land cards from your graveyard") &&
			(strings.Contains(ot, "play") || strings.Contains(ot, "to the battlefield")) {
			return true
		}
	}
	return false
}

// seatHasDeliriumPayoff reports whether the seat owns a card (commander
// or battlefield permanent) whose oracle text references the delirium
// mechanic. Drives the 1.5x scalar on the delirium-active bonus —
// hitting 4-types-in-graveyard is roughly neutral on a generic value
// deck, but on a Tovolar's Huntmaster / Mishra Eternal Apprentice /
// Liliana the Last Hope / Emrakul the Promised End shell it's a
// load-bearing combat-or-cost-reduction enabler.
func seatHasDeliriumPayoff(seat *gameengine.Seat) bool {
	if seat == nil {
		return false
	}
	for _, cn := range seat.CommanderNames {
		if strings.Contains(strings.ToLower(cn), "delirium") {
			return true
		}
	}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if strings.Contains(gameengine.OracleTextLower(p.Card), "delirium") {
			return true
		}
	}
	return false
}

// seatHasThresholdPayoff is the threshold-keyword sibling of
// seatHasDeliriumPayoff. Sees Werebear / Krosan Reclamation /
// Cabal Coffers-style "threshold —" payoffs that activate at 7+
// cards in graveyard.
func seatHasThresholdPayoff(seat *gameengine.Seat) bool {
	if seat == nil {
		return false
	}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if strings.Contains(gameengine.OracleTextLower(p.Card), "threshold") {
			return true
		}
	}
	return false
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

	// Deck-shape constants apply first so stage/position scaling rides
	// on top of the protection-adjusted baseline. ProtectionThreatScalar
	// reads StrategyProfile.ProtectedKeyPieces / UnprotectedKeyPieces
	// (wave-3 freya-hat integration audit, 2026-05-30) and adjusts the
	// ThreatExposure weight: high-protection decks (≥50% of key pieces
	// shrouded / wardable / indestructible) get the weight downscaled
	// 0.85x — the combo piece survives the average removal spell, so
	// the eval shouldn't panic at incoming threats; low-protection
	// decks (≤25%) get the weight upscaled 1.15x — a single Path or
	// Pongify resets the line, so the eval SHOULD panic and prioritize
	// protection / disruption. No-op when the deck has fewer than 3
	// key pieces (non-combo midrange — protection ratio is noise).
	w.ThreatExposure *= protectionThreatScalar(e.Strategy)

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
