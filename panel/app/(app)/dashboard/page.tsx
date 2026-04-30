import { ComingSoonBanner } from "@/components/layout/coming-soon-banner";
import { PageShell } from "@/components/layout/page-shell";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export const metadata = {
  title: "Dashboard",
};

export default function DashboardPage() {
  return (
    <PageShell
      title="Dashboard"
      description="At-a-glance health, traffic, and DLP outcomes for your tenant."
    >
      <ComingSoonBanner
        phase={2}
        description="KPI tiles, request throughput, blocked-vs-allowed breakdown, and top users will live here."
      />
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4">
        {[
          { label: "Requests / 24h", value: "—" },
          { label: "Blocked", value: "—" },
          { label: "Active virtual keys", value: "—" },
          { label: "p95 latency", value: "—" },
        ].map((kpi) => (
          <Card key={kpi.label}>
            <CardHeader className="pb-2">
              <CardDescription>{kpi.label}</CardDescription>
              <CardTitle className="text-3xl">{kpi.value}</CardTitle>
            </CardHeader>
            <CardContent className="text-xs text-muted-foreground">
              Live data once analytics ships.
            </CardContent>
          </Card>
        ))}
      </div>
    </PageShell>
  );
}
