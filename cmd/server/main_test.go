package main

import (
	"strings"
	"testing"

	"bitriver-live/internal/server"
)

func TestResolveMode(t *testing.T) {
	cases := []struct {
		name    string
		flag    string
		env     string
		want    string
		wantErr bool
	}{
		{name: "FlagWins", flag: "development", env: "production", want: "development"},
		{name: "EnvOnly", env: "production", want: "production"},
		{name: "Missing", wantErr: true},
		{name: "Invalid", flag: "staging", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mode, err := resolveMode(tc.flag, tc.env)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mode != tc.want {
				t.Fatalf("expected %q got %q", tc.want, mode)
			}
		})
	}
}

func TestValidateMetricsProtection(t *testing.T) {
	cases := []struct {
		name    string
		mode    string
		cfg     server.MetricsAccessConfig
		wantErr bool
	}{
		{name: "ProductionRequiresProtection", mode: "production", wantErr: true},
		{name: "ProductionWithToken", mode: "production", cfg: server.MetricsAccessConfig{Token: "secret"}},
		{name: "DevelopmentAllowsEmpty", mode: "development"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMetricsProtection(tc.mode, tc.cfg)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				if !strings.Contains(err.Error(), "BITRIVER_LIVE_METRICS_TOKEN") {
					t.Fatalf("expected guidance, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestResolveStorageDriverDefaultsToPostgres(t *testing.T) {
	driver, explicit, err := resolveStorageDriver("", "", "postgres://example")
	if err != nil {
		t.Fatalf("resolveStorageDriver returned error: %v", err)
	}
	if explicit {
		t.Fatalf("expected implicit default")
	}
	if driver != "postgres" {
		t.Fatalf("expected postgres, got %q", driver)
	}
}
