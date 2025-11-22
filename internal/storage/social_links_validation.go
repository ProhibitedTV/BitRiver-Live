package storage

import (
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"bitriver-live/internal/models"
)

const (
	maxSocialLinks          = 8
	maxSocialPlatformLength = 64
	maxSocialLinkLength     = 2048
)

// NormalizeSocialLinks trims and validates a slice of social links, ensuring each
// entry contains a platform label and a valid HTTP(S) URL.
func NormalizeSocialLinks(links []models.SocialLink) ([]models.SocialLink, error) {
	if len(links) > maxSocialLinks {
		return nil, fmt.Errorf("social links cannot exceed %d entries", maxSocialLinks)
	}

	normalized := make([]models.SocialLink, 0, len(links))
	seen := make(map[string]struct{}, len(links))

	for _, link := range links {
		platform := strings.TrimSpace(link.Platform)
		if platform == "" {
			return nil, fmt.Errorf("social link platform is required")
		}
		if utf8.RuneCountInString(platform) > maxSocialPlatformLength {
			return nil, fmt.Errorf("social link platform cannot exceed %d characters", maxSocialPlatformLength)
		}

		linkURL := strings.TrimSpace(link.URL)
		if linkURL == "" {
			return nil, fmt.Errorf("social link URL is required")
		}
		if utf8.RuneCountInString(linkURL) > maxSocialLinkLength {
			return nil, fmt.Errorf("social link URL cannot exceed %d characters", maxSocialLinkLength)
		}

		parsed, err := url.Parse(linkURL)
		if err != nil {
			return nil, fmt.Errorf("invalid social link URL")
		}
		if !parsed.IsAbs() || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return nil, fmt.Errorf("social link URL must be absolute and use http or https")
		}

		normalizedURL := parsed.String()
		key := strings.ToLower(platform) + "|" + strings.ToLower(normalizedURL)
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("duplicate social link detected")
		}
		seen[key] = struct{}{}

		normalized = append(normalized, models.SocialLink{Platform: platform, URL: normalizedURL})
	}

	return normalized, nil
}
