package ingest

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
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
	"airensoft/ovenmediaengine:0.15.10": strings.TrimSpace(`<?xml version="1.0" encoding="utf-8"?>
<Server version="10">
    <Name>OvenMediaEngine</Name>

    <!-- Host type (origin/edge) -->
    <Type>origin</Type>

    <!-- Specify IP address to bind (* means all IPs) -->
    <IP>*</IP>

    <!-- Delete client IPs from logs / APIs if true -->
    <PrivacyProtection>false</PrivacyProtection>

    <!-- Used to discover public IP for WebRTC in some environments -->
    <StunServer>stun.l.google.com:19302</StunServer>

    <!--
      Optional server-wide modules.
      Modern OME supports HTTP2/LLHLS/P2P here; there is no <Control> module.
    -->
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

    <!--
      Bind: managers (API), providers (ingest), and publishers (playback).
      The quickstart script will rewrite Address/Port/TLSPort values from BITRIVER_OME_*.
    -->
    <Bind>
        <Managers>
            <!-- REST API for control at /v1/... -->
            <API>
                <Port>8081</Port>
                <TLSPort>8082</TLSPort>
                <WorkerCount>1</WorkerCount>
            </API>
        </Managers>

        <Providers>
            <!-- RTMP ingest: rtmp://host:1935/live/stream -->
            <RTMP>
                <Port>1935</Port>
                <WorkerCount>1</WorkerCount>
            </RTMP>

            <!-- WebRTC ingest/signalling (optional but useful for WHIP / internal tools) -->
            <WebRTC>
                <Signalling>
                    <!-- WebSocket signalling for WebRTC (publish + play) -->
                    <Port>3333</Port>
                    <TLSPort>3334</TLSPort>
                    <WorkerCount>1</WorkerCount>
                </Signalling>
                <IceCandidates>
                    <!-- TURN / WebRTC-over-TCP relay -->
                    <TcpRelay>*:3478</TcpRelay>
                    <TcpForce>false</TcpForce>
                    <TcpRelayWorkerCount>1</TcpRelayWorkerCount>

                    <!-- UDP ports for WebRTC media -->
                    <IceCandidate>*:10000-10009/udp</IceCandidate>
                </IceCandidates>
            </WebRTC>
        </Providers>

        <Publishers>
            <!-- LL-HLS (low-latency HLS) playback -->
            <LLHLS>
                <Port>8080</Port>
                <TLSPort>8443</TLSPort>
                <WorkerCount>1</WorkerCount>
            </LLHLS>

            <!-- WebRTC playback (reuses same signalling + ICE ports) -->
            <WebRTC>
                <Signalling>
                    <Port>3333</Port>
                    <TLSPort>3334</TLSPort>
                    <WorkerCount>1</WorkerCount>
                </Signalling>
                <IceCandidates>
                    <TcpRelay>*:3478</TcpRelay>
                    <TcpForce>false</TcpForce>
                    <TcpRelayWorkerCount>1</TcpRelayWorkerCount>
                    <IceCandidate>*:10000-10009/udp</IceCandidate>
                </IceCandidates>
            </WebRTC>
        </Publishers>
    </Bind>

    <!--
      Virtual host + application config.

      We keep a single vhost "default" with one application "live" to match
      BitRiver Live’s expectation of /live as the app name.
    -->
    <VirtualHosts>
        <VirtualHost>
            <Name>default</Name>

            <Host>
                <Names>
                    <!-- * = any host -->
                    <Name>*</Name>
                </Names>
            </Host>

            <Applications>
                <Application>
                    <Name>live</Name>

                    <!-- Use default Providers/Publishers from Bind -->
                    <!-- You can add per-app settings here if needed later -->
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
	//     image: airensoft/ovenmediaengine:0.15.10
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
//	airensoft/ovenmediaengine:${OME_TAG:-0.15.10}
//
// becomes:
//
//	airensoft/ovenmediaengine:0.15.10
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
