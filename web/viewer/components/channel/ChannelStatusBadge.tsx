import { getChannelStatusLabel } from "../../lib/channel-presenters";

interface ChannelStatusBadgeProps {
  live: boolean;
  offlineClassName?: string;
}

export function ChannelStatusBadge({ live, offlineClassName = "badge--muted" }: ChannelStatusBadgeProps) {
  const className = live ? "badge badge--live" : `badge ${offlineClassName}`;
  return <span className={className}>{getChannelStatusLabel(live)}</span>;
}
