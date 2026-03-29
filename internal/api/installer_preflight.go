package api

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"bitriver-live/internal/executil"
)

const (
	installerPreflightDefaultInstallDir  = "/opt/bitriver-live"
	installerPreflightDefaultDataDir     = "/var/lib/bitriver-live"
	installerPreflightDefaultServiceUser = "bitriver"
	installerPreflightDefaultAddr        = ":8080"
)

// InstallerPreflightRequest captures the installer draft fields the server
// needs in order to inspect host readiness without mutating the system.
type InstallerPreflightRequest struct {
	InstallDir      string `json:"installDir"`
	DataDir         string `json:"dataDir"`
	ServiceUser     string `json:"serviceUser"`
	Addr            string `json:"addr"`
	TLSCert         string `json:"tlsCert"`
	TLSKey          string `json:"tlsKey"`
	StorageDriver   string `json:"storageDriver"`
	PostgresDsn     string `json:"postgresDsn"`
	SessionStore    string `json:"sessionStore"`
	SessionStoreDsn string `json:"sessionStoreDsn"`
	RedisAddr       string `json:"redisAddr"`
}

// InstallerPreflightCheck reports one actionable host-readiness result for the
// installer System Check step.
type InstallerPreflightCheck struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Status           string   `json:"status"`
	Summary          string   `json:"summary"`
	Action           string   `json:"action,omitempty"`
	TechnicalDetails []string `json:"technicalDetails,omitempty"`
}

// InstallerPreflightResponse captures the full dry-run result returned to the
// installer UI.
type InstallerPreflightResponse struct {
	Status    string                    `json:"status"`
	CheckedAt string                    `json:"checkedAt"`
	Checks    []InstallerPreflightCheck `json:"checks"`
}

// InstallerPreflightChecker inspects host readiness for the browser installer.
type InstallerPreflightChecker interface {
	Check(context.Context, InstallerPreflightRequest) (InstallerPreflightResponse, error)
}

type hostInstallerPreflightChecker struct {
	lookPath    func(string) (string, error)
	stat        func(string) (fs.FileInfo, error)
	readFile    func(string) ([]byte, error)
	dialAddress func(context.Context, string, string) error
	goos        string
	now         func() time.Time
}

func newHostInstallerPreflightChecker() InstallerPreflightChecker {
	dialer := &net.Dialer{Timeout: 750 * time.Millisecond}
	return hostInstallerPreflightChecker{
		lookPath: executil.LookPath,
		stat:     os.Stat,
		readFile: os.ReadFile,
		dialAddress: func(ctx context.Context, network, address string) error {
			dialCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
			defer cancel()
			conn, err := dialer.DialContext(dialCtx, network, address)
			if err != nil {
				return err
			}
			return conn.Close()
		},
		goos: runtime.GOOS,
		now:  time.Now,
	}
}

// InstallerPreflight performs a read-only host inspection for the browser
// installer and returns actionable pass/warn/fail checks.
func (h *Handler) InstallerPreflight(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteMethodNotAllowed(w, r, http.MethodPost)
		return
	}

	if _, ok := h.requireRole(w, r, roleAdmin); !ok {
		return
	}

	var req InstallerPreflightRequest
	if !DecodeAllowUnknownAndValidate(w, r, &req) {
		return
	}

	resp, err := h.installerPreflightChecker().Check(r.Context(), req)
	if err != nil {
		WriteRequestError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, resp)
}

func (c hostInstallerPreflightChecker) Check(ctx context.Context, req InstallerPreflightRequest) (InstallerPreflightResponse, error) {
	data := normalizeInstallerPreflightRequest(req)
	checks := []InstallerPreflightCheck{
		c.checkSupportedTarget(),
		c.checkServiceManager(),
		c.checkInstallerTools(),
		c.checkFilesystemPaths(data),
		c.checkPortReadiness(data),
		c.checkExternalServices(ctx, data),
		c.checkTLSAssets(data),
	}

	return InstallerPreflightResponse{
		Status:    installerPreflightOverallStatus(checks),
		CheckedAt: c.now().UTC().Format(time.RFC3339),
		Checks:    checks,
	}, nil
}

