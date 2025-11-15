import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Navbar } from "../components/Navbar";
import { useAuth } from "../hooks/useAuth";
import { fetchManagedChannels } from "../lib/viewer-api";

jest.mock("next/link", () => {
  const React = require("react");
  return React.forwardRef(function MockLink({ children, ...props }: any, ref: any) {
    return React.createElement("a", { ...props, ref }, children);
  });
});

jest.mock("../hooks/useAuth");

jest.mock("../lib/viewer-api", () => ({
  ...jest.requireActual("../lib/viewer-api"),
  fetchManagedChannels: jest.fn(),
}));

const mockUseAuth = useAuth as jest.MockedFunction<typeof useAuth>;
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

  const authBase = {
    loading: false,
    error: undefined,
    login: jest.fn(),
    signup: jest.fn(),
    logout: jest.fn(),
    refresh: jest.fn(),
  } as const;

  test("shows a dashboard link to admins", () => {
    mockUseAuth.mockReturnValue({
      ...authBase,
      user: {
        id: "admin-1",
        displayName: "Admin",
        email: "admin@example.com",
        roles: ["admin"],
      },
    });

    render(<Navbar />);

    expect(screen.getByRole("link", { name: /dashboard/i })).toBeInTheDocument();
  });

  test("does not render a dashboard link for non-admins", () => {
    mockUseAuth.mockReturnValue({
      ...authBase,
      user: {
        id: "viewer-1",
        displayName: "Viewer",
        email: "viewer@example.com",
        roles: [],
      },
    });

    render(<Navbar />);

    expect(screen.queryByRole("link", { name: /dashboard/i })).not.toBeInTheDocument();
  });

  test("closes the mobile menu after visiting the dashboard link", async () => {
    mockUseAuth.mockReturnValue({
      ...authBase,
      user: {
        id: "admin-1",
        displayName: "Admin",
        email: "admin@example.com",
        roles: ["admin"],
      },
    });

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
});
