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

export async function completeMagicLinkSignIn() {
  if (!isSignInWithEmailLink(auth, window.location.href)) return null
  let email = window.localStorage.getItem('hexdek_email_for_signin')
  if (!email) {
    email = window.prompt('CONFIRM EMAIL FOR VERIFICATION:')
  }
  if (!email) return null
  const result = await signInWithEmailLink(auth, email, window.location.href)
  window.localStorage.removeItem('hexdek_email_for_signin')
  return result.user
}

export function onAuthChange(callback) {
  return onAuthStateChanged(auth, callback)
}

export async function signOutUser() {
  await signOut(auth)
}

export { auth }
