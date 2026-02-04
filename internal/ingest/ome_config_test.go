package ingest

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// expectedServerTemplates contains canonical Server.xml templates keyed by the
// OME Docker image reference used in docker-compose.
//
// When bumping the OME image tag in deploy/docker-compose.yml, you should:
//  1. Update the key here to match the new image.
//  2. Update the Server.xml template value to reflect any config changes.
var expectedServerTemplates = map[string]string{
	"airensoft/ovenmediaengine:0.16.0": strings.TrimSpace(`<?xml version="1.0" encoding="utf-8"?>
<Server version="10">
    <Name>OvenMediaEngine</Name>
    <Type>origin</Type>
    <IP>0.0.0.0</IP>
    <PrivacyProtection>false</PrivacyProtection>
    <StunServer>stun.l.google.com:19302</StunServer>

    <!-- Server.bind.Address was removed in 0.15.x; use the global <IP> and the per-protocol <Bind> block instead. -->
    <Modules>
        <HTTP2>
            <Enable>true</Enable>
        </HTTP2>

        <LLHLS>
            <Enable>true</Enable>
        </LLHLS>

        <P2P>
            <Enable>false</Enable>
            <MaxClientPeersPerHostPeer>2</MaxClientPeersPerHostPeer>
        </P2P>
    </Modules>

    <!-- Corrected: <Bind> replaces the deprecated <Server.bind.Address> container. -->
    <Bind>
        <Address>0.0.0.0</Address>

        <Managers>
            <API>
                <Port>8081</Port>
                <TLSPort>8082</TLSPort>
                <WorkerCount>1</WorkerCount>
                <!--
                  APIServer rejects an empty token; customize this value if you
                  want to protect API access, or leave the placeholder for
                  open/local use.
                -->
                <AccessTokens>
                    <AccessToken>BITRIVER_OME_API_TOKEN_PLACEHOLDER</AccessToken>
                </AccessTokens>
                <Authentication>
                    <AutoRegister>true</AutoRegister>
                    <AllowAnonymousUser>false</AllowAnonymousUser>
                    <AllowAnonymousReferrer>false</AllowAnonymousReferrer>
                    <User>
                        <ID>BITRIVER_OME_USERNAME_PLACEHOLDER</ID>
                        <Password>BITRIVER_OME_PASSWORD_PLACEHOLDER</Password>
                    </User>
                </Authentication>
            </API>
        </Managers>

        <Providers>
            <RTMP>
                <Port>1935</Port>
                <WorkerCount>1</WorkerCount>
            </RTMP>

            <WebRTC>
                <Signalling>
                    <Port>9000</Port>
                    <TLSPort>9443</TLSPort>
                    <WorkerCount>1</WorkerCount>
                </Signalling>
                <IceCandidates>
                    <!-- When port-forwarding or NAT rewrites the container ports, update
                         BITRIVER_OME_TCP_RELAY/BITRIVER_OME_ICE_CANDIDATE so the
                         advertised candidates match the reachable host:port tuples. -->
                    <TcpRelay>*:3478</TcpRelay>
                    <TcpForce>false</TcpForce>
                    <TcpRelayWorkerCount>1</TcpRelayWorkerCount>
                    <IceCandidate>*:10000-10009/udp</IceCandidate>
                </IceCandidates>
            </WebRTC>
        </Providers>

        <Publishers>
            <LLHLS>
                <Port>8080</Port>
                <TLSPort>8443</TLSPort>
                <WorkerCount>1</WorkerCount>
            </LLHLS>

            <WebRTC>
                <Signalling>
                    <Port>9000</Port>
                    <TLSPort>9443</TLSPort>
                    <WorkerCount>1</WorkerCount>
                </Signalling>
                <IceCandidates>
                    <!-- Renderer requires <TcpRelay> and <IceCandidate> to exist for token replacement. -->
                    <TcpRelay>*:3478</TcpRelay>
                    <TcpForce>false</TcpForce>
                    <TcpRelayWorkerCount>1</TcpRelayWorkerCount>
                    <IceCandidate>*:10000-10009/udp</IceCandidate>
                </IceCandidates>
            </WebRTC>
        </Publishers>
    </Bind>

    <!-- Corrected: Applications must live under <VirtualHosts>/<VirtualHost>, not <Server.Applications>. -->
    <VirtualHosts>
        <VirtualHost>
            <Name>default</Name>

            <Host>
                <Names>
                    <Name>*</Name>
                </Names>
            </Host>

            <Applications>
                <Application>
                    <Name>live</Name>
                    <Type>live</Type>

                    <Outputs>
                        <OutputProfiles>
                            <OutputProfile>
                                <Name>copy_passthrough</Name>
                                <OutputStreams>
                                    <OutputStream>
                                        <Name>copy</Name>
                                        <Video>
                                            <Codec>copy</Codec>
                                        </Video>
                                        <Audio>
                                            <Codec>copy</Codec>
                                        </Audio>
                                    </OutputStream>
                                </OutputStreams>
                                <!-- OutputStreams are required for the pinned 0.16.x schema. -->
                            </OutputProfile>
                        </OutputProfiles>

                        <LLHLS>
                            <SegmentDuration>6</SegmentDuration>
                            <PartDuration>1</PartDuration>
                            <SegmentCount>5</SegmentCount>
                            <PartCount>3</PartCount>
                            <PreloadHint>true</PreloadHint>
                            <AdditionalPlaylist>true</AdditionalPlaylist>
                        </LLHLS>
                    </Outputs>

                    <Publishers>
                        <WebRTC>
                            <Enable>true</Enable>
                            <OutputProfile>copy_passthrough</OutputProfile>
                        </WebRTC>
                        <LLHLS>
                            <Enable>true</Enable>
                            <OutputProfile>copy_passthrough</OutputProfile>
                        </LLHLS>
                    </Publishers>

                    <Providers>
                        <RTMP>
                            <Enable>true</Enable>
                        </RTMP>
                        <WebRTC>
                            <Enable>true</Enable>
                        </WebRTC>
                    </Providers>
                </Application>
            </Applications>
        </VirtualHost>
    </VirtualHosts>
</Server>`),
}

