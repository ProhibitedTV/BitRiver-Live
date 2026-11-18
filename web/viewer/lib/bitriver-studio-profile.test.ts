import { buildStudioOSIngestProfile } from "./bitriver-studio-profile";

const channel = "0123456789abcdef0123456789abcdef";
const viewer = "https://watch.example.test/viewer/channels/" + channel;

describe("BitRiver Studio OS export", () => {
  it("exports interoperable, key-free RTMP metadata", () => {
    const result = buildStudioOSIngestProfile(channel, "rtmp://ingest.example.test:1935/live", viewer);
    expect(result).toEqual({
      schemaVersion: "bitriver.studio.ingest/v1",
      provider: "bitriver-live",
      channelId: channel,
      rtmpServer: "rtmp://ingest.example.test:1935/live",
      viewerUrl: viewer
    });
    expect(JSON.stringify(result)).not.toMatch(/streamKey|password|token/);
  });

  it.each([
    "rtmp://ingest.example.test:1935/live/PRIVATEKEY",
    "rtmp://u:p@ingest.example.test:1935/live",
    "rtmp://ingest.example.test/live?key=private",
    "https://ingest.example.test/live",
    "rtmp://ingest.example.test/live|another-target",
    "rtmp://ingest.example.test"
  ])("rejects non-canonical or credential-bearing ingest %s", (endpoint) => {
    expect(buildStudioOSIngestProfile(channel, endpoint, viewer)).toBeNull();
  });

  it("rejects unsafe channel and viewer URLs", () => {
    expect(buildStudioOSIngestProfile("../channel", "rtmp://ingest.example.test/live", viewer)).toBeNull();
    expect(buildStudioOSIngestProfile(channel, "rtmp://ingest.example.test/live", "https://alice:secret@watch.example.test/")).toBeNull();
    expect(buildStudioOSIngestProfile(channel, "rtmp://ingest.example.test/live", "https://watch.example.test/viewer?session=secret")).toBeNull();
  });
});
