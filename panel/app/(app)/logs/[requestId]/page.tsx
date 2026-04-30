import { ComingSoonBanner } from "@/components/layout/coming-soon-banner";
import { PageShell } from "@/components/layout/page-shell";

export const metadata = {
  title: "Request details",
};

export default async function RequestDetailPage({
  params,
}: {
  params: Promise<{ requestId: string }>;
}) {
  const { requestId } = await params;
  return (
    <PageShell
      title={`Request ${requestId}`}
      description="Full audit row: messages, verdict reasoning, masked spans, upstream timing, and hash-chain attestation."
    >
      <ComingSoonBanner phase={2} />
    </PageShell>
  );
}
