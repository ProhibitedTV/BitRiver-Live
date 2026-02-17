import Image from "next/image";
import { getChannelAvatarFallback } from "../../lib/channel-presenters";

interface ChannelAvatarProps {
  displayName: string;
  avatarUrl?: string;
  size?: number;
  className?: string;
  imageClassName?: string;
}

export function ChannelAvatar({
  displayName,
  avatarUrl,
  size = 44,
  className = "overlay__avatar",
  imageClassName = "overlay__avatar-image",
}: ChannelAvatarProps) {
  return (
    <div className={className} aria-hidden="true">
      {avatarUrl ? (
        <Image
          src={avatarUrl}
          alt=""
          width={size}
          height={size}
          sizes={`${size}px`}
          className={imageClassName}
        />
      ) : (
        <span>{getChannelAvatarFallback(displayName)}</span>
      )}
    </div>
  );
}
