import { ComingSoonBanner } from "@/components/layout/coming-soon-banner";
import { PageShell } from "@/components/layout/page-shell";

export const metadata = {
  title: "Policy versions",
};

export default function PolicyVersionsPage() {
  return (
    <PageShell
      title="Policy versions"
      description="Every published policy is hash-chained and gated by an adversarial test suite. Roll back, diff, or re-run the gate for any version here."
    >
      <ComingSoonBanner phase={3} />
    </PageShell>
  );
}
