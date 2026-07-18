import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import Hls from "hls.js";
import { Player } from "../components/Player";

jest.mock("../lib/viewer-api", () => ({
  __esModule: true,
  reportViewerQoE: jest.fn()
}));

jest.mock("next/link", () => ({
  __esModule: true,
  default: ({ children, href, prefetch, ...props }: any) => (
    <a href={typeof href === "string" ? href : href?.pathname ?? "#"} {...props}>
      {children}
    </a>
  )
}));

jest.mock("hls.js", () => ({
  __esModule: true,
  default: (() => {
    const loadSource = jest.fn();
    const attachMedia = jest.fn();
    const destroy = jest.fn();
    const on = jest.fn();
    const constructor = jest.fn(() => ({ loadSource, attachMedia, destroy, on, levels: [] }));
    return Object.assign(constructor, {
      supported: false,
      loadSource,
      attachMedia,
      destroy,
      on,
      isSupported: () => constructor.supported,
      Events: { LEVEL_SWITCHED: "levelSwitched", ERROR: "error" }
    });
  })()
}));

const mockHlsConstructor = Hls as unknown as jest.Mock & {
  supported: boolean;
  loadSource: jest.Mock;
  attachMedia: jest.Mock;
  destroy: jest.Mock;
  on: jest.Mock;
};

describe("Player", () => {
	beforeEach(() => {
		mockHlsConstructor.supported = false;
		mockHlsConstructor.mockClear();
		mockHlsConstructor.loadSource.mockClear();
		mockHlsConstructor.attachMedia.mockClear();
		mockHlsConstructor.destroy.mockClear();
		mockHlsConstructor.on.mockClear();
	});

  test("renders loading state copy", () => {
    render(<Player channelId="chan-1" loading />);

    expect(screen.getByRole("heading", { name: "Loading stream" })).toBeInTheDocument();
    expect(screen.getByText(/preparing the player/i)).toBeInTheDocument();
  });

  test("renders stream starting soon when channel is live without playback details", () => {
    render(<Player channelId="chan-1" live />);

    expect(screen.getByRole("heading", { name: "Stream starting soon" })).toBeInTheDocument();
  });

  test("renders stream ended when channel is offline", () => {
    render(<Player channelId="chan-1" />);

    expect(screen.getByRole("heading", { name: "Stream ended" })).toBeInTheDocument();
    expect(screen.getByText("Offline")).toBeInTheDocument();
  });

  test("renders stream unavailable when playback URL is missing", async () => {
    render(
      <Player
        channelId="chan-1"
        playback={{
          sessionId: "session-1",
          startedAt: new Date().toISOString(),
          protocol: "hls"
        }}
      />
    );

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Stream unavailable" })).toBeInTheDocument();
    });
  });

  test("offers retry and browse recovery when playback is unavailable", async () => {
    const user = userEvent.setup();
    const onRetry = jest.fn();

    render(
      <Player
        channelId="chan-1"
        onRetry={onRetry}
        recoveryHref="/browse"
        playback={{
          sessionId: "session-1",
          startedAt: new Date().toISOString(),
          protocol: "hls"
        }}
      />
    );

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Stream unavailable" })).toBeInTheDocument();
    });

    const retryButton = screen.getByRole("button", { name: "Retry playback" });
    expect(screen.getByRole("link", { name: "Browse live channels" })).toHaveAttribute("href", "/browse");

    await user.click(retryButton);

    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  test("offers a live check action when a stream has ended", async () => {
    const user = userEvent.setup();
    const onRetry = jest.fn();

    render(<Player channelId="chan-1" onRetry={onRetry} recoveryHref="/browse" />);

    expect(screen.getByRole("heading", { name: "Stream ended" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Check for live stream" }));

    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  test("renders playback when a recovered source replaces an unavailable one", async () => {
    const { container, rerender } = render(
      <Player
        channelId="chan-1"
        playback={{
          sessionId: "session-1",
          startedAt: new Date().toISOString(),
          protocol: "hls"
        }}
      />
    );

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Stream unavailable" })).toBeInTheDocument();
    });

    rerender(
      <Player
        channelId="chan-1"
        playback={{
          sessionId: "session-1",
          startedAt: new Date().toISOString(),
          protocol: "hls",
          playbackUrl: "https://example.test/live.m3u8"
        }}
      />
    );

    await waitFor(() => {
      expect(container.querySelector("video")).toBeInTheDocument();
    });
    expect(screen.queryByRole("heading", { name: "Stream unavailable" })).not.toBeInTheDocument();
  });

  test("prefers hls.js over an unreliable native HLS capability signal", async () => {
    mockHlsConstructor.supported = true;
    const nativeCapability = jest.spyOn(HTMLMediaElement.prototype, "canPlayType").mockReturnValue("maybe");

    render(
      <Player
        channelId="chan-1"
        playback={{
          sessionId: "session-1",
          startedAt: new Date().toISOString(),
          protocol: "ll-hls",
          latencyMode: "low-latency",
          playbackUrl: "https://stream.example.test/live/channel/llhls.m3u8"
        }}
      />
    );

    await waitFor(() => {
      expect(mockHlsConstructor.loadSource).toHaveBeenCalledWith(
        "https://stream.example.test/live/channel/llhls.m3u8"
      );
    });
    expect(mockHlsConstructor).toHaveBeenCalledWith({ lowLatencyMode: true });
    expect(mockHlsConstructor.attachMedia).toHaveBeenCalledWith(expect.any(HTMLVideoElement));

    nativeCapability.mockRestore();
  });

  test("delays stream unavailable to allow reconnect attempts", async () => {
    jest.useFakeTimers();
    const { container } = render(
      <Player
        channelId="chan-1"
        playback={{
          sessionId: "session-1",
          startedAt: new Date().toISOString(),
          protocol: "hls",
          playbackUrl: "https://example.test/live.m3u8"
        }}
      />
    );

    const video = container.querySelector("video");
    expect(video).not.toBeNull();

    act(() => {
      video?.dispatchEvent(new Event("error"));
    });

    expect(screen.getByText("Reconnecting")).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Stream unavailable" })).not.toBeInTheDocument();

    act(() => {
      jest.advanceTimersByTime(3500);
    });

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Stream unavailable" })).toBeInTheDocument();
    });

    jest.useRealTimers();
  });
});
