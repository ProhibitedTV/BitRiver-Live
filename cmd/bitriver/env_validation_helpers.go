package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"bitriver-live/internal/envutil"
	"bitriver-live/internal/envutil/pgdsn"
	"bitriver-live/internal/stringsutil"
)

var entropyRead = rand.Read
var interactivePromptInput io.Reader = os.Stdin
var interactivePromptOutput io.Writer = os.Stdout
var interactivePromptAvailable = stdinIsTerminal

// This file contains environment template helpers, secret generation, and
// env validation rules used by quickstart and installer flows.

func loadSampleCredentialValues(path string, keys []string) (map[string]string, error) {
	templateLines, err := readEnvTemplate(path)
	if err != nil {
		return nil, fmt.Errorf("load sample credentials: %w", err)
	}

	allowed := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		allowed[key] = struct{}{}
	}

	placeholders := make(map[string]string)
	for _, line := range templateLines {
		if line.Key == "" {
			continue
		}
		if _, ok := allowed[line.Key]; ok {
			placeholders[line.Key] = line.Value
		}
	}

	var missing []string
	for _, key := range keys {
		if _, ok := placeholders[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing sample credentials for %s in %s", strings.Join(missing, ", "), path)
	}

	return placeholders, nil
}

func readEnvTemplate(path string) ([]templateLine, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open template: %w", err)
	}
	defer file.Close()

	var lines []templateLine
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		raw := scanner.Text()
		key, value, ok := parseTemplateLine(raw)
		if ok {
			lines = append(lines, templateLine{Key: key, Value: value, Raw: raw})
			continue
		}
		lines = append(lines, templateLine{Raw: raw})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan template: %w", err)
	}
	return lines, nil
}

func loadEnvValues(path string, allowMissing bool) (map[string]string, error) {
	if !allowMissing {
		if _, err := os.Stat(path); err != nil {
			return nil, err
		}
	}

	values, err := envutil.LoadFile(path, nil)
	if err != nil {
		return nil, err
	}

	quotedValues, err := quotedEnvValues(path)
	if err != nil {
		if allowMissing && errors.Is(err, os.ErrNotExist) {
			return values, nil
		}
		return nil, err
	}

	for key, value := range quotedValues {
		values[key] = value
	}

	return values, nil
}

func quotedEnvValues(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := parseTemplateLine(scanner.Text())
		if ok && len(value) >= 2 && strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
			values[key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan env: %w", err)
	}
	return values, nil
}

type templateLine struct {
	Key   string
	Value string
	Raw   string
}

func parseTemplateLine(line string) (string, string, bool) {
	if strings.HasPrefix(strings.TrimSpace(line), "#") || !strings.Contains(line, "=") {
		return "", "", false
	}
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if key == "" {
		return "", "", false
	}
	return key, value, true
}

func mergeEnv(template []templateLine, existing, generated map[string]string) string {
	seen := make(map[string]struct{})
	var out []string

	for _, line := range template {
		if line.Key == "" {
			out = append(out, line.Raw)
			continue
		}

		seen[line.Key] = struct{}{}

		value := line.Value
		if v, ok := existing[line.Key]; ok && strings.TrimSpace(v) != "" {
			value = v
		} else if v, ok := generated[line.Key]; ok {
			value = v
		}
		out = append(out, fmt.Sprintf("%s=%s", line.Key, value))
	}

	extraKeys := make([]string, 0, len(existing))
	for k := range existing {
		if _, ok := seen[k]; !ok {
			extraKeys = append(extraKeys, k)
		}
	}
	sort.Strings(extraKeys)
	for _, k := range extraKeys {
		out = append(out, fmt.Sprintf("%s=%s", k, existing[k]))
	}

	return strings.Join(out, "\n") + "\n"
}

