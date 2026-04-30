import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatDate(value: Date | string | number): string {
  const d = value instanceof Date ? value : new Date(value);
  return d.toLocaleString();
}
