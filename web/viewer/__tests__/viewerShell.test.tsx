import { fireEvent, render, screen } from "@testing-library/react";
import { ViewerShell } from "../components/ViewerShell";

jest.mock("../components/FollowingSidebar", () => ({
  FollowingSidebar: () => (
    <div data-testid="following-sidebar">
      <button type="button">Sidebar action</button>
      <a href="/following">Following link</a>
    </div>
  )
}));

describe("ViewerShell", () => {
  it("does not render duplicate sidebar intro copy above the following rail", () => {
    render(
      <ViewerShell>
        <div>Page content</div>
      </ViewerShell>
    );

    expect(screen.queryByText(/keep an eye on the creators you already know/i)).not.toBeInTheDocument();
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
