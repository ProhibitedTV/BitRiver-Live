export const directoryInputMatrix = [
  { label: "empty query", query: "", normalized: "", mode: "directory" as const },
  { label: "whitespace query", query: "   ", normalized: "", mode: "directory" as const },
  { label: "valid search query", query: "retro", normalized: "retro", mode: "search" as const },
  { label: "api error", query: "retro", normalized: "retro", mode: "error" as const, errorMessage: "Gateway timeout" },
];
