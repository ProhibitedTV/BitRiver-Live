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
  login: (email: string, password: string) => Promise<boolean>;
  signup: (displayName: string, email: string, password: string) => Promise<boolean>;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
};

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
const AuthContext = createContext<AuthContextValue | undefined>(undefined);

type AuthResponse = {
  user: {
    id: string;
    displayName: string;
    email: string;
    roles: string[];
  };
};

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {})
    }
  });
  if (!response.ok) {
    const detail = await response.text();
    throw new Error(detail || `${response.status}`);
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

  const loadSession = useCallback(async () => {
    try {
      setLoading(true);
      setError(undefined);
      const data = await request<AuthResponse>("/api/auth/session");
      setUser({
        id: data.user.id,
        displayName: data.user.displayName,
        email: data.user.email,
        roles: data.user.roles
      });
    } catch (err) {
      setUser(undefined);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadSession();
  }, [loadSession]);

  const login = useCallback(async (email: string, password: string) => {
    try {
      setError(undefined);
      const data = await request<AuthResponse>("/api/auth/login", {
        method: "POST",
        body: JSON.stringify({ email, password })
      });
      setUser({
        id: data.user.id,
        displayName: data.user.displayName,
        email: data.user.email,
        roles: data.user.roles
      });
      return true;
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
      return false;
    }
  }, []);

  const signup = useCallback(
    async (displayName: string, email: string, password: string) => {
      try {
        setError(undefined);
        const data = await request<AuthResponse>("/api/auth/signup", {
          method: "POST",
          body: JSON.stringify({ displayName, email, password })
        });
        setUser({
          id: data.user.id,
          displayName: data.user.displayName,
          email: data.user.email,
          roles: data.user.roles
        });
        return true;
      } catch (err) {
        setError(err instanceof Error ? err.message : "Signup failed");
        return false;
      }
    },
    []
  );

  const logout = useCallback(async () => {
    try {
      await request<void>("/api/auth/session", { method: "DELETE" });
    } finally {
      setUser(undefined);
    }
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      loading,
      error,
      login,
      signup,
      logout,
      refresh: loadSession
    }),
    [user, loading, error, login, signup, logout, loadSession]
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
