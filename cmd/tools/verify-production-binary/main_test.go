package main

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestVerifyBuildInfoAcceptsVersionedUpstreamModules(t *testing.T) {
	info := &debug.BuildInfo{Deps: []*debug.Module{
		{Path: "github.com/jackc/pgx/v5", Version: "v5.10.0"},
		{Path: "example.com/forked", Version: "v1.0.0", Replace: &debug.Module{Path: "example.com/fork", Version: "v1.1.0"}},
	}}
	if err := verifyBuildInfo(info, []string{"github.com/jackc/pgx/v5"}); err != nil {
		t.Fatalf("verify build info: %v", err)
	}
}

func TestVerifyBuildInfoRejectsLocalReplacement(t *testing.T) {
	info := &debug.BuildInfo{Deps: []*debug.Module{
		{Path: "github.com/jackc/pgx/v5", Version: "v5.10.0", Replace: &debug.Module{Path: "./third_party/github.com/jackc/pgx/v5", Version: "(devel)"}},
	}}
	err := verifyBuildInfo(info, []string{"github.com/jackc/pgx/v5"})
	if err == nil || !strings.Contains(err.Error(), "local replacement") {
		t.Fatalf("expected local replacement error, got %v", err)
	}
}

func TestVerifyBuildInfoRequiresNamedModule(t *testing.T) {
	err := verifyBuildInfo(&debug.BuildInfo{}, []string{"github.com/jackc/pgx/v5"})
	if err == nil || !strings.Contains(err.Error(), "is not linked") {
		t.Fatalf("expected missing module error, got %v", err)
	}
}
