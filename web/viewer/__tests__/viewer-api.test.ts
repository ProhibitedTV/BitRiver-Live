import { fetchChannelUploads, fetchDirectory, searchDirectory } from "../lib/viewer-api";

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

  it("uses the internal API origin for server-side relative requests when no public base is configured", async () => {
    const originalWindow = global.window;
    const originalApiBase = process.env.NEXT_PUBLIC_API_BASE_URL;

    // Simulate server-side rendering where relative fetch targets are invalid.
    Object.defineProperty(global, "window", {
      configurable: true,
      value: undefined,
    });
    process.env.NEXT_PUBLIC_API_BASE_URL = '""';

    try {
      await fetchDirectory();

      expect(fetchMock).toHaveBeenCalledWith(
        "http://bitriver-live:8080/api/directory",
        expect.objectContaining({ credentials: "include" }),
      );
    } finally {
      Object.defineProperty(global, "window", {
        configurable: true,
        value: originalWindow,
      });
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
