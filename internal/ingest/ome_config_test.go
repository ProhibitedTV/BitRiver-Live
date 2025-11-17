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

var expectedServerTemplates = map[string]string{
	"airensoft/ovenmediaengine:0.15.10": strings.TrimSpace(`<?xml version="1.0" encoding="utf-8"?>
<Server version="8">
    <Name>OvenMediaEngine</Name>
    <!-- Required for health endpoint and origin-mode APIs; Compose mounts this file at /opt/ovenmediaengine/bin/origin_conf/Server.xml -->
    <Type>origin</Type>
    <IP>0.0.0.0</IP>
    <PrivacyProtection>false</PrivacyProtection>
    <StunServer>stun.l.google.com:19302</StunServer>

    <Modules>
        <Control>
            <Server>
                <Ports>
                    <TCP>8081</TCP>
                </Ports>
            </Server>
            <Authentication>
                <User>
                    <ID>admin</ID>
                    <!-- Replace with a secure password for production deployments. -->
                    <Password>local-dev-password</Password>
                </User>
            </Authentication>
        </Control>

        <Host>
            <VirtualHosts>
                <VirtualHost>
                    <Name>default</Name>
                    <Host>*</Host>
                    <App>
                        <Name>live</Name>
                    </App>
                </VirtualHost>
            </VirtualHosts>
        </Host>
    </Modules>
</Server>`),
}

func TestOMEConfigMatchesComposeTemplate(t *testing.T) {
	repoRoot := repoRoot(t)
	composePath := filepath.Join(repoRoot, "deploy", "docker-compose.yml")
	serverPath := filepath.Join(repoRoot, "deploy", "ome", "Server.xml")

	image := omeImageFromCompose(t, composePath)
	serverXML := readFile(t, serverPath)

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

func repoRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to resolve caller for repo root detection")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func omeImageFromCompose(t *testing.T, composePath string) string {
	t.Helper()

	data := readFile(t, composePath)
	re := regexp.MustCompile(`(?m)^  ome:\n(?:^[ \t]+.*\n)*?^[ \t]+image:\s*([^\s#]+)`) //nolint:revive
	matches := re.FindSubmatch(data)
	if len(matches) < 2 {
		t.Fatalf("failed to locate ome image in %s", composePath)
	}

	return string(matches[1])
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}

	return data
}

func validateServerXML(t *testing.T, serverXML []byte) {
	t.Helper()

	decoder := xml.NewDecoder(bytes.NewReader(serverXML))
	depth := 0
	var serverType string
	var bindFound bool

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
					depth--
				case "Bind":
					bindFound = true
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

	if bindFound {
		t.Fatalf("disallowed top-level <Bind> element found in Server.xml")
	}
}

func normalizeXML(xmlContent string) string {
	normalized := strings.ReplaceAll(xmlContent, "\r\n", "\n")
	normalized = strings.TrimSpace(normalized)
	return normalized
}
