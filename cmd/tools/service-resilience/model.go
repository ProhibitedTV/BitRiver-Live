package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

const reportSchema = "bitriver.service-resilience/v1"

var requiredScenarioNames = []string{
	"api",
	"postgres",
	"redis",
	"srs_path",
	"ovenmediaengine",
	"transcoder",
	"viewer",
}

type sourceEvidence struct {
	Commit        string `json:"commit"`
	Qualification string `json:"qualification"`
}

type isolationEvidence struct {
	CleanTrackedTree             bool `json:"cleanTrackedTree"`
	PrivateEnvironmentCopy       bool `json:"privateEnvironmentCopy"`
	IsolatedRuntimeStorage       bool `json:"isolatedRuntimeStorage"`
	OperatorEnvironmentUnchanged bool `json:"operatorEnvironmentUnchanged"`
	OperatorOMEConfigUnchanged   bool `json:"operatorOmeConfigUnchanged"`
	TeardownComplete             bool `json:"teardownComplete"`
}

type durableEvidence struct {
	SessionPreserved bool `json:"sessionPreserved"`
	ChannelPreserved bool `json:"channelPreserved"`
}

type scenarioEvidence struct {
	Name                string          `json:"name"`
	Targets             []string        `json:"targets"`
	ExpectedSignal      string          `json:"expectedSignal"`
	DegradationObserved bool            `json:"degradationObserved"`
	DegradationSeconds  float64         `json:"degradationSeconds"`
	RecoverySeconds     float64         `json:"recoverySeconds"`
	DurableState        durableEvidence `json:"durableState"`
	RestartCountsStable bool            `json:"restartCountsStable"`
	Result              string          `json:"result"`
}

type resilienceReport struct {
	Schema              string             `json:"schema"`
	GeneratedAt         string             `json:"generatedAt"`
	Source              sourceEvidence     `json:"source"`
	Isolation           isolationEvidence  `json:"isolation"`
	Scenarios           []scenarioEvidence `json:"scenarios"`
	OverallResult       string             `json:"overallResult"`
	RemainingAcceptance []string           `json:"remainingAcceptance"`
}

func newReport(commit string) resilienceReport {
	return resilienceReport{
		Schema:      reportSchema,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Source: sourceEvidence{
			Commit:        commit,
			Qualification: "local-build",
		},
		Isolation: isolationEvidence{
			CleanTrackedTree:       true,
			PrivateEnvironmentCopy: true,
			IsolatedRuntimeStorage: true,
		},
		OverallResult: "passed",
		RemainingAcceptance: []string{
			"exact-candidate clean-host execution",
			"physical Docker-daemon and host reboot",
			"network partition and resource-pressure injection",
			"credential and control-plane failure injection",
			"active-stream continuity and alert delivery",
		},
	}
}

func (r resilienceReport) validate(secretSentinels []string) error {
	if r.Schema != reportSchema {
		return fmt.Errorf("unexpected report schema %q", r.Schema)
	}
	if r.Source.Commit == "" || r.Source.Qualification != "local-build" {
		return fmt.Errorf("source identity is incomplete")
	}
	if r.OverallResult != "passed" {
		return fmt.Errorf("overall result is %q", r.OverallResult)
	}
	if !r.Isolation.CleanTrackedTree || !r.Isolation.PrivateEnvironmentCopy ||
		!r.Isolation.IsolatedRuntimeStorage || !r.Isolation.OperatorEnvironmentUnchanged ||
		!r.Isolation.OperatorOMEConfigUnchanged || !r.Isolation.TeardownComplete {
		return fmt.Errorf("isolation evidence is incomplete")
	}

	seen := make(map[string]bool, len(r.Scenarios))
	for _, scenario := range r.Scenarios {
		if scenario.Name == "" || seen[scenario.Name] {
			return fmt.Errorf("scenario name %q is missing or duplicated", scenario.Name)
		}
		seen[scenario.Name] = true
		if len(scenario.Targets) == 0 || scenario.ExpectedSignal == "" {
			return fmt.Errorf("scenario %s lacks target or signal identity", scenario.Name)
		}
		if !scenario.DegradationObserved || scenario.DegradationSeconds < 0 || scenario.RecoverySeconds < 0 {
			return fmt.Errorf("scenario %s lacks bounded degradation/recovery evidence", scenario.Name)
		}
		if !scenario.DurableState.SessionPreserved || !scenario.DurableState.ChannelPreserved {
			return fmt.Errorf("scenario %s lost durable state", scenario.Name)
		}
		if !scenario.RestartCountsStable || scenario.Result != "passed" {
			return fmt.Errorf("scenario %s did not recover stably", scenario.Name)
		}
	}
	for _, name := range requiredScenarioNames {
		if !seen[name] {
			return fmt.Errorf("required scenario %s is missing", name)
		}
	}
	if len(r.RemainingAcceptance) == 0 {
		return fmt.Errorf("remaining acceptance is missing")
	}

	payload, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshal report for secret scan: %w", err)
	}
	text := string(payload)
	for _, secret := range secretSentinels {
		secret = strings.TrimSpace(secret)
		if len(secret) >= 8 && strings.Contains(text, secret) {
			return fmt.Errorf("report contains a private sentinel")
		}
	}
	return nil
}

func writeReport(path string, report resilienceReport, secretSentinels []string) error {
	if err := report.validate(secretSentinels); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	payload = append(payload, '\n')
	if err := os.MkdirAll(filepathDir(path), 0o755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, payload, 0o600); err != nil {
		return fmt.Errorf("write temporary report: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("publish report: %w", err)
	}
	return nil
}

func filepathDir(path string) string {
	index := strings.LastIndexAny(path, `/\\`)
	if index < 0 {
		return "."
	}
	if index == 0 {
		return path[:1]
	}
	return path[:index]
}

func restartCountsStable(first, second map[string]int) bool {
	if len(first) != len(second) {
		return false
	}
	keys := make([]string, 0, len(first))
	for key := range first {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if second[key] != first[key] {
			return false
		}
	}
	return true
}
