import userEvent from "@testing-library/user-event";
import { render, screen, waitFor } from "@testing-library/react";
import ProfilePage from "../app/profile/page";
import { useAuth } from "../hooks/useAuth";
import { fetchProfile, updateProfile } from "../lib/viewer-api";

jest.mock("next/link", () => {
  const React = require("react");
  return React.forwardRef(function MockLink({ children, ...props }: any, ref: any) {
    return React.createElement("a", { ...props, ref }, children);
  });
});

jest.mock("../hooks/useAuth");

jest.mock("../lib/viewer-api", () => ({
  ...jest.requireActual("../lib/viewer-api"),
  fetchProfile: jest.fn(),
  updateProfile: jest.fn(),
}));

const mockUseAuth = useAuth as jest.MockedFunction<typeof useAuth>;
const fetchProfileMock = fetchProfile as jest.MockedFunction<typeof fetchProfile>;
const updateProfileMock = updateProfile as jest.MockedFunction<typeof updateProfile>;

const authBase = {
  loading: false,
  error: undefined,
  signIn: jest.fn(),
  signOut: jest.fn(),
} as const;

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
    mockUseAuth.mockReturnValue({ ...authBase, user: undefined });

    render(<ProfilePage />);

    expect(screen.getByRole("heading", { level: 2, name: /sign in to manage your profile/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
  });

  test("loads the current profile and pre-fills the form", async () => {
    mockUseAuth.mockReturnValue({
      ...authBase,
      user: { id: "viewer-1", displayName: "Viewer One", email: "viewer@example.com", roles: [] },
    });

    render(<ProfilePage />);

    await waitFor(() => expect(fetchProfileMock).toHaveBeenCalledWith("viewer-1"));

    expect(screen.getByDisplayValue("Viewer One")).toBeInTheDocument();
    expect(screen.getByDisplayValue("viewer@example.com")).toBeInTheDocument();
    expect(screen.getByDisplayValue(profileFixture.avatarUrl)).toBeInTheDocument();
    expect(screen.getByDisplayValue(profileFixture.bannerUrl)).toBeInTheDocument();
    expect(screen.getByDisplayValue(/sci-fi strategy games/i)).toBeInTheDocument();
    expect(screen.getByRole("img", { name: /profile avatar/i })).toHaveAttribute("src", profileFixture.avatarUrl);
  });

  test("saves profile changes and shows a confirmation", async () => {
    mockUseAuth.mockReturnValue({
      ...authBase,
      user: { id: "viewer-1", displayName: "Viewer One", email: "viewer@example.com", roles: [] },
    });
    const user = userEvent.setup();

    render(<ProfilePage />);

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
    mockUseAuth.mockReturnValue({
      ...authBase,
      user: { id: "viewer-1", displayName: "Viewer One", email: "viewer@example.com", roles: [] },
    });

    render(<ProfilePage />);

    expect(await screen.findByRole("heading", { level: 2, name: /unable to load profile/i })).toBeInTheDocument();
    expect(screen.getByText(/server offline/i)).toBeInTheDocument();
  });
});
