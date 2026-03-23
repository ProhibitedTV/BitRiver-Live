import Link from "next/link";
import { buttonClassName } from "../../components/ui/Button";

export default function CreatorIndexPage() {
  return (
    <section className="creator-overview-grid" aria-label="Creator overview">
      <article className="creator-overview-card">
        <span className="page-eyebrow">Onboarding</span>
        <h2>Start with the guided setup</h2>
        <p className="muted">
          Pick a channel, copy your OBS settings, test the stream, and confirm the public viewer link before you go live.
        </p>
        <div className="creator-overview-card__actions">
          <Link href="/creator/getting-started" className={buttonClassName()}>
            Open getting started
          </Link>
        </div>
      </article>
      <article className="creator-overview-card">
        <span className="page-eyebrow">Go live</span>
        <h2>Use channel-specific dashboards</h2>
        <p className="muted">
          When you open a channel dashboard, this studio adds direct links for both live control and uploads so you can move between them quickly.
        </p>
      </article>
      <article className="creator-overview-card">
        <span className="page-eyebrow">Uploads</span>
        <h2>Keep playback follow-up visible</h2>
        <p className="muted">
          Register VODs, watch processing states, and jump back to the public channel when recordings are ready.
        </p>
      </article>
      <article className="creator-overview-card">
        <span className="page-eyebrow">Tip</span>
        <h2>Return here anytime</h2>
        <p className="muted">
          The studio is now structured as a persistent workspace, so onboarding and daily operations live in the same place.
        </p>
      </article>
    </section>
  );
}
