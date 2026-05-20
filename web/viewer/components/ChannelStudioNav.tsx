import Link from "next/link";

type ChannelStudioTool = "preview" | "live" | "uploads" | "schedule" | "share";

type ChannelStudioNavProps = {
  channelId: string;
  channelTitle: string;
  liveState?: string;
  activeTool?: ChannelStudioTool;
  eyebrow?: string;
  heading?: string;
  description?: string;
  className?: string;
};

const TOOL_LABELS: Record<ChannelStudioTool, string> = {
  preview: "Public preview",
  live: "Go live",
  uploads: "Uploads",
  schedule: "Schedule",
  share: "Share link",
};

export function ChannelStudioNav({
  channelId,
  channelTitle,
  liveState,
  activeTool,
  eyebrow = "Channel tools",
  heading = `${channelTitle} studio`,
  description = "Manage the same channel your viewers see: preview the public page, prepare a stream, keep uploads current, update schedule, and copy the share link.",
  className,
}: ChannelStudioNavProps) {
  const liveHref = `/creator/live/${channelId}`;
  const headingId = `channel-studio-nav-${channelId.replace(/[^a-zA-Z0-9_-]/g, "-")}-${activeTool ?? "tools"}-title`;
  const tools: Array<{ id: ChannelStudioTool; href: string }> = [
    { id: "preview", href: `/channels/${channelId}` },
    { id: "live", href: liveHref },
    { id: "uploads", href: `/creator/uploads/${channelId}` },
    { id: "schedule", href: `${liveHref}#channel-schedule` },
    { id: "share", href: `${liveHref}#channel-share` },
  ];

  return (
    <section className={`channel-studio-nav${className ? ` ${className}` : ""}`} aria-labelledby={headingId}>
      <div className="channel-studio-nav__copy">
        <div className="eyebrow-row">
          <span className="page-eyebrow">{eyebrow}</span>
          {liveState ? <span className="pill pill--ghost">{liveState}</span> : null}
        </div>
        <h2 id={headingId}>{heading}</h2>
        <p className="muted">{description}</p>
      </div>
      <nav className="channel-studio-nav__links" aria-label={`${channelTitle} channel tools`}>
        {tools.map((tool) => {
          const isActive = activeTool === tool.id;
          return (
            <Link
              key={tool.id}
              href={tool.href}
              className={`channel-studio-nav__link${isActive ? " channel-studio-nav__link--active" : ""}`}
              aria-current={isActive ? "page" : undefined}
            >
              {TOOL_LABELS[tool.id]}
            </Link>
          );
        })}
      </nav>
    </section>
  );
}
