import Link from "next/link";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Step4Categories } from "@/components/onboarding/step-4-categories";

const TOTAL_STEPS = 5;

export function OnboardingStep({
  step,
  title,
  description,
}: {
  step: number;
  title: string;
  description: string;
}) {
  const next = step < TOTAL_STEPS ? `/onboarding/${step + 1}` : "/dashboard";
  const prev = step > 1 ? `/onboarding/${step - 1}` : null;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <span>
          Step {step} of {TOTAL_STEPS}
        </span>
        <Badge variant="outline">Onboarding</Badge>
      </div>
      <ProgressBar step={step} total={TOTAL_STEPS} />
      <Card>
        <CardHeader>
          <CardTitle>{title}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">{description}</p>
          {step === 4 ? <Step4Categories /> : null}
          <Alert>
            <AlertTitle>Wiring in next iteration</AlertTitle>
            <AlertDescription>
              The form and the gateway calls for this step are placeholders —
              the structure is in so the wizard is browsable end-to-end.
            </AlertDescription>
          </Alert>
        </CardContent>
      </Card>
      <div className="flex items-center justify-between">
        {prev ? (
          <Button variant="outline" asChild>
            <Link href={prev}>Back</Link>
          </Button>
        ) : (
          <span />
        )}
        <Button asChild>
          <Link href={next}>{step < TOTAL_STEPS ? "Continue" : "Finish"}</Link>
        </Button>
      </div>
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
