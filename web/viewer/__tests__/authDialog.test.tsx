import { screen } from "@testing-library/react";
import { AuthDialog } from "../components/auth/AuthDialog";
import { buildAuthState, mockUseAuth, renderWithProviders } from "../test/test-utils";

jest.mock("../hooks/useAuth");

describe("AuthDialog", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("keeps the return path visible without the old auto-return explanation copy", () => {
    mockUseAuth.mockReturnValue(
      buildAuthState({
        user: undefined,
        authDialogOpen: true,
        authRedirectTo: "/viewer",
      }),
    );

    renderWithProviders(<AuthDialog />);

    expect(screen.getByText("Continue where you left off")).toBeInTheDocument();
    expect(screen.getByText("/viewer")).toBeInTheDocument();
    expect(screen.queryByText(/featured broadcasts and live discovery/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/land back in browse/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/jump right back into the stream page/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/viewer route that opened this auth step/i)).not.toBeInTheDocument();
  });
});