func normalizeInstallerPreflightRequest(req InstallerPreflightRequest) InstallerPreflightRequest {
	normalized := InstallerPreflightRequest{
		InstallDir:      normalizeInstallerPreflightPath(req.InstallDir),
		DataDir:         normalizeInstallerPreflightPath(req.DataDir),
		ServiceUser:     strings.TrimSpace(req.ServiceUser),
		Addr:            strings.TrimSpace(req.Addr),
		TLSCert:         normalizeInstallerPreflightPath(req.TLSCert),
		TLSKey:          normalizeInstallerPreflightPath(req.TLSKey),
		StorageDriver:   strings.ToLower(strings.TrimSpace(req.StorageDriver)),
		PostgresDsn:     strings.TrimSpace(req.PostgresDsn),
		SessionStore:    strings.ToLower(strings.TrimSpace(req.SessionStore)),
		SessionStoreDsn: strings.TrimSpace(req.SessionStoreDsn),
		RedisAddr:       strings.TrimSpace(req.RedisAddr),
	}

	if normalized.InstallDir == "" {
		normalized.InstallDir = installerPreflightDefaultInstallDir
	}
	if normalized.DataDir == "" {
		normalized.DataDir = installerPreflightDefaultDataDir
	}
	if normalized.ServiceUser == "" {
		normalized.ServiceUser = installerPreflightDefaultServiceUser
	}
	if normalized.Addr == "" {
		normalized.Addr = installerPreflightDefaultAddr
	}
	if normalized.StorageDriver != "postgres" {
		normalized.StorageDriver = "json"
	}
	if normalized.SessionStore != "postgres" {
		normalized.SessionStore = "memory"
		normalized.SessionStoreDsn = ""
	}

	return normalized
}

func normalizeInstallerPreflightPath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "/" {
		return trimmed
	}
	return strings.TrimRight(trimmed, "/")
}

func installerPreflightOverallStatus(checks []InstallerPreflightCheck) string {
	status := "pass"
	for _, check := range checks {
		switch check.Status {
		case "fail":
			return "fail"
		case "warning":
			status = "warning"
		}
	}
	return status
}

func installerPreflightWorstStatus(values ...string) string {
	status := "pass"
	for _, value := range values {
		switch value {
		case "fail":
			return "fail"
		case "warning":
			status = "warning"
		}
	}
	return status
}

func (c hostInstallerPreflightChecker) checkSupportedTarget() InstallerPreflightCheck {
	details := []string{fmt.Sprintf("goos=%s", c.goos)}
	if c.goos != "linux" {
		return InstallerPreflightCheck{
			ID:               "supported-target",
			Title:            "Supported target",
			Status:           "fail",
			Summary:          fmt.Sprintf("This host reports %s. The guided install targets Ubuntu 22.04+ with systemd.", c.goos),
			Action:           "Run the installer on an Ubuntu 22.04+ server, or use the CLI installers for other platforms.",
			TechnicalDetails: details,
		}
	}

	release, err := c.readOSRelease()
	if err != nil {
		details = append(details, fmt.Sprintf("os-release unavailable: %v", err))
		return InstallerPreflightCheck{
			ID:               "supported-target",
			Title:            "Supported target",
			Status:           "warning",
			Summary:          "Linux host detected, but Ubuntu version could not be confirmed from /etc/os-release.",
			Action:           "Use Ubuntu 22.04+ with systemd for the supported path, or continue only if you know this host matches that shape.",
			TechnicalDetails: details,
		}
	}

	id := strings.ToLower(strings.TrimSpace(release["ID"]))
	versionID := strings.TrimSpace(release["VERSION_ID"])
	if id != "" {
		details = append(details, fmt.Sprintf("id=%s", id))
	}
	if versionID != "" {
		details = append(details, fmt.Sprintf("version=%s", versionID))
	}

	switch {
	case id == "ubuntu" && installerUbuntuVersionAtLeast(versionID, 22, 4):
		return InstallerPreflightCheck{
			ID:               "supported-target",
			Title:            "Supported target",
			Status:           "pass",
			Summary:          fmt.Sprintf("Ubuntu %s detected on this host, which matches the guided installer target.", versionID),
			Action:           "You can keep using the guided path for this machine.",
			TechnicalDetails: details,
		}
	case id == "ubuntu":
		return InstallerPreflightCheck{
			ID:               "supported-target",
			Title:            "Supported target",
			Status:           "fail",
			Summary:          fmt.Sprintf("Ubuntu %s was detected, but the guided path expects Ubuntu 22.04 or newer.", versionID),
			Action:           "Upgrade the host to Ubuntu 22.04+ before relying on this install path.",
			TechnicalDetails: details,
		}
	default:
		return InstallerPreflightCheck{
			ID:               "supported-target",
			Title:            "Supported target",
			Status:           "warning",
			Summary:          fmt.Sprintf("This Linux host reports %q. The guided path is documented for Ubuntu 22.04+ with systemd.", id),
			Action:           "Continue only if you know this distro provides the same systemd/service tooling the Ubuntu helper expects.",
			TechnicalDetails: details,
		}
	}
}

