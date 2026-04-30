"use client";

import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { Copy, KeyRound, ShieldAlert } from "lucide-react";
import { toast } from "sonner";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

export interface MeData {
  user: {
    id: string;
    email: string;
    name: string;
    role?: "admin" | "member" | "super_admin";
  };
  tenant: { id: string; name: string; slug?: string } | null;
}

interface IssuedKey {
  id: string;
  prefix: string;
  plaintext: string;
  allowed_providers?: string[];
  allowed_models?: string[];
}

interface UserListItem {
  id: string;
  name: string;
  surname?: string;
  email: string;
  department?: string;
  role?: string;
}

interface UsersListResponse {
  items?: UserListItem[];
  ok?: boolean;
  error?: string;
  demo?: boolean;
}

/**
 * Client-side body of onboarding step 3.
 *
 * Two tabs:
 *   - "Use my account"  — single button that issues a virtual key for the
 *                         signed-in admin, no extra form.
 *   - "Add a teammate"  — name/email/optional fields, creates the user and
 *                         then immediately issues a key.
 *
 * Once a key is issued the screen swaps to a one-time reveal pane: the
 * plaintext is shown, the operator must click Copy at least once, and only
 * then can they advance to step 4. The plaintext is never re-fetchable.
 */
export function Step3UserClient({ me }: { me: MeData | null }) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [tab, setTab] = useState<"me" | "teammate">("me");
  const [issuedKey, setIssuedKey] = useState<IssuedKey | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [demo, setDemo] = useState(false);

  // Teammate form state
  const [name, setName] = useState("");
  const [surname, setSurname] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [department, setDepartment] = useState("");

  const myAccountReady = Boolean(me?.user.id);
  const teammateValid =
    name.trim().length >= 2 &&
    /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(email.trim());

  function handleGenerateMine() {
    if (!me?.user.id) {
      setError("Your user id wasn't returned by the gateway. Add a teammate instead, or wait for the gateway to come up.");
      setDemo(true);
      return;
    }
    issueKeyFor(me.user.id, "Personal admin key");
  }

  function handleAddTeammate() {
    if (!teammateValid) return;
    setError(null);
    setDemo(false);

    startTransition(async () => {
      const createRes = await fetch("/api/admin/users", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: name.trim(),
          surname: surname.trim() || undefined,
          email: email.trim().toLowerCase(),
          phone: phone.trim() || undefined,
          department: department.trim() || undefined,
        }),
      });
      const createData = (await createRes.json().catch(() => ({}))) as {
        id?: string;
        ok?: boolean;
        error?: string;
        demo?: boolean;
      };

      if (!createRes.ok || !createData.id) {
        setDemo(Boolean(createData.demo));
        setError(createData.error ?? `Couldn't create the user (HTTP ${createRes.status}).`);
        return;
      }

      // Chain straight into key issuance using the new user's id.
      issueKeyFor(createData.id, `${name.trim()}'s key`);
    });
  }

  function issueKeyFor(userId: string, keyName: string) {
    setError(null);
    setDemo(false);

    startTransition(async () => {
      const res = await fetch(
        `/api/admin/users/${encodeURIComponent(userId)}/keys`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ name: keyName }),
        },
      );
      const data = (await res.json().catch(() => ({}))) as Partial<IssuedKey> & {
        ok?: boolean;
        error?: string;
        demo?: boolean;
      };

      if (!res.ok || !data.plaintext) {
        setDemo(Boolean(data.demo));
        setError(data.error ?? `Couldn't issue the key (HTTP ${res.status}).`);
        return;
      }

      setIssuedKey({
        id: String(data.id ?? ""),
        prefix: String(data.prefix ?? ""),
        plaintext: data.plaintext,
        allowed_providers: data.allowed_providers,
        allowed_models: data.allowed_models,
      });
      toast.success("Virtual key issued");
    });
  }

  if (issuedKey) {
    return (
      <KeyRevealPane
        keyData={issuedKey}
        onContinue={() => {
          router.replace("/onboarding/4");
          router.refresh();
        }}
      />
    );
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Create the first user</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">
            Issue a virtual key — the only thing teammates ever paste into
            their tooling. Upstream provider keys never leave the gateway.
          </p>

          {error ? (
            <Alert variant={demo ? "default" : "destructive"}>
              <AlertTitle>{demo ? "Demo mode" : "Couldn't issue the key"}</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}

          <Tabs
            value={tab}
            onValueChange={(v) => {
              setTab(v as "me" | "teammate");
              setError(null);
              setDemo(false);
            }}
          >
            <TabsList>
              <TabsTrigger value="me">Use my account</TabsTrigger>
              <TabsTrigger value="teammate">Add a teammate</TabsTrigger>
            </TabsList>

            <TabsContent value="me" className="space-y-4 pt-4">
              {me ? (
                <div className="rounded-lg border p-4">
                  <dl className="grid gap-2 text-sm sm:grid-cols-3">
                    <div>
                      <dt className="text-xs uppercase tracking-wide text-muted-foreground">Name</dt>
                      <dd className="font-medium">{me.user.name || "—"}</dd>
                    </div>
                    <div>
                      <dt className="text-xs uppercase tracking-wide text-muted-foreground">Email</dt>
                      <dd className="font-mono text-xs">{me.user.email}</dd>
                    </div>
                    <div>
                      <dt className="text-xs uppercase tracking-wide text-muted-foreground">Role</dt>
                      <dd>
                        <Badge variant="outline" className="capitalize">
                          {me.user.role ?? "admin"}
                        </Badge>
                      </dd>
                    </div>
                  </dl>
                  {!myAccountReady ? (
                    <p className="mt-3 text-xs text-muted-foreground">
                      Gateway didn&apos;t return your user id — issuing a key
                      on this tab will fail in demo mode. Use the teammate
                      tab to walk through the flow.
                    </p>
                  ) : null}
                </div>
              ) : (
                <Alert>
                  <AlertTitle>Demo mode</AlertTitle>
                  <AlertDescription>
                    Couldn&apos;t read your account from the gateway. Use
                    the teammate tab, or bring the backend up and refresh.
                  </AlertDescription>
                </Alert>
              )}

              <Button
                type="button"
                onClick={handleGenerateMine}
                disabled={pending}
                className="w-full"
              >
                <KeyRound className="h-4 w-4" />
                {pending ? "Issuing your key…" : "Generate my virtual key"}
              </Button>
            </TabsContent>

            <TabsContent value="teammate" className="space-y-4 pt-4">
              <form
                className="space-y-4"
                onSubmit={(e) => {
                  e.preventDefault();
                  handleAddTeammate();
                }}
              >
                <div className="grid gap-4 sm:grid-cols-2">
                  <Field
                    id="t_name"
                    label="Name"
                    value={name}
                    onChange={setName}
                    autoComplete="given-name"
                    required
                  />
                  <Field
                    id="t_surname"
                    label="Surname (optional)"
                    value={surname}
                    onChange={setSurname}
                    autoComplete="family-name"
                  />
                </div>
                <Field
                  id="t_email"
                  label="Email"
                  type="email"
                  value={email}
                  onChange={setEmail}
                  autoComplete="email"
                  required
                />
                <div className="grid gap-4 sm:grid-cols-2">
                  <Field
                    id="t_phone"
                    label="Phone (optional)"
                    value={phone}
                    onChange={setPhone}
                    autoComplete="tel"
                  />
                  <Field
                    id="t_department"
                    label="Department (optional)"
                    value={department}
                    onChange={setDepartment}
                  />
                </div>
                <Button
                  type="submit"
                  className="w-full"
                  disabled={pending || !teammateValid}
                >
                  <KeyRound className="h-4 w-4" />
                  {pending ? "Creating user & key…" : "Create user & generate key"}
                </Button>
              </form>
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>

      <UsersList />
    </div>
  );
}

