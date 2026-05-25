import { useState, useEffect, useMemo, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../services/api'
import { useAuth } from '../context/AuthContext'
import { toast } from './Toast'
import { trackEvent } from '../hooks/useAnalytics'
import AuthPrompt from './AuthPrompt'
import ContextBox from './ContextBox'
import TagInput from './TagInput'
import { BASIC_LANDS, inferCommander, parseDeckLines } from '../lib/deckParser'
import './ImportModal.css'

// ─── Constants ──────────────────────────────────────────────────
const MODES = [
  { id: 'paste', label: 'PASTE', sub: 'Paste a decklist from clipboard' },
  { id: 'moxfield', label: 'MOXFIELD', sub: 'Import from Moxfield URL' },
  { id: 'archidekt', label: 'ARCHIDEKT', sub: 'Import from Archidekt URL' },
  { id: 'file', label: 'FILE', sub: 'Upload .txt / .dec / .mwDeck' },
]

const ACCEPTED_EXTENSIONS = ['.txt', '.dec', '.mwDeck']
const MOXFIELD_REGEX = /^https?:\/\/(www\.)?moxfield\.com\/decks\/[\w-]+/i
// Archidekt URLs always carry a numeric deck ID; the optional trailing
// /slug is purely cosmetic and not required for the API call.
const ARCHIDEKT_REGEX = /^https?:\/\/(www\.)?archidekt\.com\/decks\/\d+/i

// Debounce helper
function useDebouncedValue(value, delay) {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const t = setTimeout(() => setDebounced(value), delay)
    return () => clearTimeout(t)
  }, [value, delay])
  return debounced
}

// Parser helpers (inferCommander / parseDeckLines / BASIC_LANDS) moved
// to lib/deckParser.js so the inline QUICK PASTE box on /decks shares
// the exact same parsing semantics and the format coverage gets
// unit-tested without spinning up React.

function stripCommanderLine(text) {
  return text
    .split('\n')
    .filter(line => !/^\s*commander\s*:/i.test(line))
    .join('\n')
}

// ─── Validation Hook ────────────────────────────────────────────
function useCardValidation(parsedCards) {
  const [validationResults, setValidationResults] = useState({})
  const [validating, setValidating] = useState(false)
  const abortRef = useRef(null)

  const debouncedCards = useDebouncedValue(parsedCards, 600)

  useEffect(() => {
    if (!debouncedCards || debouncedCards.length === 0) {
      setValidationResults({})
      setValidating(false)
      return
    }

    // Abort previous validation run
    if (abortRef.current) abortRef.current.abort()
    const controller = new AbortController()
    abortRef.current = controller

    const validate = async () => {
      setValidating(true)
      const results = {}

      // Only validate unique card names (case-insensitive)
      const seen = new Set()
      const toValidate = []
      for (const card of debouncedCards) {
        const key = card.name.toLowerCase()
        if (seen.has(key)) continue
        seen.add(key)
        // Skip basic lands — always valid
        if (BASIC_LANDS.has(key)) {
          results[key] = { valid: true, name: card.name }
          continue
        }
        toValidate.push(card)
      }

      // Batch validate — make parallel requests but limit concurrency
      const BATCH_SIZE = 5
      for (let i = 0; i < toValidate.length; i += BATCH_SIZE) {
        if (controller.signal.aborted) return
        const batch = toValidate.slice(i, i + BATCH_SIZE)
        const promises = batch.map(async (card) => {
          try {
            const res = await fetch(
              `${import.meta.env.VITE_API_URL ?? ''}/api/cards/search?q=${encodeURIComponent(card.name)}&limit=3`,
              { signal: controller.signal }
            )
            if (!res.ok) return { key: card.name.toLowerCase(), valid: false, suggestion: null }
            const data = await res.json()
            const hits = data.results || []
            // Exact match (case-insensitive)
            const exact = hits.find(h => h.name.toLowerCase() === card.name.toLowerCase())
            if (exact) return { key: card.name.toLowerCase(), valid: true, name: exact.name }
            // Close match — suggest
            if (hits.length > 0) return { key: card.name.toLowerCase(), valid: false, suggestion: hits[0].name }
            return { key: card.name.toLowerCase(), valid: false, suggestion: null }
          } catch (e) {
            if (e.name === 'AbortError') return null
            return { key: card.name.toLowerCase(), valid: false, suggestion: null }
          }
        })
        const batchResults = await Promise.all(promises)
        for (const r of batchResults) {
          if (r) results[r.key] = r
        }
      }

      if (!controller.signal.aborted) {
        setValidationResults(results)
        setValidating(false)
      }
    }

    validate()
    return () => controller.abort()
  }, [debouncedCards])

  return { validationResults, validating }
}

