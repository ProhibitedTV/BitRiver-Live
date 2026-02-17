import type { DirectoryChannel } from "./viewer-api";

function formatCountLabel(count: number, singular: string, plural = `${singular}s`) {
  return `${count.toLocaleString()} ${count === 1 ? singular : plural}`;
}

export function formatFollowerLabel(followerCount: number) {
  return formatCountLabel(followerCount, "follower");
}

export function formatViewerLabel(viewerCount: number) {
  return formatCountLabel(viewerCount, "viewer");
}

export function getChannelStatusLabel(live: boolean) {
  return live ? "Live" : "Offline";
}

export function getChannelPreviewImage(channel: DirectoryChannel) {
  return channel.profile.bannerUrl ?? channel.profile.avatarUrl ?? channel.owner.avatarUrl;
}

export function getChannelAvatarImage(channel: DirectoryChannel) {
  return channel.owner.avatarUrl ?? channel.profile.avatarUrl;
}

export function getChannelAvatarFallback(displayName: string) {
  return displayName.trim().charAt(0).toUpperCase() || "?";
}
