// Run with: node --test hexdek/src/utils/replayBookmarks.test.js

import { test } from 'node:test'
import assert from 'node:assert/strict'

import {
  BOOKMARK_KINDS,
  DEFAULT_BOOKMARK_KIND,
  bookmarkStorageKey,
  isValidKind,
  loadBookmarks,
  saveBookmarks,
  addBookmark,
  removeBookmark,
  sortBookmarks,
  findBookmarkAtTurn,
  nextBookmarkTurn,
  kindMeta,
  resolveBookmarkKeyAction,
} from './replayBookmarks.js'

// Stub localStorage: in-memory backing object, same API surface as
// `window.localStorage` (`getItem` / `setItem` / `removeItem` / `clear`).
function makeStorage(initial = {}) {
  const data = { ...initial }
  return {
    getItem: (k) => (k in data ? data[k] : null),
    setItem: (k, v) => { data[k] = String(v) },
    removeItem: (k) => { delete data[k] },
    clear: () => { for (const k of Object.keys(data)) delete data[k] },
    _data: data, // for inspection in tests only
  }
}

function ev(opts) {
  return {
    key: opts.key,
    metaKey: !!opts.meta,
    ctrlKey: !!opts.ctrl,
    altKey: !!opts.alt,
    target: opts.target ?? { tagName: 'BODY', isContentEditable: false },
  }
}

// -----------------------------------------------------------------------------
// Static config
// -----------------------------------------------------------------------------

test('BOOKMARK_KINDS lists combat / combo / wipe / note', () => {
  const ids = BOOKMARK_KINDS.map(k => k.id)
  assert.deepEqual(ids, ['combat', 'combo', 'wipe', 'note'])
})

test('DEFAULT_BOOKMARK_KIND points to a valid entry', () => {
  assert.equal(isValidKind(DEFAULT_BOOKMARK_KIND), true)
})

test('isValidKind rejects unknowns', () => {
  assert.equal(isValidKind('combat'), true)
  assert.equal(isValidKind('hocus pocus'), false)
  assert.equal(isValidKind(null), false)
})

test('kindMeta returns the entry for a known kind, falls back to note', () => {
  assert.equal(kindMeta('combat').label, 'Combat')
  assert.equal(kindMeta('unknown').id, DEFAULT_BOOKMARK_KIND)
})

test('bookmarkStorageKey is namespaced per game', () => {
  assert.equal(bookmarkStorageKey(42),      'hexdek.replay.bookmarks.42')
  assert.equal(bookmarkStorageKey('abc'),   'hexdek.replay.bookmarks.abc')
  assert.equal(bookmarkStorageKey(null),    'hexdek.replay.bookmarks.unknown')
})

// -----------------------------------------------------------------------------
// add / remove / sort / find
// -----------------------------------------------------------------------------

test('addBookmark inserts a new bookmark with the kind+turn id', () => {
  const list = addBookmark([], 5, 'combat', 'big swing', 1000)
  assert.equal(list.length, 1)
  assert.equal(list[0].turn, 5)
  assert.equal(list[0].kind, 'combat')
  assert.equal(list[0].note, 'big swing')
  assert.equal(list[0].id, '5-combat')
  assert.equal(list[0].createdAt, 1000)
})

test('addBookmark dedupes on (turn, kind) and updates the note', () => {
  let list = addBookmark([], 5, 'combat', 'first', 1000)
  list      = addBookmark(list, 5, 'combat', 'updated', 2000)
  assert.equal(list.length, 1)
  assert.equal(list[0].note, 'updated')
  assert.equal(list[0].createdAt, 2000)
})

test('addBookmark on same turn with different kind keeps both', () => {
  let list = addBookmark([], 5, 'combat', '')
  list      = addBookmark(list, 5, 'combo', '')
  assert.equal(list.length, 2)
})

test('addBookmark coerces an invalid kind to the default', () => {
  const list = addBookmark([], 3, 'sparkles', 'whoops')
  assert.equal(list[0].kind, DEFAULT_BOOKMARK_KIND)
})

