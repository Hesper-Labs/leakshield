import Link from "next/link";
import { Suspense } from "react";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { SignInForm } from "@/app/(auth)/sign-in/sign-in-form";

export const metadata = {
  title: "Sign in",
};

export default function SignInPage() {
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
          New install?{" "}
          <Link className="text-primary underline-offset-4 hover:underline" href="/sign-up">
            Bootstrap the root admin
          </Link>
          .
        </p>
      </CardContent>
    </Card>
  );
}
