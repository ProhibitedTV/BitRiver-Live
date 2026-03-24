"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useMemo, useRef } from "react";
import { normalizeDirectoryCategory, normalizeDirectoryQuery, resolveDirectoryNavigation } from "../lib/directory-state";

export function useDirectorySearch({
  fallbackPathname,
}: {
  fallbackPathname: string;
}) {
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname() ?? fallbackPathname;
  const searchParamQuery = useMemo(() => normalizeDirectoryQuery(searchParams.get("q")), [searchParams]);
  const categoryFromParams = useMemo(() => normalizeDirectoryCategory(searchParams.get("category")), [searchParams]);
  const lastQueryFromParams = useRef(searchParamQuery);
  const lastCategoryFromParams = useRef(categoryFromParams);

  const navigateWithDirectoryState = useCallback(
    ({ query, category }: { query: string; category?: string | null }) => {
      const next = resolveDirectoryNavigation({
        pathname,
        currentParams: new URLSearchParams(searchParams.toString()),
        nextQuery: query,
        previousQuery: lastQueryFromParams.current,
        nextCategory: category,
        previousCategory: lastCategoryFromParams.current,
      });

      lastQueryFromParams.current = next.normalizedQuery;
      lastCategoryFromParams.current = next.normalizedCategory;

      if (next.useReplace) {
        router.replace(next.url);
      } else {
        router.push(next.url);
      }

      return next;
    },
    [pathname, router, searchParams]
  );

  const navigateWithQuery = useCallback(
    (value: string) => {
      const next = navigateWithDirectoryState({ query: value, category: categoryFromParams });
      return next.normalizedQuery;
    },
    [categoryFromParams, navigateWithDirectoryState]
  );

  return {
    queryFromParams: searchParamQuery,
    categoryFromParams,
    lastQueryFromParams,
    lastCategoryFromParams,
    navigateWithDirectoryState,
    navigateWithQuery,
  };
}
