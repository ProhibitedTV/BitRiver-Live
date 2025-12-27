package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bitriver-live/internal/envutil"
	"bitriver-live/internal/executil"
	"bitriver-live/internal/platformutil"
)

type omeRenderInputs struct {
	ScriptPath   string
	Template     string
	Output       string
	Bind         string
	ServerIP     string
	Port         string
	TLSPort      string
	TCPRelay     string
	ICECandidate string
	Username     string
	Password     string
	APIToken     string
	AccessToken  string
	ImageTag     string
}

type processRunner interface {
	Run(name string, args []string, opts ...executil.RunOption) error
}

type execRunner struct{}

func (execRunner) Run(name string, args []string, opts ...executil.RunOption) error {
	return executil.Run(name, args, opts...)
}

func runOME(args []string) {
	fs := flag.NewFlagSet("ome", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s ome <command>\n", os.Args[0])
		fmt.Fprintln(fs.Output(), "Commands:")
		fmt.Fprintln(fs.Output(), "  render    Render deploy/ome/Server.generated.xml")
	}

	_ = fs.Parse(args)
	if fs.NArg() < 1 {
		fs.Usage()
		os.Exit(1)
	}

	switch fs.Arg(0) {
	case "render":
		if err := runOMERender(fs.Args()[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown ome subcommand: %s\n", fs.Arg(0))
		fs.Usage()
		os.Exit(1)
	}
}

func runOMERender(args []string) error {
	return runOMERenderWithRunner(args, platformutil.FindPythonCommands, execRunner{})
}

func runOMERenderWithRunner(args []string, findPython func() ([]platformutil.Command, error), runner processRunner) error {
	fs := flag.NewFlagSet("ome render", flag.ExitOnError)
	envFile := fs.String("env-file", ".env", "Path to the environment file with OME settings")
	template := fs.String("template", "", "Path to the Server.xml template")
	output := fs.String("output", "", "Destination for the rendered Server.xml")
	bind := fs.String("bind", "", "Bind address for OME")
	serverIP := fs.String("server-ip", "", "Public IP advertised by OME")
	port := fs.String("port", "", "OME server port")
	tlsPort := fs.String("tls-port", "", "OME server TLS port")
	tcpRelay := fs.String("tcp-relay", "", "TCP relay address")
	iceCandidate := fs.String("ice-candidate", "", "Advertised ICE candidate")
	username := fs.String("username", "", "Authentication username")
	password := fs.String("password", "", "Authentication password")
	apiToken := fs.String("api-token", "", "Managers API token")
	accessToken := fs.String("access-token", "", "Health probe access token")
	imageTag := fs.String("image-tag", "", "OME image tag recorded in the render")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s ome render [options]\n", os.Args[0])
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to determine working directory: %w", err)
	}

	envPath := *envFile
	if !filepath.IsAbs(envPath) {
		envPath = filepath.Join(workDir, envPath)
	}

	envValues, err := loadEnvValues(envPath)
	if err != nil {
		return fmt.Errorf("failed to read .env: %w", err)
	}

	scriptPath := filepath.Join(workDir, "scripts", "render_ome_config.py")

	inputs := omeRenderInputs{
		ScriptPath:   scriptPath,
		Template:     *template,
		Output:       *output,
		Bind:         *bind,
		ServerIP:     *serverIP,
		Port:         *port,
		TLSPort:      *tlsPort,
		TCPRelay:     *tcpRelay,
		ICECandidate: *iceCandidate,
		Username:     *username,
		Password:     *password,
		APIToken:     *apiToken,
		AccessToken:  *accessToken,
		ImageTag:     *imageTag,
	}

	config, err := prepareOMERenderConfig(envValues, inputs, workDir)
	if err != nil {
		return err
	}

	pythonCommands, err := findPython()
	if err != nil {
		return err
	}

	return renderOMEConfig(pythonCommands, config, runner)
}

func prepareOMERenderConfig(envValues map[string]string, inputs omeRenderInputs, workDir string) (omeRenderInputs, error) {
	apply := func(current, envKey string) string {
		if current != "" {
			return current
		}
		return envValues[envKey]
	}

	if inputs.ScriptPath == "" {
		inputs.ScriptPath = filepath.Join(workDir, "scripts", "render_ome_config.py")
	}

	inputs.Template = apply(inputs.Template, "")
	if inputs.Template == "" {
		inputs.Template = filepath.Join(workDir, "deploy", "ome", "Server.xml")
	}

	inputs.Output = apply(inputs.Output, "")
	if inputs.Output == "" {
		inputs.Output = filepath.Join(workDir, "deploy", "ome", "Server.generated.xml")
	}

	var err error
	inputs, err = normalizeOMEPaths(inputs, workDir)
	if err != nil {
		return omeRenderInputs{}, err
	}

	inputs.Bind = apply(inputs.Bind, "BITRIVER_OME_BIND")
	inputs.ServerIP = apply(inputs.ServerIP, "BITRIVER_OME_IP")
	inputs.Port = apply(inputs.Port, "BITRIVER_OME_SERVER_PORT")
	inputs.TLSPort = apply(inputs.TLSPort, "BITRIVER_OME_SERVER_TLS_PORT")
	inputs.TCPRelay = apply(inputs.TCPRelay, "BITRIVER_OME_TCP_RELAY")
	inputs.ICECandidate = apply(inputs.ICECandidate, "BITRIVER_OME_ICE_CANDIDATE")
	inputs.Username = apply(inputs.Username, "BITRIVER_OME_USERNAME")
	inputs.Password = apply(inputs.Password, "BITRIVER_OME_PASSWORD")
	inputs.APIToken = apply(inputs.APIToken, "BITRIVER_OME_API_TOKEN")
	inputs.AccessToken = apply(inputs.AccessToken, "BITRIVER_OME_ACCESS_TOKEN")
	inputs.ImageTag = apply(inputs.ImageTag, "BITRIVER_OME_IMAGE_TAG")

	if inputs.ServerIP == "" {
		inputs.ServerIP = inputs.Bind
	}

	if inputs.AccessToken == "" {
		inputs.AccessToken = inputs.APIToken
	}

	missing := []string{}
	for key, value := range map[string]string{
		"bind":          inputs.Bind,
		"port":          inputs.Port,
		"tls-port":      inputs.TLSPort,
		"tcp-relay":     inputs.TCPRelay,
		"ice-candidate": inputs.ICECandidate,
		"username":      inputs.Username,
		"password":      inputs.Password,
		"api-token":     inputs.APIToken,
	} {
		if value == "" {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		return omeRenderInputs{}, fmt.Errorf("missing required values: %s", strings.Join(missing, ", "))
	}

	return inputs, nil
}

func renderOMEConfig(pythonCommands []platformutil.Command, inputs omeRenderInputs, runner processRunner) error {
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to determine working directory: %w", err)
	}

	normalizedInputs, err := normalizeOMEPaths(inputs, workDir)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(normalizedInputs.Output), 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	scriptArgs := []string{
		normalizedInputs.ScriptPath,
		"--template", normalizedInputs.Template,
		"--output", normalizedInputs.Output,
		"--bind", normalizedInputs.Bind,
		"--server-ip", normalizedInputs.ServerIP,
		"--tcp-relay", normalizedInputs.TCPRelay,
		"--ice-candidate", normalizedInputs.ICECandidate,
		"--port", normalizedInputs.Port,
		"--tls-port", normalizedInputs.TLSPort,
		"--username", normalizedInputs.Username,
		"--password", normalizedInputs.Password,
		"--api-token", normalizedInputs.APIToken,
	}

	if normalizedInputs.AccessToken != "" {
		scriptArgs = append(scriptArgs, "--access-token", normalizedInputs.AccessToken)
	}
	if normalizedInputs.ImageTag != "" {
		scriptArgs = append(scriptArgs, "--image-tag", normalizedInputs.ImageTag)
	}

	var lastErr error
	for _, pythonCmd := range pythonCommands {
		args := append(append([]string{}, pythonCmd.Args...), scriptArgs...)
		if err := runner.Run(pythonCmd.Executable, args, executil.WithStdout(os.Stdout), executil.WithStderr(os.Stderr)); err != nil {
			lastErr = err
			continue
		}

		return nil
	}

	if lastErr != nil {
		return lastErr
	}

	return errors.New("python executable not configured")
}

func loadEnvValues(envPath string) (map[string]string, error) {
	base := envutil.FromEnviron(os.Environ())

	values, err := envutil.LoadFile(envPath, base)
	if err != nil {
		return nil, err
	}

	return values, nil
}

func normalizeOMEPaths(inputs omeRenderInputs, baseDir string) (omeRenderInputs, error) {
	normalize := func(path string) (string, error) {
		if path == "" {
			return "", nil
		}

		cleaned := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(path, "\\", "/")))
		if isAbsolutePath(cleaned) {
			return cleaned, nil
		}

		if baseDir != "" {
			cleaned = filepath.Join(baseDir, cleaned)
		}

		abs, err := filepath.Abs(cleaned)
		if err != nil {
			return "", err
		}

		return filepath.Clean(abs), nil
	}

	var err error
	inputs.ScriptPath, err = normalize(inputs.ScriptPath)
	if err != nil {
		return omeRenderInputs{}, fmt.Errorf("normalize script path: %w", err)
	}

	inputs.Template, err = normalize(inputs.Template)
	if err != nil {
		return omeRenderInputs{}, fmt.Errorf("normalize template path: %w", err)
	}

	inputs.Output, err = normalize(inputs.Output)
	if err != nil {
		return omeRenderInputs{}, fmt.Errorf("normalize output path: %w", err)
	}

	return inputs, nil
}

func isAbsolutePath(path string) bool {
	if filepath.IsAbs(path) {
		return true
	}

	if len(path) >= 2 {
		letter := path[0]
		if path[1] == ':' && ((letter >= 'A' && letter <= 'Z') || (letter >= 'a' && letter <= 'z')) {
			return true
		}
	}

	return false
}
