package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const releaseCanarySchemaVersion = "bitriver.releaseCanary.v1"

type releaseCanaryConfig struct {
	BaseURL              string
	ArtifactDir          string
	LogsFile             string
	RollbackNotes        string
	RequireRollbackNotes bool
	Timeout              time.Duration
}

type releaseCanaryReport struct {
	SchemaVersion string               `json:"schemaVersion"`
	Status        string               `json:"status"`
	BaseURL       string               `json:"baseUrl"`
	ArtifactDir   string               `json:"artifactDir"`
	Checks        []releaseCanaryCheck `json:"checks"`
}

type releaseCanaryCheck struct {
	Name     string   `json:"name"`
	Status   string   `json:"status"`
	URL      string   `json:"url,omitempty"`
	Artifact string   `json:"artifact,omitempty"`
	Details  string   `json:"details"`
	Matches  []string `json:"matches,omitempty"`
}

type releaseCanaryEndpoint struct {
	Name           string
	Path           string
	Expect2xx      bool
	CheckJSONState bool
}

type releaseCanaryLogPattern struct {
	Name    string
	Pattern *regexp.Regexp
}

var releaseCanaryLogPatterns = []releaseCanaryLogPattern{
	{Name: "fatal startup error", Pattern: regexp.MustCompile(`(?i)\b(fatal|panic)\b`)},
	{Name: "migration failure", Pattern: regexp.MustCompile(`(?i)migration.*(fail|error)|failed.*migration`)},
	{Name: "connection refused loop", Pattern: regexp.MustCompile(`(?i)connection refused`)},
	{Name: "missing required config", Pattern: regexp.MustCompile(`(?i)(missing|required).*(env|config|secret|token)`)},
	{Name: "auth/session failure", Pattern: regexp.MustCompile(`(?i)(auth|session).*(fail|error)`)},
	{Name: "transcoder failure", Pattern: regexp.MustCompile(`(?i)transcoder.*(fail|error)|ffmpeg.*(fail|error)`)},
}

