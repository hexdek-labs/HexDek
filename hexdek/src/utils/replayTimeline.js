// Replay event-timeline helpers.
//
// Walks a timeline's per-turn events and surfaces the "key moments"
// (elimination, board wipe, extra turn, combat, counter, removal,
// reanimation) so the Replay screen can render a horizontal strip
// of colored markers — one badge per turn, picked by the most
// significant moment that occurred during that turn.
//
// Pure helpers: classifyEvent (one event → moment kind), summarizeTurn
// (one snapshot → primary moment + counts), extractKeyMoments
// (timeline → flat list for rendering).

// Moment kinds in PRIORITY ORDER (most significant first). The
// strip picks each turn's "primary" badge by walking events through
// classifyEvent and keeping the lowest-index match.
export const KEY_MOMENT_KINDS = [
  { id: 'elimination', label: 'Elimination', icon: '✗', color: 'var(--danger)' },
  { id: 'wipe',        label: 'Board wipe',  icon: '☄', color: 'var(--danger)' },
  { id: 'extra_turn',  label: 'Extra turn',  icon: '⟳', color: 'var(--warn)' },
  { id: 'combat',      label: 'Combat',      icon: '⚔', color: 'var(--danger)' },
  { id: 'counter',     label: 'Counter',     icon: '⊘', color: 'var(--warn)' },
  { id: 'removal',     label: 'Removal',     icon: '⌫', color: 'var(--warn)' },
  { id: 'reanimate',   label: 'Reanimate',   icon: '↺', color: 'var(--warn)' },
  { id: 'big_cast',    label: 'Big cast',    icon: '✦', color: 'var(--ok)' },
]
const KIND_ORDER = new Map(KEY_MOMENT_KINDS.map((m, i) => [m.id, i]))

// ELIMINATION_KINDS mirrors Spectator.jsx — the engine emits one of
// these as a distinct event kind when a seat dies. Imported here so
// the classifier doesn't have to keep its own copy in sync.
const ELIMINATION_KINDS = new Set([
  'elimination', 'sba_704_5a', 'sba_704_5b', 'sba_704_5c', 'sba_704_5d',
])

// Keyword regex sets. Mass-removal text is the cheapest reliable
// signal for "board wipe" since the engine doesn't tag wipes with a
// dedicated event kind — Wrath of God, Damnation, Cyclonic Rift, etc.
// all surface as `kind=removal` with the spell name in the action
// string.
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
  /\bin garruk\b.*sweep/i, // long-tail flavor
]

// Big-cast text — commander cast or planeswalker cast lines. Engine
// emits kind=cast and the action carries the type/name.
const BIG_CAST_PATTERNS = [
  /\bcasts? .* commander\b/i,
  /\bcasts? planeswalker\b/i,
  /\bplaneswalker\b/i,
]

// classifyEvent maps an event (kind+action) to one of the
// KEY_MOMENT_KINDS ids, or null if it doesn't qualify. Order of the
// checks matches KEY_MOMENT_KINDS priority — an event that could
// match multiple (e.g. a "destroys all creatures" removal that also
// kills a player) reports the highest-priority match.
//
// Returns null for events that don't move the strategic needle
// (lands, taps, draws, scry, etc.) — those still show in the per-
// turn event log; they just don't earn a badge.
export function classifyEvent(event) {
  if (!event || typeof event !== 'object') return null
  const kind = typeof event.kind === 'string' ? event.kind.toLowerCase() : ''
  const action = typeof event.action === 'string' ? event.action : ''
  if (ELIMINATION_KINDS.has(kind)) return 'elimination'
  if (kind === 'removal' && WIPE_PATTERNS.some(re => re.test(action))) return 'wipe'
  if (kind === 'extra_turn') return 'extra_turn'
  if (kind === 'combat') return 'combat'
  if (kind === 'counter') return 'counter'
  if (kind === 'removal') return 'removal'
  if (kind === 'reanimate') return 'reanimate'
  if (kind === 'cast' && BIG_CAST_PATTERNS.some(re => re.test(action))) return 'big_cast'
  return null
}

// summarizeTurn walks one timeline snapshot's events and returns a
// summary object the UI can render as a single badge. `primary` is
// the highest-priority moment kind (null when nothing fires);
// `counts` is a kind → occurrence count map so a tooltip can say
// "T7 — 3 removals, 1 counter."
export function summarizeTurn(snapshot) {
  const events = snapshot?.events || []
  const counts = new Map()
  let primary = null
  for (const ev of events) {
    const k = classifyEvent(ev)
    if (!k) continue
    counts.set(k, (counts.get(k) || 0) + 1)
    const p = KIND_ORDER.get(k) ?? 999
    if (primary == null || p < (KIND_ORDER.get(primary) ?? 999)) {
      primary = k
    }
  }
  return {
    turn: snapshot?.turn,
    primary,
    counts,
    totalKeyEvents: Array.from(counts.values()).reduce((s, n) => s + n, 0),
  }
}

// extractKeyMoments runs summarizeTurn across every timeline snapshot
// and returns the list — ordered by snapshot index, NOT by turn
// number, so a caller iterating it can render `i ↔ snapshot[i]`
// without re-aligning. Snapshots that produce no moments are still
// included (with primary=null) so the strip has a consistent
// per-turn slot.
export function extractKeyMoments(timeline) {
  if (!Array.isArray(timeline)) return []
  return timeline.map(snap => summarizeTurn(snap))
}

// momentMeta returns the KEY_MOMENT_KINDS entry for an id, or null
// if the id isn't a known kind. Callers should guard the null case
// when rendering — a stray bad id should silently fall back to "no
// badge."
export function momentMeta(kindId) {
  if (!kindId) return null
  return KEY_MOMENT_KINDS.find(m => m.id === kindId) ?? null
}

// countsTooltip builds a single-line "3 removals, 1 counter, 2
// combats" summary from a counts map. Used in the strip's hover
// title so the user can see EVERYTHING that happened on a turn
// without expanding the event log.
export function countsTooltip(counts) {
  if (!counts || counts.size === 0) return ''
  const ordered = KEY_MOMENT_KINDS
    .map(m => [m, counts.get(m.id) || 0])
    .filter(([, n]) => n > 0)
  return ordered
    .map(([m, n]) => `${n} ${m.label.toLowerCase()}${n > 1 ? 's' : ''}`)
    .join(', ')
}
