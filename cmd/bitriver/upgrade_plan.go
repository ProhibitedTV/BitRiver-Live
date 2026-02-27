package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type upgradePlan struct {
	CurrentVersion string
	TargetVersion  string
	Supported      bool
	Warnings       []string
	Steps          []string
}

func runUpgradePlan(args []string) error {
	fs := flag.NewFlagSet("upgrade-plan", flag.ContinueOnError)
	target := fs.String("to", "", "target BitRiver Live tag (example: v1.3.0)")
	envFile := fs.String("env-file", defaultEnvFile(), "environment file used to detect deployed image tags")
	current := fs.String("current", "", "override currently deployed BitRiver Live tag")
	checkSchema := fs.Bool("check-schema", false, "compare current schema version with expected migration version")
	currentSchema := fs.String("current-schema", "", "current applied schema version (for --check-schema)")
	expectedSchema := fs.String("expected-schema", "", "expected schema version override (defaults to latest deploy/migrations prefix)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*target) == "" {
		return errors.New("missing required --to tag")
	}

	currentVersion := strings.TrimSpace(*current)
	if currentVersion == "" {
		values, err := loadEnvValues(*envFile, false)
		if err != nil {
			return fmt.Errorf("load env values: %w", err)
		}
		currentVersion = strings.TrimSpace(values["BITRIVER_LIVE_IMAGE_TAG"])
		if currentVersion == "" {
			return fmt.Errorf("BITRIVER_LIVE_IMAGE_TAG missing in %s; set it or pass --current", *envFile)
		}
	}

	plan, err := buildUpgradePlan(currentVersion, *target)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, "BitRiver Live upgrade plan")
	fmt.Fprintf(os.Stdout, "Current: %s\n", plan.CurrentVersion)
	fmt.Fprintf(os.Stdout, "Target:  %s\n", plan.TargetVersion)
	if plan.Supported {
		fmt.Fprintln(os.Stdout, "Path status: SUPPORTED")
	} else {
		fmt.Fprintln(os.Stdout, "Path status: NOT SUPPORTED")
	}

	if len(plan.Warnings) > 0 {
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "Warnings:")
		for _, warning := range plan.Warnings {
			fmt.Fprintf(os.Stdout, "- %s\n", warning)
		}
	}

	if *checkSchema {
		expected := strings.TrimSpace(*expectedSchema)
		if expected == "" {
			latest, err := detectLatestMigrationVersion(filepath.Join(repoRoot(), "deploy", "migrations"))
			if err != nil {
				return fmt.Errorf("detect latest migration version: %w", err)
			}
			expected = latest
		}
		if strings.TrimSpace(*currentSchema) == "" {
			fmt.Fprintf(os.Stdout, "\nSchema check: WARN (missing --current-schema, expected %s)\n", expected)
			fmt.Fprintln(os.Stdout, "- Provide --current-schema from your migration metadata before maintenance starts.")
		} else if strings.TrimSpace(*currentSchema) != expected {
			fmt.Fprintf(os.Stdout, "\nSchema check: WARN (current=%s expected=%s)\n", strings.TrimSpace(*currentSchema), expected)
			fmt.Fprintln(os.Stdout, "- Apply pending migrations (or verify metadata) before upgrading application services.")
		} else {
			fmt.Fprintf(os.Stdout, "\nSchema check: PASS (current=%s expected=%s)\n", strings.TrimSpace(*currentSchema), expected)
		}
	}

	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Recommended sequence:")
	for i, step := range plan.Steps {
		fmt.Fprintf(os.Stdout, "%d. %s\n", i+1, step)
	}

	releaseNotesPath := filepath.Join(repoRoot(), "docs", "releases", normalizeReleaseNotesTag(plan.TargetVersion)+".md")
	if _, err := os.Stat(releaseNotesPath); err == nil {
		fmt.Fprintf(os.Stdout, "\nRelease notes: %s\n", releaseNotesPath)
	} else {
		fmt.Fprintln(os.Stdout, "\nRelease notes: not found in docs/releases/ for target tag; review release announcement manually.")
	}

	return nil
}

func buildUpgradePlan(current, target string) (upgradePlan, error) {
	normalizedCurrent, currentSemver, err := normalizeSemverTag(current)
	if err != nil {
		return upgradePlan{}, fmt.Errorf("parse current version %q: %w", current, err)
	}
	normalizedTarget, targetSemver, err := normalizeSemverTag(target)
	if err != nil {
		return upgradePlan{}, fmt.Errorf("parse target version %q: %w", target, err)
	}

	cmp := compareSemver(currentSemver, targetSemver)
	if cmp == 0 {
		return upgradePlan{}, errors.New("current and target versions are identical")
	}
	if cmp > 0 {
		return upgradePlan{}, errors.New("downgrade detected; use docs/upgrades.md rollback section instead of upgrade-plan")
	}

	currentParts := parseSemverParts(currentSemver)
	targetParts := parseSemverParts(targetSemver)

	warnings := []string{}
	supported := true
	if targetParts[0] > currentParts[0]+1 {
		supported = false
		warnings = append(warnings, "major version skip detected. Upgrade only one major at a time with intermediate releases.")
	} else if targetParts[0] == currentParts[0]+1 {
		warnings = append(warnings, "major upgrade detected. Expect possible breaking changes and non-reversible schema migrations.")
	} else if targetParts[0] == currentParts[0] && targetParts[1] > currentParts[1]+1 {
		supported = false
		warnings = append(warnings, "minor version skip detected. Supported path is N-1 minor hops only.")
	}

	steps := []string{
		"Read docs/upgrades.md and target release notes; identify breaking changes and new env keys.",
		"Take backups (database dump + deploy/data, deploy/transcoder-data, deploy/ome, and .env).",
		"Stop services without deleting volumes: docker compose -f deploy/docker-compose.yml down",
		"Update .env image tags to target, run deploy/check-env.sh, and rerender OME config.",
		"Run migrations with docker compose -f deploy/docker-compose.yml run --rm postgres-migrations and inspect output.",
		"Start stack: docker compose -f deploy/docker-compose.yml up -d, then run go run ./cmd/bitriver verify --compose-file deploy/docker-compose.yml --env-file .env",
	}

	return upgradePlan{
		CurrentVersion: normalizedCurrent,
		TargetVersion:  normalizedTarget,
		Supported:      supported,
		Warnings:       warnings,
		Steps:          steps,
	}, nil
}

func normalizeSemverTag(raw string) (string, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", errors.New("empty version")
	}
	cleaned := strings.TrimPrefix(trimmed, "v")
	version := extractVersion(cleaned)
	if version == "" {
		return "", "", errors.New("unable to parse semantic version")
	}
	parts := parseSemverParts(version)
	return "v" + version, fmt.Sprintf("%d.%d.%d", parts[0], parts[1], parts[2]), nil
}

func normalizeReleaseNotesTag(tag string) string {
	if strings.HasPrefix(strings.TrimSpace(tag), "v") {
		return strings.TrimSpace(tag)
	}
	return "v" + strings.TrimSpace(tag)
}

func detectLatestMigrationVersion(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	versions := make([]int, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		prefix := strings.SplitN(name, "_", 2)[0]
		if prefix == "" {
			continue
		}
		n, convErr := strconv.Atoi(prefix)
		if convErr != nil {
			continue
		}
		versions = append(versions, n)
	}
	if len(versions) == 0 {
		return "", errors.New("no migration versions found")
	}
	sort.Ints(versions)
	return fmt.Sprintf("%04d", versions[len(versions)-1]), nil
}
