import { formatFollowerLabel, formatViewerLabel } from "../../lib/channel-presenters";

interface ChannelMetaChipsProps {
  live: boolean;
  followerCount: number;
  viewerCount?: number;
  category?: string;
  showFollowerSummary?: boolean;
  followerSummaryPrefix?: string;
}

export function ChannelMetaChips({
  live,
  followerCount,
  viewerCount,
  category,
  showFollowerSummary = false,
  followerSummaryPrefix,
}: ChannelMetaChipsProps) {
  const primaryMeta = live ? formatViewerLabel(viewerCount ?? 0) : formatFollowerLabel(followerCount);

  return (
    <>
      <span className="meta-chip">{primaryMeta}</span>
      {showFollowerSummary && (
        <span className="meta-chip meta-chip--muted">
          {followerSummaryPrefix ? `${followerSummaryPrefix}${formatFollowerLabel(followerCount)}` : formatFollowerLabel(followerCount)}
        </span>
      )}
      {category && <span className="meta-chip meta-chip--pill">{category}</span>}
    </>
  );
}
