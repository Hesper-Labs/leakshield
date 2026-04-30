import { ComingSoonBanner } from "@/components/layout/coming-soon-banner";
import { PageShell } from "@/components/layout/page-shell";

export const metadata = {
  title: "Connect provider",
};

export default function NewProviderPage() {
  return (
    <PageShell
      title="Connect a provider"
      description="Pick OpenAI, Anthropic, Google, or Azure. Paste your master key, test the connection, and choose which models to allow."
    >
      <ComingSoonBanner phase={1} />
    </PageShell>
  );
}
