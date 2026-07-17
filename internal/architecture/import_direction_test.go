package architecture

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportDirectionContract(t *testing.T) {
	t.Parallel()

	root := filepath.Clean("../..")
	fset := token.NewFileSet()

	forbiddenByPrefix := map[string][]string{
		"internal/storage/":       {"bitriver-live/internal/api", "bitriver-live/internal/app", "bitriver-live/cmd/"},
		"internal/ingest/":        {"bitriver-live/internal/api", "bitriver-live/internal/app", "bitriver-live/cmd/"},
		"internal/chat/":          {"bitriver-live/internal/api", "bitriver-live/internal/app", "bitriver-live/cmd/"},
		"internal/auth/":          {"bitriver-live/internal/api", "bitriver-live/internal/app", "bitriver-live/cmd/"},
		"internal/observability/": {"bitriver-live/internal/api", "bitriver-live/internal/app", "bitriver-live/cmd/"},
		"internal/service/":       {"bitriver-live/internal/api", "bitriver-live/cmd/"},
		"internal/domain/":        {"bitriver-live/internal/api", "bitriver-live/cmd/"},
		"internal/api/":           {"bitriver-live/internal/storage", "bitriver-live/internal/app", "bitriver-live/cmd/"},
	}

	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "third_party" || base == "web" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		parsed, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}

		for prefix, forbidden := range forbiddenByPrefix {
			if !strings.HasPrefix(rel, prefix) {
				continue
			}
			for _, spec := range parsed.Imports {
				imp := strings.Trim(spec.Path.Value, "\"")
				for _, ban := range forbidden {
					if imp == strings.TrimSuffix(ban, "/") || strings.HasPrefix(imp, ban) {
						t.Fatalf("%s imports forbidden package %s (rule for %s)", rel, imp, prefix)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk import graph: %v", err)
	}
}
