import { redirect } from "next/navigation";

import { auth } from "@/auth";
import { fetchSetupStatus } from "@/lib/setup-status";

/**
 * Smart root: route the visitor to the most useful page given the current
 * install state.
 *
 *   fresh install (no admin yet, OR gateway offline) → /onboarding/1
 *   already signed in                                → /dashboard
 *   admin exists, signed out                         → /sign-in
 */
export default async function RootIndex() {
  const session = await auth();
  if (session?.user) {
    redirect("/dashboard");
  }

  const status = await fetchSetupStatus();
  if (!status.reachable || status.hasAdmin === false) {
    redirect("/onboarding/1");
  }
  redirect("/sign-in");
}