// TestOMEConfigMatchesComposeTemplate ensures that the OvenMediaEngine
// Server.xml configuration used by docker-compose matches the canonical
// template for the configured OME image.
//
// This guards against accidental drift between:
//   - deploy/docker-compose.yml (OME image tag), and
//   - deploy/ome/Server.xml (mounted into the container as origin_conf/Server.xml).
func TestOMEConfigMatchesComposeTemplate(t *testing.T) {
	repoRoot := repoRoot(t)
	composePath := filepath.Join(repoRoot, "deploy", "docker-compose.yml")
	serverPath := filepath.Join(repoRoot, "deploy", "ome", "Server.xml")

	image := omeImageFromCompose(t, composePath)
	serverXML := readFile(t, serverPath)

	// Basic structural sanity checks on Server.xml.
	validateServerXML(t, serverXML)

	expected, ok := expectedServerTemplates[image]
	if !ok {
		t.Fatalf("missing expected template for OME image %q; update expectedServerTemplates in ome_config_test.go when bumping the image tag", image)
	}

	normalizedExpected := normalizeXML(expected)
	normalizedActual := normalizeXML(string(serverXML))
	if normalizedActual != normalizedExpected {
		t.Fatalf("deploy/ome/Server.xml does not match expected template for %s\n\nExpected:\n%s\n\nActual:\n%s", image, normalizedExpected, normalizedActual)
	}
}

