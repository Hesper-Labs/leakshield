import { ComingSoonBanner } from "@/components/layout/coming-soon-banner";
import { PageShell } from "@/components/layout/page-shell";
import { AnalyticsPreview } from "@/components/analytics/analytics-preview";

export const metadata = {
  title: "Analytics",
};

export default function AnalyticsPage() {
  return (
    <PageShell
      title="Analytics"
      description="Per-user requests, tokens, cost, blocked categories, and latency percentiles."
    >
      <ComingSoonBanner phase={4} />
      <AnalyticsPreview />
    </PageShell>
  );
}
