import Link from "next/link";
import type { CategorySummary } from "../lib/viewer-api";

interface CategoryRailProps {
  id?: string;
  categories: CategorySummary[];
  loading?: boolean;
}

export function CategoryRail({ id, categories, loading = false }: CategoryRailProps) {
  return (
    <section className="content-rail content-rail--panel surface" id={id}>
      <header className="content-rail__header">
        <div className="stack">
          <span className="muted content-rail__eyebrow">Top Categories</span>
          <h2>Browse categories</h2>
          <p className="muted">The busiest categories on this install.</p>
        </div>
        {!loading && categories.length > 0 && <span className="muted">{categories.length} to explore</span>}
      </header>

      {loading ? (
        <div className="state-panel state-panel--loading" aria-busy="true">
          <strong>Loading categories</strong>
          <p className="muted">Loading the latest categories now.</p>
        </div>
      ) : categories.length === 0 ? (
        <div className="state-panel">
          <strong>No categories available yet</strong>
          <p className="muted">Categories will appear here once channels start using them.</p>
        </div>
      ) : (
        <div className="chip-rail" role="list">
          {categories.map((category) => (
            <div key={category.name} role="listitem">
              <Link className="filter-chip" href={`/browse?category=${encodeURIComponent(category.name)}`}>
                <div className="filter-chip__label">{category.name}</div>
                <div className="filter-chip__meta muted">{category.channelCount} live</div>
              </Link>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
