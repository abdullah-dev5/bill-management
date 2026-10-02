# 02 — Requirements

**Priority (MoSCoW):** **M** = Must (MVP) · **S** = Should (MVP if time allows) · **C** = Could (later) · **W** = Won't (this version)
**Milestone** refers to [09-roadmap](09-roadmap.md).

IDs are stable. Never renumber them; deprecate an ID instead.

---

## 1. Functional requirements

### 1.1 Accounts, auth & household (AUTH)

| ID | Requirement | Pri | MS |
|---|---|---|---|
| FR-AUTH-01 | A user can sign up and sign in with email + password. | M | M1 |
| FR-AUTH-02 | A user can sign in with Google (OAuth). | S | M1 |
| FR-AUTH-03 | Password reset by email link. | M | M1 |
| FR-AUTH-04 | On sign-up, a **Household** and a default **Property** ("My Home") are created automatically. | M | M1 |
| FR-AUTH-05 | The household owner can invite members by email with a role: owner / editor / viewer. | S | M1 |
| FR-AUTH-06 | Viewers can read but not modify. Editors can modify but not manage members. | S | M1 |
| FR-AUTH-07 | Sessions expire; the user can sign out of all devices. | M | M1 |

**Acceptance (FR-AUTH-04):** *Given* a new sign-up, *when* the account is created, *then* exactly one household and one property exist, and the user is owner of both.

### 1.2 Properties (PROP)

| ID | Requirement | Pri | MS |
|---|---|---|---|
| FR-PROP-01 | Every bill profile belongs to a property (the default one in the MVP). | M | M1 |
| FR-PROP-02 | A user can create, rename and archive properties, and switch between them. | C | M7 |
| FR-PROP-03 | The dashboard and analytics can be filtered by property or show all. | C | M7 |

### 1.3 Bill profiles & providers (ACCT)

A **bill profile** is a named tracker the user creates for one bill or expense stream, e.g. "Home Electricity", "Shop Electricity", "Flat Rent", "Netflix", "Home Maintenance". A user can create **any number of profiles**, including several of the same type. **Every bill belongs to exactly one profile, and every profile has its own dashboard.**

| ID | Requirement | Pri | MS |
|---|---|---|---|
| FR-ACCT-01 | The system ships a seed catalogue of providers (name, type, region, reference-number format, logo). | M | M1 |
| FR-ACCT-02 | A user can add a custom provider. | M | M1 |
| FR-ACCT-03 | A user creates a **bill profile** with: **name** (required), **type** (required, e.g. Electricity, Rent, or a custom type), **kind** (set by the type, changeable), property, and optionally a provider + reference/consumer number. | M | M1 |
| FR-ACCT-04 | A profile stores its billing cycle (monthly by default), typical issue and due days, and consumption unit (kWh, hm³, gallons, none). | M | M1 |
| FR-ACCT-05 | A metered profile can store the meter number and multiplying factor (default 1). | M | M3 |
| FR-ACCT-06 | Reference numbers are validated against the provider format (length/checksum) when one is defined. | S | M1 |
| FR-ACCT-07 | Profiles can be archived (e.g. after moving house). History is kept and stays viewable. | M | M1 |
| FR-ACCT-08 | **Unlimited profiles per type.** Two profiles of the same type are separate (e.g. "Home Electricity" and "Shop Electricity"). Names are unique within a household. | M | M1 |
| FR-ACCT-09 | **Profile kinds** decide fields and dashboard widgets: **metered** (electricity, gas, water: readings, units, rate), **recurring** (rent, internet, subscriptions: expected amount each cycle), **expense log** (maintenance, repairs: any number of entries, no cycle). | M | M1 |
| FR-ACCT-10 | **Profile dashboard:** every profile has its own page with the tabs Overview · Bills · Payments · Documents · Settings (see [03](03-user-flows-and-screens.md) S10). The records list ships in M1; charts and KPIs in M3. | M | M1/M3 |
| FR-ACCT-11 | Each profile has an icon and a colour (from the profile palette, see [10](10-design-system.md)). Profiles can be **pinned** to the home dashboard and reordered. | S | M1 |
| FR-ACCT-12 | **Type overview:** one page per type (e.g. "Electricity") totals all profiles of that type and compares them side by side. | S | M4 |
| FR-ACCT-13 | Each property gets a default **"General expenses"** profile (kind expense log), so a quick one-off expense never needs a profile to be created first. | M | M1 |
| FR-ACCT-14 | A user can move a bill to another profile. This is audited, and reconciliation re-runs for the bill. | S | M3 |

