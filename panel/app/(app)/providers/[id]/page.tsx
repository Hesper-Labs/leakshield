import { ComingSoonBanner } from "@/components/layout/coming-soon-banner";
import { PageShell } from "@/components/layout/page-shell";

export const metadata = {
  title: "Provider details",
};

export default async function ProviderDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return (
    <PageShell
      title="Provider details"
      description={`Configuration, allowed models, and recent traffic for provider ${id}.`}
    >
      <ComingSoonBanner phase={1} />
    </PageShell>
  );
}
