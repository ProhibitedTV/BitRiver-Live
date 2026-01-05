package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
)

type SetupConfig struct {
	AdminEmail       string `json:"adminEmail"`
	AdminPassword    string `json:"adminPassword"`
	ViewerURL        string `json:"viewerUrl"`
	ViewerOrigin     string `json:"viewerOrigin"`
	PublicAPIURL     string `json:"publicApiUrl"`
	APIPort          int    `json:"apiPort"`
	TLSCertPath      string `json:"tlsCertPath"`
	TLSKeyPath       string `json:"tlsKeyPath"`
	PostgresPassword string `json:"postgresPassword"`
	RedisPassword    string `json:"redisPassword"`
	MetricsToken     string `json:"metricsToken"`
	SRSToken         string `json:"srsToken"`
	OMEToken         string `json:"omeToken"`
	TranscoderToken  string `json:"transcoderToken"`
}

type SetupResult struct {
	RestartScheduled bool `json:"restartScheduled"`
}

type SetupManager interface {
	ApplySetup(rctx context.Context, cfg SetupConfig) (SetupResult, error)
}

func (h *Handler) SetupWizard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		WriteError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}

	if _, ok := h.requireRole(w, r, roleAdmin); !ok {
		return
	}

	if h.Setup == nil {
		WriteError(w, http.StatusServiceUnavailable, fmt.Errorf("setup service unavailable"))
		return
	}

	var cfg SetupConfig
	if err := DecodeJSON(r, &cfg); err != nil {
		WriteError(w, http.StatusBadRequest, fmt.Errorf("invalid payload: %w", err))
		return
	}

	if err := validateSetupConfig(cfg); err != nil {
		WriteError(w, http.StatusBadRequest, err)
		return
	}

	result, err := h.Setup.ApplySetup(r.Context(), normalizeSetupConfig(cfg))
	if err != nil {
		var statusErr interface{ Status() int }
		if errors.As(err, &statusErr) {
			WriteError(w, statusErr.Status(), err)
			return
		}
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

func validateSetupConfig(cfg SetupConfig) error {
	if strings.TrimSpace(cfg.AdminEmail) == "" {
		return fmt.Errorf("adminEmail is required")
	}
	if _, err := mail.ParseAddress(cfg.AdminEmail); err != nil {
		return fmt.Errorf("invalid adminEmail: %w", err)
	}
	if strings.TrimSpace(cfg.ViewerURL) == "" {
		return fmt.Errorf("viewerUrl is required")
	}
	if _, err := parseURL(cfg.ViewerURL); err != nil {
		return fmt.Errorf("invalid viewerUrl: %w", err)
	}
	if cfg.PublicAPIURL != "" {
		if _, err := parseURL(cfg.PublicAPIURL); err != nil {
			return fmt.Errorf("invalid publicApiUrl: %w", err)
		}
	}
	if cfg.ViewerOrigin != "" {
		if _, err := parseURL(cfg.ViewerOrigin); err != nil {
			return fmt.Errorf("invalid viewerOrigin: %w", err)
		}
	}
	if cfg.APIPort <= 0 || cfg.APIPort > 65535 {
		return fmt.Errorf("apiPort must be between 1 and 65535")
	}
	if (cfg.TLSCertPath == "") != (cfg.TLSKeyPath == "") {
		return fmt.Errorf("both tlsCertPath and tlsKeyPath must be provided together")
	}

	required := map[string]string{
		"postgresPassword": cfg.PostgresPassword,
		"redisPassword":    cfg.RedisPassword,
		"metricsToken":     cfg.MetricsToken,
		"srsToken":         cfg.SRSToken,
		"omeToken":         cfg.OMEToken,
		"transcoderToken":  cfg.TranscoderToken,
	}
	for field, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", field)
		}
	}

	return nil
}

func parseURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("must include scheme and host")
	}
	return parsed, nil
}

func normalizeSetupConfig(cfg SetupConfig) SetupConfig {
	cfg.AdminEmail = strings.TrimSpace(cfg.AdminEmail)
	cfg.AdminPassword = strings.TrimSpace(cfg.AdminPassword)
	cfg.ViewerURL = strings.TrimSpace(cfg.ViewerURL)
	cfg.ViewerOrigin = strings.TrimSpace(cfg.ViewerOrigin)
	cfg.PublicAPIURL = strings.TrimSpace(cfg.PublicAPIURL)
	cfg.TLSCertPath = strings.TrimSpace(cfg.TLSCertPath)
	cfg.TLSKeyPath = strings.TrimSpace(cfg.TLSKeyPath)
	cfg.PostgresPassword = strings.TrimSpace(cfg.PostgresPassword)
	cfg.RedisPassword = strings.TrimSpace(cfg.RedisPassword)
	cfg.MetricsToken = strings.TrimSpace(cfg.MetricsToken)
	cfg.SRSToken = strings.TrimSpace(cfg.SRSToken)
	cfg.OMEToken = strings.TrimSpace(cfg.OMEToken)
	cfg.TranscoderToken = strings.TrimSpace(cfg.TranscoderToken)
	return cfg
}