**Acceptance (FR-ACCT-08/10):** *Given* the profiles "Home Electricity" and "Shop Electricity", *when* the user opens "Shop Electricity", *then* only that profile's bills, payments, documents and charts are shown, and its totals do not include "Home Electricity".

### 1.4 Bills (BILL)

| ID | Requirement | Pri | MS |
|---|---|---|---|
| FR-BILL-01 | Create a bill in a profile with: billing period, issue date, due date, amounts, status, notes. | M | M1 |
| FR-BILL-02 | Amounts: current charges, arrears/previous balance, adjustments, **payable within due date**, **payable after due date**. | M | M1 |
| FR-BILL-03 | Optional line items (label, amount, type: charge / tax / fee / adjustment / arrears). | S | M2 |
| FR-BILL-04 | Entries in an expense-log profile (e.g. "Electrician Rs 3,500") need only description, amount and date. An optional sub-category is allowed (e.g. Electrician, Plumber). | M | M1 |
| FR-BILL-05 | Edit a bill. Every change is written to the audit log (who, when, field, old → new). | M | M1 |
| FR-BILL-06 | Archive (soft delete) and restore. Hard delete only from the archive, with confirmation. | M | M1 |
| FR-BILL-07 | Status is derived and stored: Draft, Unpaid, Partially Paid, Paid, Overdue, Cancelled (see the state machine in [04](04-domain-model.md)). | M | M1 |
| FR-BILL-08 | Per-field provenance: each key field records its source (manual / scan / upload / official / user-edited) and confidence. | M | M2 |
| FR-BILL-09 | The system warns, without blocking, when payable ≠ current charges + arrears + adjustments. | S | M2 |
| FR-BILL-10 | Profiles of kind **recurring** can auto-create the bill each cycle from their expected amount (e.g. rent Rs 40,000 on the 1st). The user can edit it before it is due. | S | M5 |
| FR-BILL-11 | Tags on bills (free text) for custom grouping. | C | M4 |

**Acceptance (FR-BILL-05):** *Given* a saved bill, *when* the user changes the due date, *then* the bill shows the new date *and* the audit log records the old and new values with the user and a timestamp.

### 1.5 Capture & extraction (EXTR)

| ID | Requirement | Pri | MS |
|---|---|---|---|
| FR-EXTR-01 | Capture a photo through the device camera from the web app. | M | M2 |
| FR-EXTR-02 | Upload JPG, PNG, WEBP, HEIC and PDF (max 10 MB, max 5 pages). | M | M2 |
| FR-EXTR-03 | Client-side preprocessing: auto-rotate, crop, compress to ≤ 1 MB, convert HEIC to JPG. | M | M2 |
| FR-EXTR-04 | Decode a barcode or QR code on the device when present, to get the reference number. | S | M2 |
| FR-EXTR-05 | Extraction runs as an asynchronous job. The UI shows progress and is never blocked for more than 2 s. | M | M2 |
| FR-EXTR-06 | Extraction returns structured fields with per-field confidence (see the schema in [05](05-extraction-pipeline.md)). | M | M2 |
| FR-EXTR-07 | Validation rules run on the extracted data (totals, meter maths, date sanity). Failures lower the confidence. | M | M2 |
| FR-EXTR-08 | **Verification screen:** document preview next to the editable fields. Low-confidence or failed fields are highlighted. The user must confirm before the bill becomes non-Draft. | M | M2 |
| FR-EXTR-09 | The extracted reference number is matched to an existing bill profile, or the user is offered to create one. | M | M2 |
| FR-EXTR-10 | The provider is pluggable, with a fallback chain. If all providers fail, the user falls back to manual entry with the document attached. | M | M2 |
| FR-EXTR-11 | Using a third-party AI extractor requires household-level consent. Without consent, only the privacy-preserving path is used. | M | M2 |
| FR-EXTR-12 | Provider-specific templates or parsers for top providers (LESCO/other DISCOs, SNGPL, PTCL). | S | M2 |
| FR-EXTR-13 | Offline capture queue: photos taken offline upload when back online. | S | M5 |
| FR-EXTR-14 | User corrections are stored (input vs. confirmed) to measure accuracy and improve templates. | S | M2 |

