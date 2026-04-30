import type { NextConfig } from "next";
import createNextIntlPlugin from "next-intl/plugin";

const withNextIntl = createNextIntlPlugin("./i18n/request.ts");

const config: NextConfig = {
  reactStrictMode: true,
  output: "standalone",
  images: {
    remotePatterns: [],
  },
};

export default withNextIntl(config);
