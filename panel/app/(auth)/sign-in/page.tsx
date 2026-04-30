import Link from "next/link";
import { Suspense } from "react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { SignInForm } from "@/app/(auth)/sign-in/sign-in-form";
import { fetchSetupStatus } from "@/lib/setup-status";

export const metadata = {
  title: "Sign in",
};

export default async function SignInPage() {
  const status = await fetchSetupStatus();
  const firstTime = !status.reachable || status.hasAdmin === false;

  if (firstTime) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Welcome to LeakShield</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <Alert>
            <AlertTitle>Looks like a fresh install</AlertTitle>
            <AlertDescription>
              {status.reachable
                ? "No admin account exists yet. Create one to start using the gateway."
                : "The gateway isn't reachable, so we'll set things up in demo mode for now."}
            </AlertDescription>
          </Alert>
          <Button asChild className="w-full">
            <Link href="/onboarding/1">Create your admin account →</Link>
          </Button>
          <p className="text-center text-xs text-muted-foreground">
            Already have an account?{" "}
            <Link
              className="text-primary underline-offset-4 hover:underline"
              href="/sign-in?continue=1"
            >
              Use the sign-in form anyway
            </Link>
            .
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Sign in to LeakShield</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <Suspense fallback={<Skeleton className="h-44 w-full" />}>
          <SignInForm />
        </Suspense>
        <p className="text-center text-sm text-muted-foreground">
          Forgot your password?{" "}
          <Link
            className="text-primary underline-offset-4 hover:underline"
            href="/sign-in/reset"
          >
            Reset it
          </Link>
          .
        </p>
      </CardContent>
    </Card>
  );
}
