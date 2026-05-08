import { normalizeConfiguredUrlValue } from "./auth-links";

const API_BASE = normalizeConfiguredUrlValue(process.env.NEXT_PUBLIC_API_BASE_URL) ?? "";
const SERVER_API_BASE = normalizeConfiguredUrlValue(process.env.BITRIVER_INTERNAL_API_BASE_URL) || "http://bitriver-live:8080";

function resolveRequestTarget(path: string) {
  if (path.startsWith("http://") || path.startsWith("https://")) {
    return path;
  }

  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  if (typeof window !== "undefined") {
    return `${API_BASE}${normalizedPath}`;
  }

  const base = API_BASE || SERVER_API_BASE;
  return new URL(normalizedPath, base).toString();
}

export function viewerWebSocketUrl(path: string): string {
  const target = new URL(
    resolveRequestTarget(path),
    typeof window !== "undefined" ? window.location.href : SERVER_API_BASE
  );
  target.protocol = target.protocol === "https:" ? "wss:" : "ws:";
  return target.toString();
}

export class ViewerApiError extends Error {
  status: number;
  body?: unknown;
  text?: string;

  constructor(status: number, body?: unknown, text?: string, message?: string) {
    const fallbackMessage = text?.trim() || `${status}`;
    const bodyMessage =
      typeof body === "object" && body !== null && "message" in body && typeof body.message === "string"
        ? body.message
        : undefined;
    super(message ?? bodyMessage ?? fallbackMessage);
    this.name = "ViewerApiError";
    this.status = status;
    this.body = body;
    this.text = text;
  }
}

export async function viewerRequest<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers ?? {});
  if (!(init?.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const response = await fetch(resolveRequestTarget(path), {
    ...init,
    credentials: "include",
    headers,
    cache: "no-store"
  });
  if (!response.ok) {
    const rawBody = await response.text();
    let parsedBody: unknown;
    try {
      parsedBody = rawBody ? JSON.parse(rawBody) : undefined;
    } catch {
      parsedBody = undefined;
    }
    throw new ViewerApiError(response.status, parsedBody, rawBody);
  }
  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
}

export function multipartRequest<T>(
  path: string,
  form: FormData,
  onProgress?: (progress: { percent: number; loadedBytes: number; totalBytes: number }) => void,
  signal?: AbortSignal,
): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    if (signal?.aborted) {
      reject(new DOMException("The operation was aborted", "AbortError"));
      return;
    }

    const xhr = new XMLHttpRequest();
    const onAbortSignal = () => {
      xhr.abort();
    };
    const cleanupAbortSignal = () => {
      signal?.removeEventListener("abort", onAbortSignal);
    };

    signal?.addEventListener("abort", onAbortSignal);

    xhr.open("POST", resolveRequestTarget(path));
    xhr.withCredentials = true;
    xhr.onload = () => {
      cleanupAbortSignal();
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          resolve(JSON.parse(xhr.responseText) as T);
        } catch {
          reject(new Error("invalid server response"));
        }
        return;
      }
      const rawBody = xhr.responseText;
      let parsedBody: unknown;
      try {
        parsedBody = rawBody ? JSON.parse(rawBody) : undefined;
      } catch {
        parsedBody = undefined;
      }
      reject(new ViewerApiError(xhr.status, parsedBody, rawBody));
    };
    xhr.onerror = () => {
      cleanupAbortSignal();
      reject(new ViewerApiError(0, undefined, "upload failed", "upload failed"));
    };
    xhr.onabort = () => {
      cleanupAbortSignal();
      reject(new DOMException("The operation was aborted", "AbortError"));
    };
    if (onProgress) {
      xhr.upload.onprogress = (event) => {
        if (!event.lengthComputable) {
          return;
        }
        const percent = Math.round((event.loaded / event.total) * 100);
        onProgress({
          percent,
          loadedBytes: event.loaded,
          totalBytes: event.total,
        });
      };
    }
    xhr.send(form);
  });
}
