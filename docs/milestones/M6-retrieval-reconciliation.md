# M6 — Official retrieval & full reconciliation: progress notes

Branch `m6-retrieval-reconciliation` (PR #8 → `main`). Exit criteria from [09-roadmap](../09-roadmap.md) M6: a retrieved bill and a scanned bill of the same
month merge into one record; when a connector is broken, nothing else is affected.

| Phase | Scope | State |
|---|---|---|
| 1 — Reconciliation | Sources per bill, field conflicts, resolution, side-by-side sources, undo merge | **Done** |
| 2 — Official retrieval | Assisted retrieval for providers with CAPTCHAs; connector framework (throttled, health-tracked, contained); scheduler; Fetch now | **Done** |
| 3 — Exit | Exit checks end to end, docs, public lookup page decision | Next |

## Phase 2 — Official retrieval (delivered)
**What we found:** the official lookup pages for the PITC electricity companies (LESCO, HESCO, IESCO…) and SSGC use a CAPTCHA
and anti-forgery tokens; SNGPL's site didn't respond from here. A CAPTCHA means the provider doesn't want automated access, and
ADR-002 / docs/08 rule out getting around it. So no automatic connector fetches from these sites.

| Area | What's in | Requirements |
|---|---|---|
| Assisted retrieval | Profiles whose provider has an official lookup page show **Official bill**: **Open the official bill page** (opens the provider's own page with the reference number copied), the person solves the security check and saves the bill as a PDF, then **Upload the official bill**. It is read like any upload and recorded as an **official** copy, merging with any copy of the same month. Lookup pages are stored per provider (verified links only: the PITC companies, K-Electric, SSGC). | ADR-002, FR-RETR-02 |
| Connector framework | `internal/retrieval`: a connector only fetches the document; the app reads, validates and reconciles it like any copy (FR-RETR-04). A per-connector throttle (6 a minute), a timeout, and panic recovery keep a bad connector contained. | FR-RETR-02/04 |
| Health | Each failure is counted; after 5 in a row a connector is paused for 24 h (the profile says so and Fetch now is disabled), then tried again. "No bill yet" is not a failure. Connector health is listed at `GET /retrieval/connectors`. | FR-RETR-05 |
| Scheduler | The tick checks opted-in profiles at most once a day each, only while this month's bill is missing and from two days before the usual issue day. | FR-RETR-03 |
| Fetch now | On profiles with a connector: a switch for automatic checks, **Fetch now** (at most every 15 minutes), and the last result. | FR-RETR-01/06 |
| Results | A fetched bill for a new month is created unverified ("check it") with its official PDF and an official source, and everyone gets a "fetched from the official site" notification (also pushed); for an existing month it merges, conflicts and all. The same PDF twice is recognised. Every attempt is logged (`retrieval_runs`). | FR-RETR-04, FR-REM-06 |
| Sample connector | Development and tests only (never production): serves the masked HESCO sample for its reference number, with no network requests. It proves the pipeline; production has no automatic connectors until a provider permits one. | — |

Quality: Go tests for Fetch now (created, draft, official source and document, notification, throttle, same document
unchanged, unknown reference = not found), retrieved + typed-in bill of the same month merging into one (exit criterion), a
failing connector paused after 5 attempts while bills, Home and the tick keep working, a panicking connector contained (exit
criterion), assisted providers (lookup link, Fetch now refused, official uploads), the daily scheduler and the on/off switch;
end-to-end tests for Fetch now merging on the bill page, and the assisted flow (provider page intercepted, official PDF upload).

## Phase 1 — Reconciliation (delivered)
| Area | What's in | Requirements |
|---|---|---|
| Sources | Every bill records each channel that produced or confirmed it (typed in, scan, uploaded file, automatic, and official in phase 2) with the values it gave. Existing bills were backfilled with how they arrived (migration 00014). | docs/06 §1, FR-RECO-06 |
| Merge | Attaching a scan or upload to an existing bill compares it field by field (billing month, bill number, issue/due date, current charges, arrears, payable, after-due amount, readings, units): empty fields are filled in, equal values confirm, differing values become **conflicts**. The bill's own value never changes silently. Equality: money and dates exact, readings ±0.5, bill numbers ignoring spaces/punctuation; zero arrears from a scan counts as "not read". | FR-RECO-02/04 |
| Resolve | "Needs review" on the bill: for each difference, keep the bill's value, use the other copy's, **the provider revised it** (use the new value, noted as a revision), or enter the right one. Each choice is in the bill's history. A later copy that agrees with the bill settles an open conflict by itself ("confirmed"). | FR-RECO-05 |
| Side by side | A sources table on the bill: each detail as on the bill and as each copy gave it, differences highlighted. | FR-RECO-06 |
| Undo merge | Within 30 days, **Separate** a merged copy: details only it filled are emptied again (unless changed since), its differences are dropped, and its document is detached, with a **Save it as a bill** shortcut to the scan screen. | FR-RECO-07, docs/06 §5 |
| Attention | Home shows "N bills need review"; the bills list has a **Needs review** filter; saving a scan into an existing bill says how many details differ. | docs/06 §4 |

Quality: Go tests for sources on create, merge (filled / confirmed / conflicting fields, bill unchanged), review count and
filter, resolving with source / revised / keep / manual (with validation), already-resolved and cross-household refusals,
automatic confirmation by an agreeing copy, separate (fields emptied, document detached, history) and its 30-day limit; an
end-to-end test on phone and desktop: scan a PDF of a bill already typed in, attach, see the differences, decide from Home's
review link, then separate.

Fixed along the way:
- Source and conflict timestamps used the database clock; they now use the app clock like every other query.
- On phones, a wide table (or the hidden screen-reader text inside it) could widen the whole page, so the browser zoomed out
  and taps landed beside buttons. Tables now scroll inside a positioned box, and the main area clips sideways overflow;
  the end-to-end test checks the page width.
- Translation helpers no longer take the typed `t` function as a parameter (TypeScript hit its depth limit as the strings
  grew); they use `lib/tr.ts`.
