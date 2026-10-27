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
import zh from './locales/zh.json'
import es from './locales/es.json'
import hi from './locales/hi.json'
import ar from './locales/ar.json'
import pt from './locales/pt.json'
import ru from './locales/ru.json'
import ja from './locales/ja.json'
import de from './locales/de.json'
import fr from './locales/fr.json'
import ko from './locales/ko.json'
import it from './locales/it.json'
import tr from './locales/tr.json'
import pl from './locales/pl.json'
import nl from './locales/nl.json'
import id from './locales/id.json'
import vi from './locales/vi.json'
import th from './locales/th.json'
import uk from './locales/uk.json'
import sv from './locales/sv.json'
import cs from './locales/cs.json'
import el from './locales/el.json'
import ro from './locales/ro.json'
import hu from './locales/hu.json'
import da from './locales/da.json'

// Add new languages here. Keep keys ISO 639-1 (en, de, ja, ...).
// Missing keys in any catalog fall through to English, then to the
// literal key string. Card names always stay English (Scryfall
// localization is a separate step).
// Ordered roughly by global reach; order here drives the picker.
const LOCALES = {
  en, zh, es, hi, ar, pt, ru, ja, de, fr, ko, it, tr,
  pl, nl, id, vi, th, uk, sv, cs, el, ro, hu, da,
}

// Locales written right-to-left. The <html dir> attribute follows the
// active locale so the whole shell mirrors, not just the text runs.
const RTL = new Set(['ar'])
const FALLBACK = 'en'
const STORAGE_KEY = 'hexdek.locale'

// Display-name for each locale, used by the in-app language picker.
// Self-rendered (the user reads them in their own script).
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
  if (!LOCALES[locale] || locale === currentLocale) return
  currentLocale = locale
  try { localStorage.setItem(STORAGE_KEY, locale) } catch {}
  applyDocumentLocale(locale)
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

// Set <html lang>/<html dir> for the locale resolved at module load.
applyDocumentLocale(currentLocale)
