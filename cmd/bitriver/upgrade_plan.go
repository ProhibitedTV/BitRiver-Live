package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
)

type migrationPlanStatus struct {
	Status  string
	Details string
}

type upgradeImageTag struct {
	Service string
	Image   string
	Tag     string
	Source  string
}

var upgradePlanComposePSRunner = runComposePS

func runUpgradePlan(args []string) error {
	fs := flag.NewFlagSet("upgrade-plan", flag.ContinueOnError)
	composeFile := fs.String("compose-file", defaultComposeFile(), "compose file used for stack inspection")
	envFile := fs.String("env-file", defaultEnvFile(), "environment file used for image tag fallbacks")
	target := fs.String("target", "", "target BitRiver Live tag (example: v1.3.0)")
	legacyTarget := fs.String("to", "", "deprecated alias for --target")
	if err := fs.Parse(args); err != nil {
		return err
	}

	resolvedTarget := strings.TrimSpace(*target)
	if resolvedTarget == "" {
		resolvedTarget = strings.TrimSpace(*legacyTarget)
	}
	if resolvedTarget == "" {
		return errors.New("missing required --target tag")
	}

	warnings := []string{}
	tags, tagWarnings := detectCurrentUpgradeTags(*composeFile, *envFile)
	warnings = append(warnings, tagWarnings...)
	migration := detectMigrationPlanStatus(*composeFile)

	fmt.Fprintln(os.Stdout, "BitRiver Live upgrade plan")
	fmt.Fprintf(os.Stdout, "Planner version: %s (%s, %s)\n", valueOrFallback(Version, "dev"), valueOrFallback(Commit, "unknown"), valueOrFallback(Date, "unknown"))
	fmt.Fprintf(os.Stdout, "Compose file: %s\n", *composeFile)
	fmt.Fprintf(os.Stdout, "Env file: %s\n", *envFile)
	fmt.Fprintf(os.Stdout, "Target tag: %s\n", resolvedTarget)

	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Current image tags (best-effort):")
	if len(tags) == 0 {
		fmt.Fprintln(os.Stdout, "- none detected")
	} else {
		for _, tag := range tags {
			fmt.Fprintf(os.Stdout, "- %s: %s (tag=%s, source=%s)\n", tag.Service, tag.Image, tag.Tag, tag.Source)
		}
	}

	fmt.Fprintf(os.Stdout, "\nMigrations: %s\n", migration.Status)
	fmt.Fprintf(os.Stdout, "- %s\n", migration.Details)

	if len(warnings) > 0 {
		fmt.Fprintln(os.Stdout, "\nWarnings:")
		for _, warning := range warnings {
			fmt.Fprintf(os.Stdout, "- WARN: %s\n", warning)
		}
	}

	fmt.Fprintln(os.Stdout, "\nOperator checklist:")
	fmt.Fprintln(os.Stdout, "[ ] 1) Review upgrade notes: docs/upgrades.md and docs/production-release.md")
	fmt.Fprintln(os.Stdout, "[ ] 2) Complete backups before maintenance (docs/upgrades.md#backup-and-restore-checklist-required)")
	fmt.Fprintln(os.Stdout, "[ ] 3) Run read-only migration preflight: bitriver migrations --mode plan --compose-file deploy/docker-compose.yml --env-file .env")
	fmt.Fprintf(os.Stdout, "[ ] 4) Update image tags/digests in .env to target %s and run deploy/check-env.sh\n", resolvedTarget)
	fmt.Fprintln(os.Stdout, "[ ] 5) Stop stack safely: docker compose -f deploy/docker-compose.yml down")
	fmt.Fprintln(os.Stdout, "[ ] 6) Apply pending migrations: bitriver migrations --mode apply --compose-file deploy/docker-compose.yml --env-file .env")
	fmt.Fprintln(os.Stdout, "[ ] 7) Start updated stack and validate health: go run ./cmd/bitriver verify --compose-file deploy/docker-compose.yml --env-file .env")
	fmt.Fprintln(os.Stdout, "[ ] 8) Record migration status and rollback decision in your runbook/change ticket")

	fmt.Fprintln(os.Stdout, "\nRollback caveats:")
	fmt.Fprintln(os.Stdout, "- Safe rollback usually requires that irreversible migrations have NOT run.")
	fmt.Fprintln(os.Stdout, "- If schema/data migrations were irreversible, restore Postgres + volume/config backups before downgrading images.")
	fmt.Fprintln(os.Stdout, "- When uncertain, treat rollback as unsafe and follow docs/upgrades.md rollback guidance.")

	return nil
}

