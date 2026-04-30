import { ComingSoonBanner } from "@/components/layout/coming-soon-banner";
import { PageShell } from "@/components/layout/page-shell";

export const metadata = {
  title: "Team settings",
};

export default function TeamSettingsPage() {
  return (
    <PageShell
      title="Team"
      description="Invite teammates, assign roles, and manage SSO bindings."
    >
      <ComingSoonBanner phase={5} />
    </PageShell>
  );
}
