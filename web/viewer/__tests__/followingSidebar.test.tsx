import { mockUseAuth, renderWithProviders, signedInAuthState, viewerApiMocks } from "../test/test-utils";
import { screen, waitFor } from "@testing-library/react";
import { FollowingSidebar } from "../components/FollowingSidebar";

jest.mock("../hooks/useAuth");

const fetchFollowingMock = viewerApiMocks.fetchFollowingChannels;

describe("FollowingSidebar", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockUseAuth.mockReturnValue(signedInAuthState());
  });

  it("shows a loading state while checking followed channels", () => {
    fetchFollowingMock.mockReturnValue(new Promise(() => {}) as Promise<any>);

    renderWithProviders(<FollowingSidebar />);

    expect(screen.getByText(/checking which creators are live/i)).toBeInTheDocument();
  });

  it("renders a compact empty message when no channels are followed", async () => {
    fetchFollowingMock.mockResolvedValue({
      channels: [],
      generatedAt: new Date().toISOString()
    });

    renderWithProviders(<FollowingSidebar />);

    await waitFor(() => {
      expect(screen.getByText(/you['’]re not following any channels yet/i)).toBeInTheDocument();
    });
  });
});
