"use client";

import { useEffect, useState } from "react";
import { DirectoryGrid } from "../components/DirectoryGrid";
import { FeaturedChannel } from "../components/FeaturedChannel";
import { FollowingRail } from "../components/FollowingRail";
import { LiveNowGrid } from "../components/LiveNowGrid";
import { SearchBar } from "../components/SearchBar";
import type { DirectoryChannel } from "../lib/viewer-api";
import {
  fetchDirectory,
  fetchFeaturedChannels,
  fetchFollowingChannels,
  fetchLiveNowChannels,
  searchDirectory,
} from "../lib/viewer-api";

export default function DirectoryPage() {
  const [channels, setChannels] = useState<DirectoryChannel[]>([]);
  const [featured, setFeatured] = useState<DirectoryChannel[]>([]);
  const [following, setFollowing] = useState<DirectoryChannel[]>([]);
  const [liveNow, setLiveNow] = useState<DirectoryChannel[]>([]);
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
        const [featuredResponse, followingResponse, liveResponse] = await Promise.all([
          fetchFeaturedChannels(),
          fetchFollowingChannels(),
          fetchLiveNowChannels(),
        ]);
        if (!cancelled) {
          setFeatured(featuredResponse.channels);
          setFollowing(followingResponse.channels);
          setLiveNow(liveResponse.channels);
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
    <div className="container stack home-page">
      <section className="home-hero">
        <div className="home-hero__intro stack">
          <h1>Discover live channels</h1>
          <p className="muted">
            Explore community broadcasts, follow your favourite creators, and jump into ultra-low-latency playback powered by
            BitRiver Live.
          </p>
          <SearchBar onSearch={handleSearch} defaultValue={query} />
        </div>
        <FeaturedChannel channel={featured[0]} loading={homeLoading} />
      </section>

      {homeError && (
        <div className="surface" role="alert">
          {homeError}
        </div>
      )}

      <FollowingRail channels={following} loading={homeLoading} />

      <section className="stack">
        <div className="section-heading">
          <div>
            <h2>Live now</h2>
            <p className="muted">Creators currently on air</p>
          </div>
          {!homeLoading && liveNow.length > 0 && <span className="muted">{liveNow.length} streams</span>}
        </div>
        <LiveNowGrid channels={liveNow} loading={homeLoading} />
      </section>

      <section className="stack">
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
  );
}
