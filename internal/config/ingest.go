package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var ErrIngestConfigDisabled = errors.New("ingest disabled: no configuration provided")

// MissingIngestConfigError reports required ingest environment variables.
type MissingIngestConfigError struct {
	Missing []string
}

func (e MissingIngestConfigError) Error() string {
	return fmt.Sprintf("missing ingest configuration: %s", strings.Join(e.Missing, ", "))
}

type Rendition struct {
	Name    string
	Bitrate int
}

// IngestConfig stores parsed ingest integration settings.
type IngestConfig struct {
	SRSBaseURL         string
	SRSToken           string
	OMEBaseURL         string
	OMEPlaybackBaseURL string
	OMEAccessToken     string
	OMEUsername        string
	OMEPassword        string
	JobBaseURL         string
	JobToken           string
	LadderProfiles     []Rendition
	HealthEndpoint     string
	HealthTimeout      time.Duration
	MaxBootAttempts    int
	RetryInterval      time.Duration
	HTTPMaxAttempts    int
	HTTPRetryInterval  time.Duration
}

func LoadIngestFromEnv(env Environment) (IngestConfig, error) {
	cfg := IngestConfig{
		SRSBaseURL:         strings.TrimSpace(env.Get("BITRIVER_SRS_API")),
		SRSToken:           strings.TrimSpace(env.Get("BITRIVER_SRS_TOKEN")),
		OMEBaseURL:         strings.TrimSpace(env.Get("BITRIVER_OME_API")),
		OMEPlaybackBaseURL: strings.TrimRight(strings.TrimSpace(env.Get("BITRIVER_OME_PUBLIC_LLHLS_BASE_URL")), "/"),
		OMEAccessToken:     firstNonEmpty(strings.TrimSpace(env.Get("BITRIVER_OME_HEALTHCHECK_TOKEN")), strings.TrimSpace(env.Get("BITRIVER_OME_API_TOKEN"))),
		OMEUsername:        strings.TrimSpace(env.Get("BITRIVER_OME_USERNAME")),
		OMEPassword:        strings.TrimSpace(env.Get("BITRIVER_OME_PASSWORD")),
		JobBaseURL:         strings.TrimSpace(env.Get("BITRIVER_TRANSCODER_API")),
		JobToken:           strings.TrimSpace(env.Get("BITRIVER_TRANSCODER_TOKEN")),
		HealthEndpoint:     strings.TrimSpace(env.Get("BITRIVER_INGEST_HEALTH")),
		HealthTimeout:      2 * time.Second,
		MaxBootAttempts:    3,
		RetryInterval:      500 * time.Millisecond,
		HTTPMaxAttempts:    30,
		HTTPRetryInterval:  2 * time.Second,
	}
	parsePositiveInt := func(key string, target *int) error {
		if raw := strings.TrimSpace(env.Get(key)); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil {
				return fmt.Errorf("parse %s: %w", key, err)
			}
			if parsed > 0 {
				*target = parsed
			}
		}
		return nil
	}
	parseDuration := func(key string, allowZero bool, target *time.Duration) error {
		if raw := strings.TrimSpace(env.Get(key)); raw != "" {
			parsed, err := time.ParseDuration(raw)
			if err != nil {
				return fmt.Errorf("parse %s: %w", key, err)
			}
			if parsed > 0 || (allowZero && parsed == 0) {
				*target = parsed
			}
		}
		return nil
	}
	if err := parsePositiveInt("BITRIVER_INGEST_MAX_BOOT_ATTEMPTS", &cfg.MaxBootAttempts); err != nil {
		return IngestConfig{}, err
	}
	if err := parseDuration("BITRIVER_INGEST_RETRY_INTERVAL", false, &cfg.RetryInterval); err != nil {
		return IngestConfig{}, err
	}
	if err := parsePositiveInt("BITRIVER_INGEST_HTTP_MAX_ATTEMPTS", &cfg.HTTPMaxAttempts); err != nil {
		return IngestConfig{}, err
	}
	if err := parseDuration("BITRIVER_INGEST_HTTP_RETRY_INTERVAL", true, &cfg.HTTPRetryInterval); err != nil {
		return IngestConfig{}, err
	}
	if err := parseDuration("BITRIVER_INGEST_HEALTH_TIMEOUT", false, &cfg.HealthTimeout); err != nil {
		return IngestConfig{}, err
	}

	if ladder := strings.TrimSpace(env.Get("BITRIVER_TRANSCODE_LADDER")); ladder != "" {
		profiles, err := parseLadder(ladder)
		if err != nil {
			return IngestConfig{}, err
		}
		cfg.LadderProfiles = profiles
	} else {
		cfg.LadderProfiles = []Rendition{{Name: "1080p", Bitrate: 6000}, {Name: "720p", Bitrate: 4000}, {Name: "480p", Bitrate: 2500}}
	}
	if cfg.HealthEndpoint == "" {
		cfg.HealthEndpoint = "/healthz"
	}
	if err := cfg.Validate(); err != nil {
		if errors.Is(err, ErrIngestConfigDisabled) {
			return cfg, ErrIngestConfigDisabled
		}
		return IngestConfig{}, err
	}
	return cfg, nil
}