func generateEnvValues(existing map[string]string) (map[string]string, map[string]string, error) {
	if existing == nil {
		existing = make(map[string]string)
	}

	generated := make(map[string]string)
	newlyGenerated := make(map[string]string)

	generated["BITRIVER_LIVE_MODE"] = productionSafeMode(existing["BITRIVER_LIVE_MODE"])
	existing["BITRIVER_LIVE_MODE"] = generated["BITRIVER_LIVE_MODE"]
	generated["BITRIVER_OME_HEALTHCHECK_AUTH_MODE"] = normalizedOMEHealthcheckAuthMode(existing["BITRIVER_OME_HEALTHCHECK_AUTH_MODE"])
	existing["BITRIVER_OME_HEALTHCHECK_AUTH_MODE"] = generated["BITRIVER_OME_HEALTHCHECK_AUTH_MODE"]
	generated["BITRIVER_TRANSCODER_PUBLIC_BASE_URL"] = defaultIfPlaceholder("BITRIVER_TRANSCODER_PUBLIC_BASE_URL", existing, "http://localhost:9001/hls")
	generated["BITRIVER_SRS_PUBLIC_RTMP_BASE_URL"] = defaultIfPlaceholder("BITRIVER_SRS_PUBLIC_RTMP_BASE_URL", existing, "rtmp://localhost:1935/live")
	generated["BITRIVER_OME_PUBLIC_LLHLS_BASE_URL"] = defaultIfPlaceholder("BITRIVER_OME_PUBLIC_LLHLS_BASE_URL", existing, "http://localhost:8080/live")
	generated["NEXT_PUBLIC_VIEWER_URL"] = defaultIfPlaceholder("NEXT_PUBLIC_VIEWER_URL", existing, "http://localhost:8080/viewer")

	for key := range defaultEnvSecrets.secrets {
		current := strings.TrimSpace(existing[key])
		if current == "" || isForbiddenValue(key, current) {
			secret, err := randomSecret()
			if err != nil {
				return nil, nil, fmt.Errorf("generate %s: %w", key, err)
			}
			generated[key] = secret
			newlyGenerated[key] = secret
			existing[key] = ""
		}
		if isForbiddenValue(key, existing[key]) {
			existing[key] = ""
		}
	}

	if current := existing["BITRIVER_LIVE_METRICS_TOKEN"]; current == "" || isForbiddenValue("BITRIVER_LIVE_METRICS_TOKEN", current) {
		if generated["BITRIVER_LIVE_METRICS_TOKEN"] == "" {
			secret, err := randomSecret()
			if err != nil {
				return nil, nil, fmt.Errorf("generate BITRIVER_LIVE_METRICS_TOKEN: %w", err)
			}
			generated["BITRIVER_LIVE_METRICS_TOKEN"] = secret
			newlyGenerated["BITRIVER_LIVE_METRICS_TOKEN"] = secret
		}
		existing["BITRIVER_LIVE_METRICS_TOKEN"] = ""
	}

	if current := existing["BITRIVER_OME_USERNAME"]; current == "" || isForbiddenValue("BITRIVER_OME_USERNAME", current) {
		suffix, err := randomSuffix()
		if err != nil {
			return nil, nil, fmt.Errorf("generate BITRIVER_OME_USERNAME suffix: %w", err)
		}
		generated["BITRIVER_OME_USERNAME"] = fmt.Sprintf("ome-operator-%s", suffix)
		existing["BITRIVER_OME_USERNAME"] = ""
	}

	if val := existing["BITRIVER_REDIS_PASSWORD"]; val != "" && !isForbiddenValue("BITRIVER_REDIS_PASSWORD", val) {
		generated["BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD"] = stringsutil.FirstNonEmpty(existing["BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD"], val)
		delete(newlyGenerated, "BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD")
	} else {
		generated["BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD"] = generated["BITRIVER_REDIS_PASSWORD"]
		if _, ok := newlyGenerated["BITRIVER_REDIS_PASSWORD"]; ok {
			newlyGenerated["BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD"] = generated["BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD"]
		}
	}

	if current := strings.TrimSpace(existing["BITRIVER_LIVE_ADMIN_EMAIL"]); current == "" || isForbiddenValue("BITRIVER_LIVE_ADMIN_EMAIL", current) {
		generated["BITRIVER_LIVE_ADMIN_EMAIL"] = defaultEnvSecrets.adminEmail
		if current != "" {
			existing["BITRIVER_LIVE_ADMIN_EMAIL"] = ""
		}
	}

	return generated, newlyGenerated, nil
}

func normalizedOMEHealthcheckAuthMode(raw string) string {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch mode {
	case "", "accesstoken":
		return "accesstoken"
	case "basic":
		return "basic"
	default:
		return strings.TrimSpace(raw)
	}
}

func promptForAdminEmail(existing map[string]string) {
	current := strings.TrimSpace(existing["BITRIVER_LIVE_ADMIN_EMAIL"])
	if current != "" && !isForbiddenValue("BITRIVER_LIVE_ADMIN_EMAIL", current) {
		return
	}

	defaultEmail := defaultEnvSecrets.adminEmail
	if !interactivePromptAvailable() {
		existing["BITRIVER_LIVE_ADMIN_EMAIL"] = defaultEmail
		return
	}

	reader := bufio.NewReader(interactivePromptInput)
	fmt.Fprintf(interactivePromptOutput, "Enter the administrator email for BitRiver Live [%s]: ", defaultEmail)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		existing["BITRIVER_LIVE_ADMIN_EMAIL"] = defaultEmail
		return
	}
	email := strings.TrimSpace(line)
	if email == "" {
		email = defaultEmail
	}
	existing["BITRIVER_LIVE_ADMIN_EMAIL"] = email
}

