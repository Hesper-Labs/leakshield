# LeakShield Admin Panel

The admin web app for LeakShield. Next.js 15 with the App Router and Server Components,
TypeScript strict mode, Tailwind CSS v4, shadcn/ui primitives owned in
[`components/ui/`](components/ui), TanStack Query for server state, Zustand for ephemeral
client state, Recharts for analytics, and Monaco for the DLP policy editor (lazy-loaded
on the policy page only). Auth.js v5 handles session management against the Go gateway's
auth endpoint — no DB adapter, the gateway owns the user table. i18n via next-intl with
English default and Turkish second locale. Typed API client generated from the gateway's
OpenAPI spec via openapi-typescript + openapi-fetch.

## Getting started

```bash
cd panel
npm install
cp .env.example .env.local         # then edit AUTH_SECRET
npm run dev                        # http://localhost:3000
```

The first request to `/` redirects to `/sign-in`. After a fresh install (no admin in the
gateway) the gateway routes you to `/onboarding/1` instead — see
[`docs/setup-wizard.md`](../docs/setup-wizard.md) for the full flow.

## Scripts

| Script | What it does |
|---|---|
| `npm run dev` | Local dev server. |
| `npm run build` | Production build (Next standalone output, used by the Dockerfile). |
| `npm run start` | Run the production build. |
| `npm run lint` | ESLint with the Next.js recommended config. |
| `npm run typecheck` | `tsc --noEmit` against the strict tsconfig. |
| `npm run generate:api` | Run `openapi-typescript` against the gateway's OpenAPI spec → `lib/api/types.ts`. |

`lib/api/types.ts` is gitignored; the generator script writes it locally each time you
clone. `lib/api/client.ts` falls back to a loose `paths` interface so the panel still
typechecks before the gateway publishes its first spec.

## Layout

```
app/
  (auth)/                         # focused auth flow, no sidebar
    sign-in/, sign-up/
  (onboarding)/onboarding/[step]/ # 5-step setup wizard, full-bleed layout
  (app)/                          # authenticated app shell (sidebar + topbar)
    dashboard/, providers/, users/, keys/,
    policy/ (+ categories, versions),
    analytics/, logs/, settings/...
  super-admin/                    # role-gated operator console
  api/
    auth/[...nextauth]/route.ts   # Auth.js v5 catch-all
    stream/logs/route.ts          # SSE proxy to /admin/v1/stream/logs

components/
  ui/                             # shadcn primitives (New York, slate base, CSS vars)
  layout/                         # AppShell, Sidebar, Topbar, TenantSwitcher,
                                  # ConnectionIndicator, Logo, PageShell, ComingSoonBanner
  policy/                         # PolicyEditor (Monaco), PolicyEditorWorkspace,
                                  # CategoriesEditor (built-in vs custom)
  onboarding/                     # OnboardingStep + Step4Categories templates
  analytics/                      # AnalyticsPreview (Recharts)
  logs/                           # LiveLogTable (auto-pause on scroll)
  providers/                      # AppProviders, QueryProvider

hooks/
  use-log-stream.ts               # SSE hook backing the live audit log

lib/
  api/                            # openapi-fetch client + generated types
  stores/                         # Zustand stores (sidebar, modal stack)
  env.ts, utils.ts

i18n/request.ts                   # next-intl request config
messages/{en,tr}.json             # locale dictionaries
auth.ts                           # Auth.js v5 (Credentials → gateway)
```

## Visual identity

Light theme is the unmodified default — white backgrounds, navy `#0F172A` text, blue
`#3B82F6` accents, green/red for allow/deny verdicts. Tokens live in `app/globals.css`
under `:root` plus a `.dark` block; dark mode is opt-in via the `class` attribute and
shows the same primitives with cool-slate surfaces.

The brand mark renders from `/logo.png` (drop in via `panel/public/logo.png` or symlink
to `assets/logo.png`). When the file is missing the `Logo` component paints a small
inline-SVG fallback so the chrome still looks intentional.

## Status by route

| Route | Phase | Notes |
|---|---|---|
| `/sign-in` | 1 | Form is wired against the Auth.js Credentials provider. |
| `/sign-up` | 1 | Stubbed; bootstraps via the gateway's onboarding endpoint. |
| `/onboarding/[step]` | 1 | 5-step wizard skeleton, with quick-start template picker on Step 4. |
| `/dashboard` | 2 | KPI tile layout; data wiring next. |
| `/providers`, `/providers/[id]`, `/providers/new` | 1 | Layout only. |
| `/users`, `/users/[id]`, `/users/import` | 2 | Layout + bulk-import shell. |
| `/keys` | 2 | Layout only. |
| `/policy` | 3 | Monaco editor + 60/40 test-harness split + Variables/Templates rail + Versions sheet. |
| `/policy/categories` | 3 | Built-in catalog (read-only) on the left, custom categories on the right. |
| `/policy/versions` | 3 | Layout only. |
| `/analytics` | 4 | Recharts area chart with mock data. |
| `/logs`, `/logs/[requestId]` | 2 | Live SSE table with auto-pause-on-scroll. |
| `/settings/*` | 5 | Layout only. |
| `/super-admin` | 5 | Role-gated layout. |

## What's deferred

- Real form wiring against the Go gateway (waiting on the gateway to publish
  `/admin/v1/openapi.json`).
- True virtualization on the live audit log — current sketch renders a sliding window of
  the most recent 200 entries; we'll swap to `@tanstack/react-virtual` once the table
  shape settles.
- "First request lit up" confetti on the verify step.
- CSV column-mapping UI for `/users/import`.
- Locale switching surface (next-intl reads `Accept-Language` and the cookie set by a
  future settings toggle).
