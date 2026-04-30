"use client";

import { useEffect, useMemo, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import {
  CheckCircle2,
  Cloud,
  CloudCog,
  Globe,
  Plus,
  Sparkles,
  Trash2,
  XCircle,
} from "lucide-react";
import type { ComponentType, SVGProps } from "react";
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
import { cn } from "@/lib/utils";

type ProviderId = "openai" | "anthropic" | "google" | "azure";

interface ProviderTile {
  id: ProviderId;
  name: string;
  blurb: string;
  icon: ComponentType<SVGProps<SVGSVGElement>>;
}

const PROVIDERS: ProviderTile[] = [
  {
    id: "openai",
    name: "OpenAI",
    blurb: "GPT-4o, GPT-4.1, o-series",
    icon: Sparkles,
  },
  {
    id: "anthropic",
    name: "Anthropic",
    blurb: "Claude Sonnet, Opus, Haiku",
    icon: Cloud,
  },
  {
    id: "google",
    name: "Google",
    blurb: "Gemini via Vertex AI",
    icon: Globe,
  },
  {
    id: "azure",
    name: "Azure OpenAI",
    blurb: "Deployed via Azure resources",
    icon: CloudCog,
  },
];

interface DeploymentRow {
  model: string;
  deployment: string;
}

interface FormState {
  label: string;
  apiKey: string;
  // OpenAI
  baseUrl: string;
  orgId: string;
  // Anthropic shares baseUrl with OpenAI's optional one.
  // Google
  projectId: string;
  // Azure
  azureEndpoint: string;
  azureApiVersion: string;
  azureDeployments: DeploymentRow[];
}

function emptyForm(): FormState {
  return {
    label: "",
    apiKey: "",
    baseUrl: "",
    orgId: "",
    projectId: "",
    azureEndpoint: "",
    azureApiVersion: "2024-08-01-preview",
    azureDeployments: [{ model: "", deployment: "" }],
  };
}

interface TestResult {
  ok: boolean;
  models?: string[];
  error?: string;
  demo?: boolean;
}

interface SavedProvider {
  id: string;
  provider: ProviderId;
  label: string;
  allowed_models?: string[];
}

interface ProviderListResponse {
  items?: SavedProvider[];
  ok?: boolean;
  error?: string;
  demo?: boolean;
}

/**
 * Onboarding step 2: connect at least one upstream LLM provider.
 *
 * Flow:
 *   1. User picks one of OpenAI / Anthropic / Google / Azure.
 *   2. The form for the selected provider renders inline.
 *   3. "Test connection" probes the gateway, which makes a cheap upstream
 *      call (e.g. `GET /v1/models`) and returns the discovered model list.
 *   4. Once a test has succeeded — or the user explicitly opts out — Save
 *      persists the provider and routes to step 3.
 *
 * Below the form a `<ProviderList />` is mounted so users can see the
 * providers they've already connected; remove/disable are stubs for now.
 */
export function Step2Provider() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [selected, setSelected] = useState<ProviderId | null>(null);
  const [form, setForm] = useState<FormState>(emptyForm());
  const [testResult, setTestResult] = useState<TestResult | null>(null);
  const [skipTest, setSkipTest] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saveDemo, setSaveDemo] = useState(false);
  const [demoBanner, setDemoBanner] = useState<string | null>(null);
  const [refreshSignal, setRefreshSignal] = useState(0);

  // Probe the gateway from the browser so we can show the same demo-mode
  // banner step 1 shows when the backend isn't running. The page's server
  // component already renders one banner on initial paint; this picks up
  // mid-session backend churn without a hard reload.
  useEffect(() => {
    let cancelled = false;
    fetch("/api/setup/status", { cache: "no-store" })
      .then((r) => r.json())
      .then((data) => {
        if (cancelled) return;
        if (data && data.reachable === false) {
          setDemoBanner(
            "Gateway is offline — the form is wired but Test/Save will fail until the backend is reachable.",
          );
        } else {
          setDemoBanner(null);
        }
      })
      .catch(() => {
        if (cancelled) return;
        setDemoBanner("Gateway is offline — Test/Save will fail until the backend is reachable.");
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const requiredOk = useMemo(() => isFormValid(selected, form), [selected, form]);
  const canSave = requiredOk && (testResult?.ok === true || skipTest);

  function pick(id: ProviderId) {
    setSelected(id);
    setForm((prev) => ({ ...emptyForm(), label: prev.label }));
    setTestResult(null);
    setSaveError(null);
    setSaveDemo(false);
  }

  function update<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((prev) => ({ ...prev, [key]: value }));
    // Any edit invalidates the previous test result.
    if (testResult) setTestResult(null);
  }

  function updateAzureRow(index: number, patch: Partial<DeploymentRow>) {
    setForm((prev) => ({
      ...prev,
      azureDeployments: prev.azureDeployments.map((row, i) =>
        i === index ? { ...row, ...patch } : row,
      ),
    }));
    if (testResult) setTestResult(null);
  }

  function addAzureRow() {
    setForm((prev) => ({
      ...prev,
      azureDeployments: [...prev.azureDeployments, { model: "", deployment: "" }],
    }));
  }

  function removeAzureRow(index: number) {
    setForm((prev) => ({
      ...prev,
      azureDeployments:
        prev.azureDeployments.length > 1
          ? prev.azureDeployments.filter((_, i) => i !== index)
          : prev.azureDeployments,
    }));
  }

  function handleTest() {
    if (!selected || !requiredOk) return;
    setTestResult(null);
    startTransition(async () => {
      const res = await fetch("/api/admin/providers/test", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          provider: selected,
          api_key: form.apiKey,
          config: buildConfig(selected, form),
        }),
      });
      const data = (await res.json().catch(() => ({}))) as TestResult;

      if (res.ok && data.ok) {
        setTestResult({ ok: true, models: data.models ?? [] });
      } else {
        setTestResult({
          ok: false,
          error: data.error ?? `Server returned ${res.status}.`,
          demo: Boolean(data.demo),
        });
      }
    });
  }

  function handleSave() {
    if (!selected || !canSave) return;
    setSaveError(null);
    setSaveDemo(false);

    startTransition(async () => {
      const res = await fetch("/api/admin/providers", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          provider: selected,
          label: form.label.trim(),
          api_key: form.apiKey,
          config: buildConfig(selected, form),
          allowed_models: testResult?.models ?? [],
        }),
      });
      const data = (await res.json().catch(() => ({}))) as {
        ok?: boolean;
        error?: string;
        demo?: boolean;
        id?: string;
      };

      if (!res.ok) {
        setSaveDemo(Boolean(data.demo));
        setSaveError(data.error ?? `Server returned ${res.status}.`);
        return;
      }

      toast.success("Provider connected");
      setRefreshSignal((s) => s + 1);
      router.replace("/onboarding/3");
      router.refresh();
    });
  }

  return (
    <div className="space-y-6">
      {demoBanner ? (
        <Alert>
          <AlertTitle>Demo mode</AlertTitle>
          <AlertDescription>{demoBanner}</AlertDescription>
        </Alert>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle>Connect a provider</CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          <p className="text-sm text-muted-foreground">
            Point one or more LLM keys at the gateway. We test the key once and
            store it encrypted under the tenant DEK; teammates use a virtual key
            instead of the upstream secret.
          </p>

          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            {PROVIDERS.map((p) => {
              const Icon = p.icon;
              const active = selected === p.id;
              return (
                <button
                  key={p.id}
                  type="button"
                  onClick={() => pick(p.id)}
                  className={cn(
                    "flex flex-col items-start gap-2 rounded-lg border p-4 text-left transition-colors",
                    active
                      ? "border-primary bg-primary/5 ring-2 ring-primary/30"
                      : "border-border hover:bg-muted",
                  )}
                  aria-pressed={active}
                >
                  <span className="flex w-full items-center justify-between">
                    <Icon className="h-5 w-5 text-muted-foreground" aria-hidden />
                    {active ? (
                      <Badge variant="default" className="text-[10px]">
                        Selected
                      </Badge>
                    ) : null}
                  </span>
                  <span className="font-medium">{p.name}</span>
                  <span className="text-xs text-muted-foreground">{p.blurb}</span>
                </button>
              );
            })}
          </div>

          {selected ? (
            <>
              <Separator />

              <ProviderForm
                provider={selected}
                form={form}
                onUpdate={update}
                onUpdateAzureRow={updateAzureRow}
                onAddAzureRow={addAzureRow}
                onRemoveAzureRow={removeAzureRow}
              />

              {testResult ? (
                testResult.ok ? (
                  <Alert variant="success">
                    <CheckCircle2 />
                    <AlertTitle>
                      Detected {testResult.models?.length ?? 0} model
                      {testResult.models?.length === 1 ? "" : "s"}
                    </AlertTitle>
                    <AlertDescription>
                      {testResult.models && testResult.models.length > 0 ? (
                        <span className="flex flex-wrap gap-1.5">
                          {testResult.models.slice(0, 12).map((m) => (
                            <Badge key={m} variant="secondary" className="font-mono text-[11px]">
                              {m}
                            </Badge>
                          ))}
                          {testResult.models.length > 12 ? (
                            <span className="text-xs text-muted-foreground">
                              +{testResult.models.length - 12} more
                            </span>
                          ) : null}
                        </span>
                      ) : (
                        <span>The key works but no models were returned.</span>
                      )}
                    </AlertDescription>
                  </Alert>
                ) : (
                  <Alert variant={testResult.demo ? "default" : "destructive"}>
                    <XCircle />
                    <AlertTitle>{testResult.demo ? "Demo mode" : "Connection failed"}</AlertTitle>
                    <AlertDescription>{testResult.error ?? "Unknown error."}</AlertDescription>
                  </Alert>
                )
              ) : null}

              <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <label className="flex items-center gap-2 text-sm text-muted-foreground">
                  <input
                    type="checkbox"
                    className="h-4 w-4 rounded border-input"
                    checked={skipTest}
                    onChange={(e) => setSkipTest(e.target.checked)}
                  />
                  Skip test connection (save without verifying)
                </label>
                <div className="flex gap-2">
                  <Button
                    type="button"
                    variant="outline"
                    onClick={handleTest}
                    disabled={pending || !requiredOk}
                  >
                    {pending && testResult === null ? "Testing…" : "Test connection"}
                  </Button>
                  <Button
                    type="button"
                    onClick={handleSave}
                    disabled={pending || !canSave}
                  >
                    {pending && testResult?.ok ? "Saving…" : "Save & continue"}
                  </Button>
                </div>
              </div>

              {saveError ? (
                <Alert variant={saveDemo ? "default" : "destructive"}>
                  <AlertTitle>{saveDemo ? "Demo mode" : "Couldn't save the provider"}</AlertTitle>
                  <AlertDescription>{saveError}</AlertDescription>
                </Alert>
              ) : null}
            </>
          ) : (
            <p className="text-sm text-muted-foreground">
              Pick a provider above to continue.
            </p>
          )}
        </CardContent>
      </Card>

      <ProviderList refreshSignal={refreshSignal} />
    </div>
  );
}

function ProviderForm({
  provider,
  form,
  onUpdate,
  onUpdateAzureRow,
  onAddAzureRow,
  onRemoveAzureRow,
}: {
  provider: ProviderId;
  form: FormState;
  onUpdate: <K extends keyof FormState>(key: K, value: FormState[K]) => void;
  onUpdateAzureRow: (index: number, patch: Partial<DeploymentRow>) => void;
  onAddAzureRow: () => void;
  onRemoveAzureRow: (index: number) => void;
}) {
  return (
    <div className="space-y-4">
      <div className="grid gap-4 sm:grid-cols-2">
        <Field
          id="label"
          label="Label"
          value={form.label}
          onChange={(v) => onUpdate("label", v)}
          hint="Shown in the audit log and routing UI"
          placeholder="e.g. Production OpenAI"
          required
        />
        <Field
          id="api_key"
          label="API key"
          type="password"
          value={form.apiKey}
          onChange={(v) => onUpdate("apiKey", v)}
          hint="Stored encrypted under the tenant DEK"
          autoComplete="off"
          mono
          required
        />
      </div>

      {provider === "openai" ? (
        <div className="grid gap-4 sm:grid-cols-2">
          <Field
            id="base_url"
            label="Base URL (optional)"
            value={form.baseUrl}
            onChange={(v) => onUpdate("baseUrl", v)}
            placeholder="https://api.openai.com/v1"
            hint="Override for OpenAI-compatible endpoints"
          />
          <Field
            id="org_id"
            label="Organization ID (optional)"
            value={form.orgId}
            onChange={(v) => onUpdate("orgId", v)}
            placeholder="org-…"
          />
        </div>
      ) : null}

      {provider === "anthropic" ? (
        <Field
          id="base_url"
          label="Base URL (optional)"
          value={form.baseUrl}
          onChange={(v) => onUpdate("baseUrl", v)}
          placeholder="https://api.anthropic.com"
          hint="Override for self-hosted or proxied Anthropic-compatible endpoints"
        />
      ) : null}

      {provider === "google" ? (
        <Field
          id="project_id"
          label="Project ID"
          value={form.projectId}
          onChange={(v) => onUpdate("projectId", v)}
          placeholder="my-gcp-project"
          hint="Vertex AI requires the GCP project ID alongside the key"
          required
        />
      ) : null}

      {provider === "azure" ? (
        <div className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <Field
              id="azure_endpoint"
              label="Endpoint"
              value={form.azureEndpoint}
              onChange={(v) => onUpdate("azureEndpoint", v)}
              placeholder="https://my-resource.openai.azure.com"
              required
            />
            <Field
              id="azure_api_version"
              label="API version"
              value={form.azureApiVersion}
              onChange={(v) => onUpdate("azureApiVersion", v)}
              required
            />
          </div>
          <div className="space-y-2">
            <Label>Deployment map</Label>
            <p className="text-xs text-muted-foreground">
              Azure exposes models under per-resource deployment names. Map the
              canonical model id to your deployment so the gateway can route by
              name.
            </p>
            <div className="space-y-2">
              {form.azureDeployments.map((row, index) => (
                <div
                  key={index}
                  className="grid grid-cols-[1fr_1fr_auto] items-center gap-2"
                >
                  <Input
                    placeholder="model (e.g. gpt-4o)"
                    value={row.model}
                    onChange={(e) =>
                      onUpdateAzureRow(index, { model: e.target.value })
                    }
                    className="font-mono text-xs"
                  />
                  <Input
                    placeholder="deployment name"
                    value={row.deployment}
                    onChange={(e) =>
                      onUpdateAzureRow(index, { deployment: e.target.value })
                    }
                    className="font-mono text-xs"
                  />
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    onClick={() => onRemoveAzureRow(index)}
                    disabled={form.azureDeployments.length <= 1}
                    aria-label="Remove deployment"
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              ))}
            </div>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={onAddAzureRow}
            >
              <Plus className="h-4 w-4" />
              Add deployment
            </Button>
          </div>
        </div>
      ) : null}
    </div>
  );
}

function ProviderList({ refreshSignal }: { refreshSignal: number }) {
  const [items, setItems] = useState<SavedProvider[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [demo, setDemo] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    fetch("/api/admin/providers", { cache: "no-store" })
      .then(async (r) => {
        const data = (await r.json().catch(() => ({}))) as ProviderListResponse;
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
  }, [refreshSignal]);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Connected providers</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {loading ? (
          <p className="text-sm text-muted-foreground">Loading…</p>
        ) : error ? (
          <Alert variant={demo ? "default" : "destructive"}>
            <AlertTitle>{demo ? "Demo mode" : "Couldn't load providers"}</AlertTitle>
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        ) : items && items.length > 0 ? (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Label</TableHead>
                <TableHead>Provider</TableHead>
                <TableHead>Models</TableHead>
                <TableHead className="w-[80px]"> </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-medium">{item.label}</TableCell>
                  <TableCell>
                    <Badge variant="outline" className="capitalize">
                      {item.provider}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {item.allowed_models && item.allowed_models.length > 0
                      ? `${item.allowed_models.length} allowed`
                      : "all"}
                  </TableCell>
                  <TableCell className="text-right">
                    {/* TODO: wire up disable/delete once the gateway exposes
                        PATCH/DELETE /admin/v1/providers/{id}. */}
                    <Button variant="ghost" size="sm" disabled>
                      Manage
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ) : (
          <p className="text-sm text-muted-foreground">
            No providers yet. Connect one above to enable the gateway.
          </p>
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
  hint,
  placeholder,
  required,
  type = "text",
  autoComplete,
  mono = false,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (next: string) => void;
  hint?: string;
  placeholder?: string;
  required?: boolean;
  type?: string;
  autoComplete?: string;
  mono?: boolean;
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
        placeholder={placeholder}
        autoComplete={autoComplete}
        required={required}
        className={mono ? "font-mono" : undefined}
      />
      {hint ? <p className="text-xs text-muted-foreground">{hint}</p> : null}
    </div>
  );
}

function isFormValid(provider: ProviderId | null, form: FormState): boolean {
  if (!provider) return false;
  if (form.label.trim().length === 0) return false;
  if (form.apiKey.length === 0) return false;
  if (provider === "google" && form.projectId.trim().length === 0) return false;
  if (provider === "azure") {
    if (form.azureEndpoint.trim().length === 0) return false;
    if (form.azureApiVersion.trim().length === 0) return false;
    // At least one row with both fields filled in is required.
    const hasRow = form.azureDeployments.some(
      (r) => r.model.trim().length > 0 && r.deployment.trim().length > 0,
    );
    if (!hasRow) return false;
  }
  return true;
}

function buildConfig(provider: ProviderId, form: FormState): Record<string, unknown> {
  switch (provider) {
    case "openai": {
      const config: Record<string, unknown> = {};
      if (form.baseUrl.trim()) config.base_url = form.baseUrl.trim();
      if (form.orgId.trim()) config.org_id = form.orgId.trim();
      return config;
    }
    case "anthropic": {
      const config: Record<string, unknown> = {};
      if (form.baseUrl.trim()) config.base_url = form.baseUrl.trim();
      return config;
    }
    case "google":
      return { project_id: form.projectId.trim() };
    case "azure":
      return {
        endpoint: form.azureEndpoint.trim(),
        api_version: form.azureApiVersion.trim(),
        deployments: form.azureDeployments
          .filter((r) => r.model.trim() && r.deployment.trim())
          .map((r) => ({ model: r.model.trim(), deployment: r.deployment.trim() })),
      };
  }
}
