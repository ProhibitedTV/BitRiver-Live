const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";

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
  const response = await fetch(`${API_BASE}${path}`, {
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

    xhr.open("POST", `${API_BASE}${path}`);
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
