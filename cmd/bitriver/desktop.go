package main

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"bitriver-live/internal/executil"
)

//go:embed desktop.tmpl.html
var desktopTemplateFS embed.FS

type composeServiceStatus struct {
	Service string `json:"service"`
	State   string `json:"state"`
	Health  string `json:"health"`
}

type desktopState struct {
	composeFile string
	envFile     string
	dashboard   string
	logs        []string
	logErr      string
	services    []composeServiceStatus
	statusErr   string
	mu          sync.Mutex
}

var composeStatusRunner = readComposeStatus
var composeCommandOutput = runComposeCommandOutput

func runDesktop(args []string) error {
	fs := flag.NewFlagSet("desktop", flag.ContinueOnError)
	composeFile := fs.String("compose-file", defaultComposeFile(), "compose file to use")
	envFile := fs.String("env-file", defaultEnvFile(), "environment file path")
	dashboard := fs.String("dashboard", "http://localhost:3000/", "health dashboard url")
	listen := fs.String("listen", "127.0.0.1:7845", "host:port to bind the desktop UI")
	openBrowser := fs.Bool("open-browser", true, "open the control panel in the default browser")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if _, err := os.Stat(*composeFile); err != nil {
		return fmt.Errorf("compose file missing: %w", err)
	}
	if _, err := os.Stat(*envFile); err != nil {
		return fmt.Errorf("env file missing: %w", err)
	}
	if _, err := executil.LookPath("docker"); err != nil {
		return err
	}

	state := &desktopState{
		composeFile: *composeFile,
		envFile:     *envFile,
		dashboard:   *dashboard,
		logs:        make([]string, 0, 200),
		services:    []composeServiceStatus{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go state.pollStatuses(ctx, 3*time.Second)
	go state.tailLogs(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/", state.servePage)
	mux.HandleFunc("/api/status", state.serveStatus)
	mux.HandleFunc("/api/action", state.serveAction)

	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", *listen, err)
	}

	if *openBrowser {
		go func(addr string) {
			time.Sleep(250 * time.Millisecond)
			_ = openURL("http://" + addr + "/")
		}((*listen))
	}

	fmt.Printf("BitRiver Live control panel running on http://%s/\n", ln.Addr().String())
	server := &http.Server{Handler: mux}
	return server.Serve(ln)
}

func (s *desktopState) servePage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(desktopTemplateFS, "desktop.tmpl.html")
	if err != nil {
		http.Error(w, fmt.Sprintf("template load error: %v", err), http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, map[string]string{"Dashboard": s.dashboard}); err != nil {
		http.Error(w, fmt.Sprintf("render error: %v", err), http.StatusInternalServerError)
	}
}

func (s *desktopState) serveStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	response := map[string]any{
		"services":  s.services,
		"statusErr": s.statusErr,
		"logs":      s.logs,
		"logErr":    s.logErr,
		"dashboard": s.dashboard,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (s *desktopState) serveAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, fmt.Sprintf("decode: %v", err), http.StatusBadRequest)
		return
	}

	args, err := actionArgs(payload.Action)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	output, runErr := composeCommandOutput(s.composeFile, s.envFile, args...)
	s.mu.Lock()
	if output != "" {
		s.logs = appendLogLines(s.logs, strings.Split(strings.TrimRight(output, "\n"), "\n"))
	}
	if runErr != nil {
		s.logErr = runErr.Error()
	}
	s.mu.Unlock()

	if runErr != nil {
		http.Error(w, runErr.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *desktopState) pollStatuses(ctx context.Context, interval time.Duration) {
	for {
		statuses, err := composeStatusRunner(s.composeFile, s.envFile)
		s.mu.Lock()
		s.services = statuses
		if err != nil {
			s.statusErr = err.Error()
		} else {
			s.statusErr = ""
		}
		s.mu.Unlock()

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

func (s *desktopState) tailLogs(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "docker", "compose", "--file", s.composeFile, "--env-file", s.envFile, "logs", "--tail", "200", "--follow")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.mu.Lock()
		s.logErr = err.Error()
		s.mu.Unlock()
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.mu.Lock()
		s.logErr = err.Error()
		s.mu.Unlock()
		return
	}

	if err := cmd.Start(); err != nil {
		s.mu.Lock()
		s.logErr = err.Error()
		s.mu.Unlock()
		return
	}

	go s.scanLog(stdout)
	go s.scanLog(stderr)

	_ = cmd.Wait()
}

func (s *desktopState) scanLog(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		s.mu.Lock()
		s.logs = appendLogLines(s.logs, []string{scanner.Text()})
		s.mu.Unlock()
	}
}

func appendLogLines(existing []string, newLines []string) []string {
	combined := append(existing, newLines...)
	const limit = 400
	if len(combined) > limit {
		combined = combined[len(combined)-limit:]
	}
	return combined
}

func actionArgs(action string) ([]string, error) {
	switch action {
	case "start":
		return []string{"up", "-d", "--build"}, nil
	case "stop":
		return []string{"stop"}, nil
	case "restart":
		return []string{"restart"}, nil
	case "logs":
		return []string{"logs", "--tail", "200"}, nil
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

func readComposeStatus(composeFile, envFile string) ([]composeServiceStatus, error) {
	output, err := composeCommandOutput(composeFile, envFile, "ps", "--format", "{{.Service}}|{{.State}}|{{.Health}}")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	statuses := make([]composeServiceStatus, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}
		statuses = append(statuses, composeServiceStatus{
			Service: parts[0],
			State:   parts[1],
			Health:  parts[2],
		})
	}
	return statuses, nil
}

func runComposeCommandOutput(composeFile, envFile string, args ...string) (string, error) {
	composeArgs := []string{"compose", "--file", composeFile, "--env-file", envFile}
	composeArgs = append(composeArgs, args...)
	cmd := exec.Command("docker", composeArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed != "" {
			return string(output), fmt.Errorf("docker compose %s: %s", strings.Join(args, " "), trimmed)
		}
		return string(output), fmt.Errorf("docker compose %s: %w", strings.Join(args, " "), err)
	}
	return string(output), nil
}

func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
