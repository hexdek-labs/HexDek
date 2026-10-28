import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Panel, KV, Btn, Tape } from '../components/chrome'
import { sendMagicLink } from '../lib/firebase'
import { listenAuth, AUTH_EVENT } from '../lib/authBroadcast'
import MagicLinkConsole from '../components/MagicLinkConsole'
import { useT } from '../i18n'

// The magic link's time-to-live, in minutes. The screen used to state this
// duration TWO different ways -- the prose said '1 HOUR' and the KV row said
// '60 MIN' -- with neither derived from anything, so a TTL change would have
// left two strings lying in 25 locales each. Both now come from here.
const LINK_TTL_MIN = 60

// The three failure messages are SENTINELS, not prose: they are assigned in a
// .catch() and in a broadcast listener and rendered later, so a worded string
// would be fixed at failure time and would not follow a locale switch. The
// render site maps the code to a catalog path.
const AUTH_ERR = {
  link_failed:   'auth.err_link_failed',
  invalid_email: 'auth.err_invalid_email',
  unreachable:   'auth.err_unreachable',
}

export default function Login() {
  const t = useT()
  const [email, setEmail] = useState('')
  const [sent, setSent] = useState(false)
  const [error, setError] = useState(null)
  const [sending, setSending] = useState(false)
  const [authed, setAuthed] = useState(null)
  const navigate = useNavigate()

  // Listen for the magic-link tab's broadcast. We arm the listener as
  // soon as the user lands on /login so a slow email-client (or a user
  // who pasted the link into a new browser entirely) still hands off
  // cleanly. When auth succeeds, swap to the MagicLinkConsole; when it
  // fails, surface the failure inline.
  useEffect(() => {
    const stop = listenAuth((msg) => {
      if (msg?.type === AUTH_EVENT.SUCCEEDED) {
        setAuthed({ email: msg.email })
      } else if (msg?.type === AUTH_EVENT.FAILED) {
        setError('link_failed')
      }
    })
    return stop
  }, [])

  if (authed) {
    return (
      <MagicLinkConsole
        email={authed.email || email || null}
        redirectTo="/operator"
        onComplete={() => navigate('/operator', { replace: true })}
      />
    )
  }

  const handleSubmit = async (e) => {
    e.preventDefault()
    if (!email || sending) return
    setSending(true)
    setError(null)
    try {
      await sendMagicLink(email)
      setSent(true)
    } catch (err) {
      setError(err.code === 'auth/invalid-email' ? 'invalid_email' : 'unreachable')
    } finally {
      setSending(false)
    }
  }

  return (
    <>
      <Tape left="AUTH / / DOC HX-AUTH" mid={t('auth.tape_mid')} right={t('auth.tape_right')} />

      <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 36 }}>
        <div style={{ width: '100%', maxWidth: 480 }}>
          <Panel code="AUTH.01" title={t('auth.panel_title')} solid>
            {!sent ? (
              <form onSubmit={handleSubmit}>
                <div className="t-md muted" style={{ lineHeight: 1.6, textTransform: 'uppercase', letterSpacing: '0.04em', marginBottom: 18 }}>
                  {t('auth.intro_1')}<br />
                  {t('auth.intro_2')}<br />
                  {t('auth.intro_3')}
                </div>

                <div className="t-xs muted" style={{ marginBottom: 4 }}>{t('auth.email_label')}</div>
                <div className="panel" style={{ padding: 0, borderStyle: email ? 'solid' : 'dashed' }}>
                  <input
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder={t('auth.email_ph')}
                    autoFocus
                    style={{
                      width: '100%',
                      padding: '12px 14px',
                      background: 'transparent',
                      border: 'none',
                      color: 'var(--ink)',
                      fontFamily: 'inherit',
                      fontSize: 14,
                      letterSpacing: '0.06em',
                      textTransform: 'uppercase',
                      outline: 'none',
                    }}
                  />
                </div>

                {error && (
                  <div className="t-xs" style={{ color: 'var(--danger)', marginTop: 8 }}>
                    &gt; {t('frg.err_prefix', { error: AUTH_ERR[error] ? t(AUTH_ERR[error]) : error })}
                  </div>
                )}

                <div className="hr" style={{ margin: '18px 0' }} />

                <div style={{ display: 'flex', gap: 10 }}>
                  <Btn solid arrow="▶" onClick={handleSubmit}>
                    {sending ? t('auth.btn_sending') : t('auth.btn_send')}
                  </Btn>
                  <Btn ghost arrow="←" onClick={() => navigate('/')}>{t('auth.btn_back')}</Btn>
                </div>
              </form>
            ) : (
              <div>
                <div className="t-2xl" style={{ fontWeight: 700, color: 'var(--ok)', marginBottom: 12 }}>{t('auth.link_sent')}</div>
                <div className="t-md muted" style={{ lineHeight: 1.6, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                  {t('auth.check_inbox')}<br />
                  {t('auth.sent_to')} <span style={{ color: 'var(--ink)' }}>{email}</span><br />
                  {t(LINK_TTL_MIN / 60 === 1 ? 'auth.expires_in_one' : 'auth.expires_in', { hours: LINK_TTL_MIN / 60 })}
                </div>

                <div className="hr" style={{ margin: '18px 0' }} />

                <KV rows={[
                  [t('spec.kv_status'), t('auth.kv_awaiting')],
                  [t('auth.kv_method'), t('auth.kv_method_value')],
                  [t('auth.kv_expires'), t('auth.kv_expires_value', { minutes: LINK_TTL_MIN })],
                ]} />

                <div className="hr" style={{ margin: '18px 0' }} />

                <div style={{ display: 'flex', gap: 10 }}>
                  <Btn ghost arrow="↺" onClick={() => { setSent(false); setEmail('') }}>{t('auth.btn_resend')}</Btn>
                  <Btn ghost arrow="←" onClick={() => navigate('/')}>{t('auth.btn_back')}</Btn>
                </div>
              </div>
            )}
          </Panel>
        </div>
      </div>
    </>
  )
}
