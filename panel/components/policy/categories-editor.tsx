"use client";

import { Lock, Plus } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

interface BuiltInCategory {
  name: string;
  description: string;
  severity: "ALLOW" | "MASK" | "BLOCK";
}

const BUILT_IN: BuiltInCategory[] = [
  { name: "PII.NAME", description: "Proper-name detection.", severity: "MASK" },
  { name: "PII.EMAIL", description: "RFC 5322 emails.", severity: "MASK" },
  {
    name: "PII.PHONE",
    description: "International + Turkish GSM formats.",
    severity: "MASK",
  },
  {
    name: "PII.TC_KIMLIK",
    description: "Turkish national ID with checksum.",
    severity: "BLOCK",
  },
  {
    name: "PII.IBAN",
    description: "IBAN with MOD-97 validation.",
    severity: "MASK",
  },
  { name: "PII.PASSPORT", description: "Common passport formats.", severity: "MASK" },
  { name: "PII.DOB", description: "Dates of birth.", severity: "MASK" },
  {
    name: "PII.ADDRESS",
    description: "Street addresses (heuristic).",
    severity: "MASK",
  },
  {
    name: "FINANCIAL.CREDIT_CARD",
    description: "Luhn-validated card numbers.",
    severity: "BLOCK",
  },
  {
    name: "FINANCIAL.AMOUNT_TL",
    description: "Turkish lira above threshold.",
    severity: "ALLOW",
  },
  {
    name: "CREDENTIAL.OPENAI_KEY",
    description: "sk-… keys.",
    severity: "BLOCK",
  },
  {
    name: "CREDENTIAL.ANTHROPIC_KEY",
    description: "sk-ant-… keys.",
    severity: "BLOCK",
  },
  {
    name: "CREDENTIAL.AWS_ACCESS_KEY",
    description: "AKIA… patterns.",
    severity: "BLOCK",
  },
  {
    name: "CREDENTIAL.PRIVATE_KEY",
    description: "PEM headers.",
    severity: "BLOCK",
  },
  {
    name: "CODE.SECRET_IN_SOURCE",
    description: "Source-code blocks containing the above.",
    severity: "BLOCK",
  },
];

export function CategoriesEditor() {
  return (
    <div className="grid gap-4 lg:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <Lock className="h-4 w-4 text-muted-foreground" />
            Built-in catalog
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <ScrollArea className="max-h-[500px]">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[200px]">Category</TableHead>
                  <TableHead>Description</TableHead>
                  <TableHead className="w-[90px]">Severity</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {BUILT_IN.map((c) => (
                  <TableRow key={c.name}>
                    <TableCell className="font-mono text-xs">{c.name}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {c.description}
                    </TableCell>
                    <TableCell>
                      <SeverityBadge severity={c.severity} />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </ScrollArea>
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="text-base">Custom categories</CardTitle>
          <Button size="sm">
            <Plus className="h-4 w-4" />
            Add custom category
          </Button>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            None yet. The wizard walks you through name, description, severity,
            and the matching mechanism: keyword list, regex, document
            fingerprint, LLM-only description, or hashed customer directory CSV.
          </p>
        </CardContent>
      </Card>
    </div>
  );
}

function SeverityBadge({
  severity,
}: {
  severity: BuiltInCategory["severity"];
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
