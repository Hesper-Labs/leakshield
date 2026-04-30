import { redirect } from "next/navigation";

import { auth } from "@/auth";

export default async function RootIndex() {
  const session = await auth();
  if (session?.user) {
    redirect("/dashboard");
  }
  redirect("/sign-in");
}
