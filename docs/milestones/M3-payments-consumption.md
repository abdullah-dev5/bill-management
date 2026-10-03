# M3 — Payments, consumption & history: progress notes

Branch `m3-payments-consumption` (based on `m2-documents-extraction`). Exit criteria from [09-roadmap](../09-roadmap.md) M3.
M3 ships in three phases; each is committed and pushed, and the branch is merged once all three are done.

| Phase | Scope | State |
|---|---|---|
| 1 — Payments & monthly total | Monthly bills total (FR-DASH-07), receipts on payments, arrears settlement | **Done** |
| 2 — Readings & consumption | Meter readings, multiplying factor, replacement, estimated readings, consumption chart | Next |
| 3 — Profile dashboards | Overview / Bills / Payments / Documents tabs, history table, URL filters, full-month end-to-end test | — |

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
