"use client";

import { ReactNode } from "react";
import { AuthProvider } from "../hooks/useAuth";

export function Providers({ children }: { children: ReactNode }) {
  return <AuthProvider>{children}</AuthProvider>;
}
