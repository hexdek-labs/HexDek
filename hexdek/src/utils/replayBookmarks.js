// Replay bookmark store.
//
// Lets a spectator mark interesting turns while watching a replay — a
// big combat, the turn the combo went off, the boardwipe — and jump
// back to them later. Pure helpers (storage passed as a parameter)
// keep the dedupe / sort / serialize logic unit-testable without
// touching localStorage in the test runner.
//
// Storage shape: localStorage key `hexdek.replay.bookmarks.<gameId>`
// holds a JSON array of bookmark objects:
//
//   { id, turn, kind, note, createdAt }
//
// `id` is a deterministic string (`turn-kind`) so the same kind on
// the same turn dedupes on a second add (note gets overwritten). That
// keeps the slider track from growing duplicate markers when a user
// taps "+" twice — they get the most recent annotation instead of
// two stacked entries.

export const BOOKMARK_KINDS = [
  { id: 'combat',   label: 'Combat',     icon: '⚔', color: 'var(--danger)' },
  { id: 'combo',    label: 'Combo turn', icon: '∞', color: 'var(--warn)' },
  { id: 'wipe',     label: 'Board wipe', icon: '☄', color: 'var(--danger)' },
  { id: 'note',     label: 'Note',       icon: '★', color: 'var(--ok)' },
]
export const DEFAULT_BOOKMARK_KIND = 'note'

const STORAGE_PREFIX = 'hexdek.replay.bookmarks.'

export function bookmarkStorageKey(gameId) {
  return STORAGE_PREFIX + String(gameId ?? 'unknown')
}

export function isValidKind(kind) {
  return BOOKMARK_KINDS.some(k => k.id === kind)
}

function makeId(turn, kind) {
  return `${turn}-${kind}`
}

// loadBookmarks returns the saved list for the given game, or [] if
// none / malformed. `storage` is any object with getItem(key) — pass
// window.localStorage at the call site; tests pass a stub.
export function loadBookmarks(storage, gameId) {
  if (!storage || gameId == null) return []
  try {
    const raw = storage.getItem(bookmarkStorageKey(gameId))
    if (!raw) return []
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter(b =>
      b && typeof b === 'object' &&
      Number.isFinite(b.turn) &&
      isValidKind(b.kind)
    )
  } catch {
    return []
  }
}

export function saveBookmarks(storage, gameId, list) {
  if (!storage || gameId == null) return
  try {
    storage.setItem(bookmarkStorageKey(gameId), JSON.stringify(list))
  } catch {
    // Quota exceeded / private-mode storage — silently ignore. The
    // in-memory list still works for the rest of the session.
  }
}

// addBookmark inserts or updates a bookmark at the given turn+kind.
// Returns a new list (immutable update) sorted by turn ascending,
// then by kind index so the slider markers and the bookmark list
// render in the same order.
export function addBookmark(list, turn, kind, note, now = Date.now()) {
  if (!Number.isFinite(turn)) return list
  const k = isValidKind(kind) ? kind : DEFAULT_BOOKMARK_KIND
  const id = makeId(turn, k)
  const filtered = (Array.isArray(list) ? list : []).filter(b => b.id !== id)
  const next = [...filtered, {
    id,
    turn,
    kind: k,
    note: typeof note === 'string' ? note : '',
    createdAt: now,
  }]
  return sortBookmarks(next)
}

export function removeBookmark(list, id) {
  if (!Array.isArray(list)) return []
  return list.filter(b => b.id !== id)
}

export function sortBookmarks(list) {
  if (!Array.isArray(list)) return []
  const kindOrder = new Map(BOOKMARK_KINDS.map((k, i) => [k.id, i]))
  return [...list].sort((a, b) => {
    if (a.turn !== b.turn) return a.turn - b.turn
    return (kindOrder.get(a.kind) ?? 99) - (kindOrder.get(b.kind) ?? 99)
  })
}

export function findBookmarkAtTurn(list, turn) {
  if (!Array.isArray(list)) return undefined
  return list.find(b => b.turn === turn)
}

// nextBookmarkTurn returns the turn number of the next bookmark
// strictly forward (direction = +1) or backward (-1) of `currentTurn`,
// or null if no such bookmark exists. The list is assumed unsorted —
// we sort by turn here so callers can pass whatever order they
// have.
export function nextBookmarkTurn(list, currentTurn, direction) {
  if (!Array.isArray(list) || list.length === 0) return null
  const turns = [...new Set(list.map(b => b.turn))].sort((a, b) => a - b)
  if (direction > 0) {
    for (const t of turns) if (t > currentTurn) return t
    return null
  }
  if (direction < 0) {
    for (let i = turns.length - 1; i >= 0; i--) {
      if (turns[i] < currentTurn) return turns[i]
    }
    return null
  }
  return null
}

// kindMeta returns the BOOKMARK_KINDS entry for the given id, or the
// default-note entry as a fallback so callers can render unconditional
// `meta.icon` / `meta.color` without null checks.
export function kindMeta(kindId) {
  return BOOKMARK_KINDS.find(k => k.id === kindId)
    ?? BOOKMARK_KINDS.find(k => k.id === DEFAULT_BOOKMARK_KIND)
}

// resolveBookmarkKeyAction maps a KeyboardEvent to a bookmark action,
// or null to pass through. Keeps the same suppression rules as the
// other replay key handlers (no system modifiers, no editable
// targets).
//
//   B         add bookmark at current turn (default kind)
//   ,         jump to previous bookmark
//   .         jump to next bookmark
//
// `,` and `.` are the angle-bracket keys on a standard layout —
// natural shift-less aliases for "step backward / forward through
// my markers."
export function resolveBookmarkKeyAction(event) {
  if (!event) return null
  if (event.metaKey || event.ctrlKey || event.altKey) return null
  const t = event.target
  if (t) {
    const tag = t.tagName
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return null
    if (t.isContentEditable === true) return null
  }
  switch (event.key) {
    case 'b':
    case 'B':
      return 'addBookmark'
    case ',':
    case '<':
      return 'prevBookmark'
    case '.':
    case '>':
      return 'nextBookmark'
    default:
      return null
  }
}
