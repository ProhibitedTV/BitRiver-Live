package main

import (
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// OME rendering is isolated in this file because it is the most implementation-
// dense portion of the CLI. New developers can learn command flow in
// commands_env_compose.go first, then deep-dive here when they need to evolve
// the XML templating behavior.

func runOME(args []string) error {
	if len(args) == 0 {
		return errors.New("ome subcommand required")
	}

	switch args[0] {
	case "render":
		return runOMERender(args[1:])
	default:
		return fmt.Errorf("unknown ome subcommand: %s", args[0])
	}
}

func runOMERender(args []string) error {
	fs := flag.NewFlagSet("ome render", flag.ContinueOnError)
	envPath := fs.String("env-file", defaultEnvFile(), "path to env file")
	force := fs.Bool("force", false, "force regeneration")
	checkOnly := fs.Bool("check", false, "only verify the file exists")
	quiet := fs.Bool("quiet", false, "suppress informational output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	return renderOMEFromEnv(*envPath, *force, *checkOnly, *quiet)
}

func renderOMEFromEnv(envPath string, force, checkOnly, quiet bool) error {
	templatePath := filepath.Join(repoRoot(), "deploy", "ome", "Server.xml")
	outputPath := filepath.Join(repoRoot(), "deploy", "ome", "Server.generated.xml")

	if checkOnly {
		if err := validateOMEGeneratedConfig(outputPath); err != nil {
			return err
		}
		if !quiet {
			fmt.Fprintf(os.Stdout, "OME config found at %s.\n", outputPath)
		}
		return nil
	}

	values, err := loadEnvValues(envPath, false)
	if err != nil {
		return err
	}

	cfg, err := buildOMERenderConfig(values, templatePath, outputPath)
	if err != nil {
		return err
	}

	if !force {
		if _, err := os.Stat(outputPath); err == nil {
			if err := validateOMEGeneratedConfig(outputPath); err != nil {
				return fmt.Errorf("invalid OME config at %s (re-render with --force): %w", outputPath, err)
			}
			if !quiet {
				fmt.Fprintf(os.Stdout, "OME config already exists at %s (use --force to regenerate).\n", outputPath)
			}
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to check generated config: %w", err)
		}
	}

	if !quiet {
		if force {
			fmt.Fprintln(os.Stdout, "Rendering OME config (--force requested)...")
		} else {
			fmt.Fprintln(os.Stdout, "Rendering OME config...")
		}
	}

	if err := renderOMEConfig(cfg); err != nil {
		return fmt.Errorf("render deploy/ome/Server.generated.xml: %w", err)
	}

	if err := validateOMEGeneratedConfig(outputPath); err != nil {
		return fmt.Errorf("validate generated OME config: %w", err)
	}

	if !quiet {
		fmt.Fprintf(os.Stdout, "Rendered OME configuration to %s\n", outputPath)
	}

	return nil
}

type omeRenderConfig struct {
	TemplatePath string
	OutputPath   string
	Bind         string
	ServerIP     string
	Port         string
	TLSPort      string
	LLHLSPort    string
	LLHLSTLSPort string
	Username     string
	Password     string
	APIToken     string
	AccessToken  string
	ImageTag     string
	TCPRelay     string
	ICECandidate string
}

var omeTestDefaults = map[string]string{
	"BITRIVER_OME_API_TOKEN": "ome-test-access-token",
	// BITRIVER_OME_ACCESS_TOKEN falls back to BITRIVER_OME_API_TOKEN when unset.
	"BITRIVER_OME_ACCESS_TOKEN": "ome-test-access-token",
}

func buildOMERenderConfig(values map[string]string, templatePath, outputPath string) (omeRenderConfig, error) {
	if _, err := os.Stat(templatePath); err != nil {
		return omeRenderConfig{}, fmt.Errorf("OME template missing at %s: %w", templatePath, err)
	}

	bind := firstNonEmpty(strings.TrimSpace(values["BITRIVER_OME_BIND"]), "0.0.0.0")
	port := firstNonEmpty(strings.TrimSpace(values["BITRIVER_OME_SERVER_PORT"]), "9000")
	tlsPort := firstNonEmpty(strings.TrimSpace(values["BITRIVER_OME_SERVER_TLS_PORT"]), "9443")
	llhlsPort := firstNonEmpty(strings.TrimSpace(values["BITRIVER_OME_LLHLS_PORT"]), "8080")
	llhlsTLSPort := firstNonEmpty(strings.TrimSpace(values["BITRIVER_OME_LLHLS_TLS_PORT"]), "8443")
	ip := firstNonEmpty(strings.TrimSpace(values["BITRIVER_OME_IP"]), bind)
	imageTag := firstNonEmpty(strings.TrimSpace(values["BITRIVER_OME_IMAGE_TAG"]), "0.16.0")
	icePortRange := firstNonEmpty(strings.TrimSpace(values["BITRIVER_OME_ICE_PORT_RANGE"]), "10000-10009")
	tcpRelay := firstNonEmpty(strings.TrimSpace(values["BITRIVER_OME_TCP_RELAY"]), strings.TrimSpace(values["BITRIVER_OME_RELAY_PORT"]), "3478")
	if !strings.Contains(tcpRelay, ":") {
		tcpRelay = "*:" + strings.Trim(tcpRelay, "*:")
	}
	iceCandidate := strings.TrimSpace(values["BITRIVER_OME_ICE_CANDIDATE"])
	if iceCandidate == "" {
		iceCandidate = fmt.Sprintf("*:%s/udp", icePortRange)
	}

	apiToken := strings.TrimSpace(values["BITRIVER_OME_API_TOKEN"])
	accessToken := strings.TrimSpace(values["BITRIVER_OME_ACCESS_TOKEN"])
	if accessToken == "" {
		accessToken = apiToken
	}

	missing := make([]string, 0)
	for key, value := range map[string]string{
		"BITRIVER_OME_API_TOKEN":       apiToken,
		"BITRIVER_OME_SERVER_PORT":     port,
		"BITRIVER_OME_SERVER_TLS_PORT": tlsPort,
	} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return omeRenderConfig{}, fmt.Errorf("missing required OME variables: %s", strings.Join(missing, ", "))
	}

	for key, forbidden := range omeTestDefaults {
		switch key {
		case "BITRIVER_OME_ACCESS_TOKEN":
			if strings.TrimSpace(accessToken) == forbidden {
				return omeRenderConfig{}, fmt.Errorf("%s is set to ome-test-* default; provide deployment credentials before rendering", key)
			}
		default:
			if strings.TrimSpace(values[key]) == forbidden {
				return omeRenderConfig{}, fmt.Errorf("%s is set to ome-test-* default; provide deployment credentials before rendering", key)
			}
		}
	}

	return omeRenderConfig{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Bind:         bind,
		ServerIP:     ip,
		Port:         port,
		TLSPort:      tlsPort,
		LLHLSPort:    llhlsPort,
		LLHLSTLSPort: llhlsTLSPort,
		APIToken:     apiToken,
		AccessToken:  accessToken,
		ImageTag:     imageTag,
		TCPRelay:     tcpRelay,
		ICECandidate: iceCandidate,
	}, nil
}

func renderOMEConfig(cfg omeRenderConfig) error {
	data, err := os.ReadFile(cfg.TemplatePath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	text := string(data)
	info, err := scanOMETemplateInfo(text)
	if err != nil {
		return err
	}

	replaced, err := rewriteOMEConfig(text, cfg, info)
	if err != nil {
		return err
	}

	replaced = stampImageTag(replaced, cfg.ImageTag)
	replaced = collapseBlankLines(replaced)

	if err := os.WriteFile(cfg.OutputPath, []byte(replaced), 0o644); err != nil {
		return fmt.Errorf("write generated config: %w", err)
	}

	return nil
}

type omeTemplateInfo struct {
	hasBindTag            bool
	rootBindHasSignalling bool
}

type xmlTokenKind int

const (
	xmlTokenCharData xmlTokenKind = iota
	xmlTokenStartTag
	xmlTokenEndTag
	xmlTokenComment
	xmlTokenDirective
)

type xmlToken struct {
	kind        xmlTokenKind
	raw         string
	name        string
	selfClosing bool
}

type xmlScanner struct {
	data string
	pos  int
}

func newXMLScanner(data string) *xmlScanner {
	return &xmlScanner{data: data}
}

func (s *xmlScanner) next() (xmlToken, bool, error) {
	if s.pos >= len(s.data) {
		return xmlToken{}, false, nil
	}

	if s.data[s.pos] != '<' {
		next := strings.IndexByte(s.data[s.pos:], '<')
		if next == -1 {
			next = len(s.data)
		} else {
			next += s.pos
		}
		raw := s.data[s.pos:next]
		s.pos = next
		return xmlToken{kind: xmlTokenCharData, raw: raw}, true, nil
	}

	if strings.HasPrefix(s.data[s.pos:], "<!--") {
		end := strings.Index(s.data[s.pos+4:], "-->")
		if end == -1 {
			return xmlToken{}, false, errors.New("unterminated XML comment")
		}
		end = s.pos + 4 + end + 3
		raw := s.data[s.pos:end]
		s.pos = end
		return xmlToken{kind: xmlTokenComment, raw: raw}, true, nil
	}

	if strings.HasPrefix(s.data[s.pos:], "<![CDATA[") {
		end := strings.Index(s.data[s.pos+9:], "]]>")
		if end == -1 {
			return xmlToken{}, false, errors.New("unterminated XML CDATA section")
		}
		end = s.pos + 9 + end + 3
		raw := s.data[s.pos:end]
		s.pos = end
		return xmlToken{kind: xmlTokenDirective, raw: raw}, true, nil
	}

	if strings.HasPrefix(s.data[s.pos:], "<?") {
		end := strings.Index(s.data[s.pos+2:], "?>")
		if end == -1 {
			return xmlToken{}, false, errors.New("unterminated XML directive")
		}
		end = s.pos + 2 + end + 2
		raw := s.data[s.pos:end]
		s.pos = end
		return xmlToken{kind: xmlTokenDirective, raw: raw}, true, nil
	}

	if strings.HasPrefix(s.data[s.pos:], "<!") {
		end, err := findTagEnd(s.data, s.pos)
		if err != nil {
			return xmlToken{}, false, err
		}
		raw := s.data[s.pos : end+1]
		s.pos = end + 1
		return xmlToken{kind: xmlTokenDirective, raw: raw}, true, nil
	}

	end, err := findTagEnd(s.data, s.pos)
	if err != nil {
		return xmlToken{}, false, err
	}
	raw := s.data[s.pos : end+1]
	s.pos = end + 1
	name, isEnd, selfClosing := parseTag(raw)
	if isEnd {
		return xmlToken{kind: xmlTokenEndTag, raw: raw, name: name}, true, nil
	}
	return xmlToken{kind: xmlTokenStartTag, raw: raw, name: name, selfClosing: selfClosing}, true, nil
}

func findTagEnd(data string, start int) (int, error) {
	var quote byte
	for i := start + 1; i < len(data); i++ {
		ch := data[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			quote = ch
			continue
		}
		if ch == '>' {
			return i, nil
		}
	}
	return -1, errors.New("unterminated XML tag")
}

func parseTag(raw string) (string, bool, bool) {
	if len(raw) < 3 {
		return "", false, false
	}
	content := strings.TrimSpace(raw[1 : len(raw)-1])
	if content == "" {
		return "", false, false
	}
	isEnd := strings.HasPrefix(content, "/")
	if isEnd {
		content = strings.TrimSpace(content[1:])
	}
	nameEnd := 0
	for nameEnd < len(content) && !isSpace(content[nameEnd]) && content[nameEnd] != '/' {
		nameEnd++
	}
	name := content[:nameEnd]
	selfClosing := !isEnd && strings.HasSuffix(strings.TrimSpace(content), "/")
	return name, isEnd, selfClosing
}

func renameTag(raw, newName string) string {
	if len(raw) < 3 {
		return raw
	}
	idx := 1
	for idx < len(raw) && isSpace(raw[idx]) {
		idx++
	}
	if idx < len(raw) && raw[idx] == '/' {
		idx++
		for idx < len(raw) && isSpace(raw[idx]) {
			idx++
		}
	}
	nameStart := idx
	for idx < len(raw) && !isSpace(raw[idx]) && raw[idx] != '/' && raw[idx] != '>' {
		idx++
	}
	nameEnd := idx
	if nameStart >= len(raw) {
		return raw
	}
	return raw[:nameStart] + newName + raw[nameEnd:]
}

func isSpace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func scanOMETemplateInfo(text string) (omeTemplateInfo, error) {
	scanner := newXMLScanner(text)
	var stack []string
	rootServerDepth := -1
	rootBindDepth := -1
	controlDepth := -1

	info := omeTemplateInfo{}
	for {
		token, ok, err := scanner.next()
		if err != nil {
			return omeTemplateInfo{}, err
		}
		if !ok {
			break
		}
		switch token.kind {
		case xmlTokenStartTag:
			name := token.name
			if name == "Bind" || name == "Server.bind" {
				info.hasBindTag = true
			}
			if name == "Control" {
				controlDepth = len(stack) + 1
			}
			if name == "Server" && controlDepth == -1 && rootServerDepth == -1 {
				rootServerDepth = len(stack) + 1
			}
			if name == "Bind" || name == "Server.bind" {
				if rootServerDepth != -1 && len(stack) >= rootServerDepth && controlDepth == -1 {
					rootBindDepth = len(stack) + 1
				}
			}
			if name == "Signalling" && rootBindDepth != -1 && len(stack) >= rootBindDepth {
				info.rootBindHasSignalling = true
			}
			if !token.selfClosing {
				stack = append(stack, name)
			}
		case xmlTokenEndTag:
			name := token.name
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			if controlDepth != -1 && len(stack) < controlDepth {
				controlDepth = -1
			}
			if rootBindDepth != -1 && len(stack) < rootBindDepth {
				rootBindDepth = -1
			}
			if rootServerDepth != -1 && len(stack) < rootServerDepth {
				rootServerDepth = -1
			}
			if name == "Control" {
				controlDepth = -1
			}
		}
	}

	return info, nil
}

type llhlsState struct {
	depth           int
	portReplaced    bool
	tlsPortReplaced bool
}

type iceCandidatesState struct {
	depth           int
	hasTcpRelay     bool
	hasIceCandidate bool
}

type replaceState struct {
	active      bool
	tagName     string
	depth       int
	endOverride string
	dropEnd     bool
}

func rewriteOMEConfig(text string, cfg omeRenderConfig, info omeTemplateInfo) (string, error) {
	scanner := newXMLScanner(text)
	var out strings.Builder
	lineTail := ""
	write := func(value string) {
		out.WriteString(value)
		lineTail = updateLineTail(lineTail, value)
	}
	var stack []string
	rootServerDepth := -1
	rootBindDepth := -1
	controlDepth := -1
	controlServerDepth := -1
	topLevelManagersDepth := -1
	topLevelManagersAPIDepth := -1
	bindManagersDepth := -1
	bindManagersAPIDepth := -1
	virtualHostsDepth := -1
	publishersDepth := -1

	var llhlsStack []llhlsState
	var iceCandidatesStack []iceCandidatesState

	rootServerFound := false
	rootBindFound := false
	publishersFound := false
	llhlsFound := false
	iceCandidatesFound := 0
	accessTokenReplaced := false
	bindManagersPortReplaced := false
	bindManagersTLSPortReplaced := false
	rootServerIPReplaced := false
	topLevelManagersAPIFound := false
	bindManagersAPIFound := false

	bindValue := xmlEscape(cfg.Bind)
	portValue := xmlEscape(cfg.Port)
	tlsPortValue := xmlEscape(cfg.TLSPort)
	llhlsPortValue := xmlEscape(cfg.LLHLSPort)
	llhlsTLSPortValue := xmlEscape(cfg.LLHLSTLSPort)
	serverIPValue := xmlEscape(cfg.ServerIP)
	tcpRelayValue := xmlEscape(cfg.TCPRelay)
	iceCandidateValue := xmlEscape(cfg.ICECandidate)
	accessTokenValue := xmlEscape(cfg.AccessToken)

	replacement := replaceState{}

	for {
		token, ok, err := scanner.next()
		if err != nil {
			return "", err
		}
		if !ok {
			break
		}

		if replacement.active {
			switch token.kind {
			case xmlTokenStartTag:
				if token.name == replacement.tagName && !token.selfClosing {
					replacement.depth++
				}
			case xmlTokenEndTag:
				if token.name == replacement.tagName {
					replacement.depth--
					if replacement.depth == 0 {
						if replacement.endOverride != "" {
							write(replacement.endOverride)
						} else if !replacement.dropEnd {
							write(token.raw)
						}
						replacement.active = false
					}
				}
			}
			continue
		}

		switch token.kind {
		case xmlTokenCharData, xmlTokenComment, xmlTokenDirective:
			write(token.raw)
		case xmlTokenStartTag:
			name := token.name
			raw := token.raw

			if name == "Server.bind" {
				name = "Bind"
				raw = renameTag(raw, name)
			}

			if name == "Control" {
				controlDepth = len(stack) + 1
			}
			if name == "Server" && controlDepth == -1 && rootServerDepth == -1 {
				rootServerDepth = len(stack) + 1
				rootServerFound = true
			}
			if name == "Server" && controlDepth != -1 && controlServerDepth == -1 {
				controlServerDepth = len(stack) + 1
			}
			if name == "Managers" && rootServerDepth != -1 && len(stack) >= rootServerDepth && controlDepth == -1 {
				if rootBindDepth != -1 && len(stack) >= rootBindDepth {
					bindManagersDepth = len(stack) + 1
				} else {
					topLevelManagersDepth = len(stack) + 1
				}
			}
			if name == "API" {
				if topLevelManagersDepth != -1 && len(stack) >= topLevelManagersDepth {
					topLevelManagersAPIDepth = len(stack) + 1
					topLevelManagersAPIFound = true
				}
				if bindManagersDepth != -1 && len(stack) >= bindManagersDepth {
					bindManagersAPIDepth = len(stack) + 1
					bindManagersAPIFound = true
				}
			}
			if name == "VirtualHosts" && rootServerDepth != -1 && len(stack) >= rootServerDepth && controlDepth == -1 {
				virtualHostsDepth = len(stack) + 1
			}
			if name == "Bind" && rootServerDepth != -1 && len(stack) >= rootServerDepth && controlDepth == -1 {
				rootBindDepth = len(stack) + 1
				rootBindFound = true
			}
			if name == "Publishers" && rootBindDepth != -1 && len(stack) >= rootBindDepth {
				publishersDepth = len(stack) + 1
				publishersFound = true
			}
			if name == "LLHLS" && publishersDepth != -1 && len(stack) >= publishersDepth {
				llhlsDepth := len(stack) + 1
				llhlsStack = append(llhlsStack, llhlsState{depth: llhlsDepth})
				llhlsFound = true
			}
			if name == "IceCandidates" {
				iceCandidatesDepth := len(stack) + 1
				iceCandidatesStack = append(iceCandidatesStack, iceCandidatesState{depth: iceCandidatesDepth})
				iceCandidatesFound++
			}

			inControlServer := controlServerDepth != -1 && len(stack) >= controlServerDepth
			inTopLevelManagersAPI := topLevelManagersAPIDepth != -1 && len(stack) >= topLevelManagersAPIDepth
			inBindManagersAPI := bindManagersAPIDepth != -1 && len(stack) >= bindManagersAPIDepth
			inRootServer := rootServerDepth != -1 && len(stack) >= rootServerDepth && controlDepth == -1
			inRootBind := rootBindDepth != -1 && len(stack) >= rootBindDepth
			inVirtualHosts := virtualHostsDepth != -1 && len(stack) >= virtualHostsDepth

			if inRootBind && (name == "Address" || name == "IP" || name == "Server.bind.Address") {
				if token.selfClosing {
					continue
				}
				replacement = replaceState{active: true, tagName: name, depth: 1, dropEnd: true}
				continue
			}

			if inControlServer && (name == "Bind" || name == "IP" || name == "Address") {
				replacement = replaceState{active: true, tagName: name, depth: 1}
				write(raw)
				write(bindValue)
				continue
			}

			if name == "IP" && inRootServer && !inRootBind && !inVirtualHosts && !rootServerIPReplaced {
				replacement = replaceState{active: true, tagName: name, depth: 1}
				write(raw)
				write(serverIPValue)
				rootServerIPReplaced = true
				continue
			}

			if name == "Port" || name == "TLSPort" {
				if len(llhlsStack) > 0 && len(stack) >= llhlsStack[len(llhlsStack)-1].depth {
					state := &llhlsStack[len(llhlsStack)-1]
					if name == "Port" && !state.portReplaced {
						replacement = replaceState{active: true, tagName: name, depth: 1}
						write(raw)
						write(llhlsPortValue)
						state.portReplaced = true
						continue
					}
					if name == "TLSPort" && !state.tlsPortReplaced {
						replacement = replaceState{active: true, tagName: name, depth: 1}
						write(raw)
						write(llhlsTLSPortValue)
						state.tlsPortReplaced = true
						continue
					}
				}
			}

			if inBindManagersAPI {
				if name == "Port" && !bindManagersPortReplaced {
					replacement = replaceState{active: true, tagName: name, depth: 1}
					write(raw)
					write(portValue)
					bindManagersPortReplaced = true
					continue
				}
				if name == "TLSPort" && !bindManagersTLSPortReplaced {
					replacement = replaceState{active: true, tagName: name, depth: 1}
					write(raw)
					write(tlsPortValue)
					bindManagersTLSPortReplaced = true
					continue
				}
			}

			if name == "TcpRelay" && len(iceCandidatesStack) > 0 {
				state := &iceCandidatesStack[len(iceCandidatesStack)-1]
				replacement = replaceState{active: true, tagName: name, depth: 1}
				write(raw)
				write(tcpRelayValue)
				state.hasTcpRelay = true
				continue
			}
			if name == "IceCandidate" && len(iceCandidatesStack) > 0 {
				state := &iceCandidatesStack[len(iceCandidatesStack)-1]
				replacement = replaceState{active: true, tagName: name, depth: 1}
				write(raw)
				write(iceCandidateValue)
				state.hasIceCandidate = true
				continue
			}

			if inTopLevelManagersAPI && name == "AccessToken" && !accessTokenReplaced {
				replacement = replaceState{active: true, tagName: name, depth: 1}
				write(raw)
				write(accessTokenValue)
				accessTokenReplaced = true
				continue
			}

			write(raw)
			if !token.selfClosing {
				stack = append(stack, name)
			}
		case xmlTokenEndTag:
			name := token.name
			raw := token.raw

			if name == "Server.bind" {
				name = "Bind"
				raw = renameTag(raw, name)
			}

			if name == "IceCandidates" && len(iceCandidatesStack) > 0 {
				state := iceCandidatesStack[len(iceCandidatesStack)-1]
				if !state.hasTcpRelay || !state.hasIceCandidate {
					indent := "    "
					if lineTail != "" {
						ws := lineTail
						indent = ""
						for i := 0; i < len(ws); i++ {
							if ws[i] != ' ' && ws[i] != '\t' {
								break
							}
							indent += string(ws[i])
						}
					}
					childIndent := indent + "    "
					if !state.hasTcpRelay {
						write("\n" + childIndent + "<TcpRelay>" + tcpRelayValue + "</TcpRelay>")
					}
					if !state.hasIceCandidate {
						write("\n" + childIndent + "<IceCandidate>" + iceCandidateValue + "</IceCandidate>")
					}
				}
				iceCandidatesStack = iceCandidatesStack[:len(iceCandidatesStack)-1]
			}

			write(raw)

			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			if controlDepth != -1 && len(stack) < controlDepth {
				controlDepth = -1
				controlServerDepth = -1
			}
			if topLevelManagersDepth != -1 && len(stack) < topLevelManagersDepth {
				topLevelManagersDepth = -1
			}
			if topLevelManagersAPIDepth != -1 && len(stack) < topLevelManagersAPIDepth {
				topLevelManagersAPIDepth = -1
			}
			if bindManagersDepth != -1 && len(stack) < bindManagersDepth {
				bindManagersDepth = -1
			}
			if bindManagersAPIDepth != -1 && len(stack) < bindManagersAPIDepth {
				bindManagersAPIDepth = -1
			}
			if controlServerDepth != -1 && len(stack) < controlServerDepth {
				controlServerDepth = -1
			}
			if rootBindDepth != -1 && len(stack) < rootBindDepth {
				rootBindDepth = -1
			}
			if publishersDepth != -1 && len(stack) < publishersDepth {
				publishersDepth = -1
			}
			if virtualHostsDepth != -1 && len(stack) < virtualHostsDepth {
				virtualHostsDepth = -1
			}
			if len(llhlsStack) > 0 && len(stack) < llhlsStack[len(llhlsStack)-1].depth {
				state := llhlsStack[len(llhlsStack)-1]
				if !state.portReplaced {
					return "", errors.New("missing <Port> in template")
				}
				llhlsStack = llhlsStack[:len(llhlsStack)-1]
			}
			if rootServerDepth != -1 && len(stack) < rootServerDepth {
				rootServerDepth = -1
			}
		}
	}

	if !rootServerFound {
		return "", errors.New("missing <Server> root element in template")
	}
	if !rootBindFound {
		return "", errors.New("missing <Bind> section under <Server> in template")
	}
	if !publishersFound {
		return "", errors.New("missing <Publishers> section under <Bind> in template")
	}
	if !llhlsFound {
		return "", errors.New("missing <LLHLS> section under <Publishers> in template")
	}
	if !topLevelManagersAPIFound {
		return "", errors.New("missing top-level <Managers><API> auth section under <Server> in template")
	}
	if !bindManagersAPIFound {
		return "", errors.New("missing <Bind><Managers><API> listener section in template")
	}
	if !bindManagersPortReplaced {
		return "", errors.New("missing <Port> under <Bind><Managers><API> in template")
	}
	if !bindManagersTLSPortReplaced {
		return "", errors.New("missing <TLSPort> under <Bind><Managers><API> in template")
	}
	if !accessTokenReplaced {
		return "", errors.New("missing <AccessToken> under top-level <Server><Managers><API> in template")
	}
	if iceCandidatesFound == 0 {
		return "", fmt.Errorf("OME template %s missing <%s> (expected under <IceCandidates>)", cfg.TemplatePath, "TcpRelay")
	}

	return out.String(), nil
}

func collapseBlankLines(text string) string {
	var out strings.Builder
	newlineCount := 0
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if ch == '\n' {
			newlineCount++
			if newlineCount > 2 {
				continue
			}
		} else {
			newlineCount = 0
		}
		out.WriteByte(ch)
	}
	return out.String()
}

func updateLineTail(tail, addition string) string {
	if idx := strings.LastIndex(addition, "\n"); idx != -1 {
		return addition[idx+1:]
	}
	return tail + addition
}

func validateOMEGeneratedConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("OME config missing at %s: %w", path, err)
	}

	contents := string(data)
	contents = regexp.MustCompile(`(?s)<!--.*?-->`).ReplaceAllString(contents, "")
	if regexp.MustCompile(`<\s*Server\.bind\.Address\b`).MatchString(contents) {
		return fmt.Errorf("deprecated <Server.bind.Address> found in %s; update the template to remove root bind address tags and rely on top-level <Server><IP>, then regenerate deploy/ome/Server.generated.xml with `go run ./cmd/bitriver ome render --force --env-file ./.env` (or `./scripts/render-ome-config.sh --force`)", path)
	}
	for key, forbidden := range omeTestDefaults {
		if strings.Contains(contents, forbidden) {
			return fmt.Errorf("%s still set to ome-test-* default in %s", key, path)
		}
	}
	if regexp.MustCompile(`<\s*AccessTokens\b`).MatchString(contents) {
		return fmt.Errorf("deprecated <AccessTokens> found in %s; use singular <Managers><API><AccessToken> instead", path)
	}
	if regexp.MustCompile(`(?s)<\s*Application\b[^>]*>.*?<\s*Outputs\b`).MatchString(contents) {
		return fmt.Errorf("deprecated <Application><Outputs> found in %s; place output definitions directly under <Application><OutputProfiles>", path)
	}

	var parsed struct {
		Managers struct {
			API struct {
				AccessToken string `xml:"AccessToken"`
				Port        string `xml:"Port"`
				TLSPort     string `xml:"TLSPort"`
				WorkerCount string `xml:"WorkerCount"`
			} `xml:"API"`
		} `xml:"Managers"`
		Bind struct {
			Managers struct {
				API struct {
					AccessToken string `xml:"AccessToken"`
					Port        string `xml:"Port"`
					TLSPort     string `xml:"TLSPort"`
					WorkerCount string `xml:"WorkerCount"`
				} `xml:"API"`
			} `xml:"Managers"`
		} `xml:"Bind"`
	}
	if err := xml.Unmarshal([]byte(contents), &parsed); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	if strings.TrimSpace(parsed.Managers.API.AccessToken) == "" {
		return fmt.Errorf("missing top-level <Server><Managers><API><AccessToken> auth block in %s", path)
	}
	if strings.TrimSpace(parsed.Managers.API.Port) != "" || strings.TrimSpace(parsed.Managers.API.TLSPort) != "" || strings.TrimSpace(parsed.Managers.API.WorkerCount) != "" {
		return fmt.Errorf("invalid listener fields found under top-level <Server><Managers><API> in %s; keep listener ports under <Server><Bind><Managers><API>", path)
	}
	if strings.TrimSpace(parsed.Bind.Managers.API.Port) == "" || strings.TrimSpace(parsed.Bind.Managers.API.TLSPort) == "" || strings.TrimSpace(parsed.Bind.Managers.API.WorkerCount) == "" {
		return fmt.Errorf("missing <Server><Bind><Managers><API> listener block with <Port>/<TLSPort>/<WorkerCount> in %s", path)
	}
	if strings.TrimSpace(parsed.Bind.Managers.API.AccessToken) != "" {
		return fmt.Errorf("invalid <AccessToken> found under <Server><Bind><Managers><API> in %s; keep auth token only under top-level <Server><Managers><API>", path)
	}

	return nil
}

