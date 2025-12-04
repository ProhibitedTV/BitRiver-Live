import { Suspense } from "react";
import {
  DirectoryPageContent,
  emptyHomeData,
  type DirectoryData,
  type HomeData,
} from "./directory-view";
import type { CategorySummary, DirectoryChannel } from "../lib/viewer-api";
import {
  fetchDirectory,
  fetchFeaturedChannels,
  fetchFollowingChannels,
  fetchLiveNowChannels,
  fetchRecommendedChannels,
  fetchTopCategories,
  fetchTrendingChannels,
  searchDirectory,
} from "../lib/viewer-api";

async function loadHomeData(): Promise<HomeData> {
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
      const message = followingResult.reason instanceof Error ? followingResult.reason.message : String(followingResult.reason);
      if (message === "401" || message === "403") {
        return [];
      }
      return [];
    })();

    return {
      featured: parseChannels(featuredResult),
      recommended: parseChannels(recommendedResult),
      following: followingChannels,
      liveNow: parseChannels(liveResult),
      trending: parseChannels(trendingResult),
      categories: parseCategories(topCategoriesResult),
    };
  } catch (error) {
    return {
      ...emptyHomeData,
      error: error instanceof Error ? error.message : "Unable to load personalised rows",
    };
  }
}

async function loadDirectoryData(query: string): Promise<DirectoryData> {
  try {
    const response = query.trim().length > 0 ? await searchDirectory(query) : await fetchDirectory();
    return { channels: response.channels };
  } catch (error) {
    return {
      channels: [],
      error: error instanceof Error ? error.message : "Unable to load directory",
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
  const query = typeof searchParams?.q === "string" ? searchParams.q : "";
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
