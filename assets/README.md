# Brand Assets

This directory holds the project's logo, banner, and other marketing-grade artwork referenced
from the README and the admin panel.

## Expected files

| File | Purpose | Notes |
|---|---|---|
| `logo.png` | Square logo (used in README header, panel sidebar, favicon source) | ~1200 × 1200 ideal |
| `logo.svg` | Vector logo for crisp rendering at any size | preferred when available |
| `banner.png` | Wide hero/banner used in README | ~1800 × 800 |
| `banner.svg` | Vector banner | optional |
| `favicon.ico` | 32×32 favicon | generated from `logo.png` |

## Visual identity

- Primary palette: white background, navy `#0F172A` for text, blue `#3B82F6` for highlights, plus
  green/red accents for the allow/deny indicators on the banner.
- Tone: clean, minimal, professional. Avoid heavy gradients or 3D effects.
- The logo is a stylized shield with a keyhole and three flowing lines on the left, evoking a
  filtered traffic flow through a secure barrier.

## Adding the official assets

The current copy of the official logo and banner lives outside the repo (sent through the
project's design channel). To install:

1. Drop `logo.png` (or `.svg`) and `banner.png` (or `.svg`) into this directory.
2. Optionally generate `favicon.ico` from the logo.
3. Verify the README header renders correctly: `open ../README.md` in a Markdown previewer.

The README references these files via relative paths (`assets/logo.png`, `assets/banner.png`),
so no other change is needed once the files are in place.
