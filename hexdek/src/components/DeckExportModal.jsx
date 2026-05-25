import { useEffect, useMemo, useState } from 'react'
import { toast } from './Toast'
import { trackEvent } from '../hooks/useAnalytics'
import { useModalKeyboard } from '../hooks/useModalKeyboard'
import { api } from '../services/api'
import { buildDeckExport, exportExtension } from '../lib/deckExport'
import {
  buildStatsCSV,
  buildStatsJSON,
  statsExportFilename,
} from '../lib/deckStatsExport'

const FORMATS = [
  { id: 'moxfield', label: 'MOXFIELD', sub: 'Moxfield bulk-paste — Commander / Deck sections', kind: 'decklist' },
  { id: 'mtgo', label: 'MTGO', sub: 'Magic Online .dec — commander in sideboard', kind: 'decklist' },
  { id: 'arena', label: 'ARENA', sub: 'MTG Arena import format with set codes', kind: 'decklist' },
  { id: 'raw',  label: 'RAW',   sub: 'Card names only, one per line', kind: 'decklist' },
  { id: 'stats-json', label: 'STATS · JSON', sub: 'Freya analysis + gauntlet winrate + recent games', kind: 'stats' },
  { id: 'stats-csv',  label: 'STATS · CSV',  sub: 'Multi-section spreadsheet — overview, weights, gauntlet, ELO, games', kind: 'stats' },
]

function copy(text) {
  if (navigator.clipboard?.writeText) {
    return navigator.clipboard.writeText(text).then(() => true).catch(() => false)
  }
  try {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.select()
    const ok = document.execCommand('copy')
    document.body.removeChild(ta)
    return Promise.resolve(ok)
  } catch {
    return Promise.resolve(false)
  }
}

