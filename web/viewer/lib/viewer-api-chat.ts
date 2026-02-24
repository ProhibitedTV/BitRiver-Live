import { viewerRequest } from "./viewer-api-core";
import type { ChatMessage, ChatMessageResponse } from "./viewer-api-types";

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
