export type ChannelPublic = {
  id: string;
  ownerId: string;
  title: string;
  category?: string;
  tags: string[];
  schedule?: ChannelScheduleEntry[];
  liveState: string;
  currentSessionId?: string;
  createdAt: string;
  updatedAt: string;
};

export type ChannelScheduleEntry = {
  id: string;
  title: string;
  startsAt: string;
  durationMinutes?: number;
  description?: string;
  createdAt: string;
  updatedAt: string;
};

export type ChannelScheduleInput = {
  id?: string;
  title: string;
  startsAt: string;
  durationMinutes?: number;
  description?: string;
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
  socialLinks?: SocialLink[];
};

export type FriendSummary = {
  userId: string;
  displayName: string;
  avatarUrl?: string;
};

export type SocialLink = {
  platform: string;
  url: string;
};

export type ProfileView = {
  userId: string;
  displayName: string;
  bio?: string;
  avatarUrl?: string;
  bannerUrl?: string;
  socialLinks: SocialLink[];
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

export type ViewerQoEEvent = {
  channelId: string;
  sessionId?: string;
  event: string;
  player?: string;
  protocol?: string;
  latencyMode?: string;
  rendition?: string;
  playbackUrl?: string;
  currentTime?: number;
  duration?: number;
  bufferedSeconds?: number;
  droppedFrames?: number;
  error?: string;
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
  badges?: { id: string; label?: string }[];
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

export type ChatReportPayload = {
  targetId: string;
  reason: string;
  messageId?: string;
  evidenceUrl?: string;
};

export type ChatReport = {
  id: string;
  channelId: string;
  reporterId: string;
  targetId: string;
  reason: string;
  status: string;
  resolution?: string;
  messageId?: string;
  evidenceUrl?: string;
  createdAt: string;
  resolvedAt?: string;
  resolverId?: string;
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

export type Recording = {
  id: string;
  channelId: string;
  sessionId: string;
  title: string;
  durationSeconds: number;
  playbackBaseUrl?: string;
  publishedAt?: string;
  createdAt: string;
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
  schedule?: ChannelScheduleInput[];
};

export type CreateChannelPayload = {
  ownerId?: string;
  title: string;
  category?: string;
  tags?: string[];
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

export type UpdateProfilePayload = {
  displayName?: string;
  email?: string;
  bio?: string;
  avatarUrl?: string;
  bannerUrl?: string;
  socialLinks?: SocialLink[];
};

export type MultipartOptions = {
  file?: File | Blob;
  onProgress?: (progress: { percent: number; loadedBytes: number; totalBytes: number }) => void;
  signal?: AbortSignal;
};
