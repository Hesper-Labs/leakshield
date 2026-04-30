"use client";

import { Check, ChevronsUpDown, Plus } from "lucide-react";
import { useState } from "react";

import {
  Avatar,
  AvatarFallback,
  AvatarImage,
} from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "@/components/ui/command";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";

/**
 * GitHub-org-style tenant picker. Wired against a mock list until the
 * gateway exposes `/admin/v1/tenants` — once it does, this component
 * swaps to a TanStack Query hook against `useApi`.
 */
interface Tenant {
  id: string;
  name: string;
  slug: string;
  avatarUrl?: string;
}

const MOCK_TENANTS: Tenant[] = [
  { id: "1", name: "Acme Inc", slug: "acme" },
  { id: "2", name: "Hesper Labs", slug: "hesper" },
];

export function TenantSwitcher() {
  const [open, setOpen] = useState(false);
  const [tenant, setTenant] = useState<Tenant>(
    MOCK_TENANTS[0] ?? { id: "0", name: "Default", slug: "default" },
  );

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          role="combobox"
          aria-expanded={open}
          aria-label="Select tenant"
          className="w-[220px] justify-between"
        >
          <span className="flex items-center gap-2 truncate">
            <Avatar className="h-5 w-5">
              <AvatarImage src={tenant.avatarUrl} alt="" />
              <AvatarFallback className="text-[10px]">
                {tenant.name.slice(0, 2).toUpperCase()}
              </AvatarFallback>
            </Avatar>
            <span className="truncate">{tenant.name}</span>
          </span>
          <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[260px] p-0" align="start">
        <Command>
          <CommandInput placeholder="Search tenants…" />
          <CommandList>
            <CommandEmpty>No tenants found.</CommandEmpty>
            <CommandGroup heading="Tenants">
              {MOCK_TENANTS.map((t) => (
                <CommandItem
                  key={t.id}
                  value={`${t.name} ${t.slug}`}
                  onSelect={() => {
                    setTenant(t);
                    setOpen(false);
                  }}
                >
                  <Avatar className="mr-2 h-5 w-5">
                    <AvatarFallback className="text-[10px]">
                      {t.name.slice(0, 2).toUpperCase()}
                    </AvatarFallback>
                  </Avatar>
                  <span className="truncate">{t.name}</span>
                  <Check
                    className={cn(
                      "ml-auto h-4 w-4",
                      tenant.id === t.id ? "opacity-100" : "opacity-0",
                    )}
                  />
                </CommandItem>
              ))}
            </CommandGroup>
            <CommandSeparator />
            <CommandGroup>
              <CommandItem
                onSelect={() => {
                  setOpen(false);
                  // Wired to a future "create tenant" dialog.
                }}
              >
                <Plus className="mr-2 h-4 w-4" />
                Create tenant
              </CommandItem>
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
