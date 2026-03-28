import { screen } from "@testing-library/react";
import { AuthDialog } from "../components/auth/AuthDialog";
import { buildAuthState, mockUseAuth, renderWithProviders } from "../test/test-utils";

jest.mock("../hooks/useAuth");

describe("AuthDialog", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("shows a friendly return summary instead of the raw viewer route", () => {
    mockUseAuth.mockReturnValue(
      buildAuthState({
        user: undefined,
        authDialogOpen: true,
        authRedirectTo: "/viewer",
      }),
    );

    renderWithProviders(<AuthDialog />);

    expect(screen.getByText("Continue where you left off")).toBeInTheDocument();
    expect(screen.getByText("Viewer home")).toBeInTheDocument();
    expect(screen.getByText("We'll bring you back to the main viewer page once you're signed in.")).toBeInTheDocument();
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
});
