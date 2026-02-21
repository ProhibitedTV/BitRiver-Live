import {
  creatorUser,
  mockAuthenticatedUser,
  mockAnonymousUser,
  mockRouter,
  mockUseAuth,
  ownerUser,
  renderWithProviders,
  resetRouterMocks,
  viewerApiMocks,
  viewerUser,
} from "../test/test-utils";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { UploadManager } from "../components/UploadManager";

jest.mock("../hooks/useAuth");

const createUploadMock = viewerApiMocks.createUpload;
const fetchUploadsMock = viewerApiMocks.fetchChannelUploads;

beforeEach(() => {
  jest.clearAllMocks();
  resetRouterMocks();
});

test("loads uploads when the viewer owns the channel", async () => {
  mockAuthenticatedUser(ownerUser);
  fetchUploadsMock.mockResolvedValue([
    {
      id: "upload-1",
      channelId: "chan-1",
      title: "Recap",
      filename: "recap.mp4",
      sizeBytes: 1_000_000,
      status: "processing",
      progress: 50,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    },
  ] as any);

  renderWithProviders(<UploadManager channelId="chan-1" ownerId="owner-1" />);

  await waitFor(() => expect(fetchUploadsMock).toHaveBeenCalledWith("chan-1"));
  expect(await screen.findByRole("heading", { name: /upload manager/i })).toBeInTheDocument();
  expect(screen.getByText(/recap/i)).toBeInTheDocument();
  expect(screen.getByText("Processing")).toBeInTheDocument();
  expect(screen.getByText(/we are transcoding and packaging this recording for playback/i)).toBeInTheDocument();
  expect(screen.getByText(/Last updated:/i)).toBeInTheDocument();
  expect(screen.queryByText(/^State:/i)).not.toBeInTheDocument();
  expect(mockRouter.replace).not.toHaveBeenCalled();
});



test("shows watch as the primary card action when playback is available", async () => {
  mockAuthenticatedUser(ownerUser);
  fetchUploadsMock.mockResolvedValue([
    {
      id: "upload-2",
      channelId: "chan-1",
      title: "Published recap",
      filename: "published.mp4",
      playbackUrl: "https://vod.example.com/recap",
      sizeBytes: 2_000_000,
      status: "completed",
      progress: 100,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    },
  ] as any);

  renderWithProviders(<UploadManager channelId="chan-1" ownerId="owner-1" />);

  const watchLink = await screen.findByRole("link", { name: "Watch" });
  expect(watchLink).toHaveAttribute("href", "https://vod.example.com/recap");
  expect(watchLink).toHaveAttribute("target", "_blank");
  expect(watchLink).toHaveAttribute("rel", "noreferrer");
  expect(screen.getByRole("button", { name: "Delete" })).toBeInTheDocument();
});



test("shows Open VOD as the primary action when playbackUrl is available", async () => {
  mockAuthenticatedUser(ownerUser);
  fetchUploadsMock.mockResolvedValue([
    {
      id: "upload-ready-vod",
      channelId: "chan-1",
      title: "Ready VOD",
      filename: "ready-vod.mp4",
      playbackUrl: "https://vod.example.com/library/ready-vod",
      recordingId: "rec-100",
      sizeBytes: 3_000_000,
      status: "ready",
      progress: 100,
      createdAt: "2026-01-02T03:04:05.000Z",
      updatedAt: "2026-01-02T03:14:05.000Z",
    },
  ] as any);

  renderWithProviders(<UploadManager channelId="chan-1" ownerId="owner-1" />);

  const primaryAction = await screen.findByRole("link", { name: /watch|open vod/i });
  expect(primaryAction).toHaveAttribute("href", "https://vod.example.com/library/ready-vod");
  expect(primaryAction).toHaveAttribute("target", "_blank");
  expect(primaryAction).toHaveAttribute("rel", "noreferrer");
  expect(screen.queryByRole("link", { name: /view recording/i })).not.toBeInTheDocument();
  expect(screen.queryByText(/playback pending/i)).not.toBeInTheDocument();
});

