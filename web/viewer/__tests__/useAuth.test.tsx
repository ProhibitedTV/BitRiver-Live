import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AuthProvider, useAuth } from "../hooks/useAuth";

const mockRouterRefresh = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({
    refresh: mockRouterRefresh,
  }),
}));

function AuthHarness() {
  const {
    allowSelfSignup,
    authDialogOpen,
    authFeedback,
    authMode,
    authRedirectTo,
    error,
    loading,
    mfaRequired,
    signIn,
    signOut,
    signUp,
    submitMFAVerification,
    submitSignIn,
    submitSignUp,
    user,
  } = useAuth();

  return (
    <div>
      <p data-testid="auth-user">{user?.displayName ?? "anonymous"}</p>
      <p data-testid="auth-loading">{loading ? "loading" : "idle"}</p>
      <p data-testid="auth-error">{error ?? "none"}</p>
      <p data-testid="auth-open">{authDialogOpen ? "open" : "closed"}</p>
      <p data-testid="auth-mode">{authMode}</p>
      <p data-testid="auth-redirect">{authRedirectTo}</p>
      <p data-testid="auth-feedback">{authFeedback?.message ?? "none"}</p>
      <p data-testid="auth-allow-signup">{allowSelfSignup ? "enabled" : "disabled"}</p>
      <p data-testid="auth-mfa">{mfaRequired ? "required" : "not-required"}</p>
      <button type="button" onClick={() => void signIn()}>
        Open sign in
      </button>
      <button type="button" onClick={() => void signUp()}>
        Open sign up
      </button>
      <button
        type="button"
        onClick={() =>
          void submitSignIn({
            email: "viewer@example.com",
            password: "supersecret",
          })
        }
      >
        Submit sign in
      </button>
      <button
        type="button"
        onClick={() =>
          void submitSignUp({
            displayName: "Viewer",
            email: "viewer@example.com",
            password: "supersecret",
          })
        }
      >
        Submit sign up
      </button>
      <button type="button" onClick={() => void submitMFAVerification("123456")}>
        Submit MFA
      </button>
      <button type="button" onClick={() => void signOut()}>
        Sign out
      </button>
    </div>
  );
}

function RapidDoubleSignOutTrigger() {
  const { signOut } = useAuth();

  return (
    <button
      type="button"
      onClick={() => {
        void signOut();
        void signOut();
      }}
    >
      Sign out twice fast
    </button>
  );
}

type MockResponse = {
  ok: boolean;
  status: number;
  json?: unknown;
  text?: string;
};

const mockJsonResponse = (json: unknown, status = 200): MockResponse => ({ ok: true, status, json });
const mockTextError = (status: number, text: string): MockResponse => ({ ok: false, status, text });

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

const overrideWindowLocation = (
  overrides: Partial<Pick<Location, "hash" | "href" | "origin" | "pathname" | "search">>,
) => {
  const originalLocation = window.location;
  const mockLocation = {
    ancestorOrigins: originalLocation.ancestorOrigins,
    assign: jest.fn(),
    hash: "",
    host: "localhost",
    hostname: "localhost",
    href: "http://localhost/",
    origin: "http://localhost",
    pathname: "/",
    port: "",
    protocol: "http:",
    reload: jest.fn(),
    replace: jest.fn(),
    search: "",
    toString: () => "http://localhost/",
    ...overrides,
  } as unknown as Location & { href: string };
  Object.defineProperty(window, "location", {
    configurable: true,
    value: mockLocation,
  });
  return {
    mockLocation,
    restore: () => Object.defineProperty(window, "location", { configurable: true, value: originalLocation }),
  };
};

