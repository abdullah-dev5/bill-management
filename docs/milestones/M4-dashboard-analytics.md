# M4 — Dashboard, analytics & export: progress notes

Branch `m4-dashboard-analytics` (from `main`). Exit criteria from [09-roadmap](../09-roadmap.md) M4: answer every
question in [01](../01-product-vision.md) §3 from the UI; dashboard < 1 s.

| Phase | Scope | State |
|---|---|---|
| 1 — Analytics | Ranges, spend rule, breakdowns, comparisons, profile stats, consumption, data tables | **Done** |
| 2 — Search & filters | Global search, natural phrases, more bill filters, recent activity | Next |
| 3 — Export & exit | CSV export, backup ZIP, dashboard speed, end-to-end test of the six questions | — |

## Phase 1 — Analytics (delivered)
| Area | What's in | Requirements |
|---|---|---|
| Page & navigation | New **Analytics** page; on phones it takes the Bills spot in the bottom bar (Bills stays one tap away from Home and Analytics; desktop sidebar has both) | M1 decision |
| Ranges | This month · last 3 months · this year · last 12 months · custom (month pickers); kept in the URL | FR-ANLT-02 |
| Spend rule | Spending = current charges + adjustments + late fees actually paid, in the bill's month; arrears never counted twice. "Cash paid" switch: payments in the month they were made | FR-ANLT-06 |
| Breakdowns | Month by month stacked by group (Utilities / Household / Other); by bill type (sub-categories roll up, e.g. electrician → Maintenance); by provider; by profile | FR-ANLT-03 |
| Profile stats | Average, highest (linked to the bill) and lowest bill per profile over the range | FR-ANLT-04 |
| Comparisons | Total vs the previous period of the same length and vs the same months last year, as % *and* rupees; per type vs last year; per profile vs previous | FR-ANLT-05 |
| Consumption | Units per month for metered profiles, using bills and, where a month has no bill, the history printed on bills; total and change vs last year | FR-CONS-06 |
| Data tables | "Show data" under the monthly chart; breakdown bars show their values | FR-ANLT-07 |

### Charts
Built to the data-viz method: the three group colours are fixed (blue, orange, aqua), validated for colour-blind
separation in light and dark themes against the app's card colour; 2px gaps between stacked segments, ≤ 24px
columns with 4px rounded tops, hairline grid, clean axis ticks, one tooltip per column listing every group (hover and
keyboard focus), a legend for two or more series, and a table view (required because aqua is below 3:1 contrast on
the light card).

### Quality
- Go: the FR-ANLT-06 acceptance example (Aug Rs 7,000 unpaid + Sep Rs 8,000 + Rs 7,000 arrears = Rs 15,000, not
  22,000), paying with arrears doesn't change spend, cash basis, a paid late fee counts, breakdowns by group/type/
  provider/profile, sub-category roll-up, profile average/high/low and highest bill, comparisons, units from bills
  and printed history, range validation, household isolation.
- Web: unit tests for axis scaling, presets and change maths; end-to-end on phone and desktop.
- Fixed: two more tests compared the database clock with the app clock or passed before a payment had saved.
