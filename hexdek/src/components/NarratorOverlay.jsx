import { useMemo } from 'react'
import { findGameChangerInText } from '../data/gameChangers'
import { useT } from '../i18n'

// NarratorOverlay — plain-language commentary strip.
//
// Reads the engine's per-game log array (each entry has turn, seat,
// action, detail, kind) and renders the last 10 narrated events as a
// semi-transparent bar pinned to the bottom of the spectator view.
// This is the foundation for Twitch / OBS overlay integration: the
// generated sentences are stable full sentences (no ALL-CAPS terminal
// styling on the prose itself, so streams can pipe them through TTS
// or chat without further processing).
//
// The component is purely derived from props — no state, no fetches.
// Mount once at the bottom of the spectator view; pass game.log and
// game.seats.

const MAX_EVENTS = 10

// Tone classes drive the left-edge accent stripe. Loud events (combat,
// game-changers, eliminations) get a colored stripe so streamers can
// pick them out at a glance.
const TONE = {
  combat:    'narrator-row--combat',
  cast:      'narrator-row--cast',
  elim:      'narrator-row--elim',
  changer:   'narrator-row--changer',
  event:     'narrator-row--event',
}

// Catalog PATHS, not English. A verb cannot ride into a sentence as an
// interpolated value and still inflect — and in several locales the case of
// the object depends on which verb governs it — so each removal verb gets its
// own full-sentence key.
const REMOVAL_KEYS = {
  DESTROYS:   'nar.destroys',
  EXILES:     'nar.exiles',
  SACRIFICES: 'nar.sacrifices',
}

// Returns the vars fragment naming the acting seat: a commander NAME when we
// have one (card names stay English in all 25 locales) or the bare seat index,
// which narratedText turns into "Player N" at the render site. narrate() is a
// pure function with no translator, so it cannot word that fallback itself.
function actorVars(seat, seats) {
  const s = seats?.[seat]
  if (s?.commander) return { actor: s.commander.split('//')[0].trim() }
  return { actorSeat: seat }
}

// Strip the "<COMMANDER> " prefix the backend prepends to most action
// strings. Returns the action text without the actor, so we can
// substitute a friendlier player label in front.
function stripActor(action, actor) {
  if (!action) return ''
  const upperActor = (actor || '').toUpperCase()
  if (upperActor && action.startsWith(upperActor + ' ')) {
    return action.slice(upperActor.length + 1)
  }
  return action
}

