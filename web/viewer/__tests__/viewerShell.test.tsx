import { fireEvent, render, screen } from "@testing-library/react";
import { ViewerShell } from "../components/ViewerShell";

const mockUsePathname = jest.fn(() => "/");
const mockUseAuth = jest.fn();
const mockUseFollowingChannels = jest.fn();

jest.mock("next/navigation", () => ({
  usePathname: () => mockUsePathname(),
}));

jest.mock("../components/FollowingSidebar", () => ({
  FollowingSidebarContent: () => (
    <div data-testid="following-sidebar">
      <button type="button">Sidebar action</button>
      <a href="/following">Following link</a>
    </div>
  )
}));

jest.mock("../hooks/useAuth", () => ({
  useAuth: () => mockUseAuth(),
}));

jest.mock("../components/following/useFollowingChannels", () => ({
  useFollowingChannels: () => mockUseFollowingChannels(),
}));

const followedChannel = {
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

describe("ViewerShell", () => {
  beforeEach(() => {
    mockUsePathname.mockReturnValue("/");
    mockUseAuth.mockReturnValue({
      user: { id: "viewer-1", displayName: "Viewer", email: "viewer@example.com", roles: ["member"] },
      loading: false,
    });
    mockUseFollowingChannels.mockReturnValue({
      channels: [followedChannel],
      status: "ready",
      reload: jest.fn(),
    });
  });

  it("does not render duplicate sidebar intro copy above the following rail", () => {
    render(
      <ViewerShell>
        <div>Page content</div>
      </ViewerShell>
    );

    expect(screen.queryByText(/keep an eye on the creators you already know/i)).not.toBeInTheDocument();
  });

  it.each(["/channels/chan-42", "/creator/live/chan-42"])(
    "keeps following side chrome off focused route %s",
    (route) => {
      mockUsePathname.mockReturnValue(route);

      const { container } = render(
        <ViewerShell>
          <div>Focused page content</div>
        </ViewerShell>
      );

      expect(screen.queryByRole("button", { name: /show following/i })).not.toBeInTheDocument();
      expect(screen.queryByTestId("following-sidebar")).not.toBeInTheDocument();
      expect(container.firstElementChild).toHaveClass("viewer-shell--following-disabled");
      expect(screen.getByText("Focused page content")).toBeInTheDocument();
    }
  );

  it.each([
    ["guest", { channels: [], status: "unauthenticated" }],
    ["empty", { channels: [], status: "empty" }],
  ])("keeps following side chrome out of discovery routes for %s state", (_label, followingState) => {
    if (followingState.status === "unauthenticated") {
      mockUseAuth.mockReturnValue({
        user: undefined,
        loading: false,
      });
    }
    mockUseFollowingChannels.mockReturnValue({
      ...followingState,
      reload: jest.fn(),
    });

    const { container } = render(
      <ViewerShell>
        <div>Discovery page content</div>
      </ViewerShell>
    );

    expect(screen.queryByRole("button", { name: /show following/i })).not.toBeInTheDocument();
    expect(screen.queryByTestId("following-sidebar")).not.toBeInTheDocument();
    expect(container.firstElementChild).toHaveClass("viewer-shell--following-disabled");
    expect(screen.getByText("Discovery page content")).toBeInTheDocument();
  });

  it("toggles the mobile following sidebar button state", () => {
    render(
      <ViewerShell>
        <div>Page content</div>
      </ViewerShell>
    );

    const toggle = screen.getByRole("button", { name: /show following/i });
    expect(toggle).toHaveAttribute("aria-expanded", "false");

    fireEvent.click(toggle);

    expect(toggle).toHaveAttribute("aria-expanded", "true");
  });

  it("moves focus into the sidebar and restores focus to toggle on close", () => {
    render(
      <ViewerShell>
        <div>Page content</div>
      </ViewerShell>
    );

    const toggle = screen.getByRole("button", { name: /show following/i });
    toggle.focus();

    fireEvent.click(toggle);

    const closeButton = screen.getByRole("button", { name: /close following sidebar/i });
    expect(closeButton).toHaveFocus();
    expect(document.body.style.overflow).toBe("hidden");

    fireEvent.click(closeButton);

    expect(toggle).toHaveFocus();
    expect(document.body.style.overflow).toBe("");
  });

  it("traps focus and closes via escape and backdrop", () => {
    render(
      <ViewerShell>
        <div>Page content</div>
      </ViewerShell>
    );

    const toggle = screen.getByRole("button", { name: /show following/i });
    fireEvent.click(toggle);

    const closeButton = screen.getByRole("button", { name: /close following sidebar/i });
    const sidebarAction = screen.getByRole("button", { name: /sidebar action/i });
    const sidebarLink = screen.getByRole("link", { name: /following link/i });

    closeButton.focus();
    fireEvent.keyDown(document, { key: "Tab", shiftKey: true });
    expect(sidebarLink).toHaveFocus();

    sidebarLink.focus();
    fireEvent.keyDown(document, { key: "Tab" });
    expect(closeButton).toHaveFocus();

    fireEvent.keyDown(document, { key: "Escape" });
    expect(toggle).toHaveFocus();

    fireEvent.click(toggle);
    expect(sidebarAction).toBeInTheDocument();

    const backdrop = document.querySelector(".viewer-shell__backdrop") as HTMLElement;
    fireEvent.click(backdrop);
    expect(toggle).toHaveFocus();
  });
});