func (c hostInstallerPreflightChecker) checkServiceManager() InstallerPreflightCheck {
	details := []string{}
	systemctlPath, err := c.lookPath("systemctl")
	if err != nil {
		details = append(details, err.Error())
		return InstallerPreflightCheck{
			ID:               "service-manager",
			Title:            "Service manager",
			Status:           "fail",
			Summary:          "systemctl is not available on this host, so the Ubuntu helper cannot register the BitRiver Live service.",
			Action:           "Run the install on a systemd-based host, or use a different installer path for this platform.",
			TechnicalDetails: details,
		}
	}
	details = append(details, fmt.Sprintf("systemctl=%s", systemctlPath))

	info, err := c.stat("/run/systemd/system")
	switch {
	case err == nil && info.IsDir():
		details = append(details, "/run/systemd/system present")
		return InstallerPreflightCheck{
			ID:               "service-manager",
			Title:            "Service manager",
			Status:           "pass",
			Summary:          "systemd is active on this host and ready for the service handoff.",
			Action:           "The guided installer can use systemd to register and start BitRiver Live.",
			TechnicalDetails: details,
		}
	case err == nil:
		details = append(details, "/run/systemd/system is not a directory")
		return InstallerPreflightCheck{
			ID:               "service-manager",
			Title:            "Service manager",
			Status:           "fail",
			Summary:          "systemctl is present, but systemd does not look active on this host.",
			Action:           "Run the installer on a host booted with systemd, or switch to a different service-management path.",
			TechnicalDetails: details,
		}
	case errors.Is(err, fs.ErrNotExist):
		details = append(details, "/run/systemd/system missing")
		return InstallerPreflightCheck{
			ID:               "service-manager",
			Title:            "Service manager",
			Status:           "fail",
			Summary:          "systemctl is installed, but this host does not appear to be running systemd right now.",
			Action:           "Use a systemd-based Ubuntu host for the guided install path.",
			TechnicalDetails: details,
		}
	default:
		details = append(details, fmt.Sprintf("systemd probe failed: %v", err))
		return InstallerPreflightCheck{
			ID:               "service-manager",
			Title:            "Service manager",
			Status:           "warning",
			Summary:          "systemctl is installed, but the host could not fully confirm whether systemd is active.",
			Action:           "Verify that systemd is the active service manager before running the handoff command.",
			TechnicalDetails: details,
		}
	}
}

func (c hostInstallerPreflightChecker) checkInstallerTools() InstallerPreflightCheck {
	required := []string{"bash", "curl", "sudo"}
	details := []string{}
	missing := []string{}

	for _, binary := range required {
		path, err := c.lookPath(binary)
		if err != nil {
			missing = append(missing, binary)
			details = append(details, err.Error())
			continue
		}
		details = append(details, fmt.Sprintf("%s=%s", binary, path))
	}

	if len(missing) > 0 {
		return InstallerPreflightCheck{
			ID:               "installer-tools",
			Title:            "Installer tools",
			Status:           "fail",
			Summary:          fmt.Sprintf("This host is missing required installer tools: %s.", strings.Join(missing, ", ")),
			Action:           "Install the missing packages before running the handoff command.",
			TechnicalDetails: details,
		}
	}

	return InstallerPreflightCheck{
		ID:               "installer-tools",
		Title:            "Installer tools",
		Status:           "pass",
		Summary:          "bash, curl, and sudo are available on this host.",
		Action:           "The generated handoff command can use the normal Ubuntu helper flow here.",
		TechnicalDetails: details,
	}
}

