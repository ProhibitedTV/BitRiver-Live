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
    topicFromParams,
    lastQueryFromParams,
    lastTopicFromParams,
    navigateWithDirectoryState,
  } = useDirectorySearch({
    fallbackPathname: "/browse",
  });
  const [channels, setChannels] = useState<DirectoryChannel[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | undefined>();
  const [query, setQuery] = useState("");
  const [sort, setSort] = useState<SortKey>("live");
  const [filter, setFilter] = useState<FilterKey>(null);
  const [knownFilters, setKnownFilters] = useState<string[]>([]);
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

    void loadChannels(query, filter ?? "");
  }, [filter, loadChannels, query, queryHydrated]);

  useEffect(() => {
    const queryChanged = lastQueryFromParams.current !== searchParamQuery;
    const topicChanged = lastTopicFromParams.current !== topicFromParams;

    if (!queryHydrated || queryChanged || topicChanged) {
      lastQueryFromParams.current = searchParamQuery;
      lastTopicFromParams.current = topicFromParams;
      setQuery(searchParamQuery);
      setFilter(topicFromParams);
      setQueryHydrated(true);
    }
  }, [lastQueryFromParams, lastTopicFromParams, queryHydrated, searchParamQuery, topicFromParams]);

  const discoveredFilters = useMemo(() => {
    const filters = new Set<string>();
    channels.forEach((entry) => {
      if (entry.channel.category) {
        filters.add(entry.channel.category);
      }
      entry.channel.tags.forEach((tag) => filters.add(tag));
    });
    return Array.from(filters).sort((a, b) => a.localeCompare(b));
  }, [channels]);

  useEffect(() => {
    if (discoveredFilters.length === 0) {
      return;
    }

    setKnownFilters((currentFilters) => {
      const mergedFilters = new Set(currentFilters);
      discoveredFilters.forEach((nextFilter) => mergedFilters.add(nextFilter));
      const nextFilters = Array.from(mergedFilters).sort((a, b) => a.localeCompare(b));
      const unchanged =
        nextFilters.length === currentFilters.length &&
        nextFilters.every((nextFilter, index) => nextFilter === currentFilters[index]);

      return unchanged ? currentFilters : nextFilters;
    });
  }, [discoveredFilters]);

  const visibleFilters = useMemo(() => {
    const filters = knownFilters.length > 0 ? knownFilters : discoveredFilters;
    if (!filter || filters.includes(filter)) {
      return filters;
    }

    return [filter, ...filters];
  }, [discoveredFilters, filter, knownFilters]);

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

  const handleSearch = (value: string) => {
    const next = navigateWithDirectoryState({ query: value, topic: filter });
    setQuery(next.normalizedQuery);
  };

  const handleReset = () => {
    const next = navigateWithDirectoryState({ query: "", topic: null });
    setFilter(next.normalizedTopic);
    setQuery(next.normalizedQuery);
    setSort("live");
  };

  const handleFilterSelect = (nextFilter: FilterKey) => {
    const next = navigateWithDirectoryState({ query, topic: nextFilter });
    setFilter(next.normalizedTopic);
  };

  const showEmpty = !loading && !error && sortedChannels.length === 0;
  const resultLabel = `${sortedChannels.length.toLocaleString()} channel${sortedChannels.length === 1 ? "" : "s"}`;
  const resultSummary = `${filter ? `${filter} - ` : ""}${resultLabel}`;
  const resultsHeading = query ? `Search results for "${query}"` : filter ? `${filter} channels` : "Live directory";
  const hasActiveControls = Boolean(query || filter || sort !== "live");

  return (
    <div className="container container--wide browse-page browse-page--compact stack stack--lg">
      <section className="surface stack stack--md browse-controls" aria-labelledby="browse-title">
        <div className="browse-controls__header">
          <div>
            <span className="page-eyebrow">Browse</span>
            <h1 id="browse-title">Browse live channels</h1>
          </div>
          {!loading && !error && <span className="browse-controls__count">{resultSummary}</span>}
        </div>

        <SearchBar onSearch={handleSearch} defaultValue={query} onClear={handleReset} />

        <div className="browse-toolbar__row">
          <div className="chip-row" role="tablist" aria-label="Sort directory">
            {[
              { key: "live", label: "Live" },
              { key: "trending", label: "Trending" },
              { key: "new", label: "New" },
            ].map((option) => (
              <button
                key={option.key}
                role="tab"
                aria-selected={sort === option.key}
                className={`chip chip--tab ${sort === option.key ? "chip--active" : ""}`}
                onClick={() => setSort(option.key as SortKey)}
              >
                <span className="chip__label">{option.label}</span>
              </button>
            ))}
          </div>

          {hasActiveControls && (
            <button className="secondary-button" onClick={handleReset}>
              Reset
            </button>
          )}
        </div>

        <div className="chip-row chip-row--wrap" aria-label="Filter by category or tag">
          <button
            className={`chip ${filter === null ? "chip--active" : ""}`}
            onClick={() => handleFilterSelect(null)}
            aria-pressed={filter === null}
          >
            All
          </button>
          {visibleFilters.map((category) => (
            <button
              key={category}
              className={`chip ${filter === category ? "chip--active" : ""}`}
              onClick={() => handleFilterSelect(category)}
              aria-pressed={filter === category}
            >
              {category}
            </button>
          ))}
        </div>
      </section>

      <section className="browse-results stack stack--md">
        <div className="browse-results__header">
          <div>
            <h2>{resultsHeading}</h2>
            {hasActiveControls && <p className="muted">Filtered by the controls above.</p>}
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
                <button className="primary-button" onClick={() => void loadChannels(query, filter ?? "")}>
                  Retry
                </button>
                <button className="secondary-button" onClick={handleReset}>
                  Reset
                </button>
              </div>
            </div>
          </div>
        )}

        {showEmpty && (
          <div className="surface surface--empty">
            <div className="stack">
              <h2>No channels match your filters</h2>
              <p className="muted">Clear the filters or try another search.</p>
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
