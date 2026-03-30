"use client";

import { ReactNode, createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { appendRedirectParam } from "../lib/auth-links";
import { ViewerApiError, viewerRequest } from "../lib/viewer-api-core";

type AuthUser = {
  id: string;
  displayName: string;
  email: string;
  roles: string[];
};

type RawAuthUser = {
  id: string;
  displayName: string;
  email?: string;
  roles?: string[];
};

type AuthMode = "signin" | "signup";
type MFAMode = "verify" | "enroll";

type MFAEnrollment = {
  secret: string;
  otpauthUrl: string;
  recoveryCodes: string[];
};

type AuthFeedback = {
  message: string;
  variant: "info" | "error";
};

type AuthContextValue = {
  user?: AuthUser;
  loading: boolean;
  error?: string;
  allowSelfSignup: boolean;
  authDialogOpen: boolean;
  authMode: AuthMode;
  authFeedback?: AuthFeedback;
  authRedirectTo: string;
  mfaRequired: boolean;
  mfaEnrollment?: MFAEnrollment;
  signIn: (redirectTo?: string) => Promise<void>;
  signUp: (redirectTo?: string) => Promise<void>;
  setAuthMode: (mode: AuthMode) => void;
  closeAuthDialog: () => void;
  clearAuthFeedback: () => void;
  submitSignIn: (input: { email: string; password: string }) => Promise<void>;
  submitSignUp: (input: { displayName: string; email: string; password: string }) => Promise<void>;
  submitMFAVerification: (code: string) => Promise<void>;
  signOut: () => Promise<void>;
};

type ViewerAuthResponse = {
  allowSelfSignup?: boolean;
  user?: RawAuthUser;
  loginUrl?: string;
  logoutUrl?: string;
};

type LoginResponse = {
  user?: RawAuthUser;
  mfaRequired?: boolean;
  mfaToken?: string;
  enrollment?: MFAEnrollment;
};

type ViewerAuthMeta = Pick<ViewerAuthResponse, "allowSelfSignup" | "loginUrl" | "logoutUrl">;

const AuthContext = createContext<AuthContextValue | undefined>(undefined);
const AUTH_STATE_QUERY_KEYS = ["auth", "next", "mfa"] as const;
const AUTH_URL_ORIGIN = "http://bitriver.local";
const SIGNUP_DISABLED_MESSAGE =
  "Public self-signup is disabled on this server. Sign in with an existing account or ask an administrator for access.";

function isSafeOnsitePath(candidate?: string) {
  if (!candidate || typeof window === "undefined") {
    return false;
  }

  try {
    const url = new URL(candidate, window.location.origin);
    return url.origin === window.location.origin;
  } catch {
    return false;
  }
}

function stripAuthStateFromPath(path: string, origin = AUTH_URL_ORIGIN) {
  const url = new URL(path, origin);
  AUTH_STATE_QUERY_KEYS.forEach((key) => {
    url.searchParams.delete(key);
  });
  const normalizedPath = `${url.pathname}${url.search}${url.hash}`;
  return normalizedPath || "/";
}

function buildCurrentViewerPath() {
  if (typeof window === "undefined") {
    return "/";
  }

  return stripAuthStateFromPath(
    `${window.location.pathname}${window.location.search}${window.location.hash}`,
    window.location.origin,
  );
}

function normalizeAuthMode(raw: string | null | undefined): AuthMode | undefined {
  switch (raw?.trim().toLowerCase()) {
    case "signup":
      return "signup";
    case "signin":
      return "signin";
    default:
      return undefined;
  }
}

function normalizeMFAMode(raw: string | null | undefined): MFAMode | undefined {
  switch (raw?.trim().toLowerCase()) {
    case "enroll":
      return "enroll";
    case "verify":
      return "verify";
    default:
      return undefined;
  }
}

function formatAuthUser(user?: RawAuthUser): AuthUser | undefined {
  if (!user) {
    return undefined;
  }

  return {
    id: user.id,
    displayName: user.displayName,
    email: user.email ?? "",
    roles: user.roles ?? [],
  };
}

function readViewerAuthMeta(body: unknown): ViewerAuthMeta {
  if (!body || typeof body !== "object") {
    return {};
  }

  const record = body as Record<string, unknown>;
  return {
    allowSelfSignup: typeof record.allowSelfSignup === "boolean" ? record.allowSelfSignup : undefined,
    loginUrl: typeof record.loginUrl === "string" ? record.loginUrl : undefined,
    logoutUrl: typeof record.logoutUrl === "string" ? record.logoutUrl : undefined,
  };
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | undefined>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | undefined>();
  const [allowSelfSignup, setAllowSelfSignup] = useState(true);
  const [loginUrl, setLoginUrl] = useState<string | undefined>();
  const [logoutUrl, setLogoutUrl] = useState("/api/viewer/me");
  const [authDialogOpen, setAuthDialogOpen] = useState(false);
  const [authMode, setAuthModeState] = useState<AuthMode>("signin");
  const [authFeedback, setAuthFeedback] = useState<AuthFeedback | undefined>();
  const [authRedirectTo, setAuthRedirectTo] = useState("/");
  const [mfaMode, setMFAMode] = useState<MFAMode | undefined>();
  const [mfaEnrollment, setMFAEnrollment] = useState<MFAEnrollment | undefined>();
  const [mfaEnrollmentRequested, setMFAEnrollmentRequested] = useState(false);
  const [pendingMFAToken, setPendingMFAToken] = useState<string | undefined>();

  const loadViewer = useCallback(async () => {
    try {
      setLoading(true);
      setError(undefined);
      const data = await viewerRequest<ViewerAuthResponse>("/api/viewer/me");
      setAllowSelfSignup(data.allowSelfSignup ?? true);
      setLoginUrl(data.loginUrl);
      setLogoutUrl(data.logoutUrl ?? "/api/viewer/me");
      setUser(formatAuthUser(data.user));
    } catch (err) {
      const status = err instanceof ViewerApiError ? err.status : undefined;
      const meta = err instanceof ViewerApiError ? readViewerAuthMeta(err.body) : {};

      setUser(undefined);
      setLoginUrl(meta.loginUrl);
      setLogoutUrl(meta.logoutUrl ?? "/api/viewer/me");
      if (meta.allowSelfSignup !== undefined) {
        setAllowSelfSignup(meta.allowSelfSignup);
      }

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

  const resolveRedirectTarget = useCallback((redirectTo?: string) => {
    const fallback = buildCurrentViewerPath();
    if (!redirectTo || !isSafeOnsitePath(redirectTo)) {
      return fallback;
    }
    return stripAuthStateFromPath(redirectTo, window.location.origin) || fallback;
  }, []);

  const syncAuthLocation = useCallback(
    (mode?: AuthMode, redirectTo?: string, nextMFAMode?: MFAMode) => {
      if (typeof window === "undefined") {
        return;
      }

      const url = new URL(window.location.href);
      AUTH_STATE_QUERY_KEYS.forEach((key) => {
        url.searchParams.delete(key);
      });

      if (mode) {
        url.searchParams.set("auth", mode);
        url.searchParams.set("next", resolveRedirectTarget(redirectTo));
      }
      if (nextMFAMode) {
        url.searchParams.set("mfa", nextMFAMode);
      }

      window.history.replaceState(window.history.state, "", `${url.pathname}${url.search}${url.hash}`);
    },
    [resolveRedirectTarget],
  );

  const resetAuthFlow = useCallback(
    (shouldSyncLocation = true) => {
      setAuthDialogOpen(false);
      setAuthFeedback(undefined);
      setMFAMode(undefined);
      setMFAEnrollment(undefined);
      setMFAEnrollmentRequested(false);
      setPendingMFAToken(undefined);
      if (shouldSyncLocation) {
        syncAuthLocation(undefined);
      }
    },
    [syncAuthLocation],
  );

  const openAuthFlow = useCallback(
    (
      requestedMode: AuthMode,
      redirectTo?: string,
      options?: { feedback?: AuthFeedback; nextMFAMode?: MFAMode; enrollment?: MFAEnrollment; token?: string },
    ) => {
      const resolvedRedirect = resolveRedirectTarget(redirectTo);
      const coercedMode = requestedMode === "signup" && !allowSelfSignup ? "signin" : requestedMode;

      setAuthDialogOpen(true);
      setAuthModeState(coercedMode);
      setAuthRedirectTo(resolvedRedirect);
      setAuthFeedback(
        options?.feedback ??
          (requestedMode === "signup" && !allowSelfSignup
            ? { message: SIGNUP_DISABLED_MESSAGE, variant: "error" }
            : undefined),
      );
      setMFAMode(options?.nextMFAMode);
      setMFAEnrollment(options?.enrollment);
      setMFAEnrollmentRequested(Boolean(options?.enrollment));
      setPendingMFAToken(options?.token);
      syncAuthLocation(coercedMode, resolvedRedirect, options?.nextMFAMode);
    },
    [allowSelfSignup, resolveRedirectTarget, syncAuthLocation],
  );

  const syncAuthStateFromLocation = useCallback(() => {
    if (typeof window === "undefined") {
      return;
    }

    const url = new URL(window.location.href);
    const requestedMode = normalizeAuthMode(url.searchParams.get("auth"));
    const requestedMFAMode = normalizeMFAMode(url.searchParams.get("mfa"));
    if (!requestedMode && !requestedMFAMode) {
      resetAuthFlow(false);
      setAuthRedirectTo(buildCurrentViewerPath());
      return;
    }

    const resolvedRedirect = resolveRedirectTarget(url.searchParams.get("next") ?? undefined);
    const coercedMode = requestedMode === "signup" && !allowSelfSignup ? "signin" : requestedMode ?? "signin";

    setAuthDialogOpen(true);
    setAuthModeState(coercedMode);
    setAuthRedirectTo(resolvedRedirect);
    setMFAMode(requestedMFAMode);
    setMFAEnrollment(undefined);
    setMFAEnrollmentRequested(false);
    setPendingMFAToken(undefined);

    if (requestedMode === "signup" && !allowSelfSignup) {
      setAuthFeedback({ message: SIGNUP_DISABLED_MESSAGE, variant: "error" });
      syncAuthLocation("signin", resolvedRedirect, requestedMFAMode);
      return;
    }

    setAuthFeedback(undefined);
  }, [allowSelfSignup, resetAuthFlow, resolveRedirectTarget, syncAuthLocation]);

  useEffect(() => {
    syncAuthStateFromLocation();
    window.addEventListener("popstate", syncAuthStateFromLocation);
    return () => {
      window.removeEventListener("popstate", syncAuthStateFromLocation);
    };
  }, [syncAuthStateFromLocation]);

  useEffect(() => {
    if (!authDialogOpen || mfaMode !== "enroll" || mfaEnrollmentRequested || mfaEnrollment) {
      return;
    }

    let cancelled = false;
    const loadEnrollment = async () => {
      try {
        setLoading(true);
        setMFAEnrollmentRequested(true);
        const enrollment = await viewerRequest<MFAEnrollment>("/api/auth/mfa/enroll", {
          method: "POST",
          body: JSON.stringify(pendingMFAToken ? { token: pendingMFAToken } : {}),
        });
        if (!cancelled) {
          setMFAEnrollment(enrollment);
        }
      } catch (err) {
        if (!cancelled) {
          setAuthFeedback({
            message: err instanceof Error ? err.message : "Unable to load MFA setup.",
            variant: "error",
          });
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    void loadEnrollment();
    return () => {
      cancelled = true;
    };
  }, [authDialogOpen, mfaEnrollment, mfaEnrollmentRequested, mfaMode, pendingMFAToken]);

  const completeAuthSuccess = useCallback(async () => {
    const resolvedRedirect = resolveRedirectTarget(authRedirectTo);
    const currentPath = buildCurrentViewerPath();

    await loadViewer();
    resetAuthFlow();

    if (typeof window === "undefined") {
      return;
    }

    if (resolvedRedirect !== currentPath) {
      window.location.assign(resolvedRedirect);
    }
  }, [authRedirectTo, loadViewer, resetAuthFlow, resolveRedirectTarget]);

  const signIn = useCallback(
    async (redirectTo?: string) => {
      const resolvedRedirect = resolveRedirectTarget(redirectTo);
      if (loginUrl && typeof window !== "undefined") {
        window.location.href = appendRedirectParam(loginUrl, window.location.origin, resolvedRedirect, "redirect");
        return;
      }

      openAuthFlow("signin", resolvedRedirect);
    },
    [loginUrl, openAuthFlow, resolveRedirectTarget],
  );

  const signUp = useCallback(
    async (redirectTo?: string) => {
      openAuthFlow("signup", redirectTo);
    },
    [openAuthFlow],
  );

  const setAuthMode = useCallback(
    (mode: AuthMode) => {
      const resolvedRedirect = resolveRedirectTarget(authRedirectTo);
      if (mode === "signup" && !allowSelfSignup) {
        setAuthModeState("signin");
        setAuthFeedback({ message: SIGNUP_DISABLED_MESSAGE, variant: "error" });
        setMFAMode(undefined);
        setMFAEnrollment(undefined);
        setMFAEnrollmentRequested(false);
        setPendingMFAToken(undefined);
        syncAuthLocation("signin", resolvedRedirect);
        return;
      }

      setAuthModeState(mode);
      setAuthFeedback(undefined);
      setMFAMode(undefined);
      setMFAEnrollment(undefined);
      setMFAEnrollmentRequested(false);
      setPendingMFAToken(undefined);
      syncAuthLocation(mode, resolvedRedirect);
    },
    [allowSelfSignup, authRedirectTo, resolveRedirectTarget, syncAuthLocation],
  );

  const closeAuthDialog = useCallback(() => {
    resetAuthFlow();
  }, [resetAuthFlow]);

  const clearAuthFeedback = useCallback(() => {
    setAuthFeedback(undefined);
  }, []);

  const submitSignIn = useCallback(
    async ({ email, password }: { email: string; password: string }) => {
      try {
        setLoading(true);
        setError(undefined);
        setAuthFeedback(undefined);
        const response = await viewerRequest<LoginResponse>("/api/auth/login", {
          method: "POST",
          body: JSON.stringify({ email, password }),
        });

        if (response.mfaRequired) {
          const nextMode: MFAMode = response.enrollment ? "enroll" : "verify";
          setMFAMode(nextMode);
          setPendingMFAToken(response.mfaToken);
          setMFAEnrollment(response.enrollment);
          setMFAEnrollmentRequested(Boolean(response.enrollment));
          setAuthFeedback({
            message: "Multi-factor verification is required to finish signing in.",
            variant: "info",
          });
          syncAuthLocation("signin", authRedirectTo, nextMode);
          return;
        }

        await completeAuthSuccess();
      } catch (err) {
        setAuthFeedback({
          message: err instanceof Error ? err.message : "Unable to sign in.",
          variant: "error",
        });
      } finally {
        setLoading(false);
      }
    },
    [authRedirectTo, completeAuthSuccess, syncAuthLocation],
  );

  const submitSignUp = useCallback(
    async ({ displayName, email, password }: { displayName: string; email: string; password: string }) => {
      try {
        setLoading(true);
        setError(undefined);
        setAuthFeedback(undefined);
        await viewerRequest<LoginResponse>("/api/auth/signup", {
          method: "POST",
          body: JSON.stringify({ displayName, email, password }),
        });
        await completeAuthSuccess();
      } catch (err) {
        setAuthFeedback({
          message: err instanceof Error ? err.message : "Unable to create account.",
          variant: "error",
        });
      } finally {
        setLoading(false);
      }
    },
    [completeAuthSuccess],
  );

  const submitMFAVerification = useCallback(
    async (code: string) => {
      try {
        setLoading(true);
        setError(undefined);
        setAuthFeedback(undefined);
        await viewerRequest<LoginResponse>("/api/auth/mfa/verify", {
          method: "POST",
          body: JSON.stringify(pendingMFAToken ? { code, token: pendingMFAToken } : { code }),
        });
        await completeAuthSuccess();
      } catch (err) {
        setAuthFeedback({
          message: err instanceof Error ? err.message : "Unable to verify MFA.",
          variant: "error",
        });
      } finally {
        setLoading(false);
      }
    },
    [completeAuthSuccess, pendingMFAToken],
  );

  const signOut = useCallback(async () => {
    let signOutError: string | undefined;
    setLoading(true);
    try {
      setError(undefined);
      await viewerRequest<void>(logoutUrl || "/api/viewer/me", { method: "DELETE" });
    } catch (err) {
      signOutError = err instanceof Error ? err.message : "Unable to sign out";
    } finally {
      await loadViewer();
      if (signOutError) {
        setError(signOutError);
      }
    }
  }, [loadViewer, logoutUrl]);

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      loading,
      error,
      allowSelfSignup,
      authDialogOpen,
      authMode,
      authFeedback,
      authRedirectTo,
      mfaRequired: Boolean(mfaMode),
      mfaEnrollment,
      signIn,
      signUp,
      setAuthMode,
      closeAuthDialog,
      clearAuthFeedback,
      submitSignIn,
      submitSignUp,
      submitMFAVerification,
      signOut,
    }),
    [
      allowSelfSignup,
      authDialogOpen,
      authFeedback,
      authMode,
      authRedirectTo,
      clearAuthFeedback,
      closeAuthDialog,
      error,
      loading,
      mfaEnrollment,
      mfaMode,
      setAuthMode,
      signIn,
      signOut,
      signUp,
      submitMFAVerification,
      submitSignIn,
      submitSignUp,
      user,
    ],
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
