import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import { defineConfig, globalIgnores } from 'eslint/config'

// CI gate config — the BLOCKING lint ratchet for .github/workflows/
// frontend-ci.yml. The full eslint.config.js currently reports 160+
// pre-existing errors, so gating it outright would make CI red on debt
// nobody is fixing in this PR. Instead this config enables ONLY rules
// that are (a) clean on the current tree and (b) indicate runtime
// crashes, not style:
//
//   no-undef — the exact class that shipped the `pls` ReferenceError
//   (commit 55386105 deleted a binding and left four live reads;
//   DeckArchive + SharePreview died at render for weeks with nothing
//   gating it).
//
// Ratchet plan: as existing debt classes are paid down, promote rules
// from eslint.config.js into this gate — react-hooks/rules-of-hooks
// next (10 known conditional-hook sites as of r61: ManaCost, CardLink,
// CardPage, SharePreview, PublicProfile), then react-hooks/immutability
// (the rule that flags TDZ reads like the cardSearchOpen bug).
//
// The full config still runs in CI as a non-blocking visibility step.

export default defineConfig([
  globalIgnores(['dist', 'node_modules']),
  {
    files: ['src/**/*.{js,jsx,mjs}'],
    // The react-hooks plugin is registered (not enabled) so the inline
    // `eslint-disable-next-line react-hooks/...` comments in src/
    // resolve instead of erroring as unknown rules.
    plugins: { 'react-hooks': reactHooks },
    languageOptions: {
      globals: globals.browser,
      parserOptions: { ecmaFeatures: { jsx: true } },
    },
    rules: {
      'no-undef': 'error',
    },
  },
])
