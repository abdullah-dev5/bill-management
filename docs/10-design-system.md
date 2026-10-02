# 10 — Design System

The visual language of the app. It is adapted from the reference token file the owner supplied: a warm off-white canvas, deep charcoal, a lime accent, pill buttons and colour-blocking instead of shadows. It is reworked for a **finance app** that people check on their phones every day.

> **Feel:** calm, clear, confident. Numbers come first. One clear action per screen.

---

## 1. Principles

1. **Numbers are the hero.** The amount due, its status and the date are the largest and clearest things on a screen.
2. **Colour-blocking for depth.** Hierarchy comes from surface colour changes (canvas → card → dark featured card), not from shadows.
3. **Pills mean "you can press this".** Every button and chip is a full pill. Cards are softly rounded (24 px).
4. **Lime is a spotlight, not paint.** It marks the single most important highlight on a screen: the add button, the active tab, "needs review". It is never used as large backgrounds on light surfaces.
5. **Status is never colour alone.** Every status has a colour, an icon and a word.
6. **Generous whitespace.** Bills are stressful; the interface shouldn't be.

---

## 2. Colour

### 2.1 Core tokens

| Token | Light | Dark | Use |
|---|---|---|---|
| `--bg` | `#ECEBE8` | `#0F0F10` | Page background (canvas) |
| `--card` | `#F8F8F8` | `#1D1D1E` | Standard cards, lists, inputs |
| `--card-muted` | `#E4E4E0` | `#29292A` | Secondary buttons, icon tiles, chips at rest |
| `--featured` | `#2D2D2D` | `#303031` | Featured card (current bill), bottom nav pill, primary button |
| `--on-featured` | `#FFFFFF` | `#FFFFFF` | Text on featured surfaces |
| `--text` | `#000000` | `#F7F7F7` | Headings and amounts |
| `--text-body` | `#29303D` | `#E2E2E2` | Body text |
| `--text-muted` | `#5A5F68` | `#9F9F9F` | Secondary labels, captions (≥ 4.5:1 on both `--bg` and `--card`) |
| `--border` | `#D0CFCC` | `#353536` | Hairlines, input borders |
| `--accent` | `#E3EF7A` | `#E3EF7A` | Lime spotlight (see principle 4) |
| `--on-accent` | `#2D2D2D` | `#2D2D2D` | Text and icons on lime (11:1) |
| `--focus` | `#2D2D2D` | `#E3EF7A` | Focus ring |

The reference file's electric blue (`#0000EE`) is **not used**. It is the default browser link colour and clashes with the warm palette. Inline links use `--text` with an underline.

### 2.2 Status colours (bill and payment states)

Badges use a tinted background with dark text. Every pair passes WCAG AA (≥ 4.5:1).

| Status | Icon | Light text / bg | Dark text / bg |
|---|---|---|---|
| **Paid** | ✓ check | `#1F6B3A` / `#DCEFD9` | `#7BD88F` / `#1F3325` |
| **Unpaid** | ○ circle | `#29303D` / `#E4E4E0` | `#C9C9C9` / `#2A2A2B` |
| **Due soon** | ⏰ clock | `#7A4A00` / `#F6E3B8` | `#F2C46B` / `#3A2E14` |
| **Overdue** | ⚠ alert | `#9B1C1C` / `#F6D6D3` | `#F28B82` / `#3D1F1E` |
| **Partially paid** | ◐ half | `#1E4E8C` / `#DCE6F5` | `#8AB4F8` / `#1D2B40` |
| **Needs review / Draft** | ✎ pencil | `#2D2D2D` / `#E3EF7A` | `#E3EF7A` / `#33361A` |
| **Cancelled** | — dash | `#5A5F68` / `#E4E4E0` | `#9F9F9F` / `#2A2A2B` |

Status colours are reserved for states. They are never used as profile colours or chart series.

