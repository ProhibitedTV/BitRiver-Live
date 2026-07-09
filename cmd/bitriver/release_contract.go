package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const contractSnapshotSchemaVersion = "bitriver.contract.v1"

type contractSnapshot struct {
	SchemaVersion    string                      `json:"schemaVersion"`
	Sources          contractSnapshotSources     `json:"sources"`
	Env              []contractEnvVar            `json:"env"`
	ComposeServices  []contractComposeService    `json:"composeServices"`
	Migrations       []contractMigration         `json:"migrations"`
	Generated        []contractGeneratedArtifact `json:"generatedArtifacts"`
	HealthEndpoints  []contractHealthEndpoint    `json:"healthEndpoints"`
	ReleaseArtifacts contractReleaseArtifacts    `json:"releaseArtifacts"`
}

type contractSnapshotSources struct {
	EnvFile       string `json:"envFile"`
	ComposeFile   string `json:"composeFile"`
	MigrationsDir string `json:"migrationsDir"`
}

type contractEnvVar struct {
	Key               string   `json:"key"`
	Default           string   `json:"default"`
	Comments          []string `json:"comments,omitempty"`
	Required          bool     `json:"required"`
	SecuritySensitive bool     `json:"securitySensitive"`
	Documented        bool     `json:"documented"`
}

