// i18n foundation for hexdek.dev.
//
// Locale catalogs are JSON files under src/locales/. Add a new language
// by dropping a new file (e.g. src/locales/de.json) and importing it
// into the LOCALES map below.
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

import { useSyncExternalStore } from 'react'

import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import de from './locales/de.json'
import fr from './locales/fr.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'
import pt from './locales/pt.json'

// Add new languages here. Keep keys ISO 639-1 (en, de, ja, ...).
// Missing keys in any catalog fall through to English, then to the
// literal key string. Card names always stay English (Scryfall
// localization is a separate step).
const LOCALES = { en, es, ja, de, fr, ko, zh, pt }
const FALLBACK = 'en'
const STORAGE_KEY = 'hexdek.locale'

// Display-name for each locale, used by the in-app language picker.
// Self-rendered (the user reads them in their own script).
export const LOCALE_NAMES = {
  en: 'English',
  es: 'Español',
  ja: '日本語',
  de: 'Deutsch',
  fr: 'Français',
  ko: '한국어',
  zh: '中文',
  pt: 'Português',
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
      if (fromURL && LOCALES[fromURL]) return fromURL
    } catch {}
  }
  try {
    const saved = typeof localStorage !== 'undefined' && localStorage.getItem(STORAGE_KEY)
    if (saved && LOCALES[saved]) return saved
  } catch {}
  if (typeof navigator !== 'undefined' && typeof navigator.language === 'string') {
    const prefix = navigator.language.toLowerCase().split('-')[0]
    if (LOCALES[prefix]) return prefix
  }
  return FALLBACK
}

let currentLocale = initialLocale()

const subscribers = new Set()

function notify() {
  for (const fn of subscribers) fn()
}

function subscribe(fn) {
  subscribers.add(fn)
  return () => subscribers.delete(fn)
}

function getSnapshot() {
  return currentLocale
}

export function setLocale(locale) {
  if (!LOCALES[locale] || locale === currentLocale) return
  currentLocale = locale
  try { localStorage.setItem(STORAGE_KEY, locale) } catch {}
  notify()
}

export function getLocale() {
  return currentLocale
}

export function availableLocales() {
  return Object.keys(LOCALES)
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
  const catalog = LOCALES[currentLocale] || LOCALES[FALLBACK]
  let value = lookup(catalog, key)
  if (value === undefined && currentLocale !== FALLBACK) {
    value = lookup(LOCALES[FALLBACK], key)
  }
  if (value === undefined) return key
  return interpolate(value, vars)
}

// useTranslation subscribes the component to locale changes and
// returns the translator + setter. No Provider required.
export function useTranslation() {
  const locale = useSyncExternalStore(subscribe, getSnapshot, getSnapshot)
  return {
    t: (key, vars) => t(key, vars),
    locale,
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
