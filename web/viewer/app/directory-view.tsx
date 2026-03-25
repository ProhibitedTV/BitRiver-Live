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
  const hasChannelsToBrowse =
    featured.length > 0 ||
    recommended.length > 0 ||
    following.length > 0 ||
    liveNow.length > 0 ||
    trending.length > 0 ||
    channels.length > 0;
  const hasDiscoveryContent = hasChannelsToBrowse || categories.length > 0;
  const showLiveNowSection = homeLoading || liveNow.length > 0;
  const showRecommendedSection = homeLoading || recommended.length > 0;
  const showCategoriesSection = homeLoading || categories.length > 0;
  const showTrendingSection = homeLoading || trending.length > 0;
  const quickLinks = [
    showLiveNowSection ? { href: "#live-now", label: "Live now" } : null,
    showRecommendedSection ? { href: "#recommended", label: "Recommended" } : null,
    showCategoriesSection ? { href: "#top-categories", label: "Categories" } : null,
    { href: "#directory", label: query ? "Search results" : "Directory" },
  ].filter((item): item is { href: string; label: string } => Boolean(item));
  const homeErrorMessage = homeError
    ? `We couldn't load the personalized discovery rows right now: ${homeError}`
    : null;
  const directoryErrorMessage = directoryError ? `We couldn't load the directory right now: ${directoryError}` : null;
  const heroEyebrow = hasDiscoveryContent ? "Self-hosted live" : "First stream";
  const heroTitle = liveNow.length > 0
    ? "Watch live or launch your own channel"
    : hasChannelsToBrowse
      ? "Browse channels or launch your own"
      : "Launch your first self-hosted channel";
  const heroLede = liveNow.length > 0
    ? "Watch what is live now, then use creator setup when you are ready to stream from your own stack."
    : hasChannelsToBrowse
      ? "Browse what is already on this install, then use creator setup when you are ready to start streaming yourself."
      : "Your install is ready. Create a channel, copy the OBS settings, and share the viewer link once the preview is live.";
  const primaryHeroAction = liveNow.length > 0
    ? { href: "#live-now", label: "Watch live now" }
    : hasChannelsToBrowse
      ? { href: "#directory", label: "Browse channels" }
      : { href: "/creator/getting-started", label: "Start creator setup" };
  const secondaryHeroAction = hasChannelsToBrowse
    ? { href: "/creator/getting-started", label: "Start streaming" }
    : { href: "#directory", label: "Browse directory" };
  const featuredEmptyMessage = hasDiscoveryContent
    ? "Featured streams will show up here as soon as one is available."
    : "No one is live yet. Use creator setup to bring the first channel online, then come back here to watch it like a viewer.";

  return (
    <div className="home-page">
      <section className="home-hero">
        <div className="home-hero__layout">
          <div className="home-hero__spotlight">
            <div className="home-hero__spotlight-header">
              <div className="stack stack--xs home-hero__spotlight-copy">
                <span className="home-hero__eyebrow">{heroEyebrow}</span>
                <h1>{heroTitle}</h1>
                <p className="home-hero__lede muted">{heroLede}</p>
              </div>
              <div className="home-hero__spotlight-actions">
                <Link href={primaryHeroAction.href} className="primary-button">
                  {primaryHeroAction.label}
                </Link>
                <Link href={secondaryHeroAction.href} className="secondary-button">
                  {secondaryHeroAction.label}
                </Link>
              </div>
            </div>
            {homeLoading || featured.length > 0 ? (
              <FeaturedChannel channels={featured} loading={homeLoading} />
            ) : (
              <div className="state-panel">
                <strong>No featured stream yet</strong>
                <p className="muted">{featuredEmptyMessage}</p>
              </div>
            )}
          </div>

          <aside className="home-hero__side">
            <section className="home-hero__utility">
              <div className="stack stack--2xs">
                <span className="home-hero__eyebrow">Find a stream fast</span>
                <h2>Search creators, channels, or tags</h2>
                <p className="muted">
                  Use search when you know what you want, or scroll into live and directory sections below.
                </p>
              </div>
              <DirectorySearchBar defaultValue={query} />
              {quickLinks.length > 1 ? (
                <nav aria-label="Quick jump links" className="home-hero__quick-links">
                  {quickLinks.map((item) => (
                    <a key={item.href} href={item.href} className="pill pill--ghost">
                      {item.label}
                    </a>
                  ))}
                </nav>
              ) : null}
            </section>
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

        {showLiveNowSection ? (
          <section className="home-section surface" id="live-now">
            <div className="home-section__header">
              <div className="stack stack--2xs">
                <span className="home-section__eyebrow">On air</span>
                <h2>Live now</h2>
                <p className="muted">Channels you can watch right away.</p>
              </div>
              {!homeLoading && liveNow.length > 0 && <span className="muted">{liveNow.length} live streams</span>}
            </div>
            <LiveNowGrid channels={liveNow} loading={homeLoading} />
          </section>
        ) : null}

        {showRecommendedSection ? (
          <ChannelRail
            id="recommended"
            title="Recommended for you"
            subtitle="Good channels to open next."
            channels={recommended}
            loading={homeLoading}
            eyebrow="Suggested"
          />
        ) : null}

        {showCategoriesSection || showTrendingSection ? (
          <div className="home-section-grid">
            {showCategoriesSection ? <CategoryRail id="top-categories" categories={categories} loading={homeLoading} /> : null}
            {showTrendingSection ? (
              <ChannelRail
                id="trending"
                title="Trending now"
                subtitle="Streams getting attention right now."
                channels={trending}
                loading={homeLoading}
                density="compact"
                eyebrow="Trending"
              />
            ) : null}
          </div>
        ) : null}

        <section className="home-section surface" id="directory">
          <div className="home-section__header">
            <div className="stack stack--2xs">
              <span className="home-section__eyebrow">Browse everything</span>
              <h2>{query ? "Search results" : "Full directory"}</h2>
              <p className="muted">
                {query
                  ? `Showing channels that match "${query}".`
                  : "Every channel on this install, in one place."}
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
          ) : channels.length === 0 && !query ? (
            <div className="state-panel">
              <strong>No channels yet</strong>
              <p className="muted">
                Start with creator setup, go live from OBS, and this directory will become the public home for every stream on your install.
              </p>
              <div className="home-hero__spotlight-actions">
                <Link href="/creator/getting-started" className="primary-button">
                  Start creator setup
                </Link>
              </div>
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
