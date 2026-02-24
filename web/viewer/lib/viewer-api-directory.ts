import { viewerRequest } from "./viewer-api-core";
import type {
  CategoryDirectoryResponse,
  ChannelPlaybackResponse,
  DirectoryResponse,
} from "./viewer-api-types";

export function fetchDirectory(): Promise<DirectoryResponse> {
  return viewerRequest<DirectoryResponse>("/api/directory");
}

export function fetchFeaturedChannels(): Promise<DirectoryResponse> {
  return viewerRequest<DirectoryResponse>("/api/directory/featured");
}

export function fetchRecommendedChannels(): Promise<DirectoryResponse> {
  return viewerRequest<DirectoryResponse>("/api/directory/recommended");
}

export function fetchFollowingChannels(): Promise<DirectoryResponse> {
  return viewerRequest<DirectoryResponse>("/api/directory/following");
}

export function fetchLiveNowChannels(): Promise<DirectoryResponse> {
  return viewerRequest<DirectoryResponse>("/api/directory/live");
}

export function fetchTrendingChannels(): Promise<DirectoryResponse> {
  return viewerRequest<DirectoryResponse>("/api/directory/trending");
}

export function fetchTopCategories(): Promise<CategoryDirectoryResponse> {
  return viewerRequest<CategoryDirectoryResponse>("/api/directory/categories");
}

export function fetchChannelPlayback(channelId: string): Promise<ChannelPlaybackResponse> {
  return viewerRequest<ChannelPlaybackResponse>(`/api/channels/${channelId}/playback`);
}

export function searchDirectory(query: string): Promise<DirectoryResponse> {
  const params = new URLSearchParams();
  if (query.trim().length > 0) {
    params.set("q", query.trim());
  }
  const suffix = params.toString();
  return viewerRequest<DirectoryResponse>(`/api/directory${suffix ? `?${suffix}` : ""}`);
}