// ─── Progress Indicator ─────────────────────────────────────────
function AnalysisProgress() {
  const [dots, setDots] = useState(0)
  useEffect(() => {
    const t = setInterval(() => setDots(d => (d + 1) % 4), 400)
    return () => clearInterval(t)
  }, [])

  return (
    <div className="import-modal__progress">
      <div className="import-modal__progress-bar">
        <div className="import-modal__progress-fill" />
      </div>
      <div className="import-modal__progress-label">
        ANALYZING DECK{'.'.repeat(dots)}
      </div>
      <div className="import-modal__progress-sub">
        FREYA IS EVALUATING SYNERGY, MANA CURVE, AND THREAT DENSITY
      </div>
    </div>
  )
}

// ─── Main Component ─────────────────────────────────────────────
export default function ImportModal({ onClose, onImported }) {
  const navigate = useNavigate()
  const { user } = useAuth()

  // If not authenticated, show auth prompt over a teaser landing instead
  // of a centered modal floating in a black void. Gives anon visitors who
  // hit /import directly some context about what they'd unlock by signing in.
  if (!user) {
    return (
      <div style={{ padding: '40px 20px', maxWidth: 920, margin: '0 auto' }}>
        <div className="t-xs muted" style={{ letterSpacing: '0.08em', marginBottom: 6 }}>
          IMPORT / / DECK
        </div>
        <h1 style={{
          fontSize: 48, fontWeight: 800, letterSpacing: '0.02em',
          margin: '0 0 18px', lineHeight: 1.05,
        }}>PASTE.<br />ANALYZE.<br />RANK.</h1>
        <div className="t-md muted" style={{ maxWidth: 540, lineHeight: 1.5, marginBottom: 28 }}>
          Drop a Moxfield URL, paste a text list, or upload a .txt — HexDek's Freya engine returns an archetype, win lines, mana base grade, cut suggestions, and a bracket estimate in seconds. Then run a gauntlet to get a real HexELO rating.
        </div>
        <div style={{
          display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
          gap: 12, marginBottom: 32,
        }}>
          {[
            ['FREYA ANALYSIS', 'Archetype + roles + combos + cuts on every import.'],
            ['HEXELO RATING', 'Run a 500-game gauntlet, see how your deck stacks up.'],
            ['DECK ARCHIVE', 'Versioned. Iterate, save, compare across builds.'],
          ].map(([h, b]) => (
            <div key={h} style={{
              padding: '14px 16px', background: 'var(--panel)',
              border: '1px solid var(--rule-2)',
            }}>
              <div style={{ fontSize: 11, fontWeight: 800, letterSpacing: '0.08em', marginBottom: 6 }}>{h}</div>
              <div className="t-xs muted" style={{ lineHeight: 1.4 }}>{b}</div>
            </div>
          ))}
        </div>
        <AuthPrompt onClose={onClose} action="import deck" inline />
      </div>
    )
  }

  return <ImportModalInner onClose={onClose} onImported={onImported} navigate={navigate} user={user} />
}

