import type { ChannelPlaybackResponse, ChatMessageResponse } from "../../lib/viewer-api";

export const channelId = "chan-hls";

export const authenticatedViewer = {
  user: {
    id: "viewer-5",
    displayName: "Mission Specialist",
    email: "viewer5@example.com",
    roles: ["member"]
  },
  loginUrl: "https://auth.example.com/login",
  logoutUrl: "https://auth.example.com/logout"
};

export const playbackResponse: ChannelPlaybackResponse = {
  channel: {
    id: channelId,
    ownerId: "owner-200",
    title: "Orbital Maintenance",
    category: "Science & Tech",
    tags: ["iss", "maintenance"],
    liveState: "live",
    currentSessionId: "session-200",
    createdAt: new Date("2024-03-20T11:00:00Z").toISOString(),
    updatedAt: new Date("2024-03-20T11:30:00Z").toISOString()
  },
  owner: {
    id: "owner-200",
    displayName: "Station Lead"
  },
  profile: {
    bio: "Live walk-throughs from orbit.",
    avatarUrl: undefined,
    bannerUrl: undefined,
    socialLinks: []
  },
  live: true,
  follow: {
    followers: 87,
    following: true
  },
  donationAddresses: [],
  subscription: {
    subscribers: 14,
    subscribed: true,
    tier: "Crew"
  },
  playback: {
    sessionId: "session-200",
    startedAt: new Date("2024-03-20T11:00:00Z").toISOString(),
    playbackUrl: "https://cdn.example.com/hls/orbit.m3u8",
    originUrl: "https://cdn.example.com/thumbs/orbit.jpg",
    protocol: "hls",
    latencyMode: "low-latency",
    renditions: [
      { name: "1080p", manifestUrl: "https://cdn.example.com/hls/orbit-1080p.m3u8", bitrate: 4200 },
      { name: "720p", manifestUrl: "https://cdn.example.com/hls/orbit-720p.m3u8", bitrate: 2400 }
    ]
  },
  viewerCount: 256,
  chat: {
    roomId: "room-200"
  }
};

export const chatHistory: ChatMessageResponse[] = [
  {
    id: "msg-1",
    channelId,
    userId: "owner-200",
    content: "Welcome aboard the orbital maintenance stream!",
    createdAt: new Date("2024-03-20T11:05:00Z").toISOString()
  },
  {
    id: "msg-2",
    channelId,
    userId: "viewer-19",
    content: "How do you tether the tools?",
    createdAt: new Date("2024-03-20T11:05:20Z").toISOString()
  }
];

export const vodCollection = { channelId, items: [] };

export const unauthenticatedViewer = {
  loginUrl: "https://auth.example.com/login",
  logoutUrl: "https://auth.example.com/logout"
};

export function nextChatMessage(content: string, index: number): ChatMessageResponse {
  return {
    id: `msg-${index}`,
    channelId,
    userId: "viewer-5",
    content,
    createdAt: new Date("2024-03-20T11:06:00Z").toISOString()
  };
}
