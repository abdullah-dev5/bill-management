# M5 — Reminders, expected bills & PWA: progress notes

Branch `m5-reminders-pwa` (PR #7 → `main`).
Exit criteria from [09-roadmap](../09-roadmap.md) M5: used for real for one full billing cycle with no missed due
dates. **MVP release.**

| Phase | Scope | State |
|---|---|---|
| 1 — Reminders & expected bills | Tick reminders, notification inbox, settings, expected-but-missing bills, recurring bills that add themselves | **Done** |
| 2 — Installable & offline | Web app manifest, service worker, offline shell, offline capture queue, Web Push | **Done** |
| 3 — On-device reading & release | Tesseract.js tier, deploy checklist, exit run | Next |

## Phase 2 — Installable & offline (delivered)
| Area | What's in | Requirements |
|---|---|---|
| Installable | Web app manifest (name, theme, standalone, portrait), PNG icons rendered from the logo (192, 512, maskable, Apple touch icon), shortcuts for "Scan a bill" and Notifications. Settings → Reminders offers **Install app** where the browser allows it, and the Share → Add to Home Screen steps on iPhone. | M5 |
| Offline shell | A service worker (production builds only) precaches the app so it opens without a connection. Read-only screens use the last copy of their data when offline (network first, 4 s timeout, kept 7 days); files, exports and changes are never cached. Signing in or out clears that copy. A banner says when you're offline. | M5 |
| Offline capture queue | A photo or PDF added without a connection (or when the upload drops) is kept in IndexedDB on the device and uploads by itself when the connection returns (also every 20 s while waiting, or **Upload now**). A queued bill with no bill attached offers **Check it** to open the review screen once uploaded. Signing out empties the queue. | FR-EXTR-13 |
| Web Push | Settings → Reminders → **Push notifications on this device**: asks permission and subscribes the device; the tick pushes each new reminder once, with the same wording as the inbox, opening the bill (or the new-bill form for a missing bill). Quiet hours hold pushes until they end (the bell still shows them); reminders older than a day are never pushed. Devices that unsubscribe (404/410) or keep failing are removed. **Send a test notification** checks a device. VAPID keys are derived from the deployment secret: no setup. | FR-REM-02/03 |

Quality: Go tests for the push key, push off by default (and not back-filled), quiet hours across midnight, a reminder
pushed once after quiet hours with the right title/body/link, dead devices removed, test push, unsubscribe and
validation, rupee formatting; end-to-end tests for the offline queue (offline → saved → back online → uploaded) and, on
the production build, installability and opening offline with cached data.

## Phase 1 — Reminders & expected bills (delivered)
| Area | What's in | Requirements |
|---|---|---|
| Tick | The 10-minute tick (`POST /internal/tick`, called by cron-job.org in production) now also sends reminders and creates recurring bills. In development the API ticks itself every 10 minutes, starting at launch. Everything is idempotent: each reminder has a dedupe key, and a recurring bill is created once per period. | FR-REM-01 |
| Reminders | Per person: "due in N days" (N from settings, default 3), "due today", and "overdue" (once, the day after). Only open, verified bills; paid or settled bills stop reminding. | FR-REM-01 |
| Expected bills | A monthly (or every 2/3/12 months) profile whose bill hasn't been added by its usual issue day (default the 10th) + 5 days is "not added yet": Home lists it with an **Add bill** button that opens the form on the right month, and the bell reminds once. A profile made after the deadline gets a cycle before it's flagged; profiles that create their own bills are never flagged. | FR-REM-04, FR-DASH-04 |
| Recurring bills | Recurring profiles with "Add this bill automatically" create the bill on their issue day with the expected amount and the usual due day (next month if the due day falls before the issue day), recorded in the bill's history as an automatic change, and notify everyone in the household. | FR-BILL-10 |
| Inbox | Bell with an unread count (sidebar on desktop, header on phones); the Notifications page lists reminders with amounts and dates, marks one read when opened, or all at once. | FR-REM-01 |
| Settings | Settings → Reminders: how early (on the day … 14 days before), and switches for coming-due, overdue and not-arrived reminders; saved per person per household. | FR-REM-02 |

Quality: Go tests for due-soon → due-today → overdue over several days, idempotent ticks, read/unread, paid bills going quiet,
household isolation, lead days and switches, settings validation, expected bills (missing September found on 3 Oct, cleared
when added, October missing on 20 Oct, new profiles given a cycle), recurring bills (created once, due date, history,
notification, never "missing", next month's created on its own) and billing-cycle periods; an end-to-end test of the bell,
inbox, opening a reminder, mark all read and the settings.

Also: the generated API constants for bill statuses are now prefixed (`BillStatusOverdue`), because notifications
introduced a second `overdue` value.
