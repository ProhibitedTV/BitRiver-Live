import { act, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AuthDialog } from "../components/auth/AuthDialog";
import { buildAuthState, mockUseAuth, renderWithProviders } from "../test/test-utils";

jest.mock("../hooks/useAuth");

describe("AuthDialog", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("does not render when the auth dialog is closed", () => {
    mockUseAuth.mockReturnValue(
      buildAuthState({
        user: undefined,
        authDialogOpen: false,
      }),
    );

    renderWithProviders(<AuthDialog />);

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
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
    expect(screen.queryByRole("button", { name: /need an account/i })).not.toBeInTheDocument();
  });

  it("keeps the sign-in form free of the old return-summary copy", () => {
    mockUseAuth.mockReturnValue(
      buildAuthState({
        user: undefined,
        authDialogOpen: true,
        authMode: "signin",
        authRedirectTo: "/viewer",
      }),
    );

    renderWithProviders(<AuthDialog />);

    expect(screen.queryByText("Continue where you left off")).not.toBeInTheDocument();
    expect(screen.queryByText("Viewer home")).not.toBeInTheDocument();
    expect(screen.queryByText("We'll bring you back to the main viewer page once you're signed in.")).not.toBeInTheDocument();
    expect(screen.queryByText("/viewer")).not.toBeInTheDocument();
    expect(screen.getByLabelText("Email")).toBeInTheDocument();
    expect(screen.getByLabelText("Password")).toBeInTheDocument();
  });

  it("closes when the backdrop button is clicked", async () => {
    const closeAuthDialog = jest.fn();
    mockUseAuth.mockReturnValue(
      buildAuthState({
        user: undefined,
        authDialogOpen: true,
        closeAuthDialog,
      }),
    );

    const user = userEvent.setup();
    renderWithProviders(<AuthDialog />);

    await act(async () => {
      await user.click(screen.getByRole("button", { name: /close sign-in dialog/i }));
    });

    expect(closeAuthDialog).toHaveBeenCalled();
  });
});
