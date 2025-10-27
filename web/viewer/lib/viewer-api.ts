export type ChannelPublic = {
  id: string;
  ownerId: string;
  title: string;
  category?: string;
  tags: string[];
  liveState: string;
  currentSessionId?: string;
  createdAt: string;
  updatedAt: string;
};

export type ChannelOwner = {
  id: string;
  displayName: string;
  avatarUrl?: string;
};

export type ProfileSummary = {
  bio?: string;
  avatarUrl?: string;
  bannerUrl?: string;
};

export type DirectoryChannel = {
  channel: ChannelPublic;
  owner: ChannelOwner;
  profile: ProfileSummary;
  live: boolean;
  followerCount: number;
};

export type DirectoryResponse = {
  channels: DirectoryChannel[];
  generatedAt: string;
};

export type Rendition = {
  name: string;
  manifestUrl: string;
  bitrate?: number;
};

export type Playback = {
  sessionId: string;
  startedAt: string;
  playbackUrl?: string;
  originUrl?: string;
  protocol?: string;
  playerHint?: string;
  latencyMode?: string;
  renditions?: Rendition[];
};

export type FollowState = {
  followers: number;
  following: boolean;
};

export type ChannelPlaybackResponse = {
  channel: ChannelPublic;
  owner: ChannelOwner;
  profile: ProfileSummary;
  live: boolean;
  follow: FollowState;
  playback?: Playback;
};

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";

async function viewerRequest<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {})
    },
    cache: "no-store"
  });
  if (!response.ok) {
    const detail = await response.text();
    throw new Error(detail || `${response.status}`);
  }
  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
}

export function fetchDirectory(): Promise<DirectoryResponse> {
  return viewerRequest<DirectoryResponse>("/api/directory");
}

export function fetchChannelPlayback(channelId: string): Promise<ChannelPlaybackResponse> {
  return viewerRequest<ChannelPlaybackResponse>(`/api/channels/${channelId}/playback`);
}

export function followChannel(channelId: string): Promise<FollowState> {
  return viewerRequest<FollowState>(`/api/channels/${channelId}/follow`, {
    method: "POST"
  });
}

export function unfollowChannel(channelId: string): Promise<FollowState> {
  return viewerRequest<FollowState>(`/api/channels/${channelId}/follow`, {
    method: "DELETE"
  });
}
