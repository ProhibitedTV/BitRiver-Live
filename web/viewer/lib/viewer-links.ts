function normalizeBasePath(basePath?: string): string {
  const trimmed = basePath?.trim();
  if (!trimmed || trimmed === "/") {
    return "";
  }

  const withLeadingSlash = trimmed.startsWith("/") ? trimmed : `/${trimmed}`;
  return withLeadingSlash.replace(/\/+$/, "");
}

function normalizePath(path: string): string {
  if (!path) {
    return "/";
  }
  return path.startsWith("/") ? path : `/${path}`;
}

export function buildViewerPath(path: string, basePath = process.env.NEXT_PUBLIC_VIEWER_BASE_PATH): string {
  const normalizedPath = normalizePath(path);
  const normalizedBasePath = normalizeBasePath(basePath);

  if (!normalizedBasePath) {
    return normalizedPath;
  }

  if (normalizedPath === normalizedBasePath || normalizedPath.startsWith(`${normalizedBasePath}/`)) {
    return normalizedPath;
  }

  return `${normalizedBasePath}${normalizedPath}`;
}

export function buildViewerUrl(
  path: string,
  origin: string,
  basePath = process.env.NEXT_PUBLIC_VIEWER_BASE_PATH,
): string {
  return new URL(buildViewerPath(path, basePath), origin).toString();
}
