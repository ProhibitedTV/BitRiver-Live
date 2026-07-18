package ingest

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"bitriver-live/internal/config"
)

var ErrConfigDisabled = config.ErrIngestConfigDisabled

// MissingConfigError reports which environment variables are required to enable
// the ingest controller.
type MissingConfigError struct {
	Missing []string
}

func (e MissingConfigError) Error() string {
	return config.MissingIngestConfigError{Missing: e.Missing}.Error()
}

// Config stores connectivity information for the ingest controller.
type Config struct {
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
	HTTPClient         *http.Client
	HealthEndpoint     string
	HealthTimeout      time.Duration
	MaxBootAttempts    int
	RetryInterval      time.Duration
	HTTPMaxAttempts    int
	HTTPRetryInterval  time.Duration
}

// LoadConfigFromEnv initialises a Config from environment variables.
func LoadConfigFromEnv() (Config, error) {
	parsed, err := config.LoadIngestFromEnv(config.LoadEnvironment())
	if err != nil {
		if errors.Is(err, config.ErrIngestConfigDisabled) {
			return fromConfig(parsed), ErrConfigDisabled
		}
		var missing config.MissingIngestConfigError
		if errors.As(err, &missing) {
			return fromConfig(parsed), MissingConfigError{Missing: missing.Missing}
		}
		return Config{}, err
	}
	return fromConfig(parsed), nil
}

func fromConfig(parsed config.IngestConfig) Config {
	profiles := make([]Rendition, 0, len(parsed.LadderProfiles))
	for _, profile := range parsed.LadderProfiles {
		profiles = append(profiles, Rendition{Name: profile.Name, Bitrate: profile.Bitrate})
	}
	return Config{SRSBaseURL: parsed.SRSBaseURL, SRSToken: parsed.SRSToken, OMEBaseURL: parsed.OMEBaseURL, OMEPlaybackBaseURL: parsed.OMEPlaybackBaseURL, OMEAccessToken: parsed.OMEAccessToken, OMEUsername: parsed.OMEUsername, OMEPassword: parsed.OMEPassword, JobBaseURL: parsed.JobBaseURL, JobToken: parsed.JobToken, LadderProfiles: profiles, HealthEndpoint: parsed.HealthEndpoint, HealthTimeout: parsed.HealthTimeout, MaxBootAttempts: parsed.MaxBootAttempts, RetryInterval: parsed.RetryInterval, HTTPMaxAttempts: parsed.HTTPMaxAttempts, HTTPRetryInterval: parsed.HTTPRetryInterval}
}

// Enabled reports whether enough configuration has been provided to talk to
// external ingest services.
func (c Config) Enabled() bool {
	return c.Validate() == nil
}

// Validate ensures the configuration is usable.
func (c Config) Validate() error {
	if !c.hasAnyConfig() {
		return ErrConfigDisabled
	}
	if missing := c.missingRequiredFields(); len(missing) > 0 {
		return MissingConfigError{Missing: missing}
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

func (c Config) hasAnyConfig() bool {
	return c.SRSBaseURL != "" || c.SRSToken != "" ||
		c.OMEBaseURL != "" || c.OMEPlaybackBaseURL != "" || c.OMEAccessToken != "" || c.OMEUsername != "" || c.OMEPassword != "" ||
		c.JobBaseURL != "" || c.JobToken != ""
}

func (c Config) missingRequiredFields() []string {
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

// NewHTTPController constructs a Controller backed by HTTP APIs.
func (c Config) NewHTTPController() (*HTTPController, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	controller := &HTTPController{config: c, retryAttempts: c.HTTPMaxAttempts, retryInterval: c.HTTPRetryInterval}
	if controller.config.HTTPClient == nil {
		controller.config.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	controller.logger = slog.Default()
	return controller, nil
}
