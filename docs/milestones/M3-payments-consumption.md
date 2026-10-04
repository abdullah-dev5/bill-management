# M3 — Payments, consumption & history: progress notes

Branch `m3-payments-consumption` (based on `m2-documents-extraction`). Exit criteria from [09-roadmap](../09-roadmap.md) M3.
M3 ships in three phases; each is committed and pushed, and the branch is merged once all three are done.

| Phase | Scope | State |
|---|---|---|
| 1 — Payments & monthly total | Monthly bills total (FR-DASH-07), receipts on payments, arrears settlement | **Done** |
| 2 — Readings & consumption | Meter readings, multiplying factor, replacement, estimated readings, consumption chart | **Done** |
| 3 — Profile dashboards | Overview / Bills / Payments / Documents tabs, history table, URL filters, full-month end-to-end test | **Done** |

## Phase 1 — Payments & monthly total (delivered)
| Area | What's in | Requirements |
|---|---|---|
| Monthly bills total | Home leads with one figure: what the household has to pay for the month's bills, all profiles together. Paid, left to pay (at today's amounts, late fees included once due), totals per group (Utilities / Household / Other) and per profile, and the next bill due. Expense entries count in the month of their date. Updates as soon as a bill is added, typed, scanned or uploaded | FR-DASH-07 |
| Earlier months | Money still owed on earlier months' bills is shown separately, except amounts a current bill already carries as arrears (so nothing is counted twice) | FR-ANLT-06 |
| Receipts | "Attach receipt" while recording a payment (photo or PDF); the payment row links to it; deleting the payment keeps the document on the bill | FR-PAY-03 |
| Arrears settlement | When a bill carrying arrears is paid in full, the earlier unpaid bills that make up those arrears (one or several months, at their original or late amount) are marked paid through it, with a history entry. Removing the payment reopens them | docs/04 §3 |

### Quality
- Go: tests for the FR-DASH-07 acceptance example, settlement of one and of two months, non-matching arrears, undo,
  receipts (including a receipt from another bill being rejected).
- Fixed: a few queries compared the database clock with the app clock; they now use one clock, so tests no longer
  depend on the time of day. Network deadlines for file transfers use real time.
- Web: end-to-end on phone and desktop — monthly total on Home, paying with a receipt, the earlier bill settled.
- The sign-up rate limit is relaxed in development only (end-to-end runs create many accounts); production keeps 10 per hour.

## Phase 2 — Readings & consumption (delivered)
| Area | What's in | Requirements |
|---|---|---|
| Readings | Metered bills store the previous and present readings, the units and the multiplying factor (taken from the profile, editable per bill) | FR-CONS-01 |
| Consumption | Units = (present − previous) × MF, worked out as you type; printed units within 1 of that are accepted, larger differences are explained and rejected | FR-CONS-02 |
| Meter replacement | "Meter replaced this month": the readings restart and the printed units are used, so consumption never goes negative | FR-CONS-03 |
| Estimated readings | The existing "estimated" flag is shown on the bill | FR-CONS-04 |
| Next bill | A new bill starts from the profile's last present reading | — |
| Scanned bills | Readings, units and MF are filled in from the extraction; units confirmed by the readings no longer need checking | FR-EXTR-08 |
| Consumption history | The profile's history chart switches between amount and units, combining its bills with the units printed on them | FR-CONS-06 |

### Quality
- Go: readings arithmetic, MF from the profile, lower reading, a single reading, contradicting units, replaced meter
  with and without units, rounding tolerance, readings ignored for non-metered profiles, last reading, history units.
- Web: unit tests for reading parsing and the units calculation; the scan end-to-end test now checks the readings are
  filled in, shown on the bill ("18,690 → 18,854 · 164 kWh") and charted as units on the profile.

## Phase 3 — Profile dashboards (delivered)
| Area | What's in | Requirements |
|---|---|---|
| Profile tabs | Overview · Bills · Payments · Documents · Settings; the tab is kept in the URL | FR-ACCT-10 |
| Bills tab | Status filter (all / unpaid / overdue / paid) kept in the URL, so a reload or a shared link keeps it | — |
| Payments tab | Every payment towards the profile's bills, newest first, with the bill, method, total and receipts | FR-PAY-01…03 |
| Documents tab | Every bill photo, PDF and receipt of the profile, opening in the document viewer | FR-DOC-01 |
| History table | Month, units, bill, paid and the bill's status (or "Printed" for history read from a bill), linked to the bill; spend and consumption charts above it | FR-ANLT-01, FR-CONS-06 |

### Exit criterion
**"A full month of real household bills can be tracked end to end"** — an end-to-end test on phone and desktop:
sign up → pick electricity, gas, internet and rent → upload the electricity bill PDF and save it with its history →
type the gas bill with meter readings and pay it with a receipt → rent paid in two parts → internet already paid →
a quick expense → Home shows the month's total, paid and left (Rs 50,850 / 25,850 / 25,000) → finish paying from the
URL-filtered "Unpaid" list → nothing left → the electricity profile's history, payments, documents and filtered bills.

### Fixed during phase 3
- **Filtering bills by several statuses (the "Unpaid" filter) failed since M1**: the API expected one comma-separated
  value while the app sends repeated parameters. The API now takes the standard repeated form; a test covers it.
- The dev server reloaded open pages whenever end-to-end tests wrote traces; it now ignores the test output folders.
- A React key warning on the Profiles page while bill types were loading.

## M3 summary
Payments with receipts, arrears settlement, the monthly bills total, meter readings and consumption, and complete
profile dashboards. Tests: Go integration and unit tests (API package coverage 76%), 36 web unit tests, 14 end-to-end
tests on phone and desktop (plus 4 optional live Gemini/OCR tests).
