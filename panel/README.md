# LeakShield Admin Panel

The admin web app for LeakShield. Built with Next.js 15 (App Router, Server Components),
TypeScript, Tailwind CSS v4, shadcn/ui, TanStack Query, Recharts, and Monaco for the DLP policy
editor. Real-time updates flow over SSE.

## Status

Not yet scaffolded — this directory is reserved. The next iteration runs `create-next-app` and
adds the routes laid out in the architecture plan:

```
/auth/sign-in, /auth/sign-up
/onboarding/[step]            # 5-step setup wizard
/app/dashboard
/app/providers, /app/users, /app/keys
/app/policy                   # Monaco editor + test harness
/app/analytics
/app/logs                     # live audit log via SSE
/app/settings/...
/super-admin
```

## Visual direction

- **Light theme by default**, with optional dark mode.
- Clean white backgrounds, professional but minimal.
- Navy headings, blue accents, status colors for allow/deny.
- Logo sourced from `../assets/logo.png` (or `.svg`).

## Setup wizard

The setup wizard (`/onboarding/[step]`) walks an admin from a fresh install to a working proxied
LLM call in under five minutes:

1. Create the root admin and tenant.
2. Connect a provider (OpenAI / Anthropic / Google / Azure) — paste key, click "Test connection",
   pick which models to allow.
3. Create the first user and issue a virtual key (one-time-show pattern).
4. Choose a DLP strategy:
   - Off (no inspection)
   - Mock (always allow — current default in dev)
   - Hybrid (Presidio + LLM)
   - Specialized DLP classifier
   - LLM Judge with custom prompt
   The wizard offers recommended models, but the admin can pick **any** model their backend
   supports. LeakShield does not download models; if Ollama is selected, the wizard verifies the
   model is already pulled and surfaces the `ollama pull <model>` command otherwise.
5. Verify: a curl snippet, an SSE-driven "first request lit up" detector, and a confetti
   transition to the dashboard once the first proxied request flows through.
