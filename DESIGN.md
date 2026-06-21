# Sub2API Design System

## 1. Atmosphere & Identity

A dense dark operations console for administrators watching account health, quotas, cooldowns, and dispatch readiness. The signature is compact table-first monitoring: subtle graphite surfaces, restrained blue focus, and status colors used only for operational meaning.

## 2. Color

| Role | Token | Dark | Usage |
|------|-------|------|-------|
| Surface/canvas | `--ops-canvas` | `rgb(8 17 25)` | Page background |
| Surface/panel | `--ops-panel` | `rgb(18 35 47 / 0.94)` | Cards, rails, tables |
| Surface/subtle | `--ops-subtle` | `rgb(12 27 38 / 0.82)` | Table rows, inputs |
| Border/default | `--ops-border` | `rgb(44 62 79)` | Panel and row borders |
| Text/primary | `--ops-text` | `rgb(235 242 248)` | Main copy |
| Text/muted | `--ops-muted` | `rgb(155 168 181)` | Metadata |
| Accent/focus | `--ops-focus` | `rgb(68 145 220)` | Selected rows, links, focus |
| Status/success | `--ops-success` | `rgb(52 211 153)` | Healthy |
| Status/warning | `--ops-warning` | `rgb(250 204 21)` | Cooldown/quota caution |
| Status/error | `--ops-error` | `rgb(248 113 113)` | Error |

Rules: keep the page dark-only; do not introduce decorative gradients or extra accent hues. Raw colors in the SFC should map to these roles.

## 3. Typography

Primary font is the app system sans stack. Use tabular numbers for metrics. Scale: page title `20px`, panel title `14px`, table body `12px`, metadata `11px`. Letter spacing stays `0`.

## 4. Spacing & Layout

Base unit is `4px`. Desktop uses a three-column console: left rail `270-310px`, center flexible, right rail `300-340px`. Panels use `12px` padding, rows `8-10px`, and `4-6px` gaps.

## 5. Components

### Ops Panel
- Structure: bordered dark surface with compact header and dense body.
- States: selected rows use blue border and subtle blue fill; hover uses tonal lift only.

### Ops Table
- Structure: sticky-like header row, compact body rows, tabular numeric cells, horizontal scroll on narrow widths.
- States: status chips and quota bars reflect live data; empty states are inline rows.

## 6. Motion & Interaction

Use `120-160ms ease` for hover, focus, and toggle feedback. Animate only color, border, opacity, or transform. Every button/input needs visible focus.

## 7. Depth & Surface

Depth strategy is borders plus tonal shifts. Shadows are limited to subtle inset highlights on panels; avoid floating card shadows.
