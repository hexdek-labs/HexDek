import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Panel, KV, Btn, Tape } from '../components/chrome'
import { sendMagicLink } from '../lib/firebase'

export default function Login() {
  const [email, setEmail] = useState('')
  const [sent, setSent] = useState(false)
  const [error, setError] = useState(null)
  const [sending, setSending] = useState(false)
  const navigate = useNavigate()

  const handleSubmit = async (e) => {
    e.preventDefault()
    if (!email || sending) return
    setSending(true)
    setError(null)
    try {
      await sendMagicLink(email)
      setSent(true)
    } catch (err) {
      setError(err.code === 'auth/invalid-email' ? 'INVALID EMAIL FORMAT.' : 'AUTH SERVICE UNREACHABLE. TRY AGAIN.')
    } finally {
      setSending(false)
    }
  }

  return (
    <>
      <Tape left="AUTH / / DOC HX-AUTH" mid="MAGIC LINK" right="PASSWORDLESS" />

      <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 36 }}>
        <div style={{ width: '100%', maxWidth: 480 }}>
          <Panel code="AUTH.01" title="OPERATOR AUTHENTICATION" solid>
            {!sent ? (
              <form onSubmit={handleSubmit}>
                <div className="t-md muted" style={{ lineHeight: 1.6, textTransform: 'uppercase', letterSpacing: '0.04em', marginBottom: 18 }}>
                  &gt; ENTER YOUR EMAIL TO RECEIVE A MAGIC LINK.<br />
                  &gt; NO PASSWORD REQUIRED. NO TRACKING.<br />
                  &gt; ONE CLICK. YOU'RE IN.
                </div>

                <div className="t-xs muted" style={{ marginBottom: 4 }}>EMAIL ADDRESS</div>
                <div className="panel" style={{ padding: 0, borderStyle: email ? 'solid' : 'dashed' }}>
                  <input
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="OPERATOR@DOMAIN.COM"
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
                    &gt; ERROR: {error}
                  </div>
                )}

                <div className="hr" style={{ margin: '18px 0' }} />

                <div style={{ display: 'flex', gap: 10 }}>
                  <Btn solid arrow="▶" onClick={handleSubmit}>
                    {sending ? 'TRANSMITTING...' : 'SEND MAGIC LINK'}
                  </Btn>
                  <Btn ghost arrow="←" onClick={() => navigate('/')}>BACK</Btn>
                </div>
              </form>
            ) : (
              <div>
                <div className="t-2xl" style={{ fontWeight: 700, color: 'var(--ok)', marginBottom: 12 }}>LINK SENT.</div>
                <div className="t-md muted" style={{ lineHeight: 1.6, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                  &gt; CHECK YOUR INBOX FOR THE MAGIC LINK.<br />
                  &gt; SENT TO: <span style={{ color: 'var(--ink)' }}>{email}</span><br />
                  &gt; LINK EXPIRES IN 1 HOUR.
                </div>

                <div className="hr" style={{ margin: '18px 0' }} />

                <KV rows={[
                  ['STATUS', 'AWAITING CONFIRMATION'],
                  ['METHOD', 'EMAIL LINK (PASSWORDLESS)'],
                  ['EXPIRES', '60 MIN'],
                ]} />

                <div className="hr" style={{ margin: '18px 0' }} />

                <div style={{ display: 'flex', gap: 10 }}>
                  <Btn ghost arrow="↺" onClick={() => { setSent(false); setEmail('') }}>RESEND</Btn>
                  <Btn ghost arrow="←" onClick={() => navigate('/')}>BACK</Btn>
                </div>
              </div>
            )}
          </Panel>
        </div>
      </div>
    </>
  )
}
