"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { DirectoryGrid } from "../../components/DirectoryGrid";
import { SearchBar } from "../../components/SearchBar";
import type { DirectoryChannel } from "../../lib/viewer-api";
import { loadDirectoryChannels, mapDirectoryError } from "../../lib/directory-state";
import { useDirectorySearch } from "../../hooks/useDirectorySearch";

type SortKey = "live" | "trending" | "new";
type FilterKey = string | null;

export default function BrowsePage() {
  const {
    queryFromParams: searchParamQuery,
    categoryFromParams,
    lastQueryFromParams,
    lastCategoryFromParams,
    navigateWithDirectoryState,
    navigateWithQuery,
  } = useDirectorySearch({
    fallbackPathname: "/browse",
  });
  const [channels, setChannels] = useState<DirectoryChannel[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | undefined>();
  const [query, setQuery] = useState("");
  const [sort, setSort] = useState<SortKey>("live");
  const [filter, setFilter] = useState<FilterKey>(null);
  const [queryHydrated, setQueryHydrated] = useState(false);

  const loadChannels = useCallback(async (search = "", category = "") => {
    try {
      setLoading(true);
      setError(undefined);
      const response = await loadDirectoryChannels(search, category);
      setChannels(response.channels);
    } catch (err) {
      setError(mapDirectoryError(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!queryHydrated) {
      return;
    }

    void loadChannels(query, categoryFromParams);
  }, [categoryFromParams, loadChannels, query, queryHydrated]);

  useEffect(() => {
    if (
      !queryHydrated ||
      lastQueryFromParams.current !== searchParamQuery ||
      lastCategoryFromParams.current !== categoryFromParams
    ) {
      lastQueryFromParams.current = searchParamQuery;
      lastCategoryFromParams.current = categoryFromParams;
      setFilter(categoryFromParams || null);
      setQuery(searchParamQuery);
      setQueryHydrated(true);
    }
  }, [categoryFromParams, lastCategoryFromParams, lastQueryFromParams, queryHydrated, searchParamQuery]);

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
    const list = channels
      .map((entry) => ({
        entry,
        createdAtTs: new Date(entry.channel.createdAt).getTime(),
      }))
      .filter(({ entry }) => {
        if (!filter) {
          return true;
        }
        return entry.channel.category === filter || entry.channel.tags.includes(filter);
      });

    return list
      .sort((a, b) => {
        if (sort === "new") {
          return b.createdAtTs - a.createdAtTs;
        }

        const viewersA = a.entry.viewerCount ?? 0;
        const viewersB = b.entry.viewerCount ?? 0;

        if (sort === "trending") {
          return viewersB - viewersA;
        }

        if (sort === "live") {
          if (a.entry.live !== b.entry.live) {
            return Number(b.entry.live) - Number(a.entry.live);
          }
          return viewersB - viewersA;
        }

        return 0;
      })
      .map(({ entry }) => entry);
  }, [channels, filter, sort]);

  const featuredChannels = useMemo(() => {
    const liveChannels = sortedChannels.filter((entry) => entry.live);
    if (liveChannels.length > 0) {
      return liveChannels.slice(0, 3);
    }
    return sortedChannels.slice(0, 3);
  }, [sortedChannels]);

  const handleSearch = (value: string) => {
    const normalizedQuery = navigateWithQuery(value);
    setQuery(normalizedQuery);
    setFilter(null);
  };

  const handleReset = () => {
    const next = navigateWithDirectoryState({ query: "", category: "" });
    setFilter(next.normalizedCategory || null);
    setQuery(next.normalizedQuery);
    setSort("live");
  };

  const showEmpty = !loading && !error && sortedChannels.length === 0;
  const resultSummary = `${sortedChannels.length.toLocaleString()} result${sortedChannels.length === 1 ? "" : "s"}`;

  return (
    <div className="container container--wide browse-page stack stack--xl">
      <header className="page-header surface surface--glow">
        <div className="page-header__copy stack stack--sm">
          <div className="stack stack--2xs">
            <span className="page-eyebrow">Browse</span>
            <h1>Browse every channel</h1>
          </div>
          <p className="muted">
            Use search, sort, and topic filters to move from a broad scan to the exact stream you want.
          </p>
        </div>

        <div className="page-header__stats">
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
      </header>

      <section className="surface stack stack--md browse-controls">
        <div className="section-heading">
          <div>
            <h2>Search and filter the directory</h2>
            <p className="muted">Search by creator, category, or tags, then refine the results below.</p>
          </div>
          {(query || filter || sort !== "live") && (
            <button className="secondary-button" onClick={handleReset}>
              Reset directory
            </button>
          )}
        </div>

        <SearchBar onSearch={handleSearch} defaultValue={query} onClear={handleReset} />

        <div className="browse-toolbar__row">
          <div className="chip-row" role="tablist" aria-label="Sort directory">
            {[
              { key: "live", label: "Live", description: "See live channels first" },
              { key: "trending", label: "Trending", description: "Sort by viewers" },
              { key: "new", label: "New", description: "Recently created" },
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

          <span className="muted">{loading ? "Loading directory..." : resultSummary}</span>
        </div>

        <div className="chip-row chip-row--wrap" aria-label="Filter by category or tag">
          <button className={`chip ${filter === null ? "chip--active" : ""}`} onClick={() => setFilter(null)} aria-pressed={filter === null}>
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

      {!loading && !error && featuredChannels.length > 0 && (
        <section className="surface stack stack--md">
          <div className="section-heading">
            <div>
              <h2>Featured right now</h2>
              <p className="muted">A small shortlist when you want a fast answer instead of a full scan.</p>
            </div>
            <span className="muted">{featuredChannels.length} highlights</span>
          </div>

          <div className="browse-hero__rail">
            {featuredChannels.map((entry) => {
              const viewers = entry.viewerCount ?? 0;
              return (
                <div key={entry.channel.id} className="featured-card">
                  <div className="featured-card__header">
                    <div className={`badge ${entry.live ? "badge--live" : "badge--muted"}`}>{entry.live ? "Live" : "Offline"}</div>
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
        </section>
      )}

      <section className="browse-results stack stack--md">
        <div className="section-heading">
          <div>
            <h2>{query ? `Results for "${query}"` : "All channels"}</h2>
            <p className="muted">
              {query
                ? "Review the filtered lineup below or keep refining the search."
                : "Scan the full network and open a channel when it looks promising."}
            </p>
          </div>
          {!loading && !error && <span className="muted">{resultSummary}</span>}
        </div>

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
              <p className="muted">Try a different query, switch tabs, or clear your filters to see more of BitRiver Live.</p>
              <div className="browse-actions">
                <button className="primary-button" onClick={handleReset}>
                  Clear filters
                </button>
                <button className="secondary-button" onClick={() => setSort("live")}>
                  Back to Live
                </button>
              </div>
            </div>
          </div>
        )}

        {!loading && !error && !showEmpty && <DirectoryGrid channels={sortedChannels} />}
      </section>
    </div>
  );
}
