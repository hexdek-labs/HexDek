package main

import (
	"fmt"
	"strings"
)

// gameplan_script — turn-by-turn ideal sequence + branching decisions
// + graceful degradation paths for the deck's primary archetype.
//
// Companion to the one-sentence dp.GameplanSummary surfaced today.
// The summary answers "what does this deck do?" — this script answers
// "how does it play, turn by turn, and what does it do when things
// go wrong?" Useful for new pilots picking up a deck and for AI
// (Yggdrasilhat) heuristic priors.
//
// Per-archetype templates live in archetypeGameplanTemplates. The
// templates are kept short (4-6 turn entries, 2-3 decision points,
// 2-3 degradation paths) because the goal is a scannable cheat sheet,
// not a play guide. Deck-context substitutions are limited to a few
// holes ({commander}, {finisher}) so the template stays template-y
// without falling into per-deck hand-tuning.

// GameplanScript is the structured, scannable play script. Surfaced
// in both the text and JSON reports as a sibling to GameplanSummary.
type GameplanScript struct {
	Archetype        string           `json:"archetype"`
	Commander        string           `json:"commander,omitempty"`
	TurnByTurn       []TurnPlan       `json:"turn_by_turn"`
	DecisionPoints   []DecisionPoint  `json:"decision_points,omitempty"`
	DegradationPaths []DegradationPath `json:"degradation_paths,omitempty"`
}

// TurnPlan is one row of the ideal-sequence table.
type TurnPlan struct {
	Turn   int    `json:"turn"`
	Action string `json:"action"`
	Note   string `json:"note,omitempty"`
}

// DecisionPoint is a branching choice the pilot makes during the
// gameplan. Trigger states the observed condition; Action states the
// recommended response.
type DecisionPoint struct {
	Trigger string `json:"trigger"`
	Action  string `json:"action"`
}

// DegradationPath is a graceful-degradation fallback: when the ideal
// sequence breaks (commander removed twice, combo piece exiled, mana
// flooded), how does the deck recover and re-anchor its plan?
type DegradationPath struct {
	Setback string `json:"setback"`
	Recover string `json:"recover"`
}

// archetypeGameplanTemplate is the per-archetype script skeleton. The
// {commander} and {finisher} holes are filled by buildGameplanScript
// from DeckProfile fields; unfilled holes are left as the literal
// template token (defensive — better than dropping the line entirely).
type archetypeGameplanTemplate struct {
	turnByTurn       []TurnPlan
	decisionPoints   []DecisionPoint
	degradationPaths []DegradationPath
}

