import type { ViewerQoEEvent } from "./viewer-api-types";

export async function reportViewerQoE(payload: ViewerQoEEvent): Promise<void> {
  const body = JSON.stringify(payload);
  if (typeof navigator !== "undefined" && typeof navigator.sendBeacon === "function") {
    const blob = new Blob([body], { type: "application/json" });
    navigator.sendBeacon("/api/metrics/qoe", blob);
    return;
  }
  await fetch("/api/metrics/qoe", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body,
    keepalive: true
  });
}
