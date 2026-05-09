import "../test/test-utils";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { CategoryRail } from "../components/CategoryRail";
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



function buildFeaturedChannels(): DirectoryChannel[] {
  return [
    {
      ...liveChannel,
      channel: {
        ...liveChannel.channel,
        id: "chan-featured-1",
        title: "Neon Nights",
      },
      owner: {
        ...liveChannel.owner,
        id: "owner-featured-1",
        displayName: "DJ Nova",
      },
      profile: {
        ...liveChannel.profile,
        bannerUrl: "https://cdn.example.com/neon-nights.jpg",
      },
    },
    {
      ...offlineChannel,
      channel: {
        ...offlineChannel.channel,
        id: "chan-featured-2",
        title: "Archive Sessions",
      },
      owner: {
        ...offlineChannel.owner,
        id: "owner-featured-2",
        displayName: "Archive DJ",
      },
      profile: {
        ...offlineChannel.profile,
        bannerUrl: "https://cdn.example.com/archive-sessions.jpg",
      },
    },
  ];
}

type MatchMediaController = {
  setMatches: (nextValue: boolean) => void;
};

function mockReducedMotionPreference(initialMatches: boolean): MatchMediaController {
  let matches = initialMatches;
  const listeners = new Set<(event: MediaQueryListEvent) => void>();

  const matchMediaMock = jest.fn().mockImplementation((query: string) => ({
    matches,
    media: query,
    onchange: null,
    addEventListener: (_event: string, listener: (event: MediaQueryListEvent) => void) => {
      listeners.add(listener);
    },
    removeEventListener: (_event: string, listener: (event: MediaQueryListEvent) => void) => {
      listeners.delete(listener);
    },
    addListener: (listener: (event: MediaQueryListEvent) => void) => {
      listeners.add(listener);
    },
    removeListener: (listener: (event: MediaQueryListEvent) => void) => {
      listeners.delete(listener);
    },
    dispatchEvent: jest.fn(),
  }));

  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: matchMediaMock,
  });

  return {
    setMatches(nextValue: boolean) {
      matches = nextValue;
      const event = { matches: nextValue, media: "(prefers-reduced-motion: reduce)" } as MediaQueryListEvent;
      listeners.forEach((listener) => listener(event));
    },
  };
}

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


  test("applies lazy loading only to non-leading directory preview cards", () => {
    const directoryChannels = [
      {
        ...liveChannel,
        profile: {
          ...liveChannel.profile,
          bannerUrl: "https://cdn.example.com/neon-nights.jpg",
        },
      },
      {
        ...offlineChannel,
        profile: {
          ...offlineChannel.profile,
          bannerUrl: "https://cdn.example.com/archive-sessions.jpg",
        },
      },
    ];

    const { container } = render(<DirectoryGrid channels={directoryChannels} />);

    const previewImages = Array.from(container.querySelectorAll("img.directory-card__media"));
    expect(previewImages).toHaveLength(2);
    expect(previewImages[0]).not.toHaveAttribute("loading", "lazy");
    expect(previewImages[1]).toHaveAttribute("loading", "lazy");
  });

  test("applies lazy loading only to non-leading live-now preview cards", () => {
    const liveNowChannels = [
      {
        ...liveChannel,
        profile: {
          ...liveChannel.profile,
          bannerUrl: "https://cdn.example.com/neon-nights.jpg",
        },
      },
      {
        ...liveChannel,
        channel: {
          ...liveChannel.channel,
          id: "chan-live-2",
          title: "Pulse Hour",
        },
        owner: {
          ...liveChannel.owner,
          id: "owner-2",
          displayName: "VJ Pulse",
        },
        profile: {
          ...liveChannel.profile,
          bannerUrl: "https://cdn.example.com/pulse-hour.jpg",
        },
      },
    ];

    const { container } = render(<LiveNowGrid channels={liveNowChannels} />);

    const previewImages = Array.from(container.querySelectorAll("img.live-card__media-image"));
    expect(previewImages).toHaveLength(2);
    expect(previewImages[0]).not.toHaveAttribute("loading", "lazy");
    expect(previewImages[1]).toHaveAttribute("loading", "lazy");
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
    expect(screen.getByRole("link", { name: "View featured channel" })).toHaveTextContent("Watch channel");
    expect(featured.asFragment()).toMatchSnapshot();
  });

  test("starts with autoplay disabled when reduced motion is preferred", () => {
    mockReducedMotionPreference(true);
    jest.useFakeTimers();

    render(<FeaturedChannel channels={buildFeaturedChannels()} autoPlayIntervalMs={1000} />);

    expect(screen.getByRole("button", { name: "Resume autoplay" })).toHaveTextContent("Play");
    expect(screen.getByRole("heading", { level: 2, name: "Neon Nights" })).toBeInTheDocument();

    act(() => {
      jest.advanceTimersByTime(1200);
    });

    expect(screen.getByRole("heading", { level: 2, name: "Neon Nights" })).toBeInTheDocument();
    jest.useRealTimers();
  });

  test("manual play enables featured rotation when reduced motion starts enabled", () => {
    mockReducedMotionPreference(true);
    jest.useFakeTimers();

    render(<FeaturedChannel channels={buildFeaturedChannels()} autoPlayIntervalMs={1000} />);

    fireEvent.click(screen.getByRole("button", { name: "Resume autoplay" }));

    act(() => {
      jest.advanceTimersByTime(1200);
    });

    expect(screen.getByRole("heading", { level: 2, name: "Archive Sessions" })).toBeInTheDocument();
    jest.useRealTimers();
  });

  test("switching preference to reduced motion stops autoplay", () => {
    const preference = mockReducedMotionPreference(false);
    jest.useFakeTimers();

    render(<FeaturedChannel channels={buildFeaturedChannels()} autoPlayIntervalMs={1000} />);

    act(() => {
      jest.advanceTimersByTime(1200);
    });

    expect(screen.getByRole("heading", { level: 2, name: "Archive Sessions" })).toBeInTheDocument();

    act(() => {
      preference.setMatches(true);
    });

    expect(screen.getByRole("button", { name: "Resume autoplay" })).toHaveTextContent("Play");

    act(() => {
      jest.advanceTimersByTime(2500);
    });

    expect(screen.getByRole("heading", { level: 2, name: "Archive Sessions" })).toBeInTheDocument();
    jest.useRealTimers();
  });

  test("turns category chips into browse links", () => {
    render(<CategoryRail categories={[{ name: "Music", channelCount: 7 }]} />);

    expect(screen.getByRole("link", { name: /browse music channels/i })).toHaveAttribute(
      "href",
      "/browse?topic=Music",
    );
  });

  test("shows recovery actions when discovery sections are empty", () => {
    const view = render(<CategoryRail categories={[]} />);
    expect(screen.getByRole("link", { name: "Open full directory" })).toHaveAttribute("href", "/browse");

    view.rerender(<ChannelRail title="Live right now" channels={[]} />);
    expect(screen.getByRole("link", { name: "Browse full directory" })).toHaveAttribute("href", "/browse");

    view.rerender(<FeaturedChannel channels={[]} autoPlay={false} />);
    expect(screen.getByRole("link", { name: "Browse full directory" })).toHaveAttribute("href", "/browse");

    view.rerender(<LiveNowGrid channels={[]} />);
    expect(screen.getByRole("link", { name: "Browse full directory" })).toHaveAttribute("href", "/browse");

    view.rerender(<DirectoryGrid channels={[]} />);
    expect(screen.getByRole("link", { name: "Open creator setup" })).toHaveAttribute(
      "href",
      "/creator/getting-started",
    );
  });

});
