"use client";

import { useEffect, useId, useRef, useState } from "react";
import Hls from "hls.js";
import { reportViewerQoE } from "../lib/viewer-api";
import type { Playback } from "../lib/viewer-api";
import { UIState } from "./player-ui-state";

type PlayerProps = {
  playback?: Playback;
  channelId: string;
  live?: boolean;
  liveState?: string;
  loading?: boolean;
};

export function Player({ playback, channelId, live = false, liveState, loading = false }: PlayerProps) {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const playerId = useId();
  const lastSampleRef = useRef<number>(0);
  const [runtimeState, setRuntimeState] = useState<UIState>(loading ? UIState.LoadingStream : UIState.StreamEnded);

  const setUIState = (state: UIState) => {
    setRuntimeState(state);
  };

  useEffect(() => {
    if (!playback) {
      return;
    }
    setUIState(UIState.LoadingStream);
    if (playback.protocol === "webrtc") {
      let instance: any;
      const setup = async () => {
        try {
          const mod = await import("ovenplayer");
          const OvenPlayer = mod.default ?? (mod as any);
          instance = OvenPlayer.create(playerId, {
            autoStart: true,
            mute: false,
            sources: [
              {
                type: "webrtc",
                file: playback.playbackUrl
              }
            ]
          });
          setUIState(UIState.LiveHealthy);
        } catch {
          setUIState(UIState.StreamUnavailable);
        }
      };
      void setup();
      void reportViewerQoE({
        channelId,
        sessionId: playback.sessionId,
        event: "player_init",
        player: "ovenplayer",
        protocol: playback.protocol ?? "webrtc",
        latencyMode: playback.latencyMode
      });
      return () => {
        if (instance && typeof instance.remove === "function") {
          instance.remove();
        }
      };
    }

    const video = videoRef.current;
    if (!video || !playback.playbackUrl) {
      setUIState(UIState.StreamUnavailable);
      return;
    }

    if (video.canPlayType("application/vnd.apple.mpegurl")) {
      video.src = playback.playbackUrl;
      void video.play().catch(() => {
        setUIState(UIState.StreamUnavailable);
      });
      void reportViewerQoE({
        channelId,
        sessionId: playback.sessionId,
        event: "player_init",
        player: "native",
        protocol: playback.protocol ?? "hls",
        latencyMode: playback.latencyMode,
        playbackUrl: playback.playbackUrl
      });
      return () => {
        video.pause();
        video.removeAttribute("src");
        video.load();
      };
    }

    if (Hls.isSupported()) {
      const hls = new Hls({ lowLatencyMode: playback.latencyMode === "low-latency" });
      hls.loadSource(playback.playbackUrl);
      hls.attachMedia(video);
      void reportViewerQoE({
        channelId,
        sessionId: playback.sessionId,
        event: "player_init",
        player: "hls.js",
        protocol: playback.protocol ?? "hls",
        latencyMode: playback.latencyMode,
        playbackUrl: playback.playbackUrl
      });
      hls.on(Hls.Events.LEVEL_SWITCHED, (_, data) => {
        const level = hls.levels[data.level];
        const rendition = level?.name || (level?.height ? `${level.height}p` : `level-${data.level}`);
        void reportViewerQoE({
          channelId,
          sessionId: playback.sessionId,
          event: "rendition_change",
          player: "hls.js",
          protocol: playback.protocol ?? "hls",
          latencyMode: playback.latencyMode,
          rendition
        });
      });
      hls.on(Hls.Events.ERROR, (_, data) => {
        if (data.fatal) {
          setUIState(UIState.StreamUnavailable);
          hls.destroy();
        }
        void reportViewerQoE({
          channelId,
          sessionId: playback.sessionId,
          event: "player_error",
          player: "hls.js",
          protocol: playback.protocol ?? "hls",
          latencyMode: playback.latencyMode,
          error: data?.type ? String(data.type) : undefined
        });
      });
      return () => {
        hls.destroy();
      };
    }

    return undefined;
  }, [channelId, playback, playerId]);

  useEffect(() => {
    if (!playback || playback.protocol === "webrtc") {
      return;
    }

    const video = videoRef.current;
    if (!video) {
      return;
    }

    const payloadBase = {
      channelId,
      sessionId: playback.sessionId,
      player: Hls.isSupported() ? "hls.js" : "native",
      protocol: playback.protocol ?? "hls",
      latencyMode: playback.latencyMode,
      playbackUrl: playback.playbackUrl
    };

    const bufferedSeconds = () => {
      if (!video.buffered || video.buffered.length === 0) {
        return 0;
      }
      const end = video.buffered.end(video.buffered.length - 1);
      return Math.max(0, end - video.currentTime);
    };

    const report = (event: string, extra: Record<string, unknown> = {}) => {
      const droppedFrames =
        typeof video.getVideoPlaybackQuality === "function"
          ? video.getVideoPlaybackQuality().droppedVideoFrames
          : undefined;
      void reportViewerQoE({
        ...payloadBase,
        event,
        currentTime: video.currentTime,
        duration: video.duration,
        bufferedSeconds: bufferedSeconds(),
        droppedFrames,
        ...extra
      });
    };

    const onPlaying = () => {
      setUIState(UIState.LiveHealthy);
      report("playing");
    };
    const onWaiting = () => {
      setUIState(UIState.Reconnecting);
      report("buffering");
    };
    const onStalled = () => {
      setUIState(UIState.Reconnecting);
      report("stalled");
    };
    const onPause = () => report("paused");
    const onEnded = () => {
      setUIState(UIState.StreamEnded);
      report("ended");
    };
    const onError = () => {
      const errorMessage = video.error?.message ?? "unknown";
      setUIState(UIState.StreamUnavailable);
      report("player_error", { error: errorMessage });
    };
    const onTimeUpdate = () => {
      const now = Date.now();
      if (now - lastSampleRef.current < 15000) {
        return;
      }
      lastSampleRef.current = now;
      report("playback_sample");
    };

    video.addEventListener("playing", onPlaying);
    video.addEventListener("waiting", onWaiting);
    video.addEventListener("stalled", onStalled);
    video.addEventListener("pause", onPause);
    video.addEventListener("ended", onEnded);
    video.addEventListener("error", onError);
    video.addEventListener("timeupdate", onTimeUpdate);

    return () => {
      video.removeEventListener("playing", onPlaying);
      video.removeEventListener("waiting", onWaiting);
      video.removeEventListener("stalled", onStalled);
      video.removeEventListener("pause", onPause);
      video.removeEventListener("ended", onEnded);
      video.removeEventListener("error", onError);
      video.removeEventListener("timeupdate", onTimeUpdate);
    };
  }, [channelId, playback]);

  const stateFromChannel = (): UIState => {
    if (loading) {
      return UIState.LoadingStream;
    }
    if (!playback) {
      if (live || liveState === "starting") {
        return UIState.StreamStartingSoon;
      }
      return UIState.StreamEnded;
    }
    if (runtimeState === UIState.StreamEnded) {
      return UIState.LoadingStream;
    }
    return runtimeState;
  };

  const uiState = stateFromChannel();

  const stateContent: Record<UIState, { icon: string; title: string; body: string }> = {
    [UIState.LoadingStream]: {
      icon: "⏳",
      title: "Loading stream",
      body: "We are preparing the player now."
    },
    [UIState.StreamStartingSoon]: {
      icon: "🛠️",
      title: "Stream starting soon",
      body: "The creator is live soon. Stay on this page and playback will begin when ready."
    },
    [UIState.LiveHealthy]: {
      icon: "🟢",
      title: "Live and healthy",
      body: "You are watching the live stream."
    },
    [UIState.Reconnecting]: {
      icon: "🔄",
      title: "Reconnecting",
      body: "The stream connection dropped briefly. We are reconnecting now."
    },
    [UIState.StreamEnded]: {
      icon: "✅",
      title: "Stream ended",
      body: "This stream has ended. Check back later for the next session."
    },
    [UIState.StreamUnavailable]: {
      icon: "⚠️",
      title: "Stream unavailable",
      body: "We couldn't play this stream right now. Please refresh or try again shortly."
    }
  };

  const renderStateScreen = (state: UIState) => {
    const content = stateContent[state];
    return (
      <div className="surface stack" role="status" aria-live="polite" data-testid="player-state-screen">
        <p aria-hidden="true" style={{ fontSize: "1.5rem", margin: 0 }}>
          {content.icon}
        </p>
        <h3>{content.title}</h3>
        <p className="muted">{content.body}</p>
      </div>
    );
  };

  if (!playback) {
    return renderStateScreen(uiState);
  }

  if (uiState === UIState.LoadingStream || uiState === UIState.StreamUnavailable) {
    return renderStateScreen(uiState);
  }

  if (uiState === UIState.StreamEnded || uiState === UIState.StreamStartingSoon) {
    return renderStateScreen(uiState);
  }

  if (playback.protocol === "webrtc") {
    if (uiState !== UIState.LiveHealthy) {
      return renderStateScreen(uiState);
    }
    return <div id={playerId} className="video-container webrtc-player" />;
  }

  return (
    <div className="stack">
      {renderStateScreen(uiState)}
      <div className="video-container">
        <video ref={videoRef} controls playsInline muted={false} poster={playback.originUrl ?? undefined} />
      </div>
    </div>
  );
}
