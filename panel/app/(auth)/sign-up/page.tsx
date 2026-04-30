import { redirect } from "next/navigation";

/**
 * /sign-up doesn't exist as its own form anymore. On a fresh install the
 * onboarding wizard handles bootstrap end-to-end; on an already-installed
 * gateway sign-up is invite-only via the admin panel. Either way, send the
 * visitor to /onboarding/1 and let the wizard's setup-status check decide
 * what to render.
 */
export default function SignUpRedirect() {
  redirect("/onboarding/1");
}
