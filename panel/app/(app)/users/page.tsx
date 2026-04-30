import Link from "next/link";
import { Upload, UserPlus } from "lucide-react";

import { ComingSoonBanner } from "@/components/layout/coming-soon-banner";
import { PageShell } from "@/components/layout/page-shell";
import { Button } from "@/components/ui/button";

export const metadata = {
  title: "Users",
};

export default function UsersPage() {
  return (
    <PageShell
      title="Users"
      description="People in this tenant — name, email, department, and the virtual keys they hold."
      actions={
        <div className="flex items-center gap-2">
          <Button variant="outline" asChild>
            <Link href="/users/import">
              <Upload className="h-4 w-4" />
              Bulk import
            </Link>
          </Button>
          <Button>
            <UserPlus className="h-4 w-4" />
            New user
          </Button>
        </div>
      }
    >
      <ComingSoonBanner phase={2} />
    </PageShell>
  );
}
