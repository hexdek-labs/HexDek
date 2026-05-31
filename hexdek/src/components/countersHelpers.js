// Pure helpers powering CounterBadge. Lives in a .js file so
// node --test can exercise it directly without a JSX transform.

// COUNTER_KIND_DISPLAY maps the canonical engine counter-kind strings
// (matching internal/gameengine/state.go Permanent.Counters keys) to
// short display labels. Unknown kinds fall through to the raw value.
//
// Visual convention: +1/+1 and -1/-1 are the dominant cases and get
// the tightest two-char form ("+1" / "-1"). Loyalty / charge / etc.
// get a single-letter prefix so the badge fits on the perm tile.
const COUNTER_KIND_DISPLAY = {
  '+1/+1': '+1',
  '-1/-1': '-1',
  'loyalty': 'L',
  'charge': 'C',
  'fade': 'F',
  'time': 'T',
  'stun': 'S',
  'lore': 'L',
  'level': 'L',
  'shield': 'SH',
  'finality': 'FN',
}

// COUNTER_KIND_PRIORITY orders counter kinds for display when a perm
// carries multiple kinds. Higher priority renders first (top-left of
// the perm tile). The dominant gameplay kinds (+1/+1, -1/-1, loyalty)
// come first; flavor / utility counters come last.
const COUNTER_KIND_PRIORITY = {
  '+1/+1': 100,
  '-1/-1': 95,
  'loyalty': 90,
  'charge': 70,
  'level': 65,
  'lore': 60,
  'shield': 55,
  'fade': 40,
  'time': 35,
  'stun': 30,
  'finality': 20,
}

// formatCounterLabel returns the short display string for a counter
// kind. Falls back to the raw kind upper-cased so unknown counter
// types still render meaningfully.
export function formatCounterLabel(kind) {
  if (!kind) return ''
  const known = COUNTER_KIND_DISPLAY[kind]
  if (known != null) return known
  // Unknown kind — uppercase first 3 chars as a defensive fallback.
  return kind.toUpperCase().slice(0, 3)
}

// orderCounters takes the raw {kind: count} map from
// PermanentSnapshot.Counters and returns an ordered array of
// {kind, count, label} entries with the highest-priority kind first.
// Zero/negative counts are filtered out (defensive — the backend
// already strips zero counts before sending).
export function orderCounters(counters) {
  if (!counters || typeof counters !== 'object') return []
  const entries = []
  for (const [kind, count] of Object.entries(counters)) {
    if (typeof count !== 'number' || count === 0) continue
    entries.push({
      kind,
      count,
      label: formatCounterLabel(kind),
    })
  }
  // Sort by priority desc; unknown kinds (priority undefined) sort
  // to the END so the dominant kinds stay visible when truncated.
  entries.sort((a, b) => {
    const pa = COUNTER_KIND_PRIORITY[a.kind] ?? -1
    const pb = COUNTER_KIND_PRIORITY[b.kind] ?? -1
    if (pa !== pb) return pb - pa
    return a.kind.localeCompare(b.kind)
  })
  return entries
}

// counterBadgeColor returns the CSS color value for a counter kind's
// badge. +1/+1 is green (good), -1/-1 is danger (bad), loyalty +
// utility counters are accent. Falls back to ink-2 for unknowns.
export function counterBadgeColor(kind) {
  switch (kind) {
    case '+1/+1': return 'var(--ok)'
    case '-1/-1': return 'var(--danger)'
    case 'loyalty': return 'var(--accent, #c8a26b)'
    default:       return 'var(--ink-2)'
  }
}
