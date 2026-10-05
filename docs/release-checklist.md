# MVP release checklist (M5)

Use this when putting the app live for real use, and again after each release. The setup steps themselves are in
[deploy.md](deploy.md); keys for the reading services are in [setup-external-services.md](setup-external-services.md).

## Before going live
- [ ] All checks pass on the release branch: Go tests (`go test ./...`), web unit tests (`pnpm exec vitest run`),
      `pnpm build`, and end-to-end tests (`pnpm exec playwright test`, with `vite preview` running for the offline test).
- [ ] Render: `DATABASE_URL`, `APP_BASE_URL`, `DATA_ENCRYPTION_KEY` (generated, stored in your password manager),
      `TICK_TOKEN`, storage (`STORAGE_DRIVER=s3` + `S3_*`), and optionally `GEMINI_API_KEY` / `OCRSPACE_API_KEY`.
- [ ] `DATA_ENCRYPTION_KEY` is backed up. It encrypts reference numbers and signs file links; it also derives the push
      keys. Losing it makes reference numbers unreadable; changing it signs everyone out of push.
- [ ] Cloudflare Pages builds from root `web` with `pnpm install --frozen-lockfile && pnpm build` (the install step also
      copies the on-device OCR engine into `public/tesseract`), output `dist`. The `/api/*` proxy works:
      `https://<app>.pages.dev/api/v1/auth/me` returns 401, not 404.
- [ ] cron-job.org calls `POST https://<service>.onrender.com/internal/tick` every 10 minutes with
      `Authorization: Bearer <TICK_TOKEN>`, failure e-mails on. Its history shows 200s.
- [ ] GitHub repository secrets `DATABASE_URL` and `BACKUP_PASSPHRASE` are set; the daily backup workflow has run once.

## Smoke test on the live site (phone)
- [ ] Sign up, add your real profiles (electricity, gas, water, internet, rent…) with reference numbers and usual issue
      and due days.
- [ ] Scan a real bill: it is read (AI, OCR, or **Read on this phone**), you check it, it saves to the right profile.
- [ ] Install the app (Settings → Reminders → **Install app**, or Share → Add to Home Screen on iPhone). It opens from
      the home screen without the browser bar.
- [ ] Turn on **Push notifications on this device** and **Send a test notification**: it arrives.
- [ ] Turn on airplane mode: the app still opens with your bills; a photo taken now uploads after airplane mode is off.
- [ ] Download a CSV export and a full backup from Settings → Your data, and open them.

## Exit run: one real billing cycle
M5's exit criterion is using the app for one full billing cycle with no missed due dates.
- [ ] Every bill of the month is in the app (scanned, uploaded, typed, or created automatically for rent).
- [ ] Each "due soon" reminder arrived (in the bell, and as a push where enabled) before its due date.
- [ ] A bill that arrived late showed under **Not added yet** on Home.
- [ ] Every bill was marked paid by its due date; Home shows the month as all paid.
- [ ] Note anything that went wrong in `IDEAS.md` (or as issues) before starting M6.

## Rollback
- Render keeps previous deploys: **Manual Deploy → Rollback** to the last good one. Migrations only ever add; a rolled-back
  API still runs against the newer schema.
- Cloudflare Pages: **Deployments → … → Rollback**.
- Data: restore the latest encrypted backup from the backup workflow's artifacts (see `.github/workflows/backup.yml`).
