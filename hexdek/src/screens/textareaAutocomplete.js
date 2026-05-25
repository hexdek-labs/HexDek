// Pure helpers for the Workshop textarea card-name autocomplete. Lives
// in its own module so the fragment extraction + suggestion application
// can be unit-tested without React, then composed into the textarea
// wrapper inside DeckArchive.jsx.

// Minimum characters typed before we'll fire a search query. Two chars
// keeps single-letter mistypes from spamming the API while still firing
// fast enough that the dropdown appears mid-word.
export const MIN_FRAGMENT_CHARS = 2

// extractFragment inspects the textarea `text` at `caret` (string
// offset). Returns the "card name being typed on the current line" so
// the caller can run a search and propose completions. Returns null
// when autocomplete should NOT fire:
//   - blank line / caret at start of line
//   - fragment shorter than MIN_FRAGMENT_CHARS
//   - the caret sits before non-whitespace tail content (user is editing
//     an existing name in the middle of a finished line)
//
// Result shape:
//   {
//     fragment: 'Sol Ri',         // raw text from after the qty prefix up to the caret
//     fragmentStart: number,      // string offset of fragment[0]
//     fragmentEnd: number,        // equal to caret (where typing continues)
//     qtyPrefix: '1 ',            // matched "N " or "COMMANDER:" prefix (preserved on apply)
//     lineStart: number,
//     lineEnd: number,
//   }
export function extractFragment(text, caret) {
  if (typeof text !== 'string' || typeof caret !== 'number') return null
  if (caret < 0 || caret > text.length) return null

  // Find current line bounds.
  let lineStart = text.lastIndexOf('\n', caret - 1)
  lineStart = lineStart < 0 ? 0 : lineStart + 1
  let lineEnd = text.indexOf('\n', caret)
  if (lineEnd < 0) lineEnd = text.length

  const line = text.slice(lineStart, lineEnd)
  const beforeCaret = text.slice(lineStart, caret)
  const afterCaret = line.slice(caret - lineStart)

  // Suppress when the caret sits before non-whitespace tail. Lets the
  // user click into the middle of an existing name to fix a typo
  // without an unsolicited dropdown.
  if (afterCaret.trim().length > 0) return null

  // Match the leading qty prefix ("1 ", "12   ", "COMMANDER: ") so we
  // can preserve it when applying a suggestion. Anything before the
  // fragment goes back unchanged.
  const prefixMatch = beforeCaret.match(/^(\s*\d+\s+|\s*COMMANDER:\s*)?(.*)$/i)
  if (!prefixMatch) return null
  const qtyPrefix = prefixMatch[1] || ''
  const rawFragment = prefixMatch[2] || ''
  const fragment = rawFragment.trimEnd()

  if (fragment.length < MIN_FRAGMENT_CHARS) return null

  const fragmentStart = lineStart + qtyPrefix.length
  const fragmentEnd = caret

  return { fragment, fragmentStart, fragmentEnd, qtyPrefix, lineStart, lineEnd }
}

// applySuggestion swaps the fragment range with the picked card name,
// preserving any qty prefix. Adds a default "1 " when the user typed a
// bare name (no leading qty / commander marker) so the saved line is
// already valid decklist syntax.
//
// Returns { text, caret } — the new textarea value and the caret
// position to set after the replacement.
export function applySuggestion(text, fragmentInfo, suggestion) {
  if (!fragmentInfo || typeof suggestion !== 'string' || !suggestion) {
    return { text, caret: text.length }
  }
  const { fragmentStart, fragmentEnd, qtyPrefix, lineStart } = fragmentInfo

  // If the user typed a bare name (no qty / no commander prefix), insert
  // a default "1 " so the saved line parses. We do this by widening the
  // replacement range to lineStart so the prefix lands in the right
  // place without duplicating earlier whitespace.
  const needsDefaultQty = qtyPrefix.trim() === ''
  const replaceStart = needsDefaultQty ? lineStart : fragmentStart
  const replacement = needsDefaultQty ? `1 ${suggestion}` : suggestion

  const next = text.slice(0, replaceStart) + replacement + text.slice(fragmentEnd)
  return { text: next, caret: replaceStart + replacement.length }
}

// nextSuggestionIndex advances the keyboard-highlighted suggestion
// index in the dropdown. Wraps both directions, clamps to the result
// length, and tolerates an empty list (returns 0 so the dropdown
// renders nothing pinned).
export function nextSuggestionIndex(current, delta, length) {
  if (!length || length <= 0) return 0
  const n = ((current + delta) % length + length) % length
  return n
}
