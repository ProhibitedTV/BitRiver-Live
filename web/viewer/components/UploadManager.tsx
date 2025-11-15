"use client";

import { ChangeEvent, DragEvent, FormEvent, useCallback, useEffect, useRef, useState } from "react";
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

type MetadataEntry = {
  id: string;
  key: string;
  value: string;
};

const FILE_METADATA_KEYS = ["contentType", "fileLastModified"];

export function UploadManager({ channelId, ownerId }: UploadManagerProps) {
  const router = useRouter();
  const { user, loading: authLoading } = useAuth();
  const [items, setItems] = useState<UploadItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | undefined>();
  const [formError, setFormError] = useState<string | undefined>();
  const [submitting, setSubmitting] = useState(false);
  const [formValues, setFormValues] = useState({
    title: "",
    filename: "",
    playbackUrl: "",
    sizeBytes: "",
  });
  const [metadataEntries, setMetadataEntries] = useState<MetadataEntry[]>([
    { id: "meta-0", key: "source", value: "upload" },
  ]);
  const metadataIdRef = useRef(1);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [uploadProgress, setUploadProgress] = useState<number | null>(null);
  const [isDragActive, setIsDragActive] = useState(false);
  const hasUploadSource = selectedFile !== null || formValues.playbackUrl.trim().length > 0;

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

  const upsertMetadataEntries = useCallback((pairs: Array<{ key: string; value: string }>) => {
    if (pairs.length === 0) {
      return;
    }
    const lookup = new Map(pairs.filter((pair) => pair.key).map((pair) => [pair.key, pair.value]));
    setMetadataEntries((current) => {
      const next = current.map((entry) => {
        if (!lookup.has(entry.key)) {
          return entry;
        }
        const value = lookup.get(entry.key) ?? entry.value;
        return { ...entry, value };
      });
      const existingKeys = new Set(current.map((entry) => entry.key));
      for (const [key, value] of lookup.entries()) {
        if (existingKeys.has(key)) {
          continue;
        }
        next.push({ id: `meta-${metadataIdRef.current++}`, key, value });
      }
      return next;
    });
  }, []);

  const handleFileSelection = useCallback(
    (files: FileList | null) => {
      if (!files || files.length === 0) {
        return;
      }
      const file = files[0];
      setSelectedFile(file);
      setUploadProgress(null);
      setFormValues((prev) => ({
        ...prev,
        title: prev.title || deriveTitleFromFilename(file.name),
        filename: file.name,
        sizeBytes: `${file.size}`,
      }));
      const pairs = [{ key: "source", value: "upload" }];
      if (file.type) {
        pairs.push({ key: "contentType", value: file.type });
      }
      if (file.lastModified) {
        pairs.push({ key: "fileLastModified", value: new Date(file.lastModified).toISOString() });
      }
      upsertMetadataEntries(pairs);
    },
    [upsertMetadataEntries],
  );

  const handleFileInputChange = (event: ChangeEvent<HTMLInputElement>) => {
    handleFileSelection(event.target.files);
    event.target.value = "";
  };

  const handleDragOver = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    event.stopPropagation();
    if (!isDragActive) {
      setIsDragActive(true);
    }
  };

  const handleDragLeave = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    event.stopPropagation();
    if (isDragActive) {
      setIsDragActive(false);
    }
  };

  const handleDrop = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    event.stopPropagation();
    setIsDragActive(false);
    handleFileSelection(event.dataTransfer?.files ?? null);
  };

  const handleInputChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { name, value } = event.target;
    setFormValues((prev) => ({
      ...prev,
      [name]: value,
    }));
  };

  const addMetadataEntry = () => {
    setMetadataEntries((current) => [...current, { id: `meta-${metadataIdRef.current++}`, key: "", value: "" }]);
  };

  const updateMetadataEntry = (id: string, field: "key" | "value", value: string) => {
    setMetadataEntries((current) => current.map((entry) => (entry.id === id ? { ...entry, [field]: value } : entry)));
  };

  const removeMetadataEntry = (id: string) => {
    setMetadataEntries((current) => {
      if (current.length === 1) {
        return current;
      }
      return current.filter((entry) => entry.id !== id);
    });
  };

  const clearSelectedFile = () => {
    setSelectedFile(null);
    setUploadProgress(null);
    setMetadataEntries((current) => {
      const next = current.filter((entry) => !FILE_METADATA_KEYS.includes(entry.key));
      if (next.length === 0) {
        return [{ id: `meta-${metadataIdRef.current++}`, key: "", value: "" }];
      }
      return next;
    });
    setFormValues((prev) => ({
      ...prev,
      filename: "",
      sizeBytes: "",
    }));
  };

  const handleDropzoneClick = () => {
    fileInputRef.current?.click();
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canManage) {
      return;
    }
    const metadata = metadataEntries.reduce<Record<string, string>>((acc, entry) => {
      const key = entry.key.trim();
      if (key.length === 0) {
        return acc;
      }
      acc[key] = entry.value.trim();
      return acc;
    }, {});
    const sizeBytes = Number.parseInt(formValues.sizeBytes || "0", 10);
    const payload = {
      channelId,
      title: formValues.title,
      filename: formValues.filename,
      playbackUrl: formValues.playbackUrl,
      sizeBytes: Number.isNaN(sizeBytes) ? undefined : sizeBytes,
      metadata: Object.keys(metadata).length > 0 ? metadata : undefined,
    };
    if (!selectedFile && !payload.playbackUrl) {
      setFormError("Select a media file or provide a playback URL");
      return;
    }
    try {
      setFormError(undefined);
      setSubmitting(true);
      setUploadProgress(selectedFile ? 0 : null);
      await createUpload(payload, selectedFile ? { file: selectedFile, onProgress: setUploadProgress } : undefined);
      setFormValues({ title: "", filename: "", playbackUrl: "", sizeBytes: "" });
      setMetadataEntries([{ id: "meta-0", key: "source", value: "upload" }]);
      setSelectedFile(null);
      setUploadProgress(null);
      await load(true);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unable to create upload";
      setFormError(message);
    } finally {
      setSubmitting(false);
      setUploadProgress(null);
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
          <span>Media</span>
          <div
            className={`upload-dropzone${isDragActive ? " upload-dropzone--active" : ""}${selectedFile ? " upload-dropzone--has-file" : ""}`}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onDrop={handleDrop}
            onClick={handleDropzoneClick}
            role="button"
            tabIndex={0}
            onKeyDown={(event) => {
              if (event.key === "Enter" || event.key === " ") {
                event.preventDefault();
                handleDropzoneClick();
              }
            }}
            aria-label="Upload media file"
          >
            <input ref={fileInputRef} type="file" className="sr-only" onChange={handleFileInputChange} />
            {selectedFile ? (
              <div className="upload-dropzone__file">
                <div>
                  <strong>{selectedFile.name}</strong>
                  <p className="muted">{formatFileSize(selectedFile.size)}</p>
                </div>
                <button
                  type="button"
                  className="secondary-button"
                  onClick={(event) => {
                    event.stopPropagation();
                    clearSelectedFile();
                  }}
                >
                  Clear
                </button>
              </div>
            ) : (
              <div className="upload-dropzone__hint">
                <p>Drag &amp; drop a file, or click to browse</p>
                <p className="muted">MP4, MOV, MKV</p>
              </div>
            )}
          </div>
          {uploadProgress !== null && (
            <div className="upload-progress">
              <div className="upload-progress__track">
                <div className="upload-progress__bar" style={{ width: `${uploadProgress}%` }} />
              </div>
              <span className="upload-progress__value">{uploadProgress}%</span>
            </div>
          )}
        </label>
        <label className="stack">
          <span>Title</span>
          <input
            name="title"
            type="text"
            placeholder="Community recap"
            value={formValues.title}
            onChange={handleInputChange}
          />
        </label>
        <label className="stack">
          <span>Filename</span>
          <input
            name="filename"
            type="text"
            placeholder="recap.mp4"
            value={formValues.filename}
            onChange={handleInputChange}
          />
        </label>
        <label className="stack">
          <span>Playback URL (optional)</span>
          <input
            name="playbackUrl"
            type="url"
            placeholder="https://cdn.example.com/recap.m3u8"
            value={formValues.playbackUrl}
            onChange={handleInputChange}
          />
        </label>
        <label className="stack">
          <span>Size (bytes)</span>
          <input
            name="sizeBytes"
            type="number"
            min="0"
            step="1"
            placeholder="0"
            value={formValues.sizeBytes}
            onChange={handleInputChange}
          />
        </label>
        <div className="stack">
          <span>Metadata</span>
          <div className="metadata-grid">
            {metadataEntries.map((entry) => (
              <div key={entry.id} className="metadata-row">
                <input
                  type="text"
                  placeholder="Key"
                  value={entry.key}
                  onChange={(event) => updateMetadataEntry(entry.id, "key", event.target.value)}
                />
                <input
                  type="text"
                  placeholder="Value"
                  value={entry.value}
                  onChange={(event) => updateMetadataEntry(entry.id, "value", event.target.value)}
                />
                <button
                  type="button"
                  className="metadata-row__remove"
                  onClick={() => removeMetadataEntry(entry.id)}
                  disabled={metadataEntries.length === 1}
                >
                  Remove
                </button>
              </div>
            ))}
          </div>
          <button type="button" className="secondary-button" onClick={addMetadataEntry}>
            Add metadata
          </button>
        </div>
        {formError && <p className="error">{formError}</p>}
        <button type="submit" className="primary-button" disabled={submitting || !hasUploadSource}>
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

function formatFileSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return "0 B";
  }
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  const precision = unitIndex === 0 ? 0 : 1;
  return `${value.toFixed(precision)} ${units[unitIndex]}`;
}

function deriveTitleFromFilename(name: string): string {
  if (!name) {
    return "";
  }
  const withoutExt = name.replace(/\.[^.]+$/, "");
  const cleaned = withoutExt.replace(/[-_]+/g, " ").trim();
  return cleaned || withoutExt || name;
}
