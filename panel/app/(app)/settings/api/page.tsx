import { ComingSoonBanner } from "@/components/layout/coming-soon-banner";
import { PageShell } from "@/components/layout/page-shell";

export const metadata = {
  title: "API settings",
};

export default function ApiSettingsPage() {
  return (
    <PageShell
      title="API"
      description="Gateway base URL, rate limit defaults, and CORS configuration for client SDKs."
    >
      <ComingSoonBanner phase={5} />
    </PageShell>
  );
}
