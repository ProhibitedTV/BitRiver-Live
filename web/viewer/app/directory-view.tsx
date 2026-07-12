import Link from "next/link";
import { CategoryRail } from "../components/CategoryRail";
import { DirectoryGrid } from "../components/DirectoryGrid";
import { DirectorySearchBar } from "../components/DirectorySearchBar";
import { FeaturedChannel } from "../components/FeaturedChannel";
import { LiveNowGrid } from "../components/LiveNowGrid";
import { AuthActionLink } from "../components/auth/AuthActionLink";
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
  const { featured, liveNow, categories, error: homeError } = homeData;
  const { channels, error: directoryError } = directoryData;
  const hasLiveChannels = liveNow.length > 0 || featured.length > 0;
  const hasBrowseContent = hasLiveChannels || channels.length > 0 || categories.length > 0;
  const heroTitle = hasLiveChannels
    ? "Watch live channels"
    : hasBrowseContent
      ? "Browse channels"
      : "Start the first stream";
  const heroLede = hasLiveChannels
    ? "Live rooms, replays, and creator tools are all close by. Start with what is on air now."
    : hasBrowseContent
      ? "Find an active channel or set up your own stream when you are ready."
      : "This install is ready. Create a channel, connect OBS, and the first stream will appear here.";
  const primaryHeroAction = hasLiveChannels
    ? { href: "#live-now", label: "Watch live" }
    : hasBrowseContent
      ? { href: "#directory", label: "Browse channels" }
      : { href: "/creator/getting-started", label: "Start streaming" };
  const directoryHeading = query ? "Search results" : "Channel directory";
  const directorySummary = query ? `Matching "${query}".` : "All public channels on this install.";
  const networkState = homeLoading ? "Syncing" : hasLiveChannels ? "Signal live" : "Relay ready";

  return (
    <div className="home-page home-page--simple">
      <section className="home-hero home-hero--simple">
        <div className="home-network-status" role="status" aria-label="BitRiver network status">
          <div className="home-network-status__identity">
            <span className="home-network-status__marker" aria-hidden="true" />
            <span>BitRiver public relay</span>
          </div>
          <dl className="home-network-status__metrics">
            <div>
              <dt>Node</dt>
              <dd>BR-LIVE-01</dd>
            </div>
            <div>
              <dt>Live signals</dt>
              <dd>{homeLoading ? "--" : liveNow.length}</dd>
            </div>
            <div>
              <dt>Directory</dt>
              <dd>{directoryLoading ? "--" : channels.length}</dd>
            </div>
          </dl>
          <span className={`home-network-status__signal${hasLiveChannels ? " home-network-status__signal--live" : ""}`}>
            <span aria-hidden="true" />
            {networkState}
          </span>
        </div>
        <div className="home-hero__layout home-hero__layout--simple">
          <div className="home-hero__main home-hero__main--simple stack stack--lg">
            <div className="home-hero__copy stack stack--md">
              <div className="stack stack--xs">
                <span className="home-hero__eyebrow">Community-owned live network</span>
                <h1>{heroTitle}</h1>
                <p className="home-hero__lede muted">{heroLede}</p>
              </div>

              <div className="home-hero__actions">
                <Link href={primaryHeroAction.href} className="primary-button">
                  {primaryHeroAction.label}
                </Link>
                {homeData.isAuthenticated ? (
                  <Link href="/creator/getting-started" className="secondary-button">
                    Go live
                  </Link>
                ) : (
                  <AuthActionLink mode="signup" className="secondary-button">
                    Create account
                  </AuthActionLink>
                )}
              </div>

              <div className="home-hero__search-panel">
                <div className="stack stack--2xs">
                  <span className="home-hero__search-label">Find a channel</span>
                  <span className="muted">Search by channel, creator, category, or tag.</span>
                </div>
                <DirectorySearchBar defaultValue={query} />
              </div>
            </div>

            <div className="home-hero__featured">
              <div className="home-hero__aside-header">
                <div className="stack stack--2xs">
                  <span className="home-hero__eyebrow">Priority signal</span>
                  <h2>Featured relay</h2>
                </div>
                <p className="muted">The network&apos;s selected live broadcast.</p>
              </div>
              <FeaturedChannel channels={featured} loading={homeLoading} />
            </div>
          </div>
        </div>
      </section>

      <div className="home-sections home-sections--simple">
        {!homeLoading && homeError ? (
          <div className="state-panel state-panel--error" role="alert">
            <strong>Discovery unavailable</strong>
            <p className="muted">{homeError}</p>
          </div>
        ) : null}

        <section className="home-section surface" id="live-now">
          <div className="home-section__header">
            <div className="stack stack--2xs">
              <span className="home-section__eyebrow">On air</span>
              <h2>Live now</h2>
              <p className="muted">Channels currently broadcasting.</p>
            </div>
            {!homeLoading && liveNow.length > 0 ? <span className="muted">{liveNow.length} live</span> : null}
          </div>
          <LiveNowGrid channels={liveNow} loading={homeLoading} />
        </section>

        <CategoryRail id="top-categories" categories={categories} loading={homeLoading} />

        <section className="home-section surface" id="directory">
          <div className="home-section__header">
            <div className="stack stack--2xs">
              <span className="home-section__eyebrow">Browse</span>
              <h2>{directoryHeading}</h2>
              <p className="muted">{directorySummary}</p>
            </div>
            {!directoryLoading && !directoryError && channels.length > 0 ? <span className="muted">{channels.length} channels</span> : null}
          </div>

          {directoryLoading ? (
            <div className="state-panel state-panel--loading" aria-busy="true">
              <strong>Loading channels</strong>
              <p className="muted">Refreshing the directory.</p>
            </div>
          ) : directoryError ? (
            <div className="state-panel state-panel--error" role="alert">
              <strong>Directory unavailable</strong>
              <p className="muted">{directoryError}</p>
            </div>
          ) : channels.length === 0 && !query ? (
            <div className="state-panel">
              <strong>No channels yet</strong>
              <p className="muted">Create a channel and go live to make the directory useful.</p>
              <div className="home-hero__spotlight-actions">
                <Link href="/creator/getting-started" className="primary-button">
                  Start streaming
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
