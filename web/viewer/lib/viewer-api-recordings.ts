import { viewerRequest } from "./viewer-api-core";
import type { Recording } from "./viewer-api-types";

export function publishRecording(recordingId: string): Promise<Recording> {
  return viewerRequest<Recording>(`/api/recordings/${encodeURIComponent(recordingId)}/publish`, {
    method: "POST",
  });
}
