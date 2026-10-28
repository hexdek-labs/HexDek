import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Panel, Tag, Tape, Btn } from '../components/chrome'
import { api, cardArtUrl } from '../services/api'
import { useAuth } from '../context/AuthContext'
import { useArtContrast } from '../hooks/useArtContrast'
import { countryFlagEmoji } from '../lib/flag'
import { toast } from '../components/Toast'
import { useT } from '../i18n'

// Names the `q.length < 2` search gate. RESIDUAL MIRROR, FLAGGED: the reused
// `search.min_chars` bakes "2+" into its value in all 25 locales, so this
// constant and that string can still drift. Re-wording a SHARED key across 25
// for a second surface (the universal search) is a wording decision, so the
// constant makes the drift greppable rather than silent.
const MIN_QUERY = 2

// Friends — /friends. Lightweight pub model: see who you've added,
// peek at their decks and ELO via PublicProfile, search for new
// players to add. No feed, no notifications.
//
// Data flow:
//   - api.listFriends(userOwner)              → ['slug', 'slug', ...]
//   - api.getOwnerProfiles(allSlugs)          → { slug: 'US', ... } for flags
//   - api.getDecks({ owner: slug }) per row   → first deck = commander art tile
//   - api.search(query) filtered to .owners  → search hits
//
// Per-friend deck fetches are sequential by useEffect identity so a
// long friend list doesn't slam the API on mount; results are kept in
// a module-level cache so navigating back doesn't refetch.

const COVER_CACHE = new Map() // owner -> { commander, deckCount }

function deriveOwner(user) {
  if (typeof localStorage !== 'undefined') {
    const stored = localStorage.getItem('hexdek_owner')
    if (stored) return stored.toLowerCase()
  }
  if (user?.displayName) return user.displayName.toLowerCase()
  if (user?.email) return user.email.split('@')[0].split('.')[0].toLowerCase()
  return ''
}

function FriendTile({ owner, onUnfriend, onOpen, country }) {
  const t = useT()
  const cached = COVER_CACHE.get(owner)
  const [cover, setCover] = useState(cached || null)

  useEffect(() => {
    if (cached) return
    let alive = true
    api.getDecks({ owner })
      .then(decks => {
        if (!alive) return
        const list = Array.isArray(decks) ? decks : []
        const first = list[0]
        const c = {
          commander: first?.commander_card || first?.commander || '',
          deckCount: list.length,
        }
        COVER_CACHE.set(owner, c)
        setCover(c)
      })
      .catch(() => {})
    return () => { alive = false }
  }, [owner, cached])

  const artUrl = cover?.commander ? cardArtUrl(cover.commander) : null
  const artContrast = useArtContrast(artUrl)
  const flag = countryFlagEmoji(country)
  const titleStyle = artContrast === 'light'
    ? { color: '#0c0d0a', textShadow: '0 1px 3px rgba(255,255,255,0.85)' }
    : { color: '#f4f0e6', textShadow: '0 1px 3px rgba(0,0,0,0.9)' }

  return (
    <div
      data-art-contrast={artContrast || undefined}
      style={{
        background: 'var(--panel)',
        border: '1px solid var(--rule-2)',
        display: 'flex',
        flexDirection: 'column',
        cursor: 'pointer',
        ...(artContrast ? { '--art-contrast': artContrast } : null),
      }}
      onClick={() => onOpen(owner)}
    >
      <div
        className={artUrl ? '' : 'hatch'}
        style={{ aspectRatio: '5/4', position: 'relative', overflow: 'hidden', background: 'var(--bg-2)' }}
      >
        {artUrl && (
          <img
            src={artUrl}
            alt=""
            loading="lazy"
            style={{ width: '100%', height: '100%', objectFit: 'cover', display: 'block' }}
            onError={(e) => { e.target.style.display = 'none'; e.target.parentElement.classList.add('hatch') }}
          />
        )}
        <div
          style={{
            position: 'absolute', inset: 0,
            background: artContrast === 'light'
              ? 'linear-gradient(to bottom, rgba(255,255,255,0) 45%, rgba(255,255,255,0.7) 100%)'
              : 'linear-gradient(to bottom, rgba(0,0,0,0) 45%, rgba(0,0,0,0.78) 100%)',
            pointerEvents: 'none',
          }}
        />
        <button
          type="button"
          onClick={(e) => { e.stopPropagation(); onUnfriend(owner) }}
          title={t('dka.unfriend_title', { name: owner.toUpperCase() })}
          style={{
            position: 'absolute', top: 6, right: 6,
            background: 'rgba(0,0,0,0.65)', color: 'var(--ink)',
            border: '1px solid var(--rule-2)',
            padding: '2px 6px', fontSize: 9, fontWeight: 700,
            letterSpacing: '0.08em', cursor: 'pointer',
            fontFamily: 'inherit',
          }}
        >✕</button>
        <div
          style={{
            position: 'absolute', bottom: 8, left: 10, right: 10,
            ...titleStyle,
          }}
        >
          <div style={{ fontSize: 14, fontWeight: 700, lineHeight: 1.15, letterSpacing: '0.02em', display: 'flex', alignItems: 'center', gap: 6 }}>
            {flag && <span style={{ fontSize: 16, lineHeight: 1 }}>{flag}</span>}
            {owner.toUpperCase()}
          </div>
          {cover?.commander && (
            <div style={{ fontSize: 10, marginTop: 2, opacity: 0.85 }}>
              {cover.commander}
            </div>
          )}
        </div>
      </div>
      <div style={{
        padding: '6px 10px',
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        borderTop: '1px solid var(--rule-2)',
        fontSize: 10,
        letterSpacing: '0.06em',
      }}>
        <span className="muted">
          {cover
            ? t(cover.deckCount === 1 ? 'deckpower.decks_one' : 'deckpower.decks', { count: cover.deckCount })
            : t('dka.loading_ell')}
        </span>
        <span style={{ color: 'var(--accent)' }}>{t('dash.open')} ↗</span>
      </div>
    </div>
  )
}

