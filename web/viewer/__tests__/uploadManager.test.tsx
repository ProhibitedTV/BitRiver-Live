import {
  creatorUser,
  mockAuthenticatedUser,
  mockRouter,
  ownerUser,
  renderWithProviders,
  resetRouterMocks,
  viewerApiMocks,
  viewerUser,
} from "../test/test-utils";
import { screen, waitFor } from "@testing-library/react";
import { UploadManager } from "../components/UploadManager";

jest.mock("../hooks/useAuth");

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
  expect(screen.getByText(/this may take a few minutes/i)).toBeInTheDocument();
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
