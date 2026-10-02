# 01 — Product Vision

## 1. One-line pitch

A centralised bill management platform that turns physical, digital, manual and automatically retrieved bills into **one organised, searchable and trackable financial history**.

## 2. Short pitch

Managing household bills shouldn't mean keeping piles of paper, searching WhatsApp, or checking several utility websites. This application brings everything into one place. Users can:

- scan a physical bill,
- upload a PDF or image,
- enter a bill or an informal expense by hand, or
- optionally connect supported official providers for automatic retrieval.

The system extracts and **validates** bill data. It prevents duplicates, tracks payments and meter consumption, keeps the original documents, and shows the full history of household spending.

**Differentiator:** automatic retrieval is a *backup channel*, not the foundation. If a provider website changes or goes down, nothing else breaks.

## 3. Problem

Bills and household expenses are scattered across paper bills, WhatsApp, PDFs, email, provider websites, banking apps, handwritten receipts and personal notes. Simple questions become hard to answer:

- How much did electricity cost me this year?
- How much gas did I use last winter?
- Did my internet bill go up?
- Which bills are unpaid right now?
- Where is my September electricity bill?
- How much did I spend on household maintenance?

## 4. Target users & personas

| Persona | Description | Key needs |
|---|---|---|
| **Family organiser** (primary) | Runs the household budget and pays 5–10 recurring bills | Never miss a due date, see the monthly total, find old bills |
| **Tenant** | Pays utilities and rent, sometimes shares them with flatmates | Proof of payment, consumption history to dispute bills |
| **Homeowner** | Utilities plus maintenance, repairs and appliances | Long-term spending, maintenance history |
| **Landlord / property manager** (secondary) | Several properties, each with its own meters | Per-property view, per-tenant attribution (later) |
| **Small office / shop / freelancer** (secondary) | Business utilities and subscriptions | Category totals, export for accounting |

The first launch targets **Pakistan**: PKR, and providers such as LESCO, IESCO, K-Electric, SNGPL, SSGC, PTCL, WASA and ISPs. The design keeps currency, provider and locale configurable.

## 5. Product principles

1. **Works without retrieval.** Every core feature works with manual, scan and upload input only.
2. **Never silently overwrite.** Conflicting data is shown to the user, never replaced without a trace.
3. **The human is the final verifier.** Machine extraction suggests values and the user confirms them. Accuracy comes from validation plus review, not blind trust in OCR.
4. **One bill, many sources.** A bill is one canonical record. Scans, PDFs and official copies attach to it.
5. **Show the numbers.** Analytics always show the underlying figures, never conclusions without them.
6. **The user owns the data.** Full export at any time. No lock-in.
7. **Production quality, zero running cost.** Build to a real-product standard but run within free tiers (see [07](07-architecture.md)).
8. **Privacy by default.** Bills contain names, addresses and sometimes CNIC numbers. Sending a document to a third-party AI requires explicit consent.

## 6. Bill categories

Categories are user-extensible. Defaults:

- **Utilities:** Electricity, Gas, Water, Internet, Mobile/Telephone, Cable/TV
- **Household:** Rent, Maintenance, Repairs, Cleaning, Security, Electrician/Plumber, Appliances
- **Other:** Insurance, Education, Subscriptions, Government fees, Custom, Other

Utility categories (electricity, gas, water) are **metered**: they support meter readings and consumption analytics.

## 7. Scope

### In scope: MVP (see [09-roadmap](09-roadmap.md) M1–M5)
- Bill CRUD with archive, categories, providers, bill profiles
- Image/PDF upload, camera capture, extraction with a verification screen
- Manual entry (no document needed)
- Payments (full / partial), payment history, receipts
- Meter readings and consumption history
- Monthly / yearly history, spending by category and provider
- Dashboard: current month, outstanding, upcoming, recent
- Document storage and search
- **Additions to the original concept:** in-app due-date reminders, **expected-bill detection**, CSV export, audit log, household members, and a default property (hidden in the UI)

### Later
- Official provider retrieval and advanced reconciliation (M6)
- Email and push notifications
- Multi-property UI, email ingestion (M7)
- Bank/payment integrations, advanced analytics, budgets

### Out of scope (for now)
- Paying bills from inside the app (we track payments; we don't process them)
- Storing provider login credentials
- Accounting or tax filing

## 8. Success metrics

| Metric | Target |
|---|---|
| Time to add a scanned bill (capture to saved) | < 30 s median |
| Fields correct after the verification screen | ≥ 99% (user-confirmed) |
| Fields correct straight from extraction, before review | ≥ 90% on supported providers |
| Duplicate bills created when the same bill comes from two sources | 0 (reconciliation catches them) |
| Dashboard load time | < 1 s on a 4G phone (warm) |
| Running cost | Rs 0 / month (free tiers) |

## 9. Key refinements over the original concept

The original concept is solid. These additions come from how Pakistani utility bills actually work and from data-integrity needs:

- **Two payable amounts:** *within due date* and *after due date* (late surcharge). Both are stored.
- **Arrears handling:** the previous unpaid balance is printed inside the current bill's total. Spending analytics use **current charges**, so the same money is never counted twice.
- **Line items** (energy charge, FPA, GST, electricity duty, TV fee, meter rent, …) as a generic structure.
- **Barcode / reference number** detection for instant identification of provider and bill profile.
- **Meter specifics:** multiplying factor, meter replacement, estimated bills, peak/off-peak (TOU) readings, per-profile units (kWh, hm³, gallons).
- **Expected bills:** each recurring bill profile expects one bill per cycle, and a missing bill is flagged. Reminders then work even without retrieval.
- **Per-field provenance and confidence**, plus an **audit log**.
- **Offline capture** in the PWA and **client-side image compression**.
