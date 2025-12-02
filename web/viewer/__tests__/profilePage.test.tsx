import { mockAnonymousUser, mockAuthenticatedUser, renderWithProviders, viewerApiMocks } from "../test/test-utils";
import userEvent from "@testing-library/user-event";
import { screen, waitFor } from "@testing-library/react";
import ProfilePage from "../app/profile/page";

jest.mock("../hooks/useAuth");

const fetchProfileMock = viewerApiMocks.fetchProfile;
const updateProfileMock = viewerApiMocks.updateProfile;

const profileFixture = {
  userId: "viewer-1",
  displayName: "Viewer One",
  bio: "Streaming sci-fi strategy games.",
  avatarUrl: "https://cdn.example.com/avatar.png",
  bannerUrl: "https://cdn.example.com/banner.jpg",
  featuredChannelId: undefined,
  topFriends: [],
  donationAddresses: [],
  channels: [],
  liveChannels: [],
  createdAt: new Date("2023-10-21T11:00:00Z").toISOString(),
  updatedAt: new Date("2023-10-21T12:00:00Z").toISOString(),
};

describe("ProfilePage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    fetchProfileMock.mockResolvedValue(profileFixture as any);
    updateProfileMock.mockResolvedValue(profileFixture as any);
  });

  test("prompts unauthenticated viewers to sign in", () => {
    mockAnonymousUser();

    renderWithProviders(<ProfilePage />);

    expect(screen.getByRole("heading", { level: 2, name: /sign in to manage your profile/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
  });

  test("loads the current profile and pre-fills the form", async () => {
    mockAuthenticatedUser({ id: "viewer-1", displayName: "Viewer One", email: "viewer@example.com" });

    renderWithProviders(<ProfilePage />);

    await waitFor(() => expect(fetchProfileMock).toHaveBeenCalledWith("viewer-1"));

    expect(screen.getByDisplayValue("Viewer One")).toBeInTheDocument();
    expect(screen.getByDisplayValue("viewer@example.com")).toBeInTheDocument();
    expect(screen.getByDisplayValue(profileFixture.avatarUrl)).toBeInTheDocument();
    expect(screen.getByDisplayValue(profileFixture.bannerUrl)).toBeInTheDocument();
    expect(screen.getByDisplayValue(/sci-fi strategy games/i)).toBeInTheDocument();
    expect(screen.getByRole("img", { name: /profile avatar/i })).toHaveAttribute("src", profileFixture.avatarUrl);
  });

  test("saves profile changes and shows a confirmation", async () => {
    mockAuthenticatedUser({ id: "viewer-1", displayName: "Viewer One", email: "viewer@example.com" });
    const user = userEvent.setup();

    renderWithProviders(<ProfilePage />);

    await screen.findByDisplayValue(profileFixture.avatarUrl);

    await user.clear(screen.getByLabelText("Display name"));
    await user.type(screen.getByLabelText("Display name"), "New Viewer Name");
    await user.clear(screen.getByLabelText("Email"));
    await user.type(screen.getByLabelText("Email"), "viewer+alerts@example.com");
    await user.clear(screen.getByLabelText("Bio"));
    await user.type(screen.getByLabelText("Bio"), "New bio for my streams");
    await user.clear(screen.getByLabelText("Avatar URL"));
    await user.type(screen.getByLabelText("Avatar URL"), "https://new.example.com/me.png");
    await user.click(screen.getByRole("button", { name: /save profile/i }));

    await waitFor(() => expect(updateProfileMock).toHaveBeenCalledTimes(1));
    expect(updateProfileMock).toHaveBeenCalledWith("viewer-1", {
      displayName: "New Viewer Name",
      email: "viewer+alerts@example.com",
      bio: "New bio for my streams",
      avatarUrl: "https://new.example.com/me.png",
      bannerUrl: profileFixture.bannerUrl,
      socialLinks: [],
    });

    expect(await screen.findByText(/profile saved/i)).toBeInTheDocument();
  });

  test("surfaces errors when the profile fails to load", async () => {
    fetchProfileMock.mockRejectedValueOnce(new Error("Server offline"));
    mockAuthenticatedUser({ id: "viewer-1", displayName: "Viewer One", email: "viewer@example.com" });

    renderWithProviders(<ProfilePage />);

    expect(await screen.findByRole("heading", { level: 2, name: /unable to load profile/i })).toBeInTheDocument();
    expect(screen.getByText(/server offline/i)).toBeInTheDocument();
  });
});
