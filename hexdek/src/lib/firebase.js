import { initializeApp, getApps } from 'firebase/app'
import {
  getAuth,
  sendSignInLinkToEmail,
  isSignInWithEmailLink,
  signInWithEmailLink,
  onAuthStateChanged,
  signOut,
} from 'firebase/auth'

const firebaseConfig = {
  apiKey: import.meta.env.VITE_FIREBASE_API_KEY || 'AIzaSyBRdiLKuwYDGGj-g9LCB-QiZh9sGH6Y15I',
  authDomain: import.meta.env.VITE_FIREBASE_AUTH_DOMAIN || 'bluefroganalytics.firebaseapp.com',
  projectId: import.meta.env.VITE_FIREBASE_PROJECT_ID || 'bluefroganalytics',
  storageBucket: import.meta.env.VITE_FIREBASE_STORAGE_BUCKET || 'bluefroganalytics.firebasestorage.app',
  messagingSenderId: import.meta.env.VITE_FIREBASE_MESSAGING_SENDER_ID || '143686250562',
  appId: import.meta.env.VITE_FIREBASE_APP_ID || '1:143686250562:web:30bc8d246678539dc57ddb',
}

const app = getApps().length === 0 ? initializeApp(firebaseConfig) : getApps()[0]
const auth = getAuth(app)

export async function sendMagicLink(email) {
  const actionCodeSettings = {
    url: `${window.location.origin}/auth/callback`,
    handleCodeInApp: true,
  }
  await sendSignInLinkToEmail(auth, email, actionCodeSettings)
  window.localStorage.setItem('hexdek_email_for_signin', email)
}

// Error codes that mean "the email we supplied doesn't satisfy this
// link" — the shape a STALE stored email produces. Retrying with the
// right email can succeed. Deliberately excludes auth/invalid-action-code
// and auth/expired-action-code (the link itself is dead — no email fixes
// that).
const EMAIL_MISMATCH_CODES = new Set([
  'auth/invalid-email',
  'auth/user-mismatch',
  'auth/missing-email',
  'auth/argument-error',
])

export async function completeMagicLinkSignIn() {
  if (!isSignInWithEmailLink(auth, window.location.href)) return null

  // A wedged half-session from a prior attempt can shadow the new
  // sign-in (stale refresh token in IndexedDB persistence). Signing it
  // out first makes link completion start from a clean slate.
  if (auth.currentUser) {
    try { await signOut(auth) } catch { /* proceed regardless */ }
  }

  const stored = window.localStorage.getItem('hexdek_email_for_signin')
  let email = stored
  if (!email) {
    email = window.prompt('CONFIRM EMAIL FOR VERIFICATION:')
  }
  if (!email) return null
  email = email.trim()

  try {
    const result = await signInWithEmailLink(auth, email, window.location.href)
    window.localStorage.removeItem('hexdek_email_for_signin')
    return result.user
  } catch (err) {
    // THE stale-state wedge (works-only-in-private bug): the stored
    // email is only ever removed on SUCCESS, so one abandoned attempt
    // leaves a wrong email behind and every later link fails the
    // email<->link match in this profile forever — while private
    // windows (no stored email) prompt and succeed. Clear the stale
    // key unconditionally so failure is never sticky, and when the
    // failure is mismatch-shaped, re-prompt and retry once.
    window.localStorage.removeItem('hexdek_email_for_signin')
    if (stored && EMAIL_MISMATCH_CODES.has(err?.code)) {
      const confirmed = window.prompt(
        'STORED EMAIL DID NOT MATCH THIS LINK. CONFIRM THE EMAIL THE LINK WAS SENT TO:',
      )
      const retryEmail = confirmed?.trim()
      if (retryEmail && retryEmail.toLowerCase() !== stored.trim().toLowerCase()) {
        const result = await signInWithEmailLink(auth, retryEmail, window.location.href)
        return result.user
      }
    }
    throw err
  }
}

// resetAuthClientState — recovery hatch for corrupt client auth state.
// Clears everything the auth flow persists on this origin: our own
// localStorage keys, any firebase:* localStorage fallback entries, and
// the Firebase IndexedDB persistence database. Used by AuthCallback's
// RESET & RETRY path so users never need a manual storage clear.
export async function resetAuthClientState() {
  try { await signOut(auth) } catch { /* state may be too wedged to sign out */ }
  try {
    window.localStorage.removeItem('hexdek_email_for_signin')
    Object.keys(window.localStorage)
      .filter((k) => k.startsWith('firebase:'))
      .forEach((k) => window.localStorage.removeItem(k))
  } catch { /* storage may be unavailable */ }
  await new Promise((resolve) => {
    let settled = false
    const done = () => { if (!settled) { settled = true; resolve() } }
    try {
      const req = window.indexedDB.deleteDatabase('firebaseLocalStorageDb')
      req.onsuccess = done
      req.onerror = done
      req.onblocked = done
    } catch { done() }
    // Never hang the recovery path on a stuck deleteDatabase.
    setTimeout(done, 2000)
  })
}

export function onAuthChange(callback) {
  return onAuthStateChanged(auth, callback)
}

export async function signOutUser() {
  await signOut(auth)
}

export { auth }
