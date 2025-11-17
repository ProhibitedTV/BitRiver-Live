"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { DirectoryGrid } from "../../components/DirectoryGrid";
import { SearchBar } from "../../components/SearchBar";
import type { DirectoryChannel } from "../../lib/viewer-api";
import { fetchDirectory, searchDirectory } from "../../lib/viewer-api";

type SortKey = "live" | "trending" | "new";
type FilterKey = string | null;

export default function BrowsePage() {
  const [channels, setChannels] = useState<DirectoryChannel[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | undefined>();
  const [query, setQuery] = useState("");
  const [sort, setSort] = useState<SortKey>("live");
  const [filter, setFilter] = useState<FilterKey>(null);

  const loadChannels = useCallback(
    async (search?: string) => {
      try {
        setLoading(true);
        setError(undefined);
        const response = search?.trim().length
          ? await searchDirectory(search)
          : await fetchDirectory();
        setChannels(response.channels);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Unable to load directory");
      } finally {
        setLoading(false);
      }
    },
    []
  );

  useEffect(() => {
    void loadChannels(query);
  }, [loadChannels, query]);

  const categoryFilters = useMemo(() => {
    const filters = new Set<string>();
    channels.forEach((entry) => {
      if (entry.channel.category) {
        filters.add(entry.channel.category);
      }
      entry.channel.tags.forEach((tag) => filters.add(tag));
    });
    return Array.from(filters).sort((a, b) => a.localeCompare(b));
  }, [channels]);

  const sortedChannels = useMemo(() => {
    const list = channels.filter((entry) => {
      if (!filter) return true;
      return entry.channel.category === filter || entry.channel.tags.includes(filter);
    });

    return list.sort((a, b) => {
      if (sort === "new") {
        return new Date(b.channel.createdAt).getTime() - new Date(a.channel.createdAt).getTime();
      }

      const viewersA = a.viewerCount ?? 0;
      const viewersB = b.viewerCount ?? 0;

      if (sort === "trending") {
        return viewersB - viewersA;
      }

      if (sort === "live") {
        if (a.live !== b.live) {
          return Number(b.live) - Number(a.live);
        }
        return viewersB - viewersA;
      }

      return 0;
    });
  }, [channels, filter, sort]);

  const featuredChannels = useMemo(() => {
    const liveChannels = sortedChannels.filter((entry) => entry.live);
    if (liveChannels.length > 0) {
      return liveChannels.slice(0, 3);
    }
    return sortedChannels.slice(0, 3);
  }, [sortedChannels]);

  const handleSearch = (value: string) => {
    setQuery(value);
    setFilter(null);
  };

  const handleReset = () => {
    setFilter(null);
    void loadChannels(query);
  };

  const showEmpty = !loading && !error && sortedChannels.length === 0;

  return (
    <div className="container container--wide browse-page stack">
      <section className="hero browse-hero surface surface--glow">
        <div className="stack">
          <div className="stack hero__intro">
            <div className="pill pill--frost">Live directory</div>
            <h1>Find your next live channel</h1>
            <p className="muted">
              Discover featured creators, trending streams, and new arrivals. Filter by category or tags to tailor the
              experience.
            </p>
          </div>

          <div className="browse-hero__actions">
            <SearchBar onSearch={handleSearch} defaultValue={query} />
            <div className="browse-hero__stats">
              <div className="stat-pill">
                <span className="stat-pill__label">Live now</span>
                <strong className="stat-pill__value">{channels.filter((entry) => entry.live).length}</strong>
              </div>
              <div className="stat-pill">
                <span className="stat-pill__label">Total channels</span>
                <strong className="stat-pill__value">{channels.length}</strong>
              </div>
              <div className="stat-pill">
                <span className="stat-pill__label">Categories</span>
                <strong className="stat-pill__value">{categoryFilters.length}</strong>
              </div>
            </div>
          </div>
        </div>

        <div className="browse-hero__featured">
          <div className="browse-hero__eyebrow">Featured / Live highlights</div>
          <div className="browse-hero__rail">
            {loading && <div className="featured-card featured-card--loading" />}
            {!loading && featuredChannels.length === 0 && <div className="featured-card">No featured streams yet.</div>}
            {!loading &&
              featuredChannels.map((entry) => {
                const viewers = entry.viewerCount ?? 0;
                return (
                  <div key={entry.channel.id} className="featured-card">
                    <div className="featured-card__header">
                      <div className="badge badge--live">{entry.live ? "Live" : "Offline"}</div>
                      <span className="overlay__meta">{`${viewers.toLocaleString()} watching`}</span>
                    </div>
                    <h3>{entry.channel.title}</h3>
                    <p className="muted">{entry.channel.category ?? "Streaming"}</p>
                    <div className="featured-card__footer">
                      <span className="pill pill--tag">{entry.owner.displayName}</span>
                      {entry.channel.tags.slice(0, 2).map((tag) => (
                        <span key={tag} className="pill pill--tag">
                          #{tag}
                        </span>
                      ))}
                    </div>
                  </div>
                );
              })}
          </div>
        </div>
      </section>

      <section className="stack browse-controls">
        <div className="chip-row" role="tablist" aria-label="Sort directory">
          {[
            { key: "live", label: "Live", description: "See live channels first" },
            { key: "trending", label: "Trending", description: "Sort by viewers" },
            { key: "new", label: "New", description: "Recently created" }
          ].map((option) => (
            <button
              key={option.key}
              role="tab"
              aria-selected={sort === option.key}
              className={`chip chip--tab ${sort === option.key ? "chip--active" : ""}`}
              onClick={() => setSort(option.key as SortKey)}
            >
              <span className="chip__label">{option.label}</span>
              <span className="chip__hint">{option.description}</span>
            </button>
          ))}
        </div>

        <div className="chip-row chip-row--wrap" aria-label="Filter by category or tag">
          <button
            className={`chip ${filter === null ? "chip--active" : ""}`}
            onClick={() => setFilter(null)}
            aria-pressed={filter === null}
          >
            All
          </button>
          {categoryFilters.map((category) => (
            <button
              key={category}
              className={`chip ${filter === category ? "chip--active" : ""}`}
              onClick={() => setFilter(category)}
              aria-pressed={filter === category}
            >
              {category}
            </button>
          ))}
        </div>
      </section>

      {loading && (
        <div className="grid directory-grid" aria-label="Loading channels">
          {Array.from({ length: 8 }).map((_, index) => (
            <div key={index} className="directory-card directory-card--skeleton">
              <div className="directory-card__preview skeleton" />
              <div className="directory-card__content">
                <div className="skeleton skeleton--text" />
                <div className="skeleton skeleton--text skeleton--short" />
                <div className="skeleton skeleton--text" />
              </div>
              <div className="directory-card__footer">
                <div className="skeleton skeleton--chip" />
                <div className="skeleton skeleton--button" />
              </div>
            </div>
          ))}
        </div>
      )}

      {error && (
        <div className="surface surface--alert" role="alert">
          <div className="stack">
            <h2>We hit a snag</h2>
            <p className="muted">{error}</p>
            <div className="browse-actions">
              <button className="primary-button" onClick={() => void loadChannels(query)}>
                Retry loading
              </button>
              <button className="secondary-button" onClick={handleReset}>
                Reset filters
              </button>
            </div>
          </div>
        </div>
      )}

      {showEmpty && (
        <div className="surface surface--empty">
          <div className="stack">
            <h2>No channels match your filters</h2>
            <p className="muted">
              Try a different query, switch tabs, or clear your filters to see more of BitRiver Live.
            </p>
            <div className="browse-actions">
              <button className="primary-button" onClick={handleReset}>
                Clear filters
              </button>
              <button className="secondary-button" onClick={() => setSort("live")}>Back to Live</button>
            </div>
          </div>
        </div>
      )}

      {!loading && !error && !showEmpty && <DirectoryGrid channels={sortedChannels} />}
    </div>
  );
}
