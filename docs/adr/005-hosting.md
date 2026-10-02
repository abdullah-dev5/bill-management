# ADR-005 — Hosting: free tiers with no payment card

- **Status:** Accepted
- **Date:** 2026-10-03

## Context
The owner has no server and wants **no bills, ever**. Choosing services that never ask for a payment card removes any risk of a surprise charge.

## Decision

| Layer | Service |
|---|---|
| Frontend (PWA) | **Cloudflare Pages** |
| Go API + workers (one process) | **Render** free web service |
| PostgreSQL | **Supabase** |
| Document storage | **Supabase Storage** (S3-compatible API) |
| CI/CD, daily backup | **GitHub Actions** |
| 10-minute tick | **cron-job.org** |
| Error tracking | **Sentry** free tier |
| Email reminders (M5) | **Resend** free tier |
| Push notifications (M5) | **Web Push (VAPID)** |

## Why
- None of these services need a card.
- The Go app's small memory footprint fits Render's 512 MB instance.
- One Supabase project provides both Postgres and file storage.
- The cron-job.org tick does three jobs: it runs scheduled work, keeps Render awake, and prevents Supabase's inactivity pause. It is not on GitHub Actions because the private repo's 2,000 free minutes a month would not cover it.

## Consequences
- The API and workers share one process (Render free allows one web service). They can be split later without code changes.
- Storage stays within 1 GB through client-side compression and per-household quotas.
- Daily `pg_dump` to Supabase Storage + user export protect against data loss.
- Only standard Postgres, the S3 API and a container image are used, so moving host stays easy.
- Limits and risks: [07-architecture](../07-architecture.md) §5.