function download(text, filename, mimeType = 'text/plain') {
  const blob = new Blob([text], { type: mimeType })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

export default function DeckExportModal({ deck, deckId, onClose }) {
  const panelRef = useModalKeyboard({ onClose })
  const [format, setFormat] = useState('moxfield')
  const cards = deck?.cards || []
  const commanderName = deck?.commander_card || deck?.commander || ''
  const isStats = format === 'stats-json' || format === 'stats-csv'

  // Stats bundle is fetched lazily — only when the user picks a stats
  // format — so the decklist-export happy path stays a single render
  // with no network. Once fetched, the bundle is cached for the
  // lifetime of the modal so flipping between JSON / CSV is instant.
  const [statsBundle, setStatsBundle] = useState(null)
  const [statsLoading, setStatsLoading] = useState(false)
  const [statsError, setStatsError] = useState(null)
  useEffect(() => {
    if (!isStats || statsBundle || statsLoading || !deckId) return
    let cancelled = false
    setStatsLoading(true)
    setStatsError(null)
    // Fetch in parallel; any individual endpoint may 404 (no gauntlet
    // run yet, etc.) — we record null for that section rather than
    // failing the whole export.
    Promise.all([
      api.getDeckAnalysis(deckId).catch(() => null),
      api.getGauntlet(deckId).catch(() => null),
      api.getDeckEloHistory(deckId, 20).catch(() => null),
      api.searchGameSummaries({ deck: deckId, limit: 20 }).catch(() => null),
    ]).then(([analysis, gauntlet, eloHistory, gamesRes]) => {
      if (cancelled) return
      const games = Array.isArray(gamesRes) ? gamesRes : (gamesRes?.summaries || gamesRes?.games || null)
      setStatsBundle({ deck: { ...deck, id: deckId }, analysis, gauntlet, eloHistory, games })
      setStatsLoading(false)
    }).catch(() => {
      if (cancelled) return
      setStatsError('FAILED TO FETCH DECK STATS')
      setStatsLoading(false)
    })
    return () => { cancelled = true }
  }, [isStats, statsBundle, statsLoading, deckId, deck])

  const text = useMemo(() => {
    if (format === 'stats-json') return statsBundle ? buildStatsJSON(statsBundle) : ''
    if (format === 'stats-csv')  return statsBundle ? buildStatsCSV(statsBundle)  : ''
    return buildDeckExport(format, cards, commanderName)
  }, [format, cards, commanderName, statsBundle])

  // Card-line count excludes blank separators and the section header
  // tokens (Sideboard / Commander / Commanders / Deck) that the format
  // builders emit. Anything else is a `qty name` row. For stats
  // exports it's just "line count" of the rendered payload.
  const lineCount = text
    ? text.split('\n').filter(l => l && !/^(Sideboard|Commander|Commanders|Deck)$/.test(l)).length
    : 0
  const hasArenaData = cards.some(c => c.set || c.collector_number || c.cn)
  const baseFilename = (deckId || 'deck').replace(/[^a-z0-9_-]/gi, '_')
  const statsKind = format === 'stats-csv' ? 'csv' : 'json'
  const statsMime = format === 'stats-csv' ? 'text/csv' : 'application/json'
  const ext = isStats ? `_stats.${statsKind}` : exportExtension(format)
  const downloadFilename = isStats ? statsExportFilename(deckId, statsKind) : `${baseFilename}${ext}`

  const onCopy = async () => {
    if (!text) return
    const ok = await copy(text)
    trackEvent('deck_export_copy', { format, lines: lineCount })
    if (ok) toast.success(isStats ? `COPIED STATS (${statsKind.toUpperCase()})` : `COPIED ${format.toUpperCase()} (${lineCount} CARDS)`)
    else    toast.error('COPY FAILED — TRY DOWNLOAD')
  }
  const onDownload = () => {
    if (!text) return
    download(text, downloadFilename, isStats ? statsMime : 'text/plain')
    trackEvent('deck_export_download', { format, lines: lineCount })
    toast.success(`DOWNLOADED ${downloadFilename}`)
  }

  return (
    <div className="export-modal" onMouseDown={onClose}>
      <div
        ref={panelRef}
        className="export-modal__panel"
        onMouseDown={e => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label={`Export deck ${deckId || ''}`}
      >
        <div className="export-modal__hd">
          <span>EXPORT DECK / / {(deckId || '?').toUpperCase()}</span>
          <button
            type="button"
            className="export-modal__close"
            onClick={onClose}
            aria-label="Close export dialog"
            style={{ background: 'transparent', border: 'none', color: 'inherit', font: 'inherit', cursor: 'pointer', padding: 0 }}
          >ESC</button>
        </div>

        <div className="export-modal__formats">
          {FORMATS.map(f => (
            <button
              key={f.id}
              type="button"
              className={`export-modal__fmt ${format === f.id ? 'is-on' : ''}`}
              onClick={() => setFormat(f.id)}
            >
              <span className="export-modal__fmt-label">{f.label}</span>
              <span className="export-modal__fmt-sub">{f.sub}</span>
            </button>
          ))}
        </div>

        {format === 'arena' && !hasArenaData && (
          <div className="export-modal__warn">
            &gt; SET / COLLECTOR DATA NOT IN DECK — ARENA OUTPUT FALLS BACK TO PLAIN NAMES.
            <br />&gt; ARENA WILL STILL ACCEPT IT BUT MAY PICK A DEFAULT PRINTING.
          </div>
        )}

        {isStats && statsError && (
          <div className="export-modal__warn">&gt; {statsError}</div>
        )}

        <div className="export-modal__preview-wrap">
          <div className="export-modal__preview-hd">
            <span>{format.toUpperCase()} PREVIEW</span>
            <span className="t-xs muted">
              {isStats
                ? (statsLoading ? 'FETCHING STATS…' : `${lineCount} LINES`)
                : `${lineCount} CARDS`}
            </span>
          </div>
          <pre className="export-modal__preview" aria-label={isStats ? 'Stats preview' : 'Decklist preview'}>
            {isStats && statsLoading && !statsBundle
              ? '> FETCHING ANALYSIS + GAUNTLET + RECENT GAMES…'
              : (text || '— EMPTY DECK —')}
          </pre>
        </div>

        <div className="export-modal__actions">
          <button
            type="button"
            className="export-modal__btn export-modal__btn--solid"
            onClick={onCopy}
            disabled={!text || (isStats && statsLoading)}
          >
            COPY {isStats ? `STATS ${statsKind.toUpperCase()}` : format.toUpperCase()}<span className="arr">⎘</span>
          </button>
          <button
            type="button"
            className="export-modal__btn"
            onClick={onDownload}
            disabled={!text || (isStats && statsLoading)}
          >
            DOWNLOAD {isStats ? `.${statsKind}` : ext}<span className="arr">↓</span>
          </button>
          <button type="button" className="export-modal__btn export-modal__btn--ghost" onClick={onClose}>
            CLOSE
          </button>
        </div>
      </div>
    </div>
  )
}
