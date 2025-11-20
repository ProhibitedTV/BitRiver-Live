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

export type ManagedChannel = ChannelPublic & {
  streamKey: string;
  ingestEndpoints?: string[];
};

export type RenditionManifest = {
  name: string;
  manifestUrl: string;
  bitrate?: number;
};

export type StreamSession = {
  id: string;
  channelId: string;
  startedAt: string;
  endedAt?: string;
  renditions: string[];
  peakConcurrent: number;
  originUrl?: string;
  playbackUrl?: string;
  ingestEndpoints?: string[];
  ingestJobIds?: string[];
  renditionManifests?: RenditionManifest[];
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

export type FriendSummary = {
  userId: string;
  displayName: string;
  avatarUrl?: string;
};

export type ProfileView = {
  userId: string;
  displayName: string;
  bio?: string;
  avatarUrl?: string;
  bannerUrl?: string;
  featuredChannelId?: string;
  topFriends: FriendSummary[];
  donationAddresses: CryptoAddress[];
  channels: ChannelPublic[];
  liveChannels: ChannelPublic[];
  createdAt: string;
  updatedAt: string;
};

export type CryptoAddress = {
  currency: string;
  address: string;
  note?: string;
};

export type TipResponse = {
  id: string;
  channelId: string;
  fromUserId: string;
  amount: number;
  currency: string;
  provider: string;
  reference: string;
  walletAddress?: string;
  message?: string;
  createdAt: string;
};

export type CreateTipPayload = {
  amount: number;
  currency: string;
  provider?: string;
  reference: string;
  walletAddress?: string;
  message?: string;
};

export type DirectoryChannel = {
  channel: ChannelPublic;
  owner: ChannelOwner;
  profile: ProfileSummary;
  live: boolean;
  followerCount: number;
  viewerCount?: number;
};

export type DirectoryResponse = {
  channels: DirectoryChannel[];
  generatedAt: string;
};

export type CategorySummary = {
  name: string;
  channelCount: number;
  viewerCount?: number;
  thumbnailUrl?: string;
  tags?: string[];
};

export type CategoryDirectoryResponse = {
  categories: CategorySummary[];
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

export type SubscriptionState = {
  subscribers: number;
  subscribed: boolean;
  tier?: string;
  renewsAt?: string;
};

export type ChatUser = {
  id: string;
  displayName: string;
  role?: string;
  avatarUrl?: string;
};

export type ChatMessage = {
  id: string;
  message: string;
  sentAt: string;
  user?: ChatUser;
};

export type ChatMessageResponse = {
  id: string;
  channelId: string;
  userId: string;
  content: string;
  createdAt: string;
};

export type VodItem = {
  id: string;
  title: string;
  durationSeconds: number;
  publishedAt: string;
  thumbnailUrl?: string;
  playbackUrl?: string;
};

export type VodCollection = {
  channelId: string;
  items: VodItem[];
};

export type UploadItem = {
  id: string;
  channelId: string;
  title: string;
  filename: string;
  sizeBytes: number;
  status: string;
  progress: number;
  createdAt: string;
  updatedAt: string;
  recordingId?: string;
  playbackUrl?: string;
  error?: string;
};

export type CreateUploadPayload = {
  channelId: string;
  title?: string;
  filename?: string;
  sizeBytes?: number;
  playbackUrl?: string;
  metadata?: Record<string, string>;
};

export type UpdateChannelPayload = {
  title?: string;
  category?: string;
  tags?: string[];
};

type MultipartOptions = {
  file?: File | Blob;
  onProgress?: (percent: number) => void;
};

export type ChannelPlaybackResponse = {
  channel: ChannelPublic;
  owner: ChannelOwner;
  profile: ProfileSummary;
  donationAddresses: CryptoAddress[];
  live: boolean;
  follow: FollowState;
  subscription?: SubscriptionState;
  playback?: Playback;
  viewerCount?: number;
  chat?: {
    roomId: string;
  };
};

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";

async function viewerRequest<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers ?? {});
  if (!(init?.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    credentials: "include",
    headers,
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

function multipartRequest<T>(path: string, form: FormData, onProgress?: (percent: number) => void): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("POST", `${API_BASE}${path}`);
    xhr.withCredentials = true;
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          resolve(JSON.parse(xhr.responseText) as T);
        } catch (err) {
          reject(new Error("invalid server response"));
        }
        return;
      }
      reject(new Error(xhr.responseText || `${xhr.status}`));
    };
    xhr.onerror = () => {
      reject(new Error("upload failed"));
    };
    if (onProgress) {
      xhr.upload.onprogress = (event) => {
        if (!event.lengthComputable) {
          return;
        }
        const percent = Math.round((event.loaded / event.total) * 100);
        onProgress(percent);
      };
    }
    xhr.send(form);
  });
}

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

