"use client";

import { useEffect, useState } from "react";
import { DirectoryGrid } from "../components/DirectoryGrid";
import { FeaturedChannel } from "../components/FeaturedChannel";
import { FollowingRail } from "../components/FollowingRail";
import { LiveNowGrid } from "../components/LiveNowGrid";
import { SearchBar } from "../components/SearchBar";
import { CategoryRail } from "../components/CategoryRail";
import { ChannelRail } from "../components/ChannelRail";
import type { DirectoryChannel } from "../lib/viewer-api";
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
import type { CategorySummary } from "../lib/viewer-api";

export default function DirectoryPage() {
  const [channels, setChannels] = useState<DirectoryChannel[]>([]);
  const [featured, setFeatured] = useState<DirectoryChannel[]>([]);
  const [recommended, setRecommended] = useState<DirectoryChannel[]>([]);
  const [following, setFollowing] = useState<DirectoryChannel[]>([]);
  const [liveNow, setLiveNow] = useState<DirectoryChannel[]>([]);
  const [trending, setTrending] = useState<DirectoryChannel[]>([]);
  const [categories, setCategories] = useState<CategorySummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [homeLoading, setHomeLoading] = useState(true);
  const [error, setError] = useState<string | undefined>();
  const [homeError, setHomeError] = useState<string | undefined>();
  const [query, setQuery] = useState("");

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        setLoading(true);
        setError(undefined);
        const data = query.trim().length > 0 ? await searchDirectory(query) : await fetchDirectory();
        if (!cancelled) {
          setChannels(data.channels);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Unable to load directory");
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
  }, [query]);

  useEffect(() => {
    let cancelled = false;
    const loadSlices = async () => {
      try {
        setHomeLoading(true);
        setHomeError(undefined);
        const [
          featuredResponse,
          followingResponse,
          liveResponse,
          recommendedResponse,
          trendingResponse,
          topCategoriesResponse,
        ] = await Promise.all([
          fetchFeaturedChannels(),
          fetchFollowingChannels(),
          fetchLiveNowChannels(),
          fetchRecommendedChannels(),
          fetchTrendingChannels(),
          fetchTopCategories(),
        ]);
        if (!cancelled) {
          setFeatured(featuredResponse.channels);
          setRecommended(recommendedResponse.channels);
          setFollowing(followingResponse.channels);
          setLiveNow(liveResponse.channels);
          setTrending(trendingResponse.channels);
          setCategories(topCategoriesResponse.categories ?? []);
        }
      } catch (err) {
        if (!cancelled) {
          setHomeError(err instanceof Error ? err.message : "Unable to load personalised rows");
        }
      } finally {
        if (!cancelled) {
          setHomeLoading(false);
        }
      }
    };
    void loadSlices();
    return () => {
      cancelled = true;
    };
  }, []);

  const handleSearch = (value: string) => {
    setQuery(value);
  };

  return (
    <div className="home-page">
      <section className="home-hero">
        <div className="home-hero__background" aria-hidden="true">
          <span className="home-hero__layer home-hero__layer--aurora" />
          <span className="home-hero__layer home-hero__layer--grid" />
          <span className="home-hero__layer home-hero__layer--orb" />
        </div>
        <div className="home-hero__inner container container--wide">
          <div className="home-hero__content stack">
            <div className="home-hero__chips">
              <span className="badge">Seasonal spotlight</span>
              <span className="badge badge--live">Happening now</span>
            </div>
            <h1>Discover live channels</h1>
            <p className="muted">
              Explore community broadcasts, follow your favourite creators, and jump into ultra-low-latency playback powered
              by BitRiver Live.
            </p>
            <div className="home-hero__actions">
              <a className="primary-button" href="#live-now">
                Start watching
              </a>
              <a className="secondary-button" href="#directory">
                Browse directory
              </a>
            </div>
            <div className="home-hero__search">
              <SearchBar onSearch={handleSearch} defaultValue={query} />
              <p className="muted">Surface creators by category, title, or tag.</p>
            </div>
          </div>

          <div className="home-hero__media">
            <FeaturedChannel channel={featured[0]} loading={homeLoading} />
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
        {homeError && (
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
          {loading && <div className="surface">Loading channels…</div>}
          {error && (
            <div className="surface" role="alert">
              {error}
            </div>
          )}
          {!loading && !error && <DirectoryGrid channels={channels} />}
        </section>
      </div>
    </div>
  );
}