// narrate transforms one LogEntry into { key, vars, tone } — a catalog PATH
// plus its interpolation values, never a worded sentence. Three reasons it has
// to be a key: this is an exported pure function shared with Spectator and
// SpectateRoom and has no access to a translator; the consumer below holds the
// result in a useMemo keyed on [log, seats], so a worded sentence would freeze
// at the last log update and never re-word on a locale switch; and the whole
// sentence has to live in one catalog value because word order is part of the
// translation — ja/ko/tr are SOV and ar is VSO, so "{actor} casts {name}" is
// not a shape every locale can reuse.
//
// Counts never take an English plural "s": the call site picks between a
// singular key and a plural one, matching the app's 40 existing *_one pairs.
//
// Returns null when the entry doesn't surface in the narrator (turn
// markers, internal events, etc.) so the consumer can skip it.
function narrate(entry, seats) {
  if (!entry || !entry.action) return null
  const a = actorVars(entry.seat, seats)
  const actorRaw = seats?.[entry.seat]?.commander?.split('//')[0].trim() || ''
  const tail = stripActor(entry.action, actorRaw)
  const lower = tail.toLowerCase()
  const detail = entry.detail || ''
  const source = entry.source || ''

  // Game-changer detection wins over the kind switch — the engine fires
  // the same enter_battlefield kind for any permanent ETB; we only want
  // to call out the meta-warping ones.
  const gc = findGameChangerInText(entry.action)
  if (gc) {
    return {
      key: 'nar.gc_etb',
      vars: { ...a, name: gc.toUpperCase() },
      tone: 'changer',
    }
  }

  switch (entry.kind) {
    case 'combat': {
      // Engine emits "X ATTACKS WITH N CREATURE(S)" or
      // "X ATTACKS WITH N CREATURE(S) [→ Y]". Re-cast as a sentence.
      // P/T isn't in the log payload today; if added later, splice it in.
      const m = lower.match(/attacks with (\d+) creature/)
      if (m) {
        const n = parseInt(m[1], 10)
        const target = lower.includes('→') ? lower.split('→')[1].trim() : null
        if (target) {
          return {
            key: n === 1 ? 'nar.attack_t_one' : 'nar.attack_t',
            vars: { ...a, n, target: target.replace(/\.$/, '') },
            tone: 'combat',
          }
        }
        return {
          key: n === 1 ? 'nar.attack_one' : 'nar.attack',
          vars: { ...a, n },
          tone: 'combat',
        }
      }
      // No shape we recognise: echo the backend's own words behind the actor.
      return { key: 'nar.raw', vars: { ...a, text: tail.toLowerCase() }, tone: 'combat' }
    }
    case 'cast': {
      const m = tail.match(/^CASTS (.+)$/i)
      if (m) return { key: 'nar.casts', vars: { ...a, name: titleCase(m[1]) }, tone: 'cast' }
      if (source) return { key: 'nar.casts', vars: { ...a, name: titleCase(source) }, tone: 'cast' }
      return { key: 'nar.casts_spell', vars: a, tone: 'cast' }
    }
    case 'land': {
      const m = tail.match(/^PLAYS LAND: (.+)$/i)
      if (m) return { key: 'nar.plays', vars: { ...a, name: titleCase(m[1]) }, tone: 'event' }
      return null
    }
    case 'enter_battlefield':
    case 'etb': {
      const m = tail.match(/(?:→ ETB|ETB|ENTERS):? *(.+)$/i)
      const name = m ? titleCase(m[1]) : detail
      // nameKey, not the English words: narratedText reads it at the render
      // site. detail is a server-worded value, so it is echoed when present.
      return {
        key: 'nar.enters',
        vars: name ? { ...a, name } : { ...a, nameKey: 'nar.a_permanent' },
        tone: 'event',
      }
    }
    case 'elimination':
    case 'player_lost': {
      // detail often carries the reason ("commander damage", "lethal", …).
      if (detail) return { key: 'nar.elim_reason', vars: { ...a, reason: detail }, tone: 'elim' }
      return { key: 'nar.elim', vars: a, tone: 'elim' }
    }
    case 'damage': {
      const m = tail.match(/DEALS (\d+) DAMAGE TO (.+)$/i)
      if (m) {
        const vars = { ...a, n: m[1], target: titleCase(m[2]) }
        if (detail) return { key: 'nar.damage_detail', vars: { ...vars, detail }, tone: 'event' }
        return { key: 'nar.damage', vars, tone: 'event' }
      }
      return null
    }
    case 'counter': {
      const m = tail.match(/^COUNTERS (.+)$/i)
      if (m) return { key: 'nar.counters', vars: { ...a, name: titleCase(m[1]) }, tone: 'cast' }
      return null
    }
    case 'removal': {
      const m = tail.match(/^(DESTROYS|EXILES|SACRIFICES) (.+)$/i)
      if (m) {
        const key = REMOVAL_KEYS[m[1].toUpperCase()]
        if (!key) return null
        return { key, vars: { ...a, name: titleCase(m[2]) }, tone: 'event' }
      }
      return null
    }
    case 'token': {
      const m = tail.match(/^CREATES TOKEN: (.+)$/i)
      if (m) return { key: 'nar.token', vars: { ...a, name: titleCase(m[1]) }, tone: 'event' }
      return null
    }
    case 'reanimate': {
      const m = tail.match(/^REANIMATES (.+)$/i)
      if (m) return { key: 'nar.reanimates', vars: { ...a, name: titleCase(m[1]) }, tone: 'changer' }
      return null
    }
    case 'extra_turn':
      return { key: 'nar.extra_turn', vars: a, tone: 'changer' }
    case 'mill': {
      const m = tail.match(/MILLS (\d+) CARDS?/i)
      if (m) return {
        key: m[1] === '1' ? 'nar.mills_one' : 'nar.mills',
        vars: { ...a, n: m[1] },
        tone: 'event',
      }
      return null
    }
    case 'draw': {
      const m = tail.match(/DRAWS? (\d+) CARDS?/i) || tail.match(/DRAWS? A CARD/i)
      if (m) {
        const n = m[1] ? parseInt(m[1], 10) : 1
        return {
          key: n === 1 ? 'nar.draws_one' : 'nar.draws',
          vars: { ...a, n },
          tone: 'event',
        }
      }
      return null
    }
    case 'discard': {
      const m = tail.match(/DISCARDS? (.+)$/i)
      if (m) return { key: 'nar.discards', vars: { ...a, name: titleCase(m[1]) }, tone: 'event' }
      return null
    }
    case 'scry': {
      const m = tail.match(/SCRIES? (\d+)/i)
      if (m) return { key: 'nar.scries', vars: { ...a, n: m[1] }, tone: 'event' }
      return null
    }
    case 'surveil': {
      const m = tail.match(/SURVEILS? (\d+)/i)
      if (m) return { key: 'nar.surveils', vars: { ...a, n: m[1] }, tone: 'event' }
      return null
    }
    case 'untap': {
      const m = tail.match(/UNTAPS? (\d+) PERMANENTS?/i)
      if (m) return {
        key: m[1] === '1' ? 'nar.untaps_n_one' : 'nar.untaps_n',
        vars: { ...a, n: m[1] },
        tone: 'event',
      }
      const m2 = tail.match(/UNTAPS? (.+)$/i)
      if (m2) return { key: 'nar.untaps', vars: { ...a, name: titleCase(m2[1]) }, tone: 'event' }
      return null
    }
    case 'tap': {
      const m = tail.match(/TAPS? (\d+) PERMANENTS?/i)
      if (m) return {
        key: m[1] === '1' ? 'nar.taps_n_one' : 'nar.taps_n',
        vars: { ...a, n: m[1] },
        tone: 'event',
      }
      const m2 = tail.match(/TAPS? (.+)$/i)
      if (m2) return { key: 'nar.taps', vars: { ...a, name: titleCase(m2[1]) }, tone: 'event' }
      return null
    }
    case 'bounce': {
      const m = tail.match(/(?:BOUNCES?|RETURNS?) (.+?)(?:\s+TO HAND)?$/i)
      if (m) return { key: 'nar.bounces', vars: { ...a, name: titleCase(m[1]) }, tone: 'event' }
      return null
    }
    case 'equip': {
      const m = tail.match(/EQUIPS? (.+)$/i)
      if (m) return { key: 'nar.equips', vars: { ...a, name: titleCase(m[1]) }, tone: 'event' }
      return null
    }
    case 'shuffle':
      return { key: 'nar.shuffles', vars: a, tone: 'event' }
    case 'monarch':
      return { key: 'nar.monarch', vars: a, tone: 'changer' }
    case 'life': {
      const mg = tail.match(/GAINS? (\d+) LIFE/i)
      if (mg) return { key: 'nar.gains_life', vars: { ...a, n: mg[1] }, tone: 'event' }
      const ml = tail.match(/(?:LOSES?|PAYS?) (\d+) LIFE/i)
      if (ml) return { key: 'nar.loses_life', vars: { ...a, n: ml[1] }, tone: 'event' }
      return null
    }
    case 'trigger': {
      const m = tail.match(/TRIGGERS? (.+)$/i)
      if (m) return { key: 'nar.triggers', vars: { ...a, name: titleCase(m[1]) }, tone: 'event' }
      return null
    }
    case 'activate': {
      const m = tail.match(/ACTIVATES? (.+)$/i)
      if (m) return { key: 'nar.activates', vars: { ...a, name: titleCase(m[1]) }, tone: 'event' }
      return null
    }
    case 'search':
      return { key: 'nar.searches', vars: a, tone: 'event' }
    default:
      return null
  }
}