export function fetchProfile(userId: string): Promise<ProfileView> {
  return viewerRequest<ProfileView>(`/api/profiles/${userId}`);
}

export type UpdateProfilePayload = {
  displayName?: string;
  email?: string;
  bio?: string;
  avatarUrl?: string;
  bannerUrl?: string;
};

export function updateProfile(userId: string, payload: UpdateProfilePayload): Promise<ProfileView> {
  return viewerRequest<ProfileView>(`/api/profiles/${userId}`, {
    method: "PUT",
    body: JSON.stringify(payload),
  });
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

export function subscribeChannel(channelId: string): Promise<SubscriptionState> {
  return viewerRequest<SubscriptionState>(`/api/channels/${channelId}/subscribe`, {
    method: "POST"
  });
}

export function unsubscribeChannel(channelId: string): Promise<SubscriptionState> {
  return viewerRequest<SubscriptionState>(`/api/channels/${channelId}/subscribe`, {
    method: "DELETE"
  });
}

export function createTip(channelId: string, payload: CreateTipPayload): Promise<TipResponse> {
  return viewerRequest<TipResponse>(`/api/channels/${channelId}/monetization/tips`, {
    method: "POST",
    body: JSON.stringify({
      amount: payload.amount,
      currency: payload.currency,
      provider: payload.provider ?? "viewer",
      reference: payload.reference,
      walletAddress: payload.walletAddress,
      message: payload.message
    })
  });
}

function toChatMessage(response: ChatMessageResponse): ChatMessage {
  const normalizedUserId = response.userId.trim();
  const displayName = normalizedUserId.length > 0 ? normalizedUserId : response.userId || "Anonymous";
  const user = response.userId
    ? {
        id: response.userId,
        displayName,
      }
    : undefined;

  return {
    id: response.id,
    message: response.content,
    sentAt: response.createdAt,
    user,
  };
}

export function fetchChannelChat(channelId: string, limit = 50): Promise<ChatMessage[]> {
  const params = new URLSearchParams({ limit: `${limit}` });
  return viewerRequest<ChatMessageResponse[]>(
    `/api/channels/${channelId}/chat?${params.toString()}`
  ).then((messages) => messages.map(toChatMessage));
}

export function sendChatMessage(
  channelId: string,
  userId: string,
  content: string
): Promise<ChatMessage> {
  return viewerRequest<ChatMessageResponse>(`/api/channels/${channelId}/chat`, {
    method: "POST",
    body: JSON.stringify({ userId, content })
  }).then(toChatMessage);
}

export function fetchChannelVods(channelId: string): Promise<VodCollection> {
  return viewerRequest<VodCollection>(`/api/channels/${channelId}/vods`);
}

export function fetchChannelUploads(channelId: string): Promise<UploadItem[]> {
  return viewerRequest<UploadItem[]>(`/api/uploads?channelId=${encodeURIComponent(channelId)}`);
}

export function fetchChannelSessions(channelId: string): Promise<StreamSession[]> {
  return viewerRequest<StreamSession[]>(`/api/channels/${channelId}/sessions`);
}

export function fetchManagedChannels(ownerId?: string): Promise<ManagedChannel[]> {
  const suffix = ownerId ? `?ownerId=${ownerId}` : "";
  return viewerRequest<ManagedChannel[]>(`/api/channels${suffix}`);
}

export function updateChannel(channelId: string, payload: UpdateChannelPayload): Promise<ManagedChannel> {
  return viewerRequest<ManagedChannel>(`/api/channels/${channelId}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  });
}

export function createUpload(payload: CreateUploadPayload, options?: MultipartOptions): Promise<UploadItem> {
  if (options?.file) {
    const form = new FormData();
    form.append("channelId", payload.channelId);
    if (payload.title) {
      form.append("title", payload.title);
    }
    if (payload.filename) {
      form.append("filename", payload.filename);
    }
    if (payload.playbackUrl) {
      form.append("playbackUrl", payload.playbackUrl);
    }
    if (typeof payload.sizeBytes === "number" && !Number.isNaN(payload.sizeBytes)) {
      form.append("sizeBytes", `${payload.sizeBytes}`);
    }
    if (payload.metadata) {
      for (const [key, value] of Object.entries(payload.metadata)) {
        if (!key) {
          continue;
        }
        form.append(`metadata[${key}]`, value ?? "");
      }
    }
    const file = options.file;
    const filename = file instanceof File ? file.name : payload.filename ?? "upload.bin";
    form.append("file", file, filename);
    return multipartRequest<UploadItem>("/api/uploads", form, options.onProgress);
  }
  return viewerRequest<UploadItem>("/api/uploads", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export function deleteUpload(id: string): Promise<void> {
  return viewerRequest<void>(`/api/uploads/${id}`, {
    method: "DELETE",
  });
}
