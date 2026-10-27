// hexdek/src/i18n/config.js — locale configuration shim.
//
// The actual i18n machinery lives in ../i18n.js (single-file by
// historical accident; predates the "scaffolding spec" naming). This
// module re-exports the public locale-config surface for callers who
// want to import strictly from `../i18n/config`:
//
//   import { LOCALES, LOCALE_NAMES, FALLBACK_LOCALE } from '../i18n/config'
//
// `LOCALES` is the list of ISO-639-1 codes that have a JSON catalog
// loaded; `LOCALE_NAMES` is a code → self-rendered-display-name map
// (so each language renders in its own script in the picker).
//
// Card names stay English regardless of locale — Scryfall localization
// is a separate concern and lives outside this module.

import { availableLocales, LOCALE_NAMES } from '../i18n.js'

export const LOCALES = availableLocales()
export const FALLBACK_LOCALE = 'en'
export { LOCALE_NAMES }

// Re-export the hooks + setter so consumers can grab everything from
// one path if they prefer. The original ../i18n.js exports are still
// the canonical source of truth.
export { useT, useTranslation, setLocale, getLocale, t, isRTL } from '../i18n.js'
