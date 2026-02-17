import { useCallback, useEffect, useRef, useState } from "react";
import type { DirectoryChannel } from "../../lib/viewer-api";
import { fetchFollowingChannels } from "../../lib/viewer-api";
import type { FollowingStatus } from "./FollowingState";

type UseFollowingChannelsOptions = {
  isAuthenticated: boolean;
  authLoading: boolean;
  refreshIntervalMs?: number;
};

type UseFollowingChannelsResult = {
  channels: DirectoryChannel[];
  status: FollowingStatus;
  error?: string;
  reload: () => Promise<void>;
};

export function useFollowingChannels({
  isAuthenticated,
  authLoading,
  refreshIntervalMs,
}: UseFollowingChannelsOptions): UseFollowingChannelsResult {
  const [channels, setChannels] = useState<DirectoryChannel[]>([]);
  const [status, setStatus] = useState<FollowingStatus>("loading");
  const [error, setError] = useState<string | undefined>();
  const mountedRef = useRef(true);

  const reload = useCallback(async () => {
    if (authLoading) {
      setStatus("loading");
      return;
    }

    if (!isAuthenticated) {
      setChannels([]);
      setError(undefined);
      setStatus("unauthenticated");
      return;
    }

    setStatus("loading");
    setError(undefined);

    try {
      const response = await fetchFollowingChannels();
      if (!mountedRef.current) {
        return;
      }
      setChannels(response.channels);
      setStatus(response.channels.length === 0 ? "empty" : "ready");
    } catch (err) {
      if (!mountedRef.current) {
        return;
      }
      setChannels([]);
      setError(err instanceof Error ? err.message : "Unable to load followed channels");
      setStatus("error");
    }
  }, [authLoading, isAuthenticated]);

  useEffect(() => {
    mountedRef.current = true;
    void reload();

    if (!refreshIntervalMs) {
      return () => {
        mountedRef.current = false;
      };
    }

    const intervalId = setInterval(() => {
      void reload();
    }, refreshIntervalMs);

    return () => {
      mountedRef.current = false;
      clearInterval(intervalId);
    };
  }, [refreshIntervalMs, reload]);

  return { channels, status, error, reload };
}
