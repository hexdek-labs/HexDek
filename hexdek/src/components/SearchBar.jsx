import { useState, useEffect, useRef, useCallback, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, cardArtUrl } from '../services/api'
import { useT } from '../i18n'

const KIND_GLYPH = {
  deck: '▣',
  commander: '◆',
  owner: '◉',
  card: '▢',
}

const SECTION_ORDER = ['deck', 'commander', 'owner', 'card']
const SECTION_LABEL_KEY = {
  deck: 'search.section.deck',
  commander: 'search.section.commander',
  owner: 'search.section.owner',
  card: 'search.section.card',
}

function flattenResults(payload) {
  if (!payload) return []
  if (Array.isArray(payload.top) && payload.top.length) return payload.top
  const r = payload.results || {}
  return [
    ...(r.owners || []),
    ...(r.commanders || []),
    ...(r.decks || []),
    ...(r.cards || []),
  ]
}

function resolveHref(item) {
  if (!item) return null
  if (item.kind === 'deck' && item.owner && item.id) {
    return `/decks/${encodeURIComponent(item.owner)}/${encodeURIComponent(item.id)}`
  }
  if (item.kind === 'owner') {
    return `/decks?owner=${encodeURIComponent(item.label)}`
  }
  if (item.kind === 'card') {
    return `/cards/${encodeURIComponent(item.label)}`
  }
  if (item.kind === 'commander') {
    return `/decks?tab=all&q=${encodeURIComponent(item.label)}`
  }
  return null
}

export default function SearchBar() {
  const [query, setQuery] = useState('')
  const [items, setItems] = useState([])
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(0)
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()
  const t = useT()
  const inputRef = useRef(null)
  const reqIdRef = useRef(0)

  useEffect(() => {
    const trimmed = query.trim()
    if (trimmed.length < 2) {
      setItems([])
      setLoading(false)
      return
    }
    setLoading(true)
    const reqId = ++reqIdRef.current
    const handle = setTimeout(async () => {
      try {
        const data = await api.search(trimmed, 6)
        if (reqId !== reqIdRef.current) return
        setItems(flattenResults(data))
        setActive(0)
      } catch {
        if (reqId !== reqIdRef.current) return
        setItems([])
      } finally {
        if (reqId === reqIdRef.current) setLoading(false)
      }
    }, 300)
    return () => clearTimeout(handle)
  }, [query])

  useEffect(() => {
    const onKey = (e) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        setOpen(o => !o)
        return
      }
      if (e.key === 'Escape' && open) {
        e.preventDefault()
        setOpen(false)
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open])

  useEffect(() => {
    if (open) {
      requestAnimationFrame(() => inputRef.current?.focus())
    } else {
      setQuery('')
      setItems([])
      setActive(0)
    }
  }, [open])

  const sections = useMemo(() => {
    const grouped = {}
    for (const it of items) {
      const k = it.kind || 'other'
      if (!grouped[k]) grouped[k] = []
      grouped[k].push(it)
    }
    return SECTION_ORDER
      .filter(k => grouped[k] && grouped[k].length)
      .map(k => ({ kind: k, labelKey: SECTION_LABEL_KEY[k], items: grouped[k] }))
  }, [items])

  const flatForKeyboard = useMemo(
    () => sections.flatMap(s => s.items),
    [sections],
  )

  const choose = useCallback((item) => {
    if (!item) return
    const href = resolveHref(item)
    setOpen(false)
    if (href) navigate(href)
  }, [navigate])

  const onKeyDown = (e) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setActive(i => Math.min(i + 1, Math.max(flatForKeyboard.length - 1, 0)))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setActive(i => Math.max(i - 1, 0))
    } else if (e.key === 'Enter') {
      e.preventDefault()
      if (flatForKeyboard[active]) choose(flatForKeyboard[active])
    } else if (e.key === 'Escape') {
      e.preventDefault()
      setOpen(false)
    }
  }

  let runningIndex = -1

  return (
    <>
      <button
        type="button"
        className="searchbar-trigger"
        aria-label={t('search.open')}
        onClick={() => setOpen(true)}
      >
        <span aria-hidden>⌖</span>
        <span className="searchbar-trigger__label">{t('search.label')}</span>
        <span className="searchbar-trigger__hint">⌘K</span>
      </button>

      {open && (
        <div className="searchbar-overlay" onMouseDown={() => setOpen(false)}>
          <div className="searchbar-overlay__panel" onMouseDown={e => e.stopPropagation()}>
            <div className="searchbar-overlay__inputrow">
              <span className="searchbar-overlay__icon" aria-hidden>⌖</span>
              <input
                ref={inputRef}
                type="text"
                value={query}
                onChange={e => setQuery(e.target.value)}
                onKeyDown={onKeyDown}
                placeholder={t('search.placeholder')}
                aria-label={t('search.aria')}
                spellCheck={false}
                autoComplete="off"
                className="searchbar-overlay__input"
              />
              <span className="searchbar-overlay__close" onClick={() => setOpen(false)}>ESC</span>
            </div>
            <div className="searchbar-overlay__results">
              {query.trim().length < 2 ? (
                <div className="searchbar-overlay__note">{t('search.min_chars')}</div>
              ) : loading && items.length === 0 ? (
                <div className="searchbar-overlay__note">{t('search.scanning')}</div>
              ) : sections.length === 0 ? (
                <div className="searchbar-overlay__note">{t('search.no_results')}</div>
              ) : (
                sections.map(section => (
                  <div key={section.kind} className="searchbar-overlay__section">
                    <div className="searchbar-overlay__section-hd">
                      {t(section.labelKey)} <span className="searchbar-overlay__count">{section.items.length}</span>
                    </div>
                    {section.items.map((item) => {
                      runningIndex += 1
                      const idx = runningIndex
                      const isCard = item.kind === 'card'
                      const art = isCard ? cardArtUrl(item.label) : null
                      return (
                        <button
                          key={`${item.kind}-${item.label}-${item.owner || ''}-${item.id || ''}-${idx}`}
                          type="button"
                          onMouseEnter={() => setActive(idx)}
                          onClick={() => choose(item)}
                          className={`searchbar-overlay__row ${idx === active ? 'is-active' : ''}`}
                        >
                          {isCard ? (
                            <span
                              className="searchbar-overlay__art"
                              style={art ? { backgroundImage: `url(${art})` } : undefined}
                              aria-hidden
                            />
                          ) : (
                            <span className="searchbar-overlay__glyph" aria-hidden>
                              {KIND_GLYPH[item.kind] || '·'}
                            </span>
                          )}
                          <span className="searchbar-overlay__label">{item.label}</span>
                          {item.sub && <span className="searchbar-overlay__sub">{item.sub}</span>}
                        </button>
                      )
                    })}
                  </div>
                ))
              )}
            </div>
            <div className="searchbar-overlay__ft">
              <span>↑↓ NAVIGATE</span>
              <span>↵ OPEN</span>
              <span>ESC CLOSE</span>
            </div>
          </div>
        </div>
      )}
    </>
  )
}
