# M4 — Dashboard, analytics & export: progress notes

**Status: complete.** Both exit criteria are met (see Phase 3).

Branch `m4-dashboard-analytics` (from `main`). Exit criteria from [09-roadmap](../09-roadmap.md) M4: answer every
question in [01](../01-product-vision.md) §3 from the UI; dashboard < 1 s.

| Phase | Scope | State |
|---|---|---|
| 1 — Analytics | Ranges, spend rule, breakdowns, comparisons, profile stats, consumption, data tables | **Done** |
| 1b — Gas bills & UI consistency | Layout reader for any provider (tested on SSGC), themed date/month pickers, clearer dropdowns | **Done** |
| 2 — Search & filters | Global search, natural phrases, more bill filters, recent activity | **Done** |
| 3 — Export & exit | CSV export, backup ZIP, dashboard speed, end-to-end test of the six questions | **Done** |

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

## Phase 3 — Export & exit (delivered)
| Area | What's in | Requirements |
|---|---|---|
| CSV export | Settings → Your data: bills (by bill date) and payments (by payment date) as spreadsheets for this year, last year, the last 12 months, everything, or custom dates. Amounts in rupees, UTF-8 with a BOM so Excel opens it correctly, and cells that a spreadsheet would run as a formula are neutralised. Every member can export. | FR-EXP-01 |
| Full backup | One ZIP: household, profiles (with reference numbers), bills, payments, document list, custom types and providers, printed history as JSON, plus every photo and PDF, and a README. Owner only, at most 10 an hour; a missing file is listed rather than failing the backup. Versioned (`format: 1`) for a future import (FR-EXP-03). | FR-EXP-02 |
| Dashboard speed | A Go test seeds a household with 5 years of bills (360 bills and their payments) and times the home screen, bill lists, analytics (12 and 36 months), search, activity and profiles. Slowest p95 was 86 ms, against the 300 ms server budget; the 1 s render target is met with room to spare. | NFR-01, M4 exit |
| Exit test | An end-to-end test on phone and desktop answers all six questions from the UI: electricity cost this year and maintenance spend (Analytics → By bill type, electrician rolls up to Maintenance), gas used last winter (search understands the season; Analytics totals 160 units), did internet go up (+20% vs the previous 3 months), what is unpaid now (Bills → Unpaid), and where is September's electricity bill (search → opens it). | M4 exit |

Also fixed: Settings → Household showed "Choose…" for currency and timezone, because the form received its values after the new dropdowns had mounted. The form now waits for the household to load; the export test checks it.

## Phase 2 — Search & filters (delivered)
| Area | What's in | Requirements |
|---|---|---|
| Global search | One box for everything: profile names, bill types (and their sub-categories), providers, bill numbers, notes, descriptions, tags, payment references and the text of scanned bills/receipts. Every word must match. Opens with **/** on desktop, from the sidebar, or the search button on phones; the query stays in the URL. | FR-SRCH-01 |
| Phrases | "electricity september 2025", "gas last winter", "internet this year", "lesco last month", "unpaid", "2026-08", everyday words (bijli, wifi, pani…) become type, provider, period and status filters; the page shows what it understood as chips. A month without a year means the latest one; "last winter" is the winter just gone. | FR-SRCH-02 |
| Reference numbers | Typed any way ("09 12345 0012345", with dashes) they find the profile and its bills, matched by keyed hash; the number is never stored or shown in plain text (shown as •••• 2345). | FR-SRCH-01, docs/08 |
| Document text | Matching documents with a short highlighted snippet, linking to their bill. | FR-SRCH-01, FR-DOC-03 |
| Bill filters | Bill type (multi, includes sub-categories), provider, amount range, bill date range, how it was added (photo / uploaded file / typed in), alongside status and profile; shown as removable chips with "Clear all", kept in the URL. | FR-SRCH-03/04 |
| Recent activity | Home lists the latest bills added, payments (amount and method), removed payments, documents and new profiles, with "by Name" when more than one person adds things. | FR-DASH-05 |

Quality: Go tests for the phrase parser (months, years, seasons in October and January, statuses, aliases, providers, sub-categories), search across all fields, reference numbers typed with spaces, literal wildcards, household isolation, every bill filter, and the activity feed; an end-to-end test of search, filters and activity on phone and desktop.

## Phase 1b — Gas bills & UI consistency (delivered)
| Area | What's in |
|---|---|
| Any bill layout | Bills that print labels in a row with values below (gas, water, telecom…) are read for any provider, not only SSGC: account summary rows, the tear-off stub, meter tables and month-by-month history, each value checked against its column. Reference numbers with a bracketed check digit ("5051274880 (4)") are read without it. The SSGC sample is a test case, with no SSGC-specific code. Gemini gets the same rules. |
| Provider catalogue | SSGC reference label "Customer No.", format `^\d{10}$` (migration 00010). The profile form tells you what your provider calls the number. |
| Date & month pickers | One themed field everywhere (bill form, payments, analytics custom range): type a date the way it's printed (8/10/2026, 08-Oct-26, Sep 2026…) or pick from a calendar that matches the cards, with Monday-first weeks, a month/year jump, today marker, Today/Clear, and full keyboard support. Text that isn't a date is put back, not saved. |
| Dropdowns | Themed dropdowns with group headings and a hint per option: providers show the full company name and "Added by you"; bill types grouped Utilities / Household / Other; payment methods grouped In person / Bank & card / Mobile wallet; billing cycles and profile kinds explained; "None" options say what they mean. |
| Dates on screen | Short month names are fixed ("Sep", never the browser's "Sept"). |

Quality: Go tests on the masked SSGC sample and on SNGPL-, telecom- and water-style layouts, plus rows that don't fit their columns being ignored; unit tests for typed date/month parsing; an end-to-end test that drives the dropdowns, month grid, calendar (mouse and keyboard) and typed dates on phone and desktop.

Later (see M6): fetching bills from official sites by reference number, possibly from a public page.
