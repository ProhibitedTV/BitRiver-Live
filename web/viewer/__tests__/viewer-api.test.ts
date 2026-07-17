/** @jest-environment node */

import { fetchChannelUploads, fetchDirectory, publishRecording, reportChatMessage, searchDirectory } from "../lib/viewer-api";

describe("viewer api", () => {
  const originalFetch = global.fetch;
  let fetchMock: jest.Mock;

  beforeEach(() => {
    fetchMock = jest.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => [],
    });
    global.fetch = fetchMock as unknown as typeof fetch;
  });

  afterEach(() => {
    fetchMock.mockReset();
    global.fetch = originalFetch;
  });

  it("encodes channel IDs in upload requests", async () => {
    await fetchChannelUploads("channel/with spaces?");

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining("/api/uploads?channelId=channel%2Fwith%20spaces%3F"),
      expect.objectContaining({ credentials: "include" })
    );
  });

  it("encodes recording IDs when publishing recordings", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ id: "rec/with spaces?" }),
    });

    await publishRecording("rec/with spaces?");

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining("/api/recordings/rec%2Fwith%20spaces%3F/publish"),
      expect.objectContaining({ method: "POST", credentials: "include" })
    );
  });

  it("submits chat reports with message context", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      status: 202,
      json: async () => ({ id: "report-1" }),
    });

    await reportChatMessage("channel-1", {
      targetId: "viewer-2",
      messageId: "message-1",
      reason: "Harassment",
    });

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining("/api/channels/channel-1/chat/reports"),
      expect.objectContaining({
        method: "POST",
        credentials: "include",
        body: JSON.stringify({
          targetId: "viewer-2",
          messageId: "message-1",
          reason: "Harassment",
        }),
      })
    );
  });

  it("uses the internal API origin for server-side relative requests when no public base is configured", async () => {
    const originalApiBase = process.env.NEXT_PUBLIC_API_BASE_URL;

    process.env.NEXT_PUBLIC_API_BASE_URL = '""';

    try {
      await fetchDirectory();

      expect(fetchMock).toHaveBeenCalledWith(
        "http://bitriver-live:8080/api/directory",
        expect.objectContaining({ credentials: "include" }),
      );
    } finally {
      if (originalApiBase === undefined) {
        delete process.env.NEXT_PUBLIC_API_BASE_URL;
      } else {
        process.env.NEXT_PUBLIC_API_BASE_URL = originalApiBase;
      }
    }
  });

  it("encodes exact category filters in directory requests", async () => {
    await fetchDirectory("Science & Tech");

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining("/api/directory?category=Science+%26+Tech"),
      expect.objectContaining({ credentials: "include" })
    );
  });

  it("preserves category filters when searching the directory", async () => {
    await searchDirectory("retro", "Science & Tech");

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining("/api/directory?q=retro&category=Science+%26+Tech"),
      expect.objectContaining({ credentials: "include" })
    );
  });
});
