"use client";

import type { MouseEvent, ReactNode } from "react";
import { useAuth } from "../../hooks/useAuth";

type AuthActionLinkProps = {
  children: ReactNode;
  className?: string;
  mode: "signin" | "signup";
  fallbackHref?: string;
  redirectTo?: string;
};

export function AuthActionLink({
  children,
  className,
  mode,
  fallbackHref = mode === "signup" ? "/?auth=signup" : "/?auth=signin",
  redirectTo,
}: AuthActionLinkProps) {
  const { signIn, signUp } = useAuth();

  const handleClick = async (event: MouseEvent<HTMLAnchorElement>) => {
    event.preventDefault();
    if (mode === "signup") {
      await signUp(redirectTo);
      return;
    }
    await signIn(redirectTo);
  };

  return (
    <a href={fallbackHref} className={className} onClick={handleClick}>
      {children}
    </a>
  );
}
