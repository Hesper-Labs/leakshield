"use client";

import dynamic from "next/dynamic";

import { Skeleton } from "@/components/ui/skeleton";

/**
 * Monaco is heavy (~1.5 MB minified) and only used on the policy
 * editor screen, so we lazy-load it client-side. Importing
 * `@monaco-editor/react` directly here keeps the component tree below
 * tree-shakable and lets Next.js code-split it into its own chunk.
 */
const Monaco = dynamic(
  () => import("@monaco-editor/react").then((m) => m.default),
  {
    ssr: false,
    loading: () => <Skeleton className="h-full min-h-[400px] w-full rounded-md" />,
  },
);

export function PolicyEditor({
  value,
  onChange,
  language = "yaml",
  height = "100%",
}: {
  value: string;
  onChange?: (next: string) => void;
  language?: "yaml" | "json" | "markdown";
  height?: string | number;
}) {
  return (
    <Monaco
      value={value}
      onChange={(v) => onChange?.(v ?? "")}
      defaultLanguage={language}
      height={height}
      theme="light"
      options={{
        minimap: { enabled: false },
        fontSize: 13,
        scrollBeyondLastLine: false,
        smoothScrolling: true,
        wordWrap: "on",
        renderLineHighlight: "all",
        tabSize: 2,
        automaticLayout: true,
      }}
    />
  );
}