describe("useAuth", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    mockRouterRefresh.mockReset();
    window.history.replaceState({}, "", "/viewer");
  });

  afterEach(() => {
    global.fetch = originalFetch;
    jest.clearAllMocks();
    window.history.replaceState({}, "", "/viewer");
  });

  test("opens the in-viewer auth dialog and preserves the current route as redirect target when loginUrl is unavailable", async () => {
    global.fetch = jest.fn(async () => ({
      ok: true,
      status: 200,
      json: async () => ({ allowSelfSignup: true }),
      text: async () => "",
    })) as jest.MockedFunction<typeof fetch>;

    window.history.replaceState({}, "", "/viewer?tab=discover#live");
    const user = userEvent.setup();

    render(
      <AuthProvider>
        <AuthHarness />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("auth-loading")).toHaveTextContent("idle");
    });

    await act(async () => {
      await user.click(screen.getByRole("button", { name: /open sign in/i }));
    });

    expect(screen.getByTestId("auth-open")).toHaveTextContent("open");
    expect(screen.getByTestId("auth-mode")).toHaveTextContent("signin");
    expect(screen.getByTestId("auth-redirect")).toHaveTextContent("/viewer?tab=discover#live");

    const search = new URLSearchParams(window.location.search);
    expect(search.get("auth")).toBe("signin");
    expect(search.get("next")).toBe("/viewer?tab=discover#live");
  });

  test("redirects to the configured loginUrl when the runtime provides one", async () => {
    global.fetch = jest.fn(async () => ({
      ok: true,
      status: 200,
      json: async () => ({
        allowSelfSignup: true,
        loginUrl: "https://auth.example.com/login",
      }),
      text: async () => "",
    })) as jest.MockedFunction<typeof fetch>;

    const { mockLocation, restore } = overrideWindowLocation({
      pathname: "/channels/alpha",
      search: "?view=live",
      hash: "#info",
    });
    const user = userEvent.setup();

    render(
      <AuthProvider>
        <AuthHarness />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("auth-loading")).toHaveTextContent("idle");
    });

    await act(async () => {
      await user.click(screen.getByRole("button", { name: /open sign in/i }));
    });

    expect(mockLocation.href).toBe("https://auth.example.com/login?redirect=%2Fchannels%2Falpha%3Fview%3Dlive%23info");
    restore();
  });

  test("coerces sign-up requests back to sign-in when self-signup is disabled", async () => {
    global.fetch = jest.fn(async () => ({
      ok: true,
      status: 200,
      json: async () => ({ allowSelfSignup: false }),
      text: async () => "",
    })) as jest.MockedFunction<typeof fetch>;

    const user = userEvent.setup();
    render(
      <AuthProvider>
        <AuthHarness />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("auth-allow-signup")).toHaveTextContent("disabled");
    });

    await act(async () => {
      await user.click(screen.getByRole("button", { name: /open sign up/i }));
    });

    expect(screen.getByTestId("auth-open")).toHaveTextContent("open");
    expect(screen.getByTestId("auth-mode")).toHaveTextContent("signin");
    expect(screen.getByTestId("auth-feedback")).toHaveTextContent("Public self-signup is disabled");
  });

  test("enters MFA mode when the login endpoint requires verification", async () => {
    const responses = [
      {
        ok: true,
        status: 200,
        json: async () => ({ allowSelfSignup: true }),
        text: async () => "",
      },
      {
        ok: true,
        status: 200,
        json: async () => ({ mfaRequired: true, mfaToken: "mfa-token" }),
        text: async () => "",
      },
    ];

    global.fetch = jest.fn(async () => {
      const next = responses.shift();
      if (!next) {
        throw new Error("Unexpected fetch call");
      }
      return next as Response;
    }) as jest.MockedFunction<typeof fetch>;

    const user = userEvent.setup();
    render(
      <AuthProvider>
        <AuthHarness />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("auth-loading")).toHaveTextContent("idle");
    });

    await act(async () => {
      await user.click(screen.getByRole("button", { name: /open sign in/i }));
      await user.click(screen.getByRole("button", { name: /submit sign in/i }));
    });

    await waitFor(() => {
      expect(screen.getByTestId("auth-mfa")).toHaveTextContent("required");
      expect(screen.getByTestId("auth-feedback")).toHaveTextContent("Multi-factor verification is required");
    });

    const search = new URLSearchParams(window.location.search);
    expect(search.get("mfa")).toBe("verify");
  });

  test("successful same-page sign-in refreshes route data after the viewer session loads", async () => {
    const responses = [
      {
        ok: true,
        status: 200,
        json: async () => ({ allowSelfSignup: true }),
        text: async () => "",
      },
      {
        ok: true,
        status: 200,
        json: async () => ({ user: { id: "viewer-1", displayName: "Viewer" } }),
        text: async () => "",
      },
      {
        ok: true,
        status: 200,
        json: async () => ({
          allowSelfSignup: true,
          user: {
            id: "viewer-1",
            displayName: "Viewer",
            email: "viewer@example.com",
            roles: ["viewer"],
          },
        }),
        text: async () => "",
      },
    ];

    global.fetch = jest.fn(async () => {
      const next = responses.shift();
      if (!next) {
        throw new Error("Unexpected fetch call");
      }
      return next as Response;
    }) as jest.MockedFunction<typeof fetch>;

    const user = userEvent.setup();
    render(
      <AuthProvider>
        <AuthHarness />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("auth-loading")).toHaveTextContent("idle");
    });

    await act(async () => {
      await user.click(screen.getByRole("button", { name: /open sign in/i }));
      await user.click(screen.getByRole("button", { name: /submit sign in/i }));
    });

    await waitFor(() => {
      expect(screen.getByTestId("auth-user")).toHaveTextContent("Viewer");
      expect(screen.getByTestId("auth-open")).toHaveTextContent("closed");
      expect(screen.getByTestId("auth-error")).toHaveTextContent("none");
    });
    expect(mockRouterRefresh).toHaveBeenCalledTimes(1);
    expect(global.fetch).toHaveBeenNthCalledWith(
      3,
      "/api/viewer/me",
      expect.objectContaining({ credentials: "include", cache: "no-store" }),
    );
  });

  test("successful sign-up refreshes the viewer session and closes the dialog in place", async () => {
    const responses = [
      {
        ok: true,
        status: 200,
        json: async () => ({ allowSelfSignup: true }),
        text: async () => "",
      },
      {
        ok: true,
        status: 201,
        json: async () => ({ user: { id: "viewer-1", displayName: "Viewer" } }),
        text: async () => "",
      },
      {
        ok: true,
        status: 200,
        json: async () => ({
          allowSelfSignup: true,
          user: {
            id: "viewer-1",
            displayName: "Viewer",
            email: "viewer@example.com",
            roles: [],
          },
        }),
        text: async () => "",
      },
    ];

    global.fetch = jest.fn(async () => {
      const next = responses.shift();
      if (!next) {
        throw new Error("Unexpected fetch call");
      }
      return next as Response;
    }) as jest.MockedFunction<typeof fetch>;

    const user = userEvent.setup();
    render(
      <AuthProvider>
        <AuthHarness />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("auth-loading")).toHaveTextContent("idle");
    });

    await act(async () => {
      await user.click(screen.getByRole("button", { name: /open sign up/i }));
      await user.click(screen.getByRole("button", { name: /submit sign up/i }));
    });

    await waitFor(() => {
      expect(screen.getByTestId("auth-user")).toHaveTextContent("Viewer");
      expect(screen.getByTestId("auth-open")).toHaveTextContent("closed");
      expect(screen.getByTestId("auth-error")).toHaveTextContent("none");
    });
    expect(mockRouterRefresh).toHaveBeenCalledTimes(1);

    const search = new URLSearchParams(window.location.search);
    expect(search.get("auth")).toBeNull();
    expect(search.get("next")).toBeNull();
  });

  test("signOut signs the viewer out via /api/viewer/me and clears user after refresh", async () => {
    const responses: MockResponse[] = [
      mockJsonResponse({
        allowSelfSignup: true,
        user: { id: "viewer-1", displayName: "Viewer", email: "viewer@example.com", roles: [] },
      }),
      { ok: true, status: 204 },
      mockJsonResponse({ allowSelfSignup: true }),
    ];

    global.fetch = jest.fn(async () => {
      const next = responses.shift();
      if (!next) {
        throw new Error("Unexpected fetch call");
      }
      return {
        ok: next.ok,
        status: next.status,
        json: async () => next.json,
        text: async () => next.text ?? "",
      } as Response;
    }) as jest.MockedFunction<typeof fetch>;

    const user = userEvent.setup();
    render(
      <AuthProvider>
        <AuthHarness />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("auth-user")).toHaveTextContent("Viewer");
      expect(screen.getByTestId("auth-loading")).toHaveTextContent("idle");
    });

    await act(async () => {
      await user.click(screen.getByRole("button", { name: /^sign out$/i }));
    });

    await waitFor(() => {
      expect(screen.getByTestId("auth-user")).toHaveTextContent("anonymous");
      expect(screen.getByTestId("auth-error")).toHaveTextContent("none");
      expect(screen.getByTestId("auth-loading")).toHaveTextContent("idle");
    });
    expect(mockRouterRefresh).toHaveBeenCalledTimes(1);
  });

  test("signOut preserves failure message while loadViewer remains source of truth", async () => {
    const responses: MockResponse[] = [
      mockJsonResponse({
        allowSelfSignup: true,
        user: { id: "viewer-1", displayName: "Viewer", email: "viewer@example.com", roles: [] },
      }),
      mockTextError(500, "sign out failed"),
      mockJsonResponse({
        allowSelfSignup: true,
        user: { id: "viewer-1", displayName: "Viewer", email: "viewer@example.com", roles: [] },
      }),
    ];

    global.fetch = jest.fn(async () => {
      const next = responses.shift();
      if (!next) {
        throw new Error("Unexpected fetch call");
      }
      return {
        ok: next.ok,
        status: next.status,
        json: async () => next.json,
        text: async () => next.text ?? "",
      } as Response;
    }) as jest.MockedFunction<typeof fetch>;

    const user = userEvent.setup();
    render(
      <AuthProvider>
        <AuthHarness />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("auth-user")).toHaveTextContent("Viewer");
    });

    await act(async () => {
      await user.click(screen.getByRole("button", { name: /^sign out$/i }));
    });

    await waitFor(() => {
      expect(screen.getByTestId("auth-user")).toHaveTextContent("Viewer");
      expect(screen.getByTestId("auth-error")).toHaveTextContent("sign out failed");
      expect(screen.getByTestId("auth-loading")).toHaveTextContent("idle");
    });
    expect(mockRouterRefresh).toHaveBeenCalledTimes(1);
  });

  test("signOut keeps loading true until loadViewer refresh settles", async () => {
    const delayedViewer = deferred<Response>();

    global.fetch = jest.fn(async (_input, init) => {
      if ((init as RequestInit | undefined)?.method === "DELETE") {
        return {
          ok: true,
          status: 204,
          json: async () => ({}),
          text: async () => "",
        } as Response;
      }

      if ((global.fetch as jest.MockedFunction<typeof fetch>).mock.calls.length === 1) {
        return {
          ok: true,
          status: 200,
          json: async () => ({
            allowSelfSignup: true,
            user: { id: "viewer-1", displayName: "Viewer", email: "viewer@example.com", roles: [] },
          }),
          text: async () => "",
        } as Response;
      }

      return delayedViewer.promise;
    }) as jest.MockedFunction<typeof fetch>;

    const user = userEvent.setup();
    render(
      <AuthProvider>
        <AuthHarness />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("auth-user")).toHaveTextContent("Viewer");
      expect(screen.getByTestId("auth-loading")).toHaveTextContent("idle");
    });

    await act(async () => {
      await user.click(screen.getByRole("button", { name: /^sign out$/i }));
    });

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledTimes(3);
    });
    expect(screen.getByTestId("auth-user")).toHaveTextContent("Viewer");
    expect(screen.getByTestId("auth-loading")).toHaveTextContent("loading");

    delayedViewer.resolve({
      ok: true,
      status: 200,
      json: async () => ({ allowSelfSignup: true }),
      text: async () => "",
    } as Response);

    await waitFor(() => {
      expect(screen.getByTestId("auth-user")).toHaveTextContent("anonymous");
      expect(screen.getByTestId("auth-loading")).toHaveTextContent("idle");
    });
    expect(mockRouterRefresh).toHaveBeenCalledTimes(1);
  });

  test("enters MFA mode when the login endpoint requires verification after initial load", async () => {
    const responses = [
      {
        ok: true,
        status: 200,
        json: async () => ({ allowSelfSignup: true }),
        text: async () => "",
      },
      {
        ok: true,
        status: 200,
        json: async () => ({ mfaRequired: true, mfaToken: "mfa-token" }),
        text: async () => "",
      },
    ];

    global.fetch = jest.fn(async () => {
      const next = responses.shift();
      if (!next) {
        throw new Error("Unexpected fetch call");
      }
      return next as Response;
    }) as jest.MockedFunction<typeof fetch>;

    const user = userEvent.setup();
    render(
      <AuthProvider>
        <AuthHarness />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("auth-loading")).toHaveTextContent("idle");
    });

    await act(async () => {
      await user.click(screen.getByRole("button", { name: /open sign in/i }));
      await user.click(screen.getByRole("button", { name: /submit sign in/i }));
    });

    await waitFor(() => {
      expect(screen.getByTestId("auth-mfa")).toHaveTextContent("required");
      expect(screen.getByTestId("auth-feedback")).toHaveTextContent("Multi-factor verification is required");
    });
  });

  test("rapid double signOut settles to final refresh outcome without unstable visible state", async () => {
    const initialViewer = {
      allowSelfSignup: true,
      user: { id: "viewer-1", displayName: "Viewer", email: "viewer@example.com", roles: [] },
    };
    const finalViewer = {
      allowSelfSignup: true,
      user: { id: "viewer-2", displayName: "Viewer Final", email: "final@example.com", roles: ["viewer"] },
    };
    const firstRefresh = deferred<Response>();
    const secondRefresh = deferred<Response>();
    const calls: Array<{ url: string; method: string }> = [];

    let meGetCount = 0;
    global.fetch = jest.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method ?? "GET";
      calls.push({ url, method });

      if (url !== "/api/viewer/me") {
        throw new Error(`Unexpected url: ${url}`);
      }

      if (method === "DELETE") {
        return {
          ok: true,
          status: 204,
          json: async () => ({}),
          text: async () => "",
        } as Response;
      }

      meGetCount += 1;
      if (meGetCount === 1) {
        return {
          ok: true,
          status: 200,
          json: async () => initialViewer,
          text: async () => "",
        } as Response;
      }
      if (meGetCount === 2) {
        return firstRefresh.promise;
      }
      if (meGetCount === 3) {
        return secondRefresh.promise;
      }
      throw new Error("Unexpected viewer refresh");
    }) as jest.MockedFunction<typeof fetch>;

    const user = userEvent.setup();
    render(
      <AuthProvider>
        <AuthHarness />
        <RapidDoubleSignOutTrigger />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("auth-user")).toHaveTextContent("Viewer");
    });

    await act(async () => {
      await user.click(screen.getByRole("button", { name: /sign out twice fast/i }));
    });

    await waitFor(() => {
      expect(calls.filter((call) => call.method === "DELETE")).toHaveLength(2);
      expect(meGetCount).toBe(3);
    });

    await act(async () => {
      firstRefresh.resolve({
        ok: true,
        status: 200,
        json: async () => ({ allowSelfSignup: true }),
        text: async () => "",
      } as Response);
      secondRefresh.resolve({
        ok: true,
        status: 200,
        json: async () => finalViewer,
        text: async () => "",
      } as Response);
    });

    await waitFor(() => {
      expect(screen.getByTestId("auth-user")).toHaveTextContent("Viewer Final");
      expect(screen.getByTestId("auth-error")).toHaveTextContent("none");
      expect(screen.getByTestId("auth-loading")).toHaveTextContent("idle");
    });
  });
});
