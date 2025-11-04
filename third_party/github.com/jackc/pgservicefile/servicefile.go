package pgservicefile

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Service struct {
	Name     string
	Settings map[string]string
}

type Servicefile struct {
	services map[string]*Service
}

func ReadServicefile(path string) (*Servicefile, error) {
	resolved, err := resolvePath(path)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(resolved)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	services := make(map[string]*Service)
	var current *Service

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.TrimSpace(line[1 : len(line)-1])
			if name == "" {
				current = nil
				continue
			}
			svc := &Service{Name: name, Settings: make(map[string]string)}
			services[strings.ToLower(name)] = svc
			current = svc
			continue
		}
		if current == nil {
			continue
		}
		key, val, ok := parseKeyValue(line)
		if !ok {
			continue
		}
		current.Settings[key] = val
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &Servicefile{services: services}, nil
}

func resolvePath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	if env := os.Getenv("PGSERVICEFILE"); env != "" {
		return env, nil
	}
	if env := os.Getenv("PGSYSCONFDIR"); env != "" {
		return filepath.Join(env, "pg_service.conf"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if home == "" {
		return "", errors.New("pgservicefile: cannot determine home directory")
	}
	return filepath.Join(home, ".pg_service.conf"), nil
}

func parseKeyValue(line string) (string, string, bool) {
	idx := strings.Index(line, "=")
	if idx == -1 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:idx])
	val := strings.TrimSpace(line[idx+1:])
	if key == "" {
		return "", "", false
	}
	val = strings.Trim(val, "\"'")
	return strings.ToLower(key), val, true
}

func (sf *Servicefile) GetService(name string) (*Service, error) {
	if sf == nil {
		return nil, errors.New("pgservicefile: no services available")
	}
	svc, ok := sf.services[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("service %q not found", name)
	}
	return svc, nil
}
