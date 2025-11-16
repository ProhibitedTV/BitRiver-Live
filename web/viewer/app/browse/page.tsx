"use client";

import { useEffect, useState } from "react";
import { DirectoryGrid } from "../../components/DirectoryGrid";
import { SearchBar } from "../../components/SearchBar";
import type { DirectoryChannel } from "../../lib/viewer-api";
import { fetchDirectory, searchDirectory } from "../../lib/viewer-api";

export default function BrowsePage() {
  const [channels, setChannels] = useState<DirectoryChannel[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | undefined>();
  const [query, setQuery] = useState("");

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        setLoading(true);
        setError(undefined);
        const response = query.trim().length > 0 ? await searchDirectory(query) : await fetchDirectory();
        if (!cancelled) {
          setChannels(response.channels);
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

  return (
    <div className="container stack">
      <header className="stack">
        <h1>Browse channels</h1>
        <p className="muted">Search the full directory of BitRiver Live creators.</p>
        <SearchBar onSearch={setQuery} defaultValue={query} />
      </header>

      {loading && <div className="surface">Loading channels…</div>}
      {error && (
        <div className="surface" role="alert">
          {error}
        </div>
      )}
      {!loading && !error && channels.length === 0 && (
        <div className="surface">No channels found. Try another search or check back soon.</div>
      )}
      {!loading && !error && channels.length > 0 && <DirectoryGrid channels={channels} />}
    </div>
  );
}
