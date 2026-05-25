import { useMemo, useState } from 'react'
import { toast } from './Toast'
import { trackEvent } from '../hooks/useAnalytics'
import { useModalKeyboard } from '../hooks/useModalKeyboard'
import { buildDeckExport, exportExtension } from '../lib/deckExport'

const FORMATS = [
  { id: 'moxfield', label: 'MOXFIELD', sub: 'Moxfield bulk-paste — Commander / Deck sections' },
  { id: 'mtgo', label: 'MTGO', sub: 'Magic Online .dec — commander in sideboard' },
  { id: 'arena', label: 'ARENA', sub: 'MTG Arena import format with set codes' },
  { id: 'raw',  label: 'RAW',   sub: 'Card names only, one per line' },
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

function download(text, filename) {
  const blob = new Blob([text], { type: 'text/plain' })
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
  const text = useMemo(() => buildDeckExport(format, cards, commanderName), [format, cards, commanderName])
  // Card-line count excludes blank separators and the section header
  // tokens (Sideboard / Commander / Commanders / Deck) that the format
  // builders emit. Anything else is a `qty name` row.
  const lineCount = text
    ? text.split('\n').filter(l => l && !/^(Sideboard|Commander|Commanders|Deck)$/.test(l)).length
    : 0
  const hasArenaData = cards.some(c => c.set || c.collector_number || c.cn)
  const baseFilename = (deckId || 'deck').replace(/[^a-z0-9_-]/gi, '_')
  const ext = exportExtension(format)

  const onCopy = async () => {
    if (!text) return
    const ok = await copy(text)
    trackEvent('deck_export_copy', { format, lines: lineCount })
    if (ok) toast.success(`COPIED ${format.toUpperCase()} (${lineCount} CARDS)`)
    else    toast.error('COPY FAILED — TRY DOWNLOAD')
  }
  const onDownload = () => {
    if (!text) return
    download(text, `${baseFilename}${ext}`)
    trackEvent('deck_export_download', { format, lines: lineCount })
    toast.success(`DOWNLOADED ${baseFilename}${ext}`)
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

        <div className="export-modal__preview-wrap">
          <div className="export-modal__preview-hd">
            <span>{format.toUpperCase()} PREVIEW</span>
            <span className="t-xs muted">{lineCount} CARDS</span>
          </div>
          <pre className="export-modal__preview" aria-label="Decklist preview">{text || '— EMPTY DECK —'}</pre>
        </div>

        <div className="export-modal__actions">
          <button type="button" className="export-modal__btn export-modal__btn--solid" onClick={onCopy} disabled={!text}>
            COPY {format.toUpperCase()}<span className="arr">⎘</span>
          </button>
          <button type="button" className="export-modal__btn" onClick={onDownload} disabled={!text}>
            DOWNLOAD {ext}<span className="arr">↓</span>
          </button>
          <button type="button" className="export-modal__btn export-modal__btn--ghost" onClick={onClose}>
            CLOSE
          </button>
        </div>
      </div>
    </div>
  )
}
