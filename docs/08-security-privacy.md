# 08 — Security & Privacy

Bills are sensitive. They contain **full name, home address, reference/consumer numbers, sometimes CNIC numbers or phone numbers**, plus a household's financial patterns. Treat every bill document as PII.

## 1. Threat model (summary)

| Threat | Example | Control |
|---|---|---|
| Cross-household data access (IDOR) | Changing `/bills/{id}` to another user's bill | Every query is filtered by `household_id` from the session; repository-layer guard; tests for every endpoint |
| Account takeover | Credential stuffing | argon2id hashing, rate limits + lockout, optional TOTP 2FA (later), breached-password check |
| Document leakage | Public bucket URL shared | Private bucket; **signed URLs valid for 5 minutes**; random storage keys |
| Third-party exposure | Bill sent to an AI provider | Explicit household consent; privacy mode available; minimal data sent; provider list disclosed |
| Malicious upload | Polyglot file, oversized PDF | MIME sniffing and allowlist, size/page caps, server-side image re-encode, no execution, PDF parsed in the worker with timeouts |
| Retrieval abuse | Our server used to scrape someone else's bills | Retrieval only for the user's own bill profiles; per-profile and global rate limits |
| Session hijack | XSS stealing a token | HTTP-only, Secure, SameSite=Lax cookies; strict CSP; no tokens in localStorage |
| CSRF | Cross-site form post | SameSite cookies + CSRF token on state-changing requests |
| Insider / operator | Developer browsing data | Production DB access limited; audit of admin actions; no PII in logs |

## 2. Authentication & sessions
- Email + password (argon2id, min 10 characters, breached-password check), plus Google OAuth (PKCE).
- Server-side sessions (session ID in an HTTP-only cookie, stored hashed in the DB). Revocable, with "sign out everywhere".
- Email verification is required before inviting members.
- Rate limits: login 5/min per IP+email; password reset 3/hour.

## 3. Authorisation
- Role matrix (owner / editor / viewer), checked in a single middleware/policy layer.
- **Household scoping is mandatory at the data-access layer.** Repository functions *require* a `householdID` parameter; there is no unscoped query function. Optionally, Postgres Row-Level Security can be added as defence in depth.

## 4. Data protection
- TLS everywhere (HTTPS-only, HSTS).
- At rest: provider-managed encryption (Supabase Postgres and Storage).
- **Field-level encryption** (AES-GCM, key held in an environment secret) for `reference_number` display values and any identity-like fields. A **keyed hash** (HMAC) of the normalised reference number is used for matching.
- **Not stored:** customer name, CNIC and phone extracted from bills. They are used transiently for matching, then dropped. OCR text has CNIC-like patterns (`\d{5}-\d{7}-\d`) redacted before it is saved.
- Logs: structured, with no request bodies, no PII, and IDs only.

## 5. Third-party AI & OCR
- Household setting **"Use AI-assisted extraction"**, default **off**, with a plain-language notice:
  > "Bill images will be sent to Google (Gemini) to read the text. On the free tier, Google may use the content to improve its services. You can turn this off at any time; extraction will then use privacy mode."
- Privacy mode = OCR.space (text only, files not stored by the service) + our templates, or **on-device OCR** (Tesseract.js, nothing leaves the phone), or manual only (user's choice).
- Send only the derived, compressed image. Crop out the header area with the name and address where a template knows the layout (later improvement).

## 6. Retention & deletion
- Account deletion: soft delete with a 14-day grace period, then a hard delete of DB rows **and** storage objects (verified by a job).
- Backups expire after 30 days, so deleted data disappears from backups within 30 days.
- The user can delete any single document; extraction results tied to it are deleted too.

## 7. Compliance notes
- Pakistan's Personal Data Protection Bill: **check its current status before public launch.** Either way, follow its principles (consent, purpose limitation, data subject rights) and GDPR-style practice.
- Provider retrieval: follow each provider's terms of service, use public lookup pages only, identify our client honestly (no browser spoofing to evade blocks), and stop if asked.

## 8. Security checklist per milestone
- [ ] M1: auth, sessions, CSRF, household scoping tests, rate limits, secret management, dependency scanning (Dependabot)
- [ ] M2: upload validation ✓, signed URLs ✓, AI consent ✓, reference-number encryption ✓, redaction (phase 2)
- [ ] M4: export access control (owner/editor only), export links expire
- [ ] M6: connector sandboxing (timeouts, response size caps), outbound allowlist
