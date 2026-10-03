# External services: step-by-step setup

Every service here is **free and needs no payment card**. Some are needed now for development;
the rest only when the app goes live.

| Service | What it does in the app | Needed | Key / setting |
|---|---|---|---|
| [Google Gemini](#1-google-gemini-ai-extraction) | Reads bill photos and PDFs (AI extraction) | **Now** | `GEMINI_API_KEY`, `GEMINI_MODEL` |
| [OCR.space](#2-ocrspace-ocr-fallback) | Turns bill images into text when AI isn't allowed or is unavailable | **Now** | `OCRSPACE_API_KEY` |
| [Supabase](#3-supabase-database-and-file-storage) | Database and document storage in production | Going live | `DATABASE_URL`, `S3_*` |
| [Resend](#4-resend-email) | Password-reset and reminder emails | Going live | `RESEND_API_KEY`, `MAIL_FROM` |
| [Render, Cloudflare Pages, cron-job.org](#5-hosting-accounts) | Hosting the API and web app, and the 10-minute tick | Going live | see [deploy.md](deploy.md) |
| [GitHub secrets](#6-github-secrets-backups) | Encrypted daily database backup | Going live | `DATABASE_URL`, `BACKUP_PASSPHRASE` |

> Free-tier limits and screens change. The steps below were written in October 2026; if a button has moved,
> look for the same words nearby.

---

## 0. Where keys go (local development)

1. Copy the template: `backend/.env.example` → `backend/.env`.
2. Paste each key on its own line, e.g. `GEMINI_API_KEY=AIza...`. No spaces around `=`, no quotes needed.
3. Restart the API (`go run ./cmd/app serve` from `backend/`). It reads `backend/.env` on start.
   A variable already set in your shell wins over the file.

`backend/.env` is git-ignored: **never commit it, paste keys into chat, or put them in screenshots.**
If a key leaks, delete it in the service's dashboard and create a new one.

---

## 1. Google Gemini (AI extraction)

Gemini is the most accurate free way to read photographed bills in any layout. The app only uses it when
the household owner turns on **Settings → Privacy → Use AI-assisted extraction**.

**Get the key**
1. Open **https://aistudio.google.com** and sign in with a Google account.
2. Accept the terms if asked.
3. Click **Get API key** (left menu), then **Create API key**.
4. If asked for a project, choose **Create API key in new project**.
5. Copy the key (it starts with `AIza`).

**Keep it free**
- Don't attach a billing account to the project. Without billing, the key stays on the free tier:
  requests beyond the limit are refused (the app then falls back to OCR.space); you're never charged.
- On the free tier Google may use the content you send to improve its products. That's why AI extraction
  is off by default and the app shows a notice before it's turned on.

**Choose the model**
1. In AI Studio, open the model picker (or the **Models** page in the docs) and find the current **Flash** model.
   Flash models are fast and have the most generous free limits.
2. Copy its API name, e.g. `gemini-3.8-flash`. You can list several, separated by commas: when the first is busy
   ("high demand") or over its free limit, the next is tried.

**Add to `backend/.env`**
```
GEMINI_API_KEY=AIza...your key...
GEMINI_MODEL=gemini-3.8-flash,gemini-3.5-flash,gemini-flash-latest
```
If `GEMINI_MODEL` is left out, the app uses `gemini-flash-latest` (Google's alias for the current Flash model).
Older models are withdrawn for new accounts over time; if extraction logs "model … is no longer available",
pick a current name. To see the models your key can use:
```powershell
Invoke-RestMethod -Uri "https://generativelanguage.googleapis.com/v1beta/models?key=$k&pageSize=200" |
  Select-Object -ExpandProperty models | Where-Object name -like '*flash*' | Select-Object -ExpandProperty name
```

**Check the key works** (PowerShell; replace the key and model):
```powershell
$k = "AIza...your key..."
Invoke-RestMethod -Method Post -ContentType "application/json" `
  -Uri "https://generativelanguage.googleapis.com/v1beta/models/gemini-3.8-flash:generateContent?key=$k" `
  -Body '{"contents":[{"parts":[{"text":"Reply with OK"}]}]}' | ConvertTo-Json -Depth 6
```
A reply containing `OK` means the key works. `API_KEY_INVALID` → copy the key again. `NOT_FOUND` → the model
name isn't available to your key (list them as above). `UNAVAILABLE` / "high demand" → Google is busy; try again
in a minute (the app retries and falls back by itself).

**Turn it on in the app:** sign in as the household owner → **Settings → Privacy → Use AI-assisted extraction**.

---

## 2. OCR.space (OCR fallback)

OCR.space reads the text on a bill image. Our own parsers then pick out the fields. It's used when AI
extraction is off (privacy mode), when Gemini is over its limit or down, and to cross-check AI results.

**Get the key**
1. Open **https://ocr.space/ocrapi/freekey**.
2. Enter your email address, tick the terms and submit.
3. The key arrives by email within a few minutes (check spam). It looks like `K8...`.

**Free limits (approx.):** 25,000 requests a month, files up to 1 MB, PDFs up to 3 pages. The app compresses
photos to ≤ 1 MB on the phone, so they fit.

**Add to `backend/.env`**
```
OCRSPACE_API_KEY=K8...your key...
```

**Check the key works** (PowerShell):
```powershell
$k = "K8...your key..."
Invoke-RestMethod -Uri "https://api.ocr.space/parse/imageurl?apikey=$k&url=https://www.google.com/images/branding/googlelogo/2x/googlelogo_color_272x92dp.png" |
  Select-Object -ExpandProperty ParsedResults | Select-Object -ExpandProperty ParsedText
```
`Google` (the logo's text) means the key works. `Invalid API key` → check the emailed key.

---

## 3. Supabase (database and file storage)

*Going live only. Locally the app uses your PostgreSQL and the `backend/var/storage` folder.*

1. Sign up at **https://supabase.com** (GitHub or email, no card) → **New project**.
   - Name: `bill-management`. Region: closest to Pakistan (e.g. Singapore).
   - Set a strong database password and save it in your password manager.
2. **Database URL:** **Project Settings → Database → Connection string → Session pooler**.
   Copy the URI, put in your password, add `?sslmode=require` → this is `DATABASE_URL`.
3. **Bucket:** **Storage → New bucket** → name `documents` → keep it **private**.
4. **S3 keys:** **Project Settings → Storage → S3 Connection** → enable → note the **Endpoint** and **Region**
   → **New access key** → copy the **Access key ID** and **Secret access key** (shown once).
5. These become:
   ```
   S3_ENDPOINT=https://<project-ref>.supabase.co/storage/v1/s3
   S3_REGION=<region shown>
   S3_BUCKET=documents
   S3_ACCESS_KEY_ID=...
   S3_SECRET_ACCESS_KEY=...
   ```

---

## 4. Resend (email)

*Going live only. Locally, emails (such as password-reset links) are printed in the API log.*

1. Sign up at **https://resend.com** (no card).
2. **API Keys → Create API key** → permission **Sending access** → copy it (starts with `re_`).
3. Until you own a domain, send from the test address: `MAIL_FROM=Bill Manager <onboarding@resend.dev>`
   (Resend then only delivers to your own sign-up email). To email anyone, add and verify your domain under
   **Domains**, then use an address on it.
4. Set `RESEND_API_KEY=re_...` and `MAIL_FROM=...`.

---

## 5. Hosting accounts

*Going live only.* Create free accounts at **https://render.com**, **https://dash.cloudflare.com** and
**https://cron-job.org** (all no card), then follow [deploy.md](deploy.md) step by step. It covers the Render
blueprint, the Cloudflare Pages build, and the cron-job.org tick.

Secrets you generate yourself (`TICK_TOKEN`, `DATA_ENCRYPTION_KEY`, `BACKUP_PASSPHRASE`) must be long and random.
Render can generate the first two. To make one yourself in PowerShell:
```powershell
[Convert]::ToBase64String((1..48 | ForEach-Object { Get-Random -Maximum 256 }))
```
Store `DATA_ENCRYPTION_KEY` and `BACKUP_PASSPHRASE` in your password manager: without them, encrypted
reference numbers and backups can't be read.

---

## 6. GitHub secrets (backups)

*Going live only.* In the repository on GitHub: **Settings → Secrets and variables → Actions → New repository secret**:
- `DATABASE_URL`: the same Supabase URL as Render.
- `BACKUP_PASSPHRASE`: a long random string (see above).

The daily backup workflow starts working once both exist.

---

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| Extraction says "AI extraction is off" | Settings → Privacy switch is off (the household owner must turn it on) |
| Extraction falls back to OCR every time | `GEMINI_API_KEY` missing or invalid, or the daily free limit is used up; check the API log |
| "No extraction service is configured" | Neither `GEMINI_API_KEY` nor `OCRSPACE_API_KEY` is set and the file isn't a text PDF |
| Keys in `backend/.env` are ignored | The API was started from another folder, or the same variable is set in your shell |
