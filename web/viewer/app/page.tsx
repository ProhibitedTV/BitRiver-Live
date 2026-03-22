import { DirectoryDataBoundary, DirectoryPageShell } from "./directory-page-shell";
import { normalizeDirectoryQuery } from "../lib/directory-state";

type PageProps = {
  searchParams?: {
    q?: string;
  };
};

export default function DirectoryPage({ searchParams }: PageProps) {
  const query = normalizeDirectoryQuery(typeof searchParams?.q === "string" ? searchParams.q : "");

  return (
    <DirectoryPageShell query={query}>
      <DirectoryDataBoundary query={query} />
    </DirectoryPageShell>
  );
}
