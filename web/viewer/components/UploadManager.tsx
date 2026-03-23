"use client";

import { ChangeEvent, DragEvent, FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useAuth } from "../hooks/useAuth";
import { UploadItem, ViewerApiError, createUpload, deleteUpload, fetchChannelUploads } from "../lib/viewer-api";
import { Badge } from "./ui/Badge";
import { Button, buttonClassName } from "./ui/Button";
import { Card, CardBody, CardHeader } from "./ui/Card";
import { EmptyState } from "./ui/EmptyState";
import { InlineAlert } from "./ui/InlineAlert";

type UploadManagerProps = { channelId: string; ownerId: string };
type MetadataEntry = { id: string; key: string; value: string };
type UploadPhase = "selecting" | "uploading" | "processing" | "ready" | "failed";
type UploadListFilter = "all" | "active" | "ready" | "failed";
type UploadProgressState = { percent: number; loadedBytes: number; totalBytes: number };
type UploadStatusPresentation = { label: string; summary: string; tone: "neutral" | "info" | "success" | "danger" };
type PendingUploadSnapshot = {
  payload: {
    channelId: string;
    title: string;
    filename: string;
    playbackUrl: string;
    sizeBytes: number | undefined;
    metadata: Record<string, string> | undefined;
  };
  file: File | null;
};

const FILE_METADATA_KEYS = ["contentType", "fileLastModified"];

