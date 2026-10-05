# ADR-002 — Official retrieval is an optional input channel

- **Status:** Accepted
- **Date:** 2026-10-03

## Context
Automatic retrieval from provider websites is attractive, but provider sites change layout, go down, add CAPTCHAs, or may forbid automated access. A product built *on* retrieval breaks whenever they do.

## Decision
Retrieval is **one of several equal input channels** (manual, scan, upload, official, later email). All channels produce a candidate bill that goes through the **same** validation and reconciliation pipeline. No core feature (history, analytics, payments, reminders, documents) depends on retrieval being available.

## Consequences
- Retrieval lives in an isolated `retrieval` module with connectors. Failures are contained, tracked, and auto-disable a connector.
- Expected-bill detection (FR-REM-04) provides "bill is due" awareness without retrieval.
- Retrieval is scheduled for M6, after the MVP is proven with manual and scan input.
- Only official public lookup pages/APIs are used, rate-limited, with no stored provider passwords.

## Update (M6, 2026-10-05)
The PITC (electricity) and SSGC lookup pages use CAPTCHAs, so they are not fetched automatically. Instead the app offers
**assisted retrieval**: it opens the provider's official page in the person's browser with the reference number copied, and the
bill they save is uploaded as an official copy. The connector framework (throttle, health, scheduler) is in place for any
provider that permits automated access; none does yet, so production runs with no automatic connectors.
