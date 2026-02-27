import "../test/test-utils";
import { render, screen, within } from "@testing-library/react";
import { DirectoryGrid } from "../components/DirectoryGrid";
import { LiveNowGrid } from "../components/LiveNowGrid";
import { ChannelRail } from "../components/ChannelRail";
import { FeaturedChannel } from "../components/FeaturedChannel";
import type { DirectoryChannel } from "../lib/viewer-api";

const liveChannel: DirectoryChannel = {
  channel: {
    id: "chan-live",
    ownerId: "owner-1",
    title: "Neon Nights",
    category: "Music",
    tags: ["synthwave", "retro"],
    liveState: "live",
    currentSessionId: "session-1",
    createdAt: new Date("2024-01-01T00:00:00.000Z").toISOString(),
    updatedAt: new Date("2024-01-01T00:00:00.000Z").toISOString(),
  },
  owner: {
    id: "owner-1",
    displayName: "DJ Nova",
  },
  profile: {
    bio: "Late-night electronic set.",
  },
  live: true,
  followerCount: 1,
  viewerCount: 27,
};

const offlineChannel: DirectoryChannel = {
  ...liveChannel,
  channel: {
    ...liveChannel.channel,
    id: "chan-offline",
    title: "Archive Sessions",
    liveState: "offline",
  },
  live: false,
  followerCount: 2,
  viewerCount: 0,
};

describe("channel display primitives", () => {
  test("renders consistent badge and count labels in directory and featured layouts", () => {
    const { asFragment } = render(<DirectoryGrid channels={[liveChannel, offlineChannel]} />);

    const liveCard = screen.getByRole("heading", { level: 3, name: "Neon Nights" }).closest("article");
    expect(liveCard).toBeTruthy();
    expect(within(liveCard!).getAllByText("Live").length).toBeGreaterThan(0);
    expect(within(liveCard!).getAllByText("27 viewers").length).toBeGreaterThan(0);
    expect(within(liveCard!).getByText("Followers: 1 follower")).toBeInTheDocument();

    const offlineCard = screen.getByRole("heading", { level: 3, name: "Archive Sessions" }).closest("article");
    expect(offlineCard).toBeTruthy();
    expect(within(offlineCard!).getAllByText("Offline").length).toBeGreaterThan(0);
    expect(within(offlineCard!).getAllByText("2 followers").length).toBeGreaterThan(0);

    expect(asFragment()).toMatchSnapshot();
  });

  test("uses shared viewer labels in live grids", () => {
    const { asFragment } = render(<LiveNowGrid channels={[liveChannel]} />);

    expect(screen.getByText("Live")).toBeInTheDocument();
    expect(screen.getByText("27 viewers")).toBeInTheDocument();

    expect(asFragment()).toMatchSnapshot();
  });

  test("uses shared badge copy in rails and featured cards", () => {
    const rail = render(<ChannelRail title="Live right now" channels={[liveChannel]} />);

    expect(screen.getByText("Live")).toBeInTheDocument();
    expect(rail.asFragment()).toMatchSnapshot();

    rail.unmount();

    const featured = render(<FeaturedChannel channels={[offlineChannel]} autoPlay={false} />);

    expect(screen.getByText("Offline")).toBeInTheDocument();
    expect(screen.getByText("2 followers")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "View featured channel" })).toHaveTextContent("View stream");
    expect(featured.asFragment()).toMatchSnapshot();
  });
});
