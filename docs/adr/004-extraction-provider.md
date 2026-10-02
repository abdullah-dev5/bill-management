# ADR-004 — OCR / extraction chain

- **Status:** Accepted
- **Date:** 2026-10-03

## Context
We need high field accuracy on Pakistani utility bills (photos and PDFs), at zero cost with no payment card, and a privacy option for households that don't want their bills sent to an AI service.

## Decision
A pluggable **Extractor** interface with a **config-driven fallback chain**. Every tier is free and needs no card:

| Tier | Extractor | Role |
|---|---|---|
| 0 | On-device barcode/QR decode + PDF text layer | Identity; text PDFs need no OCR |
| 1 | **Google Gemini Flash** (structured JSON) | Primary, with household AI consent |
| 2 | **OCR.space** + our provider templates | Fallback, and the default without AI consent |
| 3 | **Tesseract.js** on device + our provider templates | Offline/private fallback (M5) |
| 4 | Manual entry, document attached | Always available |

After every tier: normalisation → template cross-check → validation rules → **mandatory human verification**.

The fallback triggers (timeouts, quota, circuit breaker, low confidence, invalid JSON) are defined in [05-extraction-pipeline](../05-extraction-pipeline.md) §4.2.

## Why
- Gemini is the most accurate free option and returns fields directly.
- OCR.space is free with no card and covers outages, quotas and households without consent.
- On-device OCR is unlimited, private and works offline.
- Accuracy is owned by **our layer**: barcode identity, provider templates, validation and learning from corrections. It works the same whichever tier produced the text.

## Consequences
- One adapter per tier, plus quota counters and a circuit breaker.
- Provider templates and masked sample-bill fixtures are a core asset (collected in M0).
- The AI consent setting and privacy notice ship with M2.
- Free-tier terms are re-checked every quarter. Tiers can be swapped by configuration.