// Words one narrate() result. It lives beside narrate so that all three
// consumers — this component, Spectator and SpectateRoom — share one wording
// site, and so the two deferred fallbacks above are resolved in exactly one
// place. Returns a plain string, which is what linkifyNarrated() needs: it
// string-searches the sentence for the card name and splits around it, and
// card names stay English in all 25 locales, so that still lands.
function narratedText(n, t) {
  if (!n) return ''
  const v = { ...n.vars }
  if (v.actor == null) v.actor = t('nar.player_n', { n: v.actorSeat + 1 })
  if (v.name == null && v.nameKey) v.name = t(v.nameKey)
  return t(n.key, v)
}

function titleCase(s) {
  if (!s) return ''
  return s.toLowerCase().replace(/\b\w/g, c => c.toUpperCase())
}

export { narrate, narratedText }

export default function NarratorOverlay({ log, seats }) {
  const t = useT()
  // The memo below holds catalog KEYS, so it is locale-independent and needs
  // no locale dep: the wording happens in the render body, which useT() re-runs
  // on a locale change. This is the one shape where getLocale() is NOT needed.
  const lines = useMemo(() => {
    if (!Array.isArray(log) || log.length === 0) return []
    const out = []
    // Walk newest → oldest so we don't waste work narrating events
    // that'll be cropped before reaching the top of the strip.
    for (let i = log.length - 1; i >= 0 && out.length < MAX_EVENTS; i--) {
      const n = narrate(log[i], seats)
      if (!n) continue
      // rowKey, not key: narrate()'s own `key` field is the catalog path.
      // This one is the React list identity and stays locale-independent.
      out.push({ ...n, turn: log[i].turn, rowKey: `${i}-${log[i].kind}` })
    }
    return out.reverse() // oldest first so newest is at the bottom (most-visible) edge
  }, [log, seats])

  if (lines.length === 0) return null

  return (
    <div className="narrator-overlay" role="log" aria-live="polite" aria-atomic="false">
      <div className="narrator-overlay-hd">
        <span>{t('nar.hd')}</span>
        <span className="narrator-overlay-count">{lines.length} / {MAX_EVENTS}</span>
      </div>
      <div className="narrator-overlay-rows">
        {lines.map((l, idx) => (
          <div
            key={l.rowKey}
            className={`narrator-row ${TONE[l.tone] || ''}${idx === lines.length - 1 ? ' narrator-row--latest' : ''}`}
          >
            <span className="narrator-row-turn">T{l.turn}</span>
            <span className="narrator-row-text">{narratedText(l, t)}</span>
          </div>
        ))}
      </div>
    </div>
  )
}
