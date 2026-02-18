package config

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

type TranscoderConfig struct {
	Bind            string
	Token           string
	OutputRoot      string
	PublicBaseURL   string
	PublicDir       string
	LiveRetention   time.Duration
	UploadRetention time.Duration
	OTELEndpoint    string
	OTELSampleRatio string
	OTELSamplerArg  string
	Environment     string
}

func LoadTranscoderFromEnv(env Environment) (TranscoderConfig, error) {
	cfg := TranscoderConfig{
		Bind:            firstNonEmpty(strings.TrimSpace(env.Get("JOB_CONTROLLER_BIND")), ":9000"),
		Token:           strings.TrimSpace(env.Get("JOB_CONTROLLER_TOKEN")),
		OutputRoot:      firstNonEmpty(strings.TrimSpace(env.Get("JOB_CONTROLLER_OUTPUT_ROOT")), "./work"),
		PublicBaseURL:   strings.TrimSpace(env.Get("BITRIVER_TRANSCODER_PUBLIC_BASE_URL")),
		PublicDir:       strings.TrimSpace(env.Get("BITRIVER_TRANSCODER_PUBLIC_DIR")),
		OTELEndpoint:    firstNonEmpty(env.Get("BITRIVER_LIVE_OTEL_EXPORTER_OTLP_ENDPOINT"), env.Get("OTEL_EXPORTER_OTLP_ENDPOINT")),
		OTELSampleRatio: env.Get("BITRIVER_LIVE_OTEL_SAMPLE_RATIO"),
		OTELSamplerArg:  env.Get("OTEL_TRACES_SAMPLER_ARG"),
		Environment:     env.Get("BITRIVER_LIVE_ENVIRONMENT"),
	}
	if cfg.PublicDir == "" {
		cfg.PublicDir = filepath.Join(cfg.OutputRoot, "public")
	}
	var err error
	if cfg.LiveRetention, err = parseDurationEnv(env, "BITRIVER_TRANSCODER_RETENTION_LIVE"); err != nil {
		return TranscoderConfig{}, fmt.Errorf("invalid BITRIVER_TRANSCODER_RETENTION_LIVE: %w", err)
	}
	if cfg.UploadRetention, err = parseDurationEnv(env, "BITRIVER_TRANSCODER_RETENTION_UPLOADS"); err != nil {
		return TranscoderConfig{}, fmt.Errorf("invalid BITRIVER_TRANSCODER_RETENTION_UPLOADS: %w", err)
	}
	if cfg.Token == "" {
		return TranscoderConfig{}, fmt.Errorf("JOB_CONTROLLER_TOKEN must be configured before starting the transcoder")
	}
	if cfg.PublicBaseURL == "" {
		return TranscoderConfig{}, fmt.Errorf("BITRIVER_TRANSCODER_PUBLIC_BASE_URL must be configured before starting the transcoder")
	}
	return cfg, nil
}

type SRSControllerConfig struct {
	Bind     string
	Upstream *url.URL
	Token    string
}

func LoadSRSControllerFromEnv(env Environment) (SRSControllerConfig, error) {
	bind := firstNonEmpty(strings.TrimSpace(env.Get("SRS_CONTROLLER_BIND")), ":1985")
	upstreamRaw := firstNonEmpty(strings.TrimSpace(env.Get("SRS_CONTROLLER_UPSTREAM")), "http://localhost:1985/api/")
	upstream, err := url.Parse(upstreamRaw)
	if err != nil {
		return SRSControllerConfig{}, fmt.Errorf("parse upstream URL: %w", err)
	}
	if upstream.Scheme == "" || upstream.Host == "" {
		return SRSControllerConfig{}, fmt.Errorf("SRS_CONTROLLER_UPSTREAM must include scheme and host")
	}
	if !strings.HasSuffix(upstream.Path, "/") {
		upstream.Path += "/"
	}
	token := strings.TrimSpace(env.Get("BITRIVER_SRS_TOKEN"))
	if token == "" {
		return SRSControllerConfig{}, fmt.Errorf("BITRIVER_SRS_TOKEN must be set")
	}
	return SRSControllerConfig{Bind: bind, Upstream: upstream, Token: token}, nil
}

func parseDurationEnv(env Environment, key string) (time.Duration, error) {
	raw := strings.TrimSpace(env.Get(key))
	if raw == "" {
		return 0, nil
	}
	return time.ParseDuration(raw)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
