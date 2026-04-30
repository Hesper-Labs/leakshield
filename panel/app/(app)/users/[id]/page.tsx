import { ComingSoonBanner } from "@/components/layout/coming-soon-banner";
import { PageShell } from "@/components/layout/page-shell";

export const metadata = {
  title: "User details",
};

export default async function UserDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return (
    <PageShell
      title="User details"
      description={`Profile, virtual keys, and recent activity for user ${id}.`}
    >
      <ComingSoonBanner phase={2} />
    </PageShell>
  );
}