type contractComposeService struct {
	Name         string   `json:"name"`
	Image        string   `json:"image,omitempty"`
	Build        string   `json:"build,omitempty"`
	Profiles     []string `json:"profiles,omitempty"`
	Ports        []string `json:"ports,omitempty"`
	Volumes      []string `json:"volumes,omitempty"`
	DependsOn    []string `json:"dependsOn,omitempty"`
	EnvRefs      []string `json:"envRefs,omitempty"`
	Healthcheck  bool     `json:"healthcheck"`
	Restart      string   `json:"restart,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

type contractMigration struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

type contractGeneratedArtifact struct {
	Path      string `json:"path"`
	Exists    bool   `json:"exists"`
	Generator string `json:"generator,omitempty"`
}

type contractHealthEndpoint struct {
	Name   string `json:"name"`
	Method string `json:"method"`
	Path   string `json:"path"`
}

type contractReleaseArtifacts struct {
	ImageTagKeys    []string `json:"imageTagKeys"`
	ImageDigestKeys []string `json:"imageDigestKeys"`
	LauncherInputs  []string `json:"launcherInputs"`
}

type contractDiffReport struct {
	Summary contractDiffSummary `json:"summary"`
	Changes []contractDiffItem  `json:"changes"`
}

type contractDiffSummary struct {
	Additive     int `json:"additive"`
	Breaking     int `json:"breaking"`
	Security     int `json:"security"`
	Undocumented int `json:"undocumented"`
}

type contractDiffItem struct {
	Severity string `json:"severity"`
	Area     string `json:"area"`
	Key      string `json:"key"`
	Detail   string `json:"detail"`
}

func runRelease(args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: release <contract-snapshot|contract-diff|smoke-gate|canary> [flags]")
		return errors.New("release subcommand required")
	}
	switch args[0] {
	case "contract-snapshot":
		return runContractSnapshot(args[1:])
	case "contract-diff":
		return runContractDiff(args[1:])
	case "smoke-gate":
		return runReleaseSmokeGate(args[1:])
	case "canary":
		return runReleaseCanary(args[1:])
	default:
		return fmt.Errorf("unknown release subcommand: %s", args[0])
	}
}

func runContractSnapshot(args []string) error {
	fs := flag.NewFlagSet("release contract-snapshot", flag.ContinueOnError)
	envFile := fs.String("env-file", defaultExampleEnv(), "environment template to snapshot")
	composeFile := fs.String("compose-file", defaultComposeFile(), "compose file to snapshot")
	migrationsDir := fs.String("migrations-dir", filepath.Join(repoRoot(), "deploy", "migrations"), "migration directory to snapshot")
	output := fs.String("output", "", "write JSON snapshot to file instead of stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}

	snapshot, err := buildContractSnapshot(*envFile, *composeFile, *migrationsDir)
	if err != nil {
		return err
	}
	return writeJSONOutput(snapshot, *output)
}

func runContractDiff(args []string) error {
	fs := flag.NewFlagSet("release contract-diff", flag.ContinueOnError)
	basePath := fs.String("base", "", "base contract snapshot JSON")
	headPath := fs.String("head", "", "head contract snapshot JSON")
	output := fs.String("output", "", "write JSON diff report to file instead of stdout")
	allowBreaking := fs.Bool("allow-breaking", false, "exit zero even when breaking/security drift is detected")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*basePath) == "" || strings.TrimSpace(*headPath) == "" {
		return errors.New("contract-diff requires --base and --head")
	}

	base, err := readContractSnapshot(*basePath)
	if err != nil {
		return fmt.Errorf("read base snapshot: %w", err)
	}
	head, err := readContractSnapshot(*headPath)
	if err != nil {
		return fmt.Errorf("read head snapshot: %w", err)
	}
	report := diffContractSnapshots(base, head)
	if err := writeJSONOutput(report, *output); err != nil {
		return err
	}
	if !*allowBreaking && (report.Summary.Breaking > 0 || report.Summary.Security > 0 || report.Summary.Undocumented > 0) {
		return fmt.Errorf("contract drift requires review: breaking=%d security=%d undocumented=%d", report.Summary.Breaking, report.Summary.Security, report.Summary.Undocumented)
	}
	return nil
}

func buildContractSnapshot(envFile, composeFile, migrationsDir string) (contractSnapshot, error) {
	envVars, err := parseContractEnv(envFile)
	if err != nil {
		return contractSnapshot{}, fmt.Errorf("snapshot env contract: %w", err)
	}
	services, err := parseComposeContract(composeFile)
	if err != nil {
		return contractSnapshot{}, fmt.Errorf("snapshot compose contract: %w", err)
	}
	migrations, err := snapshotMigrations(migrationsDir)
	if err != nil {
		return contractSnapshot{}, fmt.Errorf("snapshot migrations: %w", err)
	}

	return contractSnapshot{
		SchemaVersion: contractSnapshotSchemaVersion,
		Sources: contractSnapshotSources{
			EnvFile:       filepath.ToSlash(envFile),
			ComposeFile:   filepath.ToSlash(composeFile),
			MigrationsDir: filepath.ToSlash(migrationsDir),
		},
		Env:             envVars,
		ComposeServices: services,
		Migrations:      migrations,
		Generated: []contractGeneratedArtifact{
			generatedArtifact("deploy/ome/Server.generated.xml", "go run ./cmd/bitriver ome render --force --env-file ./.env"),
			generatedArtifact("deploy/srs/conf/srs.generated.conf", "scripts/render-srs-config.sh --force"),
		},
		HealthEndpoints: []contractHealthEndpoint{
			{Name: "API readiness", Method: "GET", Path: "/readyz"},
			{Name: "API health", Method: "GET", Path: "/healthz"},
			{Name: "Operator status", Method: "GET", Path: "/api/status"},
			{Name: "Viewer", Method: "GET", Path: "/viewer"},
			{Name: "SRS controller health", Method: "GET", Path: "/healthz"},
			{Name: "Transcoder health", Method: "GET", Path: "/healthz"},
		},
		ReleaseArtifacts: releaseArtifactInputs(envVars),
	}, nil
}

func parseContractEnv(path string) ([]contractEnvVar, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var vars []contractEnvVar
	pendingComments := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "#") {
			comment := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			if comment != "" {
				pendingComments = append(pendingComments, comment)
			}
			continue
		}
		key, value, ok := parseTemplateLine(raw)
		if !ok {
			if trimmed == "" {
				pendingComments = nil
			}
			continue
		}
		comments := append([]string(nil), pendingComments...)
		vars = append(vars, contractEnvVar{
			Key:               key,
			Default:           value,
			Comments:          comments,
			Required:          strings.Contains(value, ":?"),
			SecuritySensitive: securitySensitiveEnvKey(key),
			Documented:        len(comments) > 0,
		})
		pendingComments = nil
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Slice(vars, func(i, j int) bool { return vars[i].Key < vars[j].Key })
	return vars, nil
}

func parseComposeContract(path string) ([]contractComposeService, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	inServices := false
	var services []contractComposeService
	var current *contractComposeService
	var block []string
	serviceNamePattern := regexp.MustCompile(`^  ([A-Za-z0-9_-]+):\s*(?:#.*)?$`)

	flush := func() {
		if current == nil {
			return
		}
		fillComposeService(current, block)
		services = append(services, *current)
		current = nil
		block = nil
	}

	for _, line := range lines {
		if strings.TrimSpace(line) == "services:" {
			inServices = true
			continue
		}
		if !inServices {
			continue
		}
		if line != "" && !strings.HasPrefix(line, " ") {
			flush()
			break
		}
		if match := serviceNamePattern.FindStringSubmatch(line); match != nil {
			flush()
			current = &contractComposeService{Name: match[1]}
			continue
		}
		if current != nil {
			block = append(block, line)
		}
	}
	flush()
	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })
	return services, nil
}

