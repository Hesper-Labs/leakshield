import { ComingSoonBanner } from "@/components/layout/coming-soon-banner";
import { PageShell } from "@/components/layout/page-shell";

export const metadata = {
  title: "Profile",
};

export default function ProfileSettingsPage() {
  return (
    <PageShell
      title="Profile"
      description="Your name, email, password, locale, and notification preferences."
    >
      <ComingSoonBanner phase={5} />
    </PageShell>
  );
}
