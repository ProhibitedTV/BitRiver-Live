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
  expect(mockRouter.replace).not.toHaveBeenCalled();
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