func detectCurrentUpgradeTags(composeFile, envFile string) ([]upgradeImageTag, []string) {
	warnings := []string{}
	runningTags, err := detectRunningComposeImageTags(composeFile, envFile)
	if err == nil && len(runningTags) > 0 {
		return runningTags, warnings
	}
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("unable to read running image tags from docker compose ps: %v", err))
	}

	envTags, envErr := detectEnvImageTags(envFile)
	if envErr != nil {
		warnings = append(warnings, fmt.Sprintf("could not read env image tags: %v", envErr))
		warnings = append(warnings, "current version could not be determined. If a stack is running, rerun with Docker available, or pass the right --env-file.")
		return nil, warnings
	}
	if len(envTags) == 0 {
		warnings = append(warnings, "current version could not be determined from running stack or env tags. Ensure image tag variables exist in .env.")
		return nil, warnings
	}

	warnings = append(warnings, "using env-file tag values because running compose service tags were unavailable.")
	return envTags, warnings
}

func detectRunningComposeImageTags(composeFile, envFile string) ([]upgradeImageTag, error) {
	output, err := upgradePlanComposePSRunner(composeFile, envFile)
	if err != nil {
		return nil, err
	}
	type composeService struct {
		Service string `json:"Service"`
		Name    string `json:"Name"`
		Image   string `json:"Image"`
	}
	var services []composeService
	if err := json.Unmarshal(output, &services); err != nil {
		return nil, fmt.Errorf("parse docker compose ps output: %w", err)
	}

	tags := make([]upgradeImageTag, 0, len(services))
	for _, svc := range services {
		service := strings.TrimSpace(svc.Service)
		if service == "" {
			service = strings.TrimSpace(svc.Name)
		}
		if service == "" || strings.TrimSpace(svc.Image) == "" {
			continue
		}
		tags = append(tags, upgradeImageTag{Service: service, Image: strings.TrimSpace(svc.Image), Tag: imageTagFromRef(svc.Image), Source: "compose-ps"})
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Service < tags[j].Service })
	return tags, nil
}

func detectEnvImageTags(envFile string) ([]upgradeImageTag, error) {
	values, err := loadEnvValues(envFile, false)
	if err != nil {
		return nil, err
	}
	serviceToKey := []struct {
		service string
		key     string
	}{
		{service: "bitriver-live", key: "BITRIVER_LIVE_IMAGE_TAG"},
		{service: "viewer", key: "BITRIVER_VIEWER_IMAGE_TAG"},
		{service: "srs-controller", key: "BITRIVER_SRS_CONTROLLER_IMAGE_TAG"},
		{service: "transcoder", key: "BITRIVER_TRANSCODER_IMAGE_TAG"},
		{service: "srs", key: "BITRIVER_SRS_IMAGE_TAG"},
		{service: "ome", key: "BITRIVER_OME_IMAGE_TAG"},
	}

	tags := []upgradeImageTag{}
	for _, entry := range serviceToKey {
		tag := strings.TrimSpace(values[entry.key])
		if tag == "" {
			continue
		}
		tags = append(tags, upgradeImageTag{Service: entry.service, Image: tag, Tag: normalizeTagValue(tag), Source: "env-file"})
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Service < tags[j].Service })
	return tags, nil
}

func detectMigrationPlanStatus(composeFile string) migrationPlanStatus {
	data, err := os.ReadFile(composeFile)
	if err != nil {
		return migrationPlanStatus{Status: "UNKNOWN", Details: fmt.Sprintf("unable to inspect compose file for migration wiring: %v", err)}
	}
	body := string(data)
	if strings.Contains(body, "postgres-migrations:") {
		if strings.Contains(body, "bitriver-live:") && strings.Contains(body, "postgres-migrations:") {
			return migrationPlanStatus{Status: "EXPECTED", Details: "compose file includes postgres-migrations service; migrations are expected before API startup in the default deployment contract."}
		}
		return migrationPlanStatus{Status: "EXPECTED", Details: "postgres-migrations service exists; confirm service dependencies/order in compose overrides before maintenance."}
	}
	return migrationPlanStatus{Status: "UNKNOWN", Details: "postgres-migrations service not detected in compose file; verify migration strategy manually before upgrading."}
}

func imageTagFromRef(image string) string {
	trimmed := strings.TrimSpace(image)
	if trimmed == "" {
		return "unknown"
	}
	withoutDigest := strings.SplitN(trimmed, "@", 2)[0]
	slash := strings.LastIndex(withoutDigest, "/")
	colon := strings.LastIndex(withoutDigest, ":")
	if colon > slash {
		return normalizeTagValue(withoutDigest[colon+1:])
	}
	if strings.Contains(trimmed, "@") {
		return "digest-only"
	}
	return "unknown"
}

func normalizeTagValue(tag string) string {
	trimmed := strings.TrimSpace(tag)
	if trimmed == "" {
		return "unknown"
	}
	if strings.HasPrefix(trimmed, "v") {
		return trimmed
	}
	return "v" + trimmed
}
