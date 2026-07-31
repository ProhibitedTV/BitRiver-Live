import {
  adminUser,
  creatorUser,
  mockAnonymousUser,
  mockAuthenticatedUser,
  mockUseAuth,
  resetRouterMocks,
  renderWithProviders,
  viewerApiMocks,
  viewerUser,
  guestAuthState,
} from "../test/test-utils";
import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Navbar } from "../components/Navbar";
import { navigateBrowser } from "../lib/browser-navigation";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

jest.mock("../hooks/useAuth");
jest.mock("../lib/browser-navigation");

const fetchManagedChannelsMock = viewerApiMocks.fetchManagedChannels;
const navigateBrowserMock = jest.mocked(navigateBrowser);

describe("Navbar", () => {
  const originalApiBase = process.env.NEXT_PUBLIC_API_BASE_URL;
  const originalSignupUrl = process.env.NEXT_PUBLIC_SIGNUP_URL;
  const originalViewerUrl = process.env.NEXT_PUBLIC_VIEWER_URL;
  const mediaListeners = new Map<string, ((event: MediaQueryListEvent) => void)[]>();
  const setMatchMedia = (resolver: (query: string) => boolean) => {
    mediaListeners.clear();
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      value: jest.fn().mockImplementation((query: string) => ({
        matches: resolver(query),
        media: query,
        onchange: null,
        addListener: jest.fn(),
        removeListener: jest.fn(),
        addEventListener: jest.fn((eventName: string, listener: (event: MediaQueryListEvent) => void) => {
          if (eventName !== "change") {
            return;
          }
          const existing = mediaListeners.get(query) ?? [];
          mediaListeners.set(query, [...existing, listener]);
        }),
        removeEventListener: jest.fn((eventName: string, listener: (event: MediaQueryListEvent) => void) => {
          if (eventName !== "change") {
            return;
          }
          const existing = mediaListeners.get(query) ?? [];
          mediaListeners.set(
            query,
            existing.filter((registered) => registered !== listener),
          );
        }),
        dispatchEvent: jest.fn(),
      })),
    });
  };
  const emitMatchMediaChange = (query: string, matches: boolean) => {
    const listeners = mediaListeners.get(query) ?? [];
    const event = { matches, media: query } as MediaQueryListEvent;
    listeners.forEach((listener) => listener(event));
  };

  beforeAll(() => {
    setMatchMedia(() => false);
  });

  beforeEach(() => {
    jest.clearAllMocks();
    setMatchMedia(() => false);
    resetRouterMocks();
    window.localStorage.clear();
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

  test("keeps the control center link inside the account menu for admins", async () => {
    mockAuthenticatedUser(adminUser);
    const user = userEvent.setup();

    renderWithProviders(<Navbar />);

    expect(document.querySelector<HTMLAnchorElement>(".navbar-right .nav-cta[href='/admin']")).toBeNull();

    await act(async () => {
      await user.click(screen.getByRole("button", { name: /open account menu/i }));
    });

    const accountMenu = document.getElementById("viewer-user-menu");
    expect(accountMenu).toHaveClass("avatar-menu__items--open");
    expect(within(accountMenu!).getByRole("link", { name: /control center/i })).toHaveAttribute("href", "/admin");
  });

  test("does not render a control center link for non-admins", () => {
    mockAuthenticatedUser(viewerUser);

    renderWithProviders(<Navbar />);

    expect(screen.queryByRole("link", { name: /control center/i })).not.toBeInTheDocument();
  });

  test("moves creator go-live utilities out of the persistent header", async () => {
    mockAuthenticatedUser(creatorUser);
    fetchManagedChannelsMock.mockResolvedValue([{ id: "channel-alpha" }] as any);
    const user = userEvent.setup();

    renderWithProviders(<Navbar />);

    expect(document.querySelector<HTMLAnchorElement>(".navbar-right .nav-cta")).toBeNull();

    await act(async () => {
      await user.click(screen.getByRole("button", { name: /open account menu/i }));
    });

    await waitFor(() => {
      expect(screen.getByRole("link", { name: /go live/i })).toHaveAttribute("href", "/creator/live/channel-alpha");
    });
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


  test("renders desktop tabs and drawer links from the same primary nav list", async () => {
    mockAuthenticatedUser(adminUser);

    const user = userEvent.setup();

    renderWithProviders(<Navbar />);

    const desktopNav = screen.getByRole("navigation", { name: /primary navigation/i });
    const desktopLabels = within(desktopNav)
      .getAllByRole("link")
      .map((link) => link.textContent?.trim());

    const toggleButton = screen.getByRole("button", { name: /open navigation menu/i });
    await act(async () => {
      await user.click(toggleButton);
    });

    const drawerNav = screen.getByRole("navigation", { name: /primary navigation mobile/i });
    const drawerLabels = within(drawerNav)
      .getAllByRole("link")
      .map((link) => link.textContent?.trim());

    expect(drawerLabels).toEqual(desktopLabels);
  });

  test("closes the mobile menu after visiting the control center link", async () => {
    mockAuthenticatedUser(adminUser);

    const user = userEvent.setup();

    renderWithProviders(<Navbar />);

    const toggleButton = screen.getByRole("button", { name: /open navigation menu/i });
    await act(async () => {
      await user.click(toggleButton);
    });

    expect(toggleButton).toHaveAttribute("aria-expanded", "true");

    const shortcuts = screen.getByLabelText(/platform shortcuts/i);
    const dashboardLink = within(shortcuts).getByRole("link", { name: /control center/i });
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
    ["Home", "Browse", "Following", "Videos"].forEach((label) => {
      expect(drawer.getAllByRole("link", { name: new RegExp(label, "i") })).toHaveLength(1);
    });
  });

  test("traps Tab and Shift+Tab focus within the open mobile drawer", async () => {
    mockAnonymousUser();
    setMatchMedia((query) => query === "(max-width: 1080px)");
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
    setMatchMedia((query) => query === "(max-width: 1080px)");
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


  test("keeps keyboard focus path available for both inline and drawer navbar search inputs", async () => {
    mockAnonymousUser();
    setMatchMedia((query) => query === "(max-width: 1080px)");
    const user = userEvent.setup();

    renderWithProviders(<Navbar />);

    const inlineSearch = screen.getByRole("searchbox", { name: /search for channels or categories/i });
    inlineSearch.focus();
    expect(inlineSearch).toHaveFocus();

    const toggleButton = screen.getByRole("button", { name: /open navigation menu/i });
    await act(async () => {
      await user.click(toggleButton);
    });

    const drawer = document.getElementById("viewer-nav-menu");
    expect(drawer).not.toBeNull();
    const drawerSearch = within(drawer!).getByRole("searchbox", { name: /search for channels or categories/i });
    drawerSearch.focus();
    expect(drawerSearch).toHaveFocus();
    expect(drawerSearch.closest(".nav-search")).toHaveClass("nav-search--drawer");
  });

  test("removes the old notifications roadmap dead-end from the header", () => {
    mockAnonymousUser();

    renderWithProviders(<Navbar />);

    expect(screen.queryByRole("button", { name: /notifications roadmap details/i })).not.toBeInTheDocument();
  });

  test("defines a visible nav-search focus-within style contract for dark and light themes", () => {
    const globalsCssPath = resolve(__dirname, "../styles/globals.css");
    const navigationCssPath = resolve(__dirname, "../styles/navigation.css");
    const globalsCss = readFileSync(globalsCssPath, "utf8");
    const navigationCss = readFileSync(navigationCssPath, "utf8");

    expect(globalsCss).toContain("--navbar-search-focus-ring");
    expect(globalsCss).toContain("--navbar-search-focus-border");
    expect(navigationCss).toContain(".nav-search:focus-within");
    expect(navigationCss).toContain("box-shadow: var(--navbar-search-shadow), 0 0 0 3px var(--navbar-search-focus-ring);");
  });

  test("keeps the mobile nav toggle display rule after the final nav-toggle base rule", () => {
    const navigationCssPath = resolve(__dirname, "../styles/navigation.css");
    const navigationCss = readFileSync(navigationCssPath, "utf8");
    const hiddenToggleRules = Array.from(navigationCss.matchAll(/\.nav-toggle\s*{\s*display:\s*none;/g));
    const finalHiddenToggleRule = hiddenToggleRules[hiddenToggleRules.length - 1]?.index ?? -1;
    const mobileToggleRule = navigationCss.lastIndexOf("@media (max-width: 1080px)");

    expect(finalHiddenToggleRule).toBeGreaterThan(-1);
    expect(mobileToggleRule).toBeGreaterThan(finalHiddenToggleRule);
    expect(navigationCss.slice(mobileToggleRule)).toMatch(/\.nav-toggle\s*{\s*display:\s*inline-flex;/);
  });

  test("loads the stored theme preference on initial render", () => {
    mockAnonymousUser();
    window.localStorage.setItem("viewer-theme", "light");
    setMatchMedia(() => false);

    renderWithProviders(<Navbar />);

    expect(document.body).toHaveAttribute("data-theme", "light");
    expect(screen.getByRole("button", { name: /switch to dark theme/i })).toBeInTheDocument();
  });

  test("falls back to prefers-color-scheme when no saved preference exists", () => {
    mockAnonymousUser();
    setMatchMedia((query) => query === "(prefers-color-scheme: light)");

    renderWithProviders(<Navbar />);

    expect(document.body).toHaveAttribute("data-theme", "light");
    expect(screen.getByRole("button", { name: /switch to dark theme/i })).toBeInTheDocument();
  });

  test("persists manual theme toggle across remount and ignores media query updates", async () => {
    mockAnonymousUser();
    setMatchMedia((query) => query === "(prefers-color-scheme: light)");
    const user = userEvent.setup();

    const { unmount } = renderWithProviders(<Navbar />);

    const themeToggle = screen.getByRole("button", { name: /switch to dark theme/i });
    await act(async () => {
      await user.click(themeToggle);
    });

    expect(window.localStorage.getItem("viewer-theme")).toBe("dark");
    expect(document.body).not.toHaveAttribute("data-theme", "light");

    await act(async () => {
      emitMatchMediaChange("(prefers-color-scheme: light)", true);
    });

    expect(document.body).not.toHaveAttribute("data-theme", "light");

    unmount();
    renderWithProviders(<Navbar />);

    expect(document.body).not.toHaveAttribute("data-theme", "light");
    expect(screen.getByRole("button", { name: /switch to light theme/i })).toBeInTheDocument();
  });


  test("shows a local setup banner when API base URL is localhost", () => {
    mockAnonymousUser();
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://localhost:8080";

    renderWithProviders(<Navbar />);

    const banner = screen.getByRole("status");
    expect(within(banner).getByText(/local setup mode/i)).toBeInTheDocument();
    expect(within(banner).getByRole("link", { name: /setup guide/i })).toHaveAttribute("href", "/getting-started");
  });

  test("shows a local setup banner when viewer URL is localhost", () => {
    mockAnonymousUser();
    process.env.NEXT_PUBLIC_VIEWER_URL = "http://localhost:3000";
    delete process.env.NEXT_PUBLIC_API_BASE_URL;

    renderWithProviders(<Navbar />);

    expect(screen.getByText(/local setup mode/i)).toBeInTheDocument();
  });

  test("hides the local setup banner for non-local URLs", () => {
    mockAnonymousUser();
    process.env.NEXT_PUBLIC_VIEWER_URL = "https://viewer.example.com";
    process.env.NEXT_PUBLIC_API_BASE_URL = "https://api.example.com";

    renderWithProviders(<Navbar />);

    expect(screen.queryByText(/running in local setup mode/i)).not.toBeInTheDocument();
  });

  test("shows sign in and create-account calls-to-action when signed out", () => {
    mockAnonymousUser();

    renderWithProviders(<Navbar />);

    expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /create account/i })).toBeInTheDocument();
  });

  test("keeps authentication actions disabled until viewer auth discovery completes", () => {
    mockUseAuth.mockReturnValue({
      ...guestAuthState(),
      loading: true,
    });

    renderWithProviders(<Navbar />);

    expect(screen.getByRole("button", { name: /sign in/i })).toBeDisabled();
    expect(screen.getByRole("button", { name: /create account/i })).toBeDisabled();
    expect(screen.getByRole("group", { name: /account and preferences/i }).querySelector(".auth-buttons")).toHaveAttribute(
      "aria-busy",
      "true",
    );
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

  test("create-account CTA opens the in-viewer auth overlay with the current path as the redirect target", async () => {
    const signUp = jest.fn();
    mockUseAuth.mockReturnValue({
      ...guestAuthState(),
      signUp,
    });
    window.history.pushState({}, "", "/browse?tag=music#top");

    const user = userEvent.setup();

    renderWithProviders(<Navbar />);

    const joinButton = screen.getByRole("button", { name: /create account/i });
    await act(async () => {
      await user.click(joinButton);
    });

    expect(signUp).toHaveBeenCalledWith("/browse?tag=music#top");
  });

  test("create-account CTA routes to a configured onboarding URL", async () => {
    mockAnonymousUser();
    process.env.NEXT_PUBLIC_SIGNUP_URL = "https://accounts.example.com/onboarding";
    const user = userEvent.setup();

    renderWithProviders(<Navbar />);

    const joinButton = screen.getByRole("button", { name: /create account/i });
    await act(async () => {
      await user.click(joinButton);
    });

    expect(navigateBrowserMock).toHaveBeenCalledWith(
      expect.stringMatching(/^https:\/\/accounts\.example\.com\/onboarding\?next=%2F/),
    );
  });

  test("create-account CTA respects a configured auth base URL", async () => {
    mockAnonymousUser();
    process.env.NEXT_PUBLIC_API_BASE_URL = "https://auth.example.com";
    const user = userEvent.setup();

    renderWithProviders(<Navbar />);

    const joinButton = screen.getByRole("button", { name: /create account/i });
    await act(async () => {
      await user.click(joinButton);
    });

    expect(navigateBrowserMock).toHaveBeenCalledWith(
      expect.stringMatching(/^https:\/\/auth\.example\.com\/signup\?next=%2F/),
    );
  });

  test("shows only the sign in CTA when signup is not configured", () => {
    mockAnonymousUser();
    process.env.NEXT_PUBLIC_SIGNUP_URL = "";

    renderWithProviders(<Navbar />);

    expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /create account/i })).not.toBeInTheDocument();
  });

  test("hides the create-account CTA when self-signup is disabled on the current install", () => {
    mockUseAuth.mockReturnValue({
      ...guestAuthState(),
      allowSelfSignup: false,
    });

    renderWithProviders(<Navbar />);

    expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /create account/i })).not.toBeInTheDocument();
  });

  test("hides the join CTA when self-signup is disabled on the current install", () => {
    mockUseAuth.mockReturnValue({
      ...guestAuthState(),
      allowSelfSignup: false,
    });

    renderWithProviders(<Navbar />);

    expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /join/i })).not.toBeInTheDocument();
  });
});
