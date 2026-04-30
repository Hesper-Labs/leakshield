"use client";

import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

const SAMPLE_TRAFFIC = Array.from({ length: 24 }, (_, hour) => ({
  hour: `${String(hour).padStart(2, "0")}:00`,
  allowed: Math.round(Math.sin(hour / 3) * 80 + 200 + Math.random() * 40),
  blocked: Math.round(Math.cos(hour / 5) * 18 + 22 + Math.random() * 6),
}));

/**
 * Static preview of the analytics chart wired to mock data so the
 * panel feels populated before the gateway exposes its analytics
 * endpoint. Real charts replace this in Phase 4.
 */
export function AnalyticsPreview() {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-base">Traffic (last 24h, mock)</CardTitle>
      </CardHeader>
      <CardContent className="h-[280px]">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={SAMPLE_TRAFFIC} margin={{ top: 4, right: 12, bottom: 0, left: 0 }}>
            <defs>
              <linearGradient id="allowedFill" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="var(--color-chart-1)" stopOpacity={0.4} />
                <stop offset="100%" stopColor="var(--color-chart-1)" stopOpacity={0} />
              </linearGradient>
              <linearGradient id="blockedFill" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="var(--color-destructive)" stopOpacity={0.4} />
                <stop offset="100%" stopColor="var(--color-destructive)" stopOpacity={0} />
              </linearGradient>
            </defs>
            <CartesianGrid stroke="var(--color-border)" strokeDasharray="3 3" />
            <XAxis dataKey="hour" stroke="var(--color-muted-foreground)" fontSize={11} />
            <YAxis stroke="var(--color-muted-foreground)" fontSize={11} />
            <Tooltip
              contentStyle={{
                background: "var(--color-popover)",
                border: "1px solid var(--color-border)",
                borderRadius: 6,
                fontSize: 12,
              }}
            />
            <Area
              type="monotone"
              dataKey="allowed"
              stroke="var(--color-chart-1)"
              fill="url(#allowedFill)"
              strokeWidth={2}
            />
            <Area
              type="monotone"
              dataKey="blocked"
              stroke="var(--color-destructive)"
              fill="url(#blockedFill)"
              strokeWidth={2}
            />
          </AreaChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}