var archetypeGameplanTemplates = map[string]archetypeGameplanTemplate{
	"Combo": {
		turnByTurn: []TurnPlan{
			{Turn: 1, Action: "Play ramp", Note: "Sol Ring / signet / 1-CMC dork — accelerate to the commander turn"},
			{Turn: 2, Action: "Play a second ramp or 2-CMC tutor", Note: "you want 5+ mana available by turn 3"},
			{Turn: 3, Action: "Cast {commander}", Note: "the engine that lets the combo lines come together"},
			{Turn: 4, Action: "Hold protection or tutor for the missing piece", Note: "do NOT cast the combo without interaction backup"},
			{Turn: 5, Action: "Assemble the win", Note: "{finisher} — cast through countermagic with held mana"},
		},
		decisionPoints: []DecisionPoint{
			{Trigger: "Opponent has visible interaction (counterspell up, removal in hand)",
				Action: "Bait it with a non-essential threat or a second tutor before going for the combo"},
			{Trigger: "Tutor in opening hand",
				Action: "Find the missing combo piece, not a tutor for a tutor — a turn of tempo matters more than redundancy"},
			{Trigger: "Finisher drawn naturally on turn 4+",
				Action: "Cast a sacrificial threat first to absorb removal, THEN combo on the following turn"},
		},
		degradationPaths: []DegradationPath{
			{Setback: "Commander countered twice",
				Recover: "Pay commander tax for a third cast only with active protection; otherwise pivot to a value engine and grind"},
			{Setback: "Combo piece exiled (not in graveyard — unrecoverable)",
				Recover: "Tutor for the redundant piece or pivot to a backup line; do NOT keep tutoring exiled cards"},
			{Setback: "Stalled mana on turn 5 without the combo assembled",
				Recover: "Switch to value mode — cast incremental threats, fix mana, wait for a window"},
		},
	},
	"Control": {
		turnByTurn: []TurnPlan{
			{Turn: 1, Action: "Play a land, hold up nothing — pass priority", Note: "save mana for turn-2 counters"},
			{Turn: 2, Action: "Cantrip or cheap ramp", Note: "smooth your draws while still holding 1-2 mana for interaction"},
			{Turn: 3, Action: "Cantrip or 2-CMC counterspell into a removal slot", Note: "spend the turn answering threats, not deploying"},
			{Turn: 4, Action: "Sweeper if the board is wide, otherwise {commander}", Note: "reset the table if any opp has multiple creatures"},
			{Turn: 5, Action: "Lock or grindy threat", Note: "establish card advantage faucet and win in the long game"},
		},
		decisionPoints: []DecisionPoint{
			{Trigger: "Two opponents threaten lethal on the same turn",
				Action: "Sweeper first, never one-for-one — a single removal spell leaves one threat live"},
			{Trigger: "Combo player at the table is assembling",
				Action: "Save counters for combo pieces, NOT for ramp or card draw — let value chains play out"},
			{Trigger: "Card advantage engine resolves",
				Action: "Pivot from reactive to proactive — start deploying threats while opp can't keep up"},
		},
		degradationPaths: []DegradationPath{
			{Setback: "Hand empty after multiple counterspell wars",
				Recover: "Tutor for a draw engine or pass to find a topdeck — don't try to counter the next threat dry"},
			{Setback: "Mana-flooded with sweepers vs. an empty board",
				Recover: "Cycle/loot away the sweepers, deploy a clock — control needs to KILL opponents eventually"},
		},
	},
	"Stax": {
		turnByTurn: []TurnPlan{
			{Turn: 1, Action: "Play ramp or a 1-CMC hatebear", Note: "Mana Vault / Grand Abolisher / Thalia — pressure early"},
			{Turn: 2, Action: "Drop a tax piece", Note: "Sphere of Resistance / Thorn of Amethyst — slow everyone before you commit"},
			{Turn: 3, Action: "Cast {commander} OR a lock piece", Note: "depending on whether the commander accelerates lock or carries"},
			{Turn: 4, Action: "Stack a second tax", Note: "ramp into Tangle Wire / Smokestack / Winter Orb — the table can't keep up"},
			{Turn: 5, Action: "Close under the lock", Note: "your mana base is built for taxes; opponents' isn't"},
		},
		decisionPoints: []DecisionPoint{
			{Trigger: "Opponents have ramp / treasure that breaks tax effects",
				Action: "Sequence Stax pieces to hit the fast-mana sources first (Cursed Totem, Null Rod, Linvala)"},
			{Trigger: "Tax pieces stacking on you",
				Action: "Untap effects (Awakening / Seedborn Muse) flip the asymmetry — prioritize one before adding more taxes"},
		},
		degradationPaths: []DegradationPath{
			{Setback: "Lock pieces removed in waves",
				Recover: "Recur via Sun Titan / Aether Vial flicker; tutor for a redundant lock piece you don't run as 1-of"},
			{Setback: "Opponents pivoted to combat-first plan",
				Recover: "Crawlspace / Ensnaring Bridge / Maze of Ith — Stax also locks the red zone, not just the stack"},
		},
	},
	"Aggro": {
		turnByTurn: []TurnPlan{
			{Turn: 1, Action: "1-drop creature or 1-CMC ramp", Note: "Llanowar Elves / Birds of Paradise / Goblin Guide — board first"},
			{Turn: 2, Action: "Two-creature turn", Note: "deploy at least one 2-CMC threat plus a 1-drop if available"},
			{Turn: 3, Action: "Cast {commander}", Note: "the lord/anthem/payoff that turns the board into lethal"},
			{Turn: 4, Action: "Anthem or extra-combat enabler", Note: "Craterhoof / Sword of Feast and Famine / Aggravated Assault"},
			{Turn: 5, Action: "Swing for game", Note: "ideally 21 commander damage or 40+ on one player"},
		},
		decisionPoints: []DecisionPoint{
			{Trigger: "Opponent has a sweeper threat on table",
				Action: "Pre-bait the sweeper with an off-curve threat OR hold reach / one-shot kill in hand"},
			{Trigger: "Pillowfort player at the table",
				Action: "Switch threat focus — the pillowfort player goes LAST; the easiest target goes first"},
		},
		degradationPaths: []DegradationPath{
			{Setback: "Boardwiped on turn 4",
				Recover: "Replay with reach (burn / damage doublers / extra combats) — don't rebuild a wide board into a second wipe"},
			{Setback: "Commander stuck in command zone (tax > 6)",
				Recover: "Pivot to combat through the 99 — the deck has anthems, not a dependency on the commander"},
		},
	},
	"Midrange": {
		turnByTurn: []TurnPlan{
			{Turn: 1, Action: "Play ramp", Note: "Sol Ring / Arcane Signet / mana dork"},
			{Turn: 2, Action: "Second ramp or utility creature", Note: "smooth into the 4-mana turn"},
			{Turn: 3, Action: "Cast {commander}", Note: "the value engine that powers the mid-game"},
			{Turn: 4, Action: "Removal or threat", Note: "answer the most pressing opp threat, otherwise deploy your own"},
			{Turn: 5, Action: "Pressure opponents", Note: "trade efficiently and grind toward a card advantage win"},
		},
		decisionPoints: []DecisionPoint{
			{Trigger: "Multiple threats on table",
				Action: "Spot removal targets the player WITHOUT a backup commander — single-target removal is precious"},
			{Trigger: "Two cards in hand and a stalled mana base",
				Action: "Play the utility/card-advantage piece even off-curve — the grind plan needs fuel"},
		},
		degradationPaths: []DegradationPath{
			{Setback: "Commander removed and tax is rising",
				Recover: "Lean into the 99 — midrange has redundant engines (Solemn Simulacrum / Mulldrifter / Eternal Witness)"},
			{Setback: "Mana flooded",
				Recover: "Cycle/loot effects exist for this — don't try to win through dead-card draws"},
		},
	},
	"Voltron": {
		turnByTurn: []TurnPlan{
			{Turn: 1, Action: "Play ramp", Note: "1-CMC dork or signet — accelerate to commander"},
			{Turn: 2, Action: "Second ramp or protection (Sigarda's Aid, Lightning Greaves)", Note: "you need hexproof/shroud on the commander turn"},
			{Turn: 3, Action: "Cast {commander} with protection up", Note: "uninterrupted attack is the entire plan"},
			{Turn: 4, Action: "Attach first equipment / aura", Note: "ideally an evasion piece (Whispersilk Cloak / Aqueous Form) AND a stat boost"},
			{Turn: 5, Action: "Swing for 21 commander damage", Note: "consolidate damage on ONE player — three players take seven each is a loss"},
		},
		decisionPoints: []DecisionPoint{
			{Trigger: "Opponent has unconditional removal",
				Action: "Lead with protection BEFORE the commander — never lose tempo to a Path to Exile in response"},
			{Trigger: "Multiple players at low life",
				Action: "Focus the player at 21+ commander damage from you — finishing them removes a stack point"},
		},
		degradationPaths: []DegradationPath{
			{Setback: "Commander removed twice in two turns",
				Recover: "Hold equipment in hand, replay only when interaction is exhausted or protection is live"},
			{Setback: "Equipment / aura wiped",
				Recover: "Tutor for a redundant piece (Steelshaper's Gift / Open the Armory); pivot to the secondary win line if any"},
		},
	},
	"Equipment-Voltron": {
		turnByTurn: []TurnPlan{
			{Turn: 1, Action: "Play ramp + a 1-CMC equipment", Note: "Bonesplitter / Skullclamp — board presence cheap"},
			{Turn: 2, Action: "Sigarda's Aid / Puresteel Paladin / Sram", Note: "the free-equip enabler is the value engine"},
			{Turn: 3, Action: "Cast {commander} or Stoneforge Mystic", Note: "tutor for the build-around equipment piece"},
			{Turn: 4, Action: "Attach equipment with no equip-cost (free equip)", Note: "Hammer of Nazahn / Sigarda's Aid trigger — tempo wins"},
			{Turn: 5, Action: "Swing for commander damage", Note: "stack equipment on the commander once it's flying / unblockable"},
		},
		decisionPoints: []DecisionPoint{
			{Trigger: "Commander not in hand on turn 3",
				Action: "Cast Stoneforge / Puresteel and tutor — the engine is the equipment package, not just the commander"},
		},
		degradationPaths: []DegradationPath{
			{Setback: "Equipment package wiped (Vandalblast / Bane of Progress)",
				Recover: "Tiana / Sun Titan / Auriok Salvagers recur — Equipment-Voltron is more recursive than pure Voltron"},
		},
	},
	"Reanimator": {
		turnByTurn: []TurnPlan{
			{Turn: 1, Action: "Play ramp or 1-CMC self-mill (Stitcher's Supplier, Faithless Looting)", Note: "fill the graveyard immediately"},
			{Turn: 2, Action: "Self-mill enabler (Buried Alive / Entomb on the stack)", Note: "put the reanimate target in the bin"},
			{Turn: 3, Action: "Reanimate spell (Reanimate, Animate Dead, Necromancy)", Note: "ideally before opponents can hit your graveyard"},
			{Turn: 4, Action: "{commander} OR a second reanimate to chain another threat", Note: "depending on whether the commander IS the engine"},
			{Turn: 5, Action: "Attack with the reanimated wincon", Note: "Razaketh / Worldspine Wurm / Archon of Cruelty close from a few turns out"},
		},
		decisionPoints: []DecisionPoint{
			{Trigger: "Graveyard hate on table (Rest in Peace, Leyline of the Void)",
				Action: "Answer it FIRST — every other piece in the deck is dead until the hate is gone"},
			{Trigger: "Reanimate target stuck in hand",
				Action: "Discard outlet or self-mill — don't hardcast a 10-mana threat when the deck is built to cheat it"},
		},
		degradationPaths: []DegradationPath{
			{Setback: "Reanimate target exiled in response",
				Recover: "Find the redundant target via tutor / Buried Alive (2-of in deck for this reason)"},
			{Setback: "All reanimate spells countered",
				Recover: "Pivot to cast-from-graveyard (Wonder, Anger, flashback creatures) — graveyard-as-hand"},
		},
	},
	"Selfmill": {
		turnByTurn: []TurnPlan{
			{Turn: 1, Action: "Self-mill enabler (Stitcher's Supplier, Mesmeric Orb)", Note: "graveyard size is the resource"},
			{Turn: 2, Action: "Second mill source or {commander} if 2-CMC", Note: "stack two mill sources for scaling payoffs"},
			{Turn: 3, Action: "Mill payoff or scaling threat (Tarmogoyf, Splinterfright)", Note: "stats grow with graveyard count"},
			{Turn: 4, Action: "Big mill turn (Hedron Crab + landfall, Mind Funeral)", Note: "tip into Jarad / Mortivore / Lhurgoyf lethal range"},
			{Turn: 5, Action: "Sac for lethal (Jarad's commander ability) or attack with Lhurgoyf", Note: "graveyard-as-power closes the game"},
		},
		decisionPoints: []DecisionPoint{
			{Trigger: "Opponent exiles your graveyard mid-turn",
				Action: "Pre-empt with a sac payoff (Jarad damage trigger) — exile-after-resolve loses, exile-in-response wins"},
		},
		degradationPaths: []DegradationPath{
			{Setback: "Graveyard repeatedly hated out",
				Recover: "Self-mill into a discard-outlet plan (Survival of the Fittest) — graveyard becomes hand instead"},
		},
	},
	"Tokens": {
		turnByTurn: []TurnPlan{
			{Turn: 1, Action: "Play ramp", Note: "Sol Ring / dork — tokens decks need turn-3 commanders"},
			{Turn: 2, Action: "First token producer (Bitterblossom, Krenko, Mob Boss precursor)", Note: "start the engine"},
			{Turn: 3, Action: "Cast {commander}", Note: "the doubler or the anthem — multiplies the existing board"},
			{Turn: 4, Action: "Anthem or extra-tokens effect (Intangible Virtue / Doubling Season)", Note: "every existing token gets value"},
			{Turn: 5, Action: "Mass go-wide attack OR overrun finisher (Craterhoof / Triumph of the Hordes / Coat of Arms)", Note: "tokens close from any board state"},
		},
		decisionPoints: []DecisionPoint{
			{Trigger: "Boardwipe-favorable opponent at table",
				Action: "Hold one anthem in hand — wipe punishes the board, the anthem stays as a rebuild lever"},
		},
		degradationPaths: []DegradationPath{
			{Setback: "Board wiped",
				Recover: "Recur token producer with mass recursion (Resurrection / Living Death) — tokens are easy to rebuild"},
		},
	},
	"Lands Matter": {
		turnByTurn: []TurnPlan{
			{Turn: 1, Action: "Play a land", Note: "landfall trigger live by turn 1"},
			{Turn: 2, Action: "Extra-land effect (Exploration, Burgeoning, Azusa)", Note: "more land drops = more triggers"},
			{Turn: 3, Action: "{commander} or landfall payoff", Note: "Tireless Tracker / Lord Windgrace value engine"},
			{Turn: 4, Action: "Big-mana spell or recursion (Splendid Reclamation / Scapeshift)", Note: "convert lands into win condition fuel"},
			{Turn: 5, Action: "Lethal landfall trigger OR utility-land swing", Note: "Avenger of Zendikar / Field of the Dead / Crucible loop"},
		},
		decisionPoints: []DecisionPoint{
			{Trigger: "Land destruction on table (Strip Mine / Wasteland)",
				Action: "Crucible of Worlds / Ramunap Excavator FIRST — turn LD into your fuel"},
		},
		degradationPaths: []DegradationPath{
			{Setback: "Armageddon-style mass land destruction",
				Recover: "Splendid Reclamation / Life from the Loam — Lands Matter has the recursion built in"},
		},
	},
}