export function UploadManager({ channelId, ownerId }: UploadManagerProps) {
  const router = useRouter();
  const { user, loading: authLoading, signIn } = useAuth();
  const [items, setItems] = useState<UploadItem[]>([]);
  const [listFilter, setListFilter] = useState<UploadListFilter>("all");
  const [searchTerm, setSearchTerm] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | undefined>();
  const [formError, setFormError] = useState<string | undefined>();
  const [submitting, setSubmitting] = useState(false);
  const [formValues, setFormValues] = useState({ title: "", filename: "", playbackUrl: "", sizeBytes: "" });
  const [metadataEntries, setMetadataEntries] = useState<MetadataEntry[]>([{ id: "meta-0", key: "source", value: "upload" }]);
  const metadataIdRef = useRef(1);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [uploadProgress, setUploadProgress] = useState<UploadProgressState | null>(null);
  const [isDragActive, setIsDragActive] = useState(false);
  const [uploadPhase, setUploadPhase] = useState<UploadPhase>("selecting");
  const [lastSubmission, setLastSubmission] = useState<PendingUploadSnapshot | null>(null);
  const uploadAbortControllerRef = useRef<AbortController | null>(null);

  const hasCreatorRole = user?.roles?.includes("creator") ?? false;
  const canManage = !!user && (user.id === ownerId || hasCreatorRole);
  const hasUploadSource = selectedFile !== null || formValues.playbackUrl.trim().length > 0;
  const phasePresentation = getUploadPhasePresentation(uploadPhase, hasUploadSource, uploadProgress?.percent);

  const sortedItems = useMemo(() => [...items].sort(compareUploadsForMonitoring), [items]);
  const activeItemCount = useMemo(() => items.filter((item) => isUploadInActiveState(item.status)).length, [items]);
  const readyItemCount = useMemo(() => items.filter((item) => isUploadItemReady(item.status)).length, [items]);
  const failedItemCount = useMemo(() => items.filter((item) => isUploadItemFailed(item.status)).length, [items]);
  const visibleItems = useMemo(() => {
    const normalizedSearch = searchTerm.trim().toLowerCase();
    return sortedItems.filter((item) => {
      if (listFilter === "active" && !isUploadInActiveState(item.status)) return false;
      if (listFilter === "ready" && !isUploadItemReady(item.status)) return false;
      if (listFilter === "failed" && !isUploadItemFailed(item.status)) return false;
      if (!normalizedSearch) return true;
      const title = item.title?.toLowerCase() ?? "";
      const filename = item.filename?.toLowerCase() ?? "";
      return title.includes(normalizedSearch) || filename.includes(normalizedSearch);
    });
  }, [listFilter, searchTerm, sortedItems]);

  useEffect(() => {
    if (authLoading) return;
    if (!user) {
      const timer = window.setTimeout(() => {
        void signIn();
      }, 500);
      return () => window.clearTimeout(timer);
    }
    if (!canManage) {
      router.replace(`/channels/${channelId}`);
    }
  }, [authLoading, canManage, channelId, router, signIn, user]);

  const load = useCallback(
    async (silent = false) => {
      if (!silent) setLoading(true);
      setError(undefined);
      try {
        setItems((await fetchChannelUploads(channelId)) ?? []);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Unable to load uploads");
      } finally {
        if (!silent) setLoading(false);
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
    if (pairs.length === 0) return;
    const lookup = new Map(pairs.filter((pair) => pair.key).map((pair) => [pair.key, pair.value]));
    setMetadataEntries((current) => {
      const next = current.map((entry) => (lookup.has(entry.key) ? { ...entry, value: lookup.get(entry.key) ?? entry.value } : entry));
      const existingKeys = new Set(current.map((entry) => entry.key));
      for (const [key, value] of lookup.entries()) {
        if (!existingKeys.has(key)) next.push({ id: `meta-${metadataIdRef.current++}`, key, value });
      }
      return next;
    });
  }, []);

  const handleFileSelection = useCallback(
    (files: FileList | null) => {
      if (!files || files.length === 0) return;
      const file = files[0];
      setSelectedFile(file);
      setUploadProgress(null);
      setUploadPhase("selecting");
      setFormError(undefined);
      setFormValues((prev) => ({
        ...prev,
        title: prev.title || deriveTitleFromFilename(file.name),
        filename: file.name,
        sizeBytes: `${file.size}`,
      }));
      const pairs = [{ key: "source", value: "upload" }];
      if (file.type) pairs.push({ key: "contentType", value: file.type });
      if (file.lastModified) pairs.push({ key: "fileLastModified", value: new Date(file.lastModified).toISOString() });
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
    if (!isDragActive) setIsDragActive(true);
  };

  const handleDragLeave = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    event.stopPropagation();
    if (isDragActive) setIsDragActive(false);
  };

  const handleDrop = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    event.stopPropagation();
    setIsDragActive(false);
    handleFileSelection(event.dataTransfer?.files ?? null);
  };

  const handleInputChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { name, value } = event.target;
    setFormValues((prev) => ({ ...prev, [name]: value }));
  };

  const addMetadataEntry = () => {
    setMetadataEntries((current) => [...current, { id: `meta-${metadataIdRef.current++}`, key: "", value: "" }]);
  };

  const updateMetadataEntry = (id: string, field: "key" | "value", value: string) => {
    setMetadataEntries((current) => current.map((entry) => (entry.id === id ? { ...entry, [field]: value } : entry)));
  };

  const removeMetadataEntry = (id: string) => {
    setMetadataEntries((current) => (current.length === 1 ? current : current.filter((entry) => entry.id !== id)));
  };

  const clearSelectedFile = () => {
    setSelectedFile(null);
    setUploadProgress(null);
    setUploadPhase("selecting");
    setMetadataEntries((current) => {
      const next = current.filter((entry) => !FILE_METADATA_KEYS.includes(entry.key));
      return next.length === 0 ? [{ id: `meta-${metadataIdRef.current++}`, key: "", value: "" }] : next;
    });
    setFormValues((prev) => ({ ...prev, filename: "", sizeBytes: "" }));
  };

  const resetUploadState = useCallback(() => {
    uploadAbortControllerRef.current = null;
    setSelectedFile(null);
    setUploadProgress(null);
    setUploadPhase("selecting");
    setFormError(undefined);
    setLastSubmission(null);
  }, []);

  const handleCancelUpload = () => {
    uploadAbortControllerRef.current?.abort();
    resetUploadState();
  };

  const submitUpload = useCallback(
    async (snapshot?: PendingUploadSnapshot) => {
      if (!canManage) return;
      const metadata = snapshot
        ? snapshot.payload.metadata
        : metadataEntries.reduce<Record<string, string>>((acc, entry) => {
            const key = entry.key.trim();
            if (!key) return acc;
            acc[key] = entry.value.trim();
            return acc;
          }, {});
      const parsedSize = snapshot ? snapshot.payload.sizeBytes : Number.parseInt(formValues.sizeBytes || "0", 10);
      const payload = snapshot
        ? snapshot.payload
        : {
            channelId,
            title: formValues.title,
            filename: formValues.filename,
            playbackUrl: formValues.playbackUrl,
            sizeBytes: Number.isNaN(parsedSize) ? undefined : parsedSize,
            metadata: metadata && Object.keys(metadata).length > 0 ? metadata : undefined,
          };
      const file = snapshot ? snapshot.file : selectedFile;
      if (!file && !payload.playbackUrl) {
        setFormError("Select a media file or provide a playback URL");
        setUploadPhase("failed");
        return;
      }

      const abortController = file ? new AbortController() : null;
      uploadAbortControllerRef.current = abortController;

      try {
        setFormError(undefined);
        setSubmitting(true);
        setUploadPhase(file ? "uploading" : "processing");
        setUploadProgress(file ? { percent: 0, loadedBytes: 0, totalBytes: file.size } : null);
        setLastSubmission({ payload, file });
        await createUpload(
          payload,
          file ? { file, onProgress: setUploadProgress, signal: abortController?.signal } : undefined,
        );
        setUploadPhase("ready");
        setFormValues({ title: "", filename: "", playbackUrl: "", sizeBytes: "" });
        setMetadataEntries([{ id: "meta-0", key: "source", value: "upload" }]);
        setSelectedFile(null);
        setUploadProgress(null);
        setLastSubmission(null);
        await load(true);
      } catch (err) {
        if (err instanceof DOMException && err.name === "AbortError") {
          resetUploadState();
          return;
        }
        setFormError(mapUploadError(err));
        setUploadPhase("failed");
      } finally {
        uploadAbortControllerRef.current = null;
        setSubmitting(false);
        setUploadProgress(null);
      }
    },
    [canManage, channelId, formValues.filename, formValues.playbackUrl, formValues.sizeBytes, formValues.title, load, metadataEntries, resetUploadState, selectedFile],
  );

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await submitUpload();
  };

  const handleRetry = async () => {
    if (!lastSubmission || !isRetryableUploadError(formError)) return;
    await submitUpload(lastSubmission);
  };

  const handleDelete = async (id: string) => {
    if (!canManage) return;
    try {
      await deleteUpload(id);
      await load(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to delete upload");
    }
  };

  if (authLoading) return null;
  if (!user) {
    return (
      <Card>
        <InlineAlert>Sign in to manage uploads</InlineAlert>
      </Card>
    );
  }
  if (!canManage) return null;

  return (
    <Card className="workspace-card">
      <CardHeader className="workspace-card__header">
        <h3>Upload manager</h3>
        <p className="muted">Register new VODs, monitor active processing, and keep ready playback visible.</p>
        <div className="workspace-summary-grid">
          <article className="summary-card">
            <span className="summary-card__label">Total</span>
            <strong className="summary-card__value">{items.length}</strong>
            <p className="muted">Uploads on this channel</p>
          </article>
          <article className="summary-card">
            <span className="summary-card__label">Active</span>
            <strong className="summary-card__value">{activeItemCount}</strong>
            <p className="muted">Uploading or processing right now</p>
          </article>
          <article className="summary-card">
            <span className="summary-card__label">Ready</span>
            <strong className="summary-card__value">{readyItemCount}</strong>
            <p className="muted">Playback-ready recordings</p>
          </article>
          <article className="summary-card">
            <span className="summary-card__label">Needs attention</span>
            <strong className="summary-card__value">{failedItemCount}</strong>
            <p className="muted">Failed items to revisit</p>
          </article>
        </div>
      </CardHeader>

      <form className="upload-manager__form" onSubmit={handleSubmit}>
        <div className="upload-manager__form-grid">
          <section className="upload-manager__section">
            <span>Media</span>
            <div
              className={`upload-dropzone${isDragActive ? " upload-dropzone--active" : ""}${selectedFile ? " upload-dropzone--has-file" : ""}`}
              onDragOver={handleDragOver}
              onDragLeave={handleDragLeave}
              onDrop={handleDrop}
              onClick={() => fileInputRef.current?.click()}
              role="button"
              tabIndex={0}
              onKeyDown={(event) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault();
                  fileInputRef.current?.click();
                }
              }}
              aria-label="Upload media file"
            >
              <input ref={fileInputRef} type="file" hidden onChange={handleFileInputChange} />
              {selectedFile ? (
                <div className="upload-dropzone__file">
                  <div>
                    <strong>{selectedFile.name}</strong>
                    <p className="muted">{formatFileSize(selectedFile.size)}</p>
                  </div>
                  <Button
                    variant="secondary"
                    onClick={(event) => {
                      event.stopPropagation();
                      clearSelectedFile();
                    }}
                  >
                    Clear
                  </Button>
                </div>
              ) : (
                <div className="upload-dropzone__hint">
                  <p>Drag and drop a file, or click to browse</p>
                  <p className="muted">MP4, MOV, WEBM</p>
                </div>
              )}
            </div>
            <div className="upload-status">
              <Badge tone={phasePresentation.tone}>{phasePresentation.label}</Badge>
              <p className={`upload-state upload-state--${uploadPhase}`}>{phasePresentation.summary}</p>
            </div>
            {uploadProgress !== null ? (
              <div className="upload-progress">
                <div className="upload-progress__track">
                  <div className="upload-progress__bar" style={{ width: `${uploadProgress.percent}%` }} />
                </div>
                <span className="upload-progress__value">
                  {formatFileSize(uploadProgress.loadedBytes)} / {formatFileSize(uploadProgress.totalBytes)} - {uploadProgress.percent}%
                </span>
              </div>
            ) : null}
          </section>

          <section className="upload-manager__section">
            <label className="stack">
              <span>Title</span>
              <input name="title" type="text" placeholder="Community recap" value={formValues.title} onChange={handleInputChange} />
            </label>
            <label className="stack">
              <span>Filename</span>
              <input name="filename" type="text" placeholder="recap.mp4" value={formValues.filename} onChange={handleInputChange} />
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
              <input name="sizeBytes" type="number" min="0" step="1" placeholder="0" value={formValues.sizeBytes} onChange={handleInputChange} />
            </label>
          </section>

          <section className="upload-manager__section workspace-grid__full">
            <span>Metadata</span>
            <div className="metadata-grid">
              {metadataEntries.map((entry) => (
                <div key={entry.id} className="metadata-row">
                  <input type="text" placeholder="Key" value={entry.key} onChange={(event) => updateMetadataEntry(entry.id, "key", event.target.value)} />
                  <input type="text" placeholder="Value" value={entry.value} onChange={(event) => updateMetadataEntry(entry.id, "value", event.target.value)} />
                  <Button variant="secondary" className="metadata-row__remove" onClick={() => removeMetadataEntry(entry.id)} disabled={metadataEntries.length === 1}>
                    Remove
                  </Button>
                </div>
              ))}
            </div>
            <div className="workspace-card__actions">
              <Button variant="secondary" onClick={addMetadataEntry}>
                Add metadata
              </Button>
            </div>
          </section>
        </div>

        {formError ? <InlineAlert>{formError}</InlineAlert> : null}
        <div className="upload-submit-row">
          <Button type="submit" disabled={submitting || !hasUploadSource}>
            {submitting ? "Submitting..." : "Register upload"}
          </Button>
          {uploadPhase === "uploading" ? <Button variant="secondary" onClick={handleCancelUpload}>Cancel upload</Button> : null}
          {uploadPhase === "failed" ? <Button variant="secondary" onClick={resetUploadState}>Reset</Button> : null}
          {uploadPhase === "failed" && isRetryableUploadError(formError) ? (
            <Button variant="secondary" onClick={handleRetry} disabled={submitting || !lastSubmission}>
              Retry
            </Button>
          ) : null}
        </div>
      </form>

      <CardBody>
        <div className="upload-manager__toolbar">
          <Button variant="secondary" onClick={() => load()} disabled={loading}>
            {loading ? "Refreshing..." : "Refresh"}
          </Button>
          {error ? <InlineAlert>{error}</InlineAlert> : null}
        </div>
        <div className="upload-manager__toolbar">
          <div className="upload-filter-row" role="group" aria-label="Upload filter">
            <button type="button" className={`chip${listFilter === "all" ? " chip--active" : ""}`} onClick={() => setListFilter("all")}>All</button>
            <button type="button" className={`chip${listFilter === "active" ? " chip--active" : ""}`} onClick={() => setListFilter("active")}>Active</button>
            <button type="button" className={`chip${listFilter === "ready" ? " chip--active" : ""}`} onClick={() => setListFilter("ready")}>Ready</button>
            <button type="button" className={`chip${listFilter === "failed" ? " chip--active" : ""}`} onClick={() => setListFilter("failed")}>Failed</button>
          </div>
          <label className="upload-manager__search">
            Search uploads
            <input type="search" value={searchTerm} onChange={(event) => setSearchTerm(event.target.value)} placeholder="Search title or filename" />
          </label>
        </div>
        {loading ? <p className="muted">Loading uploads...</p> : null}
        {!loading && items.length === 0 && !error ? (
          <EmptyState className="upload-empty-state">
            <p className="muted">No uploads yet. Select media and register your first upload.</p>
          </EmptyState>
        ) : null}
        {!loading && items.length > 0 && visibleItems.length === 0 ? (
          <EmptyState className="upload-empty-state">
            <p className="muted">No uploads match the selected filters.</p>
          </EmptyState>
        ) : null}
        {visibleItems.length > 0 ? (
          <ul className="upload-list">
            {visibleItems.map((item) => {
              const statusPresentation = getUploadItemStatusPresentation(item);
              const hasPlaybackUrl = Boolean(item.playbackUrl?.trim());
              const isReadyForPlayback = hasPlaybackUrl || isUploadItemReady(item.status);
              const hasRecordingWithoutPlayback = Boolean(item.recordingId && !hasPlaybackUrl);
              const recordingDetailHref = hasRecordingWithoutPlayback ? getRecordingDetailHref(item.recordingId) : null;
              return (
                <li key={item.id} className="upload-card">
                  <div className="upload-status upload-status--card">
                    <Badge tone={statusPresentation.tone}>{statusPresentation.label}</Badge>
                    <p className="muted">{statusPresentation.summary}</p>
                  </div>
                  {item.error ? <InlineAlert>{mapUploadItemError(item.error)}</InlineAlert> : null}
                  <div className="upload-card__header">
                    <strong>{item.title || item.filename}</strong>
                    <span className="muted">{new Date(item.createdAt).toLocaleString()}</span>
                  </div>
                  {item.status.toLowerCase().trim() === "processing" ? <p className="muted">Last updated: {new Date(item.updatedAt).toLocaleString()}</p> : null}
                  <p className="muted">{formatFileSize(item.sizeBytes)}</p>
                  <div className="upload-card__actions">
                    {isReadyForPlayback && hasPlaybackUrl ? (
                      <a className={buttonClassName("primary")} href={item.playbackUrl} target="_blank" rel="noreferrer">
                        Watch
                      </a>
                    ) : null}
                    {hasRecordingWithoutPlayback && recordingDetailHref ? (
                      <Link className={buttonClassName("secondary")} href={recordingDetailHref}>
                        View recording
                      </Link>
                    ) : null}
                    {hasRecordingWithoutPlayback && !recordingDetailHref ? <span className="muted">Playback pending. Check back soon to watch this recording.</span> : null}
                    <Button variant="secondary" onClick={() => handleDelete(item.id)}>Delete</Button>
                  </div>
                </li>
              );
            })}
          </ul>
        ) : null}
      </CardBody>
    </Card>
  );
}

