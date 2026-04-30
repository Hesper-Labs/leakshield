import { OnboardingStep } from "@/components/onboarding/onboarding-step";
import { notFound } from "next/navigation";

const VALID_STEPS = ["1", "2", "3", "4", "5"] as const;

type StepValue = (typeof VALID_STEPS)[number];

const STEP_TITLES: Record<StepValue, string> = {
  "1": "Bootstrap the root admin",
  "2": "Connect a provider",
  "3": "Create the first user",
  "4": "Choose a DLP strategy",
  "5": "Verify",
};

const STEP_DESCRIPTIONS: Record<StepValue, string> = {
  "1": "Create the company, the super admin, and the per-tenant DEK.",
  "2": "Connect OpenAI, Anthropic, Google, or Azure. Test the key, pick allowed models.",
  "3": "Issue the first virtual key. Shown once — copy it now.",
  "4": "Pick the DLP strategy and the model. Optionally seed company-custom categories.",
  "5": "Send a real request through the gateway and watch the audit log light up.",
};

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
  return (
    <OnboardingStep
      step={Number(value)}
      title={STEP_TITLES[value]}
      description={STEP_DESCRIPTIONS[value]}
    />
  );
}