function KeyRevealPane({
  keyData,
  onContinue,
}: {
  keyData: IssuedKey;
  onContinue: () => void;
}) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(keyData.plaintext);
      setCopied(true);
      toast.success("Key copied to clipboard");
    } catch {
      toast.error("Couldn't access the clipboard — copy manually");
      // Allow continue anyway so the user isn't stuck.
      setCopied(true);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <KeyRound className="h-5 w-5 text-muted-foreground" />
          <span>Save your key now</span>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        <Alert variant="destructive">
          <ShieldAlert />
          <AlertTitle>Shown once</AlertTitle>
          <AlertDescription>
            This is the only time the plaintext key is visible. Copy it into
            your password manager or a teammate&apos;s onboarding email before
            continuing — we cannot re-display it.
          </AlertDescription>
        </Alert>

        <div className="space-y-2">
          <Label>Virtual key</Label>
          <div className="flex items-stretch gap-2">
            <pre className="flex-1 overflow-x-auto rounded-md border bg-muted px-3 py-2 font-mono text-xs leading-6">
              {keyData.plaintext}
            </pre>
            <Button type="button" variant="outline" onClick={copy}>
              <Copy className="h-4 w-4" />
              {copied ? "Copied" : "Copy"}
            </Button>
          </div>
        </div>

        <Separator />

        <dl className="grid gap-3 text-sm sm:grid-cols-2">
          <div>
            <dt className="text-xs uppercase tracking-wide text-muted-foreground">Prefix</dt>
            <dd className="font-mono">{keyData.prefix || "—"}</dd>
          </div>
          <div>
            <dt className="text-xs uppercase tracking-wide text-muted-foreground">Allowed providers</dt>
            <dd className="flex flex-wrap gap-1.5">
              {keyData.allowed_providers && keyData.allowed_providers.length > 0 ? (
                keyData.allowed_providers.map((p) => (
                  <Badge key={p} variant="secondary" className="capitalize">
                    {p}
                  </Badge>
                ))
              ) : (
                <Badge variant="outline">all</Badge>
              )}
            </dd>
          </div>
        </dl>

        <Button
          type="button"
          onClick={onContinue}
          disabled={!copied}
          className="w-full"
        >
          {copied ? "Continue" : "Copy the key first"}
        </Button>
      </CardContent>
    </Card>
  );
}

