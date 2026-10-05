# M6 — Official retrieval & full reconciliation: progress notes

Branch `m6-retrieval-reconciliation`, started from `m5-reminders-pwa` while PR #7 awaits review; its PR targets `main`
after #7 is merged. Exit criteria from [09-roadmap](../09-roadmap.md) M6: a retrieved bill and a scanned bill of the same
month merge into one record; when a connector is broken, nothing else is affected.

| Phase | Scope | State |
|---|---|---|
| 1 — Reconciliation | Sources per bill, field conflicts, resolution, side-by-side sources, undo merge | **Done** |
| 2 — Connector framework | Sandboxed, rate-limited, health-tracked connectors; scheduler around the issue date; Fetch now; pilot connector | Next |
| 3 — Exit | Retrieved + scanned bill merge end to end; broken-connector isolation; public lookup page (if the providers' terms allow) | — |

## Phase 1 — Reconciliation (delivered)
| Area | What's in | Requirements |
|---|---|---|
| Sources | Every bill records each channel that produced or confirmed it (typed in, scan, uploaded file, automatic, and official in phase 2) with the values it gave. Existing bills were backfilled with how they arrived (migration 00014). | docs/06 §1, FR-RECO-06 |
| Merge | Attaching a scan or upload to an existing bill compares it field by field (billing month, bill number, issue/due date, current charges, arrears, payable, after-due amount, readings, units): empty fields are filled in, equal values confirm, differing values become **conflicts**. The bill's own value never changes silently. Equality: money and dates exact, readings ±0.5, bill numbers ignoring spaces/punctuation; zero arrears from a scan counts as "not read". | FR-RECO-02/04 |
| Resolve | "Needs review" on the bill: for each difference, keep the bill's value, use the other copy's, **the provider revised it** (use the new value, noted as a revision), or enter the right one. Each choice is in the bill's history. A later copy that agrees with the bill settles an open conflict by itself ("confirmed"). | FR-RECO-05 |
| Side by side | A sources table on the bill: each detail as on the bill and as each copy gave it, differences highlighted. | FR-RECO-06 |
| Undo merge | Within 30 days, **Separate** a merged copy: details only it filled are emptied again (unless changed since), its differences are dropped, and its document is detached, with a **Save it as a bill** shortcut to the scan screen. | FR-RECO-07, docs/06 §5 |
| Attention | Home shows "N bills need review"; the bills list has a **Needs review** filter; saving a scan into an existing bill says how many details differ. | docs/06 §4 |

Quality: Go tests for sources on create, merge (filled / confirmed / conflicting fields, bill unchanged), review count and
filter, resolving with source / revised / keep / manual (with validation), already-resolved and cross-household refusals,
automatic confirmation by an agreeing copy, separate (fields emptied, document detached, history) and its 30-day limit; an
end-to-end test on phone and desktop: scan a PDF of a bill already typed in, attach, see the differences, decide from Home's
review link, then separate.

Fixed along the way:
- Source and conflict timestamps used the database clock; they now use the app clock like every other query.
- On phones, a wide table (or the hidden screen-reader text inside it) could widen the whole page, so the browser zoomed out
  and taps landed beside buttons. Tables now scroll inside a positioned box, and the main area clips sideways overflow;
  the end-to-end test checks the page width.
- Translation helpers no longer take the typed `t` function as a parameter (TypeScript hit its depth limit as the strings
  grew); they use `lib/tr.ts`.
