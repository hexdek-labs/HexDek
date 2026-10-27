// i18n foundation for hexdek.dev.
//
// Locale catalogs are JSON files under src/locales/. Add a new language
// by dropping a new file (e.g. src/locales/sw.json), adding a LOADERS
// entry for it, and giving it a LOCALE_NAMES display name.
//
// Usage from a component:
//
//   import { useTranslation } from '../i18n'
//   const { t, locale, setLocale, availableLocales } = useTranslation()
//   return <span>{t('nav.decks')}</span>
//
// Keys are dotted paths into the JSON (`nav.decks` → catalog.nav.decks).
// Missing keys fall back to English, then to the literal key string.
// {var} placeholders are interpolated from the second argument:
//
//   t('hello', { name: 'wiedeman' })  // "Hello, wiedeman"
//
// LOADING MODEL. English is bundled statically because it is also the
// fallback for every missing key, so it must be present synchronously.
// Every other catalog is a dynamic import, fetched only when that locale
// becomes active. With 25 languages, bundling them all would ship 24
// catalogs no visitor reads. Until the active catalog arrives, t() serves
// English and re-renders when it lands — a first paint in the fallback
// language, not a blank or a key string.

import { useSyncExternalStore } from 'react'

import en from './locales/en.json'

// Dynamic catalogs. Vite code-splits each of these into its own chunk.
// Ordered roughly by global reach; this order drives the picker.
const LOADERS = {
  zh: () => import('./locales/zh.json'),
  es: () => import('./locales/es.json'),
  hi: () => import('./locales/hi.json'),
  ar: () => import('./locales/ar.json'),
  pt: () => import('./locales/pt.json'),
  ru: () => import('./locales/ru.json'),
  ja: () => import('./locales/ja.json'),
  de: () => import('./locales/de.json'),
  fr: () => import('./locales/fr.json'),
  ko: () => import('./locales/ko.json'),
  it: () => import('./locales/it.json'),
  tr: () => import('./locales/tr.json'),
  pl: () => import('./locales/pl.json'),
  nl: () => import('./locales/nl.json'),
  id: () => import('./locales/id.json'),
  vi: () => import('./locales/vi.json'),
  th: () => import('./locales/th.json'),
  uk: () => import('./locales/uk.json'),
  sv: () => import('./locales/sv.json'),
  cs: () => import('./locales/cs.json'),
  el: () => import('./locales/el.json'),
  ro: () => import('./locales/ro.json'),
  hu: () => import('./locales/hu.json'),
  da: () => import('./locales/da.json'),
}

const FALLBACK = 'en'
const STORAGE_KEY = 'hexdek.locale'

// Catalogs resolved so far. English is always here; others arrive async.
const loaded = { en }

// Locales written right-to-left. The <html dir> attribute follows the
// active locale so the whole shell mirrors, not just the text runs.
const RTL = new Set(['ar'])

// Display-name for each locale, used by the in-app language picker.
// Self-rendered (the user reads them in their own script). Static on
// purpose — the picker must list every language without downloading
// 25 catalogs to find out what they are called.
export const LOCALE_NAMES = {
  en: 'English',
  zh: '中文',
  es: 'Español',
  hi: 'हिन्दी',
  ar: 'العربية',
  pt: 'Português',
  ru: 'Русский',
  ja: '日本語',
  de: 'Deutsch',
  fr: 'Français',
  ko: '한국어',
  it: 'Italiano',
  tr: 'Türkçe',
  pl: 'Polski',
  nl: 'Nederlands',
  id: 'Bahasa Indonesia',
  vi: 'Tiếng Việt',
  th: 'ไทย',
  uk: 'Українська',
  sv: 'Svenska',
  cs: 'Čeština',
  el: 'Ελληνικά',
  ro: 'Română',
  hu: 'Magyar',
  da: 'Dansk',
}

function isKnown(locale) {
  return locale === FALLBACK || Object.prototype.hasOwnProperty.call(LOADERS, locale)
}

// initialLocale resolves a locale at module load time, in priority order:
//   1. ?lang=<code> in window.location.search   (per-link sharing)
//   2. localStorage[hexdek.locale]              (sticky user choice)
//   3. navigator.language prefix match          (first-visit best guess)
//   4. FALLBACK ('en')
function initialLocale() {
  if (typeof window !== 'undefined') {
    try {
      const fromURL = new URLSearchParams(window.location.search).get('lang')
      if (fromURL && isKnown(fromURL)) return fromURL
    } catch {}
  }
  try {
    const saved = typeof localStorage !== 'undefined' && localStorage.getItem(STORAGE_KEY)
    if (saved && isKnown(saved)) return saved
  } catch {}
  if (typeof navigator !== 'undefined' && typeof navigator.language === 'string') {
    const prefix = navigator.language.toLowerCase().split('-')[0]
    if (isKnown(prefix)) return prefix
  }
  return FALLBACK
}

