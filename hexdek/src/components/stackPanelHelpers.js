// Pure helpers powering StackPanel. Lives in a .js file (vs the
// .jsx component file) so node --test can import it directly
// without a JSX transform.

// formatStackItemKind produces a 1-3 char glyph for a stack item's
// Kind discriminator. Used to prefix the source name so a quick
// glance distinguishes spells (no prefix), activated abilities
// ("ACT"), and triggered abilities ("TRG"). The empty / legacy
// kind falls through to no prefix (matches the engine's "infer
// from Card/Source" rule).
export function stackItemKindGlyph(kind) {
  switch ((kind || '').toLowerCase()) {
    case 'activated': return 'ACT'
    case 'triggered': return 'TRG'
    case 'spell':
    case '':
    case 'legacy':
      return ''
    default:
      return kind.toUpperCase()
  }
}

// formatStackItemLabel produces the human-readable label for a
// stack item — source name + a small "(copy)" / "(countered)" tag
// when those flags are set. Used inside the StackPanel list row.
//
// Copy/countered tags are surfaced inline because spectators care:
// a Counterspell that successfully resolved counters its target,
// and that target item stays on the stack as `Countered=true` until
// the engine pops it. Without the tag, the spectator log entry for
// "Spell X countered" would be the only signal — easy to miss in a
// long log.
export function formatStackItemLabel(item) {
  if (!item) return ''
  const name = item.source || '?'
  const tags = []
  if (item.is_copy) tags.push('copy')
  if (item.countered) tags.push('countered')
  return tags.length === 0 ? name : `${name} (${tags.join(', ')})`
}

// stackOrderTopFirst flips the engine's bottom-first stack order
// (Stack[0] = bottom, Stack[len-1] = top) to a top-first array for
// display. Spectators expect "the spell that resolves first" to be
// at the top of the list (visually echoes how cards are placed on
// a real game's stack). Returns a fresh array — the caller can
// safely mutate without affecting the snapshot.
export function stackOrderTopFirst(stack) {
  if (!Array.isArray(stack) || stack.length === 0) return []
  const out = []
  for (let i = stack.length - 1; i >= 0; i--) {
    out.push(stack[i])
  }
  return out
}
