import Link from "next/link";
import { Plus } from "lucide-react";

import { ComingSoonBanner } from "@/components/layout/coming-soon-banner";
import { PageShell } from "@/components/layout/page-shell";
import { Button } from "@/components/ui/button";

export const metadata = {
  title: "Providers",
};

export default function ProvidersPage() {
  return (
    <PageShell
      title="Providers"
      description="Connected upstream LLM providers (OpenAI, Anthropic, Google, Azure) and their configured models."
      actions={
        <Button asChild>
          <Link href="/providers/new">
            <Plus className="h-4 w-4" />
            Connect provider
          </Link>
        </Button>
      }
    >
      <ComingSoonBanner phase={1} />
    </PageShell>
  );
}
