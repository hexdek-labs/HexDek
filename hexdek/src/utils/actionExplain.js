// Hat action-explanation helpers.
//
// When the AI ("hat") takes a notable action — casts its commander,
// drops a planeswalker, fires a board wipe, swings into a player —
// the spectator screen wants to surface the WHY without making the
// user open the analytics panel. This module derives a short
// explanation from observable snapshot telemetry: the acting seat's
// archetype, its top eval dimensions, its threat_exposure, and the
// action's category.
//
// Pure helpers (no React, no DOM, no fetch). The Spectator log row
// hover handler calls explainAction(event, seatEval, seatCommander)
// and renders the returned { headline, bullets[] } into a tooltip.

// NOTABLE_KINDS — event kinds the explainer surfaces a "why?"
// affordance for. Lands / draws / scrys are too noisy to annotate,
// and the eval-dim explanation isn't meaningful for them.
const NOTABLE_KINDS = new Set([
  'cast',
  'combat',
  'counter',
  'removal',
  'reanimate',
  'extra_turn',
  'activate',
])

// Cast-text patterns that promote a generic kind=cast into a more
// specific category — pulled from the same wipe / big-cast keyword
// sets as replayTimeline.js so an explanation lines up with what the
// timeline strip flagged.
const WIPE_PATTERNS = [
  /destroys? all /i,
  /exiles? all /i,
  /damage to each /i,
  /\bwrath\b/i,
  /\bdamnation\b/i,
  /\btoxic deluge\b/i,
  /\bcyclonic rift\b/i,
  /\bblasphemous act\b/i,
  /\bsupreme verdict\b/i,
  /\bday of judgment\b/i,
]

const COMMANDER_PATTERNS = [/\bcasts? .* commander\b/i, /\bcommander cast\b/i]
const PLANESWALKER_PATTERNS = [/\bcasts? planeswalker\b/i, /\bplaneswalker\b/i]

// EVAL_DIM_LABELS — the eight evaluation axes carried on `seat.eval`.
// Used for "top dimension" sentences. Matches the heatmap key map in
// Spectator.jsx so the strings the user sees on hover match what the
// grid labels show.
export const EVAL_DIM_LABELS = {
  board_presence:     'Board presence',
  card_advantage:     'Card advantage',
  mana_advantage:     'Mana',
  life_resource:      'Life',
  combo_proximity:    'Combo proximity',
  threat_exposure:    'Threat exposure',
  commander_progress: 'Commander progress',
  graveyard_value:    'Graveyard value',
}

// ARCHETYPE_BIAS_SENTENCES — short justification per archetype, used
// for the "archetype bias" bullet. Each line answers "why does this
// archetype value the action it just took?" in one sentence.
export const ARCHETYPE_BIAS_SENTENCES = {
  aggro:           'Aggro decks press the strongest opponent\'s life to zero before they assemble a plan.',
  midrange:        'Midrange decks trade efficiently and grind on the value axis.',
  control:         'Control decks hold up mana for instant-speed answers and outdraw the table.',
  ramp:            'Ramp decks accelerate mana to drop game-warping threats early.',
  combo:           'Combo decks tunnel toward their two- or three-card win condition.',
  stax:            'Stax decks lock the table out of resources while assembling a slow clock.',
  reanimator:      'Reanimator cheats fatties into play and protects the single threat that matters.',
  spellslinger:    'Spellslinger draws and chains cheap spells off cost-reduction enablers.',
  tribal:          'Tribal decks flood the board with creatures sharing a type for lord-effect synergy.',
  aristocrats:     'Aristocrats sacrifice their own creatures to drain the table via Blood Artist-style triggers.',
  selfmill:        'Self-mill fills its own graveyard to fuel recursion engines and graveyard wincons.',
  enchantress:     'Enchantress draws off enchantment ETBs and wins via incremental enchantment value.',
  artifacts:       'Artifacts decks chain mana rocks into cost-reduced artifact threats.',
  lifegain:        'Lifegain grows Pridemate-style threats with each life trigger and drains via Sanguine Bond / Vito.',
  voltron:         'Voltron suits up its commander with equipment + auras to deal 21 commander damage.',
  'lands matter':  'Lands-matter decks recur lands from the graveyard and combo via Field of the Dead / Scapeshift.',
  'counters matter': 'Counters-matter decks grow proliferated +1/+1 counters into a lethal board.',
  mill:            'Mill exhausts opponent libraries via wheel-style draw + mill engines.',
  storm:           'Storm chains rituals and cantrips to pump storm count for a lethal payoff.',
  superfriends:    'Superfriends tick walkers toward ultimates while protecting their loyalty.',
  blink:           'Blink re-triggers ETB effects via flicker primitives to snowball card advantage.',
  'extra combats': 'Extra-combats decks loop attack phases for commander damage and battlefield value.',
}