func runReleaseCanary(args []string) error {
	fs := flag.NewFlagSet("release canary", flag.ContinueOnError)
	baseURL := fs.String("base-url", "http://127.0.0.1:8080", "base URL for the deployed BitRiver API/control-plane")
	artifactDir := fs.String("artifact-dir", filepath.Join(repoRoot(), ".artifacts", "release-canary"), "directory for canary evidence artifacts")
	logsFile := fs.String("logs-file", "", "optional log file to scan for high-confidence release blockers")
	rollbackNotes := fs.String("rollback-notes", "", "optional rollback notes file to validate")
	requireRollbackNotes := fs.Bool("require-rollback-notes", false, "fail when rollback notes are missing or incomplete")
	timeout := fs.Duration("timeout", 5*time.Second, "per-endpoint HTTP timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg := releaseCanaryConfig{
		BaseURL:              strings.TrimRight(strings.TrimSpace(*baseURL), "/"),
		ArtifactDir:          *artifactDir,
		LogsFile:             strings.TrimSpace(*logsFile),
		RollbackNotes:        strings.TrimSpace(*rollbackNotes),
		RequireRollbackNotes: *requireRollbackNotes,
		Timeout:              *timeout,
	}
	if cfg.BaseURL == "" {
		return errors.New("release canary requires --base-url")
	}
	if _, err := url.ParseRequestURI(cfg.BaseURL); err != nil {
		return fmt.Errorf("invalid --base-url %q: %w", cfg.BaseURL, err)
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	if err := os.MkdirAll(cfg.ArtifactDir, 0o755); err != nil {
		return fmt.Errorf("create artifact dir: %w", err)
	}

	report := releaseCanaryReport{
		SchemaVersion: releaseCanarySchemaVersion,
		BaseURL:       cfg.BaseURL,
		ArtifactDir:   filepath.ToSlash(cfg.ArtifactDir),
	}

	executeReleaseCanary(cfg, &report)
	report.Status = releaseCanaryStatus(report.Checks)
	reportPath := filepath.Join(cfg.ArtifactDir, "canary-report.json")
	if err := writeJSONFile(report, reportPath); err != nil {
		return fmt.Errorf("write canary report: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Release canary %s. Artifacts: %s\n", report.Status, cfg.ArtifactDir)
	if report.Status == "failed" {
		return fmt.Errorf("release canary failed; inspect %s", filepath.ToSlash(reportPath))
	}
	return nil
}

func executeReleaseCanary(cfg releaseCanaryConfig, report *releaseCanaryReport) {
	report.Checks = append(report.Checks, captureReleaseCanaryVersion(cfg.ArtifactDir))
	for _, endpoint := range releaseCanaryEndpoints() {
		report.Checks = append(report.Checks, runReleaseCanaryEndpoint(cfg, endpoint))
	}
	report.Checks = append(report.Checks, scanReleaseCanaryLogs(cfg.LogsFile, cfg.ArtifactDir))
	report.Checks = append(report.Checks, validateReleaseCanaryRollback(cfg.RollbackNotes, cfg.RequireRollbackNotes, cfg.ArtifactDir))
}

func releaseCanaryEndpoints() []releaseCanaryEndpoint {
	return []releaseCanaryEndpoint{
		{Name: "API readiness", Path: "/readyz", Expect2xx: true, CheckJSONState: true},
		{Name: "API health", Path: "/healthz", Expect2xx: true, CheckJSONState: true},
		{Name: "Operator status", Path: "/api/status", Expect2xx: true, CheckJSONState: true},
		{Name: "Viewer route", Path: "/viewer", Expect2xx: false, CheckJSONState: false},
	}
}

func captureReleaseCanaryVersion(artifactDir string) releaseCanaryCheck {
	artifact := filepath.Join(artifactDir, "version.txt")
	err := captureStdoutToFile(artifact, func() error {
		printVersionInfo(os.Stdout)
		return nil
	})
	if err != nil {
		return releaseCanaryCheck{Name: "Version metadata", Status: "failed", Artifact: filepath.ToSlash(artifact), Details: err.Error()}
	}
	return releaseCanaryCheck{Name: "Version metadata", Status: "passed", Artifact: filepath.ToSlash(artifact), Details: "captured CLI build metadata for release evidence."}
}

func runReleaseCanaryEndpoint(cfg releaseCanaryConfig, endpoint releaseCanaryEndpoint) releaseCanaryCheck {
	target := cfg.BaseURL + endpoint.Path
	artifact := filepath.Join(cfg.ArtifactDir, canaryArtifactName(endpoint.Name)+".json")
	check := releaseCanaryCheck{Name: endpoint.Name, URL: target, Artifact: filepath.ToSlash(artifact)}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		check.Status = "failed"
		check.Details = fmt.Sprintf("build request: %v", err)
		return check
	}
	client := &http.Client{Timeout: cfg.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		check.Status = "failed"
		check.Details = fmt.Sprintf("request failed: %v", err)
		return check
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		check.Status = "failed"
		check.Details = fmt.Sprintf("read response body: %v", err)
		return check
	}
	if err := writeCanaryResponseArtifact(artifact, resp.StatusCode, body); err != nil {
		check.Status = "failed"
		check.Details = fmt.Sprintf("write response artifact: %v", err)
		return check
	}

	if endpoint.Expect2xx && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		check.Status = "failed"
		check.Details = fmt.Sprintf("HTTP %d returned; expected 2xx.", resp.StatusCode)
		return check
	}
	if !endpoint.Expect2xx && resp.StatusCode >= 500 {
		check.Status = "failed"
		check.Details = fmt.Sprintf("HTTP %d returned; expected less than 500.", resp.StatusCode)
		return check
	}
	if endpoint.CheckJSONState {
		if unhealthy := unhealthyJSONStatuses(body); len(unhealthy) > 0 {
			check.Status = "failed"
			check.Details = "response contains unhealthy status fields."
			check.Matches = unhealthy
			return check
		}
	}

	check.Status = "passed"
	check.Details = fmt.Sprintf("HTTP %d returned and response passed canary checks.", resp.StatusCode)
	return check
}

func writeCanaryResponseArtifact(path string, statusCode int, body []byte) error {
	payload := map[string]any{
		"statusCode": statusCode,
		"body":       redactCanaryResponseBody(body),
	}
	return writeJSONFile(payload, path)
}

func redactCanaryResponseBody(body []byte) any {
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return strings.TrimSpace(string(body))
	}
	return redactCanaryJSON(value)
}

func redactCanaryJSON(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		clean := map[string]any{}
		for key, nested := range typed {
			if securitySensitiveEnvKey(key) {
				clean[key] = "[redacted]"
				continue
			}
			clean[key] = redactCanaryJSON(nested)
		}
		return clean
	case []any:
		clean := make([]any, 0, len(typed))
		for _, nested := range typed {
			clean = append(clean, redactCanaryJSON(nested))
		}
		return clean
	default:
		return typed
	}
}

func unhealthyJSONStatuses(body []byte) []string {
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return nil
	}
	var matches []string
	walkCanaryJSONStatuses(value, "", &matches)
	return matches
}

func walkCanaryJSONStatuses(value any, path string, matches *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			nextPath := key
			if path != "" {
				nextPath = path + "." + key
			}
			if strings.EqualFold(key, "status") || strings.EqualFold(key, "state") {
				if text, ok := nested.(string); ok && canaryUnhealthyStatus(text) {
					*matches = append(*matches, fmt.Sprintf("%s=%s", nextPath, text))
				}
			}
			walkCanaryJSONStatuses(nested, nextPath, matches)
		}
	case []any:
		for idx, nested := range typed {
			walkCanaryJSONStatuses(nested, fmt.Sprintf("%s[%d]", path, idx), matches)
		}
	}
}

func canaryUnhealthyStatus(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "error", "failed", "failing", "degraded", "unhealthy", "down":
		return true
	default:
		return false
	}
}

