package ingest

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config stores connectivity information for the ingest controller.
type Config struct {
	SRSBaseURL      string
	SRSToken        string
	OMEBaseURL      string
	OMEUsername     string
	OMEPassword     string
	JobBaseURL      string
	JobToken        string
	LadderProfiles  []Rendition
	HTTPClient      *http.Client
	HealthEndpoint  string
	MaxBootAttempts int
	RetryInterval   time.Duration
}

// LoadConfigFromEnv initialises a Config from environment variables.
func LoadConfigFromEnv() (Config, error) {
	cfg := Config{
		SRSBaseURL:      strings.TrimSpace(os.Getenv("BITRIVER_SRS_API")),
		SRSToken:        strings.TrimSpace(os.Getenv("BITRIVER_SRS_TOKEN")),
		OMEBaseURL:      strings.TrimSpace(os.Getenv("BITRIVER_OME_API")),
		OMEUsername:     strings.TrimSpace(os.Getenv("BITRIVER_OME_USERNAME")),
		OMEPassword:     strings.TrimSpace(os.Getenv("BITRIVER_OME_PASSWORD")),
		JobBaseURL:      strings.TrimSpace(os.Getenv("BITRIVER_TRANSCODER_API")),
		JobToken:        strings.TrimSpace(os.Getenv("BITRIVER_TRANSCODER_TOKEN")),
		HealthEndpoint:  strings.TrimSpace(os.Getenv("BITRIVER_INGEST_HEALTH")),
		MaxBootAttempts: 3,
		RetryInterval:   500 * time.Millisecond,
	}

	if cfg.HealthEndpoint == "" {
		cfg.HealthEndpoint = "/healthz"
	}

	if attempts := strings.TrimSpace(os.Getenv("BITRIVER_INGEST_MAX_BOOT_ATTEMPTS")); attempts != "" {
		parsed, err := strconv.Atoi(attempts)
		if err != nil {
			return Config{}, fmt.Errorf("parse BITRIVER_INGEST_MAX_BOOT_ATTEMPTS: %w", err)
		}
		if parsed > 0 {
			cfg.MaxBootAttempts = parsed
		}
	}

	if interval := strings.TrimSpace(os.Getenv("BITRIVER_INGEST_RETRY_INTERVAL")); interval != "" {
		parsed, err := time.ParseDuration(interval)
		if err != nil {
			return Config{}, fmt.Errorf("parse BITRIVER_INGEST_RETRY_INTERVAL: %w", err)
		}
		if parsed > 0 {
			cfg.RetryInterval = parsed
		}
	}

	if ladder := strings.TrimSpace(os.Getenv("BITRIVER_TRANSCODE_LADDER")); ladder != "" {
		profiles, err := parseLadder(ladder)
		if err != nil {
			return Config{}, err
		}
		cfg.LadderProfiles = profiles
	} else {
		cfg.LadderProfiles = []Rendition{
			{Name: "1080p", Bitrate: 6000},
			{Name: "720p", Bitrate: 4000},
			{Name: "480p", Bitrate: 2500},
		}
	}

	return cfg, nil
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

// Enabled reports whether enough configuration has been provided to talk to
// external ingest services.
func (c Config) Enabled() bool {
	return c.SRSBaseURL != "" && c.OMEBaseURL != "" && c.JobBaseURL != ""
}

// Validate ensures the configuration is usable.
func (c Config) Validate() error {
	if !c.Enabled() {
		return errors.New("ingest configuration incomplete")
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	}
	if len(c.LadderProfiles) == 0 {
		return errors.New("no rendition profiles configured")
	}
	return nil
}

// NewHTTPController constructs a Controller backed by HTTP APIs.
func (c Config) NewHTTPController() (*HTTPController, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	controller := &HTTPController{config: c}
	if controller.config.HTTPClient == nil {
		controller.config.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	return controller, nil
}
