import { ReactNode } from "react";
import type { DirectoryChannel } from "../../lib/viewer-api";

interface FollowingListProps {
  channels: DirectoryChannel[];
  as?: "div" | "ul";
  className?: string;
  itemClassName?: string;
  role?: string;
  itemRole?: string;
  renderItem: (entry: DirectoryChannel) => ReactNode;
}

export function FollowingList({
  channels,
  as = "div",
  className,
  itemClassName,
  role,
  itemRole,
  renderItem,
}: FollowingListProps) {
  if (as === "ul") {
    return (
      <ul className={className} role={role}>
        {channels.map((entry) => (
          <li key={entry.channel.id} className={itemClassName} role={itemRole}>
            {renderItem(entry)}
          </li>
        ))}
      </ul>
    );
  }

  return (
    <div className={className} role={role}>
      {channels.map((entry) => (
        <div key={entry.channel.id} className={itemClassName} role={itemRole}>
          {renderItem(entry)}
        </div>
      ))}
    </div>
  );
}
