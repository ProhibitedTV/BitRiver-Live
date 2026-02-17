import { screen, waitFor } from "@testing-library/react";
import { FOLLOWING_COPY } from "../components/following/FollowingState";
import { mockUseAuth, renderWithProviders, signedInAuthState, viewerApiMocks } from "../test/test-utils";
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

    expect(screen.getAllByText(FOLLOWING_COPY.loading).length).toBeGreaterThan(0);
  });

  it("renders the shared empty message when no channels are followed", async () => {
    fetchFollowingMock.mockResolvedValue({
      channels: [],
      generatedAt: new Date().toISOString(),
    });

    renderWithProviders(<FollowingSidebar />);

    await waitFor(() => {
      expect(screen.getByText(FOLLOWING_COPY.empty)).toBeInTheDocument();
    });
  });
});