// classifyAction maps an event to a category id used by the
// explainer. Returns null for events that don't get an explanation.
//
// Order matters: a kind=cast event that matches a wipe pattern lands
// as 'wipe', not 'cast' — the wipe explanation is more useful.
export function classifyAction(event) {
  if (!event || typeof event !== 'object') return null
  const kind = typeof event.kind === 'string' ? event.kind.toLowerCase() : ''
  const action = typeof event.action === 'string' ? event.action : ''
  if (!NOTABLE_KINDS.has(kind)) return null
  if (kind === 'cast' || kind === 'removal') {
    if (WIPE_PATTERNS.some(re => re.test(action))) return 'wipe'
  }
  if (kind === 'cast' && PLANESWALKER_PATTERNS.some(re => re.test(action))) return 'planeswalker'
  if (kind === 'cast' && COMMANDER_PATTERNS.some(re => re.test(action))) return 'commander_cast'
  if (kind === 'cast') return 'cast'
  if (kind === 'combat') return 'combat'
  if (kind === 'counter') return 'counter'
  if (kind === 'removal') return 'removal'
  if (kind === 'reanimate') return 'reanimate'
  if (kind === 'extra_turn') return 'extra_turn'
  if (kind === 'activate') return 'activate'
  return null
}

export function isNotableAction(event) {
  return classifyAction(event) != null
}

// ACTION_HEADLINES — short one-liner per category, surfacing what
// the action IS so the tooltip's headline reads naturally above the
// archetype/eval bullets.
const ACTION_HEADLINES = {
  commander_cast: 'Commander deployment',
  planeswalker:   'Planeswalker deployment',
  wipe:           'Board wipe',
  counter:        'Counterspell',
  removal:        'Targeted removal',
  reanimate:      'Reanimation',
  extra_turn:     'Extra turn',
  combat:         'Combat decision',
  activate:       'Activated ability',
  cast:           'Spell cast',
}

// topEvalDimensions returns the top-N axes from a seat.eval object,
// sorted descending. Only considers the eight named axes (the
// derived `score` field is excluded — it's an aggregate, not a
// dimension). Returns an array of { key, label, value } entries.
export function topEvalDimensions(seatEval, n = 2) {
  if (!seatEval || typeof seatEval !== 'object') return []
  const out = []
  for (const key of Object.keys(EVAL_DIM_LABELS)) {
    const v = seatEval[key]
    if (Number.isFinite(v)) out.push({ key, label: EVAL_DIM_LABELS[key], value: v })
  }
  out.sort((a, b) => b.value - a.value)
  return out.slice(0, Math.max(0, n))
}

// formatEval renders a number like +0.34 / −0.10 / 0.00 in the same
// style as the heatmap tooltip in Spectator.jsx.
export function formatEval(v) {
  if (!Number.isFinite(v)) return '—'
  return (v >= 0 ? '+' : '') + v.toFixed(2)
}

// explainAction returns a tooltip-shaped explanation object:
//   { headline, category, bullets: [{label, value}, ...] }
//
// Falsy fields (no eval, no archetype, no top dim) are quietly
// omitted so the tooltip stays terse. Returns null when the event
// isn't notable enough to bother explaining.
export function explainAction(event, seatEval, seatCommander) {
  const category = classifyAction(event)
  if (!category) return null
  const headline = ACTION_HEADLINES[category] || 'Notable action'
  const bullets = []
  const archetype = seatEval?.archetype || null
  if (archetype) {
    bullets.push({
      label: 'Archetype',
      value: archetype.toUpperCase(),
    })
    const bias = ARCHETYPE_BIAS_SENTENCES[archetype.toLowerCase()]
    if (bias) bullets.push({ label: 'Bias', value: bias })
  }
  const top = topEvalDimensions(seatEval, 2)
  for (const t of top) {
    bullets.push({ label: t.label, value: formatEval(t.value) })
  }
  if (Number.isFinite(seatEval?.threat_exposure) && (!top.length || top[0].key !== 'threat_exposure')) {
    // Always surface threat_exposure for the explanation since "what
    // the AI is afraid of" drives most counter/removal/wipe choices.
    // Skip the redundant push if it already led the top-N list.
    bullets.push({
      label: 'Threat read',
      value: formatEval(seatEval.threat_exposure),
    })
  }
  if (Number.isFinite(seatEval?.score)) {
    bullets.push({ label: 'Score', value: formatEval(seatEval.score) })
  }
  if (seatCommander && typeof seatCommander === 'string') {
    bullets.push({ label: 'Pilot', value: seatCommander.split('//')[0].trim().toUpperCase() })
  }
  return { headline, category, bullets }
}
