package pgpassfile

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type entry struct {
	host     string
	port     string
	database string
	user     string
	password string
}

type Passfile struct {
	entries []entry
}

func ReadPassfile(path string) (*Passfile, error) {
	resolved, err := resolvePath(path)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(resolved)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	entries := make([]entry, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := splitColonEscaped(line)
		if len(fields) != 5 {
			continue
		}
		entries = append(entries, entry{
			host:     fields[0],
			port:     fields[1],
			database: fields[2],
			user:     fields[3],
			password: fields[4],
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &Passfile{entries: entries}, nil
}

func resolvePath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	if env := os.Getenv("PGPASSFILE"); env != "" {
		return env, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if home == "" {
		return "", errors.New("pgpassfile: cannot determine home directory")
	}
	return filepath.Join(home, ".pgpass"), nil
}

func splitColonEscaped(line string) []string {
	parts := make([]string, 0, 5)
	var builder strings.Builder
	escaped := false
	for _, r := range line {
		switch {
		case escaped:
			builder.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == ':':
			parts = append(parts, builder.String())
			builder.Reset()
		default:
			builder.WriteRune(r)
		}
	}
	parts = append(parts, builder.String())
	return parts
}

func (pf *Passfile) FindPassword(host, port, database, user string) string {
	if pf == nil {
		return ""
	}
	for _, e := range pf.entries {
		if matchField(e.host, host) && matchField(e.port, port) && matchField(e.database, database) && matchField(e.user, user) {
			return e.password
		}
	}
	return ""
}

func matchField(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	return pattern == value
}
