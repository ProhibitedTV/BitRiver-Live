package main

import (
	"bytes"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

var (
	goBuildCommand = runGoBuild
)

func runInstall(args []string) error {
	if len(args) == 0 {
		return errors.New("install subcommand required")
	}

	switch args[0] {
	case "systemd":
		return runInstallSystemd(args[1:])
	case "launchd":
		return runInstallLaunchd(args[1:])
	case "windows-service":
		return runInstallWindowsService(args[1:])
	default:
		return fmt.Errorf("unknown install subcommand: %s", args[0])
	}
}

func runInstallSystemd(args []string) error {
	args = normalizeFlagArgs(args)
	fs := flag.NewFlagSet("install systemd", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	installDir := fs.String("install-dir", "", "directory to place BitRiver Live binaries and configs")
	dataDir := fs.String("data-dir", "", "directory for persistent data (store.json, logs)")
	serviceUser := fs.String("service-user", "", "service user for the systemd unit")
	mode := fs.String("mode", "production", "application mode (production|development)")
	addr := fs.String("addr", "", "listen address for the API (default :80 production / :8080 development)")
	enableLogs := fs.Bool("enable-logs", false, "redirect stdout/stderr to a log file")
	logDir := fs.String("log-dir", "", "directory for log files (defaults to <data-dir>/logs when logs are enabled)")
	tlsCert := fs.String("tls-cert", "", "path to TLS certificate")
	tlsKey := fs.String("tls-key", "", "path to TLS key")
	allowSelfSignup := fs.Bool("allow-self-signup", false, "allow viewer self-registration")
	storageDriver := fs.String("storage-driver", "postgres", "storage backend (postgres|json)")
	postgresDSN := fs.String("postgres-dsn", "", "Postgres DSN for storage")
	sessionStore := fs.String("session-store", "", "session store driver (defaults to postgres when storage uses Postgres)")
	sessionStoreDSN := fs.String("session-store-dsn", "", "Postgres DSN for the session store")
	rateGlobalRPS := fs.String("rate-global-rps", "", "global request rate limit")
	rateLoginLimit := fs.String("rate-login-limit", "", "login burst limit")
	rateLoginWindow := fs.String("rate-login-window", "", "login window duration")
	redisAddr := fs.String("redis-addr", "", "Redis address for rate limiting")
	redisPassword := fs.String("redis-password", "", "Redis password for rate limiting")
	bootstrapAdminEmail := fs.String("bootstrap-admin-email", "", "seed administrator email")
	bootstrapAdminPassword := fs.String("bootstrap-admin-password", "", "seed administrator password (generated when empty)")
	envFile := fs.String("env-file", "", "path to write the environment file (default <install-dir>/.env)")
	unitFile := fs.String("unit-file", "", "path to write the systemd unit (default <install-dir>/bitriver-live.service)")
	serverBinary := fs.String("server-binary", "", "path to a prebuilt bitriver-live binary")
	bootstrapBinary := fs.String("bootstrap-admin-binary", "", "path to a prebuilt bootstrap-admin binary")
	buildFromSource := fs.Bool("build-from-source", false, "build binaries from the current source tree")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(*installDir) == "" || strings.TrimSpace(*dataDir) == "" || strings.TrimSpace(*serviceUser) == "" {
		return errors.New("install-dir, data-dir, and service-user are required")
	}

	if *addr == "" {
		if strings.EqualFold(*mode, "production") {
			*addr = ":80"
		} else {
			*addr = ":8080"
		}
	}

	logDirectory := strings.TrimSpace(*logDir)
	logsEnabled := *enableLogs || logDirectory != ""
	if logsEnabled && logDirectory == "" {
		logDirectory = filepath.Join(*dataDir, "logs")
	}

	if *sessionStore == "" && strings.EqualFold(*storageDriver, "postgres") {
		*sessionStore = "postgres"
	}

	envPath := *envFile
	if envPath == "" {
		envPath = filepath.Join(*installDir, ".env")
	}
	unitPath := *unitFile
	if unitPath == "" {
		unitPath = filepath.Join(*installDir, "bitriver-live.service")
	}

	buildRequested := *buildFromSource
	userSetBuild := false
	userSetServerBinary := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "build-from-source":
			userSetBuild = true
		case "server-binary":
			userSetServerBinary = true
		}
	})
	if !userSetBuild && !userSetServerBinary {
		if _, err := os.Stat(filepath.Join(repoRoot(), "go.mod")); err == nil {
			buildRequested = true
		}
	}

	if err := os.MkdirAll(*installDir, 0o755); err != nil {
		return fmt.Errorf("create install dir: %w", err)
	}
	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	if logsEnabled && logDirectory != "" {
		if err := os.MkdirAll(logDirectory, 0o755); err != nil {
			return fmt.Errorf("create log dir: %w", err)
		}
	}

	serverTarget := filepath.Join(*installDir, "bitriver-live")
	bootstrapTarget := filepath.Join(*installDir, "bootstrap-admin")

	if buildRequested {
		if err := goBuildCommand(serverTarget, filepath.Join(repoRoot(), "cmd", "server")); err != nil {
			return err
		}
		if err := goBuildCommand(bootstrapTarget, filepath.Join(repoRoot(), "cmd", "tools", "bootstrap-admin")); err != nil {
			return err
		}
	} else {
		if *serverBinary == "" || *bootstrapBinary == "" {
			return errors.New("server-binary and bootstrap-admin-binary are required when build-from-source is false")
		}
		if err := copyExecutable(*serverBinary, serverTarget); err != nil {
			return fmt.Errorf("copy server binary: %w", err)
		}
		if err := copyExecutable(*bootstrapBinary, bootstrapTarget); err != nil {
			return fmt.Errorf("copy bootstrap-admin binary: %w", err)
		}
	}

	dataFile := filepath.Join(*dataDir, "store.json")
	envValues := map[string]string{
		"BITRIVER_LIVE_ADDR":              *addr,
		"BITRIVER_LIVE_MODE":              *mode,
		"BITRIVER_LIVE_DATA":              dataFile,
		"BITRIVER_LIVE_STORAGE_DRIVER":    *storageDriver,
		"BITRIVER_LIVE_ALLOW_SELF_SIGNUP": strconv.FormatBool(*allowSelfSignup),
	}

	if *tlsCert != "" {
		envValues["BITRIVER_LIVE_TLS_CERT"] = *tlsCert
	}
	if *tlsKey != "" {
		envValues["BITRIVER_LIVE_TLS_KEY"] = *tlsKey
	}
	if *rateGlobalRPS != "" {
		envValues["BITRIVER_LIVE_RATE_GLOBAL_RPS"] = *rateGlobalRPS
	}
	if *rateLoginLimit != "" {
		envValues["BITRIVER_LIVE_RATE_LOGIN_LIMIT"] = *rateLoginLimit
	}
	if *rateLoginWindow != "" {
		envValues["BITRIVER_LIVE_RATE_LOGIN_WINDOW"] = *rateLoginWindow
	}
	if *redisAddr != "" {
		envValues["BITRIVER_LIVE_RATE_REDIS_ADDR"] = *redisAddr
	}
	if *redisPassword != "" {
		envValues["BITRIVER_LIVE_RATE_REDIS_PASSWORD"] = *redisPassword
	}
	if *postgresDSN != "" {
		envValues["BITRIVER_LIVE_POSTGRES_DSN"] = *postgresDSN
	}
	if *sessionStore != "" {
		envValues["BITRIVER_LIVE_SESSION_STORE"] = *sessionStore
	}
	if *sessionStoreDSN != "" {
		envValues["BITRIVER_LIVE_SESSION_POSTGRES_DSN"] = *sessionStoreDSN
	}

	generatedPassword := ""
	if *bootstrapAdminEmail != "" {
		envValues["BITRIVER_LIVE_ADMIN_EMAIL"] = *bootstrapAdminEmail
		password := strings.TrimSpace(*bootstrapAdminPassword)
		if password == "" {
			var err error
			password, err = generateStrongPassword()
			if err != nil {
				return err
			}
			generatedPassword = password
		}
		envValues["BITRIVER_LIVE_ADMIN_PASSWORD"] = password
	}

	if err := writeEnvFile(envPath, envValues); err != nil {
		return err
	}

	unitContent := renderSystemdUnit(systemdUnitConfig{
		ServiceUser:  *serviceUser,
		InstallDir:   *installDir,
		EnvFile:      envPath,
		EnableLogs:   logsEnabled && logDirectory != "",
		LogDir:       logDirectory,
		RequiresBind: requiresBindCapability(*addr),
	})
	if err := os.WriteFile(unitPath, []byte(unitContent), 0o644); err != nil {
		return fmt.Errorf("write systemd unit: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Wrote environment file to %s\n", envPath)
	fmt.Fprintf(os.Stdout, "Wrote systemd unit to %s\n", unitPath)
	if generatedPassword != "" {
		fmt.Fprintf(os.Stdout, "Generated bootstrap admin password for %s: %s\n", *bootstrapAdminEmail, generatedPassword)
	}
	if requiresBindCapability(*addr) {
		fmt.Fprintln(os.Stdout, "Note: binding to a privileged port may require CAP_NET_BIND_SERVICE when registering the unit.")
	}

	return nil
}

