import { buildViewerPath, buildViewerUrl } from "../lib/viewer-links";

describe("viewer link helpers", () => {
  it("prefixes viewer paths with the configured basePath", () => {
    expect(buildViewerPath("/channels/channel-1", "/viewer")).toBe("/viewer/channels/channel-1");
    expect(buildViewerPath("/browse", "")).toBe("/browse");
  });

  it("builds absolute viewer URLs with the configured basePath", () => {
    expect(buildViewerUrl("/channels/channel-1", "https://viewer.example.com", "/viewer")).toBe(
      "https://viewer.example.com/viewer/channels/channel-1",
    );
  });
});
