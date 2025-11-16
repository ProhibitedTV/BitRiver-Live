import Link from "next/link";
import type { DirectoryChannel } from "../lib/viewer-api";

interface ChannelRailProps {
  title: string;
  subtitle?: string;
  channels: DirectoryChannel[];
  loading?: boolean;
  density?: "default" | "compact";
  eyebrow?: string;
}

function RailCard({ entry, density = "default" }: { entry: DirectoryChannel; density?: "default" | "compact" }) {
  const previewImage = entry.profile.bannerUrl ?? entry.profile.avatarUrl;
  const category = entry.channel.category ?? "Streaming";

  return (
    <Link href={`/channels/${entry.channel.id}`} className={`rail-card rail-card--${density}`}>
      <div className="rail-card__media">
        {previewImage ? (
          <img src={previewImage} alt={`${entry.owner.displayName} channel artwork`} />
        ) : (
          <div className="rail-card__media-fallback" aria-hidden="true" />
        )}
        <div className="overlay overlay--top overlay--scrim">
          {entry.live && <span className="badge badge--live">Live</span>}
          <span className="overlay__meta">{category}</span>
        </div>
      </div>
      <div className="rail-card__body">
        <p className="rail-card__meta muted">{entry.owner.displayName}</p>
        <h3 className="rail-card__title">{entry.channel.title}</h3>
        {entry.profile.bio && density === "default" && <p className="rail-card__description muted">{entry.profile.bio}</p>}
        <div className="tag-list">
          {entry.channel.category && <span className="tag">{entry.channel.category}</span>}
          {entry.channel.tags.slice(0, density === "compact" ? 1 : 2).map((tag) => (
            <span key={tag} className="tag">
              #{tag}
            </span>
          ))}
        </div>
      </div>
    </Link>
  );
}

export function ChannelRail({
  title,
  subtitle,
  channels,
  loading = false,
  density = "default",
  eyebrow,
}: ChannelRailProps) {
  return (
    <section className="content-rail stack">
      <header className="content-rail__header">
        <div className="stack">
          {eyebrow && <span className="muted content-rail__eyebrow">{eyebrow}</span>}
          <h2>{title}</h2>
          {subtitle && <p className="muted">{subtitle}</p>}
        </div>
        {!loading && channels.length > 0 && <span className="muted">{channels.length} channels</span>}
      </header>

      {loading ? (
        <div className="surface">Loading channels…</div>
      ) : channels.length === 0 ? (
        <div className="surface">
          <p className="muted">No channels to show right now.</p>
        </div>
      ) : (
        <div className="content-rail__scroller" role="list">
          {channels.map((entry) => (
            <RailCard key={entry.channel.id} entry={entry} density={density} />
          ))}
        </div>
      )}
    </section>
  );
}
