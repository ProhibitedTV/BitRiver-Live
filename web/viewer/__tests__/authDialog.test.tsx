import { screen } from "@testing-library/react";
import { AuthDialog } from "../components/auth/AuthDialog";
import { buildAuthState, mockUseAuth, renderWithProviders } from "../test/test-utils";

jest.mock("../hooks/useAuth");

describe("AuthDialog", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("does not render the return-summary box in the sign-in dialog", () => {
    mockUseAuth.mockReturnValue(
      buildAuthState({
        user: undefined,
        authDialogOpen: true,
        authRedirectTo: "/viewer",
      }),
    );

    renderWithProviders(<AuthDialog />);

    expect(screen.queryByText("Continue where you left off")).not.toBeInTheDocument();
    expect(screen.queryByText("Viewer home")).not.toBeInTheDocument();
    expect(screen.queryByText("We'll bring you back to the main viewer page once you're signed in.")).not.toBeInTheDocument();
    expect(screen.queryByText("/viewer")).not.toBeInTheDocument();
  });

  it("renders only one sign-in action when self-signup is unavailable", () => {
    mockUseAuth.mockReturnValue(
      buildAuthState({
        user: undefined,
        authDialogOpen: true,
        allowSelfSignup: false,
      }),
    );

    renderWithProviders(<AuthDialog />);

    expect(screen.getAllByRole("button", { name: /^sign in$/i })).toHaveLength(1);
    expect(screen.queryByRole("tablist", { name: /auth mode/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /need an account/i })).not.toBeInTheDocument();
  });

  it("keeps the sign-in form free of reassurance copy", () => {
    mockUseAuth.mockReturnValue(
      buildAuthState({
        user: undefined,
        authDialogOpen: true,
        authMode: "signin",
      }),
    );

    renderWithProviders(<AuthDialog />);

    expect(screen.queryByText("Sign in without losing your place")).not.toBeInTheDocument();
    expect(screen.queryByText("Your stream, category, and browse context stay right here while you authenticate.")).not.toBeInTheDocument();
    expect(screen.getByLabelText("Email")).toBeInTheDocument();
    expect(screen.getByLabelText("Password")).toBeInTheDocument();
  });
});
