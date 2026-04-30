import Link from "next/link";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export const metadata = {
  title: "Sign up",
};

export default function SignUpPage() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Create your LeakShield workspace</CardTitle>
        <CardDescription>
          A fresh install routes you to the onboarding wizard once the gateway is
          reachable. Use this page to bootstrap the very first super admin.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <Alert>
          <AlertTitle>Hooked up in the next milestone</AlertTitle>
          <AlertDescription>
            The gateway exposes <code>POST /admin/v1/auth/bootstrap</code>; the form
            wiring lives next to the onboarding wizard so the two flows share a
            single submission path. For now you can start at{" "}
            <Link className="text-primary underline-offset-4 hover:underline" href="/onboarding/1">
              /onboarding/1
            </Link>
            .
          </AlertDescription>
        </Alert>
        <p className="text-center text-sm text-muted-foreground">
          Already have an account?{" "}
          <Link className="text-primary underline-offset-4 hover:underline" href="/sign-in">
            Sign in
          </Link>
          .
        </p>
      </CardContent>
    </Card>
  );
}
