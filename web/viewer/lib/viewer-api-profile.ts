import { viewerRequest } from "./viewer-api-core";
import type { ProfileView, UpdateProfilePayload } from "./viewer-api-types";

export function fetchProfile(userId: string): Promise<ProfileView> {
  return viewerRequest<ProfileView>(`/api/profiles/${userId}`);
}

export function updateProfile(userId: string, payload: UpdateProfilePayload): Promise<ProfileView> {
  return viewerRequest<ProfileView>(`/api/profiles/${userId}`, {
    method: "PUT",
    body: JSON.stringify(payload),
  });
}
