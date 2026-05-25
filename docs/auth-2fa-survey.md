# Auth: 2FA Survey + r60 Minimal Implementation

## Current auth model

HexDek's auth layer (`internal/auth`) is intentionally minimal:

- `device` table — registered user record (id + display_name, no credentials).
- `session` table — opaque bearer-token (96 hex chars, 30-day default TTL)
  tied to a device via foreign key.
- `POST /api/device/register` (in `internal/party/handler.go`) creates a
  device + issues a session token in one round-trip. **There is no
  password, no email verification, no challenge step.**
- Token transport: `Authorization: Bearer <token>` (preferred) or
  `?token=` query (WebSocket-friendly).
- `auth.Required` / `auth.RequiredFunc` middleware validates on
  protected endpoints.

So when we say "2FA" we have a definitional problem: **there is no
first factor today.** Device registration is a one-step token issuance
that anyone can hit anonymously. "Two factors" would imply we have one.

## Real scope of "2FA support"

A spec-compliant 2FA story requires all of:

1. **A first factor.** Today: implicit, because device-registration
   issues a token immediately. To meaningfully gate things, we'd need
   either (a) password-on-register + password-on-relogin, or (b) email
   magic-link verification, or (c) WebAuthn/passkey. Each is a major
   addition.

2. **TOTP secret per user.** A 160-bit shared secret (RFC 6238),
   base32-encoded, stored alongside the device row.

3. **Enrollment flow.** Generate secret → return `otpauth://` URI for
   QR rendering → user submits a sample 6-digit code to confirm before
   the secret is activated. Until confirmed, the secret must not gate
   auth.

4. **Verification at login** with ±30s clock-drift tolerance (RFC 6238
   §5.2 recommends a one-step window).

5. **Backup codes.** Single-use recovery codes for when the
   authenticator is lost. Without these, a lost phone bricks the
   account.

6. **Rate limiting on verify.** The TOTP keyspace is 10^6. Without a
   lockout-on-N-failures policy the brute-force takes seconds.

7. **Session elevation.** Sessions that pass 2FA should be marked
   "elevated" so sensitive operations (deck delete, account changes)
   can require the elevation — and re-prompt if the elevation has
   expired.

8. **Storage migration.** New `device_totp` table; ideally a
   `session.elevated_at` column for #7.

9. **Frontend.** Enrollment screen with QR rendering, verify prompt
   in the login flow, backup-code display + download, recovery flow.

## What this PR ships (minimal)

This PR is deliberately backend-only and avoids decisions that
require coordinated UX work:

| Piece | Status |
|---|---|
| `internal/auth/totp.go` — RFC 6238 TOTP algorithm (HMAC-SHA1, 6 digits, 30s window) | ✅ shipped |
| Secret generation (160-bit, base32) + `otpauth://` URI builder | ✅ shipped |
| `internal/auth/totp_lifecycle.go` — enroll / confirm / status / disable primitives backed by SQLite | ✅ shipped |
| `device_totp` table in `internal/db/schema.sql` | ✅ shipped |
| RFC 6238 test-vector regression | ✅ shipped |
| Lifecycle tests (enroll → confirm → status → disable round-trip + edge cases) | ✅ shipped |
| Survey doc (this file) | ✅ shipped |

## What this PR explicitly defers

| Piece | Reason for deferral |
|---|---|
| HTTP endpoints (`/api/2fa/enroll`, `/confirm`, `/verify`, `/status`, `/disable`) | Requires a UX decision: are these auth-gated behind the existing session, or gated behind a pre-2FA-elevated intermediate token? Answer affects API shape. |
| Rate-limit on verify (lockout after N failures) | The verify endpoint is what would be rate-limited; without that endpoint, the lockout has nowhere to attach. Follow-up bundles with the HTTP endpoints. |
| Backup codes | UX-coupled (display once, downloadable, single-use marker on use). Worth its own focused PR. |
| Session elevation (`session.elevated_at` column + middleware that requires elevation) | The bigger architectural lift. Depends on TOTP verify being wired into a login-style flow that the current device-register-issues-token model doesn't have. |
| QR rendering | Frontend concern. Backend returns the `otpauth://` URI; client renders. |
| Recovery flow (e.g. email confirmation to reset TOTP) | Depends on an email channel HexDek doesn't have today. |
| A first factor worth the name (password, magic link, WebAuthn) | Out of scope for "2FA"; needs its own product decision. |

## Threat-model honesty

Until the deferred pieces ship, the TOTP primitives in this PR are
**not yet a security boundary**:

- Anyone holding a device's session bearer-token already has full
  access to that device's data. Adding TOTP to that device without
  also adding an elevation gate doesn't raise the bar — it just gives
  the attacker an extra obstacle they can ignore.
- The primitives are useful *now* as the foundation a follow-up PR
  wires into a real elevation flow, and they're tested against the
  RFC vectors so the cryptographic substrate doesn't need to be
  re-audited at wiring time.
- The lifecycle helpers run inside the existing SQLite transaction
  model so multi-write enrollment can't half-succeed.

## Next steps (rough sequencing)

1. **This PR.** Algorithm + schema + lifecycle + survey.
2. **PR N+1.** HTTP endpoints + rate-limit-on-verify + the elevation
   column.
3. **PR N+2.** Backup codes + recovery flow scaffolding.
4. **PR N+3.** Frontend enrollment + verify UX.
5. **PR N+M.** Decide on, and implement, a real first factor.

Steps 2–3 can land before step 4 (curl-driven testing). Step 5 is
orthogonal but matters for the marketing claim "2FA": until there's
a real first factor, TOTP is "TFA without the first F."
