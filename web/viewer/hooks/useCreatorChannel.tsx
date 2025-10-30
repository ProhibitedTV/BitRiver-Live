"use client";

import { ReactNode, createContext, useContext } from "react";
import type { ChannelPlaybackResponse } from "../lib/viewer-api";

type CreatorChannelContextValue = {
  channelId: string;
  playback?: ChannelPlaybackResponse;
  loading: boolean;
  error?: string;
  reload: (silent?: boolean) => Promise<void>;
};

const CreatorChannelContext = createContext<CreatorChannelContextValue | undefined>(undefined);

export function CreatorChannelProvider({
  value,
  children,
}: {
  value: CreatorChannelContextValue;
  children: ReactNode;
}) {
  return <CreatorChannelContext.Provider value={value}>{children}</CreatorChannelContext.Provider>;
}

export function useCreatorChannel(): CreatorChannelContextValue {
  const ctx = useContext(CreatorChannelContext);
  if (!ctx) {
    throw new Error("useCreatorChannel must be used within a CreatorChannelProvider");
  }
  return ctx;
}