### 1.6 Reconciliation & deduplication (RECO)

| ID | Requirement | Pri | MS |
|---|---|---|---|
| FR-RECO-01 | Before a bill is created from any source, look for an existing bill with the same identity key (profile + billing period, or bill number). | M | M2 |
| FR-RECO-02 | On an exact match, attach the new source and documents to the existing bill instead of creating a duplicate. | M | M2 |
| FR-RECO-03 | On a probable match, ask the user: merge / keep separate. | M | M2 |
| FR-RECO-04 | When sources disagree on a field, create a **conflict** record. Never overwrite silently. | M | M6 |
| FR-RECO-05 | Conflict resolution actions: use source A, use source B, enter manually, mark verified. Each choice is audited. | M | M6 |
| FR-RECO-06 | The bill detail page shows all sources and their values side by side. | S | M6 |
| FR-RECO-07 | The user can undo a merge within 30 days. | C | M6 |

### 1.7 Payments (PAY)

| ID | Requirement | Pri | MS |
|---|---|---|---|
| FR-PAY-01 | Record a payment: date, amount, method (cash, bank transfer, card, JazzCash, Easypaisa, other), reference no., notes. | M | M3 |
| FR-PAY-02 | Multiple payments per bill (partial payments). | M | M3 |
| FR-PAY-03 | Attach a receipt (image/PDF) to a payment. | M | M3 |
| FR-PAY-04 | The bill status updates automatically from the payments (paid in full → Paid, part → Partially Paid). | M | M3 |
| FR-PAY-05 | The system decides which payable applies (within or after due date) from the payment date, and shows a late-fee notice. | S | M3 |
| FR-PAY-06 | "Mark as paid" quick action (full amount, today, last-used method). | M | M3 |

### 1.8 Consumption (CONS)

| ID | Requirement | Pri | MS |
|---|---|---|---|
| FR-CONS-01 | Store previous and current readings per bill for metered profiles. | M | M3 |
| FR-CONS-02 | Consumption = (current − previous) × multiplying factor, computed and cross-checked against the printed units. | M | M3 |
| FR-CONS-03 | Support meter replacement (readings reset) without negative consumption. | S | M3 |
| FR-CONS-04 | Flag estimated or average (non-actual) readings. | S | M3 |
| FR-CONS-05 | Optional peak / off-peak readings for TOU meters. | C | M4 |
| FR-CONS-06 | Show consumption history as a chart and a table per profile. | M | M3 |
| FR-CONS-07 | Show the effective rate (amount ÷ units) per bill. | S | M4 |

### 1.9 Dashboard (DASH)

| ID | Requirement | Pri | MS |
|---|---|---|---|
| FR-DASH-01 | Month summary: total billed, paid, outstanding. | M | M4 |
| FR-DASH-02 | One card per **pinned** bill profile: latest amount, units, status. Tapping a card opens that profile's dashboard. | M | M4 |
| FR-DASH-03 | Upcoming dues (next 14 days) and overdue list. | M | M4 |
| FR-DASH-04 | "Expected but missing" bills list (see FR-REM-04). | M | M5 |
| FR-DASH-05 | Recent activity (bills added, payments). | S | M4 |
| FR-DASH-06 | Month switcher. | M | M4 |

### 1.10 History & analytics (ANLT)

| ID | Requirement | Pri | MS |
|---|---|---|---|
| FR-ANLT-01 | Per-profile history table: period, units, amount, status. | M | M3 |
| FR-ANLT-02 | Range selector: month, quarter, year, custom. | M | M4 |
| FR-ANLT-03 | Spending by type, provider and profile, month by month. | M | M4 |
| FR-ANLT-04 | Average, highest and lowest bill per profile over a range. | M | M4 |
| FR-ANLT-05 | Comparisons: month-on-month, year-on-year, with the % change *and* the absolute figures. | M | M4 |
| FR-ANLT-06 | **Spend rule:** spending totals use *current charges + paid late fees*, never arrears (no double counting). A toggle can show "cash paid" (sum of payments) instead. | M | M4 |
| FR-ANLT-07 | Every chart has a "show data" table view. | S | M4 |
| FR-ANLT-08 | Budgets per category with progress. | C | later |

