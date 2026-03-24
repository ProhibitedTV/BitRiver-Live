import { viewerRequest } from "./viewer-api-core";
import type {
  CategoryDirectoryResponse,
  ChannelPlaybackResponse,
  DirectoryResponse,
} from "./viewer-api-types";

function buildDirectorySuffix(query?: string, category?: string): string {
  const params = new URLSearchParams();

  const normalizedQuery = query?.trim();
  if (normalizedQuery) {
    params.set("q", normalizedQuery);
  }

  const normalizedCategory = category?.trim();
  if (normalizedCategory) {
    params.set("category", normalizedCategory);
  }

  const suffix = params.toString();
  return suffix ? `?${suffix}` : "";
}

export function fetchDirectory(category?: string): Promise<DirectoryResponse> {
  return viewerRequest<DirectoryResponse>(`/api/directory${buildDirectorySuffix(undefined, category)}`);
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

export function searchDirectory(query: string, category?: string): Promise<DirectoryResponse> {
  return viewerRequest<DirectoryResponse>(`/api/directory${buildDirectorySuffix(query, category)}`);
}
