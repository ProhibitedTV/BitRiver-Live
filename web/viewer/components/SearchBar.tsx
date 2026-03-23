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
      <div className="search-bar__field">
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
      </div>
      <div className="search-bar__actions">
        {value.length > 0 && (
          <button type="button" className="secondary-button search-bar__clear" onClick={handleClear}>
            Clear
          </button>
        )}
        <button type="submit" className="primary-button search-bar__submit">
          {submitLabel}
        </button>
      </div>
    </form>
  );
}
