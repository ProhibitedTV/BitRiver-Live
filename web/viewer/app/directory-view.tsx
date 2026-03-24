import Image from "next/image";
import Link from "next/link";
import { CategoryRail } from "../components/CategoryRail";
import { ChannelRail } from "../components/ChannelRail";
import { ChannelStatusBadge } from "../components/channel/ChannelStatusBadge";
import { DirectoryGrid } from "../components/DirectoryGrid";
import { DirectorySearchBar } from "../components/DirectorySearchBar";
import { FeaturedChannel } from "../components/FeaturedChannel";
import { LiveNowGrid } from "../components/LiveNowGrid";
import { formatFollowerLabel, formatViewerLabel, getChannelPreviewImage } from "../lib/channel-presenters";
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

function dedupeChannels(channels: DirectoryChannel[]) {
  const seen = new Set<string>();
  return channels.filter((entry) => {
    if (seen.has(entry.channel.id)) {
      return false;
    }
    seen.add(entry.channel.id);
    return true;
  });
}

function HeroQueueCard({ entry, priority = false }: { entry: DirectoryChannel; priority?: boolean }) {
  const previewImage = getChannelPreviewImage(entry);
  const audienceLabel = entry.live
    ? formatViewerLabel(entry.viewerCount ?? 0)
    : formatFollowerLabel(entry.followerCount);

  return (
    <Link href={`/channels/${entry.channel.id}`} className="home-hero__queue-card">
      <div className="home-hero__queue-media">
        {previewImage ? (
          <Image
            src={previewImage}
            alt={`${entry.owner.displayName} channel artwork`}
            fill
            sizes="(min-width: 1280px) 18vw, (min-width: 960px) 24vw, 100vw"
            className="home-hero__queue-image"
            priority={priority}
            loading={priority ? undefined : "lazy"}
          />
        ) : (
          <div className="home-hero__queue-fallback" aria-hidden="true" />
        )}
        <div className="overlay overlay--top overlay--scrim">
          <ChannelStatusBadge live={entry.live} />
          <span className="overlay__meta">{entry.channel.category ?? "Streaming"}</span>
        </div>
      </div>
      <div className="home-hero__queue-body">
        <h3>{entry.channel.title}</h3>
        <p className="muted">{entry.owner.displayName}</p>
        <span className="home-hero__queue-meta muted">{audienceLabel}</span>
      </div>
    </Link>
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
  const heroStats = [
    { label: "Live now", value: liveNow.length.toLocaleString() },
    { label: "Featured", value: featured.length.toLocaleString() },
    { label: homeData.isAuthenticated ? "Following" : "Recommended", value: (homeData.isAuthenticated ? following.length : recommended.length).toLocaleString() },
    { label: "Categories", value: categories.length.toLocaleString() },
  ];
  const quickLinks = [
    { href: "#live-now", label: "Live now" },
    { href: "#recommended", label: "Recommended" },
    { href: "#top-categories", label: "Categories" },
    { href: "#directory", label: query ? "Search results" : "Full directory" },
  ];
  const featuredIds = new Set(featured.map((entry) => entry.channel.id));
  const heroCandidates = dedupeChannels([...liveNow, ...recommended, ...featured]);
  const heroQueue = heroCandidates.filter((entry) => !featuredIds.has(entry.channel.id)).slice(0, 4);
  const heroQueueChannels = heroQueue.length > 0 ? heroQueue : heroCandidates.slice(0, 4);
  const homeErrorMessage = homeError
    ? `We couldn't load the personalized discovery rows right now: ${homeError}`
    : null;
  const directoryErrorMessage = directoryError ? `We couldn't load the directory right now: ${directoryError}` : null;

  return (
    <div className="home-page">
      <section className="home-hero">
        <div className="home-hero__layout">
          <div className="home-hero__spotlight">
            <div className="home-hero__spotlight-header">
              <div className="stack stack--xs home-hero__spotlight-copy">
                <span className="home-hero__eyebrow">Featured live</span>
                <h1>Start with creators already on air</h1>
                <p className="home-hero__lede muted">
                  Featured broadcasts lead the page, live picks stay in reach, and the full directory is still one move away.
                </p>
              </div>
              <div className="home-hero__spotlight-actions">
                <a href="#live-now" className="primary-button">
                  Watch live now
                </a>
                <a href="#directory" className="secondary-button">
                  Browse all channels
                </a>
              </div>
            </div>
            <FeaturedChannel channels={featured} loading={homeLoading} />
          </div>

          <aside className="home-hero__side">
            <section className="home-hero__editorial">
              <div className="stack stack--sm">
                <div className="stack stack--2xs">
                  <span className="home-hero__eyebrow">Jump straight in</span>
                  <h2>Video first, browse second</h2>
                </div>
                <p className="muted">
                  Creator previews now carry the first impression, while search and category browse stay ready when you want to go deeper.
                </p>
              </div>

              <dl className="home-hero__stats" aria-label="Platform snapshot">
                {heroStats.map((stat) => (
                  <div key={stat.label} className="home-stat">
                    <dt className="home-stat__label">{stat.label}</dt>
                    <dd className="home-stat__value">{stat.value}</dd>
                  </div>
                ))}
              </dl>
            </section>

            <section className="home-hero__queue" aria-labelledby="home-hero-queue-title">
              <div className="home-hero__queue-header">
                <div className="stack stack--2xs">
                  <span className="home-hero__eyebrow">On deck</span>
                  <h2 id="home-hero-queue-title">{liveNow.length > 0 ? "Live right now" : "Watch next"}</h2>
                </div>
                {!homeLoading && heroQueueChannels.length > 0 && <span className="muted">{heroQueueChannels.length} quick picks</span>}
              </div>

              {homeLoading ? (
                <div className="state-panel state-panel--loading" aria-busy="true">
                  <strong>Loading live picks</strong>
                  <p className="muted">Pulling active channels into the front page stack now.</p>
                </div>
              ) : heroQueueChannels.length === 0 ? (
                <div className="state-panel">
                  <strong>No quick picks yet</strong>
                  <p className="muted">As soon as creators start broadcasting, they will surface here beside the featured stream.</p>
                </div>
              ) : (
                <div className="home-hero__queue-grid">
                  {heroQueueChannels.map((entry, index) => (
                    <HeroQueueCard key={entry.channel.id} entry={entry} priority={index < 2} />
                  ))}
                </div>
              )}
            </section>

            <section className="home-hero__utility">
              <div className="stack stack--2xs">
                <span className="home-hero__eyebrow">Search and browse</span>
                <h2>Find a creator, category, or tag</h2>
                <p className="muted">Search stays on the homepage so you can pivot fast once a stream catches your attention.</p>
              </div>
              <DirectorySearchBar defaultValue={query} />
              <nav aria-label="Quick jump links" className="home-hero__quick-links">
                {quickLinks.map((item) => (
                  <a key={item.href} href={item.href} className="pill pill--ghost">
                    {item.label}
                  </a>
                ))}
              </nav>
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

        <section className="home-section surface" id="live-now">
          <div className="home-section__header">
            <div className="stack stack--2xs">
              <span className="home-section__eyebrow">On air</span>
              <h2>Live now</h2>
              <p className="muted">Creators currently streaming and ready to watch immediately.</p>
            </div>
            {!homeLoading && liveNow.length > 0 && <span className="muted">{liveNow.length} live streams</span>}
          </div>
          <LiveNowGrid channels={liveNow} loading={homeLoading} />
        </section>

        <ChannelRail
          id="recommended"
          title="Recommended for you"
          subtitle="Momentum channels lined up right after the featured live stage."
          channels={recommended}
          loading={homeLoading}
          eyebrow="Personalized picks"
        />

        <div className="home-section-grid">
          <CategoryRail id="top-categories" categories={categories} loading={homeLoading} />
          <ChannelRail
            id="trending"
            title="Trending now"
            subtitle="Streams pulling attention across the network right now."
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
                  : "Scan the full lineup once you are ready to move beyond the curated rows."}
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
