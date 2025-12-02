import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Navbar } from "../components/Navbar";
import { fetchManagedChannels } from "../lib/viewer-api";
import {
  adminUser,
  guestAuthState,
  mockUseAuth,
  signedInAuthState,
  viewerUser,
} from "./test-utils/auth";

jest.mock("next/link", () => {
  const React = require("react");
  return React.forwardRef(function MockLink({ children, ...props }: any, ref: any) {
    return React.createElement("a", {
      ...props,
      ref,
      onClick: (event: any) => {
        event.preventDefault();
        props.onClick?.(event);
      },
    }, children);
  });
});

jest.mock("next/navigation", () => ({
  useRouter: () => ({
    push: jest.fn(),
    replace: jest.fn(),
  }),
  usePathname: () => "/",
}));

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
    fetchManagedChannelsMock.mockResolvedValue([]);
  });

  test("shows a dashboard link to admins", () => {
    mockUseAuth.mockReturnValue(signedInAuthState(adminUser));

    render(<Navbar />);

    expect(screen.getByRole("link", { name: /dashboard/i })).toBeInTheDocument();
  });

  test("does not render a dashboard link for non-admins", () => {
    mockUseAuth.mockReturnValue(signedInAuthState(viewerUser));

    render(<Navbar />);

    expect(screen.queryByRole("link", { name: /dashboard/i })).not.toBeInTheDocument();
  });

  test("closes the mobile menu after visiting the dashboard link", async () => {
    mockUseAuth.mockReturnValue(signedInAuthState(adminUser));

    const user = userEvent.setup();

    render(<Navbar />);

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
    mockUseAuth.mockReturnValue(guestAuthState());

    const user = userEvent.setup();

    render(<Navbar />);

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
