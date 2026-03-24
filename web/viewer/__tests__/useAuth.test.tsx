import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AuthProvider, useAuth } from "../hooks/useAuth";

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

describe("useAuth", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    window.history.replaceState({}, "", "/viewer");
  });

  afterEach(() => {
    global.fetch = originalFetch;
    jest.clearAllMocks();
  });

  test("opens the in-viewer auth dialog and preserves the current route as redirect target", async () => {
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
  });

  test("signOut signs the viewer out via /api/viewer/me and clears user after refresh", async () => {
    const responses = [
      {
        ok: true,
        status: 200,
        json: async () => ({
          allowSelfSignup: true,
          user: { id: "viewer-1", displayName: "Viewer", email: "viewer@example.com", roles: [] },
        }),
        text: async () => "",
      },
      {
        ok: true,
        status: 204,
        json: async () => undefined,
        text: async () => "",
      },
      {
        ok: true,
        status: 200,
        json: async () => ({ allowSelfSignup: true }),
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
      expect(screen.getByTestId("auth-user")).toHaveTextContent("Viewer");
    });

    await act(async () => {
      await user.click(screen.getByRole("button", { name: /^sign out$/i }));
    });

    await waitFor(() => {
      expect(screen.getByTestId("auth-user")).toHaveTextContent("anonymous");
      expect(screen.getByTestId("auth-error")).toHaveTextContent("none");
      expect(screen.getByTestId("auth-loading")).toHaveTextContent("idle");
    });
  });
});
