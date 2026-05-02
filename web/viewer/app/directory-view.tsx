import Image from "next/image";
import Link from "next/link";
import { CategoryRail } from "../components/CategoryRail";
import { ChannelRail } from "../components/ChannelRail";
import { DirectoryGrid } from "../components/DirectoryGrid";
import { DirectorySearchBar } from "../components/DirectorySearchBar";
import { FeaturedChannel } from "../components/FeaturedChannel";
import { LiveNowGrid } from "../components/LiveNowGrid";
import { AuthActionLink } from "../components/auth/AuthActionLink";
import {
  formatFollowerLabel,
  formatViewerLabel,
  getChannelAvatarFallback,
  getChannelAvatarImage,
} from "../lib/channel-presenters";
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

function HomeDiscoveryCard({ entry }: { entry: DirectoryChannel }) {
  const avatarImage = getChannelAvatarImage(entry);
  const audienceLabel = entry.live
    ? formatViewerLabel(entry.viewerCount ?? 0)
    : formatFollowerLabel(entry.followerCount);

  return (
    <li>
      <Link href={`/channels/${entry.channel.id}`} className="home-discovery-card">
        <div className="home-discovery-card__avatar" aria-hidden="true">
          {avatarImage ? (
            <Image
              src={avatarImage}
              alt=""
              fill
              sizes="56px"
              className="home-discovery-card__avatar-image"
            />
          ) : (
            <span>{getChannelAvatarFallback(entry.owner.displayName)}</span>
          )}
        </div>

        <div className="home-discovery-card__body">
          <div className="home-discovery-card__header">
            <strong>{entry.owner.displayName}</strong>
            {entry.live && <span className="home-discovery-card__status">Live</span>}
          </div>
          <span className="home-discovery-card__title">{entry.channel.title}</span>
          <span className="home-discovery-card__meta muted">
            {entry.channel.category ?? "Streaming"} · {audienceLabel}
          </span>
        </div>
      </Link>
    </li>
  );
}