function normalizeUploadStatus(status: string) {
  return status.toLowerCase().trim();
}

function isUploadInActiveState(status: string) {
  const normalized = normalizeUploadStatus(status);
  return normalized === "uploading" || normalized === "processing" || normalized === "failed" || normalized === "error";
}

function isUploadItemFailed(status: string) {
  const normalized = normalizeUploadStatus(status);
  return normalized === "failed" || normalized === "error";
}

function compareUploadsForMonitoring(a: UploadItem, b: UploadItem) {
  const aActiveRank = isUploadInActiveState(a.status) ? 0 : 1;
  const bActiveRank = isUploadInActiveState(b.status) ? 0 : 1;
  if (aActiveRank !== bActiveRank) return aActiveRank - bActiveRank;
  const aUpdated = Date.parse(a.updatedAt);
  const bUpdated = Date.parse(b.updatedAt);
  return (Number.isFinite(bUpdated) ? bUpdated : 0) - (Number.isFinite(aUpdated) ? aUpdated : 0);
}

function getUploadPhasePresentation(phase: UploadPhase, hasSource: boolean, progressPercent?: number): UploadStatusPresentation {
  switch (phase) {
    case "uploading":
      return { label: "Uploading", summary: `Uploading... ${progressPercent ?? 0}% complete.`, tone: "info" };
    case "processing":
      return { label: "Processing", summary: "Processing... This may take a few minutes before playback is ready.", tone: "info" };
    case "ready":
      return { label: "Ready", summary: "Ready. Review it below and publish or share when you are set.", tone: "success" };
    case "failed":
      return { label: "Failed", summary: "Failed. Check the error details, then update fields and retry.", tone: "danger" };
    default:
      return {
        label: "Select media",
        summary: hasSource ? "Media selected. Confirm details, then register upload." : "Select media to start a new upload.",
        tone: "neutral",
      };
  }
}

