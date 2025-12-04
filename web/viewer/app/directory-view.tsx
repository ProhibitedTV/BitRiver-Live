import { CategoryRail } from "../components/CategoryRail";
import { ChannelRail } from "../components/ChannelRail";
import { DirectoryGrid } from "../components/DirectoryGrid";
import { DirectorySearchBar } from "../components/DirectorySearchBar";
import { FeaturedChannel } from "../components/FeaturedChannel";
import { FollowingRail } from "../components/FollowingRail";
import { LiveNowGrid } from "../components/LiveNowGrid";
import type { CategorySummary, DirectoryChannel } from "../lib/viewer-api";

export type HomeData = {
  featured: DirectoryChannel[];
  recommended: DirectoryChannel[];
  following: DirectoryChannel[];
  liveNow: DirectoryChannel[];
  trending: DirectoryChannel[];
  categories: CategorySummary[];
  error?: string;
};

export type DirectoryData = {
  channels: DirectoryChannel[];
  error?: string;
};

export const emptyHomeData: HomeData = {
  featured: [],
  recommended: [],
  following: [],
  liveNow: [],
  trending: [],
  categories: [],
};

export function HomePageView({
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
  const { channels, error: directoryError } = directoryData;
  const error = directoryError ?? homeError;

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
              <a href="#live-now" className="primary-button">
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

      <div className="content-rail stack">
        <ChannelRail title="Channels We Think You’ll Like" channels={recommended} loading={homeLoading} />

        <div className="content-rail__grid">
          <section className="stack">
            <div className="section-heading">
              <div>
                <h2>Top categories</h2>
                <p className="muted">Jump into an active corner of the community.</p>
              </div>
              {!homeLoading && categories.length > 0 && <span className="muted">{categories.length} results</span>}
            </div>
            <CategoryRail categories={categories} loading={homeLoading} />
          </section>

          <section className="stack">
            <div className="section-heading">
              <div>
                <h2>Trending now</h2>
                <p className="muted">Across BitRiver</p>
              </div>
              {!homeLoading && trending.length > 0 && <span className="muted">{trending.length} channels</span>}
            </div>
            <ChannelRail title="Trending now" channels={trending} loading={homeLoading} density="compact" />
          </section>
        </div>

        {error && (
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

export function DirectoryPageContent({
  query,
  homeData,
  directoryData,
  homeLoading = false,
  directoryLoading = false,
}: {
  query: string;
  homeData: HomeData;
  directoryData: DirectoryData;
  homeLoading?: boolean;
  directoryLoading?: boolean;
}) {
  return (
    <HomePageView
      query={query}
      homeData={homeData}
      directoryData={directoryData}
      homeLoading={homeLoading}
      directoryLoading={directoryLoading}
    />
  );
}
