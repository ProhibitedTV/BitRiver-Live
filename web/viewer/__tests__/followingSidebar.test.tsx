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
      expect(screen.getAllByText(FOLLOWING_COPY.empty).length).toBeGreaterThan(0);
    });
  });

  it("uses explicit followed-count summary wording in ready state", async () => {
    fetchFollowingMock.mockResolvedValue({
      channels: [
        {
          channel: {
            id: "channel-1",
            ownerId: "owner-1",
            title: "BitRiver Live",
            category: "Gaming",
            tags: [],
            liveState: "Live",
            createdAt: new Date().toISOString(),
            updatedAt: new Date().toISOString(),
          },
          owner: { id: "owner-1", displayName: "Owner" },
          profile: {},
          live: true,
          followerCount: 42,
        },
      ],
      generatedAt: new Date().toISOString(),
    });

    renderWithProviders(<FollowingSidebar />);

    await waitFor(() => {
      expect(screen.getByText(FOLLOWING_COPY.summaryFollowed(1))).toBeInTheDocument();
    });
    expect(screen.getByRole("heading", { name: "Following" })).toBeInTheDocument();
  });
});
