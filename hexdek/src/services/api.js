// import.meta.env is Vite-only; guard the read so this module loads
// cleanly under plain `node --test` for the unwrapApiError unit tests.
const API_BASE = (import.meta.env && import.meta.env.VITE_API_URL) ?? ''

function getOwnerSlug() {
  try {
    return localStorage.getItem('hexdek_owner') || ''
  } catch { return '' }
}

async function request(path, opts = {}) {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...opts.headers },
    ...opts,
  })
  if (!res.ok) {
    // hexapi's r60 unified ErrorResponse shape:
    //   {error: {code, message, details?}, request_id, status}
    // Pre-r60 shape (still on stale backends during deploy crossover):
    //   {error: "msg", status}
    // Pre-unified shape (third-party / not-yet-migrated routes):
    //   plain text or arbitrary JSON
    // unwrapApiError normalizes all three so consumers can always read
    // err.code / err.message / err.details / err.requestId without
    // re-parsing err.body.
    let body = ''
    try { body = await res.text() } catch { /* noop */ }
    throw unwrapApiError({ status: res.status, path, body })
  }
  return res.json()
}

// unwrapApiError normalizes any error body shape into a flat Error
// with code/message/details/requestId/status/body fields. Exported
// so direct fetch() callers (BugReport, MatchupsPanel, Meta, etc.)
// can opt into the same decoder if they want richer error info.
//
// Backward-compat is intentional: this layer is the deploy-crossover
// shim. Once every consumer has migrated to err.code / err.message,
// the legacy-string branch can be deleted — but for now, callers
// authored against the flat shape continue to work because the
// decoder upgrades the wire body to the rich shape regardless of
// which server version answered.
export function unwrapApiError({ status, path, body }) {
  let code = ''
  let message = (body || '').trim()
  let details = null
  let requestId
  try {
    const parsed = JSON.parse(body)
    if (parsed && typeof parsed === 'object') {
      if (typeof parsed.request_id === 'string') requestId = parsed.request_id
      const e = parsed.error
      if (e && typeof e === 'object') {
        // r60 nested envelope.
        if (typeof e.code === 'string') code = e.code
        if (typeof e.message === 'string') message = e.message
        if (e.details && typeof e.details === 'object') details = e.details
      } else if (typeof e === 'string') {
        // Pre-r60 flat envelope: hoist string error to message; also
        // expose it as code so existing switch-on-code consumers
        // (e.g. DeckArchive credits gate's "free_quota_exhausted"
        // string check) keep working against either server version.
        message = e
        code = e
        // Pre-r60 also merged extras at top level — surface them
        // under .details so the new-shape consumer path resolves.
        const extras = { ...parsed }
        delete extras.error
        delete extras.status
        delete extras.request_id
        if (Object.keys(extras).length > 0) details = extras
      }
    }
  } catch { /* not JSON — fall through with raw text as message */ }
  if (!message) message = `API ${status}: ${path}`
  const err = new Error(message)
  err.status = status
  err.code = code
  err.details = details
  err.requestId = requestId
  err.body = body
  return err
}

function authedRequest(path, opts = {}) {
  const owner = getOwnerSlug()
  return request(path, {
    ...opts,
    headers: { ...opts.headers, ...(owner ? { 'X-HexDek-Owner': owner } : {}) },
  })
}

export function cardArtUrl(name) {
  if (!name) return null
  const clean = name.split('//')[0].trim()
  return `${API_BASE}/api/card-art/${encodeURIComponent(clean)}`
}

export function cardImageUrl(name) {
  if (!name) return null
  const clean = name.split('//')[0].trim()
  return `${API_BASE}/api/card-art/${encodeURIComponent(clean)}?version=normal`
}

export { API_BASE }

