# Adding an automatic retrieval connector

Most of M6's official retrieval is **assisted**: the person opens the provider's own bill page,
clears its security check, and uploads the bill (see [milestones/M6](milestones/M6-retrieval-reconciliation.md)).
This document is for the other case — a provider that **grants automated access** — and how to turn
that into an automatic connector.

## When a connector is allowed
Add an automatic connector **only** when the provider permits automated access, in writing or by a
public, terms-compatible API:

- an official API key or data-sharing agreement with the provider / PITC, **or**
- a documented public endpoint whose terms of use allow automated requests (no clause against bots,
  scraping, or automated access), **and** no login or CAPTCHA to get the bill.

Do **not** add a connector that works around a CAPTCHA, a login, rate-blocking, or any other control
a provider uses to stop automated access — whether by solving it in code or through a third-party
solving service. That is unauthorized access (PECA 2016; the providers' terms), it points the risk at
the people using the app, and it contradicts [ADR-002](adr/002-retrieval-is-optional-channel.md) and
[docs/08](08-security-privacy.md). Those providers stay on the assisted flow. This boundary is the
reason the connector template ships unimplemented.

## How to add one
1. Copy `backend/internal/retrieval/template.go`. Set `ID`, `Name`, and `Providers` (catalogue short
   names, e.g. `{"LESCO"}`), and implement `Fetch` against the authorized source. Return the bill as
   a `Document` (PDF or image); return `ErrNotFound` when there is no bill for that reference yet.
2. Register it in `connectors()` in `backend/internal/httpapi/server.go`, gated on its credential
   (e.g. only when its API key is configured), so it is off until set up.
3. Keep the provider's lookup page in `lookup_url` (migration 00015) as the assisted fallback for
   when the connector is paused.
4. Everything else already works: the per-connector throttle, timeout and panic recovery; health
   with an automatic pause after repeated failures; the daily scheduler; **Fetch now**; and the
   reconciliation of the fetched bill against any copy already saved.

## Getting official access (start early — it is slow)
Requesting access from a Pakistani utility or PITC can take a long time, so begin before you need it.
Keep a record of each request in `docs/retrieval-outreach.md`.

- **PITC** (bill.pitc.com.pk) serves the electricity DISCOs (LESCO, HESCO, IESCO, …). Ask PITC about
  an official bill-data API or a data-sharing agreement for a registered service.
- **K-Electric**, **SSGC**, **SNGPL** — ask each provider's IT / customer-services department.
- Consider **licensed bill aggregators** and bank / PSO / 1LINK bill-fetch rails, which already have
  provider agreements, as an intermediary.

Until a provider says yes, assisted retrieval is the official channel and nothing is blocked on us.
