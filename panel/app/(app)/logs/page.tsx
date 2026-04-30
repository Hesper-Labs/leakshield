import { ComingSoonBanner } from "@/components/layout/coming-soon-banner";
import { PageShell } from "@/components/layout/page-shell";
import { LiveLogTable } from "@/components/logs/live-log-table";

export const metadata = {
  title: "Audit log",
};

export default function LogsPage() {
  return (
    <PageShell
      title="Audit log"
      description="Every gateway request — who made it, which provider it routed to, and the DLP verdict. Live updates flow over SSE; scroll down to pause."
    >
      <ComingSoonBanner
        phase={2}
        description="The SSE plumbing is in. Filtering, request-detail drill-down, and proper virtualization land in the next iteration."
      />
      <LiveLogTable />
    </PageShell>
  );
}