// repoRoot returns the repository root based on the location of this test file.
func repoRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to resolve caller for repo root detection")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func renderOMEConfig(t *testing.T, repoRoot string, envContents string) []byte {
	t.Helper()

	envPath := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(envPath, []byte(envContents+"\n"), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	outputPath := filepath.Join(repoRoot, "deploy", "ome", "Server.generated.xml")
	original, err := os.ReadFile(outputPath)
	var originalMode os.FileMode
	if err == nil {
		stat, statErr := os.Stat(outputPath)
		if statErr != nil {
			t.Fatalf("stat generated config: %v", statErr)
		}
		originalMode = stat.Mode()
		t.Cleanup(func() {
			_ = os.WriteFile(outputPath, original, originalMode)
		})
	} else if errors.Is(err, os.ErrNotExist) {
		t.Cleanup(func() {
			_ = os.Remove(outputPath)
		})
	} else {
		t.Fatalf("read generated config: %v", err)
	}

	cmd := exec.Command("go", "run", "./cmd/bitriver", "ome", "render", "--force", "--env-file", envPath)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"GOTOOLCHAIN=local",
		"GOPROXY=off",
		"GOSUMDB=off",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go renderer failed: %v\n%s", err, output)
	}

	rendered, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read generated output: %v", err)
	}

	return rendered
}

// omeImageFromCompose extracts the OME image reference from the
// deploy/docker-compose.yml file, normalizing any ${VAR:-default} expansion
// to the default value.
//
// This allows the test to work whether or not environment variables are set.
func omeImageFromCompose(t *testing.T, composePath string) string {
	t.Helper()

	data := readFile(t, composePath)

	// Match:
	//   ome:
	//     image: airensoft/ovenmediaengine:0.16.0
	// including optional extra lines between ome: and image:.
	re := regexp.MustCompile(`(?m)^  ome:\n(?:^[ \t]+.*\n)*?^[ \t]+image:\s*([^\s#]+)`) //nolint:revive
	matches := re.FindSubmatch(data)
	if len(matches) < 2 {
		t.Fatalf("failed to locate ome image in %s", composePath)
	}

	return normalizeComposeImageRef(string(matches[1]))
}

// normalizeComposeImageRef simplifies docker-compose image references that use
// the ${VAR:-default} syntax by replacing them with the default value.
//
// For example:
//
//	airensoft/ovenmediaengine:${OME_TAG:-0.16.0}
//
// becomes:
//
//	airensoft/ovenmediaengine:0.16.0
func normalizeComposeImageRef(image string) string {
	re := regexp.MustCompile(`\$\{[^:}]+:-([^}]+)\}`)
	return re.ReplaceAllString(image, "$1")
}

// readFile is a small helper that reads an entire file or fails the test.
func readFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}

	return data
}

// validateServerXML performs minimal structural validation on Server.xml.
//
// It ensures that a top-level <Type> element exists and is equal to "origin".
//
// This catches misconfigurations that would break origin-mode APIs.
func validateServerXML(t *testing.T, serverXML []byte) {
	t.Helper()

	if bytes.Contains(serverXML, []byte("<Server.bind>")) || bytes.Contains(serverXML, []byte("</Server.bind>")) {
		t.Fatalf("Server.xml contains <Server.bind> tags; use <Bind> instead")
	}

	decoder := xml.NewDecoder(bytes.NewReader(serverXML))
	depth := 0
	var serverType string

	for {
		tok, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			t.Fatalf("failed to parse Server.xml: %v", err)
		}

		switch element := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 {
				switch element.Name.Local {
				case "Type":
					var value string
					if err := decoder.DecodeElement(&value, &element); err != nil {
						t.Fatalf("failed to decode <Type>: %v", err)
					}

					serverType = strings.TrimSpace(value)
					depth-- // DecodeElement consumes the end tag, adjust depth.
				}
			}
		case xml.EndElement:
			depth--
		}
	}

	if serverType == "" {
		t.Fatalf("missing <Type> field in Server.xml")
	}

	if serverType != "origin" {
		t.Fatalf("unexpected <Type> %q in Server.xml; expected origin", serverType)
	}
}