let currentLocale = initialLocale()

// Bumped whenever a catalog finishes loading, so a component that
// rendered in the English fallback re-renders once its real catalog
// arrives. The store snapshot combines it with the locale.
let catalogVersion = 0

const subscribers = new Set()

function notify() {
  for (const fn of subscribers) fn()
}

function subscribe(fn) {
  subscribers.add(fn)
  return () => subscribers.delete(fn)
}

// Snapshot must be a primitive that changes on BOTH axes React cares
// about: which locale is active, and whether its catalog has landed.
function getSnapshot() {
  return currentLocale + ':' + catalogVersion
}

// In-flight loads, so two components switching at once share one fetch.
const pending = new Map()

// ensureLocale loads a catalog if it isn't already resolved. A failed
// import is swallowed deliberately: the locale stays on the English
// fallback and the app keeps working rather than blanking on a chunk
// that 404'd behind a stale deploy.
function ensureLocale(locale) {
  if (loaded[locale] || !LOADERS[locale]) return Promise.resolve()
  if (pending.has(locale)) return pending.get(locale)
  const p = LOADERS[locale]()
    .then(mod => {
      loaded[locale] = mod.default || mod
      catalogVersion += 1
      notify()
    })
    .catch(() => {})
    .finally(() => { pending.delete(locale) })
  pending.set(locale, p)
  return p
}

// isRTL reports whether a locale is written right-to-left.
export function isRTL(locale) {
  return RTL.has(locale || currentLocale)
}

// applyDocumentLocale mirrors the active locale onto <html lang> and
// <html dir>. Called on every locale change and once at module load, so
// an RTL locale flips the whole shell rather than only its text runs.
function applyDocumentLocale(locale) {
  if (typeof document === 'undefined') return
  try {
    document.documentElement.setAttribute('lang', locale)
    document.documentElement.setAttribute('dir', RTL.has(locale) ? 'rtl' : 'ltr')
  } catch {}
}

export function setLocale(locale) {
  if (!isKnown(locale) || locale === currentLocale) return
  currentLocale = locale
  try { localStorage.setItem(STORAGE_KEY, locale) } catch {}
  applyDocumentLocale(locale)
  notify()
  ensureLocale(locale)
}

export function getLocale() {
  return currentLocale
}

export function availableLocales() {
  return [FALLBACK, ...Object.keys(LOADERS)]
}

// localeReady reports whether the active locale's catalog has landed.
// Useful for a caller that would rather wait than flash English.
export function localeReady(locale) {
  return Boolean(loaded[locale || currentLocale])
}

// lookup walks a dotted path through a catalog object. Returns undefined
// for any missing segment.
function lookup(catalog, key) {
  const parts = key.split('.')
  let cursor = catalog
  for (const part of parts) {
    if (cursor == null || typeof cursor !== 'object') return undefined
    cursor = cursor[part]
  }
  return cursor
}

// interpolate replaces {var} tokens with values from vars. Tokens
// without a matching var are left as-is so missing data is visible.
function interpolate(str, vars) {
  if (!vars || typeof str !== 'string') return str
  return str.replace(/\{(\w+)\}/g, (m, name) => (vars[name] != null ? String(vars[name]) : m))
}

// t resolves a key against the current locale, falling back to English
// then to the raw key. Module-level export for use outside React.
export function t(key, vars) {
  const catalog = loaded[currentLocale] || loaded[FALLBACK]
  let value = lookup(catalog, key)
  if (value === undefined && currentLocale !== FALLBACK) {
    value = lookup(loaded[FALLBACK], key)
  }
  if (value === undefined) return key
  return interpolate(value, vars)
}

// useTranslation subscribes the component to locale changes and
// returns the translator + setter. No Provider required.
export function useTranslation() {
  useSyncExternalStore(subscribe, getSnapshot, getSnapshot)
  return {
    t: (key, vars) => t(key, vars),
    locale: currentLocale,
    setLocale,
    availableLocales: availableLocales(),
  }
}

// useT is a lightweight wrapper that returns just the translator —
// most callers don't need the locale or setter and can avoid the
// destructure noise.
//
//   const t = useT()
//   <span>{t('nav.decks')}</span>
//
// Re-renders on locale change via the same useSyncExternalStore
// subscription as useTranslation.
export function useT() {
  useSyncExternalStore(subscribe, getSnapshot, getSnapshot)
  return t
}

// Resolve the startup locale: set <html lang>/<html dir> and fetch its
// catalog. Until it lands, t() serves English.
applyDocumentLocale(currentLocale)
ensureLocale(currentLocale)
