"use client";

import { useRouter } from "next/navigation";
import { SearchBar } from "./SearchBar";

export function DirectorySearchBar({ defaultValue }: { defaultValue?: string }) {
  const router = useRouter();

  const handleSearch = (value: string) => {
    const params = new URLSearchParams();
    if (value.trim().length > 0) {
      params.set("q", value.trim());
    }
    const queryString = params.toString();
    router.push(queryString ? `/?${queryString}` : "/");
  };

  return <SearchBar onSearch={handleSearch} defaultValue={defaultValue} />;
}