func promptForQuickstartWizard(existing map[string]string, envPath string) error {
	if !interactivePromptAvailable() {
		return errors.New("the quickstart wizard requires an interactive terminal")
	}

	reader := bufio.NewReader(interactivePromptInput)
	fmt.Fprintln(interactivePromptOutput, "BitRiver Live first-run wizard")
	fmt.Fprintf(interactivePromptOutput, "Updating %s with guided quickstart settings. Missing required secrets will still be generated automatically.\n\n", envPath)

	adminEmail, err := promptRequiredWizardValue(reader, "Administrator email", wizardAdminEmailDefault(existing), validateWizardEmail)
	if err != nil {
		return err
	}
	viewerURL, err := promptRequiredWizardValue(reader, "Public viewer URL (example: https://stream.example.com/viewer)", wizardPublicURLDefault(existing, "NEXT_PUBLIC_VIEWER_URL"), func(value string) error {
		return validateWizardPublicURL("Public viewer URL", value)
	})
	if err != nil {
		return err
	}
	publicAPIURL, err := promptOptionalWizardValue(reader, "Public API URL (optional; leave blank to keep same-origin proxying)", wizardPublicURLDefault(existing, "NEXT_PUBLIC_API_BASE_URL"), func(value string) error {
		return validateWizardPublicURL("Public API URL", value)
	})
	if err != nil {
		return err
	}
	apiPort, err := promptRequiredWizardValue(reader, "API/control port", wizardAPIPortDefault(existing), validateWizardPort)
	if err != nil {
		return err
	}
	omeBind, err := promptRequiredWizardValue(reader, "OME listener IP", wizardHostDefault(existing, "BITRIVER_OME_BIND"), func(value string) error {
		return validateWizardHostValue("OME listener IP", value)
	})
	if err != nil {
		return err
	}
	omeIP, err := promptRequiredWizardValue(reader, "OME server listener IP", wizardHostDefault(existing, "BITRIVER_OME_IP"), func(value string) error {
		return validateWizardHostValue("OME server listener IP", value)
	})
	if err != nil {
		return err
	}
	transcoderURL, err := promptRequiredWizardValue(reader, "Transcoder public base URL (example: https://cdn.example.com/hls)", wizardPublicURLDefault(existing, "BITRIVER_TRANSCODER_PUBLIC_BASE_URL"), func(value string) error {
		return validateWizardPublicURL("Transcoder public base URL", value)
	})
	if err != nil {
		return err
	}
	allowSelfSignup, err := promptWizardBool(reader, "Allow viewers to self-register accounts", wizardAllowSelfSignupDefault(existing))
	if err != nil {
		return err
	}

	existing["BITRIVER_LIVE_ADMIN_EMAIL"] = adminEmail
	existing["NEXT_PUBLIC_VIEWER_URL"] = viewerURL
	existing["NEXT_PUBLIC_API_BASE_URL"] = publicAPIURL
	existing["BITRIVER_LIVE_PORT"] = apiPort
	existing["BITRIVER_LIVE_ADDR"] = fmt.Sprintf(":%s", apiPort)
	existing["BITRIVER_OME_BIND"] = omeBind
	existing["BITRIVER_OME_IP"] = omeIP
	existing["BITRIVER_TRANSCODER_PUBLIC_BASE_URL"] = transcoderURL
	existing["BITRIVER_LIVE_ALLOW_SELF_SIGNUP"] = strconv.FormatBool(allowSelfSignup)

	fmt.Fprintln(interactivePromptOutput)
	fmt.Fprintln(interactivePromptOutput, "Wizard selections:")
	fmt.Fprintf(interactivePromptOutput, "  Admin email: %s\n", adminEmail)
	fmt.Fprintf(interactivePromptOutput, "  Viewer URL: %s\n", viewerURL)
	if publicAPIURL == "" {
		fmt.Fprintln(interactivePromptOutput, "  Public API URL: (same-origin proxy)")
	} else {
		fmt.Fprintf(interactivePromptOutput, "  Public API URL: %s\n", publicAPIURL)
	}
	fmt.Fprintf(interactivePromptOutput, "  API/control port: %s\n", apiPort)
	fmt.Fprintf(interactivePromptOutput, "  OME bind host/IP: %s\n", omeBind)
	fmt.Fprintf(interactivePromptOutput, "  OME server listener IP: %s\n", omeIP)
	fmt.Fprintf(interactivePromptOutput, "  Transcoder public base URL: %s\n", transcoderURL)
	fmt.Fprintf(interactivePromptOutput, "  Self-signup: %t\n", allowSelfSignup)
	fmt.Fprintln(interactivePromptOutput, "  Secrets: existing values stay in place; any missing required secrets will still be generated during env init.")
	fmt.Fprintln(interactivePromptOutput)

	return nil
}

func promptRequiredWizardValue(reader *bufio.Reader, label, defaultValue string, validate func(string) error) (string, error) {
	for {
		value, err := promptWizardValue(reader, label, defaultValue)
		if err != nil {
			return "", err
		}
		if value == "" {
			fmt.Fprintln(interactivePromptOutput, "  A value is required.")
			continue
		}
		if err := validate(value); err != nil {
			fmt.Fprintf(interactivePromptOutput, "  %s\n", err)
			continue
		}
		return value, nil
	}
}

func promptOptionalWizardValue(reader *bufio.Reader, label, defaultValue string, validate func(string) error) (string, error) {
	for {
		value, err := promptWizardValue(reader, label, defaultValue)
		if err != nil {
			return "", err
		}
		if value == "" {
			return "", nil
		}
		if err := validate(value); err != nil {
			fmt.Fprintf(interactivePromptOutput, "  %s\n", err)
			continue
		}
		return value, nil
	}
}

