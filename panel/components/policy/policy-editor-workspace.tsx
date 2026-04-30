"use client";

import { History, Play, Variable } from "lucide-react";
import { useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import { PolicyEditor } from "@/components/policy/policy-editor-loader";

const SAMPLE_POLICY = `# LeakShield DLP policy
# Edit the judge prompt and category list, then run the test harness on the right.
strategy: hybrid
backend:
  kind: ollama
  model: qwen2.5:3b
categories:
  - name: PII.EMAIL
    severity: MASK
  - name: PROJECT.BLUEMOON
    severity: BLOCK
    keywords:
      - "Project Bluemoon"
      - "PBM-"
`;

const SAMPLE_VARIABLES = [
  { name: "company_name", description: "Filled at runtime from the tenant." },
  { name: "user_name", description: "Filled from the calling user." },
  { name: "request_kind", description: "chat | completion | embedding." },
];

const SAMPLE_TEMPLATES = [
  { name: "Hybrid default", description: "Recommended starting point." },
  {
    name: "Strict block-everything",
    description: "Useful for evaluating false-positive rates.",
  },
  { name: "Open / mock", description: "Always ALLOW; for stack tests." },
];

const SAMPLE_VERSIONS = [
  { id: "v3", label: "Current draft", note: "Unsaved changes" },
  { id: "v2", label: "Published 2026-04-25", note: "Active" },
  { id: "v1", label: "2026-04-12", note: "Pre-Bluemoon launch" },
];

export function PolicyEditorWorkspace() {
  const [policy, setPolicy] = useState<string>(SAMPLE_POLICY);
  const [output] = useState<string>(
    "Run the test harness with a sample prompt to see the verdict here.",
  );

  return (
    <div className="grid grid-cols-1 gap-4 lg:grid-cols-[1fr_320px]">
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-[60%_40%]">
        <Card className="flex flex-col">
          <CardHeader className="flex flex-row items-center justify-between gap-2 pb-3">
            <CardTitle className="text-base">Policy</CardTitle>
            <div className="flex items-center gap-2">
              <Sheet>
                <SheetTrigger asChild>
                  <Button variant="outline" size="sm">
                    <History className="h-4 w-4" />
                    Versions
                  </Button>
                </SheetTrigger>
                <SheetContent>
                  <SheetHeader>
                    <SheetTitle>Versions</SheetTitle>
                    <SheetDescription>
                      Prior published policies. Select one to diff against the
                      current draft.
                    </SheetDescription>
                  </SheetHeader>
                  <ul className="mt-4 flex flex-col gap-2">
                    {SAMPLE_VERSIONS.map((v) => (
                      <li
                        key={v.id}
                        className="flex items-center justify-between rounded-md border p-3 text-sm"
                      >
                        <span>
                          <span className="font-medium">{v.label}</span>
                          <span className="ml-2 text-muted-foreground">
                            {v.note}
                          </span>
                        </span>
                        <Badge variant="outline">{v.id}</Badge>
                      </li>
                    ))}
                  </ul>
                </SheetContent>
              </Sheet>
              <Button size="sm">Save draft</Button>
            </div>
          </CardHeader>
          <CardContent className="flex-1 p-0">
            <div className="h-[520px] w-full">
              <PolicyEditor value={policy} onChange={setPolicy} />
            </div>
          </CardContent>
        </Card>
        <Card className="flex flex-col">
          <CardHeader className="flex flex-row items-center justify-between pb-3">
            <CardTitle className="text-base">Test harness</CardTitle>
            <Button size="sm" variant="secondary">
              <Play className="h-4 w-4" />
              Run
            </Button>
          </CardHeader>
          <CardContent className="flex-1 space-y-3">
            <textarea
              defaultValue="Type a sample prompt here, e.g. 'My TC kimlik is 12345678901, please format an invoice.'"
              className="h-32 w-full resize-none rounded-md border border-input bg-transparent p-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
            />
            <div className="rounded-md border bg-muted/40 p-3 text-xs text-muted-foreground">
              {output}
            </div>
          </CardContent>
        </Card>
      </div>
      <Card className="self-start">
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Variables &amp; templates</CardTitle>
        </CardHeader>
        <CardContent>
          <Tabs defaultValue="variables">
            <TabsList className="grid w-full grid-cols-2">
              <TabsTrigger value="variables">
                <Variable className="mr-1 h-3.5 w-3.5" /> Variables
              </TabsTrigger>
              <TabsTrigger value="templates">Templates</TabsTrigger>
            </TabsList>
            <TabsContent value="variables" className="mt-3 space-y-2">
              {SAMPLE_VARIABLES.map((v) => (
                <div key={v.name} className="rounded-md border p-2 text-xs">
                  <code className="font-mono text-foreground">{`{{${v.name}}}`}</code>
                  <p className="mt-1 text-muted-foreground">{v.description}</p>
                </div>
              ))}
            </TabsContent>
            <TabsContent value="templates" className="mt-3 space-y-2">
              {SAMPLE_TEMPLATES.map((t) => (
                <button
                  key={t.name}
                  type="button"
                  className="w-full rounded-md border p-2 text-left text-xs transition-colors hover:bg-muted"
                >
                  <div className="font-medium text-foreground">{t.name}</div>
                  <p className="mt-1 text-muted-foreground">{t.description}</p>
                </button>
              ))}
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>
    </div>
  );
}
