import { fetchDirectory, searchDirectory, type DirectoryResponse } from "./viewer-api";

export const DIRECTORY_LOAD_ERROR = "Unable to load directory";

export function normalizeDirectoryQuery(value: string | null | undefined): string {
  return (value ?? "").trim();
}

export function normalizeDirectoryCategory(value: string | null | undefined): string {
  return (value ?? "").trim();
}

export function toDirectorySearchParams(value: string, category?: string): URLSearchParams {
  const params = new URLSearchParams();
  const normalized = normalizeDirectoryQuery(value);
  const normalizedCategory = normalizeDirectoryCategory(category);

  if (normalized.length > 0) {
    params.set("q", normalized);
  }

  if (normalizedCategory.length > 0) {
    params.set("category", normalizedCategory);
  }

  return params;
}

export function mapDirectoryError(error: unknown): string {
  return error instanceof Error ? error.message : DIRECTORY_LOAD_ERROR;
}

export async function loadDirectoryChannels(query: string, category?: string): Promise<DirectoryResponse> {
  const normalizedQuery = normalizeDirectoryQuery(query);
  const normalizedCategory = normalizeDirectoryCategory(category);
  if (normalizedQuery.length > 0) {
    return normalizedCategory.length > 0
      ? searchDirectory(normalizedQuery, normalizedCategory)
      : searchDirectory(normalizedQuery);
  }
  return normalizedCategory.length > 0 ? fetchDirectory(normalizedCategory) : fetchDirectory();
}

type DirectoryNavigationInput = {
  pathname: string;
  currentParams: URLSearchParams;
  nextQuery: string;
  previousQuery: string;
  nextCategory?: string | null;
  previousCategory?: string | null;
};

export function resolveDirectoryNavigation({
  pathname,
  currentParams,
  nextQuery,
  previousQuery,
  nextCategory,
  previousCategory,
}: DirectoryNavigationInput): { url: string; useReplace: boolean; normalizedQuery: string; normalizedCategory: string } {
  const normalizedQuery = normalizeDirectoryQuery(nextQuery);
  const normalizedPrevious = normalizeDirectoryQuery(previousQuery);
  const normalizedCategory = normalizeDirectoryCategory(nextCategory);
  const normalizedPreviousCategory = normalizeDirectoryCategory(previousCategory);
  const params = new URLSearchParams(currentParams.toString());

  if (normalizedQuery.length > 0) {
    params.set("q", normalizedQuery);
  } else {
    params.delete("q");
  }

  if (normalizedCategory.length > 0) {
    params.set("category", normalizedCategory);
  } else {
    params.delete("category");
  }

  const queryString = params.toString();
  const url = queryString.length > 0 ? `${pathname}?${queryString}` : pathname;

  return {
    url,
    useReplace:
      (normalizedQuery.length === 0 && normalizedCategory.length === 0) ||
      (normalizedQuery === normalizedPrevious && normalizedCategory === normalizedPreviousCategory),
    normalizedQuery,
    normalizedCategory,
  };
}
