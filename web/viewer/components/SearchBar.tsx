"use client";

import { FormEvent, useState } from "react";

export function SearchBar({
  onSearch,
  defaultValue
}: {
  onSearch: (query: string) => void;
  defaultValue?: string;
}) {
  const [value, setValue] = useState<string>(defaultValue ?? "");

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onSearch(value);
  };

  return (
    <form className="search-bar" onSubmit={handleSubmit} role="search">
      <label htmlFor="directory-search" className="sr-only">
        Search channels
      </label>
      <input
        id="directory-search"
        type="search"
        placeholder="Search by channel, creator, or tag"
        value={value}
        onChange={(event) => setValue(event.target.value)}
        aria-label="Search channels"
      />
      <button type="submit" className="secondary-button">
        Search
      </button>
    </form>
  );
}
