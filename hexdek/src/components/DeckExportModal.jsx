import { useEffect, useMemo, useState } from 'react'
import { toast } from './Toast'
import { trackEvent } from '../hooks/useAnalytics'
import { useModalKeyboard } from '../hooks/useModalKeyboard'
import { api } from '../services/api'
import { useT } from '../i18n'
import { buildDeckExport, exportExtension } from '../lib/deckExport'
import {
  buildStatsCSV,
  buildStatsJSON,
  statsExportFilename,
} from '../lib/deckStatsExport'

const FORMATS = [
  { id: 'moxfield',   label: 'MOXFIELD', subKey: 'exp.sub_moxfield',    kind: 'decklist' },
  { id: 'mtgo',       label: 'MTGO',     subKey: 'exp.sub_mtgo',        kind: 'decklist' },
  { id: 'arena',      label: 'ARENA',    subKey: 'exp.sub_arena',       kind: 'decklist' },
  { id: 'raw',  labelKey: 'exp.fmt_raw', subKey: 'exp.sub_raw',         kind: 'decklist' },
  { id: 'stats-json', labelKey: 'exp.fmt_stats_json', subKey: 'exp.sub_stats_json', kind: 'stats' },
  { id: 'stats-csv',  labelKey: 'exp.fmt_stats_csv',  subKey: 'exp.sub_stats_csv',  kind: 'stats' },
]

// MOXFIELD / MTGO / ARENA stay as plain labels: they are product names and
// read the same in all 25. Only RAW and the two STATS composites carry a
// labelKey, so the table holds catalog PATHS and the render site does the
// wording -- a word stored here would freeze in whatever locale was active
// when the module first evaluated.
const fmtLabel = (f, t) => (f.labelKey ? t(f.labelKey) : f.label)

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
  const t = useT()
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
      setStatsError(true)
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
  // The label of the SELECTED format, resolved once. The header and the copy
  // toast used to interpolate `format.toUpperCase()` -- the raw id, which read
  // "STATS-JSON" and would have stayed English in all 25. Interpolating the
  // resolved label fixes both at once.
  const selected = FORMATS.find(f => f.id === format)
  const formatLabel = selected ? fmtLabel(selected, t) : format.toUpperCase()
  const copyWhat = isStats ? t('exp.stats_what', { kind: statsKind.toUpperCase() }) : formatLabel
  // dka.cards / dka.cards_one is an EXISTING complete plural pair, so the card
  // count is not forked here -- and it keeps the wording identical to
  // DeckArchive, the screen this modal opens from.
  const cardCount = t(lineCount === 1 ? 'dka.cards_one' : 'dka.cards', { count: lineCount })

  const onCopy = async () => {
    if (!text) return
    const ok = await copy(text)
    trackEvent('deck_export_copy', { format, lines: lineCount })
    if (ok) {
      toast.success(isStats
        ? t('exp.toast_copied_stats', { kind: statsKind.toUpperCase() })
        : t('exp.toast_copied', { what: formatLabel, cards: cardCount }))
    } else {
      toast.error(t('exp.toast_copy_failed'))
    }
  }
  const onDownload = () => {
    if (!text) return
    download(text, downloadFilename, isStats ? statsMime : 'text/plain')
    trackEvent('deck_export_download', { format, lines: lineCount })
    toast.success(t('exp.toast_downloaded', { filename: downloadFilename }))
  }

  return (
    <div className="export-modal" onMouseDown={onClose}>
      <div
        ref={panelRef}
        className="export-modal__panel"
        onMouseDown={e => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label={t('exp.aria_dialog', { id: deckId || '' })}
      >
        <div className="export-modal__hd">
          <span>{t('exp.title', { id: (deckId || '?').toUpperCase() })}</span>
          <button
            type="button"
            className="export-modal__close"
            onClick={onClose}
            aria-label={t('exp.aria_close')}
            style={{ background: 'transparent', border: 'none', color: 'inherit', font: 'inherit', cursor: 'pointer', padding: 0 }}
          >{t('dka.esc')}</button>
        </div>

        <div className="export-modal__formats">
          {FORMATS.map(f => (
            <button
              key={f.id}
              type="button"
              className={`export-modal__fmt ${format === f.id ? 'is-on' : ''}`}
              onClick={() => setFormat(f.id)}
            >
              <span className="export-modal__fmt-label">{fmtLabel(f, t)}</span>
              <span className="export-modal__fmt-sub">{t(f.subKey)}</span>
            </button>
          ))}
        </div>

        {format === 'arena' && !hasArenaData && (
          <div className="export-modal__warn">
            {t('exp.warn_arena_1')}
            <br />{t('exp.warn_arena_2')}
          </div>
        )}

        {isStats && statsError && (
          <div className="export-modal__warn">&gt; {t('exp.err_stats')}</div>
        )}

        <div className="export-modal__preview-wrap">
          <div className="export-modal__preview-hd">
            <span>{t('exp.preview_title', { format: formatLabel })}</span>
            <span className="t-xs muted">
              {isStats
                ? (statsLoading
                    ? t('exp.fetching_stats')
                    : t(lineCount === 1 ? 'exp.n_lines_one' : 'exp.n_lines', { count: lineCount }))
                : cardCount}
            </span>
          </div>
          <pre className="export-modal__preview" aria-label={t(isStats ? 'exp.aria_preview_stats' : 'exp.aria_preview_decklist')}>
            {isStats && statsLoading && !statsBundle
              ? t('exp.fetching_bundle')
              : (text || t('exp.empty_deck'))}
          </pre>
        </div>

        <div className="export-modal__actions">
          <button
            type="button"
            className="export-modal__btn export-modal__btn--solid"
            onClick={onCopy}
            disabled={!text || (isStats && statsLoading)}
          >
            {t('exp.btn_copy', { what: copyWhat })}<span className="arr">⎘</span>
          </button>
          <button
            type="button"
            className="export-modal__btn"
            onClick={onDownload}
            disabled={!text || (isStats && statsLoading)}
          >
            {t('exp.btn_download', { ext: isStats ? `.${statsKind}` : ext })}<span className="arr">↓</span>
          </button>
          <button type="button" className="export-modal__btn export-modal__btn--ghost" onClick={onClose}>
            {t('common.close')}
          </button>
        </div>
      </div>
    </div>
  )
}
