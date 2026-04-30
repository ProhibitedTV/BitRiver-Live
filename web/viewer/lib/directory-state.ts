import { fetchDirectory, searchDirectory, type DirectoryResponse } from "./viewer-api";

export const DIRECTORY_LOAD_ERROR = "Unable to load directory";

export function normalizeDirectoryQuery(value: string | null | undefined): string {
  return (value ?? "").trim();
}

export function normalizeDirectoryTopic(value: string | null | undefined): string | null {
  const normalized = (value ?? "").trim();
  return normalized.length > 0 ? normalized : null;
}

export function toDirectorySearchParams({
  query,
  topic,
}: {
  query: string;
  topic?: string | null;
}): URLSearchParams {
  const params = new URLSearchParams();
  const normalizedQuery = normalizeDirectoryQuery(query);
  const normalizedTopic = normalizeDirectoryTopic(topic);

  if (normalizedQuery.length > 0) {
    params.set("q", normalizedQuery);
  }

  if (normalizedTopic) {
    params.set("topic", normalizedTopic);
  }

  return params;
}

export function mapDirectoryError(error: unknown): string {
  return error instanceof Error ? error.message : DIRECTORY_LOAD_ERROR;
}

export async function loadDirectoryChannels(query: string): Promise<DirectoryResponse> {
  const normalizedQuery = normalizeDirectoryQuery(query);
  return normalizedQuery.length > 0 ? searchDirectory(normalizedQuery) : fetchDirectory();
}

type DirectoryNavigationInput = {
  pathname: string;
  currentParams: URLSearchParams;
  nextQuery: string;
  previousQuery: string;
  nextTopic?: string | null;
  previousTopic?: string | null;
};

export function resolveDirectoryNavigation({
  pathname,
  currentParams,
  nextQuery,
  previousQuery,
  nextTopic,
  previousTopic,
}: DirectoryNavigationInput): { url: string; useReplace: boolean; normalizedQuery: string; normalizedTopic: string | null } {
  const normalizedQuery = normalizeDirectoryQuery(nextQuery);
  const normalizedPrevious = normalizeDirectoryQuery(previousQuery);
  const normalizedTopic = normalizeDirectoryTopic(nextTopic);
  const normalizedPreviousTopic = normalizeDirectoryTopic(previousTopic);
  const params = new URLSearchParams(currentParams.toString());

  if (normalizedQuery.length > 0) {
    params.set("q", normalizedQuery);
  } else {
    params.delete("q");
  }

  if (normalizedTopic) {
    params.set("topic", normalizedTopic);
  } else {
    params.delete("topic");
  }

  const queryString = params.toString();
  const url = queryString.length > 0 ? `${pathname}?${queryString}` : pathname;

  return {
    url,
    useReplace:
      (normalizedQuery.length === 0 && !normalizedTopic) ||
      (normalizedQuery === normalizedPrevious && normalizedTopic === normalizedPreviousTopic),
    normalizedQuery,
    normalizedTopic,
  };
}
