"use client";

import { Bell, LogOut, User } from "lucide-react";
import { signOut, useSession } from "next-auth/react";

import {
  Avatar,
  AvatarFallback,
  AvatarImage,
} from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Separator } from "@/components/ui/separator";
import { TenantSwitcher } from "@/components/layout/tenant-switcher";

export function Topbar() {
  const { data: session } = useSession();
  const initials =
    session?.user?.name?.slice(0, 2).toUpperCase() ??
    session?.user?.email?.slice(0, 2).toUpperCase() ??
    "LS";

  return (
    <header className="flex h-14 items-center gap-4 border-b border-border bg-background px-4">
      <TenantSwitcher />
      <Separator orientation="vertical" className="h-6" />
      <div className="ml-auto flex items-center gap-1">
        <Button
          variant="ghost"
          size="icon"
          aria-label="Notifications"
          className="relative"
        >
          <Bell className="h-4 w-4" />
          <span className="absolute right-2 top-2 inline-flex h-1.5 w-1.5 rounded-full bg-primary" />
        </Button>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              aria-label="User menu"
              className="rounded-full"
            >
              <Avatar className="h-7 w-7">
                <AvatarImage src={session?.user?.image ?? undefined} alt="" />
                <AvatarFallback className="text-[11px]">{initials}</AvatarFallback>
              </Avatar>
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-56">
            <DropdownMenuLabel className="flex flex-col gap-0.5">
              <span>{session?.user?.name ?? "Signed in"}</span>
              <span className="text-xs font-normal text-muted-foreground">
                {session?.user?.email}
              </span>
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem asChild>
              <a href="/settings/profile">
                <User className="h-4 w-4" />
                Profile
              </a>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => signOut({ callbackUrl: "/sign-in" })}>
              <LogOut className="h-4 w-4" />
              Sign out
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  );
}
