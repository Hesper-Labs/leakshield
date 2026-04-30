import { ComingSoonBanner } from "@/components/layout/coming-soon-banner";
import { PageShell } from "@/components/layout/page-shell";
import { CategoriesEditor } from "@/components/policy/categories-editor";

export const metadata = {
  title: "DLP categories",
};

export default function PolicyCategoriesPage() {
  return (
    <PageShell
      title="DLP categories"
      description="Built-in catalog on the left, your tenant's custom categories on the right. Built-ins are read-only; custom categories can use keywords, regex, fingerprints, LLM-only descriptions, or hashed directories."
    >
      <ComingSoonBanner
        phase={3}
        description="Read-only built-ins are listed; the custom-category wizard, CSV directory uploader, and adversarial test gate ship next."
      />
      <CategoriesEditor />
    </PageShell>
  );
}
