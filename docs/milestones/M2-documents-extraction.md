# M2 — Documents, extraction & verification: progress notes

Branch `m2-documents-extraction`. Exit criteria from [09-roadmap](../09-roadmap.md) M2.
M2 ships in three phases. Each phase is committed and pushed; the branch is merged once all three are done.

| Phase | Scope | State |
|---|---|---|
| 1 — Documents | Upload, storage, signed links, attach to bills, AI-consent setting, encrypted reference numbers | **Done** |
| 2 — Extraction engine | Job queue, extractor chain (barcode / PDF text, Gemini, OCR.space + templates), validation, redaction | Next |
| 3 — Verification & duplicates | Verification screen, profile matching, FR-RECO-01…03, corrections log | — |

## Phase 1 — Documents (delivered)
| Area | What's in | Requirements |
|---|---|---|
| Capture | Take a photo (phone camera) or choose a JPG, PNG, WEBP, HEIC or PDF; HEIC is converted to JPEG in the browser | FR-EXTR-01/02 |
| Preprocessing | EXIF auto-rotate, manual rotate, drag-to-crop, resize to ≤ 2000 px and compress to ≤ 1 MB on the phone | FR-EXTR-03 |
| Storage | Local disk in development, Supabase Storage (S3 API) in production; direct browser uploads through presigned URLs, with an API pass-through fallback | 07 §3 |
| Upload checks | Type allowlist and 10 MB cap; size, SHA-256 and real file type (magic bytes) are verified before a document becomes visible; rejected uploads are deleted | 08 §1 |
| Signed links | Every view/download link expires after 5 minutes; images are served with a sandbox CSP | FR-DOC-04 |
| Documents on bills | Many per bill (bill, receipt, other); view, download, replace, delete; attach/remove recorded in the bill's history | FR-DOC-01/02 |
| Deduplication | The same file is stored once per household; uploading it again reuses (and attaches) the existing document | M2 exit criterion |
| Quota | 100 MB per household (configurable), shown in Settings; uploads are rate-limited | FR-DOC-05 |
| Cleanup | Files of deleted documents, bills and households are removed by the tick; abandoned uploads after 24 h | 08 §6 |
| AI consent | Household setting "Use AI-assisted extraction", off by default, with the plain-language notice; owner only; audited | FR-EXTR-11 |
| Reference numbers | Stored as an AES-GCM ciphertext plus a keyed hash (for matching and uniqueness); history shows only the last four digits; existing data migrated | 08 §4 |

### Quality
- Go: integration tests for the upload flow, signed links (tampering, expiry), rejected uploads, deduplication,
  household isolation, attach/detach, storage cleanup (including cascades), quota, the viewer role, AI consent and
  reference encryption. Storage contract tests run against local disk and an in-process S3 fake.
- Web: unit tests for image sizing, file helpers and crop dragging; an end-to-end test on phone and desktop
  (photo with crop → PDF → duplicate PDF → view → delete → storage and AI consent in Settings). The M1 flow still passes.
- The encryption migration was run on the development database, rolled back and re-applied without data loss.

### Decisions
- **For photos, the compressed image is the stored file.** Keeping 4–12 MB camera originals would use up the
  1 GB free storage quickly and doesn't help read the bill. PDFs are stored unchanged.
- **Document kind is bill / receipt / other**, with a separate `source` (camera / upload). This replaces
  bill_scan / bill_official in docs/04.
- **The 5-page PDF limit** is enforced in phase 2, together with PDF text extraction.
- **PDFs open in a new tab** rather than an embedded viewer, which works poorly on phones.

### Needs your action before going live
A Supabase storage bucket and S3 key, and the new Render variables: see [deploy.md](../deploy.md).
Back up `DATA_ENCRYPTION_KEY`.
