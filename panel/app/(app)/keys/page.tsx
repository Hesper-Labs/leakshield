import { Plus } from "lucide-react";

import { ComingSoonBanner } from "@/components/layout/coming-soon-banner";
import { PageShell } from "@/components/layout/page-shell";
import { Button } from "@/components/ui/button";

export const metadata = {
  title: "Virtual keys",
};

export default function KeysPage() {
  return (
    <PageShell
      title="Virtual keys"
      description="Per-user gateway tokens with rate limits and budget caps. Master provider keys never leave envelope encryption."
      actions={
        <Button>
          <Plus className="h-4 w-4" />
          Issue key
        </Button>
      }
    >
      <ComingSoonBanner phase={2} />
    </PageShell>
  );
}
