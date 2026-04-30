import { redirect } from "next/navigation";

import { auth } from "@/auth";
import { ComingSoonBanner } from "@/components/layout/coming-soon-banner";
import { PageShell } from "@/components/layout/page-shell";

export const metadata = {
  title: "Super admin",
};

export default async function SuperAdminPage() {
  const session = await auth();
  if (!session?.user) {
    redirect("/sign-in");
  }
  if (session.user.role !== "super_admin") {
    redirect("/dashboard");
  }
  return (
    <PageShell
      title="Super admin"
      description="Cross-tenant operator console: tenant directory, KEK rotation, system health, and emergency overrides."
    >
      <ComingSoonBanner
        phase={5}
        description="Routed only when the session role is super_admin. Operator features land later in the rollout."
      />
    </PageShell>
  );
}
