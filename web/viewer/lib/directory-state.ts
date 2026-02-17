import { fetchDirectory, searchDirectory, type DirectoryResponse } from "./viewer-api";

export const DIRECTORY_LOAD_ERROR = "Unable to load directory";

export function normalizeDirectoryQuery(value: string | null | undefined): string {
  return (value ?? "").trim();
}

export function toDirectorySearchParams(value: string): URLSearchParams {
  const params = new URLSearchParams();
  const normalized = normalizeDirectoryQuery(value);

  if (normalized.length > 0) {
    params.set("q", normalized);
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
};

export function resolveDirectoryNavigation({
  pathname,
  currentParams,
  nextQuery,
  previousQuery,
}: DirectoryNavigationInput): { url: string; useReplace: boolean; normalizedQuery: string } {
  const normalizedQuery = normalizeDirectoryQuery(nextQuery);
  const normalizedPrevious = normalizeDirectoryQuery(previousQuery);
  const params = new URLSearchParams(currentParams.toString());

  if (normalizedQuery.length > 0) {
    params.set("q", normalizedQuery);
  } else {
    params.delete("q");
  }

  const queryString = params.toString();
  const url = queryString.length > 0 ? `${pathname}?${queryString}` : pathname;

  return {
    url,
    useReplace: normalizedQuery.length === 0 || normalizedQuery === normalizedPrevious,
    normalizedQuery,
  };
}
