# Glossary

| Term | Meaning |
|---|---|
| **Arrears / Previous balance** | Unpaid amount from earlier bills, carried into the current bill's total. Excluded from spending analytics to avoid double counting. |
| **Bill (canonical)** | The single authoritative record for one bill profile's billing cycle (or one entry in an expense-log profile), regardless of how many sources contributed to it. |
| **Bill Source** | One arrival of bill data: manual, scan, upload, official, or email. |
| **Billing period / Bill month** | The consumption period a bill covers. Stored as `period_start/end` and the `period_key` (`YYYY-MM`). |
| **Consumer / Reference number** | The provider's account identifier printed on the bill (e.g. LESCO 14-digit reference no.). The primary identity of a utility bill profile. |
| **Current charges** | Charges for this period only (energy + taxes + fees), without arrears. |
| **DISCO** | Electricity distribution company (LESCO, IESCO, GEPCO, FESCO, MEPCO, PESCO, HESCO, QESCO, SEPCO, TESCO…); K-Electric serves Karachi. |
| **Draft** | A bill created by extraction that the user has not yet verified. Excluded from analytics. |
| **Electricity duty / TV fee / Meter rent** | Fixed or percentage charges that appear as line items on electricity bills. |
| **Expected bill** | A bill the system expects for a recurring bill profile in a cycle. "Missing" if it is not added by the expected date. |
| **Extractor** | A pluggable component that turns a document into structured fields (Gemini, OCR.space + template, on-device Tesseract.js + template). |
| **Field conflict** | Two sources disagree on a field's value. Needs user resolution. |
| **FPA (Fuel Price Adjustment)** | A periodic electricity surcharge or credit, separate from the base tariff. |
| **GST** | General Sales Tax line item. |
| **hm³** | Hectocubic metre (100 m³), the unit for gas consumption on SNGPL/SSGC bills; MMBtu also appears. |
| **Line item** | One row of a bill's charge breakdown. |
| **MF (Multiplying Factor)** | The meter constant. Units = (current − previous) × MF. Usually 1 for homes. |
| **Minor units** | The smallest currency unit (paisa for PKR). All money is stored as integers of these. |
| **Payable within / after due date** | The two totals printed on Pakistani bills. The after-due amount includes the late payment surcharge (LPS). |
| **Period key** | `YYYY-MM` billing month. With the bill profile, it forms the bill identity key. |
| **PITC** | Power Information Technology Company. Produces the common bill format and online bill lookup for the DISCOs. |
| **Provenance** | Which source a field's value came from, with its confidence. |
| **Reconciliation** | Matching incoming bill data to existing bills, merging, and surfacing conflicts. |
| **Bill Profile** | A named, user-created tracker for one bill or expense stream, with a type and a kind (e.g. "Home Electricity", "Shop Electricity", "Flat Rent", "Home Maintenance"). Each profile has its own dashboard. |
| **Profile kind** | How a profile behaves: `metered` (utility with readings), `recurring` (fixed amount each cycle), or `expense_log` (any number of entries). It decides the profile's fields and dashboard widgets. |
| **Bill type** | The category a profile belongs to (Electricity, Gas, Rent, Maintenance, or a custom type). |
| **Slab** | A tariff tier. The per-unit price changes with consumption (e.g. protected vs. non-protected users). |
| **TOU (Time of Use)** | Meters with separate peak and off-peak registers. |
| **Verification** | The user's confirmation of the extracted fields, which moves a bill out of Draft. |
