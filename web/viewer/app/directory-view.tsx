import Link from "next/link";
import { CategoryRail } from "../components/CategoryRail";
import { ChannelRail } from "../components/ChannelRail";
import { DirectoryGrid } from "../components/DirectoryGrid";
import { DirectorySearchBar } from "../components/DirectorySearchBar";
import { FeaturedChannel } from "../components/FeaturedChannel";
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
  const primaryDiscoveryChannels = homeData.isAuthenticated && following.length > 0 ? following : recommended;
  const primaryDiscoveryTitle = homeData.isAuthenticated && following.length > 0 ? "Following now" : "Recommended live";
  const primaryDiscoverySubtitle =
    homeData.isAuthenticated && following.length > 0
      ? "Creators you already care about, surfaced first when they are active."
      : "A fast shortlist of live channels worth opening right away.";
  const primaryDiscoveryEyebrow = homeData.isAuthenticated && following.length > 0 ? "Your roster" : "Start here";
  const heroStats = [
    { label: "Live now", value: liveNow.length.toLocaleString() },
    {
      label: homeData.isAuthenticated && following.length > 0 ? "Following" : "Recommended",
      value: primaryDiscoveryChannels.length.toLocaleString(),
    },
    { label: "Categories", value: categories.length.toLocaleString() },
  ];
  const quickLinks = [
    { href: "#live-now", label: "Live now" },
    { href: "#top-categories", label: "Categories" },
    { href: "/videos", label: "Videos" },
    { href: "/creator/getting-started", label: "Go live" },
  ];
  const homeErrorMessage = homeError
    ? `We couldn't load the personalized discovery rows right now: ${homeError}`
    : null;
  const directoryErrorMessage = directoryError ? `We couldn't load the directory right now: ${directoryError}` : null;

  return (
    <div className="home-page">
      <section className="home-hero">
        <div className="home-hero__layout">
          <div className="home-hero__main">
            <div className="home-hero__copy stack stack--lg">
              <div className="stack stack--xs">
                <span className="home-hero__eyebrow">Live channels, replays, and creator tools</span>
                <h1>Watch live now, then launch your own stream with confidence</h1>
                <p className="home-hero__lede muted">
                  BitRiver Live is built around one simple loop: discover active creators, support them directly, keep replays visible, and make going live feel clear instead of intimidating.
                </p>
              </div>

              <div className="home-hero__actions">
                <a href="#live-now" className="primary-button">
                  Watch live now
                </a>
                <Link href="/creator/getting-started" className="secondary-button">
                  Go live
                </Link>
              </div>

              <div className="home-hero__search-panel">
                <div className="stack stack--2xs">
                  <span className="home-hero__search-label">Search creators, categories, and tags</span>
                  <span className="muted">Start broad, then jump straight into a channel, replay-friendly creator, or niche category.</span>
                </div>
                <DirectorySearchBar defaultValue={query} />
              </div>

              <dl className="home-hero__stats" aria-label="Platform snapshot">
                {heroStats.map((stat) => (
                  <div key={stat.label} className="home-stat">
                    <dt className="home-stat__label">{stat.label}</dt>
                    <dd className="home-stat__value">{stat.value}</dd>
                  </div>
                ))}
              </dl>

              <nav aria-label="Quick jump links" className="home-hero__quick-links">
                {quickLinks.map((item) => (
                  <Link key={item.href} href={item.href} className="pill pill--ghost">
                    {item.label}
                  </Link>
                ))}
              </nav>
            </div>
          </div>

          <aside className="home-hero__aside">
            <div className="home-hero__aside-header">
              <div className="stack stack--2xs">
                <span className="home-hero__eyebrow">Featured live</span>
                <h2>Start with the stream setting the pace right now</h2>
              </div>
              <p className="muted">A single highlighted broadcast when you want a strong first answer instead of a full scan.</p>
            </div>
            <FeaturedChannel channels={featured} loading={homeLoading} />
          </aside>
        </div>
      </section>

      <div className="home-sections">
        {!homeLoading && homeErrorMessage && (
          <div className="state-panel state-panel--error" role="alert">
            <strong>Some discovery rows are unavailable</strong>
            <p className="muted">{homeErrorMessage}</p>
          </div>
        )}

        <ChannelRail
          id="recommended"
          title={primaryDiscoveryTitle}
          subtitle={primaryDiscoverySubtitle}
          channels={primaryDiscoveryChannels}
          loading={homeLoading}
          eyebrow={primaryDiscoveryEyebrow}
        />

        <section className="home-section surface" id="live-now">
          <div className="home-section__header">
            <div className="stack stack--2xs">
              <span className="home-section__eyebrow">On air</span>
              <h2>Live now</h2>
              <p className="muted">The creators already live and ready to open right this second.</p>
            </div>
            {!homeLoading && liveNow.length > 0 && <span className="muted">{liveNow.length} live streams</span>}
          </div>
          <LiveNowGrid channels={liveNow} loading={homeLoading} />
        </section>

        <div className="home-section-grid">
          <CategoryRail id="top-categories" categories={categories} loading={homeLoading} />
          <ChannelRail
            id="trending"
            title="Trending now"
            subtitle="Channels pulling attention across the network right now."
            channels={trending}
            loading={homeLoading}
            density="compact"
            eyebrow="Momentum"
          />
        </div>

        <section className="home-section surface" id="directory">
          <div className="home-section__header">
            <div className="stack stack--2xs">
              <span className="home-section__eyebrow">Browse everything</span>
              <h2>{query ? "Search results" : "Full directory"}</h2>
              <p className="muted">
                {query
                  ? `Showing channels that match "${query}".`
                  : "Scan the full lineup once you want to move past the curated shelves and hunt for something specific."}
              </p>
            </div>
            {!directoryLoading && !directoryError && channels.length > 0 && <span className="muted">{channels.length} channels</span>}
          </div>

          {directoryLoading ? (
            <div className="state-panel state-panel--loading" aria-busy="true">
              <strong>Loading directory</strong>
              <p className="muted">Refreshing the latest channel lineup now.</p>
            </div>
          ) : directoryErrorMessage ? (
            <div className="state-panel state-panel--error" role="alert">
              <strong>Directory unavailable</strong>
              <p className="muted">{directoryErrorMessage}</p>
            </div>
          ) : (
            <DirectoryGrid channels={channels} />
          )}
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
