import { fetchChannelUploads } from "../lib/viewer-api";

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
});
