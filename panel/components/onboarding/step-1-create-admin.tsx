"use client";

import { useMemo, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { signIn } from "next-auth/react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface BootstrapResponse {
  ok: boolean;
  error?: string;
  demo?: boolean;
}

/**
 * Step 1 of the onboarding wizard on a fresh install: create the very first
 * admin and the first company in one screen. Posts to the panel's
 * `/api/setup/bootstrap` proxy, which forwards to the gateway. On success we
 * silently sign the new admin in and advance to step 2.
 */
export function Step1CreateAdmin() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [error, setError] = useState<string | null>(null);
  const [demo, setDemo] = useState(false);

  const [companyName, setCompanyName] = useState("");
  const [fullName, setFullName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");

  const passwordOk = password.length >= 8;
  const passwordsMatch = password.length > 0 && password === confirm;

  const slug = useMemo(() => slugify(companyName), [companyName]);

  return (
    <Card>
      <CardHeader>
        <CardTitle>Create your admin account</CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        <p className="text-sm text-muted-foreground">
          This is the first time LeakShield has run here. Create the company
          and the root admin in one step. You can invite teammates afterwards.
        </p>

        {error ? (
          <Alert variant={demo ? "default" : "destructive"}>
            <AlertTitle>{demo ? "Demo mode" : "Couldn't create the account"}</AlertTitle>
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        ) : null}

        <form
          className="space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            setError(null);
            setDemo(false);

            if (!passwordsMatch) {
              setError("Passwords don't match.");
              return;
            }

            startTransition(async () => {
              const res = await fetch("/api/setup/bootstrap", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                  company_name: companyName.trim(),
                  company_slug: slug,
                  full_name: fullName.trim(),
                  email: email.trim().toLowerCase(),
                  password,
                }),
              });
              const data = (await res.json().catch(() => ({}))) as BootstrapResponse;

              if (!res.ok) {
                setDemo(Boolean(data.demo));
                setError(data.error ?? `Server returned ${res.status}.`);
                return;
              }

              // Sign in immediately so steps 2-5 act on an authenticated session.
              const signInResult = await signIn("credentials", {
                email: email.trim().toLowerCase(),
                password,
                redirect: false,
              });

              if (signInResult?.error) {
                // Account was created but auto-sign-in failed; fall back to /sign-in.
                router.replace(`/sign-in?email=${encodeURIComponent(email)}`);
                return;
              }

              router.replace("/onboarding/2");
              router.refresh();
            });
          }}
        >
          <div className="grid gap-4 sm:grid-cols-2">
            <Field
              id="company_name"
              label="Company name"
              autoComplete="organization"
              value={companyName}
              onChange={setCompanyName}
              hint={slug ? `Slug: ${slug}` : "Used in URLs and audit log labels"}
              required
            />
            <Field
              id="full_name"
              label="Your name"
              autoComplete="name"
              value={fullName}
              onChange={setFullName}
              hint="Shown to teammates and in the audit log"
              required
            />
          </div>
          <Field
            id="email"
            label="Email"
            type="email"
            autoComplete="email"
            value={email}
            onChange={setEmail}
            required
          />
          <div className="grid gap-4 sm:grid-cols-2">
            <Field
              id="password"
              label="Password"
              type="password"
              autoComplete="new-password"
              value={password}
              onChange={setPassword}
              hint={password.length === 0 ? "At least 8 characters" : passwordOk ? "Looks good" : "Too short"}
              required
            />
            <Field
              id="confirm"
              label="Confirm password"
              type="password"
              autoComplete="new-password"
              value={confirm}
              onChange={setConfirm}
              hint={
                confirm.length === 0
                  ? " "
                  : passwordsMatch
                    ? "Match"
                    : "Doesn't match the password"
              }
              required
            />
          </div>

          <Button
            type="submit"
            className="w-full"
            disabled={pending || !passwordOk || !passwordsMatch}
          >
            {pending ? "Creating your account…" : "Create account & continue"}
          </Button>

          <p className="text-center text-xs text-muted-foreground">
            By continuing you agree this LeakShield install will store a
            per-tenant Data Encryption Key wrapped by the configured KEK.
          </p>
        </form>
      </CardContent>
    </Card>
  );
}

function Field({
  id,
  label,
  hint,
  type = "text",
  value,
  onChange,
  required,
  autoComplete,
}: {
  id: string;
  label: string;
  hint?: string;
  type?: string;
  value: string;
  onChange: (next: string) => void;
  required?: boolean;
  autoComplete?: string;
}) {
  return (
    <div className="space-y-2">
      <Label htmlFor={id}>{label}</Label>
      <Input
        id={id}
        name={id}
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        autoComplete={autoComplete}
        required={required}
      />
      {hint ? <p className="text-xs text-muted-foreground">{hint}</p> : null}
    </div>
  );
}

function slugify(input: string): string {
  return input
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "")
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 48);
}
