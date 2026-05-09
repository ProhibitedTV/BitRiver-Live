import { screen, waitFor } from "@testing-library/react";
import {
  mockAnonymousUser,
  mockAuthenticatedUser,
  renderWithProviders,
  viewerApiMocks,
} from "../test/test-utils";
import CreatorIndexPage from "../app/creator/page";

jest.mock("../hooks/useAuth");

const fetchManagedChannelsMock = viewerApiMocks.fetchManagedChannels;

describe("CreatorIndexPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  test("prompts guests to sign in or create an account", () => {
    mockAnonymousUser();

    renderWithProviders(<CreatorIndexPage />);

    expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /create account/i })).toBeInTheDocument();
  });

  test("sends signed-in users without channels to first-channel setup", async () => {
    mockAuthenticatedUser({ id: "viewer-1", roles: [] });
    fetchManagedChannelsMock.mockResolvedValue([] as any);

    renderWithProviders(<CreatorIndexPage />);

    await waitFor(() => {
      expect(fetchManagedChannelsMock).toHaveBeenCalled();
    });

    expect(screen.getByRole("link", { name: /open first-channel setup/i })).toHaveAttribute(
      "href",
      "/creator/getting-started",
    );
    expect(screen.getByRole("link", { name: /unlock with first channel/i })).toHaveAttribute(
      "href",
      "/creator/getting-started",
    );
    expect(screen.getByText(/create your first channel to unlock obs settings/i)).toBeInTheDocument();
  });

  test("links creators with channels back into the live dashboard", async () => {
    mockAuthenticatedUser({ id: "creator-1", roles: ["creator"] });
    fetchManagedChannelsMock.mockResolvedValue([
      {
        id: "chan-1",
        ownerId: "creator-1",
        title: "Main Stage",
        category: "Music",
        tags: [],
        liveState: "offline",
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        streamKey: "sk_live_123",
      },
    ] as any);

    renderWithProviders(<CreatorIndexPage />);

    expect(await screen.findByRole("link", { name: /open go-live dashboard/i })).toHaveAttribute(
      "href",
      "/creator/live/chan-1",
    );
    expect(screen.getByRole("link", { name: /open uploads/i })).toHaveAttribute("href", "/creator/uploads/chan-1");
  });
});
