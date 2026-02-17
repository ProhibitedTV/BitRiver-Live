import { Suspense } from "react";
import {
  DirectoryPageContent,
  emptyHomeData,
  type DirectoryData,
  type HomeData,
} from "./directory-view";
import type { CategorySummary, DirectoryChannel } from "../lib/viewer-api";
import {
  fetchFeaturedChannels,
  fetchFollowingChannels,
  fetchLiveNowChannels,
  fetchRecommendedChannels,
  fetchTopCategories,
  fetchTrendingChannels,
} from "../lib/viewer-api";
import { loadDirectoryChannels, mapDirectoryError, normalizeDirectoryQuery } from "../lib/directory-state";

function isUnauthorizedFollowingResponse(reason: unknown) {
  const status = typeof reason === "object" && reason !== null && "status" in reason
    ? (reason as { status?: number }).status
    : undefined;

  if (status === 401 || status === 403) {
    return true;
  }

  const message = reason instanceof Error ? reason.message : String(reason ?? "");
  const normalizedMessage = message.toLowerCase();

  return (
    /\b401\b/.test(message) ||
    /\b403\b/.test(message) ||
    normalizedMessage.includes("unauthorized") ||
    normalizedMessage.includes("unauthenticated")
  );
}

async function loadHomeData(): Promise<HomeData> {
  let isAuthenticated = true;
  try {
    const [
      featuredResult,
      followingResult,
      liveResult,
      recommendedResult,
      trendingResult,
      topCategoriesResult,
    ] = await Promise.allSettled([
      fetchFeaturedChannels(),
      fetchFollowingChannels(),
      fetchLiveNowChannels(),
      fetchRecommendedChannels(),
      fetchTrendingChannels(),
      fetchTopCategories(),
    ]);

    const parseChannels = (result: PromiseSettledResult<{ channels: DirectoryChannel[] }>) =>
      result.status === "fulfilled" ? result.value.channels : [];

    const parseCategories = (result: PromiseSettledResult<{ categories?: CategorySummary[] }>) =>
      result.status === "fulfilled" ? result.value.categories ?? [] : [];

    const followingChannels = (() => {
      if (followingResult.status === "fulfilled") {
        return followingResult.value.channels;
      }
      isAuthenticated = !isUnauthorizedFollowingResponse(followingResult.reason);
      return [];
    })();

    return {
      featured: parseChannels(featuredResult),
      recommended: parseChannels(recommendedResult),
      following: followingChannels,
      liveNow: parseChannels(liveResult),
      trending: parseChannels(trendingResult),
      categories: parseCategories(topCategoriesResult),
      isAuthenticated,
    };
  } catch (error) {
    return {
      ...emptyHomeData,
      isAuthenticated,
      error: error instanceof Error ? error.message : "Unable to load personalised rows",
    };
  }
}

async function loadDirectoryData(query: string): Promise<DirectoryData> {
  try {
    const response = await loadDirectoryChannels(query);
    return { channels: response.channels };
  } catch (error) {
    return {
      channels: [],
      error: mapDirectoryError(error),
    };
  }
}

function DirectoryPageFallback({ query }: { query: string }) {
  return (
    <DirectoryPageContent
      query={query}
      homeData={emptyHomeData}
      directoryData={{ channels: [] }}
      homeLoading
      directoryLoading
    />
  );
}

type PageProps = {
  searchParams?: {
    q?: string;
  };
};

export default async function DirectoryPage({ searchParams }: PageProps) {
  const query = normalizeDirectoryQuery(typeof searchParams?.q === "string" ? searchParams.q : "");
  const [homeData, directoryData] = await Promise.all([
    loadHomeData(),
    loadDirectoryData(query),
  ]);

  return (
    <Suspense fallback={<DirectoryPageFallback query={query} />}>
      <DirectoryPageContent query={query} homeData={homeData} directoryData={directoryData} />
    </Suspense>
  );
}