func promptWizardValue(reader *bufio.Reader, label, defaultValue string) (string, error) {
	if defaultValue != "" {
		fmt.Fprintf(interactivePromptOutput, "%s [%s]: ", label, defaultValue)
	} else {
		fmt.Fprintf(interactivePromptOutput, "%s: ", label)
	}

	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	if errors.Is(err, io.EOF) && line == "" {
		return "", io.EOF
	}

	value := strings.TrimSpace(line)
	if value == "" {
		value = strings.TrimSpace(defaultValue)
	}
	return value, nil
}

func promptWizardBool(reader *bufio.Reader, label string, defaultValue bool) (bool, error) {
	defaultHint := "y/N"
	if defaultValue {
		defaultHint = "Y/n"
	}

	for {
		fmt.Fprintf(interactivePromptOutput, "%s [%s]: ", label, defaultHint)
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return false, err
		}
		if errors.Is(err, io.EOF) && line == "" {
			return false, io.EOF
		}

		reply := strings.ToLower(strings.TrimSpace(line))
		if reply == "" {
			return defaultValue, nil
		}

		switch reply {
		case "y", "yes", "true":
			return true, nil
		case "n", "no", "false":
			return false, nil
		default:
			fmt.Fprintln(interactivePromptOutput, "  Please answer yes or no.")
		}
	}
}

func wizardAdminEmailDefault(existing map[string]string) string {
	current := strings.TrimSpace(existing["BITRIVER_LIVE_ADMIN_EMAIL"])
	if current != "" && !isForbiddenValue("BITRIVER_LIVE_ADMIN_EMAIL", current) {
		if err := validateWizardEmail(current); err == nil {
			return current
		}
	}
	return defaultEnvSecrets.adminEmail
}

func wizardPublicURLDefault(existing map[string]string, key string) string {
	current := strings.TrimSpace(existing[key])
	if current == "" {
		return ""
	}
	if err := validateWizardPublicURL(key, current); err != nil {
		return ""
	}
	return current
}

func wizardAPIPortDefault(existing map[string]string) string {
	if port := strings.TrimSpace(existing["BITRIVER_LIVE_PORT"]); port != "" {
		if err := validateWizardPort(port); err == nil {
			return port
		}
	}
	addr := strings.TrimSpace(existing["BITRIVER_LIVE_ADDR"])
	if strings.HasPrefix(addr, ":") {
		port := strings.TrimPrefix(addr, ":")
		if err := validateWizardPort(port); err == nil {
			return port
		}
	}
	return "8080"
}

func wizardHostDefault(existing map[string]string, key string) string {
	current := strings.TrimSpace(existing[key])
	if current == "" {
		return ""
	}
	if err := validateWizardHostValue(key, current); err != nil {
		return ""
	}
	return current
}

func wizardAllowSelfSignupDefault(existing map[string]string) bool {
	value := strings.ToLower(strings.TrimSpace(existing["BITRIVER_LIVE_ALLOW_SELF_SIGNUP"]))
	return value == "true" || value == "yes" || value == "1"
}

func validateWizardEmail(value string) error {
	if _, err := mail.ParseAddress(strings.TrimSpace(value)); err != nil {
		return fmt.Errorf("enter a valid email address")
	}
	return nil
}

func validateWizardPort(value string) error {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("enter a port between 1 and 65535")
	}
	return nil
}

func validateWizardPublicURL(fieldName, value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be a full http:// or https:// URL", fieldName)
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return fmt.Errorf("%s must use http or https", fieldName)
	}

	host := strings.ToLower(parsed.Hostname())
	if isWizardLoopbackHost(host) {
		return fmt.Errorf("%s must not point at localhost or another loopback address", fieldName)
	}
	if isWizardExampleHost(host) {
		return fmt.Errorf("%s must not use example.com placeholder values", fieldName)
	}

	return nil
}

func validateWizardHostValue(fieldName, value string) error {
	host := strings.TrimSpace(value)
	if host == "" {
		return fmt.Errorf("%s is required", fieldName)
	}
	if strings.Contains(host, "://") || strings.Contains(host, "/") || strings.Contains(host, " ") {
		return fmt.Errorf("%s must be a host or IP value, not a URL or path", fieldName)
	}

	lower := strings.ToLower(host)
	if isWizardLoopbackHost(lower) {
		return fmt.Errorf("%s must not use localhost or loopback placeholders", fieldName)
	}
	if isWizardExampleHost(lower) {
		return fmt.Errorf("%s must not use example.com placeholder values", fieldName)
	}

	return nil
}

func isWizardLoopbackHost(host string) bool {
	host = strings.TrimSpace(strings.ToLower(host))
	return host == "localhost" || host == "0.0.0.0" || host == "::" || host == "::1" || strings.HasPrefix(host, "127.")
}

func isWizardExampleHost(host string) bool {
	host = strings.TrimSpace(strings.ToLower(host))
	return host == "example.com" || strings.HasSuffix(host, ".example.com")
}