type xmlComment struct {
	placeholder string
	content     string
}

func stripXMLComments(text string) (string, []xmlComment) {
	commentRe := regexp.MustCompile(`(?s)<!--.*?-->`)
	comments := []xmlComment{}
	index := 0
	stripped := commentRe.ReplaceAllStringFunc(text, func(match string) string {
		placeholder := fmt.Sprintf("%%BITRIVER_XML_COMMENT_%d%%", index)
		index++
		comments = append(comments, xmlComment{
			placeholder: placeholder,
			content:     match,
		})
		return placeholder
	})
	return stripped, comments
}

func restoreXMLComments(text string, comments []xmlComment) string {
	restored := text
	for _, comment := range comments {
		restored = strings.ReplaceAll(restored, comment.placeholder, comment.content)
	}
	return restored
}

func stampImageTag(text, imageTag string) string {
	if strings.TrimSpace(imageTag) == "" {
		return text
	}

	marker := fmt.Sprintf("<!-- Rendered for BITRIVER_OME_IMAGE_TAG=%s -->", xmlEscape(imageTag))
	prefix := "<!-- Rendered for BITRIVER_OME_IMAGE_TAG="
	if start := strings.Index(text, prefix); start != -1 {
		if end := strings.Index(text[start:], "-->"); end != -1 {
			end += start + len("-->")
			return text[:start] + marker + text[end:]
		}
	}

	return strings.Replace(text, "<Server version=\"10\">", "<Server version=\"10\">\n    "+marker, 1)
}

func xmlEscape(value string) string {
	return html.EscapeString(value)
}
