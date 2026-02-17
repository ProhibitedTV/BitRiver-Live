"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useMemo, useRef } from "react";
import { normalizeDirectoryQuery, resolveDirectoryNavigation } from "../lib/directory-state";

export function useDirectorySearch({
  fallbackPathname,
}: {
  fallbackPathname: string;
}) {
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname() ?? fallbackPathname;
  const searchParamQuery = useMemo(() => normalizeDirectoryQuery(searchParams.get("q")), [searchParams]);
  const lastQueryFromParams = useRef(searchParamQuery);

  const navigateWithQuery = useCallback(
    (value: string) => {
      const next = resolveDirectoryNavigation({
        pathname,
        currentParams: new URLSearchParams(searchParams.toString()),
        nextQuery: value,
        previousQuery: lastQueryFromParams.current,
      });

      lastQueryFromParams.current = next.normalizedQuery;

      if (next.useReplace) {
        router.replace(next.url);
      } else {
        router.push(next.url);
      }

      return next.normalizedQuery;
    },
    [pathname, router, searchParams]
  );

  return {
    queryFromParams: searchParamQuery,
    lastQueryFromParams,
    navigateWithQuery,
  };
}