### 2.3 Text on surfaces: contrast rules
- On `--featured` (charcoal), text is always `--on-featured` (white). Black text on the dark surface fails contrast (2.1:1).
- Lime is used **behind charcoal text**, never as text on light backgrounds.
- `--text-muted` is the lightest text allowed on light surfaces.

### 2.4 Profile colours (also the chart series palette)

Each bill profile picks one colour. That colour is its **identity everywhere**: its icon tile, its line in charts and its legend entry. In charts the colour follows the profile, never its rank.

The order is fixed and was run through the data-viz palette validator against our card surfaces: lightness band, chroma, colour-blind separation and normal-vision separation **pass in both themes**.

| Slot | Name | Light | Dark |
|---|---|---|---|
| 1 | Blue | `#2A78D6` | `#3987E5` |
| 2 | Orange | `#EB6834` | `#D95926` |
| 3 | Aqua | `#1BAF7A` | `#199E70` |
| 4 | Yellow | `#EDA100` | `#C98500` |
| 5 | Pink | `#E87BA4` | `#D55181` |
| 6 | Green | `#008300` | `#008300` |
| 7 | Violet | `#4A3AA7` | `#9085E9` |
| 8 | Red | `#E34948` | `#E66767` |

Rules:
- New profiles get the next unused slot by default. The user can change it.
- **Icon tile:** the profile colour at 16% opacity behind a `--text` icon, so icons stay readable whatever the colour.
- **Light-mode contrast:** aqua, yellow and pink are below 3:1 on `#F8F8F8`, so every chart has **direct labels or its table view** (FR-ANLT-07).
- **Comparison charts** (type overview): at most 8 profiles. Beyond that, the smallest fold into "Other". If two compared profiles share a colour, the later one borrows the next free slot for that chart only, and direct labels are mandatory.
- **Single-profile charts** (a profile's own dashboard) use the profile colour and need no legend; the chart title names the series.

---

## 3. Typography

**Typeface:** [Red Hat Display](https://fonts.google.com/specimen/Red+Hat+Display) (free, Google Fonts). It is self-hosted through `@fontsource/red-hat-display` so the PWA works offline.
Fallback: `"Red Hat Display", ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif`.
Urdu (later, RTL): `"Noto Nastaliq Urdu"`.
Weights used: **400, 500, 600** only. The reference's decorative script face (Caveat) is not used.

| Token | Size / line-height | Weight | Use |
|---|---|---|---|
| `amount-hero` | 40 px / 1.1 (mobile 34 px) | 500 | Amount on the featured bill card |
| `display` | 44 px / 1.2 (mobile 28 px) | 500 | Page titles on desktop (rare) |
| `heading-lg` | 28 px / 1.2 (mobile 24 px) | 600 | Screen titles ("Home Electricity") |
| `heading-md` | 20 px / 1.3 | 600 | Card and section titles |
| `stat` | 24 px / 1.3 | 500 | KPI values ("Rs 68,310") |
| `body` | 16 px / 1.5 | 400 | Default text; **inputs are always 16 px** (stops iOS zoom) |
| `body-strong` | 16 px / 1.5 | 500 | List row primary text, amounts in rows |
| `body-sm` | 14 px / 1.5 | 500 | Secondary text, table cells |
| `label` | 14 px / 1 | 600 | Buttons, tabs, chips |
| `caption` | 12 px / 1.5 | 500 | Metadata, timestamps, axis labels |
| `caption-strong` | 12 px / 1.5 | 600 | Badges, section overlines ("ELECTRICITY") |

Rules:
- **Amounts:** use `font-variant-numeric: tabular-nums` in tables and lists so digits line up. Right-align amounts in rows. Hero amounts use proportional figures. Check during M1 that Red Hat Display ships tabular figures; if not, numeric columns use Red Hat Mono.
- **Currency:** `Rs 8,420` with a thin space, formatted per locale setting (NFR-12). Minus signs are real minus signs (−).
- **No automatic text transforms on user content.** Profile names show exactly as typed ("iPhone EMI" must not become "Iphone Emi"). Our own copy uses sentence case. Overlines use `caption-strong` with letter-spacing 0.04 em, typed in capitals, not transformed.
- Letter-spacing is 0 everywhere else.

---

## 4. Shape, spacing & elevation

### Radius
| Token | Value | Use |
|---|---|---|
| `--r-pill` | 9999 px | All buttons, chips, segmented tabs, badges, bottom-nav active pill |
| `--r-card` | 24 px | Cards, featured card, bottom sheets (top corners only) |
| `--r-field` | 14 px | Inputs, selects, search |
| `--r-tile` | 12 px | Profile icon tiles, icon buttons, thumbnails |

The reference file uses square (0 px) inputs next to pill buttons. Here inputs are rounded, so pills and sharp corners never mix inside one form.

### Spacing (4 px base)
`4 · 8 · 12 · 16 · 20 · 24 · 28 · 32 · 36 · 40`

- **Page gutter:** 16 px on mobile, 24 px on tablet, 40 px on desktop. The reference's 0 px mobile gutter is not used, because cards need breathing room from the screen edge.
- **Card padding:** 20 px on mobile, 24 px on tablet, 32 px on desktop.
- **Between cards:** 12 px on mobile, 16–24 px on desktop. **Between sections:** 32–40 px.

### Elevation
- **Flat by default:** `--bg` → `--card` → `--featured` (colour-blocking).
- **One exception, floating layers only** (bottom sheets, menus, the add button, toasts): `--shadow-float: 0 8px 24px rgba(0,0,0,.12)` (dark: none, plus a 1 px `--border` ring). Things that float over content need separating; nothing else gets a shadow.

---

## 5. Components

### Buttons (all pills, minimum 44 px tall)
| Variant | Style | Use |
|---|---|---|
| **Primary** | `--featured` bg, white `label` text, 48 px tall, 24 px side padding | One per screen: "Save bill", "Mark paid" |
| **Secondary** | `--card-muted` bg, `--text` | "Upload", "Cancel" |
| **Outline** | transparent, 1 px `--text` border; hover border `--accent` | Low-emphasis actions next to a primary button |
| **Ghost** | transparent, `--text`; hover `--card-muted` wash | Toolbar and inline actions |
| **Danger** | `#9B1C1C` bg, white text | Destructive confirmation only |
| **On featured** | lime bg + charcoal text (primary), or white outline (secondary) | Buttons inside the dark featured card |
| **Add button (＋)** | 56 px circle, `--accent` bg, `--on-accent` plus icon, `--shadow-float` | Centre of the bottom nav; always one tap away |

States: hover darkens by about 6%; pressed scales to 0.98; disabled has 50% opacity and no pointer; loading shows an inline spinner and keeps the width. **Focus is always visible:** a 2 px `--focus` ring with a 2 px offset. The reference file's `outline: none` is not used, because keyboard users need to see focus.

### Profile tile
```
┌──────────────────────────────────────┐
│ [⚡] Home Electricity       Rs 8,420 │
│      LESCO · ⚠ Due Oct 08            │
└──────────────────────────────────────┘
```
`--card` background, 16 px padding, a 44 px icon tile (`--r-tile`, profile colour at 16%), name in `body-strong`, meta in `body-sm --text-muted`, status badge, and amount right-aligned in tabular figures. The whole row is the tap target.

### Featured bill card
The current or most urgent bill of a profile, or of the home dashboard. `--featured` background, `--r-card`, 24 px padding. It holds: the period (`caption-strong`, white at 70%), a status badge, the amount (`amount-hero`, white), the due date, units and rate (metered only), and the actions (lime primary + white outline). At most **one featured card per screen**.

### Stat (KPI)
Label (`caption`, `--text-muted`) above the value (`stat`). An optional delta line shows the direction **and** the absolute numbers: "+6.2% vs Aug (Rs 7,930 → 8,420)" (FR-ANLT-05). The delta text stays in `--text-body`; a small ▲/▼ icon carries the direction. Up means more spending, so it is not coloured as good.

### Segmented tabs (profile dashboard)
A horizontally scrollable row of pills: Overview · Bills · Payments · Documents · Settings. Active: `--featured` bg + white text. Inactive: transparent + `--text-body`. It sticks under the header while scrolling.

### Chips / filters
Pills with `--card-muted` bg. Selected: `--featured` bg + white text + check icon. Used for type filters on the Profiles screen and for the range selector (Month · Quarter · Year · Custom).

### Inputs
`--card` bg, 1 px `--border`, `--r-field`, 48 px tall, `body` 16 px. Focus: the border becomes `--focus`, 2 px. Error: the border is `#9B1C1C`, with a message below in the same colour and an ⚠ icon.
**Money input:** a "Rs" prefix, right-aligned digits, `inputmode="decimal"`, and live thousands separators.
**Date input:** native date picker, with quick chips (Today · Yesterday).

### Status badge
A pill with 4 px × 10 px padding, `caption-strong`, icon + word, colours from §2.2.

### Records row / table
Mobile: a two-line row (period + meta on the left; amount + status on the right) with 1 px `--border` dividers inside a card. Desktop: a real `<table>` with sticky header, tabular figures, right-aligned amount columns and sortable headers.

### Bottom navigation (mobile)
Five items: Home · Profiles · **＋** · Analytics · More. 64 px tall plus the safe-area inset, on a `--card` background with a top hairline. The active item gets a `--featured` pill behind its icon, with the icon in `--accent`. Labels are always visible (`caption`).

### Sidebar (desktop ≥ 1024 px)
264 px wide on `--bg`. It holds nav items (pill highlight when active), then a **Pinned profiles** section with each profile's icon tile and name, then "＋ New profile".

### Bottom sheet / dialog
Mobile actions and pickers open as bottom sheets (`--r-card` top corners, `--shadow-float`, drag handle). Desktop uses centred dialogs at most 560 px wide. Destructive confirmations are built into the sheet; no browser `confirm()`.

### Toast
Bottom-centre, `--featured` bg, white text, with an optional **Undo** text button in `--accent`. Disappears after 5 s; undo is available for 10 s (see [03](03-user-flows-and-screens.md) §4).

### Empty state
A simple line icon, one sentence and one primary button: "No bills yet in Shop Electricity. [Add the first bill]".

### Skeleton loading
`--card-muted` blocks with the same shape as the content, with a gentle shimmer (off when the user prefers reduced motion). No spinners for page loads.

---

## 6. Charts
Charts follow the data-viz method. In short:
- Bars for spend per period. Lines for consumption trends. Horizontal bars for breakdowns (sub-categories, profile comparison). Use a **stat** instead of a chart when there is a single number.
- Thin marks with 4 px rounded bar ends, 2 px lines, recessive gridlines (`--border`) and `caption` axis labels in `--text-muted`.
- **Never a dual-axis chart.** Spend and units are two separate charts, stacked (as on the profile dashboard).
- Hover/tap tooltips on every chart, and a "Show data" table view (FR-ANLT-07).
- Series colours come from §2.4 (profile colours); status colours never appear as series.
- Charts sit on `--card`, never on the featured card.

---

## 7. Layout & responsiveness

| Breakpoint | Width | Layout |
|---|---|---|
| Mobile | < 768 px (designed at 375) | One column, bottom nav, 16 px gutter |
| Tablet | 768–1023 px | Two-column card grid, bottom nav, 24 px gutter |
| Desktop | 1024–1439 px | Sidebar + content (max 1200 px), 12-column grid, 40 px gutter |
| Wide | ≥ 1440 px | Same, centred; the content never stretches past 1200 px |

**Profile dashboard grid (desktop):** featured bill card (5 columns) + KPI stats (7 columns), then spend and units charts (6 + 6, stacked on tablet), then the records table at full width.

**Home dashboard (desktop):** month summary (4 cols) + due soon / overdue (8 cols), then pinned profile tiles in a 3-column grid.

Touch targets are ≥ 44 × 44 px, with ≥ 8 px between tappable items.

---

## 8. Motion
- 150 ms for hover/press, 200–250 ms ease-out for sheets and page transitions.
- Numbers don't animate counting up; amounts must be readable instantly.
- `prefers-reduced-motion`: transitions become instant fades and shimmer stops.

---

## 9. Accessibility checklist
- Text contrast ≥ 4.5:1, large text and UI parts ≥ 3:1 (all tokens above comply).
- Visible focus on every interactive element. Logical tab order. Skip-to-content link.
- Status is shown with icon + word + colour. Charts have labels and table views.
- All inputs have visible labels (no placeholder-only labels).
- Works at 200% zoom and with large system fonts. Supports `forced-colors`.
- Dark mode is a designed theme (§2), not an automatic inversion.

---

## 10. Implementation (Tailwind + shadcn/ui)

Tokens are CSS variables. Tailwind and shadcn/ui read the variables, so the light/dark switch happens in one place.

```css
:root {
  color-scheme: light;
  --bg: #ECEBE8; --card: #F8F8F8; --card-muted: #E4E4E0;
  --featured: #2D2D2D; --on-featured: #FFFFFF;
  --text: #000000; --text-body: #29303D; --text-muted: #5A5F68;
  --border: #D0CFCC; --accent: #E3EF7A; --on-accent: #2D2D2D; --focus: #2D2D2D;
  --r-pill: 9999px; --r-card: 24px; --r-field: 14px; --r-tile: 12px;
  --shadow-float: 0 8px 24px rgba(0,0,0,.12);
  --font-sans: "Red Hat Display", ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
}
@media (prefers-color-scheme: dark) {
  :root:not([data-theme="light"]) {
    color-scheme: dark;
    --bg: #0F0F10; --card: #1D1D1E; --card-muted: #29292A;
    --featured: #303031; --text: #F7F7F7; --text-body: #E2E2E2; --text-muted: #9F9F9F;
    --border: #353536; --focus: #E3EF7A; --shadow-float: none;
  }
}
:root[data-theme="dark"] {
  color-scheme: dark;
  --bg: #0F0F10; --card: #1D1D1E; --card-muted: #29292A;
  --featured: #303031; --text: #F7F7F7; --text-body: #E2E2E2; --text-muted: #9F9F9F;
  --border: #353536; --focus: #E3EF7A; --shadow-float: none;
}
```

- The theme follows the system setting by default; Settings has Light · Dark · System.
- Icons: **Lucide** (shadcn's default set) at a 1.75 px stroke, 20 px in rows and 24 px in the nav.
- shadcn/ui components are restyled to these tokens: pill buttons, 24 px cards, 14 px fields.

## 11. Differences from the reference token file

| Reference | This system | Why |
|---|---|---|
| Black text on the dark surface (2.1:1) | White text on dark surfaces | Contrast failure |
| `outline: none` on focus | Visible 2 px focus ring | Keyboard accessibility |
| Square inputs beside pill buttons | 14 px rounded inputs | Consistent shapes in forms |
| 0 px mobile gutters | 16 px gutter | Cards need edge spacing on phones |
| Electric blue accent | Not used | Clashes with the warm palette; looks like a default link |
| Caveat script font | Not used | Wrong tone for a finance app |
| No status colours | Full status set (§2.2) | Bills need paid/overdue/due-soon states |
| No shadows at all | Shadow on floating layers only | Overlays need separation from content |
| Automatic capitalisation | No transforms on user content | Keeps profile names exactly as typed |