**Acceptance (FR-ANLT-06):** *Given* an August bill of Rs 7,000 left unpaid and a September bill showing current charges Rs 8,000 + arrears Rs 7,000 = Rs 15,000, *when* the user views spending for Aug–Sep, *then* the total is Rs 15,000 (7,000 + 8,000), not Rs 22,000.

### 1.11 Reminders & notifications (REM)

| ID | Requirement | Pri | MS |
|---|---|---|---|
| FR-REM-01 | In-app notifications: due in N days (default 3), due today, overdue. | M | M5 |
| FR-REM-02 | Per-user notification preferences (lead days, channels, quiet hours). | S | M5 |
| FR-REM-03 | Web Push notifications (PWA). | S | M5 |
| FR-REM-04 | **Expected-bill detection:** if a recurring profile has no bill for the current cycle by the typical issue day + grace period, notify "September electricity bill not added yet". | M | M5 |
| FR-REM-05 | Email notifications. | C | M5 |
| FR-REM-06 | "New bill retrieved" notification. | M | M6 |

### 1.12 Documents (DOC)

| ID | Requirement | Pri | MS |
|---|---|---|---|
| FR-DOC-01 | A bill can have many documents: scan, official PDF, receipt, other. | M | M2 |
| FR-DOC-02 | View, download and replace a document. Originals are kept immutable; the processed image is a derivative. | M | M2 |
| FR-DOC-03 | Store the extracted text of each document for search. | S | M2 |
| FR-DOC-04 | Documents are only reachable through short-lived signed URLs (see [08](08-security-privacy.md)). | M | M2 |
| FR-DOC-05 | Per-household storage quota shown in settings (to stay within free tiers). | S | M2 |

### 1.13 Search & filtering (SRCH)

| ID | Requirement | Pri | MS |
|---|---|---|---|
| FR-SRCH-01 | Global search across provider, category, profile name, reference no., bill no., notes and document text. | M | M4 |
| FR-SRCH-02 | Natural phrases such as `electricity september 2025` resolve to category + period. | S | M4 |
| FR-SRCH-03 | Filters: type, provider, profile, status, date range, amount range, source. | M | M3 |
| FR-SRCH-04 | Filters are reflected in the URL (shareable, survive a refresh). | S | M3 |

### 1.14 Export & backup (EXP)

| ID | Requirement | Pri | MS |
|---|---|---|---|
| FR-EXP-01 | Export bills and payments to CSV for a date range. | M | M4 |
| FR-EXP-02 | Full backup: ZIP containing JSON data + all documents. | S | M4 |
| FR-EXP-03 | Import from the backup ZIP. | C | later |
| FR-EXP-04 | Printable / PDF yearly summary per profile. | C | later |

### 1.15 Official retrieval (RETR)

| ID | Requirement | Pri | MS |
|---|---|---|---|
| FR-RETR-01 | A bill profile can opt into retrieval if its provider has a supported connector. | M | M6 |
| FR-RETR-02 | Connectors use **only official public bill-lookup pages or APIs**, keyed by reference number. No stored passwords. | M | M6 |
| FR-RETR-03 | The scheduler checks around the expected issue date, rate-limited (≤ 1 request per profile per day, global throttle per provider). | M | M6 |
| FR-RETR-04 | Retrieved bills go through the same validation and reconciliation as every other source. | M | M6 |
| FR-RETR-05 | Connector health is tracked. After repeated failures a connector is disabled automatically and users fall back to manual input, with nothing else affected. | M | M6 |
| FR-RETR-06 | "Fetch now" button (user-initiated, throttled). | S | M6 |

### 1.16 Admin & settings (ADM)

