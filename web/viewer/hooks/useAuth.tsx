"use client";

import {
  ReactNode,
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState
} from "react";

type AuthUser = {
  id: string;
  displayName: string;
  email: string;
  roles: string[];
};

type AuthContextValue = {
  user?: AuthUser;
  loading: boolean;
  error?: string;
  signIn: (redirectTo?: string) => Promise<void>;
  signOut: () => Promise<void>;
};

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
const AuthContext = createContext<AuthContextValue | undefined>(undefined);

type ViewerAuthResponse = {
  user?: {
    id: string;
    displayName: string;
    email?: string;
    roles?: string[];
  };
  loginUrl?: string;
  logoutUrl?: string;
};

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const target = path.startsWith("http") ? path : `${API_BASE}${path}`;
  const response = await fetch(target, {
    ...init,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {})
    }
  });
  if (!response.ok) {
    const detail = await response.text();
    const error = new Error(detail || `${response.status}`) as Error & { status?: number };
    error.status = response.status;
    throw error;
  }
  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | undefined>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | undefined>();
  const [loginUrl, setLoginUrl] = useState<string | undefined>();
  const [logoutUrl, setLogoutUrl] = useState<string | undefined>();

  const loadViewer = useCallback(async () => {
    try {
      setLoading(true);
      setError(undefined);
      const data = await request<ViewerAuthResponse>("/api/viewer/me");
      setLoginUrl(data.loginUrl);
      setLogoutUrl(data.logoutUrl);
      if (data.user) {
        setUser({
          id: data.user.id,
          displayName: data.user.displayName,
          email: data.user.email ?? "",
          roles: data.user.roles ?? []
        });
      } else {
        setUser(undefined);
      }
    } catch (err) {
      setUser(undefined);
      setLoginUrl(undefined);
      setLogoutUrl(undefined);
      const status = err instanceof Error && "status" in err ? (err as { status?: number }).status : undefined;
      if (status !== 401 && status !== 403) {
        setError(err instanceof Error ? err.message : "Unable to load viewer");
      } else {
        setError(undefined);
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadViewer();
  }, [loadViewer]);

  const buildRedirectTarget = useCallback((redirectTo?: string) => {
    if (redirectTo) {
      return redirectTo;
    }
    if (typeof window === "undefined") {
      return "/";
    }
    return `${window.location.pathname}${window.location.search}${window.location.hash}`;
  }, []);

  const signIn = useCallback(
    async (redirectTo?: string) => {
      const target = buildRedirectTarget(redirectTo);
      const destination = (() => {
        if (loginUrl) {
          return loginUrl;
        }
        if (API_BASE) {
          return `${API_BASE}/login`;
        }
        return "/login";
      })();

      if (typeof window === "undefined") {
        return;
      }

      const url = new URL(destination, window.location.origin);
      if (!url.searchParams.has("redirect")) {
        url.searchParams.set("redirect", target);
      }
      window.location.href = url.toString();
    },
    [buildRedirectTarget, loginUrl]
  );

  const signOut = useCallback(async () => {
    try {
      setError(undefined);
      const path = logoutUrl ?? "/api/viewer/me";
      await request<void>(path, { method: "DELETE" });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to sign out");
    } finally {
      setUser(undefined);
      setLoading(false);
      void loadViewer();
    }
  }, [loadViewer, logoutUrl]);

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      loading,
      error,
      signIn,
      signOut
    }),
    [user, loading, error, signIn, signOut]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return ctx;
}
