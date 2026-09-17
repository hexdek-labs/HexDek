import { useMemo, useRef, useState, useEffect } from 'react'
import { useModalKeyboard } from '../hooks/useModalKeyboard'
import { cardArtUrl } from '../services/api'

// CommanderPickerModal — resolves a deck's commander before Freya runs.
//
// The deck-import pipeline derives the commander by parsing the decklist /
// filename, which fails for set codes, `//Commander` headers, and
// punctuation. Those decks land "imported" but with an unset / unresolved
// commander, so Freya analyzes limbo (or the deck reads as illegal —
// "commander not found"). This modal is the gate: when a deck lands with an
// ambiguous commander we prompt the user to pin it here, THEN analysis runs.
//
// The control is a FILTERABLE COMBOBOX, not a plain dropdown: it opens
// showing the auto-detected legendary-creature candidates (usually one, no
// typing needed to confirm the obvious pick), but the user can type to
// filter across the ENTIRE deck. That fallback is required, not optional —
// a dropdown of only the detected set would re-trap exactly the users whose
// commander the detector missed, which is the bug we're fixing.
//
// Props:
//   deck       — the getDeck payload ({ cards: [{name, type_line, quantity}] })
//   deckId     — "owner/id" for the header label
//   candidates — precomputed legendary-creature card rows (optional; derived
//                from deck.cards when omitted)
//   saving     — parent is persisting + kicking off analysis; disables input
//   onCancel   — dismiss without pinning (deck stays unanalyzed)
//   onConfirm(name) — pin this commander, persist, then run Freya

// Strip a "COMMANDER: " prefix and a trailing "(SET) 123" printing tag so
// the label reads clean; the underlying option value keeps the full name.
function cleanName(name) {
  return String(name || '')
    .replace(/^COMMANDER:\s*/i, '')
    .replace(/\s*\([^)]*\)\s*[\dA-Za-z-]*$/, '')
    .trim()
}

function isLegendaryCreature(card) {
  const t = String(card?.type_line || '').toLowerCase()
  return t.includes('legendary') && t.includes('creature')
}