function UsersList() {
  const [items, setItems] = useState<UserListItem[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [demo, setDemo] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    fetch("/api/admin/users", { cache: "no-store" })
      .then(async (r) => {
        const data = (await r.json().catch(() => ({}))) as UsersListResponse;
        if (cancelled) return;
        if (!r.ok) {
          setItems([]);
          setError(data.error ?? `Server returned ${r.status}.`);
          setDemo(Boolean(data.demo));
          return;
        }
        setItems(data.items ?? []);
        setError(null);
        setDemo(false);
      })
      .catch((e) => {
        if (cancelled) return;
        setItems([]);
        setError(e instanceof Error ? e.message : String(e));
        setDemo(true);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Existing users</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <p className="text-sm text-muted-foreground">Loading…</p>
        ) : error ? (
          // TODO: drop the demo banner once `/admin/v1/users` GET ships and
          // we can render an empty-state instead of a soft warning.
          <Alert variant={demo ? "default" : "destructive"}>
            <AlertTitle>{demo ? "Demo mode" : "Couldn't load users"}</AlertTitle>
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        ) : items && items.length > 0 ? (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Email</TableHead>
                <TableHead>Department</TableHead>
                <TableHead>Role</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((u) => (
                <TableRow key={u.id}>
                  <TableCell className="font-medium">
                    {[u.name, u.surname].filter(Boolean).join(" ") || "—"}
                  </TableCell>
                  <TableCell className="font-mono text-xs">{u.email}</TableCell>
                  <TableCell className="text-muted-foreground">
                    {u.department || "—"}
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline" className="capitalize">
                      {u.role ?? "member"}
                    </Badge>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ) : (
          <p className="text-sm text-muted-foreground">No users yet.</p>
        )}
      </CardContent>
    </Card>
  );
}

function Field({
  id,
  label,
  value,
  onChange,
  type = "text",
  required,
  autoComplete,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (next: string) => void;
  type?: string;
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
    </div>
  );
}