function formatUploadStatus(status: string) {
  const normalized = normalizeUploadStatus(status);
  if (normalized === "pending" || normalized === "queued") return "Processing";
  if (normalized === "completed") return "Ready";
  return normalized.replace(/_/g, " ").replace(/\b\w/g, (char) => char.toUpperCase());
}

function isUploadItemReady(status: string) {
  const normalized = normalizeUploadStatus(status);
  return normalized === "completed" || normalized === "ready";
}

function getUploadItemStatusPresentation(item: UploadItem): UploadStatusPresentation {
  const label = formatUploadStatus(item.status);
  const status = label.toLowerCase();
  const percent = Math.round(item.progress);
  if (status === "processing") {
    return { label, summary: `Processing... ${percent}% complete. We are transcoding and packaging this recording for playback.`, tone: "info" };
  }
  if (status === "ready") {
    return { label, summary: "Ready. Open playback and confirm quality.", tone: "success" };
  }
  if (status === "failed" || status === "error") {
    return { label: "Failed", summary: "Failed. Fix the issue and retry this upload.", tone: "danger" };
  }
  return { label, summary: `${label} - ${percent}%`, tone: "neutral" };
}

function getRecordingDetailHref(recordingId?: string) {
  const normalizedId = recordingId?.trim();
  if (!normalizedId) return null;
  const routeTemplate = process.env.NEXT_PUBLIC_RECORDING_DETAIL_ROUTE_TEMPLATE?.trim();
  if (!routeTemplate || !routeTemplate.includes("[id]")) return null;
  return routeTemplate.replace("[id]", encodeURIComponent(normalizedId));
}

