import { notFound } from "next/navigation";

import { OnboardingStep } from "@/components/onboarding/onboarding-step";
import { Step1CreateAdmin } from "@/components/onboarding/step-1-create-admin";
import { fetchSetupStatus } from "@/lib/setup-status";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";

const VALID_STEPS = ["1", "2", "3", "4", "5"] as const;

type StepValue = (typeof VALID_STEPS)[number];

const STEP_TITLES: Record<StepValue, string> = {
  "1": "Create your admin account",
  "2": "Connect a provider",
  "3": "Create the first user",
  "4": "Choose a DLP strategy",
  "5": "Verify",
};

const STEP_DESCRIPTIONS: Record<StepValue, string> = {
  "1": "First time on this install — we'll create the company, the root admin, and the per-tenant DEK.",
  "2": "Connect OpenAI, Anthropic, Google, or Azure. Test the key, pick allowed models.",
  "3": "Issue the first virtual key. Shown once — copy it now.",
  "4": "Pick the DLP strategy and the model. Optionally seed company-custom categories.",
  "5": "Send a real request through the gateway and watch the audit log light up.",
};

const TOTAL_STEPS = 5;

export default async function OnboardingStepPage({
  params,
}: {
  params: Promise<{ step: string }>;
}) {
  const { step } = await params;
  if (!VALID_STEPS.includes(step as StepValue)) {
    notFound();
  }

  const value = step as StepValue;
  const stepNum = Number(value);
  const status = await fetchSetupStatus();
  const offline = !status.reachable;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <span>
          Step {stepNum} of {TOTAL_STEPS}
        </span>
        <Badge variant="outline">Onboarding</Badge>
      </div>
      <ProgressBar step={stepNum} total={TOTAL_STEPS} />

      {offline ? (
        <Alert>
          <AlertTitle>Gateway is offline — demo mode</AlertTitle>
          <AlertDescription>
            The Go gateway isn't reachable, so the wizard is running against
            stub responses. Submitting will surface the error inline so you
            can still walk through the UI.{" "}
            <code>docker compose up gateway</code> brings the backend up.
          </AlertDescription>
        </Alert>
      ) : null}

      <header className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">{STEP_TITLES[value]}</h1>
        <p className="text-sm text-muted-foreground">{STEP_DESCRIPTIONS[value]}</p>
      </header>

      {stepNum === 1 ? (
        <Step1CreateAdmin />
      ) : (
        <OnboardingStep
          step={stepNum}
          title={STEP_TITLES[value]}
          description={STEP_DESCRIPTIONS[value]}
        />
      )}
    </div>
  );
}

function ProgressBar({ step, total }: { step: number; total: number }) {
  const percent = Math.round(((step - 1) / (total - 1)) * 100);
  return (
    <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
      <div
        className="h-full bg-primary transition-[width]"
        style={{ width: `${Math.max(8, percent)}%` }}
        aria-hidden
      />
    </div>
  );
}
