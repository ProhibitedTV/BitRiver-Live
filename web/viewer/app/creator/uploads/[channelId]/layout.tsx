"use client";

import { ReactNode, useCallback, useEffect, useState } from "react";
import { CreatorChannelProvider } from "../../../../hooks/useCreatorChannel";
import type { ChannelPlaybackResponse } from "../../../../lib/viewer-api";
import { fetchChannelPlayback } from "../../../../lib/viewer-api";

type LayoutProps = {
  params: { channelId: string };
  children: ReactNode;
};

export default function CreatorUploadsLayout({ params, children }: LayoutProps) {
  const { channelId } = params;
  const [playback, setPlayback] = useState<ChannelPlaybackResponse | undefined>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | undefined>();

  const load = useCallback(
    async (silent = false) => {
      if (!silent) {
        setLoading(true);
      }
      setError(undefined);
      try {
        const response = await fetchChannelPlayback(channelId);
        setPlayback(response);
      } catch (err) {
        setPlayback(undefined);
        const message = err instanceof Error ? err.message : "Unable to load channel";
        setError(message);
      } finally {
        setLoading(false);
      }
    },
    [channelId],
  );

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <CreatorChannelProvider value={{ channelId, playback, loading, error, reload: load }}>
      {children}
    </CreatorChannelProvider>
  );
}
