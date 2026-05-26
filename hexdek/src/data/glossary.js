// Centralized glossary for the HexDek frontend. Every stat, label, and
// metric exposed in the UI should have an entry here so GlossaryTerm
// can present a tap-to-expand explanation. Add new terms when wiring
// new analytics into the UI; do not write inline tooltips.
//
// Shape: { title, body, source? }
//   - title: short canonical name (used in the disclosure header)
//   - body: 1-3 sentence plain-language explanation. No jargon without
//           a follow-up sentence. Speak to a brand-new player first
//           and a tournament grinder second.
//   - source: optional pointer to where the value comes from (engine
//             package, Freya analyzer, etc.) so curious players can
//             follow the trail.

export const GLOSSARY = {
  // ── Rating systems ────────────────────────────────────────────────
  hexelo: {
    title: 'HexELO',
    body: 'HexDek\'s composite skill rating. Combines a TrueSkill mean (μ) with a confidence penalty so unproven decks don\'t leapfrog veterans. Higher is better; new decks start near 1500.',
    source: 'internal/trueskill + internal/db',
  },
  ts_mu: {
    title: 'TrueSkill μ (mu)',
    body: 'The raw skill estimate from the TrueSkill model — what we\'d guess your true rating is if we had infinite games. HexELO discounts this by uncertainty; μ alone does not.',
    source: 'internal/trueskill',
  },
  ts_sigma: {
    title: 'TrueSkill σ (sigma)',
    body: 'Uncertainty around your μ. Big σ means we have not seen the deck enough to be confident. Shrinks with games played.',
    source: 'internal/trueskill',
  },
  delta: {
    title: 'Delta',
    body: 'How much HexELO has moved over the recent game window. Green = climbing, red = sliding. A large delta on a low-confidence deck is mostly noise.',
  },
  win_rate: {
    title: 'Win Rate',
    body: 'Wins divided by total finished games for this deck. In 4-player Commander a 25% baseline is "average"; sustained 30%+ is strong.',
  },
  games: {
    title: 'Games',
    body: 'Total finished games this deck has played in the HexDek system. Confidence in every other stat scales with this number.',
  },
  record: {
    title: 'Record',
    body: 'Wins versus losses across all recorded games. Draws and unfinished games are not counted here.',
  },
  confidence: {
    title: 'Confidence',
    body: 'How much we trust the displayed rating. Driven by games played and TrueSkill σ. Few games = dim dots = treat the number with a grain of salt.',
  },
  best_elo: {
    title: 'Best ELO',
    body: 'The highest HexELO any of your decks has reached. A career-high marker, not the current rating of any deck.',
  },

  // ── Brackets / power level ────────────────────────────────────────
  bracket: {
    title: 'Bracket',
    body: 'WotC-style power tier from B1 (precon-floor) to B5 (cEDH ceiling). HexDek\'s bracket reflects the cards in the deck — fast mana, tutors, infinite combos, and game-enders all push it up.',
    source: 'cmd/hexdek-freya — bracket scorer',
  },
  bracket_b1: {
    title: 'Bracket B1',
    body: 'Exhibition / precon level. No infinite combos, no Game Changers, no two-card kills. Built to play, not to win every game.',
  },
  bracket_b2: {
    title: 'Bracket B2',
    body: 'Core power level. Tuned but accessible. Light mass land destruction, no early game-ending combos.',
  },
  bracket_b3: {
    title: 'Bracket B3',
    body: 'Upgraded / "high power". Up to three Game Changers, late-game combos allowed, but no turn-1-3 kills.',
  },
  bracket_b4: {
    title: 'Bracket B4',
    body: 'Optimized non-cEDH. Unlimited Game Changers, fast mana, tutors, early combos — but not built strictly for the cEDH meta.',
  },
  bracket_b5: {
    title: 'Bracket B5',
    body: 'cEDH. The deck is constructed to win or disrupt as fast as possible against other cEDH decks. Expect turn-3 wins.',
  },
  estimated_bracket: {
    title: 'Estimated Bracket',
    body: 'We think it plays more like decks in this bracket. Freya measures density signals (game changers, fast mana, tutors, combo lines, curve) and computes the felt-power bracket — surfaced when it diverges from the deck\'s self-declared bracket.',
  },
  game_changers: {
    title: 'Game Changers',
    body: 'WotC\'s curated list of cards strong enough to bump a deck\'s bracket. Mana Crypt, Cyclonic Rift, Demonic Tutor, etc. Count of these in the deck.',
  },

  // ── Curse / DNA ───────────────────────────────────────────────────
  curse: {
    title: 'Curse',
    body: 'A deck\'s personality profile — seven sliders that shape how the AI pilots it (aggression, combo patience, threat paranoia, greed, politics, drain affinity, artifact affinity). Saved per deck and tunable.',
    source: 'internal/hat — Curse system',
  },
  curse_aggression: {
    title: 'Aggression',
    body: 'How eagerly the AI attacks. High = swing whenever favorable; low = sit back, wait for windows. Affects who gets attacked and when.',
  },
  curse_combo_patience: {
    title: 'Combo Patience',
    body: 'Willingness to wait for a combo line. High patience holds pieces and protects them; low patience jams as soon as a kill is visible.',
  },
  curse_threat_paranoia: {
    title: 'Threat Paranoia',
    body: 'How loudly other players\' boards scream "kill me first." High = preemptive removal; low = save interaction for direct attacks on you.',
  },
  curse_greed: {
    title: 'Resource Greed',
    body: 'Appetite for cards and mana over board impact. High = drawing and ramping over committing; low = drop threats, sort it out later.',
  },
  curse_politics: {
    title: 'Political Memory',
    body: 'Whether the AI remembers who attacked it. High = grudges, retaliation, deals honored; low = goldfish behavior, no alliances.',
  },
  curse_drain: {
    title: 'Drain Affinity',
    body: 'Preference for life-loss / drain win conditions. High = a drain trigger reads as a finisher; low = drains are background noise.',
  },
  curse_artifact: {
    title: 'Artifact Affinity',
    body: 'How strongly the AI values artifact-based interaction and synergy. Skews mulligan, target priority, and combo evaluation.',
  },
  fitness: {
    title: 'Fitness',
    body: 'How well the current Curse profile fits the deck\'s archetype, win lines, and Freya analysis. High fitness = the AI is being asked to do what the deck was built for.',
  },

  // ── Freya: archetype / strategy ───────────────────────────────────
  archetype: {
    title: 'Archetype',
    body: 'Freya\'s best guess at the deck\'s strategic identity (Combo, Aristocrats, Voltron, Stax, Tokens, etc). Drawn from card roles, win lines, and synergy clusters.',
    source: 'cmd/hexdek-freya — archetype classifier',
  },
  win_lines: {
    title: 'Win Lines',
    body: 'The concrete ways this deck closes the game. Each win line lists the enabler, the finisher, and the path between them. Combat damage, commander damage, combos, drains, mill, etc.',
  },
  combos: {
    title: 'Combos',
    body: 'Detected multi-card interactions that produce an infinite or game-ending loop. False positives are filtered (self-exile, hand-only pieces, attack-trigger dependencies).',
    source: 'cmd/hexdek-freya — KnownCombos + dependency analyzer',
  },
  card_role: {
    title: 'Card Role',
    body: 'Why a card is in the deck — Threat, Removal, Ramp, Draw, Tutor, Wipe, Counter, Combo, Synergy, Recursion, Protection. Most cards have several; the primary role is the one with the highest strategic weight.',
  },

  // ── Freya: deck-quality scores ────────────────────────────────────
  cmdr_synergy: {
    title: 'Commander Synergy',
    body: 'Percent of nonland cards that share a theme with the commander\'s oracle text. High synergy = the deck is built around its commander; low = the commander is along for the ride.',
  },
  mana_base_grade: {
    title: 'Mana Base Grade',
    body: 'A-F letter grade for the deck\'s lands. Penalizes taplands, rewards fetches and utility lands, and weights against the deck\'s color demand. Comes with specific upgrade suggestions.',
  },
  power_percentile: {
    title: 'Power Percentile',
    body: 'Where this deck ranks within its archetype on a multi-factor power score (tutors, mana base, interaction, draw, curve, opening hands). "TOP 10%" means stronger than 90% of similar archetype decks Freya has seen.',
  },
  keepable_hands: {
    title: 'Keepable Hands',
    body: 'Out of 10,000 simulated opening hands, the percent that look mulligan-keepable: 2-4 lands and at least one action card. Higher = the deck draws more like itself.',
    source: 'cmd/hexdek-freya — Monte Carlo opening hand sim',
  },
  keepable_hands_adjusted: {
    title: 'Keepable Hands (Commander-Adjusted)',
    body: 'Same opening-hand sim, but for commander-centric decks (Voltron, high-synergy engines) it also accepts hands satisfied by ramp, a commander enabler, or natural lands to commander CMC. Better reflects how those decks actually play.',
  },
  interaction_avg_cmc: {
    title: 'Interaction Avg CMC',
    body: 'Average mana value of the deck\'s removal, counters, and disruption. Lower means you can interact early and often — usually a good thing in Commander.',
  },
  cheap_interaction: {
    title: 'Cheap Interaction',
    body: 'Count of interaction pieces costing 2 mana or less. The currency of stopping fast combo decks.',
  },
  mana_curve: {
    title: 'Mana Curve',
    body: 'Distribution of nonland card costs across CMC buckets 0-7+. Freya flags bimodal curves and top-heavy / bottom-light shapes that signal a mana-base mismatch.',
  },
  recursion_depth: {
    title: 'Recursion Depth',
    body: 'How far the value chain loops back on itself. None / Shallow / Deep / Infinite. Deep recursion makes a deck grindy; infinite is a combo-engine signal.',
  },
  protection_density: {
    title: 'Protection Density',
    body: 'Built-in protection on combo or threat pieces (hexproof, indestructible, regenerate, ward, etc). Higher = harder to disrupt the deck\'s plan.',
  },
  threat_assessment: {
    title: 'Threat Assessment',
    body: 'Cards in the meta that are bad for this deck (graveyard hate against reanimator, artifact hate against affinity, etc). Each match comes with a vulnerability summary.',
  },
  meta_position: {
    title: 'Meta Position',
    body: 'Predicted matchup quality versus other archetypes Freya tracks. Comes with reasoning, not just a number.',
  },
  card_quality_tier: {
    title: 'Card Quality Tier',
    body: 'Per-card star rating within the deck — Star, Solid, Filler, Cuttable. Driven by role overlap, win-line presence, and CMC efficiency.',
  },
  color_demand: {
    title: 'Mana Ratio',
    body: 'How much of each color the deck actually needs to cast its spells, weighted by CMC and pip count. Compared against your land base to flag color shortfalls.',
  },
  synergy_clusters: {
    title: 'Synergy Clusters',
    body: 'Theme-grouped packages of cards that pull in the same direction (e.g. "sacrifice outlets + death triggers + recursion"). Pairwise synergy scored within each cluster.',
  },

  // ── Spectator / live telemetry ────────────────────────────────────
  eval_score: {
    title: 'Eval Score',
    body: 'The AI\'s overall position score for a player, summed across all eight evaluation dimensions. Higher = better-positioned to win.',
  },
  eval_board: {
    title: 'Board Presence',
    body: 'Power and toughness on the battlefield, weighted by evasion and protection. The "do I have stuff?" axis.',
  },
  eval_cards: {
    title: 'Card Advantage',
    body: 'Cards in hand and incoming draw versus opponents. The AI tracks this asymmetrically — your card draw matters more when you have fewer cards than the table.',
  },
  eval_mana: {
    title: 'Mana Advantage',
    body: 'Mana available now and incoming, against what your spells need. Floods and droughts both show up here.',
  },
  eval_combo: {
    title: 'Combo Proximity',
    body: 'How close the AI thinks each player is to assembling a winning combo. Drives target priority for removal.',
  },
  eval_life: {
    title: 'Life Resource',
    body: 'Life total as a usable resource — paying life, surviving damage, eating commander damage. Not just "is this player alive."',
  },
  eval_threat: {
    title: 'Threat Exposure',
    body: 'How much heat each player is taking. High exposure = expect interaction, removal, attacks. Drives politics.',
  },
  eval_cmdr: {
    title: 'Commander Progress',
    body: 'Progress toward this player\'s commander-driven win condition. Voltron damage, command-zone-cast count, commander-specific milestones.',
  },
  eval_grave: {
    title: 'Graveyard Value',
    body: 'Recursion, flashback, escape, dredge — value the player can extract from cards in the yard. Reanimator decks live and die on this number.',
  },
  conviction: {
    title: 'Conviction',
    body: 'How sure the AI is that it can still win. Drops when its relative position collapses. At zero conviction the AI may concede.',
    source: 'internal/hat — Conviction system',
  },
  budget: {
    title: 'Search Budget',
    body: 'How deep the AI is allowed to think on this decision. 0 = pure heuristic, 1-199 = evaluator-guided, 200+ = full rollout. Adapts down on complex boards.',
  },
  third_eye: {
    title: '3rd Eye',
    body: 'The AI\'s memory of opponents — cards seen, perceived archetype, threat assessment. Drives what it expects to play around.',
  },
  game_changer_pill: {
    title: 'Game Changer (live)',
    body: 'A card on the battlefield right now that\'s on the WotC Game Changer list. Marked so spectators can see why a player just spiked a bracket.',
  },

  // ── Misc ──────────────────────────────────────────────────────────
  member_since: {
    title: 'Member Since',
    body: 'Date the operator first imported a deck into HexDek. Used as the account-age marker.',
  },
  legality: {
    title: 'Legality',
    body: 'Whether the decklist is legal under the Commander format — 100 cards, color identity respected, no banned cards, singleton (basics excepted).',
  },
}

// Look up a glossary entry. Returns null when the id is unknown so
// callers can decide whether to render a plain label or skip the
// disclosure UI entirely.
export function getGlossaryTerm(id) {
  if (!id) return null
  const entry = GLOSSARY[id]
  return entry || null
}

export default GLOSSARY
