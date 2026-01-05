package main

import (
	"errors"
	"strings"
	"testing"
)

func TestActionArgs(t *testing.T) {
	cases := map[string][]string{
		"start":   {"up", "-d", "--build"},
		"stop":    {"stop"},
		"restart": {"restart"},
		"logs":    {"logs", "--tail", "200"},
	}

	for name, expected := range cases {
		args, err := actionArgs(name)
		if err != nil {
			t.Fatalf("action %s returned error: %v", name, err)
		}
		if strings.Join(args, " ") != strings.Join(expected, " ") {
			t.Fatalf("action %s: expected %v, got %v", name, expected, args)
		}
	}

	if _, err := actionArgs("unknown"); err == nil {
		t.Fatalf("expected error for unknown action")
	}
}

func TestReadComposeStatus(t *testing.T) {
	original := composeCommandOutput
	defer func() { composeCommandOutput = original }()

	composeCommandOutput = func(_, _ string, args ...string) (string, error) {
		if len(args) < 1 || args[0] != "ps" {
			return "", errors.New("unexpected command")
		}
		return "api|running|healthy\nredis|exited|unhealthy\n", nil
	}

	statuses, err := readComposeStatus("compose.yml", "env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}
	if statuses[0].Service != "api" || statuses[0].State != "running" || statuses[0].Health != "healthy" {
		t.Fatalf("unexpected first status: %+v", statuses[0])
	}
}
