import { Panel, KV } from '../components/chrome'
import { useT } from '../i18n'
import { fill } from '../components/fill'

export default function About() {
  const t = useT()
  return (
    <div style={{ padding: '20px 30px', maxWidth: 800, margin: '0 auto' }}>
      <Panel code="ABT.0" title={t('abt.title')}>
        <div className="t-md" style={{ lineHeight: 1.7 }}>
          <p>{t('abt.intro_1')}</p>
          <p style={{ marginTop: 12 }}>{t('abt.intro_2')}</p>
        </div>
      </Panel>

      {/* PHILOSOPHY reads the donations page's key: same word, same
          register, already translated in all 25. A second key holding
          the same sentence is how frg.p_value drifted into reading
          half-English in 18 locales. */}
      <Panel code="ABT.1" title={t('don.philosophy')} style={{ marginTop: 16 }}>
        <div className="t-md" style={{ lineHeight: 1.7 }}>
          <p>
            <strong>{t('abt.phil_strong')}</strong>
          </p>
          <p style={{ marginTop: 8 }}>{t('abt.phil_1')}</p>
          <p style={{ marginTop: 8 }}>{t('abt.phil_2')}</p>
        </div>
      </Panel>

      {/* The tool names inside these values — YggdrasilHat, Heimdall,
          Freya, Huginn, Muninn, Moxfield — ride through each locale
          unchanged; the prose around them is what gets worded. DECKS is
          nav.decks, the same word the sidebar already renders. */}
      <Panel code="ABT.2" title={t('abt.t_what')} style={{ marginTop: 16 }}>
        <KV rows={[
          [t('abt.k_engine'), t('abt.v_engine')],
          [t('abt.k_ai'), t('abt.v_ai')],
          [t('abt.k_analytics'), t('abt.v_analytics')],
          [t('abt.k_ratings'), t('abt.v_ratings')],
          [t('abt.k_integrity'), t('abt.v_integrity')],
          [t('nav.decks'), t('abt.v_decks')],
        ]} />
      </Panel>

      {/* Three values here stay English and have no key: 'React + Vite'
          and the eight-name tool roster are proper nouns start to
          finish, and MIT is an SPDX licence identifier, not a word.
          k_ai_hats keeps HATS for the same reason — `hat` is the
          engine's own name for an AI player. */}
      <Panel code="ABT.3" title={t('abt.t_stack')} style={{ marginTop: 16 }}>
        <KV rows={[
          [t('abt.k_engine'), t('abt.v_go')],
          [t('abt.k_frontend'), 'React + Vite'],
          [t('abt.k_ai_hats'), t('abt.v_hats')],
          [t('abt.k_tools'), 'Thor, Odin, Loki, Heimdall, Freya, Valkyrie, Huginn, Muninn'],
          [t('abt.k_data'), t('abt.v_data')],
          [t('abt.k_license'), 'MIT'],
        ]} />
      </Panel>

      <Panel code="ABT.4" title={t('abt.t_contact')} style={{ marginTop: 16 }}>
        <div className="t-md" style={{ lineHeight: 1.7 }}>
          {/* The name of the control is nav.feedback — the key the footer
              link itself renders — not a string of its own. This sentence
              used to name a "Bug Report" button; the link it points at
              reads BUG / SUGGESTION. Minting the name here would have
              frozen that mismatch into 25 catalogs instead of one. */}
          <p>
            {fill(t('abt.contact_1'), {
              button: <strong style={{ color: 'var(--danger)' }}>{t('nav.feedback')}</strong>,
            })}
          </p>
          <p style={{ marginTop: 8 }}>{t('abt.contact_2')}</p>
        </div>
      </Panel>
    </div>
  )
}
