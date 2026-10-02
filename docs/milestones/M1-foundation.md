# M1 — Foundation: delivery notes

Branch `m1-foundation`. Exit criteria from [09-roadmap](../09-roadmap.md) M1.

## Delivered
| Area | What's in | Requirements |
|---|---|---|
| Auth | Email + password sign-up and sign-in (argon2id); server-side sessions in HTTP-only cookies with sliding 30-day expiry; sign out; sign out everywhere; password reset by email (one-hour, single-use tokens); account deletion with a 14-day grace period, purged by the tick; rate limits; double-submit CSRF | FR-AUTH-01, 03, 04, 07; FR-ADM-02 |
| Household | Created at sign-up with owner membership, default property "My Home" and the default **General expenses** profile; settings for name, currency and timezone; role checks (viewer is read-only, owner-only settings) | FR-AUTH-04/06, FR-ADM-01, FR-ACCT-13 |
| Types & providers | 18 system types + 4 Maintenance sub-categories; custom types (create, edit, archive, one level of sub-categories); Pakistan provider catalogue (11 electricity, 2 gas, 3 water, 7 internet/mobile) + custom providers | FR-ACCT-01/02, FR-ADM-03 |
| **Bill profiles** | Unlimited named profiles per type; three kinds (metered, recurring, expense log) with per-kind fields; reference-number validation (LESCO-format DISCOs); unique names and references; colours from the validated palette; pin and reorder; archive and restore; kind locked once bills exist | FR-ACCT-03…11 |
| Bills | Manual bills and expense entries; one bill per profile per month with the duplicate pointing to the existing bill; arrears, adjustments, payable within and after the due date; archive, restore, permanent delete (archived only); full audit history with field-level changes | FR-BILL-01/02/04/05/06/07, FR-RECO-01 (manual) |
| Payments | "Already paid" on create, "Mark as paid", partial payments, delete; the late amount applies after the due date; a bill paid on time stays paid | FR-PAY-01/02/04/05/06 (receipts in M3) |
| Status | Derived **on read** from stored facts (`bill_status()` in SQL), so it is never stale; tested for on-time, late, partial and overdue cases | FR-BILL-07 |
| Home | Month summary (spend excludes arrears, FR-ANLT-06), overdue and due within 14 days, pinned profiles, recent bills | early part of FR-DASH |
| Web app | React + TypeScript PWA shell; design-system tokens (light and dark); bottom nav on phones and sidebar on desktop; Profiles; **profile dashboard** (Overview / Bills / Settings); bill form, detail and list with filters; onboarding; settings; all strings in i18n; pages load on demand | FR-ACCT-10, docs/10 |
| Ops | Render blueprint, Dockerfile, Cloudflare Pages proxy function and security headers, CI, encrypted daily backup, [deploy runbook](../deploy.md) | NFR-07/13/15/16 |

## Quality
- Go: 22 integration tests against real Postgres (auth, CSRF, roles, household isolation, every status transition, the arrears spend rule, pagination, archive/restore, the purge job) plus unit tests. API package coverage **74%**.
- Web: 16 unit tests (money parsing/formatting, status tones), strict TypeScript, ESLint clean.
- End to end (Playwright, phone and desktop): sign up → onboarding → add bill → mark paid → bill history → quick expense → sign out / sign in.

## Decisions made during M1
- **Bill status is derived on read**, not stored. This removes a whole class of stale-status bugs; docs 02 and 04 are updated.
- **Same-origin API through a Cloudflare Pages Function**, so cookies stay first-party (third-party cookies are blocked by Safari and others).
- **The 10-minute tick runs on cron-job.org**, because GitHub Actions' private-repo minutes can't cover it.
- **In M1 the bottom nav is Home · Profiles · ＋ · Bills · Settings.** Analytics replaces Bills when it ships in M4.

## Deferred (with reason)
| Item | Moves to |
|---|---|
| Google sign-in (FR-AUTH-02, Should) | M2, needs a Google OAuth client set up |
| Inviting members (FR-AUTH-05, Should) | M2, needs real email (Resend) configured |
| Breached-password check, 2FA | Later security pass |
| Field-level encryption of reference numbers (08 §4) | M2, together with document storage and redaction |
| Meter readings, charts, receipts | M3, as planned |
| Live deployment | Needs your Supabase, Render and Cloudflare accounts. Follow [deploy.md](../deploy.md) |

## Run it locally
See the [README](../../README.md#local-development).