// buildGameplanScript produces the per-deck GameplanScript from the
// archetype template + a few DeckProfile holes. Returns nil when the
// archetype is empty (defensive — the caller should check before
// dereferencing).
func buildGameplanScript(dp *DeckProfile, report *FreyaReport) *GameplanScript {
	if dp == nil || dp.PrimaryArchetype == "" {
		return nil
	}
	tmpl, ok := archetypeGameplanTemplates[dp.PrimaryArchetype]
	if !ok {
		tmpl = archetypeGameplanTemplates["Midrange"]
	}

	commander := ""
	if report != nil {
		commander = report.Commander
	}
	finisher := dp.PrimaryWinLine
	if finisher == "" {
		finisher = "the wincon"
	}

	script := &GameplanScript{
		Archetype: dp.PrimaryArchetype,
		Commander: commander,
	}
	for _, t := range tmpl.turnByTurn {
		script.TurnByTurn = append(script.TurnByTurn, TurnPlan{
			Turn:   t.Turn,
			Action: substituteHoles(t.Action, commander, finisher),
			Note:   substituteHoles(t.Note, commander, finisher),
		})
	}
	for _, d := range tmpl.decisionPoints {
		script.DecisionPoints = append(script.DecisionPoints, DecisionPoint{
			Trigger: substituteHoles(d.Trigger, commander, finisher),
			Action:  substituteHoles(d.Action, commander, finisher),
		})
	}
	for _, d := range tmpl.degradationPaths {
		script.DegradationPaths = append(script.DegradationPaths, DegradationPath{
			Setback: substituteHoles(d.Setback, commander, finisher),
			Recover: substituteHoles(d.Recover, commander, finisher),
		})
	}
	return script
}

