import {
  adminUser,
  mockAnonymousUser,
  mockAuthenticatedUser,
  resetRouterMocks,
  renderWithProviders,
  viewerUser,
} from "../test/test-utils";
import { act, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Navbar } from "../components/Navbar";
import { fetchManagedChannels } from "../lib/viewer-api";

jest.mock("../hooks/useAuth");

jest.mock("../lib/viewer-api", () => ({
  ...jest.requireActual("../lib/viewer-api"),
  fetchManagedChannels: jest.fn(),
}));

const fetchManagedChannelsMock = fetchManagedChannels as jest.MockedFunction<
  typeof fetchManagedChannels
>;

describe("Navbar", () => {
  beforeAll(() => {
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      value: jest.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: jest.fn(),
        removeListener: jest.fn(),
        addEventListener: jest.fn(),
        removeEventListener: jest.fn(),
        dispatchEvent: jest.fn(),
      })),
    });
  });

  beforeEach(() => {
    jest.clearAllMocks();
    resetRouterMocks();
    fetchManagedChannelsMock.mockResolvedValue([]);
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
});
