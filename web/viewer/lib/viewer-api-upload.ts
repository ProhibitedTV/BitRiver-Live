import { multipartRequest, viewerRequest } from "./viewer-api-core";
import type { CreateUploadPayload, MultipartOptions, UploadItem } from "./viewer-api-types";

export function fetchChannelUploads(channelId: string): Promise<UploadItem[]> {
  return viewerRequest<UploadItem[]>(`/api/uploads?channelId=${encodeURIComponent(channelId)}`);
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
    return multipartRequest<UploadItem>("/api/uploads", form, options.onProgress, options.signal);
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
