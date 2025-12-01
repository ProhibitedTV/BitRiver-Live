import { render, screen, waitFor } from "@testing-library/react";
import { UploadManager } from "../components/UploadManager";
import { useAuth } from "../hooks/useAuth";
import { fetchChannelUploads } from "../lib/viewer-api";

const replaceMock = jest.fn();

jest.mock("../hooks/useAuth");

jest.mock("next/navigation", () => ({
  useRouter: () => ({ replace: replaceMock }),
}));

jest.mock("../lib/viewer-api", () => ({
  ...jest.requireActual("../lib/viewer-api"),
  fetchChannelUploads: jest.fn(),
  createUpload: jest.fn(),
  deleteUpload: jest.fn(),
}));
const mockUseAuth = useAuth as jest.MockedFunction<typeof useAuth>;
const fetchUploadsMock = fetchChannelUploads as jest.MockedFunction<typeof fetchChannelUploads>;

beforeEach(() => {
  jest.clearAllMocks();
  replaceMock.mockReset();
});

test("loads uploads when the viewer owns the channel", async () => {
  mockUseAuth.mockReturnValue({
    user: { id: "owner-1", displayName: "Owner", email: "owner@example.com", roles: ["creator"] },
    loading: false,
    error: undefined,
    signIn: jest.fn(),
    signOut: jest.fn(),
  });
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

  render(<UploadManager channelId="chan-1" ownerId="owner-1" />);

  await waitFor(() => expect(fetchUploadsMock).toHaveBeenCalledWith("chan-1"));
  expect(await screen.findByRole("heading", { name: /upload manager/i })).toBeInTheDocument();
  expect(screen.getByText(/recap/i)).toBeInTheDocument();
  expect(replaceMock).not.toHaveBeenCalled();
});

test("redirects viewers who lack permission", async () => {
  mockUseAuth.mockReturnValue({
    user: { id: "viewer-1", displayName: "Viewer", email: "viewer@example.com", roles: [] },
    loading: false,
    error: undefined,
    signIn: jest.fn(),
    signOut: jest.fn(),
  });

  render(<UploadManager channelId="chan-1" ownerId="owner-2" />);

  await waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/channels/chan-1"));
  expect(fetchUploadsMock).not.toHaveBeenCalled();
  expect(screen.queryByText(/upload manager/i)).not.toBeInTheDocument();
});

test("allows creator role to manage uploads", async () => {
  mockUseAuth.mockReturnValue({
    user: { id: "creator-1", displayName: "Creator", email: "creator@example.com", roles: ["creator"] },
    loading: false,
    error: undefined,
    signIn: jest.fn(),
    signOut: jest.fn(),
  });
  fetchUploadsMock.mockResolvedValue([]);

  render(<UploadManager channelId="chan-99" ownerId="owner-2" />);

  await waitFor(() => expect(fetchUploadsMock).toHaveBeenCalledWith("chan-99"));
  expect(screen.getByRole("heading", { name: /upload manager/i })).toBeInTheDocument();
  expect(replaceMock).not.toHaveBeenCalled();
});
