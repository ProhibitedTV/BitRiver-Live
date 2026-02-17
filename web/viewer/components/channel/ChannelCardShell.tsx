import type { PropsWithChildren } from "react";

interface ChannelCardShellProps extends PropsWithChildren {
  className: string;
}

export function ChannelCardShell({ className, children }: ChannelCardShellProps) {
  return <article className={className}>{children}</article>;
}