function ImportModalInner({ onClose, onImported, navigate, user }) {
  const [mode, setMode] = useState('paste')
  const [name, setName] = useState('')
  const [owner, setOwner] = useState(() => {
    return (
      localStorage.getItem('hexdek_owner')
      || user.displayName?.toLowerCase()
      || user.email?.split('@')[0]?.split('.')[0]
      || ''
    )
  })
  const [commander, setCommander] = useState('')
  const [tags, setTags] = useState([])
  const [deckList, setDeckList] = useState('')
  const [moxfieldUrl, setMoxfieldUrl] = useState('')
  const [archidektUrl, setArchidektUrl] = useState('')
  const [fileName, setFileName] = useState('')
  const [phase, setPhase] = useState('input') // input | validating | analyzing | done
  const [error, setError] = useState(null)
  const fileInputRef = useRef(null)

  // Parse deck lines from current input
  const parsedCards = useMemo(() => parseDeckLines(deckList), [deckList])
  const cardCount = parsedCards.filter(c => !c.isCommander).length
  const detectedCommander = useMemo(() => inferCommander(deckList), [deckList])

  // Auto-detect URL in paste — moxfield OR archidekt. Order matters
  // only insofar as both regexes match disjoint domains, so the
  // sibling check is a clean either/or.
  useEffect(() => {
    if (mode !== 'paste') return
    const t = deckList.trim()
    if (!t) return
    if (MOXFIELD_REGEX.test(t)) {
      setMoxfieldUrl(t)
      setDeckList('')
      setMode('moxfield')
    } else if (ARCHIDEKT_REGEX.test(t)) {
      setArchidektUrl(t)
      setDeckList('')
      setMode('archidekt')
    }
  }, [deckList, mode])

  // Real-time card validation
  const { validationResults, validating } = useCardValidation(
    mode === 'paste' || mode === 'file' ? parsedCards : []
  )

  // Compute validation errors
  const validationErrors = useMemo(() => {
    const errors = []
    for (const card of parsedCards) {
      const key = card.name.toLowerCase()
      const result = validationResults[key]
      if (result && !result.valid) {
        errors.push({
          name: card.name,
          suggestion: result.suggestion,
          type: 'unresolved',
        })
      }
      // Commander format: check for illegal multiples (non-basic, qty > 1)
      if (!BASIC_LANDS.has(key) && card.quantity > 1) {
        errors.push({
          name: card.name,
          type: 'illegal_count',
          message: `${card.quantity}x — only 1 copy allowed in Commander`,
        })
      }
    }
    // Missing commander check
    const hasCommander = commander || detectedCommander || parsedCards.some(c => c.isCommander)
    if (parsedCards.length > 0 && !hasCommander) {
      errors.push({ type: 'missing_commander', message: 'No commander designated' })
    }
    return errors
  }, [parsedCards, validationResults, commander, detectedCommander])

  // Auto-fill commander from paste
  useEffect(() => {
    if (!commander && detectedCommander) {
      setCommander(detectedCommander)
    }
  }, [detectedCommander, commander])

  // ─── Name-collision detection ───────────────────────────────────
  // Watches the (owner, name, commander) trio. When the owner already
  // has a deck with the same effective name AND commander, surface a
  // friendly nudge to rename — non-blocking, since the user might want
  // to overwrite/version-up intentionally. Debounced to avoid hammering
  // /api/decks on every keystroke.
  const [collisionDecks, setCollisionDecks] = useState([])
  useEffect(() => {
    const o = owner.trim().toLowerCase()
    if (!o) { setCollisionDecks([]); return }
    let cancelled = false
    const t = setTimeout(() => {
      api.getDecks({ owner: o }).then(res => {
        if (cancelled) return
        const list = Array.isArray(res) ? res : (res?.decks || [])
        const cmdr = (commander || detectedCommander || '').trim().toUpperCase()
        const effectiveName = (name.trim() || cmdr).toUpperCase()
        const matches = list.filter(d => {
          const dCmdr = (d.commander_card || d.commander || '').toUpperCase()
          const dName = (d.name || '').toUpperCase()
          return dCmdr === cmdr && dName === effectiveName
        })
        setCollisionDecks(matches)
      }).catch(() => { if (!cancelled) setCollisionDecks([]) })
    }, 400)
    return () => { cancelled = true; clearTimeout(t) }
  }, [owner, name, commander, detectedCommander])

  // ─── File Upload Handler ──────────────────────────────────────
  const handleFileSelect = (e) => {
    const file = e.target.files?.[0]
    if (!file) return
    setFileName(file.name)
    const reader = new FileReader()
    reader.onload = (ev) => {
      setDeckList(ev.target.result || '')
    }
    reader.readAsText(file)
  }

  // ─── Submit Handler ───────────────────────────────────────────
  const handleSubmit = async () => {
    setError(null)

    if (mode === 'moxfield') {
      if (!moxfieldUrl.trim() || !MOXFIELD_REGEX.test(moxfieldUrl.trim())) {
        setError('ENTER A VALID MOXFIELD URL (https://www.moxfield.com/decks/...)')
        return
      }
      setPhase('analyzing')
      try {
        const result = await api.importMoxfield({
          url: moxfieldUrl.trim(),
          owner: owner.trim() || 'imported',
          tags,
        })
        trackEvent('import_moxfield', {
          owner: owner.trim() || 'imported',
          cards: result.card_count,
          moxfield_id: result.moxfield_id,
        })
        setPhase('done')
        toast.success(`DECK IMPORTED: ${result.name || 'MOXFIELD DECK'}`)
        onImported?.()
        const target = `/decks/${encodeURIComponent(result.owner)}/${encodeURIComponent(result.id)}`
        setTimeout(() => {
          onClose()
          navigate(target)
        }, 600)
      } catch (err) {
        setPhase('input')
        setError(err.message || 'MOXFIELD IMPORT FAILED')
      }
      return
    }

    if (mode === 'archidekt') {
      if (!archidektUrl.trim() || !ARCHIDEKT_REGEX.test(archidektUrl.trim())) {
        setError('ENTER A VALID ARCHIDEKT URL (https://archidekt.com/decks/...)')
        return
      }
      setPhase('analyzing')
      try {
        const result = await api.importArchidekt({
          url: archidektUrl.trim(),
          owner: owner.trim() || 'imported',
          tags,
        })
        trackEvent('import_archidekt', {
          owner: owner.trim() || 'imported',
          cards: result.card_count,
          archidekt_id: result.archidekt_id,
        })
        setPhase('done')
        toast.success(`DECK IMPORTED: ${result.name || 'ARCHIDEKT DECK'}`)
        onImported?.()
        const target = `/decks/${encodeURIComponent(result.owner)}/${encodeURIComponent(result.id)}`
        setTimeout(() => {
          onClose()
          navigate(target)
        }, 600)
      } catch (err) {
        setPhase('input')
        setError(err.message || 'ARCHIDEKT IMPORT FAILED')
      }
      return
    }

    // Paste or file mode
    if (!deckList.trim()) {
      setError('DECK LIST REQUIRED — PASTE OR UPLOAD A FILE')
      return
    }
    if (cardCount < 5) {
      setError(`ONLY ${cardCount} CARD${cardCount === 1 ? '' : 'S'} DETECTED — CHECK YOUR INPUT`)
      return
    }

    // Normalize commander line
    let payloadList = deckList
    const cmdrTrimmed = (commander || detectedCommander || '').trim()
    if (cmdrTrimmed) {
      payloadList = `COMMANDER: ${cmdrTrimmed}\n${stripCommanderLine(deckList)}`
    }

    setPhase('analyzing')
    try {
      const result = await api.importDeckFull({
        name: name.trim() || 'Imported Deck',
        owner: owner.trim() || 'imported',
        deckList: payloadList,
        tags,
      })
      trackEvent('import_deck_full', {
        name: name.trim() || 'Imported Deck',
        owner: owner.trim() || 'imported',
        cards: cardCount,
        source: mode,
      })

      const newID = result?.id

      setPhase('done')
      toast.success(`DECK IMPORTED: ${result?.name || name || 'NEW DECK'}`)
      onImported?.()
      const newOwner = result?.owner || owner.trim() || 'imported'
      const target = newID
        ? `/decks/${encodeURIComponent(newOwner)}/${encodeURIComponent(newID)}`
        : '/decks'
      setTimeout(() => {
        onClose()
        navigate(target)
      }, 600)
    } catch (err) {
      setPhase('input')
      setError(err.message || 'IMPORT FAILED')
    }
  }

  // ─── Keyboard shortcuts ───────────────────────────────────────
  useEffect(() => {
    const onKey = (e) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [onClose])

  // ─── Render ───────────────────────────────────────────────────
  const isSubmittable = mode === 'moxfield'
    ? MOXFIELD_REGEX.test(moxfieldUrl.trim())
    : mode === 'archidekt'
      ? ARCHIDEKT_REGEX.test(archidektUrl.trim())
      : cardCount >= 5

  return (
    <div className="import-modal" onMouseDown={onClose}>
      <div className="import-modal__panel" onMouseDown={e => e.stopPropagation()}>
        {/* Header */}
        <div className="import-modal__hd">
          <span>DECK IMPORT / / UNIFIED</span>
          <span className="import-modal__close" onClick={onClose}>ESC</span>
        </div>

        {/* Mode tabs */}
        <div className="import-modal__modes">
          {MODES.map(m => (
            <button
              key={m.id}
              type="button"
              className={`import-modal__mode ${mode === m.id ? 'is-on' : ''}`}
              onClick={() => { setMode(m.id); setError(null) }}
              disabled={phase !== 'input'}
            >
              <span className="import-modal__mode-label">{m.label}</span>
              <span className="import-modal__mode-sub">{m.sub}</span>
            </button>
          ))}
        </div>

        {/* Analyzing state */}
        {phase === 'analyzing' && <AnalysisProgress />}

        {/* Done state */}
        {phase === 'done' && (
          <div className="import-modal__done">
            <div className="import-modal__done-icon">&#x2713;</div>
            <div className="import-modal__done-label">DECK IMPORTED SUCCESSFULLY</div>
            <div className="import-modal__done-sub">REDIRECTING TO DECK ARCHIVE...</div>
          </div>
        )}

        {/* Input state */}
        {phase === 'input' && (
          <>
            {/* Metadata row */}
            <div className="import-modal__meta">
              <div className="import-modal__field">
                <label className="import-modal__label">DECK NAME</label>
                <input
                  type="text"
                  className="import-modal__input"
                  value={name}
                  onChange={e => setName(e.target.value)}
                  placeholder="TINYBONES, THE PICKPOCKET"
                  style={collisionDecks.length > 0 ? { borderColor: 'var(--warn)' } : undefined}
                />
                {collisionDecks.length > 0 && (
                  <div style={{
                    marginTop: 6,
                    padding: '6px 8px',
                    border: '1px solid var(--warn)',
                    borderLeft: '3px solid var(--warn)',
                    background: 'rgba(255, 200, 0, 0.06)',
                    fontSize: 10,
                    lineHeight: 1.4,
                  }}>
                    <strong style={{ color: 'var(--warn)' }}>HEY — WE NOTICED</strong>
                    <div style={{ marginTop: 3, color: 'var(--ink-2)' }}>
                      You already have {collisionDecks.length === 1 ? 'a deck' : `${collisionDecks.length} decks`} with this exact name + commander. If this is a different build (storm vs. stax, etc.), rename it now to keep them distinct — otherwise it'll show up as a version variant.
                    </div>
                  </div>
                )}
              </div>
              <div className="import-modal__field">
                <label className="import-modal__label">OWNER</label>
                <input
                  type="text"
                  className="import-modal__input"
                  value={owner}
                  onChange={e => setOwner(e.target.value)}
                  placeholder="IMPORTED"
                />
              </div>
              <div className="import-modal__field">
                <label className="import-modal__label">COMMANDER</label>
                <input
                  type="text"
                  className="import-modal__input"
                  value={commander}
                  onChange={e => setCommander(e.target.value)}
                  placeholder="AUTO-DETECTED FROM LIST"
                />
              </div>
              <div className="import-modal__field" style={{ gridColumn: '1 / -1' }}>
                <label className="import-modal__label">TAGS</label>
                <TagInput
                  value={tags}
                  onChange={setTags}
                  owner={owner.trim().toLowerCase()}
                  placeholder="ADD TAG — e.g. cedh, budget, brew"
                />
              </div>
            </div>

            {/* Mode-specific content */}
            <div className="import-modal__body">
              {mode === 'paste' && (
                <>
                  <div className="import-modal__body-hd">
                    <span>DECK LIST</span>
                    <span className="t-xs muted">
                      {cardCount > 0 ? `${cardCount} CARDS` : 'AWAITING PASTE'}
                      {validating && ' / VALIDATING...'}
                    </span>
                  </div>
                  <textarea
                    className="import-modal__textarea"
                    value={deckList}
                    onChange={e => setDeckList(e.target.value)}
                    placeholder={"COMMANDER: Tinybones, the Pickpocket\n1 Swamp\n1 Dark Ritual\n1 Sol Ring\n1 Thoughtseize\n...\n\n(Supports MTGO, Arena, or plain card names)"}
                    rows={14}
                    spellCheck={false}
                  />
                </>
              )}

              {mode === 'moxfield' && (
                <>
                  <div className="import-modal__body-hd">
                    <span>MOXFIELD URL</span>
                    <span className="t-xs muted">
                      {MOXFIELD_REGEX.test(moxfieldUrl.trim()) ? 'VALID URL' : 'PASTE URL'}
                    </span>
                  </div>
                  <div className="import-modal__moxfield">
                    <input
                      type="url"
                      className="import-modal__input import-modal__input--lg"
                      value={moxfieldUrl}
                      onChange={e => setMoxfieldUrl(e.target.value)}
                      placeholder="https://www.moxfield.com/decks/abc123..."
                    />
                    <div className="import-modal__moxfield-help">
                      &gt; PASTE A PUBLIC MOXFIELD DECK URL. THE DECK WILL BE FETCHED AND IMPORTED AUTOMATICALLY.
                      <br />&gt; COMMANDER AND CARD LIST ARE EXTRACTED FROM THE MOXFIELD API.
                    </div>
                  </div>
                </>
              )}

              {mode === 'archidekt' && (
                <>
                  <div className="import-modal__body-hd">
                    <span>ARCHIDEKT URL</span>
                    <span className="t-xs muted">
                      {ARCHIDEKT_REGEX.test(archidektUrl.trim()) ? 'VALID URL' : 'PASTE URL'}
                    </span>
                  </div>
                  <div className="import-modal__moxfield">
                    <input
                      type="url"
                      className="import-modal__input import-modal__input--lg"
                      value={archidektUrl}
                      onChange={e => setArchidektUrl(e.target.value)}
                      placeholder="https://archidekt.com/decks/123456..."
                    />
                    <div className="import-modal__moxfield-help">
                      &gt; PASTE A PUBLIC ARCHIDEKT DECK URL. THE DECK WILL BE FETCHED AND IMPORTED AUTOMATICALLY.
                      <br />&gt; COMMANDER, MAINBOARD, AND MAYBEBOARD ARE PRESERVED FROM ARCHIDEKT CATEGORIES.
                    </div>
                  </div>
                </>
              )}

              {mode === 'file' && (
                <>
                  <div className="import-modal__body-hd">
                    <span>FILE UPLOAD</span>
                    <span className="t-xs muted">
                      {fileName ? fileName.toUpperCase() : '.TXT / .DEC / .MWDECK'}
                    </span>
                  </div>
                  <div className="import-modal__file-zone">
                    <input
                      ref={fileInputRef}
                      type="file"
                      accept=".txt,.dec,.mwDeck"
                      onChange={handleFileSelect}
                      style={{ display: 'none' }}
                    />
                    {!deckList && (
                      <button
                        type="button"
                        className="import-modal__file-btn"
                        onClick={() => fileInputRef.current?.click()}
                      >
                        <span className="import-modal__file-icon">&#x2191;</span>
                        <span>SELECT FILE</span>
                        <span className="import-modal__file-ext">.txt / .dec / .mwDeck</span>
                      </button>
                    )}
                    {deckList && (
                      <div className="import-modal__file-preview">
                        <div className="import-modal__file-preview-hd">
                          <span>{fileName.toUpperCase()}</span>
                          <span className="t-xs muted">{cardCount} CARDS</span>
                        </div>
                        <pre className="import-modal__file-preview-text">
                          {deckList.split('\n').slice(0, 12).join('\n')}
                          {deckList.split('\n').length > 12 && '\n...'}
                        </pre>
                        <button
                          type="button"
                          className="import-modal__file-change"
                          onClick={() => { setDeckList(''); setFileName(''); fileInputRef.current.value = '' }}
                        >
                          CHANGE FILE
                        </button>
                      </div>
                    )}
                  </div>
                </>
              )}
            </div>

            {/* Validation errors */}
            {validationErrors.length > 0 && (mode === 'paste' || mode === 'file') && (
              <div className="import-modal__validation">
                <div className="import-modal__validation-hd">
                  <span className="led led--warn" />
                  <span>{validationErrors.length} ISSUE{validationErrors.length !== 1 ? 'S' : ''} DETECTED</span>
                </div>
                <div className="import-modal__validation-list">
                  {validationErrors.slice(0, 8).map((err, i) => (
                    <div key={i} className={`import-modal__validation-item import-modal__validation-item--${err.type}`}>
                      {err.type === 'unresolved' && (
                        <>
                          <span className="import-modal__val-name">{err.name}</span>
                          <span className="import-modal__val-msg">
                            {err.suggestion ? `DID YOU MEAN: ${err.suggestion}` : 'NOT FOUND IN CORPUS'}
                          </span>
                        </>
                      )}
                      {err.type === 'illegal_count' && (
                        <>
                          <span className="import-modal__val-name">{err.name}</span>
                          <span className="import-modal__val-msg">{err.message}</span>
                        </>
                      )}
                      {err.type === 'missing_commander' && (
                        <span className="import-modal__val-msg">{err.message}</span>
                      )}
                    </div>
                  ))}
                  {validationErrors.length > 8 && (
                    <div className="import-modal__validation-more">
                      +{validationErrors.length - 8} MORE
                    </div>
                  )}
                </div>
              </div>
            )}

            {/* Error display */}
            {error && (
              <div className="import-modal__error">
                &gt; {error}
              </div>
            )}

            {/* Actions */}
            <ContextBox id="import.modal.submit">
              Imports the deck into your archive and runs Freya analysis automatically — you'll land on the new deck's page when it's ready.
              {mode === 'moxfield' ? ' Moxfield URL must point to a public deck.' : ''}
            </ContextBox>
            <div className="import-modal__actions">
              <button
                type="button"
                className="import-modal__btn import-modal__btn--ghost"
                onClick={onClose}
              >
                CANCEL
              </button>
              <button
                type="button"
                className="import-modal__btn import-modal__btn--solid"
                onClick={handleSubmit}
                disabled={!isSubmittable}
              >
                {mode === 'moxfield' ? 'IMPORT FROM MOXFIELD' : 'IMPORT DECK'}
                <span className="arr">&#x2197;</span>
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
