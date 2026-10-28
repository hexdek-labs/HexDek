import { useState } from 'react'
import { Panel, Btn } from '../components/chrome'
import { API_BASE } from '../services/api'
import ContextBox from '../components/ContextBox'
import { useT } from '../i18n'

// Wire value -> catalog path. The chips render the TRANSLATED label; the value
// POSTed to /api/feedback stays the raw 'bug' / 'suggestion' either way.
const TYPE_KEY = { bug: 'fb.type_bug', suggestion: 'fb.type_suggestion' }

export default function BugReport() {
  const t = useT()
  const [type, setType] = useState('bug')
  const [page, setPage] = useState('')
  const [context, setContext] = useState('')
  const [symptom, setSymptom] = useState('')
  const [expected, setExpected] = useState('')
  const [contact, setContact] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [submitted, setSubmitted] = useState(false)
  // ERROR SENTINEL, not prose: { k: <catalog path>, v?: <vars> }. The old code
  // stored an English sentence here and, on a network failure, rendered the
  // BROWSER's own err.message ("Failed to fetch") — English by construction and
  // impossible to translate. Worded at the render site instead.
  const [error, setError] = useState(null)

  const handleSubmit = async () => {
    if (!symptom.trim()) {
      setError({ k: 'fb.err_symptom' })
      return
    }
    setSubmitting(true)
    setError(null)
    try {
      const res = await fetch(`${API_BASE}/api/feedback`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ type, page, context, symptom, expected, contact }),
      })
      if (!res.ok) {
        setError({ k: 'fb.err_server', v: { status: res.status } })
        return
      }
      setSubmitted(true)
    } catch {
      setError({ k: 'fb.err_network' })
    } finally {
      setSubmitting(false)
    }
  }

  if (submitted) {
    return (
      <div style={{ padding: '40px 30px', maxWidth: 600, margin: '0 auto', textAlign: 'center' }}>
        <Panel code="FB.OK" title={t('fb.ok_title')}>
          <div className="t-md" style={{ lineHeight: 1.7, padding: 20 }}>
            <p style={{ fontSize: 18, fontWeight: 700 }}>{t('fb.ok_hd')}</p>
            <p style={{ marginTop: 12, color: 'var(--ink-2)' }}>
              {t('fb.ok_body')}
            </p>
          </div>
        </Panel>
      </div>
    )
  }

  const inputStyle = {
    width: '100%', padding: '8px 10px', background: 'var(--bg-2)', border: '1px solid var(--rule-2)',
    color: 'var(--ink)', fontFamily: 'inherit', fontSize: 12, letterSpacing: '0.02em',
  }

  return (
    <div style={{ padding: '20px 30px', maxWidth: 600, margin: '0 auto' }}>
      <Panel code="FB.0" title={t('fb.title')}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>

          <div>
            <div className="t-xs muted" style={{ marginBottom: 4 }}>{t('crd.kv_type')}</div>
            <div style={{ display: 'flex', gap: 8 }}>
              {['bug', 'suggestion'].map(kind => (
                <div
                  key={kind}
                  onClick={() => setType(kind)}
                  style={{
                    padding: '6px 16px', cursor: 'pointer', fontSize: 11, letterSpacing: '0.05em',
                    border: type === kind ? '1px solid var(--ok)' : '1px dashed var(--rule-2)',
                    background: type === kind ? 'var(--bg-2)' : 'transparent',
                    color: type === kind ? 'var(--ink)' : 'var(--ink-2)',
                  }}
                >
                  {t(TYPE_KEY[kind])}
                </div>
              ))}
            </div>
          </div>

          <div>
            <div className="t-xs muted" style={{ marginBottom: 4 }}>{t('fb.lbl_page')}</div>
            <input
              type="text"
              value={page}
              onChange={e => setPage(e.target.value)}
              placeholder={t('fb.page_ph', {
                a: t('nav.spectate'), b: t('nav.decks'), c: t('nav.leaderboard'),
              })}
              style={inputStyle}
            />
          </div>

          <div>
            <div className="t-xs muted" style={{ marginBottom: 4 }}>{t('fb.lbl_context')}</div>
            <textarea
              value={context}
              onChange={e => setContext(e.target.value)}
              placeholder={t('fb.context_ph')}
              rows={3}
              style={{ ...inputStyle, resize: 'vertical' }}
            />
          </div>

          <div>
            <div className="t-xs muted" style={{ marginBottom: 4 }}>
              {type === 'bug' ? t('fb.lbl_symptom_bug') : t('fb.lbl_symptom_sug')} *
            </div>
            <textarea
              value={symptom}
              onChange={e => setSymptom(e.target.value)}
              placeholder={type === 'bug'
                ? t('fb.symptom_ph_bug')
                : t('fb.symptom_ph_sug')
              }
              rows={4}
              style={{ ...inputStyle, resize: 'vertical', borderColor: error ? 'var(--danger)' : 'var(--rule-2)' }}
            />
          </div>

          {type === 'bug' && (
            <div>
              <div className="t-xs muted" style={{ marginBottom: 4 }}>{t('fb.lbl_expected')}</div>
              <textarea
                value={expected}
                onChange={e => setExpected(e.target.value)}
                placeholder={t('fb.expected_ph')}
                rows={3}
                style={{ ...inputStyle, resize: 'vertical' }}
              />
            </div>
          )}

          <div>
            <div className="t-xs muted" style={{ marginBottom: 4 }}>{t('fb.lbl_contact')}</div>
            <input
              type="text"
              value={contact}
              onChange={e => setContact(e.target.value)}
              placeholder={t('fb.contact_ph')}
              style={inputStyle}
            />
            <div className="t-xs muted-2" style={{ marginTop: 2 }}>{t('fb.contact_note')}</div>
          </div>

          {error && <div className="t-xs" style={{ color: 'var(--danger)' }}>{t(error.k, error.v)}</div>}

          <ContextBox id="bugreport.submit">{t('fb.ctx_submit')}</ContextBox>
          <Btn onClick={handleSubmit} disabled={submitting}>
            {submitting ? t('fb.btn_submitting') : t('fb.btn_submit')}
          </Btn>
        </div>
      </Panel>
    </div>
  )
}
