import NextAuth, { type DefaultSession } from "next-auth";
import Credentials from "next-auth/providers/credentials";

import { getInternalGatewayUrl } from "@/lib/env";

declare module "next-auth" {
  interface User {
    tenantId?: string;
    role?: "admin" | "member" | "super_admin";
    accessToken?: string;
  }

  interface Session {
    accessToken?: string;
    user: {
      tenantId?: string;
      role?: "admin" | "member" | "super_admin";
    } & DefaultSession["user"];
  }
}

declare module "next-auth/jwt" {
  interface JWT {
    accessToken?: string;
    tenantId?: string;
    role?: "admin" | "member" | "super_admin";
  }
}

interface AuthApiResponse {
  token: string;
  user: {
    id: string;
    email: string;
    name?: string;
    tenant_id?: string;
    role?: "admin" | "member" | "super_admin";
  };
}

/**
 * Auth.js v5 configuration.
 *
 * The Go gateway owns the user table, so the panel only does session
 * management. The Credentials provider posts the email/password to
 * `POST /admin/v1/auth/login` and stores the returned bearer token in
 * the session. No DB adapter — the token is the only thing the panel
 * needs to make subsequent requests.
 */
export const { handlers, auth, signIn, signOut } = NextAuth({
  session: { strategy: "jwt" },
  pages: {
    signIn: "/sign-in",
  },
  providers: [
    Credentials({
      credentials: {
        email: { label: "Email", type: "email" },
        password: { label: "Password", type: "password" },
      },
      async authorize(credentials) {
        const email = credentials?.email;
        const password = credentials?.password;
        if (typeof email !== "string" || typeof password !== "string") {
          return null;
        }

        const url = `${getInternalGatewayUrl().replace(/\/$/, "")}/admin/v1/auth/login`;
        const res = await fetch(url, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ email, password }),
          // Auth.js runs server-side; this is a back-end call.
          cache: "no-store",
        });

        if (!res.ok) {
          return null;
        }

        const data = (await res.json()) as AuthApiResponse;
        if (!data?.token || !data.user) {
          return null;
        }

        return {
          id: data.user.id,
          email: data.user.email,
          name: data.user.name ?? data.user.email,
          tenantId: data.user.tenant_id,
          role: data.user.role,
          accessToken: data.token,
        };
      },
    }),
  ],
  callbacks: {
    async jwt({ token, user }) {
      if (user) {
        token.accessToken = user.accessToken;
        token.tenantId = user.tenantId;
        token.role = user.role;
      }
      return token;
    },
    async session({ session, token }) {
      if (token.accessToken) {
        session.accessToken = token.accessToken;
      }
      session.user.tenantId = token.tenantId;
      session.user.role = token.role;
      return session;
    },
  },
});
