import { mockUseAuth, renderWithProviders, signedInAuthState } from "../test/test-utils";
import { screen, waitFor } from "@testing-library/react";
import { FollowingSidebar } from "../components/FollowingSidebar";
import { fetchFollowingChannels } from "../lib/viewer-api";

jest.mock("../hooks/useAuth");

jest.mock("../lib/viewer-api", () => ({
  ...jest.requireActual("../lib/viewer-api"),
  fetchFollowingChannels: jest.fn()
}));

const fetchFollowingMock = fetchFollowingChannels as jest.MockedFunction<typeof fetchFollowingChannels>;

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
