# ADR-001 — Store money as integer minor units

- **Status:** Accepted
- **Date:** 2026-10-03

## Context
Bills, payments and analytics sum many amounts. Floating-point arithmetic introduces rounding errors (`0.1 + 0.2 ≠ 0.3`), which is unacceptable for financial records and for matching values between sources.

## Options considered
1. `float/double`: simple, but loses precision. **Rejected.**
2. `NUMERIC(14,2)` in the DB + a decimal library in code: correct, but each language's decimal handling differs and JSON serialisation is error-prone.
3. **`BIGINT` minor units (paisa) + a `currency` code:** exact and fast, identical across languages and JSON.

## Decision
All money is stored and transmitted as **integer minor units** (`amount_minor`) together with an ISO-4217 `currency`. Conversion to display format happens only at the UI edge. Rounding (e.g. rates = amount ÷ units) applies only to derived display values, never to stored money.

## Consequences
- API fields are named `*_minor` so the unit is never ambiguous.
- Extraction normalises "Rs. 8,420.00" → `842000`.
- Multi-currency is possible later (the minor-unit exponent comes from the currency table).