test("shows View recording when a recording route template exists and playbackUrl is missing", async () => {
  const originalTemplate = process.env.NEXT_PUBLIC_RECORDING_DETAIL_ROUTE_TEMPLATE;
  process.env.NEXT_PUBLIC_RECORDING_DETAIL_ROUTE_TEMPLATE = "/creator/recordings/[id]";

  mockAuthenticatedUser(ownerUser);
  fetchUploadsMock.mockResolvedValue([
    {
      id: "upload-recording-route",
      channelId: "chan-1",
      title: "Recording pending playback",
      filename: "recording.mp4",
      recordingId: "rec-abc",
      sizeBytes: 2_500_000,
      status: "ready",
      progress: 100,
      createdAt: "2026-01-02T03:04:05.000Z",
      updatedAt: "2026-01-02T03:24:05.000Z",
    },
  ] as any);

  renderWithProviders(<UploadManager channelId="chan-1" ownerId="owner-1" />);

  const viewRecordingLink = await screen.findByRole("link", { name: "View recording" });
  expect(viewRecordingLink).toHaveAttribute("href", "/creator/recordings/rec-abc");
  expect(screen.queryByText(/playback pending/i)).not.toBeInTheDocument();

  if (originalTemplate === undefined) {
    delete process.env.NEXT_PUBLIC_RECORDING_DETAIL_ROUTE_TEMPLATE;
  } else {
    process.env.NEXT_PUBLIC_RECORDING_DETAIL_ROUTE_TEMPLATE = originalTemplate;
  }
});

test("shows Playback pending when recordingId exists but no recording route template is configured", async () => {
  const originalTemplate = process.env.NEXT_PUBLIC_RECORDING_DETAIL_ROUTE_TEMPLATE;
  delete process.env.NEXT_PUBLIC_RECORDING_DETAIL_ROUTE_TEMPLATE;

  mockAuthenticatedUser(ownerUser);
  fetchUploadsMock.mockResolvedValue([
    {
      id: "upload-recording-pending",
      channelId: "chan-1",
      title: "Recording waiting for route",
      filename: "recording-waiting.mp4",
      recordingId: "rec-no-route",
      sizeBytes: 2_500_000,
      status: "ready",
      progress: 100,
      createdAt: "2026-01-02T03:04:05.000Z",
      updatedAt: "2026-01-02T03:24:05.000Z",
    },
  ] as any);

  renderWithProviders(<UploadManager channelId="chan-1" ownerId="owner-1" />);

  expect(await screen.findByText(/playback pending\. check back soon to watch this recording\./i)).toBeInTheDocument();
  expect(screen.queryByRole("link", { name: "View recording" })).not.toBeInTheDocument();

  if (originalTemplate === undefined) {
    delete process.env.NEXT_PUBLIC_RECORDING_DETAIL_ROUTE_TEMPLATE;
  } else {
    process.env.NEXT_PUBLIC_RECORDING_DETAIL_ROUTE_TEMPLATE = originalTemplate;
  }
});

test("shows processing progress, transcoding/packaging explanation, and last updated timestamp", async () => {
  mockAuthenticatedUser(ownerUser);
  fetchUploadsMock.mockResolvedValue([
    {
      id: "upload-processing",
      channelId: "chan-1",
      title: "Processing upload",
      filename: "processing.mp4",
      recordingId: "rec-processing",
      sizeBytes: 4_000_000,
      status: "processing",
      progress: 67,
      createdAt: "2026-01-02T03:04:05.000Z",
      updatedAt: "2026-01-02T04:04:05.000Z",
    },
  ] as any);

  renderWithProviders(<UploadManager channelId="chan-1" ownerId="owner-1" />);

  expect(await screen.findByText(/processing… 67% complete\./i)).toBeInTheDocument();
  expect(screen.getByText(/we are transcoding and packaging this recording for playback\./i)).toBeInTheDocument();
  expect(screen.getByText(/^Last updated:/i)).toBeInTheDocument();
});



