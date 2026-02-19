import { render, screen, waitFor } from "@testing-library/react";
import { Player } from "../components/Player";

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
});
