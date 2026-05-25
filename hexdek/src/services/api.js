const API_BASE = import.meta.env.VITE_API_URL ?? ''

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
    // hexapi's unified ErrorResponse shape is {error, status} as JSON;
    // unwrap the message so the user sees "missing card name" rather
    // than the raw JSON envelope. Fall back to the raw text body for
    // older / third-party endpoints that still return plain text.
    let body = ''
    try { body = await res.text() } catch { /* noop */ }
    let message = body?.trim()
    try {
      const parsed = JSON.parse(body)
      if (parsed && typeof parsed.error === 'string') message = parsed.error
    } catch { /* not JSON — keep the raw text */ }
    const err = new Error(message || `API ${res.status}: ${path}`)
    err.status = res.status
    err.body = body
    throw err
  }
  return res.json()
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
  getCardByName: (name) => request(`/api/cards/${encodeURIComponent(name)}`),
  runAnalysis: (id) => request(`/api/decks/${id}/analyze`, { method: 'POST' }),
  updateDeck: (id, deckList) => authedRequest(`/api/decks/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ deck_list: deckList }),
  }),
  deleteDeck: (id) => authedRequest(`/api/decks/${id}`, { method: 'DELETE' }),
  cloneDeck: (id) => authedRequest(`/api/decks/${id}/clone`, { method: 'POST' }),
  patchDeck: (id, fields) => authedRequest(`/api/decks/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(fields),
  }),
  getDeckVersions: (id) => request(`/api/decks/${id}/versions`),
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
