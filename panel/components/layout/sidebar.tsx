"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import {
  Activity,
  ChevronLeft,
  Cog,
  FileLock2,
  Gauge,
  KeyRound,
  Plug,
  ScrollText,
  ShieldAlert,
  Users,
} from "lucide-react";
import type { ComponentType, SVGProps } from "react";

import { useUIStore } from "@/lib/stores/ui-store";
import { cn } from "@/lib/utils";
import { Logo } from "@/components/layout/logo";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";

interface NavItem {
  href: string;
  labelKey: string;
  icon: ComponentType<SVGProps<SVGSVGElement>>;
}

const PRIMARY_NAV: NavItem[] = [
  { href: "/dashboard", labelKey: "dashboard", icon: Gauge },
  { href: "/providers", labelKey: "providers", icon: Plug },
  { href: "/users", labelKey: "users", icon: Users },
  { href: "/keys", labelKey: "keys", icon: KeyRound },
  { href: "/policy", labelKey: "policy", icon: FileLock2 },
  { href: "/analytics", labelKey: "analytics", icon: Activity },
  { href: "/logs", labelKey: "logs", icon: ScrollText },
];

const SECONDARY_NAV: NavItem[] = [
  { href: "/settings/profile", labelKey: "settings", icon: Cog },
  { href: "/super-admin", labelKey: "superAdmin", icon: ShieldAlert },
];

export function Sidebar() {
  const pathname = usePathname();
  const collapsed = useUIStore((s) => s.sidebarCollapsed);
  const toggle = useUIStore((s) => s.toggleSidebar);
  const t = useTranslations("nav");

  return (
    <aside
      className={cn(
        "flex h-screen flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground transition-[width] duration-200",
        collapsed ? "w-[68px]" : "w-64",
      )}
    >
      <div className="flex h-14 items-center justify-between px-3">
        <Link
          href="/dashboard"
          className="flex items-center gap-2 overflow-hidden"
        >
          <Logo size={28} withWordmark={!collapsed} />
        </Link>
        <Button
          variant="ghost"
          size="icon"
          aria-label="Toggle sidebar"
          onClick={toggle}
          className="h-8 w-8"
        >
          <ChevronLeft
            className={cn(
              "h-4 w-4 transition-transform",
              collapsed && "rotate-180",
            )}
          />
        </Button>
      </div>
      <Separator />
      <ScrollArea className="flex-1 py-3">
        <nav className="flex flex-col gap-1 px-2">
          {PRIMARY_NAV.map((item) => (
            <NavLink
              key={item.href}
              item={item}
              active={isActive(pathname, item.href)}
              collapsed={collapsed}
              label={t(item.labelKey)}
            />
          ))}
          <div className="my-3">
            <Separator />
          </div>
          {SECONDARY_NAV.map((item) => (
            <NavLink
              key={item.href}
              item={item}
              active={isActive(pathname, item.href)}
              collapsed={collapsed}
              label={t(item.labelKey)}
            />
          ))}
        </nav>
      </ScrollArea>
    </aside>
  );
}

function NavLink({
  item,
  active,
  collapsed,
  label,
}: {
  item: NavItem;
  active: boolean;
  collapsed: boolean;
  label: string;
}) {
  const Icon = item.icon;
  return (
    <Link
      href={item.href}
      className={cn(
        "group flex h-9 items-center gap-2 rounded-md px-2 text-sm font-medium transition-colors",
        active
          ? "bg-sidebar-primary/10 text-sidebar-primary"
          : "text-sidebar-foreground/80 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
      )}
      aria-current={active ? "page" : undefined}
      title={collapsed ? label : undefined}
    >
      <Icon className="h-4 w-4 shrink-0" />
      {collapsed ? null : <span className="truncate">{label}</span>}
    </Link>
  );
}

function isActive(pathname: string | null, href: string): boolean {
  if (!pathname) return false;
  if (href === "/dashboard") return pathname === href;
  return pathname === href || pathname.startsWith(`${href}/`);
}