// normalizeXML normalizes XML content for string comparison by:
//   - Converting CRLF to LF.
//   - Trimming leading and trailing whitespace.
//
// It does not pretty-print or reformat the XML, so whitespace differences
// within the body will still cause mismatches, which is desirable here to
// keep the template strict.
func normalizeXML(xmlContent string) string {
	normalized := strings.ReplaceAll(xmlContent, "\r\n", "\n")
	normalized = strings.TrimSpace(normalized)
	return normalized
}

func TestRenderOMEConfigRequiresManagersAuth(t *testing.T) {
	repoRoot := repoRoot(t)
	envContents := strings.Join([]string{
		"BITRIVER_OME_BIND=0.0.0.0",
		"BITRIVER_OME_IP=0.0.0.0",
		"BITRIVER_OME_SERVER_PORT=9000",
		"BITRIVER_OME_SERVER_TLS_PORT=9443",
		"BITRIVER_OME_USERNAME=admin",
		"BITRIVER_OME_PASSWORD=password",
		"BITRIVER_OME_API_TOKEN=token",
		"BITRIVER_OME_ACCESS_TOKEN=health-token",
		"BITRIVER_OME_IMAGE_TAG=0.16.0",
		"BITRIVER_OME_TCP_RELAY=*:3478",
		"BITRIVER_OME_ICE_CANDIDATE=example.com:10000-10009/udp",
	}, "\n")
	data := renderOMEConfig(t, repoRoot, envContents)
	hasAccessTokens := bytes.Contains(data, []byte("<AccessTokens>"))
	hasAuthentication := bytes.Contains(data, []byte("<Authentication>"))
	hasOutputs := bytes.Contains(data, []byte("<Outputs>"))
	hasOutputStreams := bytes.Contains(data, []byte("<OutputStreams>"))
	summary := fmt.Sprintf("AccessTokens=%t Authentication=%t", hasAccessTokens, hasAuthentication)

	if !hasAccessTokens || !hasAuthentication {
		t.Fatalf("expected managers auth for rendered config, but missing nodes: %s", summary)
	}

	if !hasOutputs {
		t.Fatalf("expected <Outputs> in rendered config for 0.16.0")
	}

	if !hasOutputStreams {
		t.Fatalf("expected <OutputStreams> in rendered config for 0.16.0")
	}
}

func TestRenderOMEConfigRewritesLLHLSPorts(t *testing.T) {
	repoRoot := repoRoot(t)
	envContents := strings.Join([]string{
		"BITRIVER_OME_BIND=0.0.0.0",
		"BITRIVER_OME_IP=0.0.0.0",
		"BITRIVER_OME_SERVER_PORT=9000",
		"BITRIVER_OME_SERVER_TLS_PORT=9443",
		"BITRIVER_OME_LLHLS_PORT=8099",
		"BITRIVER_OME_LLHLS_TLS_PORT=9444",
		"BITRIVER_OME_USERNAME=admin",
		"BITRIVER_OME_PASSWORD=password",
		"BITRIVER_OME_API_TOKEN=token",
		"BITRIVER_OME_ACCESS_TOKEN=health-token",
		"BITRIVER_OME_IMAGE_TAG=0.16.0",
		"BITRIVER_OME_TCP_RELAY=*:3478",
		"BITRIVER_OME_ICE_CANDIDATE=example.com:10000-10009/udp",
	}, "\n")

	data := renderOMEConfig(t, repoRoot, envContents)
	llhlsPattern := regexp.MustCompile(`(?s)<Publishers>.*?<LLHLS>.*?<Port>8099</Port>.*?<TLSPort>9444</TLSPort>.*?</LLHLS>.*?</Publishers>`)
	if !llhlsPattern.Match(data) {
		t.Fatalf("expected LLHLS ports to be rewritten in rendered config, but did not find the expected block")
	}
}
