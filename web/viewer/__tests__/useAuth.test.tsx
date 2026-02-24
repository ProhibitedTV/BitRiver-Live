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
});