| ID | Requirement | Pri | MS |
|---|---|---|---|
| FR-ADM-01 | Household settings: currency (PKR default), timezone (Asia/Karachi default), date format, AI consent. | M | M1 |
| FR-ADM-02 | User can delete their account and all data (with a grace period). | M | M1 |
| FR-ADM-03 | Type management: add, rename, recolour and archive custom bill types. Each custom type picks a default kind. | M | M1 |
| FR-ADM-04 | Operator view of extraction accuracy and connector health (internal). | S | M6 |

---

## 2. Non-functional requirements

| ID | Area | Requirement |
|---|---|---|
| NFR-01 | Performance | Dashboard and lists respond in < 300 ms server-side (p95) and render in < 1 s on mid-range phones. |
| NFR-02 | Performance | Extraction completes in < 15 s p95 (async; the user is not blocked). |
| NFR-03 | Accuracy | ≥ 90% of fields correct before review on supported providers. All money values must pass validation or be flagged. |
| NFR-04 | Integrity | Money is stored as integers in minor units ([ADR-001](adr/001-money-as-integer-minor-units.md)). No floating-point money anywhere. |
| NFR-05 | Integrity | Every mutation is audited. Writes that span multiple entities are transactional. |
| NFR-06 | Availability | Core features work when every external integration (OCR/AI/retrieval) is down. |
| NFR-07 | Cost | Runs at Rs 0/month on free tiers ([ADR-005](adr/005-hosting.md)). Design for cold starts and sleeping instances. |
| NFR-08 | Security | OWASP ASVS L1 baseline, with household-scoped authorisation on every query (see [08](08-security-privacy.md)). |
| NFR-09 | Privacy | PII is minimised. Third-party AI only with consent. Data can be fully deleted. |
| NFR-10 | Usability | Mobile-first responsive UI, installable PWA, works on a 360 px wide screen. |
| NFR-11 | Accessibility | WCAG 2.1 AA: keyboard navigation, contrast, labels, and charts with table alternatives. |
| NFR-12 | i18n | English first. All strings externalised, RTL-ready for Urdu. Locale-aware number formatting (e.g. Rs 1,25,000 vs 125,000 as a setting). |
| NFR-13 | Maintainability | Automated tests (unit + integration) ≥ 70% on domain logic. CI on every push. Typed API contract (OpenAPI). |
| NFR-14 | Observability | Structured logs, error tracking (Sentry free tier), health endpoint. |
| NFR-15 | Portability | Stateless API container, standard Postgres, S3-compatible storage. Can move hosts without code changes. |
| NFR-16 | Data durability | Daily automated DB backup. User-triggered full export. |

---

## 3. Traceability: original concept → requirements

| Concept section | Requirement IDs |
|---|---|
| §4 Categories | FR-ADM-03, FR-ACCT-03/08/09 |
| §5A Scan | FR-EXTR-01, 03, 04, 05–09 |
| §5B Upload | FR-EXTR-02, 05–09 |
| §5C Manual entry | FR-BILL-01, FR-BILL-04 |
| §6 Official retrieval | FR-RETR-01…06 |
| §7 Deduplication | FR-RECO-01…03 |
| §8 Conflict detection | FR-RECO-04…06 |
| §9 Canonical bill record | FR-BILL-01, 02, 08; [04-domain-model](04-domain-model.md) |
| §10 Dashboard | FR-DASH-01…06 |
| §11 Bill history | FR-ACCT-10, FR-ANLT-01, 02 |
| §12 Consumption | FR-CONS-01…07 |
| §13 Financial analytics | FR-ANLT-03…07 |
| §14 Payments | FR-PAY-01…06 |
| §15 Reminders | FR-REM-01…06 |
| §16 Documents | FR-DOC-01…05 |
| §17 Search & filtering | FR-SRCH-01…04 |
| §18 Multiple properties | FR-PROP-01…03 |
| §19 Data model | [04-domain-model](04-domain-model.md) |
| §20 Core principle | NFR-06, [ADR-002](adr/002-retrieval-is-optional-channel.md) |
| §21 MVP | [09-roadmap](09-roadmap.md) |
| *New* | Bill profiles & profile dashboards (FR-ACCT-08…14), FR-AUTH-*, FR-BILL-02/09/10, FR-EXTR-11/13/14, FR-REM-04, FR-EXP-*, NFR-* |
