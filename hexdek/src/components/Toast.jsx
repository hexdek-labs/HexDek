import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'

// Module-level pub/sub so toast() works from any context — React or not.
// Components subscribe via <ToastHost />. Anywhere else, just call
// toast.success(msg) / toast.error(msg) / toast.info(msg).

const listeners = new Set()
let nextId = 1

const DEFAULT_TTL = 3000

export function toast(message, kind = 'info', ttl = DEFAULT_TTL) {
  if (!message) return
  const item = { id: nextId++, message: String(message), kind, ttl }
  for (const l of listeners) l(item)
}

toast.success = (msg, ttl) => toast(msg, 'success', ttl)
toast.error   = (msg, ttl) => toast(msg, 'error', ttl)
toast.info    = (msg, ttl) => toast(msg, 'info', ttl)
// Combo toasts default to a longer 5s TTL — they carry more
// content than a one-line info / success / error message and
// spectators need time to read the participating cards.
toast.combo   = (msg, ttl) => toast(msg, 'combo', ttl ?? 5000)

export function useToast() {
  return toast
}

const KIND_BORDER = {
  success: 'var(--ok)',
  error:   'var(--danger)',
  info:    'var(--ink)',
  // R60 spectator UX: combo toasts use the accent (gold) token so
  // they read as "celebratory" vs the green/red/neutral palette of
  // the other kinds. Falls back to --ok when --accent isn't loaded.
  combo:   'var(--accent, var(--ok))',
}

const KIND_LABEL = {
  success: 'OK',
  error:   'ERR',
  info:    'INFO',
  combo:   '🎯',
}

export function ToastHost() {
  const [items, setItems] = useState([])

  useEffect(() => {
    const onAdd = (item) => {
      setItems(prev => [...prev, item])
      if (item.ttl > 0) {
        setTimeout(() => {
          setItems(prev => prev.filter(t => t.id !== item.id))
        }, item.ttl)
      }
    }
    listeners.add(onAdd)
    return () => { listeners.delete(onAdd) }
  }, [])

  const dismiss = (id) => setItems(prev => prev.filter(t => t.id !== id))

  if (typeof document === 'undefined') return null

  return createPortal(
    <div className="toast-host" style={{
      position: 'fixed',
      bottom: 18,
      right: 18,
      zIndex: 2000,
      display: 'flex',
      flexDirection: 'column',
      gap: 8,
      pointerEvents: 'none',
      maxWidth: 360,
    }}>
      {items.map(item => (
        <div
          key={item.id}
          role="status"
          aria-live="polite"
          onClick={() => dismiss(item.id)}
          style={{
            pointerEvents: 'auto',
            cursor: 'pointer',
            background: 'var(--bg)',
            color: 'var(--ink)',
            border: `2px solid ${KIND_BORDER[item.kind] || KIND_BORDER.info}`,
            borderRadius: 0,
            padding: '8px 12px',
            display: 'flex',
            alignItems: 'center',
            gap: 10,
            fontFamily: 'inherit',
            fontSize: 11,
            letterSpacing: '0.06em',
            textTransform: 'uppercase',
            boxShadow: '3px 3px 0 var(--rule-2)',
            // Combo toasts get a heavier emphasis animation: a brief
            // scale-up + glow on top of the standard slide-in.
            animation: item.kind === 'combo'
              ? 'hexdek-toast-in 140ms ease-out, hexdek-combo-pulse 600ms ease-out'
              : 'hexdek-toast-in 140ms ease-out',
          }}
        >
          <span style={{
            color: KIND_BORDER[item.kind] || KIND_BORDER.info,
            fontWeight: 800,
            fontSize: 9,
            letterSpacing: '0.12em',
            minWidth: 28,
          }}>{KIND_LABEL[item.kind] || 'INFO'}</span>
          <span style={{ flex: 1 }}>{item.message}</span>
          <span style={{
            fontSize: 10,
            color: 'var(--ink-3)',
            letterSpacing: '0.08em',
          }}>×</span>
        </div>
      ))}
      <style>{`
        @keyframes hexdek-toast-in {
          from { opacity: 0; transform: translateX(8px); }
          to   { opacity: 1; transform: translateX(0); }
        }
        @keyframes hexdek-combo-pulse {
          0%   { box-shadow: 3px 3px 0 var(--rule-2), 0 0 0 0 rgba(200, 162, 107, 0);   }
          30%  { box-shadow: 3px 3px 0 var(--rule-2), 0 0 16px 4px rgba(200, 162, 107, 0.55); }
          100% { box-shadow: 3px 3px 0 var(--rule-2), 0 0 0 0 rgba(200, 162, 107, 0);   }
        }
      `}</style>
    </div>,
    document.body,
  )
}

export default toast
