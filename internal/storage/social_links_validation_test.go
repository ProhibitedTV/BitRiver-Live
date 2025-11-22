package storage

import (
	"testing"

	"bitriver-live/internal/models"
)

func TestNormalizeSocialLinks(t *testing.T) {
	links := []models.SocialLink{
		{Platform: " Twitch ", URL: "https://twitch.example.com/streamer "},
		{Platform: "Twitter", URL: "https://twitter.com/streamer"},
	}

	normalized, err := NormalizeSocialLinks(links)
	if err != nil {
		t.Fatalf("NormalizeSocialLinks returned error: %v", err)
	}
	if len(normalized) != len(links) {
		t.Fatalf("expected %d links, got %d", len(links), len(normalized))
	}
	if normalized[0].Platform != "Twitch" {
		t.Fatalf("expected platform to be trimmed, got %q", normalized[0].Platform)
	}
	if normalized[0].URL != "https://twitch.example.com/streamer" {
		t.Fatalf("expected URL to be normalized, got %q", normalized[0].URL)
	}
}

func TestNormalizeSocialLinksValidation(t *testing.T) {
	cases := map[string][]models.SocialLink{
		"empty platform": {{Platform: "", URL: "https://example.com"}},
		"empty url":      {{Platform: "YouTube", URL: "  "}},
		"invalid url":    {{Platform: "YouTube", URL: "ftp://example.com"}},
		"duplicate": {
			{Platform: "YouTube", URL: "https://example.com"},
			{Platform: "YouTube", URL: "https://example.com"},
		},
	}

	for name, links := range cases {
		_, err := NormalizeSocialLinks(links)
		if err == nil {
			t.Fatalf("expected error for case %q", name)
		}
	}

	tooMany := make([]models.SocialLink, maxSocialLinks+1)
	for i := range tooMany {
		tooMany[i] = models.SocialLink{Platform: "Link", URL: "https://example.com"}
	}
	if _, err := NormalizeSocialLinks(tooMany); err == nil {
		t.Fatalf("expected error when exceeding max social links")
	}
}