test("cancels an in-progress upload and returns to selecting state", async () => {
  mockAuthenticatedUser(ownerUser);
  fetchUploadsMock.mockResolvedValue([]);

  let capturedSignal: AbortSignal | undefined;
  createUploadMock.mockImplementation(
    async (_payload, options?: { signal?: AbortSignal }) =>
      await new Promise((resolve, reject) => {
        capturedSignal = options?.signal;
        capturedSignal?.addEventListener("abort", () => reject(new DOMException("The operation was aborted", "AbortError")));
      }),
  );

  renderWithProviders(<UploadManager channelId="chan-1" ownerId="owner-1" />);

  await screen.findByRole("heading", { name: /upload manager/i });

  const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement;
  const file = new File(["video-data"], "demo.mp4", { type: "video/mp4" });
  fireEvent.change(fileInput, { target: { files: [file] } });

  fireEvent.click(screen.getByRole("button", { name: "Register upload" }));

  expect(await screen.findByRole("button", { name: "Cancel upload" })).toBeInTheDocument();
  expect(screen.getByText(/0 b\s*\/\s*10 b\s*·\s*0%/i)).toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: "Cancel upload" }));

  await waitFor(() => expect(capturedSignal?.aborted).toBe(true));
  await waitFor(() => expect(screen.queryByRole("button", { name: "Cancel upload" })).not.toBeInTheDocument());
  expect(screen.queryByText(/\/\s*10 b\s*·\s*0%/i)).not.toBeInTheDocument();
  expect(screen.getByText("Select media to start a new upload.")).toBeInTheDocument();
});

test("shows reset after upload failure and clears failure state", async () => {
  mockAuthenticatedUser(ownerUser);
  fetchUploadsMock.mockResolvedValue([]);
  createUploadMock.mockRejectedValue(new Error("upload failed"));

  renderWithProviders(<UploadManager channelId="chan-1" ownerId="owner-1" />);

  await screen.findByRole("heading", { name: /upload manager/i });

  const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement;
  const file = new File(["video-data"], "failed.mp4", { type: "video/mp4" });
  fireEvent.change(fileInput, { target: { files: [file] } });

  fireEvent.click(screen.getByRole("button", { name: "Register upload" }));

  expect(await screen.findByRole("button", { name: "Reset" })).toBeInTheDocument();
  expect(screen.getByText(/upload failed/i)).toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: "Reset" }));

  await waitFor(() => expect(screen.queryByRole("button", { name: "Reset" })).not.toBeInTheDocument());
  expect(screen.getByText("Select media to start a new upload.")).toBeInTheDocument();
  expect(screen.queryByText(/upload failed/i)).not.toBeInTheDocument();
});

test("prompts guests to sign in and preserves the uploads path", async () => {
  jest.useFakeTimers();
  mockAnonymousUser();

  renderWithProviders(<UploadManager channelId="chan-1" ownerId="owner-1" />);

  expect(await screen.findByText(/sign in to manage uploads/i)).toBeInTheDocument();

  const signIn = mockUseAuth.mock.results.at(-1)?.value.signIn as jest.Mock;
  jest.advanceTimersByTime(500);
  await waitFor(() => expect(signIn).toHaveBeenCalledWith());

  jest.useRealTimers();
});

test("redirects viewers who lack permission", async () => {
  mockAuthenticatedUser(viewerUser);

  renderWithProviders(<UploadManager channelId="chan-1" ownerId="owner-2" />);

  await waitFor(() => expect(mockRouter.replace).toHaveBeenCalledWith("/channels/chan-1"));
  expect(fetchUploadsMock).not.toHaveBeenCalled();
  expect(screen.queryByText(/upload manager/i)).not.toBeInTheDocument();
});

test("allows creator role to manage uploads", async () => {
  mockAuthenticatedUser(creatorUser);
  fetchUploadsMock.mockResolvedValue([]);

  renderWithProviders(<UploadManager channelId="chan-99" ownerId="owner-2" />);

  await waitFor(() => expect(fetchUploadsMock).toHaveBeenCalledWith("chan-99"));
  expect(screen.getByRole("heading", { name: /upload manager/i })).toBeInTheDocument();
  expect(mockRouter.replace).not.toHaveBeenCalled();
});
