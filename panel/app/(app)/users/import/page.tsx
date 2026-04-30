import { ComingSoonBanner } from "@/components/layout/coming-soon-banner";
import { PageShell } from "@/components/layout/page-shell";

export const metadata = {
  title: "Bulk user import",
};

export default function UserImportPage() {
  return (
    <PageShell
      title="Bulk user import"
      description="Upload a CSV with name, email, department, and optional phone. Each row becomes a user with a virtual key issued at the end of the wizard."
    >
      <ComingSoonBanner
        phase={2}
        description="The wizard maps columns, previews the parsed rows, and shows a one-time keys panel after submission. CSV upload UI lands here in Phase 2."
      />
    </PageShell>
  );
}
