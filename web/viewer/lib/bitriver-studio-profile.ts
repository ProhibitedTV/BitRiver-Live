/** Public, deliberately key-FREE transport profile for BitRiver Studio OS.
 * Creator credentials must never appear in this JSON download. */
export const STUDIO_OS_INGEST_PROFILE_VERSION = "bitriver.studio.ingest/v1" as const;

export type StudioOSIngestProfile = {
  schemaVersion: typeof STUDIO_OS_INGEST_PROFILE_VERSION;
  provider: "bitriver-live";
  channelId: string;
  rtmpServer: string;
  viewerUrl: string;
};

export function buildStudioOSIngestProfile(
  channelId: string,
  rtmpServer: string | undefined,
  viewerUrl: string
): StudioOSIngestProfile | null {
  if (!/^[a-fA-F0-9]{32}$/.test(channelId) || !rtmpServer) return null;
  try {
    const publish = new URL(rtmpServer);
    const page = new URL(viewerUrl);
    // Do not accidentally export a key-bearing OBS URL or an internal
    // controller URL. The SRS service expects a distinct stream key.
    if (!["rtmp:", "rtmps:"].includes(publish.protocol) ||
        !publish.hostname || publish.pathname !== "/live" ||
        publish.username || publish.password || publish.search || publish.hash ||
        /[|\[\]\s]/.test(rtmpServer)) return null;
    if (!["http:", "https:"].includes(page.protocol) || !page.hostname ||
        page.username || page.password || page.search || page.hash) return null;
    return {
      schemaVersion: STUDIO_OS_INGEST_PROFILE_VERSION,
      provider: "bitriver-live",
      channelId,
      rtmpServer,
      viewerUrl
    };
  } catch {
    return null;
  }
}
