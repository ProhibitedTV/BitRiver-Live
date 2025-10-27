"use client";

import { useEffect, useState } from "react";
import { DirectoryGrid } from "../components/DirectoryGrid";
import type { DirectoryChannel } from "../lib/viewer-api";
import { fetchDirectory } from "../lib/viewer-api";

export default function DirectoryPage() {
  const [channels, setChannels] = useState<DirectoryChannel[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | undefined>();

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        setLoading(true);
        setError(undefined);
        const data = await fetchDirectory();
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
  }, []);

  return (
    <div className="container stack">
      <header className="stack">
        <h1>Discover live channels</h1>
        <p className="muted">
          Explore the latest community broadcasts, follow your favourite creators, and jump into ultra-low-latency playback powered by BitRiver Live.
        </p>
      </header>
      {loading && <div className="surface">Loading channels…</div>}
      {error && <div className="surface">{error}</div>}
      {!loading && !error && <DirectoryGrid channels={channels} />}
    </div>
  );
}