function mapUploadError(err: unknown) {
  if (err instanceof ViewerApiError) {
    const bodyMessage =
      typeof err.body === "object" && err.body !== null && "message" in err.body && typeof err.body.message === "string"
        ? err.body.message
        : err.message;
    const msg = bodyMessage.toLowerCase();
    if (err.status === 413 || msg.includes("exceeds limit") || msg.includes("too large")) return "Upload failed: file size exceeds the allowed limit.";
    if (msg.includes("unsupported media") || msg.includes("unsupported") || msg.includes("invalid type")) return "Upload failed: invalid file type. Supported types are MP4, MOV, and WEBM.";
    if (err.status === 429 || msg.includes("quota")) return "Upload failed: quota exceeded. Free up storage or try again later.";
    return bodyMessage || "Unable to create upload";
  }
  return err instanceof Error ? err.message : "Unable to create upload";
}

function mapUploadItemError(message: string) {
  const normalized = message.toLowerCase();
  if (normalized.includes("quota")) return "Quota exceeded while processing this upload.";
  if (normalized.includes("unsupported media") || normalized.includes("invalid type")) return "Processing failed due to an invalid file type.";
  return message;
}

function isRetryableUploadError(message?: string) {
  if (!message) return false;
  const normalized = message.toLowerCase();
  return normalized.includes("timeout") || normalized.includes("tempor") || normalized.includes("network") || normalized.includes("unavailable") || normalized.includes("upload failed");
}

function formatFileSize(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  return `${value.toFixed(unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}

function deriveTitleFromFilename(name: string) {
  if (!name) return "";
  const withoutExt = name.replace(/\.[^.]+$/, "");
  const cleaned = withoutExt.replace(/[-_]+/g, " ").trim();
  return cleaned || withoutExt || name;
}
