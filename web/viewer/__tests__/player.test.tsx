import { act, render, screen, waitFor } from "@testing-library/react";
import { Player } from "../components/Player";

jest.mock("../lib/viewer-api", () => ({
  __esModule: true,
  reportViewerQoE: jest.fn()
}));

jest.mock("hls.js", () => ({
  __esModule: true,
  default: {
    isSupported: () => false,
    Events: {}
  }
}));

describe("Player", () => {
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
