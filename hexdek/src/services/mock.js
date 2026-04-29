export const MOCK_PROFILE = {
  username: 'WIEDEMAN',
  userId: 'USR.0047',
  joined: '04.2026',
  elo: 1847,
  eloChange: +23,
  tier: 'PLATINUM-II',
  streak: '7 W',
  primaryColor: 'MONO-B',
  archetype: '"THE EXECUTIONER"',
  percentile: 'TOP 12%',
  gamesPlayed: 247,
  winRate: 31.0,
  avgWinTurn: 7.4,
  skills: [
    { name: 'WRATH TIMING', value: 92 },
    { name: 'TUTOR TARGETS', value: 64 },
    { name: 'MANA SEQUENCING', value: 31 },
    { name: 'POLITICS', value: 24 },
    { name: 'COMBO LINES', value: 78 },
  ],
  achievements: [
    { icon: '◆', name: 'FIRST BLOOD', unlocked: true },
    { icon: '◆', name: 'GIANT KILLER · BEAT B5 AI', unlocked: true },
    { icon: '◆', name: 'SURGEON · 10× 90% OPTIMAL', unlocked: true },
    { icon: '◆', name: 'ARCHENEMY · 1V3 WIN', unlocked: true },
    { icon: '◆', name: 'READER · 5× CORRECT READ', unlocked: true },
    { icon: '◆', name: 'ON FIRE · 7-WIN STREAK', unlocked: true },
    { icon: '◇', name: 'PERFECTIONIST · B5 100% OPT', unlocked: false },
    { icon: '◇', name: 'METAL · 30-WIN STREAK', unlocked: false },
  ],
}

export const MOCK_DECKS = [
  {
    id: 'tinybones',
    slot: '04',
    name: 'TINYBONES, THE PICKPOCKET',
    color: 'MONO-B',
    power: 7.4,
    bracket: 3,
    winRate: 34,
    archetype: 'COMBO-CTRL',
    cardCount: 99,
    avgCmc: 2.8,
    manaCurve: [2, 12, 22, 18, 14, 8, 5, 3],
  },
  {
    id: 'sen-triplets',
    slot: '03',
    name: 'SEN TRIPLETS',
    color: 'WUB',
    power: 6.8,
    bracket: 3,
    winRate: 28,
    archetype: 'THEFT-CTRL',
    cardCount: 99,
    avgCmc: 3.1,
    manaCurve: [1, 8, 18, 22, 16, 12, 6, 4],
  },
  {
    id: 'krrik',
    slot: '02',
    name: "K'RRIK, SON OF YAWGMOTH",
    color: 'MONO-B',
    power: 8.1,
    bracket: 4,
    winRate: 41,
    archetype: 'STORM',
    gold: true,
    cardCount: 99,
    avgCmc: 2.4,
    manaCurve: [4, 16, 24, 16, 10, 6, 4, 2],
  },
]

export const MOCK_GAMES = [
  { id: 'G.247', deck: "K'RRIK", opponent: 'VS 3× B4 BOTS', result: 'WIN', detail: 'T6 / COMBO KILL', time: '2H AGO', kind: 'ok' },
  { id: 'G.246', deck: 'TINYBONES', opponent: 'VS 3× B3 BOTS', result: 'LOSS', detail: 'T9 / OVERRUN', time: '5H AGO', kind: 'bad' },
  { id: 'G.245', deck: 'SEN TRIPS', opponent: 'VS HUMANS LOBBY', result: 'WIN', detail: 'T11 / THEFT LOCK', time: '1D AGO', kind: 'ok' },
  { id: 'G.244', deck: "K'RRIK", opponent: 'VS 1× B5 + 2× B3', result: 'LOSS', detail: 'T5 / STAX LOCK', time: '2D AGO', kind: 'bad' },
  { id: 'G.243', deck: 'TINYBONES', opponent: 'VS 3× B2 PRECON', result: 'WIN', detail: 'T8 / DRAIN LOOP', time: '2D AGO', kind: 'ok' },
]

export const MOCK_MATCHUPS = [
  { deck: 'TINY', aggro: 38, ctrl: 31, combo: 28, stax: 19 },
  { deck: 'SEN', aggro: 22, ctrl: 41, combo: 33, stax: 27 },
  { deck: 'KRRK', aggro: 44, ctrl: 29, combo: 51, stax: 12 },
]

export const MOCK_LIVE_STATS = {
  activeForges: 1247,
  totalGames: 2847391024,
  gamesPerMin: 38210,
  userContrib: 71402,
  userRank: '4,217 / 184K',
  throughput: [34, 52, 46, 71, 63, 80, 58, 72, 69, 84, 77, 90, 82, 68, 74, 86, 94, 88, 76, 82, 90, 84, 78, 72],
}

export const MOCK_DECK_ANALYSIS = {
  tinybones: {
    archetype: 'COMBO-CONTROL',
    powerScore: 7.4,
    b5Optimal: 8.9,
    delta: -1.5,
    intent: 'DRAIN OPPONENTS VIA DISCARD TRIGGERS.',
    finish: 'SANGUINE BOND + EXQUISITE BLOOD LOOP.',
    fallback: 'TORMENT OF HAILFIRE @ X≥10.',
    combos: [
      { symbol: 'α', type: 'INFINITE', cards: 'SANGUINE BOND + EXQUISITE BLOOD', desc: '"EITHER PIECE + ANY LIFE GAIN = INFINITE LOOP"', kind: 'bad' },
      { symbol: 'β', type: 'DETERMINED', cards: 'GRAY MERCHANT + PHYREXIAN RECLAMATION', desc: '"REPEATABLE DRAIN, TERMINATES AT OPP LIFE"', kind: 'ok' },
      { symbol: 'γ', type: 'FINISHER', cards: 'TORMENT OF HAILFIRE (X=10+)', desc: '"ONLINE T7+ WITH CABAL COFFERS DEVOTION"', kind: 'warn' },
      { symbol: 'δ', type: 'SYNERGY', cards: 'TINYBONES + WASTE NOT', desc: '"EACH OPP DISCARD = DRAW + MANA + ZOMBIE"', kind: null },
    ],
    benchmarkYour: { winRate: '34%', tAvg: '7.2', consistency: '0.69', mullRate: '18%' },
    benchmarkB5: { winRate: '41%', tAvg: '6.0', consistency: '0.82', mullRate: '11%' },
    categories: { instants: 12, sorceries: 8, enchantments: 9, artifacts: 11, lands: 33, planeswalkers: 2, tokens: 4, sideboard: 0 },
    kvStats: { avgCmc: '2.8', lands: '33 ⚠', ramp: '11 ✓', draw: '14 ✓', removal: '8', tutors: '6 ✓', interaction: '12' },
  },
}
