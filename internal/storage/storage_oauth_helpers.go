package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func normalizeRoles(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	roles := make([]string, 0, len(input))
	seen := make(map[string]struct{})
	for _, role := range input {
		trimmed := strings.TrimSpace(role)
		if trimmed == "" {
			continue
		}
		normalized := strings.ToLower(trimmed)
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		roles = append(roles, normalized)
	}
	if len(roles) == 0 {
		return nil
	}
	sort.Strings(roles)
	return roles
}

// oauthAccountKey executes oauthAccountKey.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: no transaction/connection contract applies for this pure helper.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func oauthAccountKey(provider, subject string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	subject = strings.TrimSpace(subject)
	return provider + "|" + subject
}

// fallbackOAuthEmail executes fallbackOAuthEmail.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: no transaction/connection contract applies for this pure helper.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func fallbackOAuthEmail(provider, subject string) string {
	domain := strings.ToLower(strings.TrimSpace(provider))
	if domain == "" {
		domain = "provider"
	}
	domain = sanitizeOAuthComponent(domain)
	hash := sha256.Sum256([]byte(provider + ":" + subject))
	local := hex.EncodeToString(hash[:8])
	return fmt.Sprintf("%s@%s.oauth", local, domain)
}

// defaultOAuthDisplayName executes defaultOAuthDisplayName.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: no transaction/connection contract applies for this pure helper.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func defaultOAuthDisplayName(provider, email, subject string) string {
	trimmedEmail := strings.TrimSpace(email)
	if trimmedEmail != "" {
		local := strings.SplitN(trimmedEmail, "@", 2)[0]
		local = strings.ReplaceAll(local, ".", " ")
		local = strings.TrimSpace(local)
		if local != "" {
			return capitalizeWord(local)
		}
	}
	sanitized := sanitizeOAuthComponent(subject)
	if sanitized != "" {
		return sanitized
	}
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return "Viewer"
	}
	return capitalizeWord(provider) + " user"
}

// sanitizeOAuthComponent executes sanitizeOAuthComponent.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: no transaction/connection contract applies for this pure helper.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func sanitizeOAuthComponent(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	if lower == "" {
		return ""
	}
	var builder strings.Builder
	for _, r := range lower {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == ' ':
			builder.WriteRune(' ')
		default:
			builder.WriteRune('-')
		}
	}
	return strings.TrimSpace(builder.String())
}

// capitalizeWord executes capitalizeWord.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: no transaction/connection contract applies for this pure helper.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func capitalizeWord(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	return strings.ToUpper(lower[:1]) + lower[1:]
}

// NewStorage executes NewStorage.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: no transaction/connection contract applies for this pure helper.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