func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func copyEnvValues(values map[string]string) map[string]string {
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func printGeneratedSecrets(values map[string]string) {
	if len(values) == 0 {
		return
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Generated credentials (store these securely):")
	for _, key := range keys {
		fmt.Fprintf(os.Stdout, "  %s=%s\n", key, values[key])
	}
}

func defaultIfPlaceholder(key string, existing map[string]string, defaultValue string) string {
	if val, ok := existing[key]; ok && val != "" && !isForbiddenValue(key, val) {
		return val
	}
	return defaultValue
}

func isForbiddenValue(key, value string) bool {
	if placeholder, ok := forbiddenPlaceholders[key]; ok && strings.TrimSpace(value) == placeholder {
		return true
	}
	return false
}

func randomSecret() (string, error) {
	b := make([]byte, 24)
	if _, err := entropyRead(b); err != nil {
		return "", fmt.Errorf("read entropy for secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func randomSuffix() (string, error) {
	b := make([]byte, 6)
	if _, err := entropyRead(b); err != nil {
		return "", fmt.Errorf("read entropy for suffix: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

type envFileResolution struct {
	value   string
	error   string
	warning string
}

func resolveEnvValue(values map[string]string, key string) envFileResolution {
	directValue := strings.TrimSpace(values[key])
	fileKey := key + "_FILE"
	filePath := strings.TrimSpace(values[fileKey])

	if directValue != "" {
		if filePath != "" {
			return envFileResolution{
				value:   directValue,
				warning: fmt.Sprintf("%s and %s are both set; using %s and ignoring %s", key, fileKey, key, fileKey),
			}
		}
		return envFileResolution{value: directValue}
	}

	if filePath == "" {
		return envFileResolution{}
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return envFileResolution{error: fmt.Sprintf("%s points to %s but it cannot be read: %v", fileKey, filePath, err)}
	}

	trimmed := strings.TrimRight(string(content), "\r\n")
	if trimmed == "" {
		return envFileResolution{}
	}

	return envFileResolution{value: trimmed}
}

func validateEnv(values map[string]string) envValidatorResult {
	return validateEnvWithRuntimeMode(values, "")
}

func validateEnvWithRuntimeMode(values map[string]string, runtimeMode string) envValidatorResult {
	requiredVars := []string{
		"BITRIVER_POSTGRES_USER",
		"BITRIVER_POSTGRES_PASSWORD",
		"BITRIVER_REDIS_PASSWORD",
		"BITRIVER_OME_API",
		"BITRIVER_OME_PUBLIC_LLHLS_BASE_URL",
		"BITRIVER_OME_BIND",
		"BITRIVER_OME_IP",
		"BITRIVER_OME_SERVER_PORT",
		"BITRIVER_OME_SERVER_TLS_PORT",
		"BITRIVER_LIVE_ADMIN_EMAIL",
		"BITRIVER_LIVE_ADMIN_PASSWORD",
		"BITRIVER_LIVE_SESSION_TTL",
		"BITRIVER_LIVE_ALLOW_SELF_SIGNUP",
		"BITRIVER_SRS_TOKEN",
		"BITRIVER_SRS_PUBLIC_RTMP_BASE_URL",
		"BITRIVER_OME_USERNAME",
		"BITRIVER_OME_PASSWORD",
		"BITRIVER_OME_API_TOKEN",
		"BITRIVER_TRANSCODER_TOKEN",
		"BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD",
		"BITRIVER_TRANSCODER_PUBLIC_BASE_URL",
		"NEXT_PUBLIC_VIEWER_URL",
	}

	imageTags := []string{
		"BITRIVER_LIVE_IMAGE_TAG",
		"BITRIVER_VIEWER_IMAGE_TAG",
		"BITRIVER_SRS_CONTROLLER_IMAGE_TAG",
		"BITRIVER_TRANSCODER_IMAGE_TAG",
		"BITRIVER_SRS_IMAGE_TAG",
		"BITRIVER_OME_IMAGE_TAG",
	}

	modeRaw := strings.TrimSpace(values["BITRIVER_LIVE_MODE"])
	mode := strings.ToLower(modeRaw)

	res := envValidatorResult{}
	effectiveMode := mode
	if override := strings.ToLower(strings.TrimSpace(runtimeMode)); override != "" {
		switch override {
		case "production", "development":
			effectiveMode = override
		default:
			res.Errors = append(res.Errors, fmt.Sprintf("BITRIVER_LIVE_MODE runtime override must be production or development (current: %s)", strings.TrimSpace(runtimeMode)))
		}
	}
	production := effectiveMode == "production"
	effectiveValues := make(map[string]string, len(values))
	for key, value := range values {
		effectiveValues[key] = value
	}

	secretRequiredKeys := map[string]struct{}{
		"BITRIVER_POSTGRES_PASSWORD":              {},
		"BITRIVER_REDIS_PASSWORD":                 {},
		"BITRIVER_LIVE_ADMIN_PASSWORD":            {},
		"BITRIVER_SRS_TOKEN":                      {},
		"BITRIVER_OME_PASSWORD":                   {},
		"BITRIVER_OME_API_TOKEN":                  {},
		"BITRIVER_TRANSCODER_TOKEN":               {},
		"BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD": {},
		"BITRIVER_LIVE_METRICS_TOKEN":             {},
		"BITRIVER_OME_HEALTHCHECK_TOKEN":          {},
	}

	for key := range secretRequiredKeys {
		resolved := resolveEnvValue(values, key)
		if resolved.error != "" {
			res.Errors = append(res.Errors, resolved.error)
		}
		if resolved.warning != "" {
			res.Warnings = append(res.Warnings, resolved.warning)
		}
		if resolved.value != "" {
			effectiveValues[key] = resolved.value
		}
	}

	switch mode {
	case "":
		res.Errors = append(res.Errors, "BITRIVER_LIVE_MODE must be set to production in the environment file. Use an inline override (for example, BITRIVER_LIVE_MODE=development docker compose --env-file ./.env -f deploy/docker-compose.yml up) for temporary HTTP-only demos.")
	case "production":
		// Allowed.
	case "development":
		res.Errors = append(res.Errors, "BITRIVER_LIVE_MODE=development is not allowed in the saved environment file. Keep .env at production and override BITRIVER_LIVE_MODE only for one-off local demos.")
	default:
		res.Errors = append(res.Errors, fmt.Sprintf("BITRIVER_LIVE_MODE must be set to production (current: %s)", modeRaw))
	}

	if placeholderLoadErr != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("failed to load sample credentials from %s: %v", defaultExampleEnv(), placeholderLoadErr))
	}

	for _, key := range requiredVars {
		if strings.TrimSpace(effectiveValues[key]) == "" {
			res.Missing = append(res.Missing, key)
		}
	}

	for _, key := range imageTags {
		if strings.TrimSpace(values[key]) == "" {
			res.Missing = append(res.Missing, key)
		}
	}

	for key, placeholder := range forbiddenPlaceholders {
		if strings.TrimSpace(effectiveValues[key]) == placeholder {
			res.Blocked = append(res.Blocked, key)
		}
	}

	if effectiveValues["BITRIVER_REDIS_PASSWORD"] != "" && effectiveValues["BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD"] != "" &&
		effectiveValues["BITRIVER_REDIS_PASSWORD"] != effectiveValues["BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD"] {
		res.Warnings = append(res.Warnings, "BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD does not match BITRIVER_REDIS_PASSWORD. Ensure Redis credentials stay in sync unless intentionally different.")
	}

	tlsCert := strings.TrimSpace(values["BITRIVER_LIVE_TLS_CERT"])
	tlsKey := strings.TrimSpace(values["BITRIVER_LIVE_TLS_KEY"])

	if (tlsCert == "") != (tlsKey == "") {
		res.Errors = append(res.Errors, "BITRIVER_LIVE_TLS_CERT and BITRIVER_LIVE_TLS_KEY must both be set to enable HTTPS.")
	}

	httpsRequested := false
	for _, candidate := range []string{values["NEXT_PUBLIC_API_BASE_URL"], values["NEXT_PUBLIC_VIEWER_URL"]} {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(candidate)), "https://") {
			httpsRequested = true
			break
		}
	}

	if tlsCert != "" && tlsKey != "" {
		if _, err := os.Stat(tlsCert); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("BITRIVER_LIVE_TLS_CERT is set to %s but is not readable: %v", tlsCert, err))
		}
		if _, err := os.Stat(tlsKey); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("BITRIVER_LIVE_TLS_KEY is set to %s but is not readable: %v", tlsKey, err))
		}
	} else if httpsRequested {
		res.Errors = append(res.Errors, "HTTPS URLs are configured for the viewer or API, but BITRIVER_LIVE_TLS_CERT/BITRIVER_LIVE_TLS_KEY are empty. Provide TLS files or terminate HTTPS in front of the service and update the URLs accordingly.")
	}

	metricsToken := strings.TrimSpace(effectiveValues["BITRIVER_LIVE_METRICS_TOKEN"])
	metricsAllowNetworks := strings.TrimSpace(values["BITRIVER_LIVE_METRICS_ALLOW_NETWORKS"])

	if metricsToken == "" && metricsAllowNetworks == "" {
		message := "production mode requires protecting /metrics with BITRIVER_LIVE_METRICS_TOKEN or BITRIVER_LIVE_METRICS_ALLOW_NETWORKS"
		if production {
			res.Errors = append(res.Errors, message)
		} else {
			res.Warnings = append(res.Warnings, message)
		}
	}

	loginLimitRaw := strings.TrimSpace(values["BITRIVER_LIVE_RATE_LOGIN_LIMIT"])
	loginLimit := 0
	if loginLimitRaw != "" {
		parsed, err := strconv.Atoi(loginLimitRaw)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("BITRIVER_LIVE_RATE_LOGIN_LIMIT must be an integer (current: %s)", loginLimitRaw))
		} else {
			loginLimit = parsed
		}
	}

	if production && (loginLimit <= 0 || loginLimitRaw == "") {
		res.Errors = append(res.Errors, "production mode requires non-zero login throttling; set BITRIVER_LIVE_RATE_LOGIN_LIMIT")
	}

	for _, key := range []string{"BITRIVER_LIVE_POSTGRES_DSN", "BITRIVER_LIVE_SESSION_POSTGRES_DSN"} {
		if val := strings.TrimSpace(values[key]); val != "" && pgdsn.SSLModeDisable(val) && !pgdsn.IsComposePostgresDSN(val) {
			message := fmt.Sprintf("%s disables TLS. Use sslmode=require or verify-full for external Postgres; sslmode=disable is only allowed for the local Compose postgres service.", key)
			if production {
				res.Errors = append(res.Errors, message)
			} else {
				res.Warnings = append(res.Warnings, message)
			}
		}
	}

	if profiles := strings.TrimSpace(values["COMPOSE_PROFILES"]); profiles != "" {
		for _, profile := range strings.FieldsFunc(profiles, func(r rune) bool { return r == ',' || r == ':' }) {
			if profile == "postgres-host" {
				res.Warnings = append(res.Warnings, "COMPOSE_PROFILES includes postgres-host, which publishes PostgreSQL to the host.")
				break
			}
		}
	}

	if val := strings.TrimSpace(values["BITRIVER_OME_IMAGE_TAG"]); val != "" {
		parts := strings.Split(val, ".")
		if len(parts) != 3 {
			res.Errors = append(res.Errors, fmt.Sprintf("BITRIVER_OME_IMAGE_TAG must be MAJOR.MINOR.PATCH so the renderer can stamp the config (current: %s)", val))
		} else {
			if parts[0] == "0" {
				minor, _ := strconv.Atoi(parts[1])
				if minor < 16 {
					res.Errors = append(res.Errors, fmt.Sprintf("BITRIVER_OME_IMAGE_TAG must be 0.16.0 or newer to match the rendered Server.xml schema (current: %s).", val))
				}
			}
		}
	}

	authMode := normalizedOMEHealthcheckAuthMode(values["BITRIVER_OME_HEALTHCHECK_AUTH_MODE"])
	if authMode != "accesstoken" && authMode != "basic" {
		res.Errors = append(res.Errors, fmt.Sprintf("BITRIVER_OME_HEALTHCHECK_AUTH_MODE must be accesstoken or basic (current: %s)", strings.TrimSpace(values["BITRIVER_OME_HEALTHCHECK_AUTH_MODE"])))
	}

	if val := strings.TrimSpace(values["BITRIVER_SRS_IMAGE_TAG"]); val != "" && val != "v5.0.185" {
		res.Warnings = append(res.Warnings, fmt.Sprintf("BITRIVER_SRS_IMAGE_TAG is set to %s. Update systemd docs or units before upgrading.", val))
	}

	if val := strings.TrimSpace(values["BITRIVER_OME_SERVER_PORT"]); val != "" {
		if portErr := validatePort(val, "BITRIVER_OME_SERVER_PORT"); portErr != "" {
			res.Errors = append(res.Errors, portErr)
		}
	}

	if val := strings.TrimSpace(values["BITRIVER_OME_SERVER_TLS_PORT"]); val != "" {
		if portErr := validatePort(val, "BITRIVER_OME_SERVER_TLS_PORT"); portErr != "" {
			res.Errors = append(res.Errors, portErr)
		}
	}

	if val := strings.TrimSpace(values["BITRIVER_OME_LLHLS_PORT"]); val != "" {
		if portErr := validatePort(val, "BITRIVER_OME_LLHLS_PORT"); portErr != "" {
			res.Errors = append(res.Errors, portErr)
		}
	}

	if val := strings.TrimSpace(values["BITRIVER_OME_LLHLS_TLS_PORT"]); val != "" {
		if portErr := validatePort(val, "BITRIVER_OME_LLHLS_TLS_PORT"); portErr != "" {
			res.Errors = append(res.Errors, portErr)
		}
	}

	if v := strings.TrimSpace(values["BITRIVER_LIVE_ALLOW_SELF_SIGNUP"]); v != "" {
		lower := strings.ToLower(v)
		if lower != "true" && lower != "false" {
			res.Errors = append(res.Errors, fmt.Sprintf("BITRIVER_LIVE_ALLOW_SELF_SIGNUP must be true or false (current: %s)", v))
		}
	}

	if val := strings.TrimSpace(values["BITRIVER_LIVE_POSTGRES_DSN"]); strings.Contains(val, "bitriver:bitriver") {
		res.Warnings = append(res.Warnings, "BITRIVER_LIVE_POSTGRES_DSN still references bitriver:bitriver. Update or unset it to match the Postgres credentials.")
	}

	loopback := regexp.MustCompile(`^https?://(localhost|127\.[0-9.]*|0\.0\.0\.0|::1|\[::1\])([:/]|$)`)
	loopbackRTMP := regexp.MustCompile(`^rtmp://(localhost|127\.[0-9.]*|0\.0\.0\.0|::1|\[::1\])([:/]|$)`)
	localQuickstart := !production &&
		loopback.MatchString(strings.TrimSpace(values["NEXT_PUBLIC_VIEWER_URL"])) &&
		loopback.MatchString(strings.TrimSpace(values["BITRIVER_TRANSCODER_PUBLIC_BASE_URL"])) &&
		loopback.MatchString(strings.TrimSpace(values["BITRIVER_OME_PUBLIC_LLHLS_BASE_URL"])) &&
		loopbackRTMP.MatchString(strings.TrimSpace(values["BITRIVER_SRS_PUBLIC_RTMP_BASE_URL"]))
	localQuickstartMessage := " This is expected for first-run Docker Desktop quickstart demos and remains a non-fatal warning, but you must replace it with a public/routable value before any internet-facing or production deployment."

	flagEnvIssue := func(message string) {
		if localQuickstart {
			res.Warnings = append(res.Warnings, message+localQuickstartMessage)
			return
		}
		if production {
			res.Errors = append(res.Errors, message)
		} else {
			res.Warnings = append(res.Warnings, message)
		}
	}

	if val := strings.TrimSpace(values["BITRIVER_TRANSCODER_PUBLIC_BASE_URL"]); val != "" {
		switch {
		case val == "https://cdn.example.com/hls":
			res.Errors = append(res.Errors, "BITRIVER_TRANSCODER_PUBLIC_BASE_URL still uses the sample CDN URL (https://cdn.example.com/hls). Replace it with the public origin end users can reach.")
		case loopback.MatchString(val):
			flagEnvIssue(fmt.Sprintf("BITRIVER_TRANSCODER_PUBLIC_BASE_URL points at loopback (%s). Configure a CDN, reverse proxy, or routable origin instead.", val))
		}
	}

	if val := strings.TrimSpace(values["BITRIVER_OME_API"]); val != "" && loopback.MatchString(val) {
		flagEnvIssue(fmt.Sprintf("BITRIVER_OME_API points at loopback (%s). Use the ome hostname from docker-compose.yml or another reachable host/IP.", val))
	}

	if val := strings.TrimSpace(values["BITRIVER_OME_PUBLIC_LLHLS_BASE_URL"]); val != "" && loopback.MatchString(val) {
		flagEnvIssue(fmt.Sprintf("BITRIVER_OME_PUBLIC_LLHLS_BASE_URL points at loopback (%s). Configure the HTTPS LL-HLS path viewers can reach.", val))
	}

	if val := strings.TrimSpace(values["BITRIVER_SRS_PUBLIC_RTMP_BASE_URL"]); val != "" && loopbackRTMP.MatchString(val) {
		flagEnvIssue(fmt.Sprintf("BITRIVER_SRS_PUBLIC_RTMP_BASE_URL points at loopback (%s). Configure the RTMP hostname or IP creators can reach.", val))
	}

	if val := strings.TrimSpace(values["NEXT_PUBLIC_API_BASE_URL"]); val != "" {
		switch {
		case loopback.MatchString(val):
			flagEnvIssue(fmt.Sprintf("NEXT_PUBLIC_API_BASE_URL points at loopback (%s). Point it at the API hostname end users reach.", val))
		case strings.Contains(val, "example.com"):
			res.Errors = append(res.Errors, fmt.Sprintf("NEXT_PUBLIC_API_BASE_URL still uses an example.com placeholder (%s). Replace it with the production API origin.", val))
		}
	} else {
		viewerBasePath := values["NEXT_VIEWER_BASE_PATH"]
		if viewerBasePath == "" {
			viewerBasePath = "/viewer"
		}
		res.Warnings = append(res.Warnings, fmt.Sprintf("NEXT_PUBLIC_API_BASE_URL is empty; the viewer will fall back to the API origin when proxied at NEXT_VIEWER_BASE_PATH=%s.", viewerBasePath))
	}

	for key, raw := range values {
		val := strings.TrimSpace(raw)
		if val == "" || !strings.HasPrefix(key, "BITRIVER_") {
			continue
		}

		switch {
		case strings.HasSuffix(key, "_CPUS"):
			parsed, err := strconv.ParseFloat(val, 64)
			if err != nil || parsed <= 0 {
				res.Errors = append(res.Errors, fmt.Sprintf("%s must be a positive decimal CPU value (current: %s)", key, val))
			}
		case strings.HasSuffix(key, "_MEM"), strings.HasSuffix(key, "_MEM_RESERVATION"):
			if !isComposeMemoryValue(val) {
				res.Errors = append(res.Errors, fmt.Sprintf("%s must be a Docker Compose memory size (examples: 256m, 1g) (current: %s)", key, val))
			}
		}
	}

	if val := strings.TrimSpace(values["NEXT_PUBLIC_VIEWER_URL"]); val != "" {
		switch {
		case loopback.MatchString(val):
			flagEnvIssue(fmt.Sprintf("NEXT_PUBLIC_VIEWER_URL points at loopback (%s). Point it at the viewer hostname end users reach.", val))
		case strings.Contains(val, "example.com"):
			res.Errors = append(res.Errors, fmt.Sprintf("NEXT_PUBLIC_VIEWER_URL still uses an example.com placeholder (%s). Replace it with the production viewer origin.", val))
		}
	}

	return res
}

func isComposeMemoryValue(value string) bool {
	memPattern := regexp.MustCompile(`(?i)^\d+(?:\.\d+)?(?:[kmgt]i?b?|[kmgt]|b)?$`)
	return memPattern.MatchString(strings.TrimSpace(value))
}

func validatePort(value, name string) string {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Sprintf("%s must be a valid TCP port number (current: %s)", name, value)
	}

	return ""
}

func productionSafeMode(value string) string {
	mode := strings.ToLower(strings.TrimSpace(value))
	if mode == "" || mode == "development" || mode == "placeholder" || mode == "example" || mode == "changeme" || isForbiddenValue("BITRIVER_LIVE_MODE", mode) {
		return "production"
	}
	return strings.TrimSpace(value)
}