export const api = {
  getDecks: (opts = {}) => {
    const params = new URLSearchParams()
    if (opts.owner) params.set('owner', opts.owner)
    if (opts.contains) params.set('contains', opts.contains)
    const qs = params.toString()
    return request(`/api/decks${qs ? `?${qs}` : ''}`)
  },
  getDeck: (id) => request(`/api/decks/${id}`),
  getDeckAnalysis: (id) => request(`/api/decks/${id}/analysis`),
  getProfile: () => request('/api/profile'),
  getGames: (limit = 20) => request(`/api/games?limit=${limit}`),
  getGame: (id) => request(`/api/games/${id}`),
  getGameReport: (id) => request(`/api/games/${id}/report`),
  getGameSummary: (id) => request(`/api/games/${id}/summary`),
  searchGameSummaries: ({ since, until, commander, deck, winner, limit, offset } = {}) => {
    const params = new URLSearchParams()
    if (since) params.set('since', String(since))
    if (until) params.set('until', String(until))
    if (commander) params.set('commander', commander)
    if (deck) params.set('deck', deck)
    if (winner) params.set('winner', winner)
    if (limit) params.set('limit', String(limit))
    if (offset) params.set('offset', String(offset))
    const qs = params.toString()
    return request(`/api/games/summaries${qs ? `?${qs}` : ''}`)
  },
  getLiveStats: () => request('/api/live/stats'),
  getLiveGame: () => request('/api/live/game'),
  getLiveELO: () => request('/api/live/elo'),
  importDeck: (name, owner, deckList, tags) => request('/api/decks', {
    method: 'POST',
    body: JSON.stringify({ name, owner, deck_list: deckList, ...(tags?.length ? { tags } : {}) }),
  }),
  // Full-page /import flow targets the dedicated alias route so the
  // backend can split metrics if we ever care to (same handler today).
  importDeckFull: ({ name, owner, deckList, tags }) => request('/api/decks/import', {
    method: 'POST',
    body: JSON.stringify({ name, owner, deck_list: deckList, ...(tags?.length ? { tags } : {}) }),
  }),
  importMoxfield: ({ url, owner, tags }) => request('/api/import/moxfield', {
    method: 'POST',
    body: JSON.stringify({ url, owner, ...(tags?.length ? { tags } : {}) }),
  }),
  importArchidekt: ({ url, owner, tags }) => request('/api/import/archidekt', {
    method: 'POST',
    body: JSON.stringify({ url, owner, ...(tags?.length ? { tags } : {}) }),
  }),
  // Tag autocomplete — returns [{tag, count}, ...] ranked by usage.
  // Owner defaults to the caller's X-HexDek-Owner (server-side) so the
  // suggestions are personal; pass owner: '*' to span every deck.
  getTagSuggestions: ({ q = '', owner, limit = 20 } = {}) => {
    const params = new URLSearchParams()
    if (q) params.set('q', q)
    if (owner) params.set('owner', owner)
    if (limit) params.set('limit', String(limit))
    const qs = params.toString()
    return authedRequest(`/api/tags${qs ? `?${qs}` : ''}`)
  },
  searchCards: (q, limit = 6) => request(`/api/cards/search?q=${encodeURIComponent(q)}&limit=${limit}`),
  runAnalysis: (id) => request(`/api/decks/${id}/analyze`, { method: 'POST' }),
  updateDeck: (id, deckList) => authedRequest(`/api/decks/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ deck_list: deckList }),
  }),
  deleteDeck: (id) => authedRequest(`/api/decks/${id}`, { method: 'DELETE' }),
  cloneDeck: (id) => authedRequest(`/api/decks/${id}/clone`, { method: 'POST' }),
  forkDeck: (id) => authedRequest(`/api/decks/${id}/fork`, { method: 'POST' }),
  archetypeFeedback: (id, action, archetype) => authedRequest(
    `/api/decks/${id}/archetype-feedback`,
    {
      method: 'POST',
      body: JSON.stringify({ action, ...(archetype ? { archetype } : {}) }),
    },
  ),
  patchDeck: (id, fields) => authedRequest(`/api/decks/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(fields),
  }),
  getDeckVersions: (id) => request(`/api/decks/${id}/versions`),
  getDeckVersion: (id, version) => request(`/api/decks/${id}/versions/${encodeURIComponent(version)}`),
  getDeckBudget: (id) => request(`/api/decks/${id}/budget`),
  getDeckCurse: (id) => request(`/api/decks/${id}/curse`),
  patchDeckCurse: (id, constraints) => authedRequest(`/api/decks/${id}/curse`, {
    method: 'PATCH',
    body: JSON.stringify({ constraints }),
  }),
  getSimilarDecks: (id, limit = 5) => request(`/api/decks/${id}/similar?limit=${limit}`),
  getAchievements: (owner) => request(`/api/achievements/${owner}`),
  setUserCountry: (owner) => request(`/api/user/profile/country`, {
    method: 'POST',
    body: JSON.stringify({ owner }),
  }),
  getOwnerProfile: (owner) => request(`/api/profile/${encodeURIComponent(owner)}`),
  getOwnerProfiles: (owners) => {
    const list = (owners || []).filter(Boolean).join(',')
    if (!list) return Promise.resolve({})
    return request(`/api/profiles?owners=${encodeURIComponent(list)}`)
  },
  getImports: (owner, limit = 10) => request(`/api/imports/${encodeURIComponent(owner)}?limit=${limit}`),
  // Gauntlet is now credit-gated when the caller is signed in. Send
  // the X-HexDek-Owner header so the server knows who to charge /
  // bill against the daily free-tier quota.
  startGauntlet: (id, games = 500) => authedRequest(`/api/gauntlet/${id}?games=${games}`, { method: 'POST' }),
  getGauntlet: (id) => request(`/api/gauntlet/${id}`),
  // SSE stream of gauntlet/tournament progress. Returns the EventSource
  // URL so callers can `new EventSource(api.tournamentEventsUrl(id))`.
  tournamentEventsUrl: (id) => `${API_BASE}/api/tournaments/${id}/events`,
  // Matchup matrix — per-deck head-to-head records (rich dataset beyond
  // the gauntlet result's TopBeaten/TopLostTo summary).
  getDeckMatchups: (id) => request(`/api/decks/${id}/matchups`),
  // ELO history — chronological list of completed gauntlet runs for the
  // deck. Returns oldest-first so the chart can plot the calibration arc.
  getDeckEloHistory: (id, limit = 20) => request(`/api/decks/${id}/elo-history?limit=${limit}`),
  // Aggregate card stats keyed by commander — broad signal, shared by
  // every deck for a given commander. Still powers the CARD STATS panel
  // (TOP PERFORMERS / UNDERPERFORMERS) which is commander-level by design.
  getCardStatsByCommander: (commander) => request(`/api/card-stats/${encodeURIComponent(commander)}`),
  // Per-deck card stats — intersects the cross-commander card_stats pool
  // with this deck's actual card list and ranks by win-rate-above-baseline.
  // Richer signal than the commander aggregate for the HOT CARDS widget;
  // server returns the cards pre-filtered and pre-sorted by delta.
  getDeckCardStats: (id) => request(`/api/deck-card-stats/${id}`),

  // Credit economy. All four require X-HexDek-Owner.
  getCreditBalance: () => authedRequest('/api/credits'),
  getCreditHistory: (limit = 50) => authedRequest(`/api/credits/history?limit=${limit}`),
  getCreditQuota: () => authedRequest('/api/credits/quota'),
  spendCredits: (amount, reason, reference) => authedRequest('/api/credits/spend', {
    method: 'POST',
    body: JSON.stringify({ amount, reason, reference }),
  }),
  getDonationsSummary: () => request('/api/donations/summary'),
  search: (q, limit = 6) => request(`/api/search?q=${encodeURIComponent(q)}&limit=${limit}`),
  listFriends: (asSlug) => request(`/api/friends?as=${encodeURIComponent(asSlug)}`),
  addFriend: (target, asSlug) => request(`/api/friends/${encodeURIComponent(target)}?as=${encodeURIComponent(asSlug)}`, { method: 'POST' }),
  removeFriend: (target, asSlug) => request(`/api/friends/${encodeURIComponent(target)}?as=${encodeURIComponent(asSlug)}`, { method: 'DELETE' }),
  getOwnerStats: (owner) => request(`/api/owner/${encodeURIComponent(owner)}/stats`),
  getOwnerGames: (owner, limit = 20) => request(`/api/owner/${encodeURIComponent(owner)}/games?limit=${limit}`),
  spawnSpectateRoom: (deckId) => request('/api/spectate/spawn', { method: 'POST', body: JSON.stringify({ deck_id: deckId }) }),
  getSpectateRoom: (roomId) => request(`/api/spectate/rooms/${encodeURIComponent(roomId)}`),
  listSpectateRooms: () => request('/api/spectate/rooms'),
  // BOINC distributed-compute credits — see internal/hexapi/contrib.go.
  // Returns 0/null fields for owners who haven't contributed yet.
  getContribCredits: (owner) => request(`/api/contrib/credits/${encodeURIComponent(owner)}`),

  // Live conviction-diagnostic ring buffer. Gated by HEXDEK_ADMIN_OWNER on
  // the server; the X-HexDek-Owner header is what the gate checks.
  getConvictionEvents: ({ since = 0, limit = 200, triggeredOnly = false } = {}) => {
    const params = new URLSearchParams()
    if (since) params.set('since', String(since))
    if (limit) params.set('limit', String(limit))
    if (triggeredOnly) params.set('triggered', '1')
    const qs = params.toString()
    return authedRequest(`/api/admin/conviction-events${qs ? `?${qs}` : ''}`)
  },
}
