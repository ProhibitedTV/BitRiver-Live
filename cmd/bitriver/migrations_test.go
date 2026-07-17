package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestValidateMigrationCommand(t *testing.T) {
	checksum := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	tests := []struct {
		name    string
		cfg     migrationCommandConfig
		wantErr bool
	}{
		{name: "plan", cfg: migrationCommandConfig{ComposeFile: "compose.yml", EnvFile: ".env", Mode: "plan"}},
		{name: "apply", cfg: migrationCommandConfig{ComposeFile: "compose.yml", EnvFile: ".env", Mode: "apply"}},
		{name: "repair", cfg: migrationCommandConfig{ComposeFile: "compose.yml", EnvFile: ".env", Mode: "repair", Repair: "retry", Filename: "0001_initial.sql", Checksum: checksum}},
		{name: "unknown mode", cfg: migrationCommandConfig{ComposeFile: "compose.yml", EnvFile: ".env", Mode: "down"}, wantErr: true},
		{name: "repair flags on plan", cfg: migrationCommandConfig{ComposeFile: "compose.yml", EnvFile: ".env", Mode: "plan", Filename: "0001_initial.sql"}, wantErr: true},
		{name: "repair missing checksum", cfg: migrationCommandConfig{ComposeFile: "compose.yml", EnvFile: ".env", Mode: "repair", Repair: "retry", Filename: "0001_initial.sql"}, wantErr: true},
		{name: "repair invalid checksum", cfg: migrationCommandConfig{ComposeFile: "compose.yml", EnvFile: ".env", Mode: "repair", Repair: "mark-applied", Filename: "0001_initial.sql", Checksum: "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMigrationCommand(tt.cfg)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRunMigrationComposeBuildsRepairInvocation(t *testing.T) {
	originalCommandRunner := commandRunner
	originalLookPath := lookPathRunner
	t.Cleanup(func() {
		commandRunner = originalCommandRunner
		lookPathRunner = originalLookPath
	})

	lookPathRunner = func(file string) (string, error) {
		if file != "docker" {
			return "", errors.New("unexpected executable")
		}
		return "docker", nil
	}
	var got []string
	commandRunner = func(name string, args ...string) error {
		got = append([]string{name}, args...)
		return nil
	}

	checksum := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	err := runMigrationCompose(migrationCommandConfig{
		ComposeFile: "deploy/docker-compose.yml",
		EnvFile:     ".env",
		Mode:        "repair",
		Repair:      "mark-applied",
		Filename:    "0001_initial.sql",
		Checksum:    checksum,
	})
	if err != nil {
		t.Fatalf("runMigrationCompose returned error: %v", err)
	}

	wantTail := []string{"postgres-migrations", "repair", "mark-applied", "0001_initial.sql", checksum}
	if len(got) < len(wantTail) {
		t.Fatalf("docker invocation too short: %v", got)
	}
	if tail := got[len(got)-len(wantTail):]; !reflect.DeepEqual(tail, wantTail) {
		t.Fatalf("docker invocation tail = %v, want %v", tail, wantTail)
	}
}