func (c hostInstallerPreflightChecker) checkFilesystemPaths(req InstallerPreflightRequest) InstallerPreflightCheck {
	installStatus, installDetail := c.inspectInstallerPath(req.InstallDir)
	dataStatus, dataDetail := c.inspectInstallerPath(req.DataDir)
	status := installerPreflightWorstStatus(installStatus, dataStatus)

	summary := "Install and data paths look ready for the guided path."
	action := "The installer can create missing directories with sudo if needed."
	switch status {
	case "fail":
		summary = "One or more configured paths need attention on this host."
		action = "Use absolute Linux directory paths and make sure they do not collide with existing files."
	case "warning":
		summary = "The host could not fully inspect every configured path."
		action = "Double-check the target directories before you run the handoff command."
	}

	return InstallerPreflightCheck{
		ID:      "filesystem-paths",
		Title:   "Filesystem paths",
		Status:  status,
		Summary: summary,
		Action:  action,
		TechnicalDetails: []string{
			fmt.Sprintf("service_user=%s", req.ServiceUser),
			fmt.Sprintf("install_dir=%s", installDetail),
			fmt.Sprintf("data_dir=%s", dataDetail),
		},
	}
}

func (c hostInstallerPreflightChecker) inspectInstallerPath(path string) (string, string) {
	if !strings.HasPrefix(path, "/") {
		return "fail", fmt.Sprintf("%s is not an absolute Linux path", path)
	}

	info, err := c.stat(path)
	switch {
	case err == nil && info.IsDir():
		return "pass", fmt.Sprintf("%s exists as a directory", path)
	case err == nil:
		return "fail", fmt.Sprintf("%s exists as a file", path)
	case errors.Is(err, fs.ErrNotExist):
		return "pass", fmt.Sprintf("%s does not exist yet and can be created during install", path)
	default:
		return "warning", fmt.Sprintf("could not inspect %s: %v", path, err)
	}
}

func (c hostInstallerPreflightChecker) checkPortReadiness(req InstallerPreflightRequest) InstallerPreflightCheck {
	port, ok := extractInstallerPort(req.Addr)
	if !ok {
		return InstallerPreflightCheck{
			ID:      "port-readiness",
			Title:   "Port readiness",
			Status:  "fail",
			Summary: fmt.Sprintf("The listen address %q could not be parsed into a usable port.", req.Addr),
			Action:  "Use a listen address such as :8080, :80, or 0.0.0.0:8080.",
			TechnicalDetails: []string{
				fmt.Sprintf("addr=%s", req.Addr),
			},
		}
	}

	if port < 1024 {
		setcapPath, err := c.lookPath("setcap")
		if err != nil {
			return InstallerPreflightCheck{
				ID:      "port-readiness",
				Title:   "Port readiness",
				Status:  "fail",
				Summary: fmt.Sprintf("Port %d needs privileged-port support, but setcap is not available on this host.", port),
				Action:  "Install `libcap2-bin` or switch to :8080 for the simplest first run.",
				TechnicalDetails: []string{
					fmt.Sprintf("addr=%s", req.Addr),
					err.Error(),
				},
			}
		}

		return InstallerPreflightCheck{
			ID:      "port-readiness",
			Title:   "Port readiness",
			Status:  "warning",
			Summary: fmt.Sprintf("Port %d is supported here, but it will require CAP_NET_BIND_SERVICE during install.", port),
			Action:  "Stay with :8080 if you want the lowest-friction first run, or keep this port and let the installer grant the capability.",
			TechnicalDetails: []string{
				fmt.Sprintf("addr=%s", req.Addr),
				fmt.Sprintf("setcap=%s", setcapPath),
			},
		}
	}

	return InstallerPreflightCheck{
		ID:      "port-readiness",
		Title:   "Port readiness",
		Status:  "pass",
		Summary: fmt.Sprintf("Port %d avoids privileged-port setup on this host.", port),
		Action:  "You can move to :80 or :443 later when you are ready for direct public traffic or a reverse proxy.",
		TechnicalDetails: []string{
			fmt.Sprintf("addr=%s", req.Addr),
		},
	}
}