func fillComposeService(service *contractComposeService, block []string) {
	envRefs := map[string]struct{}{}
	sections := map[string][]string{}
	currentSection := ""
	envRefPattern := regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)[^}]*}`)

	for _, line := range block {
		trimmed := strings.TrimSpace(line)
		for _, match := range envRefPattern.FindAllStringSubmatch(line, -1) {
			envRefs[match[1]] = struct{}{}
		}
		if strings.HasPrefix(trimmed, "image:") {
			service.Image = cleanYAMLScalar(strings.TrimSpace(strings.TrimPrefix(trimmed, "image:")))
		}
		if strings.HasPrefix(trimmed, "build:") {
			service.Build = cleanYAMLScalar(strings.TrimSpace(strings.TrimPrefix(trimmed, "build:")))
		}
		if strings.HasPrefix(trimmed, "restart:") {
			service.Restart = cleanYAMLScalar(strings.TrimSpace(strings.TrimPrefix(trimmed, "restart:")))
		}
		if strings.HasPrefix(trimmed, "healthcheck:") {
			service.Healthcheck = true
		}
		if strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(trimmed, "-") {
			currentSection = strings.TrimSuffix(trimmed, ":")
			continue
		}
		if strings.HasPrefix(trimmed, "- ") && currentSection != "" {
			sections[currentSection] = append(sections[currentSection], cleanYAMLScalar(strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))))
		}
	}

	service.Ports = sortedUnique(sections["ports"])
	service.Volumes = sortedUnique(sections["volumes"])
	service.DependsOn = sortedUnique(sections["depends_on"])
	service.Profiles = sortedUnique(sections["profiles"])
	service.Capabilities = sortedUnique(sections["cap_add"])
	service.EnvRefs = sortedKeys(envRefs)
}

func snapshotMigrations(dir string) ([]contractMigration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var migrations []contractMigration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		migrations = append(migrations, contractMigration{File: entry.Name(), SHA256: hex.EncodeToString(sum[:])})
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].File < migrations[j].File })
	return migrations, nil
}

func generatedArtifact(path, generator string) contractGeneratedArtifact {
	_, err := os.Stat(path)
	return contractGeneratedArtifact{
		Path:      filepath.ToSlash(path),
		Exists:    err == nil,
		Generator: generator,
	}
}

func releaseArtifactInputs(envVars []contractEnvVar) contractReleaseArtifacts {
	var tags []string
	var digests []string
	for _, envVar := range envVars {
		if strings.HasSuffix(envVar.Key, "_IMAGE_TAG") {
			tags = append(tags, envVar.Key)
		}
		if strings.HasSuffix(envVar.Key, "_IMAGE_DIGEST") {
			digests = append(digests, envVar.Key)
		}
	}
	sort.Strings(tags)
	sort.Strings(digests)
	return contractReleaseArtifacts{
		ImageTagKeys:    tags,
		ImageDigestKeys: digests,
		LauncherInputs: []string{
			"deploy/docker-compose.yml",
			"deploy/.env.example",
			"deploy/ome/Server.generated.xml",
			"deploy/srs/conf/srs.conf",
			"scripts/bitriver-live-wrapper.sh",
			"scripts/bitriver-live-wrapper.ps1",
		},
	}
}

func diffContractSnapshots(base, head contractSnapshot) contractDiffReport {
	changes := []contractDiffItem{}
	changes = append(changes, diffEnvContract(base.Env, head.Env)...)
	changes = append(changes, diffNamedComposeServices(base.ComposeServices, head.ComposeServices)...)
	changes = append(changes, diffMigrations(base.Migrations, head.Migrations)...)
	changes = append(changes, diffGeneratedArtifacts(base.Generated, head.Generated)...)
	changes = append(changes, diffHealthEndpoints(base.HealthEndpoints, head.HealthEndpoints)...)
	changes = append(changes, diffStringSet("release.imageTagKeys", base.ReleaseArtifacts.ImageTagKeys, head.ReleaseArtifacts.ImageTagKeys)...)
	changes = append(changes, diffStringSet("release.imageDigestKeys", base.ReleaseArtifacts.ImageDigestKeys, head.ReleaseArtifacts.ImageDigestKeys)...)
	changes = append(changes, diffStringSet("release.launcherInputs", base.ReleaseArtifacts.LauncherInputs, head.ReleaseArtifacts.LauncherInputs)...)
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Severity != changes[j].Severity {
			return changes[i].Severity < changes[j].Severity
		}
		if changes[i].Area != changes[j].Area {
			return changes[i].Area < changes[j].Area
		}
		return changes[i].Key < changes[j].Key
	})

	var summary contractDiffSummary
	for _, change := range changes {
		switch change.Severity {
		case "additive":
			summary.Additive++
		case "breaking":
			summary.Breaking++
		case "security":
			summary.Security++
		case "undocumented":
			summary.Undocumented++
		}
	}
	return contractDiffReport{Summary: summary, Changes: changes}
}

func diffEnvContract(base, head []contractEnvVar) []contractDiffItem {
	baseByKey := map[string]contractEnvVar{}
	headByKey := map[string]contractEnvVar{}
	for _, item := range base {
		baseByKey[item.Key] = item
	}
	for _, item := range head {
		headByKey[item.Key] = item
	}
	var changes []contractDiffItem
	for key, headItem := range headByKey {
		baseItem, ok := baseByKey[key]
		if !ok {
			severity := "additive"
			if !headItem.Documented {
				severity = "undocumented"
			}
			changes = append(changes, contractDiffItem{Severity: severity, Area: "env", Key: key, Detail: "env var added"})
			continue
		}
		if baseItem.Default != headItem.Default {
			severity := "breaking"
			if headItem.SecuritySensitive || baseItem.SecuritySensitive {
				severity = "security"
			}
			changes = append(changes, contractDiffItem{Severity: severity, Area: "env", Key: key, Detail: fmt.Sprintf("default changed from %q to %q", baseItem.Default, headItem.Default)})
		}
		if baseItem.Required != headItem.Required {
			changes = append(changes, contractDiffItem{Severity: "breaking", Area: "env", Key: key, Detail: fmt.Sprintf("requiredness changed from %t to %t", baseItem.Required, headItem.Required)})
		}
		if headItem.Documented != baseItem.Documented && !headItem.Documented {
			changes = append(changes, contractDiffItem{Severity: "undocumented", Area: "env", Key: key, Detail: "env var lost adjacent documentation comments"})
		}
	}
	for key := range baseByKey {
		if _, ok := headByKey[key]; !ok {
			changes = append(changes, contractDiffItem{Severity: "breaking", Area: "env", Key: key, Detail: "env var removed"})
		}
	}
	return changes
}

func diffNamedComposeServices(base, head []contractComposeService) []contractDiffItem {
	baseByKey := map[string]contractComposeService{}
	headByKey := map[string]contractComposeService{}
	for _, item := range base {
		baseByKey[item.Name] = item
	}
	for _, item := range head {
		headByKey[item.Name] = item
	}
	var changes []contractDiffItem
	for key, headItem := range headByKey {
		baseItem, ok := baseByKey[key]
		if !ok {
			changes = append(changes, contractDiffItem{Severity: "additive", Area: "compose.service", Key: key, Detail: "service added"})
			continue
		}
		if baseItem.Image != headItem.Image {
			changes = append(changes, contractDiffItem{Severity: "breaking", Area: "compose.service", Key: key, Detail: fmt.Sprintf("image changed from %q to %q", baseItem.Image, headItem.Image)})
		}
		if baseItem.Build != headItem.Build {
			changes = append(changes, contractDiffItem{Severity: "breaking", Area: "compose.service", Key: key, Detail: fmt.Sprintf("build changed from %q to %q", baseItem.Build, headItem.Build)})
		}
		if baseItem.Healthcheck != headItem.Healthcheck {
			changes = append(changes, contractDiffItem{Severity: "breaking", Area: "compose.service", Key: key, Detail: fmt.Sprintf("healthcheck changed from %t to %t", baseItem.Healthcheck, headItem.Healthcheck)})
		}
		changes = append(changes, diffStringSet("compose."+key+".ports", baseItem.Ports, headItem.Ports)...)
		changes = append(changes, diffStringSet("compose."+key+".volumes", baseItem.Volumes, headItem.Volumes)...)
		changes = append(changes, diffStringSet("compose."+key+".dependsOn", baseItem.DependsOn, headItem.DependsOn)...)
		changes = append(changes, diffStringSet("compose."+key+".envRefs", baseItem.EnvRefs, headItem.EnvRefs)...)
	}
	for key := range baseByKey {
		if _, ok := headByKey[key]; !ok {
			changes = append(changes, contractDiffItem{Severity: "breaking", Area: "compose.service", Key: key, Detail: "service removed"})
		}
	}
	return changes
}

func diffMigrations(base, head []contractMigration) []contractDiffItem {
	baseByKey := map[string]contractMigration{}
	headByKey := map[string]contractMigration{}
	for _, item := range base {
		baseByKey[item.File] = item
	}
	for _, item := range head {
		headByKey[item.File] = item
	}
	var changes []contractDiffItem
	for key, headItem := range headByKey {
		baseItem, ok := baseByKey[key]
		if !ok {
			changes = append(changes, contractDiffItem{Severity: "additive", Area: "migration", Key: key, Detail: "migration added"})
			continue
		}
		if baseItem.SHA256 != headItem.SHA256 {
			changes = append(changes, contractDiffItem{Severity: "breaking", Area: "migration", Key: key, Detail: "migration content changed"})
		}
	}
	for key := range baseByKey {
		if _, ok := headByKey[key]; !ok {
			changes = append(changes, contractDiffItem{Severity: "breaking", Area: "migration", Key: key, Detail: "migration removed"})
		}
	}
	return changes
}

func diffGeneratedArtifacts(base, head []contractGeneratedArtifact) []contractDiffItem {
	baseByKey := map[string]contractGeneratedArtifact{}
	headByKey := map[string]contractGeneratedArtifact{}
	for _, item := range base {
		baseByKey[item.Path] = item
	}
	for _, item := range head {
		headByKey[item.Path] = item
	}
	var changes []contractDiffItem
	for key, headItem := range headByKey {
		baseItem, ok := baseByKey[key]
		if !ok {
			changes = append(changes, contractDiffItem{Severity: "additive", Area: "generated", Key: key, Detail: "generated artifact added"})
			continue
		}
		if baseItem.Exists != headItem.Exists {
			changes = append(changes, contractDiffItem{Severity: "breaking", Area: "generated", Key: key, Detail: fmt.Sprintf("existence changed from %t to %t", baseItem.Exists, headItem.Exists)})
		}
	}
	for key := range baseByKey {
		if _, ok := headByKey[key]; !ok {
			changes = append(changes, contractDiffItem{Severity: "breaking", Area: "generated", Key: key, Detail: "generated artifact removed from snapshot"})
		}
	}
	return changes
}

func diffHealthEndpoints(base, head []contractHealthEndpoint) []contractDiffItem {
	baseKeys := map[string]struct{}{}
	headKeys := map[string]struct{}{}
	for _, item := range base {
		baseKeys[item.Method+" "+item.Path+" "+item.Name] = struct{}{}
	}
	for _, item := range head {
		headKeys[item.Method+" "+item.Path+" "+item.Name] = struct{}{}
	}
	return diffKeySets("healthEndpoint", baseKeys, headKeys)
}

func diffStringSet(area string, base, head []string) []contractDiffItem {
	baseKeys := map[string]struct{}{}
	headKeys := map[string]struct{}{}
	for _, item := range base {
		baseKeys[item] = struct{}{}
	}
	for _, item := range head {
		headKeys[item] = struct{}{}
	}
	return diffKeySets(area, baseKeys, headKeys)
}

func diffKeySets(area string, base, head map[string]struct{}) []contractDiffItem {
	var changes []contractDiffItem
	for key := range head {
		if _, ok := base[key]; !ok {
			changes = append(changes, contractDiffItem{Severity: "additive", Area: area, Key: key, Detail: "added"})
		}
	}
	for key := range base {
		if _, ok := head[key]; !ok {
			changes = append(changes, contractDiffItem{Severity: "breaking", Area: area, Key: key, Detail: "removed"})
		}
	}
	return changes
}

func writeJSONOutput(value any, output string) error {
	var writer io.Writer = os.Stdout
	var file *os.File
	if strings.TrimSpace(output) != "" {
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil && filepath.Dir(output) != "." {
			return err
		}
		created, err := os.Create(output)
		if err != nil {
			return err
		}
		defer created.Close()
		file = created
		writer = created
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return err
	}
	if file != nil {
		fmt.Fprintf(os.Stdout, "Wrote %s\n", output)
	}
	return nil
}

func readContractSnapshot(path string) (contractSnapshot, error) {
	file, err := os.Open(path)
	if err != nil {
		return contractSnapshot{}, err
	}
	defer file.Close()
	var snapshot contractSnapshot
	if err := json.NewDecoder(file).Decode(&snapshot); err != nil {
		return contractSnapshot{}, err
	}
	if snapshot.SchemaVersion != contractSnapshotSchemaVersion {
		return contractSnapshot{}, fmt.Errorf("unsupported snapshot schema %q", snapshot.SchemaVersion)
	}
	return snapshot, nil
}

func cleanYAMLScalar(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	return value
}

func securitySensitiveEnvKey(key string) bool {
	upper := strings.ToUpper(key)
	sensitiveParts := []string{"PASSWORD", "TOKEN", "SECRET", "PRIVATE", "SESSION", "CORS", "PUBLIC_URL", "VIEWER_URL", "API_BASE_URL", "RATE_LOGIN", "ALLOW_SELF_SIGNUP", "LIVE_MODE"}
	for _, part := range sensitiveParts {
		if strings.Contains(upper, part) {
			return true
		}
	}
	return false
}

func sortedUnique(values []string) []string {
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		seen[value] = struct{}{}
	}
	return sortedKeys(seen)
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
