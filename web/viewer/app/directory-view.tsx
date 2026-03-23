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
  isAuthenticated: boolean;
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
  isAuthenticated: true,
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
  const heroStats = [
    { label: "Live now", value: liveNow.length.toLocaleString() },
    { label: "Categories", value: categories.length.toLocaleString() },
    { label: "Recommended", value: recommended.length.toLocaleString() },
  ];
  const homeErrorMessage = homeError
    ? `We couldnâ€™t load personalized rows (featured, recommended, following): ${homeError}`
    : null;
  const directoryErrorMessage = directoryError ? `We couldnâ€™t load the directory right now: ${directoryError}` : null;

  return (
    <div className="home-page">
      <section className="home-hero">
        <div className="home-hero__inner container container--wide">
          <div className="home-hero__content stack stack--lg">
            <div className="stack stack--xs">
              <span className="home-hero__eyebrow">Discover</span>
              <h1>Find the streams worth opening now</h1>
            </div>
            <p className="muted">
              BitRiver Live brings live channels, categories, and the creators you already follow into one clear viewer flow.
            </p>
            <div className="home-hero__stats">
              {heroStats.map((stat) => (
                <div key={stat.label} className="stat-pill">
                  <span className="stat-pill__label">{stat.label}</span>
                  <strong className="stat-pill__value">{stat.value}</strong>
                </div>
              ))}
            </div>
            <div className="home-hero__actions">
              <a href="#live-now" className="primary-button">
                Open live now
              </a>
              <a href="#directory" className="secondary-button">
                Browse directory
              </a>
            </div>
            <nav aria-label="Quick jump links" className="home-hero__quick-links">
              <a href="#top-categories" className="pill pill--tag">
                Top categories
              </a>
              <a href="#trending-now" className="pill pill--tag">
                Trending now
              </a>
              <a href="#live-now" className="pill pill--tag">
                Live now
              </a>
            </nav>
            <div className="home-hero__search">
              <div className="stack stack--2xs">
                <span className="home-hero__search-label">Search the full directory</span>
                <span className="muted">Jump straight to a creator, category, or tag.</span>
              </div>
              <DirectorySearchBar defaultValue={query} />
            </div>
          </div>

          <div className="home-hero__media stack stack--sm">
            <div className="home-hero__media-header">
              <span className="home-hero__eyebrow">Featured</span>
              <p className="muted">A highlighted creator, ready for one-click playback.</p>
            </div>
            <FeaturedChannel channels={featured} loading={homeLoading} />
          </div>
        </div>
      </section>

      <div className="content-rail stack stack--xl">
        {!homeLoading && homeErrorMessage && (
          <div className="surface" role="alert">
            {homeErrorMessage}
          </div>
        )}

        <ChannelRail
          title="Recommended for you"
          subtitle="Jump back into channels that already have momentum."
          channels={recommended}
          loading={homeLoading}
        />

        <div className="content-rail__grid">
          <section className="surface stack" id="top-categories">
            <div className="section-heading">
              <div>
                <h2>Top categories</h2>
                <p className="muted">Jump into the most active corners of the network.</p>
              </div>
              {!homeLoading && categories.length > 0 && <span className="muted">{categories.length} results</span>}
            </div>
            <CategoryRail categories={categories} loading={homeLoading} />
          </section>

          <section className="surface stack" id="trending-now">
            <div className="section-heading">
              <div>
                <h2>Trending now</h2>
                <p className="muted">Channels pulling viewers right now across BitRiver.</p>
              </div>
              {!homeLoading && trending.length > 0 && <span className="muted">{trending.length} channels</span>}
            </div>
            <ChannelRail title="Trending now" channels={trending} loading={homeLoading} density="compact" />
          </section>
        </div>

        <FollowingRail channels={following} loading={homeLoading} isAuthenticated={homeData.isAuthenticated} />

        <section className="surface stack" id="live-now">
          <div className="section-heading">
            <div>
              <h2>Live now</h2>
              <p className="muted">Creators currently on air and ready to watch.</p>
            </div>
            {!homeLoading && liveNow.length > 0 && <span className="muted">{liveNow.length} streams</span>}
          </div>
          <LiveNowGrid channels={liveNow} loading={homeLoading} />
        </section>

        <section className="surface stack" id="directory">
          <div className="section-heading">
            <div>
              <h2>Browse the directory</h2>
              <p className="muted">Search every channel or scan the full lineup below.</p>
            </div>
            {query && <span className="muted">Results for â€œ{query}â€</span>}
          </div>
          {directoryLoading && <div className="surface">Loading channelsâ€¦</div>}
          {!directoryLoading && directoryErrorMessage && (
            <div className="surface" role="alert">
              {directoryErrorMessage}
            </div>
          )}
          {!directoryLoading && !directoryError && <DirectoryGrid channels={channels} />}
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
