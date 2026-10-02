# ADR-003 — Application stack: Go backend + TypeScript PWA

- **Status:** Accepted
- **Date:** 2026-10-03

## Context
- Must run at zero cost on free tiers (512 MB RAM, sleeping instances).
- Needs strong async processing: extraction jobs, schedulers, later retrieval connectors.
- Must be production quality: typed, tested, maintainable.
- Must work well on phones without a separate native app.
- The owner wants to learn Go.

## Decision
- **Backend:** a **Go** modular monolith in one process: REST API + River background workers on Postgres. It uses chi, pgx + sqlc, goose migrations, and oapi-codegen.
- **Frontend:** a **React + TypeScript + Vite PWA** (installable, offline shell, camera capture, Web Push). It uses TanStack Router/Query, Tailwind + shadcn/ui, react-hook-form + zod, and Recharts.
- **Contract:** `api/openapi.yaml` is the single source of truth between Go and TypeScript. Both sides generate their types from it.

## Why
- Go uses ~20–50 MB RAM and starts in under a second, which is ideal for free instances.
- Goroutines and a Postgres job queue give strong processing with no extra infrastructure.
- A PWA gives one codebase for desktop and mobile, including camera and push.

## Consequences
- The M1 plan uses Go tooling for the backend and Vite/TypeScript tooling for the web app, in one monorepo.
- Details: [07-architecture](../07-architecture.md) §4.