test('addBookmark coerces missing/non-string note to empty string', () => {
  assert.equal(addBookmark([], 3, 'note', undefined)[0].note, '')
  assert.equal(addBookmark([], 3, 'note', null)[0].note, '')
  assert.equal(addBookmark([], 3, 'note', 42)[0].note, '')
})

test('addBookmark with non-finite turn returns the list unchanged', () => {
  const before = [{ id: '1-note', turn: 1, kind: 'note', note: '' }]
  assert.equal(addBookmark(before, NaN, 'note', ''), before)
  assert.equal(addBookmark(before, undefined, 'note', ''), before)
})

test('addBookmark sorts the returned list by turn then kind', () => {
  let list = []
  list = addBookmark(list, 10, 'note',   '')
  list = addBookmark(list, 3,  'wipe',   '')
  list = addBookmark(list, 3,  'combat', '')
  list = addBookmark(list, 7,  'combo',  '')
  assert.deepEqual(list.map(b => `${b.turn}-${b.kind}`), [
    '3-combat',
    '3-wipe',
    '7-combo',
    '10-note',
  ])
})

test('removeBookmark drops by id', () => {
  const list = sortBookmarks([
    { id: '3-combat', turn: 3, kind: 'combat', note: '' },
    { id: '7-combo',  turn: 7, kind: 'combo',  note: '' },
  ])
  const r = removeBookmark(list, '3-combat')
  assert.equal(r.length, 1)
  assert.equal(r[0].id, '7-combo')
})

test('findBookmarkAtTurn returns the first matching bookmark', () => {
  const list = [{ id: '5-combat', turn: 5, kind: 'combat', note: '' }]
  assert.equal(findBookmarkAtTurn(list, 5).kind, 'combat')
  assert.equal(findBookmarkAtTurn(list, 6), undefined)
})

// -----------------------------------------------------------------------------
// nextBookmarkTurn
// -----------------------------------------------------------------------------

test('nextBookmarkTurn forward picks the next strictly greater turn', () => {
  const list = [
    { id: '3-note',  turn: 3,  kind: 'note',  note: '' },
    { id: '7-note',  turn: 7,  kind: 'note',  note: '' },
    { id: '12-note', turn: 12, kind: 'note',  note: '' },
  ]
  assert.equal(nextBookmarkTurn(list, 0,  +1), 3)
  assert.equal(nextBookmarkTurn(list, 5,  +1), 7)
  assert.equal(nextBookmarkTurn(list, 11, +1), 12)
  assert.equal(nextBookmarkTurn(list, 12, +1), null,
    'no bookmark beyond the last')
})

test('nextBookmarkTurn backward picks the next strictly smaller turn', () => {
  const list = [
    { id: '3-note',  turn: 3,  kind: 'note',  note: '' },
    { id: '7-note',  turn: 7,  kind: 'note',  note: '' },
    { id: '12-note', turn: 12, kind: 'note',  note: '' },
  ]
  assert.equal(nextBookmarkTurn(list, 12, -1), 7)
  assert.equal(nextBookmarkTurn(list, 7,  -1), 3)
  assert.equal(nextBookmarkTurn(list, 3,  -1), null)
  assert.equal(nextBookmarkTurn(list, 0,  -1), null)
})

test('nextBookmarkTurn dedupes shared turns across kinds', () => {
  // Two bookmarks on turn 5 (one combat, one combo) shouldn't produce
  // two equal jumps in a row.
  const list = [
    { id: '5-combat', turn: 5, kind: 'combat', note: '' },
    { id: '5-combo',  turn: 5, kind: 'combo',  note: '' },
    { id: '9-note',   turn: 9, kind: 'note',   note: '' },
  ]
  assert.equal(nextBookmarkTurn(list, 5, +1), 9, 'skip past the shared turn')
})

test('nextBookmarkTurn returns null for empty / bad inputs', () => {
  assert.equal(nextBookmarkTurn([], 5, +1), null)
  assert.equal(nextBookmarkTurn(null, 5, +1), null)
  assert.equal(nextBookmarkTurn([{ id: '5-note', turn: 5, kind: 'note', note: '' }], 5, 0), null)
})

