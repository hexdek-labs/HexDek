const API_BASE = import.meta.env.VITE_API_URL ?? ''

async function request(path, opts = {}) {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...opts.headers },
    ...opts,
  })
  if (!res.ok) throw new Error(`API ${res.status}: ${path}`)
  return res.json()
}

export function cardArtUrl(name) {
  if (!name) return null
  const clean = name.split('//')[0].trim()
  return `${API_BASE}/api/card-art/${encodeURIComponent(clean)}`
}

export { API_BASE }

export const api = {
  getDecks: () => request('/api/decks'),
  getDeck: (id) => request(`/api/decks/${id}`),
  getDeckAnalysis: (id) => request(`/api/decks/${id}/analysis`),
  getProfile: () => request('/api/profile'),
  getGames: (limit = 20) => request(`/api/games?limit=${limit}`),
  getGame: (id) => request(`/api/games/${id}`),
  getGameReport: (id) => request(`/api/games/${id}/report`),
  getForgeStatus: () => request('/api/forge/status'),
  getForgeResults: (deckId) => request(`/api/forge/${deckId}/results`),
  startForge: (deckId, config) => request(`/api/forge/${deckId}/start`, { method: 'POST', body: JSON.stringify(config) }),
  getTournamentStats: () => request('/api/tournament/stats'),
  getLiveStats: () => request('/api/live/stats'),
  getLiveGame: () => request('/api/live/game'),
  getLiveELO: () => request('/api/live/elo'),
  importDeck: (name, owner, deckList) => request('/api/decks', {
    method: 'POST',
    body: JSON.stringify({ name, owner, deck_list: deckList }),
  }),
  runAnalysis: (id) => request(`/api/decks/${id}/analyze`, { method: 'POST' }),
  updateDeck: (id, deckList) => request(`/api/decks/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ deck_list: deckList }),
  }),
  deleteDeck: (id) => request(`/api/decks/${id}`, { method: 'DELETE' }),
  getDeckVersions: (id) => request(`/api/decks/${id}/versions`),
  startGauntlet: (id, games = 10000) => request(`/api/gauntlet/${id}?games=${games}`, { method: 'POST' }),
  getGauntlet: (id) => request(`/api/gauntlet/${id}`),
  getDonationsSummary: () => request('/api/donations/summary'),
}
