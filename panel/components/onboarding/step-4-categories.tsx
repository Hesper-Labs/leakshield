"use client";

import { useState } from "react";
import {
  Briefcase,
  Code2,
  ContactRound,
  Handshake,
  Plus,
} from "lucide-react";
import type { ComponentType, SVGProps } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

interface QuickStartTemplate {
  id: string;
  name: string;
  prompt: string;
  description: string;
  severity: "ALLOW" | "MASK" | "BLOCK";
  icon: ComponentType<SVGProps<SVGSVGElement>>;
}

const TEMPLATES: QuickStartTemplate[] = [
  {
    id: "project-codenames",
    name: "Project codenames",
    prompt: "We have proprietary project codenames",
    description:
      "Spin up a keyword-list category for your internal product / project codenames so they never leave the building.",
    severity: "BLOCK",
    icon: Briefcase,
  },
  {
    id: "customer-directory",
    name: "Customer / vendor directory",
    prompt: "We have customer / vendor lists",
    description:
      "Upload a CSV of names; it's hashed at rest and evaluated through a Bloom filter so the raw list never enters LLM context.",
    severity: "MASK",
    icon: ContactRound,
  },
  {
    id: "mna",
    name: "M&A / pending deals",
    prompt: "We want to block pending M&A discussions",
    description:
      "Adds an LLM-judged category with a pre-written description for any mention of pending deals, due diligence, or related advisors.",
    severity: "BLOCK",
    icon: Handshake,
  },
  {
    id: "code-secrets",
    name: "Source code secrets",
    prompt: "Keep source-code secrets out of prompts",
    description:
      "Toggles the built-in CODE.SECRET_IN_SOURCE category to BLOCK so PEM blocks, API keys, and AWS credentials in pasted code are stopped.",
    severity: "BLOCK",
    icon: Code2,
  },
];

/**
 * Quick-start template picker for the optional "Add company-custom DLP
 * categories" sub-screen on Step 4. Selections are local-only until
 * the gateway exposes the categories endpoint; submission then becomes
 * a single batched call.
 */
export function Step4Categories() {
  const [selected, setSelected] = useState<Set<string>>(new Set());

  function toggle(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center justify-between text-base">
          <span>Add company-custom DLP categories</span>
          <Badge variant="outline">Optional</Badge>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <p className="text-sm text-muted-foreground">
          Pick any quick-start templates that match your business. You can refine
          them — and add fully custom ones — from{" "}
          <span className="font-medium text-foreground">Policy → Categories</span>{" "}
          afterwards.
        </p>
        <div className="grid gap-3 sm:grid-cols-2">
          {TEMPLATES.map((template) => {
            const Icon = template.icon;
            const active = selected.has(template.id);
            return (
              <button
                key={template.id}
                type="button"
                onClick={() => toggle(template.id)}
                className={cn(
                  "flex flex-col items-start gap-2 rounded-lg border p-3 text-left transition-colors",
                  active
                    ? "border-primary bg-primary/5"
                    : "border-border hover:bg-muted",
                )}
                aria-pressed={active}
              >
                <span className="flex w-full items-center justify-between">
                  <span className="flex items-center gap-2">
                    <Icon className="h-4 w-4" aria-hidden />
                    <span className="font-medium">{template.name}</span>
                  </span>
                  <SeverityBadge severity={template.severity} />
                </span>
                <p className="text-xs text-muted-foreground">
                  {template.description}
                </p>
              </button>
            );
          })}
        </div>
        <Button variant="outline" type="button" className="w-full">
          <Plus className="h-4 w-4" />
          Add a fully custom category
        </Button>
      </CardContent>
    </Card>
  );
}

function SeverityBadge({
  severity,
}: {
  severity: QuickStartTemplate["severity"];
}) {
  const variant =
    severity === "BLOCK"
      ? ("destructive" as const)
      : severity === "MASK"
      ? ("warning" as const)
      : ("secondary" as const);
  return (
    <Badge variant={variant} className="text-[10px] uppercase tracking-wide">
      {severity}
    </Badge>
  );
}
