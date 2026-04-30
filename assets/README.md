# Brand Assets

This directory holds the project's logo, banner, and other marketing-grade artwork referenced
from the README and the admin panel.

The repo ships with vector fallbacks (`logo.svg`, `banner.svg`) so GitHub renders something
useful out of the box. If `logo.png` and `banner.png` are present they take precedence on
viewers that need raster art (older email clients, some social previews) — the README uses
`<picture>` tags so PNGs override the SVGs automatically.

## Expected files

| File | Purpose | Notes |
|---|---|---|
| `logo.svg` | Vector logo, ships in the repo | 256×256 viewBox, navy + blue palette |
| `banner.svg` | Vector hero banner, ships in the repo | 1600×600 viewBox |
| `logo.png` | Optional raster logo (used in README header, panel sidebar, favicon source) | ~1200 × 1200 ideal; takes precedence over `logo.svg` |
| `banner.png` | Optional raster banner | ~1800 × 800; takes precedence over `banner.svg` |
| `favicon.ico` | 32×32 favicon | generated from `logo.png` or `logo.svg` |

## Visual identity

- Primary palette: white background, navy `#0F172A` for text, blue `#3B82F6` for highlights, plus
  green/red accents for the allow/deny indicators on the banner.
- Tone: clean, minimal, professional. Avoid heavy gradients or 3D effects.
- The logo is a stylized shield with a keyhole and three flowing lines on the left, evoking a
  filtered traffic flow through a secure barrier.

## Adding higher-fidelity raster assets

The vector fallbacks are deliberately simple. To install higher-fidelity art:

1. Drop `logo.png` and / or `banner.png` into this directory.
2. Optionally generate `favicon.ico` from the logo.
3. Verify the README header renders correctly: `open ../README.md` in a Markdown previewer.

The root README uses `<picture>` elements with the SVG as the fallback, so simply adding
`logo.png` / `banner.png` next to the existing SVGs is enough — no other change required.
