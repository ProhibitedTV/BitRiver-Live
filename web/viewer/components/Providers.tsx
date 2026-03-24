"use client";

import { ReactNode } from "react";
import { AuthDialog } from "./auth/AuthDialog";
import { AuthProvider } from "../hooks/useAuth";

export function Providers({ children }: { children: ReactNode }) {
  return (
    <AuthProvider>
      {children}
      <AuthDialog />
    </AuthProvider>
  );
}