func runInstallLaunchd(args []string) error {
	args = normalizeFlagArgs(args)
	fs := flag.NewFlagSet("install launchd", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	installDir := fs.String("install-dir", "", "directory containing bitriver-live")
	dataDir := fs.String("data-dir", "", "directory for persistent data (store.json, logs)")
	label := fs.String("label", "com.bitriver.live", "launchd label")
	addr := fs.String("addr", ":8080", "listen address for the API")
	logDir := fs.String("log-dir", "", "directory for log files")
	envFile := fs.String("env-file", "", "path to write the environment file (default <install-dir>/.env)")
	plistPath := fs.String("plist", "", "path to write the launchd plist (default <install-dir>/<label>.plist)")
	serverBinary := fs.String("server-binary", "", "path to a prebuilt bitriver-live binary")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(*installDir) == "" || strings.TrimSpace(*dataDir) == "" {
		return errors.New("install-dir and data-dir are required")
	}
	if *serverBinary == "" {
		return errors.New("server-binary is required for launchd staging")
	}

	if err := os.MkdirAll(*installDir, 0o755); err != nil {
		return fmt.Errorf("create install dir: %w", err)
	}
	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	if *logDir != "" {
		if err := os.MkdirAll(*logDir, 0o755); err != nil {
			return fmt.Errorf("create log dir: %w", err)
		}
	}

	serverTarget := filepath.Join(*installDir, "bitriver-live")
	if err := copyExecutable(*serverBinary, serverTarget); err != nil {
		return fmt.Errorf("copy server binary: %w", err)
	}

	envPath := *envFile
	if envPath == "" {
		envPath = filepath.Join(*installDir, ".env")
	}
	envValues := map[string]string{
		"BITRIVER_LIVE_ADDR":           *addr,
		"BITRIVER_LIVE_MODE":           "production",
		"BITRIVER_LIVE_DATA":           filepath.Join(*dataDir, "store.json"),
		"BITRIVER_LIVE_STORAGE_DRIVER": "postgres",
	}
	if err := writeEnvFile(envPath, envValues); err != nil {
		return err
	}

	plist := renderLaunchdPlist(launchdConfig{
		Label:         *label,
		InstallDir:    *installDir,
		EnvPath:       envPath,
		LogDir:        *logDir,
		ProgramPath:   serverTarget,
		ProgramArgs:   []string{},
		SessionCreate: true,
	})

	plistOut := *plistPath
	if plistOut == "" {
		plistOut = filepath.Join(*installDir, fmt.Sprintf("%s.plist", *label))
	}

	if err := os.WriteFile(plistOut, []byte(plist), 0o644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Wrote environment file to %s\n", envPath)
	fmt.Fprintf(os.Stdout, "Wrote launchd plist to %s\n", plistOut)
	return nil
}

func runInstallWindowsService(args []string) error {
	args = normalizeFlagArgs(args)
	fs := flag.NewFlagSet("install windows-service", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	installDir := fs.String("install-dir", "", "directory containing bitriver-live.exe")
	dataDir := fs.String("data-dir", "", "directory for persistent data (store.json, logs)")
	serviceName := fs.String("service-name", "BitRiverLive", "Windows service name")
	displayName := fs.String("display-name", "BitRiver Live", "Windows service display name")
	addr := fs.String("addr", ":8080", "listen address for the API")
	envFile := fs.String("env-file", "", "path to write the environment file (default <install-dir>\\.env)")
	scriptPath := fs.String("script", "", "path to write the installer PowerShell script")
	serverBinary := fs.String("server-binary", "", "path to a prebuilt bitriver-live.exe")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(*installDir) == "" || strings.TrimSpace(*dataDir) == "" {
		return errors.New("install-dir and data-dir are required")
	}
	if *serverBinary == "" {
		return errors.New("server-binary is required for Windows staging")
	}

	if err := os.MkdirAll(*installDir, 0o755); err != nil {
		return fmt.Errorf("create install dir: %w", err)
	}
	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	serverTarget := filepath.Join(*installDir, "bitriver-live.exe")
	if err := copyExecutable(*serverBinary, serverTarget); err != nil {
		return fmt.Errorf("copy server binary: %w", err)
	}

	envPath := *envFile
	if envPath == "" {
		envPath = filepath.Join(*installDir, ".env")
	}
	envValues := map[string]string{
		"BITRIVER_LIVE_ADDR":           *addr,
		"BITRIVER_LIVE_MODE":           "production",
		"BITRIVER_LIVE_DATA":           filepath.Join(*dataDir, "store.json"),
		"BITRIVER_LIVE_STORAGE_DRIVER": "postgres",
	}
	if err := writeEnvFile(envPath, envValues); err != nil {
		return err
	}

	scriptOut := *scriptPath
	if scriptOut == "" {
		scriptOut = filepath.Join(*installDir, "install-bitriver-live-service.ps1")
	}

	script := renderWindowsServiceScript(windowsServiceConfig{
		ServiceName: *serviceName,
		DisplayName: *displayName,
		InstallDir:  *installDir,
		EnvPath:     envPath,
	})

	if err := os.WriteFile(scriptOut, []byte(script), 0o644); err != nil {
		return fmt.Errorf("write Windows service script: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Wrote environment file to %s\n", envPath)
	fmt.Fprintf(os.Stdout, "Wrote Windows service installer to %s\n", scriptOut)
	return nil
}

func renderSystemdUnit(cfg systemdUnitConfig) string {
	var buf strings.Builder
	buf.WriteString("[Unit]\n")
	buf.WriteString("Description=BitRiver Live Streaming Control Center\n")
	buf.WriteString("After=network.target\n\n")
	buf.WriteString("[Service]\n")
	buf.WriteString("Type=simple\n")
	buf.WriteString(fmt.Sprintf("User=%s\n", cfg.ServiceUser))
	buf.WriteString(fmt.Sprintf("EnvironmentFile=%s\n", cfg.EnvFile))
	buf.WriteString(fmt.Sprintf("WorkingDirectory=%s\n", cfg.InstallDir))
	buf.WriteString(fmt.Sprintf("ExecStart=%s\n", filepath.Join(cfg.InstallDir, "bitriver-live")))
	buf.WriteString("Restart=on-failure\n")
	if cfg.EnableLogs && cfg.LogDir != "" {
		logPath := filepath.Join(cfg.LogDir, "server.log")
		buf.WriteString(fmt.Sprintf("StandardOutput=append:%s\n", logPath))
		buf.WriteString(fmt.Sprintf("StandardError=append:%s\n", logPath))
	}
	if cfg.RequiresBind {
		buf.WriteString("AmbientCapabilities=CAP_NET_BIND_SERVICE\n")
		buf.WriteString("CapabilityBoundingSet=CAP_NET_BIND_SERVICE\n")
		buf.WriteString("NoNewPrivileges=yes\n")
	}
	buf.WriteString("\n[Install]\n")
	buf.WriteString("WantedBy=multi-user.target\n")
	return buf.String()
}

func renderLaunchdPlist(cfg launchdConfig) string {
	var buf strings.Builder
	buf.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	buf.WriteString("<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n")
	buf.WriteString("<plist version=\"1.0\">\n")
	buf.WriteString("<dict>\n")
	buf.WriteString(fmt.Sprintf("  <key>Label</key>\n  <string>%s</string>\n", cfg.Label))
	buf.WriteString("  <key>ProgramArguments</key>\n  <array>\n")
	buf.WriteString("    <string>/bin/sh</string>\n")
	buf.WriteString("    <string>-c</string>\n")
	exportLine := fmt.Sprintf("cd %s && set -a && . %s && exec %s", cfg.InstallDir, cfg.EnvPath, cfg.ProgramPath)
	buf.WriteString(fmt.Sprintf("    <string>%s</string>\n", exportLine))
	buf.WriteString("  </array>\n")
	buf.WriteString("  <key>WorkingDirectory</key>\n")
	buf.WriteString(fmt.Sprintf("  <string>%s</string>\n", cfg.InstallDir))
	if cfg.LogDir != "" {
		buf.WriteString("  <key>StandardOutPath</key>\n")
		buf.WriteString(fmt.Sprintf("  <string>%s</string>\n", filepath.Join(cfg.LogDir, "server.log")))
		buf.WriteString("  <key>StandardErrorPath</key>\n")
		buf.WriteString(fmt.Sprintf("  <string>%s</string>\n", filepath.Join(cfg.LogDir, "server.log")))
	}
	buf.WriteString("  <key>RunAtLoad</key>\n  <true/>\n")
	if cfg.SessionCreate {
		buf.WriteString("  <key>SessionCreate</key>\n  <true/>\n")
	}
	buf.WriteString("  <key>KeepAlive</key>\n  <true/>\n")
	buf.WriteString("</dict>\n</plist>\n")
	return buf.String()
}

func renderWindowsServiceScript(cfg windowsServiceConfig) string {
	binaryPath := windowsPath(filepath.Join(cfg.InstallDir, "bitriver-live.exe"))
	envPath := windowsPath(cfg.EnvPath)
	runWrapper := windowsPath(filepath.Join(cfg.InstallDir, "run-bitriver-live.ps1"))

	var runBuf strings.Builder
	runBuf.WriteString("param()\n")
	runBuf.WriteString(fmt.Sprintf("$envFile = \"%s\"\n", envPath))
	runBuf.WriteString("if (Test-Path $envFile) {\n")
	runBuf.WriteString("  Get-Content $envFile | ForEach-Object {\n")
	runBuf.WriteString("    if ($_ -match '^(.*?)=(.*)$') {\n")
	runBuf.WriteString("      $name = $matches[1]\n")
	runBuf.WriteString("      $value = $matches[2]\n")
	runBuf.WriteString("      [System.Environment]::SetEnvironmentVariable($name, $value, 'Process')\n")
	runBuf.WriteString("    }\n  }\n}\n")
	runBuf.WriteString(fmt.Sprintf("Set-Location \"%s\"\n", windowsPath(cfg.InstallDir)))
	runBuf.WriteString(fmt.Sprintf("& \"%s\"\n", binaryPath))

	var buf strings.Builder
	buf.WriteString("# PowerShell helper to register BitRiver Live as a Windows Service.\n")
	buf.WriteString("# Run with Administrator privileges.\n")
	buf.WriteString(fmt.Sprintf("$runScript = \"%s\"\n", runWrapper))
	buf.WriteString("$runContent = @'\n")
	buf.WriteString(runBuf.String())
	buf.WriteString("'@\n")
	buf.WriteString("Set-Content -Path $runScript -Value $runContent -Force\n")
	buf.WriteString(fmt.Sprintf("$serviceName = \"%s\"\n", cfg.ServiceName))
	buf.WriteString(fmt.Sprintf("$displayName = \"%s\"\n", cfg.DisplayName))
	buf.WriteString("$existing = Get-Service -Name $serviceName -ErrorAction SilentlyContinue\n")
	buf.WriteString("if ($existing) { Stop-Service -Name $serviceName -ErrorAction SilentlyContinue; Set-Service -Name $serviceName -StartupType Disabled }\n")
	buf.WriteString("$binaryPath = \"powershell -NoProfile -File `\"$runScript`\"\"\n")
	buf.WriteString("New-Service -Name $serviceName -BinaryPathName $binaryPath -DisplayName $displayName -Description 'BitRiver Live Streaming Control Center' -StartupType Automatic\n")
	buf.WriteString("Start-Service -Name $serviceName\n")
	buf.WriteString("Write-Output \"Registered and started $serviceName. Update .env to configure the service.\"\n")
	return buf.String()
}

func writeEnvFile(path string, values map[string]string) error {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf strings.Builder
	for _, k := range keys {
		buf.WriteString(fmt.Sprintf("%s=%s\n", k, values[k]))
	}

	if err := os.WriteFile(path, []byte(buf.String()), 0o644); err != nil {
		return fmt.Errorf("write env file: %w", err)
	}
	return nil
}

func copyExecutable(src, dst string) error {
	contents, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, contents, 0o755); err != nil {
		return err
	}
	return nil
}

func runGoBuild(outputPath, packagePath string) error {
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("go toolchain missing: %w", err)
	}

	cmd := exec.Command("go", "build", "-tags", "postgres", "-o", outputPath, packagePath)
	cmd.Dir = repoRoot()
	cmd.Env = append(os.Environ(),
		"GOTOOLCHAIN=local",
		"GOPROXY=off",
		"GOSUMDB=off",
		"GOFLAGS=-trimpath",
	)

	var stderr bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build %s: %w (%s)", packagePath, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func requiresBindCapability(addr string) bool {
	port := extractPort(addr)
	if port == 0 {
		return false
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return false
	}
	return port < 1024
}

func extractPort(addr string) int {
	trimmed := strings.TrimSpace(addr)
	trimmed = strings.TrimPrefix(trimmed, "http://")
	trimmed = strings.TrimPrefix(trimmed, "https://")

	if idx := strings.LastIndex(trimmed, ":"); idx != -1 && idx < len(trimmed)-1 {
		raw := trimmed[idx+1:]
		if p, err := strconv.Atoi(raw); err == nil {
			return p
		}
	}
	return 0
}

func windowsPath(path string) string {
	return strings.ReplaceAll(path, "/", "\\")
}

func generateStrongPassword() (string, error) {
	password := make([]byte, 48)
	charset := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	for i := 0; i < len(password); i++ {
		index, err := randomInt(len(charset))
		if err != nil {
			return "", err
		}
		password[i] = charset[index]
	}

	if !passwordHasClasses(password) {
		for i := 0; i < 4; i++ {
			index, err := randomInt(len(charset))
			if err != nil {
				return "", err
			}
			password[i] = charset[index]
		}
	}

	return string(password), nil
}

func randomInt(max int) (int, error) {
	if max <= 0 {
		return 0, errors.New("invalid max for randomInt")
	}

	var b [1]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return int(b[0]) % max, nil
}

func passwordHasClasses(password []byte) bool {
	hasLower := false
	hasUpper := false
	hasDigit := false

	for _, c := range password {
		switch {
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= '0' && c <= '9':
			hasDigit = true
		}
	}

	return hasLower && hasUpper && hasDigit
}

func normalizeFlagArgs(args []string) []string {
	normalized := make([]string, len(args))
	for i, arg := range args {
		if strings.HasPrefix(arg, "--") && arg != "--" {
			normalized[i] = "-" + strings.TrimPrefix(arg, "--")
			continue
		}
		normalized[i] = arg
	}
	return normalized
}

type systemdUnitConfig struct {
	ServiceUser  string
	InstallDir   string
	EnvFile      string
	EnableLogs   bool
	LogDir       string
	RequiresBind bool
}

type launchdConfig struct {
	Label         string
	InstallDir    string
	EnvPath       string
	LogDir        string
	ProgramPath   string
	ProgramArgs   []string
	SessionCreate bool
}

type windowsServiceConfig struct {
	ServiceName string
	DisplayName string
	InstallDir  string
	EnvPath     string
}
