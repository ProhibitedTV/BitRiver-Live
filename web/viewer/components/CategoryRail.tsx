import type { CategorySummary } from "../lib/viewer-api";

interface CategoryRailProps {
  categories: CategorySummary[];
  loading?: boolean;
}

export function CategoryRail({ categories, loading = false }: CategoryRailProps) {
  return (
    <section className="content-rail stack">
      <header className="content-rail__header">
        <div className="stack">
          <span className="muted content-rail__eyebrow">Top Categories</span>
          <h2>Browse by category</h2>
          <p className="muted">Jump into genres that fit your mood or discover something new.</p>
        </div>
        {!loading && categories.length > 0 && <span className="muted">{categories.length} to explore</span>}
      </header>

      {loading ? (
        <div className="surface">Loading categories…</div>
      ) : categories.length === 0 ? (
        <div className="surface">
          <p className="muted">No categories available.</p>
        </div>
      ) : (
        <div className="chip-rail" role="list">
          {categories.map((category) => (
            <button key={category.name} className="filter-chip" type="button" role="listitem">
              <div className="filter-chip__label">{category.name}</div>
              <div className="filter-chip__meta muted">{category.channelCount} live</div>
            </button>
          ))}
        </div>
      )}
    </section>
  );
}
