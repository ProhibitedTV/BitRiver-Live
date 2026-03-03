"use client";

import { SearchBar } from "./SearchBar";
import { useDirectorySearch } from "../hooks/useDirectorySearch";

export function DirectorySearchBar({ defaultValue }: { defaultValue?: string }) {
  const { navigateWithQuery } = useDirectorySearch({ fallbackPathname: "/" });

  const handleSearch = (value: string) => {
    navigateWithQuery(value);
  };

  const handleClear = () => {
    navigateWithQuery("");
  };

  return <SearchBar onSearch={handleSearch} defaultValue={defaultValue} onClear={handleClear} />;
}
