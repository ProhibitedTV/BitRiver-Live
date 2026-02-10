"use client";

import { FormEvent, useEffect, useState } from "react";

export function SearchBar({
  onSearch,
  defaultValue,
  submitLabel = "Search",
  onClear,
}: {
  onSearch: (query: string) => void;
  defaultValue?: string;
  submitLabel?: string;
  onClear?: () => void;
}) {
  const [value, setValue] = useState<string>(defaultValue ?? "");

  useEffect(() => {
    setValue(defaultValue ?? "");
  }, [defaultValue]);

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onSearch(value);
  };

  const handleClear = () => {
    setValue("");
    onClear?.();
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
        onKeyDown={(event) => {
          if (event.key === "Escape" && value.length > 0) {
            event.preventDefault();
            handleClear();
          }
        }}
        aria-label="Search channels"
      />
      {value.length > 0 && (
        <button type="button" className="secondary-button" onClick={handleClear}>
          Clear
        </button>
      )}
      <button type="submit" className="secondary-button">
        {submitLabel}
      </button>
    </form>
  );
}
