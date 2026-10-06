# Official retrieval — access requests

Automatic retrieval needs a provider's permission (see [retrieval-connectors.md](retrieval-connectors.md)).
These requests are slow in Pakistan, so track them here and start early. Automatic fetch is **blocked
on provider access, not on the app** — the connector framework is ready (M6).

| Provider | Covers | Contact / channel | Requested | Status | Notes |
|---|---|---|---|---|---|
| PITC | LESCO, HESCO, IESCO, GEPCO, FESCO, MEPCO, PESCO, QESCO, SEPCO, TESCO | bill.pitc.com.pk / PITC IT | — | not started | Ask for an official bill-data API or data-sharing agreement. Public page has a CAPTCHA, so assisted only for now. |
| K-Electric | K-Electric | ke.com.pk customer services / IT | — | not started | |
| SSGC | SSGC gas | ssgc.com.pk IT | — | not started | Public page has a CAPTCHA. |
| SNGPL | SNGPL gas | sngpl.com.pk IT | — | not started | Site did not respond during M6; recheck. |

When a provider grants access: record it here, implement the connector from the template, and enable
it behind its credential.
