import { fireEvent, screen, waitFor } from "@testing-library/react";
import {
  guestAuthState,
  mockUseAuth,
  renderWithProviders,
  signedInAuthState,
  viewerApiMocks,
} from "../test/test-utils";
import FollowingPage from "../app/following/page";
import { FollowingRail } from "../components/FollowingRail";
import { FollowingSidebar } from "../components/FollowingSidebar";
import { FOLLOWING_COPY, FOLLOWING_SIGN_IN_CTA } from "../components/following/FollowingState";

jest.mock("../hooks/useAuth");

const fetchFollowingMock = viewerApiMocks.fetchFollowingChannels;

const liveChannel = {
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
  owner: {
    id: "owner-1",
    displayName: "Owner",
  },
  profile: {},
  live: true,
  followerCount: 42,
};

describe("Following state presentation", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("renders the same unauthenticated copy and sign-in CTA across rail, sidebar, and following page", async () => {
    mockUseAuth.mockReturnValue(guestAuthState());

    renderWithProviders(<FollowingRail channels={[]} isAuthenticated={false} />);
    expect(screen.getByText(FOLLOWING_COPY.unauthenticated)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: FOLLOWING_SIGN_IN_CTA.label })).toHaveAttribute("href", FOLLOWING_SIGN_IN_CTA.href);

    renderWithProviders(<FollowingSidebar />);
    await waitFor(() => {
      expect(screen.getAllByText(FOLLOWING_COPY.unauthenticated).length).toBeGreaterThan(1);
    });

    renderWithProviders(<FollowingPage />);
    await waitFor(() => {
      expect(screen.getAllByRole("link", { name: FOLLOWING_SIGN_IN_CTA.label }).length).toBeGreaterThan(1);
    });
  });

  it("renders the same loading and empty copy across surfaces", async () => {
    renderWithProviders(<FollowingRail channels={[]} loading isAuthenticated />);
    expect(screen.getByText(FOLLOWING_COPY.loading)).toBeInTheDocument();

    renderWithProviders(<FollowingRail channels={[]} isAuthenticated />);
    expect(screen.getByText(FOLLOWING_COPY.empty)).toBeInTheDocument();

    mockUseAuth.mockReturnValue(signedInAuthState());
    fetchFollowingMock.mockResolvedValue({ channels: [], generatedAt: new Date().toISOString() });

    renderWithProviders(<FollowingSidebar />);
    expect(screen.getAllByText(FOLLOWING_COPY.loading).length).toBeGreaterThan(0);
    await waitFor(() => {
      expect(screen.getAllByText(FOLLOWING_COPY.empty).length).toBeGreaterThan(0);
    });

    fetchFollowingMock.mockResolvedValue({ channels: [], generatedAt: new Date().toISOString() });
    renderWithProviders(<FollowingPage />);
    await waitFor(() => {
      expect(screen.getAllByText(FOLLOWING_COPY.empty).length).toBeGreaterThan(0);
    });
  });

  it("shows shared error copy and retry behavior for sidebar and page", async () => {
    mockUseAuth.mockReturnValue(signedInAuthState());
    fetchFollowingMock.mockRejectedValueOnce(new Error("boom"));
    fetchFollowingMock.mockResolvedValueOnce({ channels: [liveChannel], generatedAt: new Date().toISOString() });

    renderWithProviders(<FollowingSidebar />);

    await waitFor(() => {
      expect(screen.getByText(FOLLOWING_COPY.error)).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: FOLLOWING_COPY.retry }));
    await waitFor(() => {
      expect(screen.getByText("Owner")).toBeInTheDocument();
    });

    fetchFollowingMock.mockRejectedValueOnce(new Error("boom"));
    fetchFollowingMock.mockResolvedValueOnce({ channels: [liveChannel], generatedAt: new Date().toISOString() });

    renderWithProviders(<FollowingPage />);

    await waitFor(() => {
      expect(screen.getAllByText(FOLLOWING_COPY.error).length).toBeGreaterThan(0);
    });

    fireEvent.click(screen.getAllByRole("button", { name: FOLLOWING_COPY.retry })[0]);
    await waitFor(() => {
      expect(screen.getAllByText("Owner").length).toBeGreaterThan(1);
    });
  });
});