func scanReleaseCanaryLogs(logsFile, artifactDir string) releaseCanaryCheck {
	artifact := filepath.Join(artifactDir, "log-scan.txt")
	check := releaseCanaryCheck{Name: "Log scan", Artifact: filepath.ToSlash(artifact)}
	if strings.TrimSpace(logsFile) == "" {
		check.Status = "warning"
		check.Details = "skipped: pass --logs-file with recent compose/application logs to scan high-confidence failure patterns."
		_ = os.WriteFile(artifact, []byte(check.Details+"\n"), 0o644)
		return check
	}

	matches, err := scanCanaryLogFile(logsFile)
	if err != nil {
		check.Status = "failed"
		check.Details = err.Error()
		return check
	}
	if err := writeCanaryLogScanArtifact(artifact, matches); err != nil {
		check.Status = "failed"
		check.Details = fmt.Sprintf("write log scan artifact: %v", err)
		return check
	}
	if len(matches) > 0 {
		check.Status = "failed"
		check.Details = fmt.Sprintf("found %d high-confidence log problem(s).", len(matches))
		check.Matches = matches
		return check
	}
	check.Status = "passed"
	check.Details = "no high-confidence fatal/error log patterns found."
	return check
}

func scanCanaryLogFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open logs file: %w", err)
	}
	defer file.Close()

	var matches []string
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		for _, pattern := range releaseCanaryLogPatterns {
			if pattern.Pattern.MatchString(line) {
				matches = append(matches, fmt.Sprintf("%d:%s: %s", lineNumber, pattern.Name, strings.TrimSpace(line)))
				break
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return matches, nil
}

func writeCanaryLogScanArtifact(path string, matches []string) error {
	var buf bytes.Buffer
	if len(matches) == 0 {
		buf.WriteString("No high-confidence log patterns found.\n")
	} else {
		buf.WriteString("High-confidence log patterns found:\n")
		for _, match := range matches {
			buf.WriteString("- ")
			buf.WriteString(match)
			buf.WriteString("\n")
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func validateReleaseCanaryRollback(notesFile string, required bool, artifactDir string) releaseCanaryCheck {
	artifact := filepath.Join(artifactDir, "rollback-readiness.txt")
	check := releaseCanaryCheck{Name: "Rollback readiness", Artifact: filepath.ToSlash(artifact)}
	if strings.TrimSpace(notesFile) == "" {
		check.Status = "warning"
		check.Details = "skipped: pass --rollback-notes with previous-version, data/migration, env/config, and artifact rollback notes."
		if required {
			check.Status = "failed"
			check.Details = "rollback notes are required but --rollback-notes was not provided."
		}
		_ = os.WriteFile(artifact, []byte(check.Details+"\n"), 0o644)
		return check
	}

	data, err := os.ReadFile(notesFile)
	if err != nil {
		check.Status = "failed"
		check.Details = fmt.Sprintf("read rollback notes: %v", err)
		return check
	}
	missing := missingRollbackSections(string(data))
	if err := writeCanaryRollbackArtifact(artifact, missing); err != nil {
		check.Status = "failed"
		check.Details = fmt.Sprintf("write rollback artifact: %v", err)
		return check
	}
	if len(missing) > 0 {
		check.Status = "failed"
		check.Details = "rollback notes are missing required coverage."
		check.Matches = missing
		return check
	}
	check.Status = "passed"
	check.Details = "rollback notes cover previous version, data/migrations, env/config, and artifact rollback path."
	return check
}

func missingRollbackSections(notes string) []string {
	lower := strings.ToLower(notes)
	requirements := map[string]*regexp.Regexp{
		"previous version/tag": regexp.MustCompile(`previous (version|tag)|rollback target|from version|current version`),
		"data/migration note":  regexp.MustCompile(`migration|database|schema|data|irreversible|no rollback`),
		"env/config note":      regexp.MustCompile(`env|config|configuration|secret`),
		"artifact path":        regexp.MustCompile(`artifact|image|tag|digest|binary|launcher`),
	}
	var missing []string
	for name, pattern := range requirements {
		if !pattern.MatchString(lower) {
			missing = append(missing, name)
		}
	}
	return missing
}

func writeCanaryRollbackArtifact(path string, missing []string) error {
	var buf bytes.Buffer
	if len(missing) == 0 {
		buf.WriteString("Rollback readiness checks passed.\n")
	} else {
		buf.WriteString("Rollback readiness missing:\n")
		for _, item := range missing {
			buf.WriteString("- ")
			buf.WriteString(item)
			buf.WriteString("\n")
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func releaseCanaryStatus(checks []releaseCanaryCheck) string {
	status := "passed"
	for _, check := range checks {
		switch check.Status {
		case "failed":
			return "failed"
		case "warning", "skipped":
			if status != "failed" {
				status = "warning"
			}
		}
	}
	return status
}

func canaryArtifactName(name string) string {
	name = strings.ToLower(name)
	replacer := strings.NewReplacer(" ", "-", "/", "-", "_", "-")
	return replacer.Replace(name)
}
