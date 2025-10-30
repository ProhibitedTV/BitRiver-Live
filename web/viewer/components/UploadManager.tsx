"use client";

import { FormEvent, useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "../hooks/useAuth";
import {
  UploadItem,
  createUpload,
  deleteUpload,
  fetchChannelUploads,
} from "../lib/viewer-api";

type UploadManagerProps = {
  channelId: string;
  ownerId: string;
};

export function UploadManager({ channelId, ownerId }: UploadManagerProps) {
  const router = useRouter();
  const { user, loading: authLoading } = useAuth();
  const [items, setItems] = useState<UploadItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | undefined>();
  const [formError, setFormError] = useState<string | undefined>();
  const [submitting, setSubmitting] = useState(false);

  const hasCreatorRole = user?.roles?.includes("creator") ?? false;
  const canManage = !!user && (user.id === ownerId || hasCreatorRole);

  useEffect(() => {
    if (authLoading) {
      return;
    }
    if (!user || !canManage) {
      router.replace(`/channels/${channelId}`);
    }
  }, [authLoading, canManage, channelId, router, user]);

  const load = useCallback(
    async (silent = false) => {
      if (!silent) {
        setLoading(true);
      }
      setError(undefined);
      try {
        const response = await fetchChannelUploads(channelId);
        setItems(response ?? []);
      } catch (err) {
        const message = err instanceof Error ? err.message : "Unable to load uploads";
        setError(message);
      } finally {
        if (!silent) {
          setLoading(false);
        }
      }
    },
    [channelId],
  );

  useEffect(() => {
    if (canManage) {
      void load();
    } else {
      setItems([]);
    }
  }, [canManage, load]);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canManage) {
      return;
    }
    const form = event.currentTarget;
    const data = new FormData(form);
    const sizeRaw = data.get("sizeBytes")?.toString() ?? "";
    const metadataRaw = data.get("metadata")?.toString() ?? "";
    let metadata: Record<string, string> | undefined;
    if (metadataRaw.trim().length > 0) {
      try {
        const parsed = JSON.parse(metadataRaw);
        if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
          throw new Error("Metadata must be a JSON object");
        }
        metadata = parsed as Record<string, string>;
      } catch (err) {
        setFormError(err instanceof Error ? err.message : "Invalid metadata");
        return;
      }
    }
    const sizeBytes = Number.parseInt(sizeRaw || "0", 10) || 0;
    const payload = {
      channelId,
      title: data.get("title")?.toString() ?? "",
      filename: data.get("filename")?.toString() ?? "",
      playbackUrl: data.get("playbackUrl")?.toString() ?? "",
      sizeBytes,
      metadata,
    };
    try {
      setFormError(undefined);
      setSubmitting(true);
      await createUpload(payload);
      form.reset();
      await load(true);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unable to create upload";
      setFormError(message);
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!canManage) {
      return;
    }
    try {
      await deleteUpload(id);
      await load(true);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unable to delete upload";
      setError(message);
    }
  };

  if (authLoading || !canManage) {
    return null;
  }

  return (
    <section className="surface stack">
      <header className="stack">
        <h3>Upload manager</h3>
        <p className="muted">Register new VODs and review processing progress.</p>
      </header>
      <form className="stack" onSubmit={handleSubmit}>
        <label className="stack">
          <span>Title</span>
          <input name="title" type="text" placeholder="Community recap" />
        </label>
        <label className="stack">
          <span>Filename</span>
          <input name="filename" type="text" placeholder="recap.mp4" />
        </label>
        <label className="stack">
          <span>Playback URL</span>
          <input name="playbackUrl" type="url" placeholder="https://cdn.example.com/recap.m3u8" />
        </label>
        <label className="stack">
          <span>Size (bytes)</span>
          <input name="sizeBytes" type="number" min="0" step="1" placeholder="0" />
        </label>
        <label className="stack">
          <span>Metadata (JSON)</span>
          <textarea name="metadata" rows={3} placeholder='{"source":"upload"}' />
        </label>
        {formError && <p className="error">{formError}</p>}
        <button type="submit" className="primary" disabled={submitting}>
          {submitting ? "Submitting…" : "Register upload"}
        </button>
      </form>
      <div className="stack">
        <div className="upload-actions">
          <button type="button" className="secondary-button" onClick={() => load()} disabled={loading}>
            {loading ? "Refreshing…" : "Refresh"}
          </button>
          {error && <span className="error">{error}</span>}
        </div>
        {!loading && items.length === 0 && !error && (
          <p className="muted">No uploads yet.</p>
        )}
        {items.length > 0 && (
          <ul className="upload-list">
            {items.map((item) => (
              <li key={item.id} className="upload-card">
                <div className="upload-card__header">
                  <strong>{item.title || item.filename}</strong>
                  <span className="muted">{new Date(item.createdAt).toLocaleString()}</span>
                </div>
                <p className="muted">
                  {item.status.replace(/_/g, " ")} · {item.progress}% · {Math.round(item.sizeBytes / 1_000_000)} MB
                </p>
                {item.error && <p className="error">{item.error}</p>}
                <div className="upload-card__actions">
                  <button type="button" className="secondary-button" onClick={() => handleDelete(item.id)}>
                    Delete
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}
