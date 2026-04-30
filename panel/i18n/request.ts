import { cookies, headers } from "next/headers";
import { getRequestConfig } from "next-intl/server";

export const locales = ["en", "tr"] as const;
export type Locale = (typeof locales)[number];
export const defaultLocale: Locale = "en";

const COOKIE_NAME = "leakshield-locale";

function negotiateLocale(headerValue: string | null): Locale {
  if (!headerValue) return defaultLocale;
  const requested = headerValue
    .split(",")
    .map((token) => token.trim().split(";")[0]?.toLowerCase() ?? "")
    .filter(Boolean);
  for (const tag of requested) {
    if (tag.startsWith("tr")) return "tr";
    if (tag.startsWith("en")) return "en";
  }
  return defaultLocale;
}

export default getRequestConfig(async () => {
  const cookieStore = await cookies();
  const headerStore = await headers();
  const fromCookie = cookieStore.get(COOKIE_NAME)?.value as Locale | undefined;
  const locale: Locale =
    fromCookie && (locales as readonly string[]).includes(fromCookie)
      ? fromCookie
      : negotiateLocale(headerStore.get("accept-language"));

  const messages = (await import(`../messages/${locale}.json`)).default as Record<
    string,
    unknown
  >;

  return {
    locale,
    messages,
  };
});
