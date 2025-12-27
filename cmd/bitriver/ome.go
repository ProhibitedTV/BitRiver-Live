package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"bitriver-live/internal/platform"
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

type pythonRunner func(pythonPath string, args []string, stderr io.Writer) error

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
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to determine working directory: %w", err)
	}

	envValues, err := loadEnvValues(filepath.Join(workDir, ".env"))
	if err != nil {
		return fmt.Errorf("failed to read .env: %w", err)
	}

	scriptPath := filepath.Join(workDir, "scripts", "render_ome_config.py")
	templateDefault := filepath.Join(workDir, "deploy", "ome", "Server.xml")
	outputDefault := filepath.Join(workDir, "deploy", "ome", "Server.generated.xml")

	fs := flag.NewFlagSet("ome render", flag.ExitOnError)
	template := fs.String("template", templateDefault, "Path to the Server.xml template")
	output := fs.String("output", outputDefault, "Destination for the rendered Server.xml")
	bind := fs.String("bind", envValues["BITRIVER_OME_BIND"], "Bind address for OME")
	serverIP := fs.String("server-ip", envValues["BITRIVER_OME_IP"], "Public IP advertised by OME")
	port := fs.String("port", envValues["BITRIVER_OME_SERVER_PORT"], "OME server port")
	tlsPort := fs.String("tls-port", envValues["BITRIVER_OME_SERVER_TLS_PORT"], "OME server TLS port")
	tcpRelay := fs.String("tcp-relay", envValues["BITRIVER_OME_TCP_RELAY"], "TCP relay address")
	iceCandidate := fs.String("ice-candidate", envValues["BITRIVER_OME_ICE_CANDIDATE"], "Advertised ICE candidate")
	username := fs.String("username", envValues["BITRIVER_OME_USERNAME"], "Authentication username")
	password := fs.String("password", envValues["BITRIVER_OME_PASSWORD"], "Authentication password")
	apiToken := fs.String("api-token", envValues["BITRIVER_OME_API_TOKEN"], "Managers API token")
	accessToken := fs.String("access-token", envValues["BITRIVER_OME_ACCESS_TOKEN"], "Health probe access token")
	imageTag := fs.String("image-tag", envValues["BITRIVER_OME_IMAGE_TAG"], "OME image tag recorded in the render")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s ome render [options]\n", os.Args[0])
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

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

	pythonPath, err := platform.FindPythonExecutable()
	if err != nil {
		return err
	}

	return renderOMEConfig(pythonPath, config, execPython)
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

func renderOMEConfig(pythonPath string, inputs omeRenderInputs, runner pythonRunner) error {
	args := []string{
		inputs.ScriptPath,
		"--template", inputs.Template,
		"--output", inputs.Output,
		"--bind", inputs.Bind,
		"--server-ip", inputs.ServerIP,
		"--tcp-relay", inputs.TCPRelay,
		"--ice-candidate", inputs.ICECandidate,
		"--port", inputs.Port,
		"--tls-port", inputs.TLSPort,
		"--username", inputs.Username,
		"--password", inputs.Password,
		"--api-token", inputs.APIToken,
	}

	if inputs.AccessToken != "" {
		args = append(args, "--access-token", inputs.AccessToken)
	}
	if inputs.ImageTag != "" {
		args = append(args, "--image-tag", inputs.ImageTag)
	}

	if err := runner(pythonPath, args, os.Stderr); err != nil {
		return err
	}

	return nil
}

func execPython(pythonPath string, args []string, stderr io.Writer) error {
	cmd := exec.Command(pythonPath, args...)
	cmd.Stdout = os.Stdout
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		if stderrBuf.Len() > 0 {
			if _, copyErr := io.Copy(stderr, bytes.NewReader(stderrBuf.Bytes())); copyErr != nil {
				return errors.Join(err, copyErr)
			}
		}
		return err
	}

	if stderrBuf.Len() > 0 {
		_, _ = io.Copy(stderr, bytes.NewReader(stderrBuf.Bytes()))
	}

	return nil
}

func loadEnvValues(envPath string) (map[string]string, error) {
	values := make(map[string]string)

	for _, pair := range os.Environ() {
		if idx := strings.Index(pair, "="); idx != -1 {
			key := pair[:idx]
			val := pair[idx+1:]
			values[key] = val
		}
	}

	file, err := os.Open(envPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return values, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if idx := strings.Index(line, "="); idx != -1 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			values[key] = val
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return values, nil
}
