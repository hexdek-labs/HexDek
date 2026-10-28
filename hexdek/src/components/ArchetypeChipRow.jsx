import { useState } from 'react'
import { Tag, Btn } from './chrome'
import { api } from '../services/api'
import { useT } from '../i18n'

// Canonical Freya archetype list (mirror of cmd/hexdek-freya/archetypes.go).
// The frontend hard-codes the suggestions so the disagree dropdown stays
// useful even when the backend doesn't expose an enumeration endpoint;
// power users posting directly to the API can send any value (server
// just normalizes + length-caps).
const FREYA_ARCHETYPES = [
  'Voltron', 'Aggro / Go Wide', 'Extra Combats', 'Combo / Infinite',
  'Storm', 'Control', 'Stax', 'Aristocrats', 'Artifacts', 'Enchantress',
  'Reanimator', 'Lands Matter', 'Tribal', 'Superfriends', 'Mill',
  'Lifegain', 'Discard / Hand Attack', 'Blink / Flicker', 'Spellslinger',
  'Counters Matter', 'Theft / Clone', 'Ninjutsu / Evasion', 'Pillowfort',
  'Group Slug', 'Damage Redirect',
]

// ArchetypeChipRow renders the FREYA system-tag chip alongside
// owner-only Confirm / Disagree / Reset affordances that feed the
// /api/decks/{owner}/{id}/archetype-feedback endpoint.
//
// Visual states:
//   - no feedback:        chip + ✓ Confirm and ✗ Disagree buttons
//   - feedback="confirmed": chip + green ✓ badge + Reset link
//   - feedback="<value>":  Freya chip (struck through) + USER YOUR-VALUE chip
//                          + Reset link
//
// Disagree opens a small inline picker of the 25 canonical Freya
// archetype names. Picking one POSTs `correct` with that value;
// closing without picking is a no-op.
export default function ArchetypeChipRow({ deck, isOwner, onFeedbackChange }) {
  const t = useT()
  const [picking, setPicking] = useState(false)
  const [busy, setBusy] = useState(false)

  const owner = deck?.owner
  const id = deck?.id
  const systemTags = deck?.system_tags || []
  const feedback = deck?.archetype_feedback || ''

  const isConfirmed = feedback === 'confirmed'
  const correction = !isConfirmed && feedback !== '' ? feedback : ''

  const stripArchetype = (raw) =>
    raw.startsWith('archetype:') ? raw.slice('archetype:'.length) : raw

  const post = async (action, archetype) => {
    if (!owner || !id) return
    setBusy(true)
    try {
      const res = await api.archetypeFeedback(`${owner}/${id}`, action, archetype)
      // Patch the deck object so the UI re-renders immediately
      // without a full reload.
      onFeedbackChange?.(prev => ({ ...(prev || {}), archetype_feedback: res.feedback ?? '' }))
      setPicking(false)
    } catch {
      // Errors surface in console; UI just unfreezes — toast
      // infrastructure isn't available in this component scope.
    } finally {
      setBusy(false)
    }
  }

  // A `title` attribute is a plain string, so t() vars are right here and
  // fill() is not. {value} and {tags} are stays-English canonical archetype
  // names riding in as values.
  const tip = correction
    ? t('dka.arch_tip_corrected', {
        value: correction,
        tags: systemTags.map(stripArchetype).join(', '),
      })
    : isConfirmed
      ? t('dka.arch_tip_confirmed')
      : t('dka.arch_tip_default')

  return (
    <div
      style={{ marginTop: 8, display: 'flex', flexWrap: 'wrap', gap: 4, alignItems: 'center' }}
      data-testid="system-tags"
      title={tip}
    >
      <span className="muted-2" style={{ fontSize: 9, letterSpacing: '0.1em', marginRight: 4 }}>
        FREYA
      </span>
      {systemTags.map(tag => {
        const label = stripArchetype(tag)
        // When the user has corrected, strikethrough the original.
        const style = correction
          ? { textDecoration: 'line-through', opacity: 0.6 }
          : undefined
        return (
          <Tag key={tag} kind="info" solid style={style}>
            {label}
          </Tag>
        )
      })}
      {isConfirmed && (
        <Tag kind="good" solid data-testid="archetype-confirmed-badge">✓</Tag>
      )}
      {correction && (
        <>
          <span className="muted-2" style={{ fontSize: 9, marginLeft: 6 }}>→</span>
          <span className="muted-2" style={{ fontSize: 9, letterSpacing: '0.1em', marginRight: 4 }}>
            {t('spec.gb_you')}
          </span>
          <Tag kind="warn" solid data-testid="archetype-correction">
            {correction}
          </Tag>
        </>
      )}
      {isOwner && (
        <span style={{ display: 'inline-flex', gap: 4, marginLeft: 8 }}>
          {!isConfirmed && !correction && (
            <>
              <Btn
                sm
                arrow={null}
                disabled={busy}
                onClick={() => post('confirm')}
                data-testid="archetype-confirm"
              >
                ✓ {t('common.confirm')}
              </Btn>
              <Btn
                sm
                ghost
                arrow={null}
                disabled={busy}
                onClick={() => setPicking(p => !p)}
                data-testid="archetype-disagree"
              >
                ✗ {t('dka.arch_disagree')}
              </Btn>
            </>
          )}
          {(isConfirmed || correction) && (
            <Btn
              sm
              ghost
              arrow={null}
              disabled={busy}
              onClick={() => post('clear')}
              data-testid="archetype-reset"
              title={t('dka.arch_reset_title')}
            >
              {t('dka.arch_reset')}
            </Btn>
          )}
        </span>
      )}
      {picking && isOwner && (
        <div
          style={{
            marginTop: 6,
            width: '100%',
            display: 'flex',
            flexWrap: 'wrap',
            gap: 4,
          }}
          data-testid="archetype-picker"
        >
          {FREYA_ARCHETYPES.map(a => (
            <button
              key={a}
              type="button"
              disabled={busy}
              onClick={() => post('correct', a)}
              className="btn btn--sm btn--ghost"
              style={{ fontSize: 9 }}
            >
              {a}
            </button>
          ))}
          <button
            type="button"
            disabled={busy}
            onClick={() => setPicking(false)}
            className="btn btn--sm btn--ghost"
            style={{ fontSize: 9, opacity: 0.6 }}
          >
            {t('common.cancel')}
          </button>
        </div>
      )}
    </div>
  )
}
