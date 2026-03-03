import {
  adminUser,
  mockAnonymousUser,
  mockAuthenticatedUser,
  mockUseAuth,
  resetRouterMocks,
  renderWithProviders,
  viewerApiMocks,
  viewerUser,
  guestAuthState,
} from "../test/test-utils";
import { act, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Navbar } from "../components/Navbar";

jest.mock("../hooks/useAuth");

const fetchManagedChannelsMock = viewerApiMocks.fetchManagedChannels;

describe("Navbar", () => {
  const originalApiBase = process.env.NEXT_PUBLIC_API_BASE_URL;
  const originalSignupUrl = process.env.NEXT_PUBLIC_SIGNUP_URL;
  const originalViewerUrl = process.env.NEXT_PUBLIC_VIEWER_URL;
  const overrideWindowLocation = (
    overrides: Partial<Pick<Location, "hash" | "href" | "origin" | "pathname" | "search">>,
  ) => {
    const originalLocation = window.location;
    const mockLocation = {
      ancestorOrigins: originalLocation.ancestorOrigins,
      assign: jest.fn(),
      hash: "",
      host: "localhost",
      hostname: "localhost",
      href: "http://localhost/",
      origin: "http://localhost",
      pathname: "/",
      port: "",
      protocol: "http:",
      reload: jest.fn(),
      replace: jest.fn(),
      search: "",
      toString: () => "http://localhost/",
      ...overrides,
    } as unknown as Location & { href: string };
    Object.defineProperty(window, "location", {
      configurable: true,
      value: mockLocation,
    });
    return {
      mockLocation,
      restore: () =>
        Object.defineProperty(window, "location", { configurable: true, value: originalLocation }),
    };
  };
  const setMatchMedia = (resolver: (query: string) => boolean) => {
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      value: jest.fn().mockImplementation((query: string) => ({
        matches: resolver(query),
        media: query,
        onchange: null,
        addListener: jest.fn(),
        removeListener: jest.fn(),
        addEventListener: jest.fn(),
        removeEventListener: jest.fn(),
        dispatchEvent: jest.fn(),
      })),
    });
  };

  beforeAll(() => {
    setMatchMedia(() => false);
  });

  beforeEach(() => {
    jest.clearAllMocks();
    setMatchMedia(() => false);
    resetRouterMocks();
    fetchManagedChannelsMock.mockResolvedValue([]);
  });

  afterEach(() => {
    if (originalApiBase === undefined) {
      delete process.env.NEXT_PUBLIC_API_BASE_URL;
    } else {
      process.env.NEXT_PUBLIC_API_BASE_URL = originalApiBase;
    }
    if (originalSignupUrl === undefined) {
      delete process.env.NEXT_PUBLIC_SIGNUP_URL;
    } else {
      process.env.NEXT_PUBLIC_SIGNUP_URL = originalSignupUrl;
    }
    if (originalViewerUrl === undefined) {
      delete process.env.NEXT_PUBLIC_VIEWER_URL;
    } else {
      process.env.NEXT_PUBLIC_VIEWER_URL = originalViewerUrl;
    }
    window.history.replaceState({}, "", "/");
  });

  test("shows a dashboard link to admins", () => {
    mockAuthenticatedUser(adminUser);

    renderWithProviders(<Navbar />);

    expect(screen.getByRole("link", { name: /dashboard/i })).toBeInTheDocument();
  });

  test("does not render a dashboard link for non-admins", () => {
    mockAuthenticatedUser(viewerUser);

    renderWithProviders(<Navbar />);

    expect(screen.queryByRole("link", { name: /dashboard/i })).not.toBeInTheDocument();
  });

  test("closes the account menu on outside click", async () => {
    mockAuthenticatedUser(viewerUser);
    const user = userEvent.setup();

    renderWithProviders(<Navbar />);

    const avatarButton = screen.getByRole("button", { name: /open account menu/i });
    await act(async () => {
      await user.click(avatarButton);
    });

    const userMenu = document.getElementById("viewer-user-menu");
    expect(userMenu).toHaveClass("avatar-menu__items--open");

    await act(async () => {
      await user.click(document.body);
    });

    expect(userMenu).not.toHaveClass("avatar-menu__items--open");
    expect(avatarButton).toHaveAttribute("aria-expanded", "false");
  });

  test("closes the account menu on Escape", async () => {
    mockAuthenticatedUser(viewerUser);
    const user = userEvent.setup();

    renderWithProviders(<Navbar />);

    const avatarButton = screen.getByRole("button", { name: /open account menu/i });
    await act(async () => {
      await user.click(avatarButton);
    });

    const userMenu = document.getElementById("viewer-user-menu");
    expect(userMenu).toHaveClass("avatar-menu__items--open");

    await act(async () => {
      await user.keyboard("{Escape}");
    });

    expect(userMenu).not.toHaveClass("avatar-menu__items--open");
    expect(avatarButton).toHaveAttribute("aria-expanded", "false");
  });

  test("restores focus to avatar toggle after Escape closes account menu", async () => {
    mockAuthenticatedUser(viewerUser);
    const user = userEvent.setup();

    renderWithProviders(<Navbar />);

    const avatarButton = screen.getByRole("button", { name: /open account menu/i });
    await act(async () => {
      await user.click(avatarButton);
    });

    const signOutButton = screen.getByRole("button", { name: /sign out/i });
    signOutButton.focus();
    expect(signOutButton).toHaveFocus();

    await act(async () => {
      await user.keyboard("{Escape}");
    });

    expect(avatarButton).toHaveFocus();
  });


  test("disables notifications action with coming-soon helper text", () => {
    mockAnonymousUser();

    renderWithProviders(<Navbar />);

    const notificationsButton = screen.getByRole("button", { name: /view notifications/i });
    expect(notificationsButton).toBeDisabled();
    expect(notificationsButton).toHaveAttribute("title", "Notifications coming soon");
  });


  test("renders desktop tabs and drawer links from the same primary nav list", async () => {
    mockAuthenticatedUser(adminUser);

    const user = userEvent.setup();

    renderWithProviders(<Navbar />);

    const desktopNav = screen.getByRole("group", { name: /viewer navigation/i });
    const desktopLabels = within(desktopNav)
      .getAllByRole("link")
      .map((link) => link.textContent?.trim());

    const toggleButton = screen.getByRole("button", { name: /open navigation menu/i });
    await act(async () => {
      await user.click(toggleButton);
    });

    const drawerNav = screen.getByRole("group", { name: /viewer navigation mobile/i });
    const drawerLabels = within(drawerNav)
      .getAllByRole("link")
      .map((link) => link.textContent?.trim());

    expect(drawerLabels).toEqual(desktopLabels);
  });

  test("closes the mobile menu after visiting the dashboard link", async () => {
    mockAuthenticatedUser(adminUser);

    const user = userEvent.setup();

    renderWithProviders(<Navbar />);

    const toggleButton = screen.getByRole("button", { name: /open navigation menu/i });
    await act(async () => {
      await user.click(toggleButton);
    });

    expect(toggleButton).toHaveAttribute("aria-expanded", "true");

    const dashboardLink = screen.getByRole("link", { name: /dashboard/i });
    await act(async () => {
      await user.click(dashboardLink);
    });

    expect(toggleButton).toHaveAttribute("aria-expanded", "false");
  });

  test("renders each primary link once in the mobile drawer", async () => {
    mockAnonymousUser();

    const user = userEvent.setup();

    renderWithProviders(<Navbar />);

    const toggleButton = screen.getByRole("button", { name: /open navigation menu/i });
    await act(async () => {
      await user.click(toggleButton);
    });

    const navDrawer = document.getElementById("viewer-nav-menu");
    expect(navDrawer).toBeInTheDocument();

    const drawer = within(navDrawer!);
    ["Home", "Following", "Browse"].forEach((label) => {
      expect(drawer.getAllByRole("link", { name: new RegExp(label, "i") })).toHaveLength(1);
    });
  });

  test("traps Tab and Shift+Tab focus within the open mobile drawer", async () => {
    mockAnonymousUser();
    setMatchMedia((query) => query === "(max-width: 800px)");
    const user = userEvent.setup();

    renderWithProviders(<Navbar />);

    const toggleButton = screen.getByRole("button", { name: /open navigation menu/i });
    await act(async () => {
      await user.click(toggleButton);
    });

    const drawer = document.getElementById("viewer-nav-menu");
    expect(drawer).toHaveAttribute("role", "dialog");
    expect(drawer).toHaveAttribute("aria-modal", "true");

    const focusable = drawer?.querySelectorAll<HTMLElement>(
      'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
    );
    const first = focusable?.item(0);
    const last = focusable?.item((focusable?.length ?? 1) - 1);
    expect(first).toHaveFocus();

    last?.focus();
    await act(async () => {
      await user.keyboard("{Tab}");
    });
    expect(first).toHaveFocus();

    first?.focus();
    await act(async () => {
      await user.keyboard("{Shift>}{Tab}{/Shift}");
    });
    expect(last).toHaveFocus();
  });

  test("restores focus to menu toggle after closing mobile drawer", async () => {
    mockAnonymousUser();
    setMatchMedia((query) => query === "(max-width: 800px)");
    const user = userEvent.setup();

    renderWithProviders(<Navbar />);

    const toggleButton = screen.getByRole("button", { name: /open navigation menu/i });
    await act(async () => {
      await user.click(toggleButton);
    });

    const closeBackdrop = document.querySelector<HTMLButtonElement>(".nav-drawer-backdrop");
    expect(closeBackdrop).not.toBeNull();
    await act(async () => {
      await user.click(closeBackdrop!);
    });

    expect(toggleButton).toHaveFocus();
    expect(toggleButton).toHaveAttribute("aria-expanded", "false");
  });


  test("shows a local setup banner when API base URL is localhost", () => {
    mockAnonymousUser();
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://localhost:8080";

    renderWithProviders(<Navbar />);

    expect(screen.getByText(/running in local setup mode/i)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /setup guide/i })).toHaveAttribute("href", "/getting-started");
  });

  test("shows a local setup banner when viewer URL is localhost", () => {
    mockAnonymousUser();
    process.env.NEXT_PUBLIC_VIEWER_URL = "http://localhost:3000";
    delete process.env.NEXT_PUBLIC_API_BASE_URL;

    renderWithProviders(<Navbar />);

    expect(screen.getByText(/running in local setup mode/i)).toBeInTheDocument();
  });

  test("hides the local setup banner for non-local URLs", () => {
    mockAnonymousUser();
    process.env.NEXT_PUBLIC_VIEWER_URL = "https://viewer.example.com";
    process.env.NEXT_PUBLIC_API_BASE_URL = "https://api.example.com";

    renderWithProviders(<Navbar />);

    expect(screen.queryByText(/running in local setup mode/i)).not.toBeInTheDocument();
  });

  test("shows sign in and join calls-to-action when signed out", () => {
    mockAnonymousUser();

    renderWithProviders(<Navbar />);

    expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /join/i })).toBeInTheDocument();
  });

  test("sign in CTA calls the auth handler with a redirect target", async () => {
    const signIn = jest.fn();
    mockUseAuth.mockReturnValue({
      ...guestAuthState(),
      signIn,
    });
    window.history.pushState({}, "", "/channels/alpha?view=live#info");

    const user = userEvent.setup();

    renderWithProviders(<Navbar />);

    const signInButton = screen.getByRole("button", { name: /sign in/i });
    await act(async () => {
      await user.click(signInButton);
    });

    expect(signIn).toHaveBeenCalledWith("/channels/alpha?view=live#info");
  });

  test("join CTA routes to the signup page with the current path as the next parameter", async () => {
    mockAnonymousUser();
    const { mockLocation, restore } = overrideWindowLocation({
      pathname: "/browse",
      search: "?tag=music",
      hash: "#top",
    });

    const user = userEvent.setup();

    renderWithProviders(<Navbar />);

    const joinButton = screen.getByRole("button", { name: /join/i });
    await act(async () => {
      await user.click(joinButton);
    });

    expect(mockLocation.href).toBe("http://localhost/signup?next=%2Fbrowse%3Ftag%3Dmusic%23top");
    restore();
  });

  test("join CTA routes to a configured onboarding URL", async () => {
    mockAnonymousUser();
    process.env.NEXT_PUBLIC_SIGNUP_URL = "https://accounts.example.com/onboarding";
    const { mockLocation, restore } = overrideWindowLocation({});

    const user = userEvent.setup();

    renderWithProviders(<Navbar />);

    const joinButton = screen.getByRole("button", { name: /join/i });
    await act(async () => {
      await user.click(joinButton);
    });

    expect(mockLocation.href).toMatch(/^https:\/\/accounts\.example\.com\/onboarding\?next=%2F/);
    restore();
  });

  test("join CTA respects a configured auth base URL", async () => {
    mockAnonymousUser();
    process.env.NEXT_PUBLIC_API_BASE_URL = "https://auth.example.com";
    const { mockLocation, restore } = overrideWindowLocation({});

    const user = userEvent.setup();

    renderWithProviders(<Navbar />);

    const joinButton = screen.getByRole("button", { name: /join/i });
    await act(async () => {
      await user.click(joinButton);
    });

    expect(mockLocation.href).toMatch(/^https:\/\/auth\.example\.com\/signup\?next=%2F/);
    restore();
  });

  test("shows only the sign in CTA when signup is not configured", () => {
    mockAnonymousUser();
    process.env.NEXT_PUBLIC_SIGNUP_URL = "";

    renderWithProviders(<Navbar />);

    expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /join/i })).not.toBeInTheDocument();
  });
});
