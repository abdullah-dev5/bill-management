# Deployment runbook (free tiers, no payment card)

The architecture is in [07-architecture](07-architecture.md); the hosting decision is in [ADR-005](adr/005-hosting.md).

```
Browser ──▶ Cloudflare Pages (PWA + /api/* proxy function) ──▶ Render (Go API) ──▶ Supabase (Postgres)
                                                                   ▲
                                         cron-job.org (10-min tick) ┘
```

The app and API share one origin (`https://<app>.pages.dev`). A Pages Function forwards `/api/*` to Render, so the session cookie stays first-party.

## 1. Supabase (database)
1. Create a free project at supabase.com (no card). Pick the region closest to your Render region (e.g. Singapore).
2. **Project Settings → Database → Connection string → Session pooler**. Copy the URI and append `?sslmode=require`:
   `postgresql://postgres.<ref>:<password>@aws-0-<region>.pooler.supabase.com:5432/postgres?sslmode=require`
   - Use the **session pooler**. The direct connection is IPv6-only on the free tier, which Render can't reach. Transaction mode (port 6543) breaks prepared statements.
3. Migrations run automatically when the API starts.
4. **Storage → New bucket** named `documents`, **private** (not public).
5. **Project Settings → Storage → S3 Connection**: enable it, note the endpoint
   (`https://<ref>.supabase.co/storage/v1/s3`) and region, and create an **S3 access key**.
   Copy the access key ID and secret; they're needed in step 2.

## 2. Render (Go API)
1. render.com → **New → Blueprint** → connect the GitHub repo. It reads [`render.yaml`](../render.yaml).
2. Fill in the values it asks for:
   | Variable | Value |
   |---|---|
   | `DATABASE_URL` | Supabase session-pooler URL from step 1 |
   | `APP_BASE_URL` | Your Pages URL, e.g. `https://bill-management.pages.dev` (used in emailed links) |
   | `RESEND_API_KEY` | Optional. Without it, reset emails are only written to the log |
   | `S3_ENDPOINT`, `S3_REGION` | From Supabase step 5 |
   | `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY` | The S3 access key from Supabase step 5 |

   `TICK_TOKEN` and `DATA_ENCRYPTION_KEY` are generated for you. Copy `TICK_TOKEN` for step 4.
   **Copy `DATA_ENCRYPTION_KEY` into your password manager now.** It encrypts reference numbers;
   if the service is ever recreated without it, those values can't be read again.
3. Deploy, then check `https://<service>.onrender.com/healthz` → `{"status":"ok"}`.

## 3. Cloudflare Pages (web app)
1. dash.cloudflare.com → **Workers & Pages → Create → Pages → Connect to Git** → pick the repo.
2. Build settings:
   | Setting | Value |
   |---|---|
   | Root directory | `web` |
   | Build command | `pnpm install --frozen-lockfile && pnpm build` |
   | Build output | `dist` |
   | Environment variables | `API_ORIGIN = https://<service>.onrender.com`, `NODE_VERSION = 22` |
3. The function in `web/functions/api/[[path]].ts` is deployed automatically. Security headers come from `web/public/_headers`.
4. Open the site, create your account and you're live.

### If uploads fail in the browser
Uploads go straight from the browser to Supabase through short-lived signed links. If the browser
blocks them (a CORS error in the console), set `STORAGE_DIRECT_UPLOADS=false` on Render: files then
travel through the API instead. Nothing else changes.

## 4. cron-job.org (tick)
1. Create a free account and a new job:
   - URL: `https://<service>.onrender.com/internal/tick`, method **POST**
   - Header: `Authorization: Bearer <TICK_TOKEN>`
   - Schedule: every 10 minutes; enable failure notifications.
2. Besides running housekeeping, the tick keeps the Render instance awake and stops Supabase pausing the project for inactivity.

## 5. GitHub (CI and backups)
- CI ([`.github/workflows/ci.yml`](../.github/workflows/ci.yml)) runs on every pull request.
- Backups ([`.github/workflows/backup.yml`](../.github/workflows/backup.yml)) run daily once you add the repository secrets `DATABASE_URL` (same as Render) and `BACKUP_PASSPHRASE` (a long random string; store it in your password manager, because a backup can't be restored without it).

## Environment variables (API)
| Variable | Required | Default | Notes |
|---|---|---|---|
| `DATABASE_URL` | yes | — | Postgres URL |
| `APP_ENV` | no | `development` | `production` enables JSON logs and strict checks |
| `PORT` | no | `8080` | Render sets this |
| `APP_BASE_URL` | no | `http://localhost:5173` | Public app URL for emailed links |
| `TICK_TOKEN` | prod | — | ≥ 32 characters in production |
| `COOKIE_SECURE` | no | `true` in production | |
| `RESEND_API_KEY` | no | — | Enables real email |
| `MAIL_FROM` | no | `Bill Manager <no-reply@localhost>` | Must be a verified Resend sender |
| `DATA_ENCRYPTION_KEY` | prod | fixed dev key | ≥ 32 characters. Encrypts reference numbers and signs file links. Back it up |
| `STORAGE_DRIVER` | no | `s3` if `S3_ENDPOINT` is set, else `local` | Production requires `s3` |
| `STORAGE_DIR` | no | `var/storage` | Local driver only |
| `S3_ENDPOINT` / `S3_REGION` / `S3_BUCKET` | prod | — / `ap-southeast-1` / `documents` | Supabase S3 connection |
| `S3_ACCESS_KEY_ID` / `S3_SECRET_ACCESS_KEY` | prod | — | Supabase S3 access key |
| `STORAGE_DIRECT_UPLOADS` | no | `true` | `false` sends file bytes through the API |
| `STORAGE_QUOTA_MB` | no | `100` | Document storage per household |
