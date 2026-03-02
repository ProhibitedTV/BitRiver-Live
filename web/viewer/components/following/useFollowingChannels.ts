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
  const channelsRef = useRef<DirectoryChannel[]>(channels);
  const statusRef = useRef<FollowingStatus>(status);
  const errorRef = useRef<string | undefined>(error);

  useEffect(() => {
    channelsRef.current = channels;
  }, [channels]);

  useEffect(() => {
    statusRef.current = status;
  }, [status]);

  useEffect(() => {
    errorRef.current = error;
  }, [error]);

  const updateStatusIfChanged = useCallback((nextStatus: FollowingStatus) => {
    if (statusRef.current === nextStatus) {
      return;
    }
    statusRef.current = nextStatus;
    setStatus(nextStatus);
  }, []);

  const updateErrorIfChanged = useCallback((nextError: string | undefined) => {
    if (errorRef.current === nextError) {
      return;
    }
    errorRef.current = nextError;
    setError(nextError);
  }, []);

  const channelsSemanticallyEqual = useCallback(
    (nextChannels: DirectoryChannel[]) => {
      const currentChannels = channelsRef.current;
      if (currentChannels.length !== nextChannels.length) {
        return false;
      }

      for (let index = 0; index < currentChannels.length; index += 1) {
        if (currentChannels[index]?.id !== nextChannels[index]?.id) {
          return false;
        }
      }

      return true;
    },
    [],
  );

  const reload = useCallback(async () => {
    if (authLoading) {
      updateStatusIfChanged("loading");
      return;
    }

    if (!isAuthenticated) {
      if (!channelsSemanticallyEqual([])) {
        channelsRef.current = [];
        setChannels([]);
      }
      updateErrorIfChanged(undefined);
      updateStatusIfChanged("unauthenticated");
      return;
    }

    updateStatusIfChanged("loading");
    updateErrorIfChanged(undefined);

    try {
      const response = await fetchFollowingChannels();
      if (!mountedRef.current) {
        return;
      }
      if (!channelsSemanticallyEqual(response.channels)) {
        channelsRef.current = response.channels;
        setChannels(response.channels);
      }
      updateStatusIfChanged(response.channels.length === 0 ? "empty" : "ready");
    } catch (err) {
      if (!mountedRef.current) {
        return;
      }
      if (!channelsSemanticallyEqual([])) {
        channelsRef.current = [];
        setChannels([]);
      }
      updateErrorIfChanged(err instanceof Error ? err.message : "Unable to load followed channels");
      updateStatusIfChanged("error");
    }
  }, [
    authLoading,
    channelsSemanticallyEqual,
    isAuthenticated,
    updateErrorIfChanged,
    updateStatusIfChanged,
  ]);

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
