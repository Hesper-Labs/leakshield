import { ComingSoonBanner } from "@/components/layout/coming-soon-banner";
import { PageShell } from "@/components/layout/page-shell";
import { PolicyEditorWorkspace } from "@/components/policy/policy-editor-workspace";

export const metadata = {
  title: "DLP policy",
};

export default function PolicyPage() {
  return (
    <PageShell
      title="DLP policy"
      description="Edit the judge prompt and the YAML category list. The editor on the left, a test harness on the right, with versions and variables one click away."
    >
      <ComingSoonBanner
        phase={3}
        description="Editor renders, but save / publish / adversarial test gating ship in Phase 3 alongside the gateway endpoints."
      />
      <PolicyEditorWorkspace />
    </PageShell>
  );
}