// substituteHoles fills {commander} / {finisher} template holes. Empty
// replacements leave the literal token in place — better than rendering
// "Cast ." or "Tutor for ." when context is missing.
func substituteHoles(s, commander, finisher string) string {
	if s == "" {
		return s
	}
	if commander != "" {
		s = strings.ReplaceAll(s, "{commander}", commander)
	}
	if finisher != "" {
		s = strings.ReplaceAll(s, "{finisher}", finisher)
	}
	return s
}

// renderGameplanScript writes the script to an io.Writer in the
// text-report format. Returns silently when script is nil (so the
// renderer can be called unconditionally).
func renderGameplanScript(write func(string), script *GameplanScript) {
	if script == nil {
		return
	}
	if len(script.TurnByTurn) > 0 {
		write("  Turn-by-turn ideal sequence:\n")
		for _, t := range script.TurnByTurn {
			write(fmt.Sprintf("    T%d: %s\n", t.Turn, t.Action))
			if t.Note != "" {
				write(fmt.Sprintf("        — %s\n", t.Note))
			}
		}
	}
	if len(script.DecisionPoints) > 0 {
		write("  Branching decisions:\n")
		for _, d := range script.DecisionPoints {
			write(fmt.Sprintf("    IF %s\n", d.Trigger))
			write(fmt.Sprintf("       THEN %s\n", d.Action))
		}
	}
	if len(script.DegradationPaths) > 0 {
		write("  Graceful degradation:\n")
		for _, d := range script.DegradationPaths {
			write(fmt.Sprintf("    WHEN %s\n", d.Setback))
			write(fmt.Sprintf("         %s\n", d.Recover))
		}
	}
}