func (c IngestConfig) Validate() error {
	if !c.hasAnyConfig() {
		return ErrIngestConfigDisabled
	}
	if missing := c.missingRequiredFields(); len(missing) > 0 {
		return MissingIngestConfigError{Missing: missing}
	}
	if len(c.LadderProfiles) == 0 {
		return errors.New("no rendition profiles configured")
	}
	if c.MaxBootAttempts <= 0 {
		return errors.New("max boot attempts must be positive")
	}
	if c.RetryInterval < 0 {
		return errors.New("retry interval cannot be negative")
	}
	if c.HTTPMaxAttempts <= 0 {
		return errors.New("HTTP max attempts must be positive")
	}
	if c.HTTPRetryInterval < 0 {
		return errors.New("HTTP retry interval cannot be negative")
	}
	if c.HealthTimeout <= 0 {
		return errors.New("health timeout must be positive")
	}
	return nil
}

func (c IngestConfig) hasAnyConfig() bool {
	return c.SRSBaseURL != "" || c.SRSToken != "" ||
		c.OMEBaseURL != "" || c.OMEPlaybackBaseURL != "" || c.OMEAccessToken != "" || c.OMEUsername != "" || c.OMEPassword != "" ||
		c.JobBaseURL != "" || c.JobToken != ""
}

func (c IngestConfig) missingRequiredFields() []string {
	missing := make([]string, 0, 6)
	if c.SRSBaseURL == "" {
		missing = append(missing, "BITRIVER_SRS_API")
	}
	if c.SRSToken == "" {
		missing = append(missing, "BITRIVER_SRS_TOKEN")
	}
	if c.OMEBaseURL == "" {
		missing = append(missing, "BITRIVER_OME_API")
	}
	if c.OMEPlaybackBaseURL == "" {
		missing = append(missing, "BITRIVER_OME_PUBLIC_LLHLS_BASE_URL")
	}
	if c.OMEAccessToken == "" && (c.OMEUsername == "" || c.OMEPassword == "") {
		missing = append(missing, "BITRIVER_OME_API_TOKEN")
	}
	if c.JobBaseURL == "" {
		missing = append(missing, "BITRIVER_TRANSCODER_API")
	}
	if c.JobToken == "" {
		missing = append(missing, "BITRIVER_TRANSCODER_TOKEN")
	}
	return missing
}

func parseLadder(spec string) ([]Rendition, error) {
	entries := strings.Split(spec, ",")
	results := make([]Rendition, 0, len(entries))
	for _, entry := range entries {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		parts := strings.Split(trimmed, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid rendition spec %q", trimmed)
		}
		bitrate, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid bitrate for rendition %q: %w", trimmed, err)
		}
		results = append(results, Rendition{Name: parts[0], Bitrate: bitrate})
	}
	if len(results) == 0 {
		return nil, errors.New("no rendition profiles configured")
	}
	return results, nil
}