// -----------------------------------------------------------------------------
// Storage round-trip
// -----------------------------------------------------------------------------

test('saveBookmarks / loadBookmarks round-trip per game id', () => {
  const storage = makeStorage()
  const a = addBookmark([], 3, 'combat', 'big')
  saveBookmarks(storage, 42, a)
  // Different game id ⇒ no leakage.
  assert.deepEqual(loadBookmarks(storage, 99), [])
  // Same id ⇒ identical list.
  assert.deepEqual(loadBookmarks(storage, 42), a)
})

test('loadBookmarks rejects malformed JSON', () => {
  const storage = makeStorage({
    [bookmarkStorageKey(42)]: '{not json',
  })
  assert.deepEqual(loadBookmarks(storage, 42), [])
})

test('loadBookmarks rejects non-array payload', () => {
  const storage = makeStorage({
    [bookmarkStorageKey(42)]: JSON.stringify({ foo: 1 }),
  })
  assert.deepEqual(loadBookmarks(storage, 42), [])
})

test('loadBookmarks filters out entries with bad turn/kind', () => {
  const storage = makeStorage({
    [bookmarkStorageKey(42)]: JSON.stringify([
      { turn: 3, kind: 'combat', note: 'ok' },
      { turn: 'oops', kind: 'combat' },         // bad turn
      { turn: 5, kind: 'space lasers' },        // bad kind
      null,
    ]),
  })
  const list = loadBookmarks(storage, 42)
  assert.equal(list.length, 1)
  assert.equal(list[0].kind, 'combat')
})

test('saveBookmarks tolerates a storage throw (quota / private mode)', () => {
  const throwingStorage = {
    getItem: () => null,
    setItem: () => { throw new Error('quota') },
  }
  // Must not propagate.
  saveBookmarks(throwingStorage, 42, [{ id: '1-note', turn: 1, kind: 'note', note: '' }])
})

test('loadBookmarks returns [] when storage is null or game id missing', () => {
  assert.deepEqual(loadBookmarks(null, 42), [])
  assert.deepEqual(loadBookmarks(makeStorage(), null), [])
})

// -----------------------------------------------------------------------------
// Keyboard
// -----------------------------------------------------------------------------

test('B adds a bookmark', () => {
  assert.equal(resolveBookmarkKeyAction(ev({ key: 'b' })), 'addBookmark')
  assert.equal(resolveBookmarkKeyAction(ev({ key: 'B' })), 'addBookmark')
})

test('Comma and period step between bookmarks (with angle-bracket aliases)', () => {
  assert.equal(resolveBookmarkKeyAction(ev({ key: ',' })), 'prevBookmark')
  assert.equal(resolveBookmarkKeyAction(ev({ key: '<' })), 'prevBookmark')
  assert.equal(resolveBookmarkKeyAction(ev({ key: '.' })), 'nextBookmark')
  assert.equal(resolveBookmarkKeyAction(ev({ key: '>' })), 'nextBookmark')
})

test('Bookmark keys suppress on modifiers and editable targets', () => {
  for (const mod of ['meta', 'ctrl', 'alt']) {
    assert.equal(resolveBookmarkKeyAction(ev({ key: 'b', [mod]: true })), null,
      `B + ${mod} should pass through`)
  }
  for (const tag of ['INPUT', 'TEXTAREA', 'SELECT']) {
    assert.equal(resolveBookmarkKeyAction(ev({ key: 'b', target: { tagName: tag } })), null,
      `B inside <${tag}> must type instead of bookmarking`)
  }
  assert.equal(
    resolveBookmarkKeyAction(ev({ key: ',', target: { tagName: 'DIV', isContentEditable: true } })),
    null,
  )
})

test('Unbound bookmark keys return null', () => {
  for (const k of ['a', 'Enter', 'Escape', 'ArrowLeft', 'Tab']) {
    assert.equal(resolveBookmarkKeyAction(ev({ key: k })), null)
  }
})
