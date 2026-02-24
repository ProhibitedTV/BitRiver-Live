import { viewerRequest } from "./viewer-api-core";
import type {
  CreateTipPayload,
  FollowState,
  ManagedChannel,
  StreamSession,
  SubscriptionState,
  TipResponse,
  UpdateChannelPayload,
  VodCollection,
} from "./viewer-api-types";

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

export function fetchChannelVods(channelId: string): Promise<VodCollection> {
  return viewerRequest<VodCollection>(`/api/channels/${channelId}/vods`);
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
