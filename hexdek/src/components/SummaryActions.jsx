import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Btn } from './chrome'
import { API_BASE } from '../services/api'

// SummaryActions surfaces two affordances:
//   * DEEP SUMMARY — client-side nav to /games/:id/summary (PR
//     #189) so users can drill past Report's heuristic numbers
//     into the R60 heimdall signals (zone visits, regret cards,
//     MVP cards, turning points).
//   * SHARE — copies the canonical /api/games/{id}/summary URL to
//     the clipboard. Picks API_BASE when set (dev builds talking
//     to a remote backend), otherwise window.location.origin.
//     Falls back to a hidden-textarea + execCommand pipeline when
//     navigator.clipboard isn't available (older browsers,
//     non-HTTPS contexts, Permissions API denial).
//
// Lives in its own file (rather than inline in Report.jsx) so
// react-refresh/only-export-components stays happy and the
// component is independently importable from any other screen
// that wants to surface the same affordances.

// copyShareUrl builds the share URL and pushes it to the
// clipboard. Exported separately so the URL-construction logic is
// unit-testable without a DOM. Always resolves with the
// constructed URL string — the clipboard write is best-effort.
export function copyShareUrl(gameId, base = '') {
  const origin = base || (typeof window !== 'undefined' ? window.location.origin : '')
  const url = `${origin}/api/games/${gameId}/summary`
  if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
    return navigator.clipboard.writeText(url).then(() => url).catch(() => url)
  }
  if (typeof document !== 'undefined') {
    const ta = document.createElement('textarea')
    ta.value = url
    ta.style.position = 'fixed'
    ta.style.top = '-1000px'
    document.body.appendChild(ta)
    ta.select()
    try {
      document.execCommand('copy')
    } finally {
      document.body.removeChild(ta)
    }
  }
  return Promise.resolve(url)
}

export default function SummaryActions({ gameId }) {
  const navigate = useNavigate()
  const [copied, setCopied] = useState(false)

  const onShare = () => {
    copyShareUrl(gameId, API_BASE).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1800)
    })
  }

  return (
    <span style={{ display: 'inline-flex', gap: 8 }} data-testid="summary-actions">
      <Btn
        sm
        arrow="→"
        onClick={() => navigate(`/games/${gameId}/summary`)}
        title="Open the R60 heimdall deep summary for this game"
      >
        DEEP SUMMARY
      </Btn>
      <Btn
        sm
        ghost
        arrow={null}
        onClick={onShare}
        title="Copy the /api/games/{id}/summary URL to the clipboard"
      >
        {copied ? 'COPIED ✓' : 'SHARE'}
      </Btn>
    </span>
  )
}