func (c hostInstallerPreflightChecker) checkExternalServices(ctx context.Context, req InstallerPreflightRequest) InstallerPreflightCheck {
	if req.StorageDriver != "postgres" && req.SessionStore != "postgres" && req.RedisAddr == "" {
		return InstallerPreflightCheck{
			ID:      "external-services",
			Title:   "External services",
			Status:  "pass",
			Summary: "Quick Install does not rely on external Postgres or Redis services.",
			Action:  "You can move to Postgres or Redis later in Advanced Install when you want them.",
			TechnicalDetails: []string{
				"storage_driver=json",
				"session_store=memory",
			},
		}
	}

	statuses := []string{}
	details := []string{}

	if req.StorageDriver == "postgres" {
		status, detail := c.inspectPostgresReachability(ctx, "primary-postgres", req.PostgresDsn)
		statuses = append(statuses, status)
		details = append(details, detail)
	}

	if req.SessionStore == "postgres" {
		switch {
		case req.SessionStoreDsn == "" && req.PostgresDsn != "":
			statuses = append(statuses, "pass")
			details = append(details, "session-store: reuses primary Postgres DSN")
		case req.SessionStoreDsn == "" && req.PostgresDsn == "":
			statuses = append(statuses, "fail")
			details = append(details, "session-store: Postgres selected without a usable DSN")
		case req.SessionStoreDsn == req.PostgresDsn:
			statuses = append(statuses, "pass")
			details = append(details, "session-store: same DSN as primary Postgres")
		default:
			status, detail := c.inspectPostgresReachability(ctx, "session-store", req.SessionStoreDsn)
			statuses = append(statuses, status)
			details = append(details, detail)
		}
	}

	if req.RedisAddr != "" {
		status, detail := c.inspectTCPReachability(ctx, "redis", req.RedisAddr, "6379", false)
		statuses = append(statuses, status)
		details = append(details, detail)
	}

	status := installerPreflightWorstStatus(statuses...)
	summary := "Configured external services are reachable from this host."
	action := "The host can reach the service endpoints currently configured in this install plan."
	switch status {
	case "fail":
		summary = "One or more configured external services are not ready from this host."
		action = "Check the DSN/address, host reachability, and firewall rules before you rely on those integrations."
	case "warning":
		summary = "Some configured external services need a quick review."
		action = "Double-check the affected addresses before starting the handoff."
	}

	return InstallerPreflightCheck{
		ID:               "external-services",
		Title:            "External services",
		Status:           status,
		Summary:          summary,
		Action:           action,
		TechnicalDetails: details,
	}
}

func (c hostInstallerPreflightChecker) inspectPostgresReachability(ctx context.Context, label, dsn string) (string, string) {
	if strings.TrimSpace(dsn) == "" {
		return "fail", fmt.Sprintf("%s: DSN missing", label)
	}
	return c.inspectTCPReachability(ctx, label, dsn, "5432", true)
}

func (c hostInstallerPreflightChecker) inspectTCPReachability(ctx context.Context, label, rawAddress, defaultPort string, parseAsURL bool) (string, string) {
	address, err := installerReachabilityAddress(rawAddress, defaultPort, parseAsURL)
	if err != nil {
		return "fail", fmt.Sprintf("%s: invalid address (%v)", label, err)
	}
	if err := c.dialAddress(ctx, "tcp", address); err != nil {
		return "fail", fmt.Sprintf("%s: cannot reach %s (%v)", label, address, err)
	}
	return "pass", fmt.Sprintf("%s: reachable at %s", label, address)
}

