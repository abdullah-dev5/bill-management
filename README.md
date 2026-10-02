# Bill Management & Household Expense Platform

A household bill manager that turns **scanned, uploaded, manually entered and officially retrieved** bills into one organised, searchable and trackable financial history.

> **Status:** Phase M0 (documentation and planning). No application code yet.
> **Stack (final):** Go backend + React/TypeScript PWA, hosted on free tiers that need no payment card: Cloudflare Pages, Render, Supabase, GitHub Actions, cron-job.org.

## Core principle

> The bill management system must work fully **without** automatic retrieval.
> Retrieval is just one more way to add bills. It is never the foundation.

## Documentation

| # | Document | What it answers |
|---|---|---|
| 01 | [Product Vision](docs/01-product-vision.md) | Why it exists, who it is for, what is in and out of scope |
| 02 | [Requirements](docs/02-requirements.md) | Every feature as a numbered, testable requirement (FR/NFR) |
| 03 | [User Flows & Screens](docs/03-user-flows-and-screens.md) | How people use it, plus a screen inventory and wireframes |
| 04 | [Domain Model](docs/04-domain-model.md) | Entities (incl. bill profiles), fields, relationships, state machines |
| 05 | [Extraction Pipeline](docs/05-extraction-pipeline.md) | Scan/upload to a structured bill; the OCR fallback chain |
| 06 | [Reconciliation](docs/06-reconciliation.md) | Duplicate detection, merging, conflict resolution |
| 07 | [Architecture](docs/07-architecture.md) | System components, Go + TypeScript stack, free hosting |
| 08 | [Security & Privacy](docs/08-security-privacy.md) | Protecting the PII inside bills |
| 09 | [Roadmap](docs/09-roadmap.md) | Milestones M0–M7 with exit criteria |
| 10 | [Design System](docs/10-design-system.md) | Colours, type, components, profile palette, layout |
| — | [Glossary](docs/glossary.md) | Bill terminology (reference no., FPA, arrears, MF, …) |
| — | [ADRs](docs/adr/) | Architecture Decision Records |

## Decision log

| ADR | Decision | Status |
|---|---|---|
| [001](docs/adr/001-money-as-integer-minor-units.md) | Store money as integer minor units | Accepted |
| [002](docs/adr/002-retrieval-is-optional-channel.md) | Official retrieval is an optional input channel | Accepted |
| [003](docs/adr/003-backend-language.md) | Go backend + React/TypeScript PWA | Accepted |
| [004](docs/adr/004-extraction-provider.md) | OCR / extraction fallback chain | Accepted |
| [005](docs/adr/005-hosting.md) | Free hosting, no payment card | Accepted |

## Next step

Collect 10–20 masked sample bills for extraction tests, then write the implementation plan for **M1: Foundation**.
