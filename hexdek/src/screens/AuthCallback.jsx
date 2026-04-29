import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Panel, Tape } from '../components/chrome'
import { completeMagicLinkSignIn } from '../lib/firebase'

export default function AuthCallback() {
  const [status, setStatus] = useState('VERIFYING...')
  const [error, setError] = useState(null)
  const navigate = useNavigate()

  useEffect(() => {
    completeMagicLinkSignIn()
      .then((user) => {
        if (user) {
          setStatus('AUTHENTICATED.')
          setTimeout(() => navigate('/dash'), 800)
        } else {
          setError('INVALID OR EXPIRED LINK.')
        }
      })
      .catch(() => {
        setError('AUTH VERIFICATION FAILED.')
      })
  }, [navigate])

  return (
    <>
      <Tape left="AUTH / / CALLBACK" mid="VERIFYING" right="MAGIC LINK" />
      <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 36 }}>
        <Panel code="AUTH.02" title="LINK VERIFICATION" solid style={{ maxWidth: 420, width: '100%' }}>
          {error ? (
            <div>
              <div className="t-xl" style={{ fontWeight: 700, color: 'var(--danger)' }}>{error}</div>
              <div className="t-xs muted" style={{ marginTop: 8 }}>&gt; TRY REQUESTING A NEW LINK.</div>
            </div>
          ) : (
            <div>
              <div className="t-xl" style={{ fontWeight: 700, color: 'var(--ok)' }}>{status}</div>
              <div className="t-xs muted" style={{ marginTop: 8 }}>&gt; REDIRECTING TO DASHBOARD<span className="blink">_</span></div>
            </div>
          )}
        </Panel>
      </div>
    </>
  )
}