function HomeDiscoveryStack({
  title,
  subtitle,
  channels,
  loading = false,
}: {
  title: string;
  subtitle: string;
  channels: DirectoryChannel[];
  loading?: boolean;
}) {
  if (loading) {
    return (
      <section className="home-discovery-stack surface" aria-busy="true">
        <div className="stack stack--2xs">
          <span className="home-section__eyebrow">Recommended</span>
          <h2>{title}</h2>
          <p className="muted">{subtitle}</p>
        </div>
        <div className="state-panel state-panel--loading">
          <strong>Loading live channels</strong>
          <p className="muted">Pulling the strongest live picks into this stack now.</p>
        </div>
      </section>
    );
  }

  if (channels.length === 0) {
    return (
      <section className="home-discovery-stack surface">
        <div className="stack stack--2xs">
          <span className="home-section__eyebrow">Recommended</span>
          <h2>{title}</h2>
          <p className="muted">{subtitle}</p>
        </div>
        <div className="state-panel">
          <strong>No channels in this stack yet</strong>
          <p className="muted">The next live creators worth opening will appear here automatically.</p>
          <div className="browse-actions">
            <Link href="/browse" className="secondary-button">
              Browse all live channels
            </Link>
          </div>
        </div>
      </section>
    );
  }

  return (
    <section className="home-discovery-stack surface">
      <div className="stack stack--2xs">
        <span className="home-section__eyebrow">Recommended</span>
        <h2>{title}</h2>
        <p className="muted">{subtitle}</p>
      </div>

      <ol className="home-discovery-stack__list">
        {channels.map((entry) => (
          <HomeDiscoveryCard key={entry.channel.id} entry={entry} />
        ))}
      </ol>
    </section>
  );
}

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
  const primaryDiscoveryTitle =
    homeData.isAuthenticated && following.length > 0 ? "Because you follow these channels" : "Live channels we think you'll like";
  const primaryDiscoverySubtitle =
    homeData.isAuthenticated && following.length > 0
      ? "Your regular creators bubble up first so the homepage feels personal the moment you land."
      : "A quick stack of live rooms worth opening right now, without making you scan the full directory first.";
  const primaryDiscoveryEyebrow = homeData.isAuthenticated && following.length > 0 ? "Your lineup" : "Recommended";
  const homepageHeadline =
    homeData.isAuthenticated && following.length > 0 ? "Your community is live right now" : "Live channels worth opening right now";
  const homepageLede = homeData.isAuthenticated && following.length > 0
    ? "Start with a featured broadcast, jump back into channels you already follow, then roam the categories pulling energy across the network."
    : "BitRiver Live should feel like a streaming destination the second you arrive: featured live content up front, strong channel recommendations beside it, and fast paths into the categories with the most momentum.";
  const secondaryAction = homeData.isAuthenticated ? { href: "/following", label: "Open following" } : { href: "#live-now", label: "Watch live now" };
  const topicLinks = categories.slice(0, 6);
  const discoveryStackChannels = (primaryDiscoveryChannels.length > 0 ? primaryDiscoveryChannels : liveNow).slice(0, 4);
  const homepageStats = [
    { label: "Featured", value: Math.max(featured.length, 1).toLocaleString() },
    { label: "Live now", value: liveNow.length.toLocaleString() },
    { label: "Categories", value: categories.length.toLocaleString() },
  ];
  const quickLinks = [
    { href: "#recommended", label: primaryDiscoveryTitle },
    { href: "#live-now", label: "Live now" },
    { href: "#top-categories", label: "Categories" },
    { href: "/videos", label: "Videos" },
    { href: homeData.isAuthenticated ? "/creator/getting-started" : "/browse", label: homeData.isAuthenticated ? "Go live" : "Browse all" },
  ];
  const homeErrorMessage = homeError
    ? `We couldn't load the personalized discovery rows right now: ${homeError}`
    : null;
  const directoryErrorMessage = directoryError ? `We couldn't load the directory right now: ${directoryError}` : null;

  return (
    <div className="home-page">
      <section className="home-hero">
        <div className="home-hero__layout">
          <div className="home-hero__main stack stack--lg">
            <div className="home-hero__copy stack stack--lg">
              <div className="stack stack--xs">
                <span className="home-hero__eyebrow">Live discovery</span>
                <h1>{homepageHeadline}</h1>
                <p className="home-hero__lede muted">{homepageLede}</p>
              </div>

              <div className="home-hero__actions">
                {homeData.isAuthenticated ? (
                  <Link href="/creator/getting-started" className="primary-button">
                    Go live
                  </Link>
                ) : (
                  <AuthActionLink mode="signup" className="primary-button">
                    Create account
                  </AuthActionLink>
                )}
                <Link href={secondaryAction.href} className="secondary-button">
                  {secondaryAction.label}
                </Link>
              </div>

              <div className="home-hero__search-panel">
                <div className="stack stack--2xs">
                  <span className="home-hero__search-label">Search channels, creators, categories, and tags</span>
                  <span className="muted">Drop into a specific creator fast, or use the homepage shelves to browse like a streaming platform should.</span>
                </div>
                <DirectorySearchBar defaultValue={query} />
              </div>

              <nav aria-label="Popular topics and quick actions" className="home-hero__quick-links">
                {topicLinks.map((category) => (
                  <Link
                    key={category.name}
                    href={`/browse?topic=${encodeURIComponent(category.name)}`}
                    className="pill pill--ghost"
                  >
                    {category.name}
                  </Link>
                ))}
                {quickLinks.map((item) => (
                  <Link key={item.href} href={item.href} className="pill pill--ghost">
                    {item.label}
                  </Link>
                ))}
              </nav>
            </div>

            <div className="home-hero__featured">
              <div className="home-hero__aside-header">
                <div className="stack stack--2xs">
                  <span className="home-hero__eyebrow">Featured live</span>
                  <h2>Start with the stream setting the pace right now</h2>
                </div>
                <p className="muted">A standout broadcast at the top, then the rest of the homepage fans out into shelves and category pivots.</p>
              </div>
              <FeaturedChannel channels={featured} loading={homeLoading} />
            </div>
          </div>

          <aside className="home-hero__aside">
            <HomeDiscoveryStack
              title={primaryDiscoveryTitle}
              subtitle={primaryDiscoverySubtitle}
              channels={discoveryStackChannels}
              loading={homeLoading}
            />

            <div className="home-promo surface">
              <div className="stack stack--2xs">
                <span className="home-section__eyebrow">{homeData.isAuthenticated ? "Your tools" : "Join the network"}</span>
                <h2>{homeData.isAuthenticated ? "Stay close to the channels you follow, then go live yourself" : "Create an account, follow channels, and launch your own stream"}</h2>
                <p className="muted">
                  {homeData.isAuthenticated
                    ? "Your account already unlocks follow lists, tipping, replays, and the creator setup flow. Keep browsing or head straight into studio tools."
                    : "Accounts turn the homepage into a personal streaming hub: follow channels, jump into chat, tip creators, and unlock the self-serve go-live flow."}
                </p>
              </div>

              <dl className="home-hero__stats home-promo__stats" aria-label="Homepage snapshot">
                {homepageStats.map((stat) => (
                  <div key={stat.label} className="home-stat">
                    <dt className="home-stat__label">{stat.label}</dt>
                    <dd className="home-stat__value">{stat.value}</dd>
                  </div>
                ))}
              </dl>

              <div className="home-promo__actions">
                {homeData.isAuthenticated ? (
                  <Link href="/creator/getting-started" className="primary-button">
                    Open creator setup
                  </Link>
                ) : (
                  <AuthActionLink mode="signup" className="primary-button">
                    Create account
                  </AuthActionLink>
                )}
                <Link href="/videos" className="secondary-button">
                  Browse videos
                </Link>
              </div>
            </div>
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
            title="Trending communities"
            subtitle="The channels and categories gathering momentum across the network right now."
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
              <h2>{query ? "Search results" : "More live channels"}</h2>
              <p className="muted">
                {query
                  ? `Showing channels that match "${query}".`
                  : "Once the homepage shelves narrow the field, use the full directory to keep hunting for the exact creator, tag, or category you want."}
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
