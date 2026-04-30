import { Construction } from "lucide-react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

/**
 * Reusable "coming soon" banner. Each placeholder page passes the phase
 * number so reviewers can see at a glance what's still under construction.
 */
export function ComingSoonBanner({
  phase,
  description,
}: {
  phase: number | string;
  description?: string;
}) {
  return (
    <Alert variant="warning">
      <Construction />
      <AlertTitle>Coming soon — Phase {phase}</AlertTitle>
      <AlertDescription>
        {description ??
          "This screen is scaffolded for navigation. Functionality lands in a later milestone."}
      </AlertDescription>
    </Alert>
  );
}