export default function CommanderPickerModal({ deck, deckId, candidates, saving = false, onCancel, onConfirm }) {
  const panelRef = useModalKeyboard({ onClose: onCancel, trapFocus: false })
  const inputRef = useRef(null)
  const listRef = useRef(null)

  const cards = deck?.cards || []

  // Full pick pool — every distinct card in the deck (nothing un-pickable),
  // keyed by clean display name so a card + its COMMANDER: twin dedupe.
  const allOptions = useMemo(() => {
    const seen = new Set()
    const out = []
    for (const c of cards) {
      const value = String(c.name || '').replace(/^COMMANDER:\s*/i, '').trim()
      if (!value) continue
      const key = value.toLowerCase()
      if (seen.has(key)) continue
      seen.add(key)
      out.push({ value, label: cleanName(c.name), type_line: c.type_line || '', legendary: isLegendaryCreature(c) })
    }
    return out
  }, [cards])

  // Detected candidates — legendary creatures. Falls back to the full pool
  // when the detector found none, so the list is never empty to start.
  const detected = useMemo(() => {
    if (Array.isArray(candidates) && candidates.length) {
      const seen = new Set()
      const out = []
      for (const c of candidates) {
        const value = String(c.name || '').replace(/^COMMANDER:\s*/i, '').trim()
        if (!value || seen.has(value.toLowerCase())) continue
        seen.add(value.toLowerCase())
        out.push({ value, label: cleanName(c.name), type_line: c.type_line || '', legendary: true })
      }
      return out
    }
    return allOptions.filter(o => o.legendary)
  }, [candidates, allOptions])

  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState(() => (detected.length === 1 ? detected[0].value : ''))
  const [highlight, setHighlight] = useState(0)

  // When the user has typed, filter across the WHOLE deck; otherwise show
  // the short detected-candidate list (or the full pool if none detected).
  const q = query.trim().toLowerCase()
  const visible = useMemo(() => {
    if (!q) return detected.length ? detected : allOptions
    return allOptions.filter(o => o.label.toLowerCase().includes(q) || o.value.toLowerCase().includes(q))
  }, [q, detected, allOptions])

  useEffect(() => { setHighlight(0) }, [q])
  useEffect(() => { inputRef.current?.focus() }, [])

  const pick = (opt) => {
    if (!opt) return
    setSelected(opt.value)
    setQuery(opt.label)
  }

  const onKeyDown = (e) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setHighlight(h => Math.min(h + 1, visible.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setHighlight(h => Math.max(h - 1, 0))
    } else if (e.key === 'Enter') {
      e.preventDefault()
      if (visible[highlight]) pick(visible[highlight])
    }
  }

  // Keep the highlighted row in view during keyboard nav.
  useEffect(() => {
    const el = listRef.current?.querySelector('[data-hl="1"]')
    if (el) el.scrollIntoView({ block: 'nearest' })
  }, [highlight])

  // Confirm resolves to the explicit selection, else an exact typed match.
  const resolved = selected
    || allOptions.find(o => o.label.toLowerCase() === q || o.value.toLowerCase() === q)?.value
    || ''

  const confirm = () => {
    if (!resolved || saving) return
    onConfirm?.(resolved)
  }

  const total = allOptions.length

  return (
    <div className="export-modal" onMouseDown={saving ? undefined : onCancel}>
      <div
        ref={panelRef}
        className="export-modal__panel"
        onMouseDown={e => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label="Select commander"
        style={{ width: 'min(560px, 94vw)' }}
      >
        <div className="export-modal__hd">
          <span>SELECT COMMANDER / / {(deckId || '?').toUpperCase()}</span>
          <button
            type="button"
            className="export-modal__close"
            onClick={onCancel}
            disabled={saving}
            aria-label="Close commander picker"
            style={{ background: 'transparent', border: '1px solid var(--rule-2)', color: 'inherit', font: 'inherit', cursor: saving ? 'not-allowed' : 'pointer' }}
          >ESC</button>
        </div>

        <div style={{ padding: '14px' }}>
          <div className="t-xs muted" style={{ lineHeight: 1.5, marginBottom: 12, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
            &gt; THIS DECK'S COMMANDER COULDN'T BE RESOLVED. PIN IT BELOW AND FREYA WILL
            ANALYZE THE DECK — NO MORE LIMBO.
            <br />&gt; PICK A DETECTED LEGENDARY CREATURE, OR TYPE TO SEARCH ALL {total} CARDS.
          </div>

          {/* Combobox */}
          <div style={{ position: 'relative' }}>
            <input
              ref={inputRef}
              type="text"
              className="import-modal__input"
              value={query}
              disabled={saving}
              onChange={e => { setQuery(e.target.value); setSelected('') }}
              onKeyDown={onKeyDown}
              placeholder={detected.length ? `${detected[0].label.toUpperCase()}${detected.length > 1 ? ` +${detected.length - 1} MORE` : ''} — OR TYPE TO SEARCH` : 'TYPE TO SEARCH ALL CARDS'}
              spellCheck={false}
              role="combobox"
              aria-expanded="true"
              aria-autocomplete="list"
              style={{ width: '100%' }}
            />

            <div
              ref={listRef}
              role="listbox"
              aria-label="Commander candidates"
              style={{
                marginTop: 6,
                border: '1px solid var(--rule-2)',
                background: 'var(--bg-2, rgba(0,0,0,0.2))',
                maxHeight: 260,
                overflowY: 'auto',
              }}
            >
              {visible.length === 0 && (
                <div className="t-xs muted" style={{ padding: '12px 10px', textAlign: 'center' }}>
                  NO CARDS MATCH "{query.trim().toUpperCase()}"
                </div>
              )}
              {!q && detected.length > 0 && (
                <div className="t-xs muted-2" style={{ padding: '5px 10px', letterSpacing: '0.1em', borderBottom: '1px solid var(--rule)' }}>
                  DETECTED — LEGENDARY CREATURES
                </div>
              )}
              {visible.map((opt, i) => {
                const isSel = opt.value === selected
                const isHl = i === highlight
                return (
                  <div
                    key={opt.value}
                    role="option"
                    aria-selected={isSel}
                    data-hl={isHl ? '1' : '0'}
                    onMouseEnter={() => setHighlight(i)}
                    onMouseDown={(e) => { e.preventDefault(); pick(opt) }}
                    style={{
                      display: 'flex', alignItems: 'center', gap: 8,
                      padding: '6px 10px', cursor: 'pointer',
                      borderBottom: '1px solid var(--rule)',
                      background: isHl ? 'var(--ink)' : (isSel ? 'var(--bg, rgba(255,255,255,0.04))' : 'transparent'),
                      color: isHl ? 'var(--inv-ink)' : 'var(--ink)',
                    }}
                  >
                    <img
                      src={cardArtUrl(opt.value)}
                      alt=""
                      style={{ width: 26, height: 20, objectFit: 'cover', flexShrink: 0, border: '1px solid var(--rule-2)', filter: 'saturate(0.6)' }}
                      onError={e => { e.target.style.visibility = 'hidden' }}
                    />
                    <span style={{ fontSize: 12, fontWeight: 700, letterSpacing: '0.03em', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {opt.label}
                    </span>
                    {opt.type_line && (
                      <span style={{ fontSize: 9, opacity: 0.7, whiteSpace: 'nowrap' }}>{opt.type_line}</span>
                    )}
                  </div>
                )
              })}
            </div>
          </div>
        </div>

        <div className="export-modal__actions">
          <button
            type="button"
            className="export-modal__btn export-modal__btn--solid"
            onClick={confirm}
            disabled={!resolved || saving}
          >
            {saving ? 'PINNING + ANALYZING…' : 'SET COMMANDER + ANALYZE'}
            <span className="arr">↗</span>
          </button>
          <button
            type="button"
            className="export-modal__btn export-modal__btn--ghost"
            onClick={onCancel}
            disabled={saving}
          >
            NOT NOW
          </button>
        </div>
      </div>
    </div>
  )
}

// commanderNeedsResolution — the gate predicate. A deck needs the picker
// when its commander is genuinely unset, unresolved, or ambiguous:
//   - unset:      no commander_card at all
//   - unresolved: commander_card set but no deck card matches it (set codes,
//                 punctuation, //Commander headers all land here)
// A deck whose commander_card resolves to a real card is left alone — the
// dropdown is NOT forced on already-correct decks. Multiple legendary
// creatures with a resolved commander is a valid partners/background build,
// not ambiguity, so it also passes through untouched.
export function commanderNeedsResolution(deck) {
  if (!deck) return false
  const cards = deck.cards || []
  if (cards.length === 0) return false // nothing to pick from yet
  const cmdr = String(deck.commander_card || '').replace(/^COMMANDER:\s*/i, '').trim()
  if (!cmdr) return true // unset
  const norm = s => String(s || '')
    .replace(/^COMMANDER:\s*/i, '')
    .replace(/\s*\([^)]*\)\s*[\dA-Za-z-]*$/, '')
    .trim()
    .toLowerCase()
  const target = norm(cmdr)
  const found = cards.some(c => norm(c.name) === target)
  return !found // set but unresolved
}
