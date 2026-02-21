import Link from "next/link";
import { buttonClassName } from "../../components/ui/Button";

export default function CreatorIndexPage() {
  return (
    <section className="surface stack">
      <h2>Select a channel</h2>
      <p className="muted">
        Choose a channel from the navigation to start managing uploads or open a direct dashboard link.
      </p>
      <Link href="/creator/getting-started" className={buttonClassName("secondary")} style={{ alignSelf: "flex-start" }}>
        Open getting started
      </Link>
    </section>
  );
}