function SearchHit({ slug, isFriend, isSelf, country, onAdd, navigate }) {
  const t = useT()
  const flag = countryFlagEmoji(country)
  return (
    <div style={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      padding: '8px 10px',
      borderBottom: '1px dashed var(--rule-2)',
      gap: 10,
    }}>
      <span
        onClick={() => navigate(`/profile/${slug}`)}
        style={{ cursor: 'pointer', fontWeight: 700, letterSpacing: '0.04em', display: 'flex', alignItems: 'center', gap: 6 }}
      >
        {flag && <span style={{ fontSize: 14, lineHeight: 1 }}>{flag}</span>}
        {slug.toUpperCase()}
        {isSelf && <span className="t-xs muted" style={{ marginLeft: 4 }}>{t('fnd.you')}</span>}
      </span>
      {isSelf ? (
        <span className="t-xs muted">—</span>
      ) : isFriend ? (
        <Tag solid kind="ok">{t('fnd.is_friend')} ✓</Tag>
      ) : (
        <Btn sm arrow="+" onClick={() => onAdd(slug)}>{t('fnd.add_friend')}</Btn>
      )}
    </div>
  )
}

export default function Friends() {
  const t = useT()
  const { user } = useAuth()
  const navigate = useNavigate()
  const userOwner = useMemo(() => deriveOwner(user), [user])

  const [friends, setFriends] = useState([])
  const [loading, setLoading] = useState(true)
  const [countries, setCountries] = useState({}) // owner -> 'US'
  const [query, setQuery] = useState('')
  const [results, setResults] = useState([])
  const [searching, setSearching] = useState(false)

  // Initial friend list. Normalize to slug strings — listFriends has
  // returned both ['slug', ...] and [{owner: 'slug'}, ...] shapes
  // historically, and downstream code expects plain strings.
  useEffect(() => {
    if (!userOwner) { setFriends([]); setLoading(false); return }
    let alive = true
    setLoading(true)
    api.listFriends(userOwner)
      .then(r => {
        if (!alive) return
        const raw = Array.isArray(r) ? r : (r?.friends || [])
        const slugs = raw
          .map(f => typeof f === 'string' ? f : (f?.owner || f?.slug || ''))
          .filter(Boolean)
        setFriends(slugs)
      })
      .catch(() => { if (alive) setFriends([]) })
      .finally(() => { if (alive) setLoading(false) })
    return () => { alive = false }
  }, [userOwner])

  // Country flags for friends + search hits, batched.
  useEffect(() => {
    const wanted = new Set(friends)
    for (const r of results) wanted.add(r.label)
    const missing = [...wanted].filter(o => o && !(o in countries))
    if (missing.length === 0) return
    api.getOwnerProfiles(missing)
      .then(map => setCountries(prev => ({ ...prev, ...map })))
      .catch(() => {})
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [friends, results])

  // Debounced player search.
  useEffect(() => {
    const q = query.trim()
    if (q.length < MIN_QUERY) { setResults([]); return }
    let alive = true
    setSearching(true)
    const timer = setTimeout(() => {
      api.search(q, 12)
        .then(r => {
          if (!alive) return
          const owners = (r?.results?.owners || []).filter(x => x.kind === 'owner')
          setResults(owners)
        })
        .catch(() => { if (alive) setResults([]) })
        .finally(() => { if (alive) setSearching(false) })
    }, 250)
    return () => { alive = false; clearTimeout(timer) }
  }, [query])

  const friendsSet = useMemo(() => new Set(friends.map(f => f.toLowerCase())), [friends])

  const handleAdd = (target) => {
    if (!userOwner || !target) return
    api.addFriend(target, userOwner)
      .then(() => {
        setFriends(prev => prev.includes(target) ? prev : [...prev, target])
        toast.success(t('dka.t_friend_added', { name: target.toUpperCase() }))
      })
      .catch(() => toast.error(t('dka.t_add_friend_failed')))
  }

  const handleUnfriend = (target) => {
    if (!userOwner || !target) return
    api.removeFriend(target, userOwner)
      .then(() => {
        setFriends(prev => prev.filter(f => f !== target))
        toast.info(t('dka.t_unfriended', { name: target.toUpperCase() }))
      })
      .catch(() => toast.error(t('dka.t_unfriend_failed')))
  }

  const openProfile = (owner) => navigate(`/profile/${owner}`)

  if (!userOwner) {
    return (
      <>
        <Tape left={t('fnd.tape_no_owner')} mid={t('fnd.tape_set_name')} right="DOC HX-510" />
        <div style={{ padding: 36, textAlign: 'center' }}>
          <div className="t-md muted">{t('fnd.no_owner_body')}</div>
        </div>
      </>
    )
  }

  return (
    <>
      <Tape
        left={t('fnd.tape_left', { owner: userOwner.toUpperCase() })}
        mid={loading ? t('common.loading') : t('fnd.tape_connections', { count: friends.length })}
        right="DOC HX-510"
      />

      <div style={{ padding: 18, flex: 1, display: 'flex', flexDirection: 'column', gap: 14, overflow: 'auto' }}>
        {/* Find players */}
        <Panel
          code="FR.0"
          title={t('fnd.find_players')}
          right={<span className="t-xs muted">{searching ? t('dka.searching') : t('fnd.n_hits', { count: results.length })}</span>}
        >
          <div className="panel" style={{ padding: 0, marginBottom: 8, borderStyle: query ? 'solid' : 'dashed' }}>
            <input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t('fnd.search_ph')}
              style={{
                width: '100%',
                padding: '8px 12px',
                background: 'transparent',
                border: 'none',
                color: 'var(--ink)',
                fontFamily: 'inherit',
                fontSize: 11,
                letterSpacing: '0.06em',
                textTransform: 'uppercase',
                outline: 'none',
              }}
            />
          </div>
          {query.trim().length < MIN_QUERY ? (
            <div className="t-xs muted" style={{ padding: '6px 0' }}>
              &gt; {t('search.min_chars')}
            </div>
          ) : results.length === 0 && !searching ? (
            <div className="t-xs muted" style={{ padding: '6px 0' }}>
              {t('fnd.no_match')}
            </div>
          ) : (
            <div>
              {results.map(r => (
                <SearchHit
                  key={r.label}
                  slug={r.label}
                  isFriend={friendsSet.has(r.label.toLowerCase())}
                  isSelf={r.label.toLowerCase() === userOwner}
                  country={countries[r.label]}
                  onAdd={handleAdd}
                  navigate={navigate}
                />
              ))}
            </div>
          )}
        </Panel>

        {/* Friend list */}
        <Panel
          code="FR.1"
          title={t('fnd.my_friends', { count: friends.length })}
        >
          {loading ? (
            <div className="t-md muted" style={{ textAlign: 'center', padding: 18 }}>
              {t('fnd.loading')}<span className="blink">_</span>
            </div>
          ) : friends.length === 0 ? (
            <div className="t-md muted" style={{ textAlign: 'center', padding: 18 }}>
              {t('fnd.no_friends')}
            </div>
          ) : (
            <div style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))',
              gap: 14,
            }}>
              {friends.map(f => (
                <FriendTile
                  key={f}
                  owner={f}
                  country={countries[f]}
                  onUnfriend={handleUnfriend}
                  onOpen={openProfile}
                />
              ))}
            </div>
          )}
        </Panel>
      </div>
    </>
  )
}
