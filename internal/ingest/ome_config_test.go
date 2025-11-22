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
//   1. Update the key here to match the new image.
//   2. Update the Server.xml template value to reflect any config changes.
var expectedServerTemplates = map[string]string{
	"airensoft/ovenmediaengine:0.15.10": strings.TrimSpace(`<?xml version="1.0" encoding="utf-8"?>
<Server version="8">
    <Name>OvenMediaEngine</Name>
    <!-- Required for health endpoint and origin-mode APIs; Compose mounts this file at /opt/ovenmediaengine/bin/origin_conf/Server.xml -->
    <Type>origin</Type>
    <IP>0.0.0.0</IP>
    <Bind>0.0.0.0</Bind>
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
                    <!-- Updated automatically by scripts/quickstart.sh before docker compose up. -->
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
//   airensoft/ovenmediaengine:${OME_TAG:-0.15.10}
// becomes:
//   airensoft/ovenmediaengine:0.15.10
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
// It ensures that:
//   - A top-level <Type> element exists and is equal to "origin".
//   - A top-level <Bind> element exists.
// This catches misconfigurations that would break origin-mode APIs or binding.
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
					depth-- // DecodeElement consumes the end tag, adjust depth.
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

	if !bindFound {
		t.Fatalf("missing top-level <Bind> element in Server.xml")
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
