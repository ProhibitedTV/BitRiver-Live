"use client";

import { useEffect, useId, useRef } from "react";
import Hls from "hls.js";
import { reportViewerQoE } from "../lib/viewer-api";
import type { Playback } from "../lib/viewer-api";

export function Player({ playback, channelId }: { playback?: Playback; channelId: string }) {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const playerId = useId();
  const lastSampleRef = useRef<number>(0);

  useEffect(() => {
    if (!playback) {
      return;
    }
    if (playback.protocol === "webrtc") {
      let instance: any;
      const setup = async () => {
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
      return;
    }

    if (video.canPlayType("application/vnd.apple.mpegurl")) {
      video.src = playback.playbackUrl;
      void video.play().catch(() => {
        /* ignore autoplay errors */
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

    const onPlaying = () => report("playing");
    const onWaiting = () => report("buffering");
    const onStalled = () => report("stalled");
    const onPause = () => report("paused");
    const onEnded = () => report("ended");
    const onError = () => {
      const errorMessage = video.error?.message ?? "unknown";
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

  if (!playback) {
    return (
      <div className="surface stack">
        <h3>No active stream</h3>
        <p className="muted">
          The broadcaster is currently offline. Follow the channel to be notified when they go live again.
        </p>
      </div>
    );
  }

  if (playback.protocol === "webrtc") {
    return <div id={playerId} className="video-container webrtc-player" />;
  }

  return (
    <div className="video-container">
      <video ref={videoRef} controls playsInline muted={false} poster={playback.originUrl ?? undefined} />
    </div>
  );
}