func (c hostInstallerPreflightChecker) checkTLSAssets(req InstallerPreflightRequest) InstallerPreflightCheck {
	if req.TLSCert == "" && req.TLSKey == "" {
		return InstallerPreflightCheck{
			ID:      "tls-assets",
			Title:   "TLS assets",
			Status:  "pass",
			Summary: "Direct HTTPS is not configured in this install plan, so no local certificate files are required yet.",
			Action:  "Leave both fields blank if you plan to terminate TLS elsewhere, or add both paths later.",
		}
	}
	if req.TLSCert == "" || req.TLSKey == "" {
		return InstallerPreflightCheck{
			ID:      "tls-assets",
			Title:   "TLS assets",
			Status:  "fail",
			Summary: "TLS needs both a certificate path and a key path.",
			Action:  "Provide both files together, or leave both fields blank for an HTTP/reverse-proxy first run.",
			TechnicalDetails: []string{
				fmt.Sprintf("tls_cert=%s", req.TLSCert),
				fmt.Sprintf("tls_key=%s", req.TLSKey),
			},
		}
	}

	certStatus, certDetail := c.inspectFile(req.TLSCert)
	keyStatus, keyDetail := c.inspectFile(req.TLSKey)
	status := installerPreflightWorstStatus(certStatus, keyStatus)
	summary := "TLS certificate files are present on this host."
	action := "The installer can copy these files into the managed certs directory."
	switch status {
	case "fail":
		summary = "One or more TLS files could not be confirmed on this host."
		action = "Fix the missing path, point at a real certificate/key pair, or leave both TLS fields blank for now."
	case "warning":
		summary = "The host could not fully inspect every TLS file path."
		action = "Verify the certificate and key paths before you rely on direct HTTPS from BitRiver Live."
	}

	return InstallerPreflightCheck{
		ID:      "tls-assets",
		Title:   "TLS assets",
		Status:  status,
		Summary: summary,
		Action:  action,
		TechnicalDetails: []string{
			fmt.Sprintf("tls_cert=%s", certDetail),
			fmt.Sprintf("tls_key=%s", keyDetail),
		},
	}
}

func (c hostInstallerPreflightChecker) inspectFile(path string) (string, string) {
	if !strings.HasPrefix(path, "/") {
		return "fail", fmt.Sprintf("%s is not an absolute path", path)
	}

	info, err := c.stat(path)
	switch {
	case err == nil && info.Mode().IsRegular():
		return "pass", fmt.Sprintf("%s exists", path)
	case err == nil:
		return "fail", fmt.Sprintf("%s exists but is not a regular file", path)
	case errors.Is(err, fs.ErrNotExist):
		return "fail", fmt.Sprintf("%s does not exist", path)
	default:
		return "warning", fmt.Sprintf("could not inspect %s: %v", path, err)
	}
}

func (c hostInstallerPreflightChecker) readOSRelease() (map[string]string, error) {
	raw, err := c.readFile("/etc/os-release")
	if err != nil {
		return nil, err
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"`)
		values[key] = value
	}
	return values, nil
}

func installerUbuntuVersionAtLeast(version string, minMajor, minMinor int) bool {
	parts := strings.Split(strings.TrimSpace(version), ".")
	if len(parts) == 0 || parts[0] == "" {
		return false
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	minor := 0
	if len(parts) > 1 && parts[1] != "" {
		minor, err = strconv.Atoi(parts[1])
		if err != nil {
			return false
		}
	}

	if major != minMajor {
		return major > minMajor
	}
	return minor >= minMinor
}

func extractInstallerPort(addr string) (int, bool) {
	trimmed := strings.TrimSpace(addr)
	switch {
	case strings.HasPrefix(trimmed, ":"):
		trimmed = "0.0.0.0" + trimmed
	case strings.HasPrefix(trimmed, "["):
		// keep as-is
	case strings.Count(trimmed, ":") == 0:
		return 0, false
	}

	_, portText, err := net.SplitHostPort(trimmed)
	if err != nil {
		return 0, false
	}

	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return 0, false
	}
	return port, true
}

func installerReachabilityAddress(rawAddress, defaultPort string, parseAsURL bool) (string, error) {
	trimmed := strings.TrimSpace(rawAddress)
	if trimmed == "" {
		return "", fmt.Errorf("address is empty")
	}

	if parseAsURL {
		parsed, err := url.Parse(trimmed)
		if err != nil {
			return "", err
		}
		host := strings.TrimSpace(parsed.Hostname())
		if host == "" {
			return "", fmt.Errorf("missing host")
		}
		port := parsed.Port()
		if port == "" {
			port = defaultPort
		}
		return net.JoinHostPort(host, port), nil
	}

	host, port, err := net.SplitHostPort(trimmed)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(host) == "" {
		return "", fmt.Errorf("missing host")
	}
	if strings.TrimSpace(port) == "" {
		port = defaultPort
	}
	return net.JoinHostPort(host, port), nil
}
