import { DirectoryDataBoundary, DirectoryPageShell } from "./directory-page-shell";
import { normalizeDirectoryQuery } from "../lib/directory-state";

type PageProps = {
  searchParams?: Promise<{ q?: string | string[] }>;
};

export default async function DirectoryPage({ searchParams }: PageProps) {
  const resolvedSearchParams = await searchParams;
  const query = normalizeDirectoryQuery(
    typeof resolvedSearchParams?.q === "string" ? resolvedSearchParams.q : "",
  );

  return (
    <DirectoryPageShell query={query}>
      <DirectoryDataBoundary query={query} />
    </DirectoryPageShell>
  );
}
