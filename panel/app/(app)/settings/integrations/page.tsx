import { ComingSoonBanner } from "@/components/layout/coming-soon-banner";
import { PageShell } from "@/components/layout/page-shell";

export const metadata = {
  title: "Integrations",
};

export default function IntegrationsPage() {
  return (
    <PageShell
      title="Integrations"
      description="SIEM forwarding, webhooks for blocked-request alerts, and OTel exporter destinations."
    >
      <ComingSoonBanner phase={5} />
    </PageShell>
  );
}
