import { Suspense } from "react";
import { CategoryRail } from "../components/CategoryRail";
import { ChannelRail } from "../components/ChannelRail";
import { DirectoryGrid } from "../components/DirectoryGrid";
import { FeaturedChannel } from "../components/FeaturedChannel";
import { FollowingRail } from "../components/FollowingRail";
import { LiveNowGrid } from "../components/LiveNowGrid";
import { DirectorySearchBar } from "../components/DirectorySearchBar";
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

type HomeData = {
  featured: DirectoryChannel[];
  recommended: DirectoryChannel[];
  following: DirectoryChannel[];
  liveNow: DirectoryChannel[];
  trending: DirectoryChannel[];
  categories: CategorySummary[];
  error?: string;
};

type DirectoryData = {
  channels: DirectoryChannel[];
  error?: string;
};

const emptyHomeData: HomeData = {
  featured: [],
  recommended: [],
  following: [],
  liveNow: [],
  trending: [],
  categories: [],
};

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

function HomePageView({
  query,
  homeData,
  directoryData,
  homeLoading,
  directoryLoading,
}: {
  query: string;
  homeData: HomeData;
  directoryData: DirectoryData;
  homeLoading: boolean;
  directoryLoading: boolean;
}) {
  const { featured, recommended, following, liveNow, trending, categories, error: homeError } = homeData;
  const { channels, error } = directoryData;

  return (
    <div className="home-page">
      <section className="home-hero">
        <div className="home-hero__inner container container--wide">
          <div className="home-hero__content stack">
            <h1>Discover live channels</h1>
            <p className="muted">
              Explore community broadcasts, follow your favourite creators, and jump into ultra-low-latency playback powered by
              BitRiver Live.
            </p>
            <div className="home-hero__actions">
              <a className="primary-button" href="#live-now">
                Start watching
              </a>
            </div>
            <div className="home-hero__search">
              <DirectorySearchBar defaultValue={query} />
            </div>
          </div>

          <div className="home-hero__media">
            <FeaturedChannel channels={featured} loading={homeLoading} />
          </div>
        </div>
      </section>

      <ChannelRail
        eyebrow="For you"
        title="Channels We Think You’ll Like"
        subtitle="A mix of broadcasters similar to what you follow."
        channels={recommended}
        loading={homeLoading}
      />

      <ChannelRail
        eyebrow="Editor’s picks"
        title="Featured Streams"
        subtitle="Hand-picked creators and spotlight broadcasts."
        channels={featured.slice(1)}
        loading={homeLoading}
      />

      <ChannelRail
        eyebrow="On the rise"
        title="Trending"
        subtitle="Streams picking up momentum right now."
        channels={trending}
        loading={homeLoading}
        density="compact"
      />

      <CategoryRail categories={categories} loading={homeLoading} />

      <div className="container stack home-page__content">
        {!homeLoading && homeError && (
          <div className="surface" role="alert">
            {homeError}
          </div>
        )}

        <FollowingRail channels={following} loading={homeLoading} />

        <section className="stack" id="live-now">
          <div className="section-heading">
            <div>
              <h2>Live now</h2>
              <p className="muted">Creators currently on air</p>
            </div>
            {!homeLoading && liveNow.length > 0 && <span className="muted">{liveNow.length} streams</span>}
          </div>
          <LiveNowGrid channels={liveNow} loading={homeLoading} />
        </section>

        <section className="stack" id="directory">
          <div className="section-heading">
            <div>
              <h2>Browse the directory</h2>
              <p className="muted">Filter every channel or search by creator.</p>
            </div>
            {query && <span className="muted">Results for “{query}”</span>}
          </div>
          {directoryLoading && <div className="surface">Loading channels…</div>}
          {!directoryLoading && error && (
            <div className="surface" role="alert">
              {error}
            </div>
          )}
          {!directoryLoading && !error && <DirectoryGrid channels={channels} />}
        </section>
      </div>
    </div>
  );
}

async function DirectoryPageContent({
  query,
  homeDataPromise,
  directoryDataPromise,
}: {
  query: string;
  homeDataPromise: Promise<HomeData>;
  directoryDataPromise: Promise<DirectoryData>;
}) {
  const [homeData, directoryData] = await Promise.all([homeDataPromise, directoryDataPromise]);

  return (
    <HomePageView
      query={query}
      homeData={homeData}
      directoryData={directoryData}
      homeLoading={false}
      directoryLoading={false}
    />
  );
}

function DirectoryPageFallback({ query }: { query: string }) {
  return (
    <HomePageView
      query={query}
      homeData={emptyHomeData}
      directoryData={{ channels: [] }}
      homeLoading
      directoryLoading
    />
  );
}

export default async function DirectoryPage({
  searchParams,
}: {
  searchParams?: {
    q?: string;
  };
}) {
  const query = typeof searchParams?.q === "string" ? searchParams.q : "";
  const homeDataPromise = loadHomeData();
  const directoryDataPromise = loadDirectoryData(query);

  return (
    <Suspense fallback={<DirectoryPageFallback query={query} />}>
      <DirectoryPageContent query={query} homeDataPromise={homeDataPromise} directoryDataPromise={directoryDataPromise} />
    </Suspense>
  );
}
