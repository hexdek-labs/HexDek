import { useNavigate } from 'react-router-dom'
import { Btn } from './chrome'
import { useModalKeyboard } from '../hooks/useModalKeyboard'
import { useT } from '../i18n'

// `action` is an action ID, NOT an English phrase. It used to be prose
// ('import deck') interpolated into both an aria-label and a panel header, so
// the phrase could never be translated from in here and no caller could
// supply a localized one.
//
// Each action carries TWO paths rather than one, because the aria-label is a
// WHOLE SENTENCE per action and not a template: German's `um ... zu` clause
// puts *zu* immediately before a clause-final verb ('um ein Deck ZU
// IMPORTIEREN'), so an interpolated verb phrase cannot be made grammatical by
// any wording of the template. The short label is interpolated only into the
// header, where a noun-ish fragment is safe.
const ACTIONS = {
  continue:    { labelKey: 'auth.action_continue',    ariaKey: 'auth.aria_continue' },
  import_deck: { labelKey: 'auth.action_import_deck', ariaKey: 'auth.aria_import_deck' },
}
const actionPaths = (id) => ACTIONS[id] || ACTIONS.continue

// AuthPrompt — contextual sign-in modal triggered when an anon user
// reaches for an auth-gated action (upload, import, save, etc.).
//
// Primary path is Firebase email magic-link. Discord OAuth is stubbed
// (no provider configured) — when Discord auth lands, swap the disabled
// onClick for a real handler and drop the `disabled` prop.
// `inline` (optional) — when true, drops the fixed-position overlay
// and renders the auth panel flush in the parent layout. Used by the
// /import landing where AuthPrompt is the *page's* CTA, not a popover.
export default function AuthPrompt({ onClose, action = 'continue', inline = false }) {
  const navigate = useNavigate()
  const goLogin = () => { onClose(); navigate('/login') }

  if (inline) {
    return (
      <div className="panel" style={{ maxWidth: 480, width: '100%' }}>
        <AuthPromptBody action={action} goLogin={goLogin} onClose={onClose} />
      </div>
    )
  }
  return <AuthPromptOverlay action={action} goLogin={goLogin} onClose={onClose} />
}

function AuthPromptOverlay({ action, goLogin, onClose }) {
  const t = useT()
  const panelRef = useModalKeyboard({ onClose })
  return (
    <div style={{
      position: 'fixed', inset: 0, zIndex: 1000,
      background: 'rgba(0,0,0,0.7)', display: 'flex', alignItems: 'center', justifyContent: 'center',
    }} onClick={onClose}>
      <div
        ref={panelRef}
        className="panel"
        onClick={e => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label={t(actionPaths(action).ariaKey)}
        style={{ maxWidth: 440, width: '100%' }}
      >
        <AuthPromptBody action={action} goLogin={goLogin} onClose={onClose} showCancel />
      </div>
    </div>
  )
}

function AuthPromptBody({ action, goLogin, onClose, showCancel = true }) {
  const t = useT()
  return (
    <>
      <div className="panel-hd">
        {/* No .toUpperCase() here: `.panel-hd` already carries
            `text-transform: uppercase` in index.css, so the JS call added
            nothing — while a locale-blind JS uppercase mangles Turkish
            dotted/dotless i and CSS text-transform does not. */}
        <span>{t('auth.prompt_hd', { action: t(actionPaths(action).labelKey) })}</span>
        {showCancel && (
          <button
            type="button"
            onClick={onClose}
            aria-label={t('auth.prompt_aria_close')}
            style={{ background: 'transparent', border: 'none', color: 'inherit', font: 'inherit', cursor: 'pointer', padding: 0 }}
          >X</button>
        )}
      </div>
      <div className="panel-bd" style={{ display: 'flex', flexDirection: 'column', gap: 16, padding: 22 }}>
        <div className="t-md" style={{ lineHeight: 1.6, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
          {t('auth.prompt_pitch')}
        </div>

        <button
          onClick={goLogin}
          style={{
            display: 'flex', alignItems: 'center', justifyContent: 'space-between',
            gap: 12, padding: '12px 16px',
            background: 'var(--accent)', color: 'var(--bg)',
            border: '1px solid var(--accent)',
            fontFamily: 'inherit', fontSize: 13, fontWeight: 800,
            letterSpacing: '0.08em', textTransform: 'uppercase', cursor: 'pointer',
          }}
        >
          <span style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <span style={{ fontSize: 16, lineHeight: 1 }}>✉</span>
            <span>{t('auth.btn_email')}</span>
          </span>
          <span>↗</span>
        </button>

        <button
          disabled
          title={t('auth.discord_tip')}
          style={{
            display: 'flex', alignItems: 'center', justifyContent: 'space-between',
            gap: 12, padding: '12px 16px',
            background: 'transparent', color: 'var(--ink-2)',
            border: '1px dashed var(--rule-2)',
            fontFamily: 'inherit', fontSize: 13, fontWeight: 700,
            letterSpacing: '0.08em', textTransform: 'uppercase',
            cursor: 'not-allowed', opacity: 0.65,
          }}
        >
          <span style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <DiscordGlyph />
            <span>{t('auth.btn_discord')}</span>
          </span>
          <span className="t-xs muted-2" style={{ letterSpacing: '0.1em' }}>{t('auth.soon')}</span>
        </button>

        <div className="t-xs muted-2" style={{ lineHeight: 1.5 }}>
          {t('auth.prompt_note')}
        </div>

        {showCancel && (
          <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
            <Btn sm ghost onClick={onClose}>{t('common.cancel')}</Btn>
          </div>
        )}
      </div>
    </>
  )
}

function DiscordGlyph() {
  return (
    <svg width="16" height="12" viewBox="0 0 71 55" fill="currentColor" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
      <path d="M60.1 4.9A58.5 58.5 0 0 0 45.6.5l-.7 1a54.4 54.4 0 0 0-15.2 0l-.7-1a58.5 58.5 0 0 0-14.5 4.4A60 60 0 0 0 .5 41a58.9 58.9 0 0 0 18 9 43.4 43.4 0 0 0 3.8-6.3 38 38 0 0 1-6-2.9c.5-.4 1-.8 1.5-1.2a42 42 0 0 0 35.6 0l1.5 1.2a38 38 0 0 1-6 3 43.4 43.4 0 0 0 3.8 6.2 58.6 58.6 0 0 0 18-9A60 60 0 0 0 60.1 5ZM23.7 35.3c-3.5 0-6.4-3.3-6.4-7.3 0-4 2.8-7.3 6.4-7.3 3.6 0 6.5 3.3 6.4 7.3 0 4-2.8 7.3-6.4 7.3Zm23.6 0c-3.5 0-6.4-3.3-6.4-7.3 0-4 2.8-7.3 6.4-7.3 3.6 0 6.5 3.3 6.4 7.3 0 4-2.8 7.3-6.4 7.3Z"/>
    </svg>
  )
}
