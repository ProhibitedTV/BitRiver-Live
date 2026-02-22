package storage

import "strings"

// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func buildObjectKey(parts ...string) string {
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.Trim(part, "/")
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return strings.Join(normalized, "/")
}

// normalizeObjectComponent executes normalizeObjectComponent.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: no transaction/connection contract applies for this pure helper.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func normalizeObjectComponent(input string) string {
	lowered := strings.ToLower(strings.TrimSpace(input))
	if lowered == "" {
		return "item"
	}
	var builder strings.Builder
	lastDash := false
	for _, r := range lowered {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	normalized := strings.Trim(builder.String(), "-")
	if normalized == "" {
		return "item"
	}
	return normalized
}

// manifestMetadataKey executes manifestMetadataKey.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: no transaction/connection contract applies for this pure helper.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func manifestMetadataKey(name string) string {
	return metadataManifestPrefix + normalizeObjectComponent(name)
}

// thumbnailMetadataKey executes thumbnailMetadataKey.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: no transaction/connection contract applies for this pure helper.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func thumbnailMetadataKey(id string) string {
	return metadataThumbnailPrefix + id
}

// normalizeRoles executes normalizeRoles.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: no transaction/connection contract applies for this pure helper.
