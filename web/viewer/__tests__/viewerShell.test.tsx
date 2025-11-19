import { fireEvent, render, screen } from "@testing-library/react";
import { ViewerShell } from "../components/ViewerShell";

jest.mock("../components/FollowingSidebar", () => ({
  FollowingSidebar: () => <div data-testid="following-sidebar">Following</div>
}));

describe("ViewerShell", () => {
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
});
