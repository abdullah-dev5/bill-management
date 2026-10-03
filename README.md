# Bill Management & Household Expense Platform

A household bill manager that turns **scanned, uploaded, manually entered and officially retrieved** bills into one organised, searchable and trackable financial history.

> **Status:** M1 Foundation complete ([delivery notes](docs/milestones/M1-foundation.md)): accounts, bill profiles with their own dashboards, manual bills and payments.
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
| — | [Deployment runbook](docs/deploy.md) | Putting it live on the free tiers |
| — | [Milestone notes](docs/milestones/) | What each milestone delivered and deferred |
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

## Repository layout

```
api/openapi.yaml     API contract: the source of truth for the Go and TypeScript types
backend/             Go API (chi, pgx + sqlc, goose migrations)
  cmd/app            `app serve` / `app migrate up|down|status`
  cmd/devdb          creates the local dev and test databases
  internal/          httpapi, store (sqlc), auth, domain, mail, ratelimit, config
  migrations/        SQL schema + seed (bill types, Pakistani providers)
  queries/           SQL for sqlc
web/                 React + TypeScript PWA (Vite, TanStack Router/Query, Tailwind)
  src/features/      auth, home, onboarding, profiles, bills, settings
  functions/         Cloudflare Pages Function: same-origin /api proxy
  e2e/               Playwright end-to-end tests
docs/                product, design and architecture documentation
```

## Local development

**Prerequisites:** Go 1.27+, Node 22 + pnpm 10, PostgreSQL 17 on `localhost:5432` (user `postgres`, password `postgres`), and the [sqlc](https://sqlc.dev) binary (only needed to change queries).

```bash
# 1. Databases (once)
cd backend
go run ./cmd/devdb                      # creates bill_dev and bill_test

# 2. API on :8080 (applies migrations on start)
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/bill_dev?sslmode=disable"
go run ./cmd/app serve

# 3. Web app on :5173 (proxies /api to :8080)
cd ../web
pnpm install
pnpm dev
```

Password-reset emails are printed in the API log while `RESEND_API_KEY` is unset.

| Task | Command |
|---|---|
| Backend tests (uses `bill_test`) | `cd backend && go test ./...` |
| Regenerate after editing `api/openapi.yaml` | `cd backend && go generate ./...` and `cd web && pnpm gen:api` |
| Regenerate after editing `queries/*.sql` | `cd backend && sqlc generate` |
| Web unit tests / lint / types | `cd web && pnpm test && pnpm lint && pnpm typecheck` |
| End-to-end tests (starts both servers if needed) | `cd web && pnpm exec playwright test` |

## Next step

M2: documents, the OCR extraction chain and the verification screen ([roadmap](docs/09-roadmap.md)). Collect 10–20 real bills (LESCO, SNGPL, PTCL) with personal details masked, for the extraction tests.
