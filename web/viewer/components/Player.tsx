"use client";

import { useEffect, useId, useRef } from "react";
import Hls from "hls.js";
import type { Playback } from "../lib/viewer-api";

export function Player({ playback }: { playback?: Playback }) {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const playerId = useId();

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
      hls.on(Hls.Events.ERROR, (_, data) => {
        if (data.fatal) {
          hls.destroy();
        }
      });
      return () => {
        hls.destroy();
      };
    }

    return undefined;
  }, [playback]);

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
