const DEFAULT_SIGNUP_PATH = "/signup";
const AUTH_SURFACE_BASE = "http://bitriver.local";

export function normalizeConfiguredUrlValue(rawValue?: string) {
  if (rawValue === undefined) {
    return undefined;
  }

  const trimmed = rawValue.trim();
  if (!trimmed || trimmed === '""' || trimmed === "''") {
    return "";
  }

  const hasWrappingQuotes =
    (trimmed.startsWith('"') && trimmed.endsWith('"')) || (trimmed.startsWith("'") && trimmed.endsWith("'"));
  if (!hasWrappingQuotes) {
    return trimmed;
  }

  const unwrapped = trimmed.slice(1, -1).trim();
  if (!unwrapped || unwrapped === '""' || unwrapped === "''") {
    return "";
  }

  return unwrapped;
}

export function joinConfiguredPath(baseUrl: string | undefined, path: string) {
  const base = normalizeConfiguredUrlValue(baseUrl);
  if (!base) {
    return path;
  }

  return `${base.replace(/\/+$/, "")}${path}`;
}

export function resolveSignupUrl(
  configuredSignupUrl = process.env.NEXT_PUBLIC_SIGNUP_URL,
  apiBaseUrl = process.env.NEXT_PUBLIC_API_BASE_URL,
) {
  const trimmedSignupUrl = normalizeConfiguredUrlValue(configuredSignupUrl);
  if (trimmedSignupUrl !== undefined) {
    return trimmedSignupUrl || undefined;
  }

  if (normalizeConfiguredUrlValue(apiBaseUrl)) {
    return joinConfiguredPath(apiBaseUrl, DEFAULT_SIGNUP_PATH);
  }

  return DEFAULT_SIGNUP_PATH;
}

export function appendRedirectParam(
  destination: string,
  origin: string,
  redirectTo: string,
  paramName: "next" | "redirect" = "next",
) {
  const url = new URL(destination, origin);
  if (!url.searchParams.has(paramName)) {
    url.searchParams.set(paramName, redirectTo);
  }
  return url.toString();
}

export function appendHash(destination: string, hash: string) {
  const normalizedHash = hash.startsWith("#") ? hash : `#${hash}`;
  const isAbsolute = /^https?:\/\//i.test(destination);
  const url = new URL(destination, AUTH_SURFACE_BASE);
  if (!url.hash) {
    url.hash = normalizedHash;
  }

  if (isAbsolute) {
    return url.toString();
  }

  return `${url.pathname}${url.search}${url.hash}`;
}
