"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useMemo, useRef } from "react";
import { normalizeDirectoryQuery, normalizeDirectoryTopic, resolveDirectoryNavigation } from "../lib/directory-state";

export function useDirectorySearch({
  fallbackPathname,
}: {
  fallbackPathname: string;
}) {
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname() ?? fallbackPathname;
  const searchParamQuery = useMemo(() => normalizeDirectoryQuery(searchParams.get("q")), [searchParams]);
  const searchParamTopic = useMemo(() => normalizeDirectoryTopic(searchParams.get("topic")), [searchParams]);
  const lastQueryFromParams = useRef(searchParamQuery);
  const lastTopicFromParams = useRef(searchParamTopic);

  const navigateWithDirectoryState = useCallback(
    ({
      query,
      topic,
    }: {
      query: string;
      topic?: string | null;
    }) => {
      const next = resolveDirectoryNavigation({
        pathname,
        currentParams: new URLSearchParams(searchParams.toString()),
        nextQuery: query,
        previousQuery: lastQueryFromParams.current,
        nextTopic: topic,
        previousTopic: lastTopicFromParams.current,
      });

      lastQueryFromParams.current = next.normalizedQuery;
      lastTopicFromParams.current = next.normalizedTopic;

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
    (value: string) =>
      navigateWithDirectoryState({
        query: value,
        topic: lastTopicFromParams.current,
      }),
    [navigateWithDirectoryState]
  );

  return {
    queryFromParams: searchParamQuery,
    topicFromParams: searchParamTopic,
    lastQueryFromParams,
    lastTopicFromParams,
    navigateWithDirectoryState,
    navigateWithQuery,
  };
}
