import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AuthProvider, useAuth } from "../hooks/useAuth";

function AuthHarness() {
  const { user, loading, error, signOut } = useAuth();

  return (
    <div>
      <p data-testid="auth-user">{user?.displayName ?? "anonymous"}</p>
      <p data-testid="auth-loading">{loading ? "loading" : "idle"}</p>
      <p data-testid="auth-error">{error ?? "none"}</p>
      <button type="button" onClick={() => void signOut()} disabled={loading}>
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

function SignInTrigger({ redirectTo }: { redirectTo?: string }) {
  const { signIn } = useAuth();

  return (
    <button type="button" onClick={() => void signIn(redirectTo)}>
      Sign in
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

describe("useAuth", () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    global.fetch = originalFetch;
    jest.clearAllMocks();
  });

  test("signIn falls back to the signup surface when loginUrl is unavailable", async () => {
    const originalLocation = window.location;
    const mockLocation = {
      ...originalLocation,
      href: "http://localhost/viewer",
      origin: "http://localhost",
      pathname: "/viewer",
      search: "?tab=discover",
      hash: "#live",
    } as Location & { href: string };
    Object.defineProperty(window, "location", {
      configurable: true,
      value: mockLocation,
    });

    global.fetch = jest.fn(async () => ({
      ok: false,
      status: 401,
      json: async () => ({}),
      text: async () => "unauthorized",
    })) as jest.MockedFunction<typeof fetch>;

    const user = userEvent.setup();
    render(
      <AuthProvider>
        <SignInTrigger />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/viewer/me",
        expect.objectContaining({ credentials: "include" }),
      );
    });

    await act(async () => {
      await user.click(screen.getByRole("button", { name: /sign in/i }));
    });

    expect(mockLocation.href).toBe("http://localhost/signup?next=%2Fviewer%3Ftab%3Ddiscover%23live#login-form");

    Object.defineProperty(window, "location", {
      configurable: true,
      value: originalLocation,
    });
  });

  test("signOut signs the viewer out via loadViewer and clears user after refresh", async () => {
    const responses: MockResponse[] = [
      mockJsonResponse({ user: { id: "viewer-1", displayName: "Viewer", email: "viewer@example.com", roles: [] } }),
      { ok: true, status: 204 },
      mockTextError(401, "unauthorized"),
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
      </AuthProvider>
    );

    await waitFor(() => {
      expect(screen.getByTestId("auth-user")).toHaveTextContent("Viewer");
      expect(screen.getByTestId("auth-loading")).toHaveTextContent("idle");
    });

    await act(async () => {
      await user.click(screen.getByRole("button", { name: /sign out/i }));
    });

    await waitFor(() => {
      expect(screen.getByTestId("auth-user")).toHaveTextContent("anonymous");
      expect(screen.getByTestId("auth-error")).toHaveTextContent("none");
      expect(screen.getByTestId("auth-loading")).toHaveTextContent("idle");
    });
  });

  test("signOut preserves failure message while loadViewer remains source of truth", async () => {
    const responses: MockResponse[] = [
      mockJsonResponse({ user: { id: "viewer-1", displayName: "Viewer", email: "viewer@example.com", roles: [] } }),
      mockTextError(500, "sign out failed"),
      mockJsonResponse({ user: { id: "viewer-1", displayName: "Viewer", email: "viewer@example.com", roles: [] } }),
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
      </AuthProvider>
    );

    await waitFor(() => {
      expect(screen.getByTestId("auth-user")).toHaveTextContent("Viewer");
    });

    await act(async () => {
      await user.click(screen.getByRole("button", { name: /sign out/i }));
    });

    await waitFor(() => {
      expect(screen.getByTestId("auth-user")).toHaveTextContent("Viewer");
      expect(screen.getByTestId("auth-error")).toHaveTextContent("sign out failed");
      expect(screen.getByTestId("auth-loading")).toHaveTextContent("idle");
    });
  });

  test("signOut keeps loading true until loadViewer refresh settles", async () => {
    const viewer = { user: { id: "viewer-1", displayName: "Viewer", email: "viewer@example.com", roles: [] } };
    const delayedViewer = deferred<Response>();

    global.fetch = jest
      .fn()
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => viewer,
        text: async () => "",
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        status: 204,
        json: async () => undefined,
        text: async () => "",
      } as Response)
      .mockImplementationOnce(() => delayedViewer.promise);

    const user = userEvent.setup();
    render(
      <AuthProvider>
        <AuthHarness />
      </AuthProvider>
    );

    await waitFor(() => {
      expect(screen.getByTestId("auth-user")).toHaveTextContent("Viewer");
      expect(screen.getByTestId("auth-loading")).toHaveTextContent("idle");
    });

    await act(async () => {
      await user.click(screen.getByRole("button", { name: /sign out/i }));
    });

    await waitFor(() => {
      expect(screen.getByTestId("auth-loading")).toHaveTextContent("loading");
    });

    await act(async () => {
      delayedViewer.resolve({
        ok: false,
        status: 401,
        json: async () => ({}),
        text: async () => "unauthorized",
      } as Response);
    });

    await waitFor(() => {
      expect(screen.getByTestId("auth-loading")).toHaveTextContent("idle");
      expect(screen.getByTestId("auth-user")).toHaveTextContent("anonymous");
    });
  });

  test("rapid double signOut settles to final refresh outcome without unstable visible state", async () => {
    const initialViewer = { user: { id: "viewer-1", displayName: "Viewer", email: "viewer@example.com", roles: [] } };
    const finalViewer = {
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
          json: async () => undefined,
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

      throw new Error("Unexpected /api/viewer/me GET call");
    }) as jest.MockedFunction<typeof fetch>;

    const user = userEvent.setup();
    render(
      <AuthProvider>
        <AuthHarness />
        <RapidDoubleSignOutTrigger />
      </AuthProvider>
    );

    await waitFor(() => {
      expect(screen.getByTestId("auth-user")).toHaveTextContent("Viewer");
      expect(screen.getByTestId("auth-loading")).toHaveTextContent("idle");
      expect(screen.getByTestId("auth-error")).toHaveTextContent("none");
    });

    await act(async () => {
      await user.click(screen.getByRole("button", { name: /sign out twice fast/i }));
    });

    await act(async () => {
      firstRefresh.resolve({
        ok: true,
        status: 200,
        json: async () => initialViewer,
        text: async () => "",
      } as Response);
    });

    await act(async () => {
      secondRefresh.resolve({
        ok: true,
        status: 200,
        json: async () => finalViewer,
        text: async () => "",
      } as Response);
    });

    await waitFor(() => {
      expect(screen.getByTestId("auth-loading")).toHaveTextContent("idle");
      expect(screen.getByTestId("auth-user")).toHaveTextContent("Viewer Final");
      expect(screen.getByTestId("auth-error")).toHaveTextContent("none");
    });

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledTimes(5);
    });

    expect(calls).toEqual([
      { url: "/api/viewer/me", method: "GET" },
      { url: "/api/viewer/me", method: "DELETE" },
      { url: "/api/viewer/me", method: "DELETE" },
      { url: "/api/viewer/me", method: "GET" },
      { url: "/api/viewer/me", method: "GET" },
    ]);
  });

});
