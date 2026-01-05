package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"bitriver-live/internal/api"
	"bitriver-live/internal/envutil"
)

type setupManager struct {
	envPath   string
	restartCh chan<- struct{}
}

func newSetupManager(envPath string, restartCh chan<- struct{}) *setupManager {
	return &setupManager{envPath: strings.TrimSpace(envPath), restartCh: restartCh}
}

func (s *setupManager) ApplySetup(ctx context.Context, cfg api.SetupConfig) (api.SetupResult, error) {
	envPath := s.envPath
	if envPath == "" {
		envPath = ".env"
	}

	values, err := envutil.LoadFile(envPath, nil)
	if err != nil {
		return api.SetupResult{}, err
	}
	if values == nil {
		values = make(map[string]string)
	}

	if cfg.TLSCertPath != "" && cfg.TLSKeyPath != "" {
		targetDir := filepath.Join(filepath.Dir(envPath), "deploy", "certs")
		stagedCert, stagedKey, stageErr := stageTLSFiles(cfg.TLSCertPath, cfg.TLSKeyPath, targetDir)
		if stageErr != nil {
			return api.SetupResult{}, stageErr
		}
		cfg.TLSCertPath = stagedCert
		cfg.TLSKeyPath = stagedKey
	}

	applySetupConfig(values, cfg)

	backupPath, err := backupEnv(envPath)
	if err != nil {
		return api.SetupResult{}, err
	}

	if err := writeEnvFile(envPath, values); err != nil {
		_ = restoreBackup(envPath, backupPath)
		return api.SetupResult{}, err
	}

	if err := requestRestart(ctx, s.restartCh); err != nil {
		_ = restoreBackup(envPath, backupPath)
		return api.SetupResult{}, err
	}

	if backupPath != "" {
		_ = os.Remove(backupPath)
	}

	return api.SetupResult{RestartScheduled: true}, nil
}

func applySetupConfig(values map[string]string, cfg api.SetupConfig) {
	values["BITRIVER_LIVE_MODE"] = "production"

	values["BITRIVER_LIVE_ADMIN_EMAIL"] = cfg.AdminEmail
	if cfg.AdminPassword != "" {
		values["BITRIVER_LIVE_ADMIN_PASSWORD"] = cfg.AdminPassword
	}

	values["BITRIVER_LIVE_PORT"] = strconv.Itoa(cfg.APIPort)
	values["BITRIVER_LIVE_ADDR"] = fmt.Sprintf(":%d", cfg.APIPort)

	if cfg.ViewerURL != "" {
		values["NEXT_PUBLIC_VIEWER_URL"] = cfg.ViewerURL
	}
	if cfg.PublicAPIURL != "" {
		values["NEXT_PUBLIC_API_BASE_URL"] = cfg.PublicAPIURL
	}
	if cfg.ViewerOrigin != "" {
		values["BITRIVER_VIEWER_ORIGIN"] = cfg.ViewerOrigin
	}
	if cfg.TLSCertPath != "" {
		values["BITRIVER_LIVE_TLS_CERT"] = cfg.TLSCertPath
	}
	if cfg.TLSKeyPath != "" {
		values["BITRIVER_LIVE_TLS_KEY"] = cfg.TLSKeyPath
	}

	values["BITRIVER_POSTGRES_PASSWORD"] = cfg.PostgresPassword
	values["BITRIVER_REDIS_PASSWORD"] = cfg.RedisPassword
	values["BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD"] = cfg.RedisPassword
	values["BITRIVER_SRS_TOKEN"] = cfg.SRSToken
	values["BITRIVER_OME_API_TOKEN"] = cfg.OMEToken
	values["BITRIVER_OME_ACCESS_TOKEN"] = cfg.OMEToken
	values["BITRIVER_TRANSCODER_TOKEN"] = cfg.TranscoderToken
	if cfg.MetricsToken != "" {
		values["BITRIVER_LIVE_METRICS_TOKEN"] = cfg.MetricsToken
	}
}

func backupEnv(envPath string) (string, error) {
	data, err := os.ReadFile(envPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}

	backupPath := fmt.Sprintf("%s.bak-%d", envPath, time.Now().UnixNano())
	if err := os.WriteFile(backupPath, data, 0o600); err != nil {
		return "", err
	}
	return backupPath, nil
}

func restoreBackup(envPath, backupPath string) error {
	if backupPath == "" {
		return nil
	}
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return err
	}
	return os.WriteFile(envPath, data, 0o600)
}

func writeEnvFile(envPath string, values map[string]string) error {
	dir := filepath.Dir(envPath)
	if err := os.MkdirAll(dir, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}

	tmp, err := os.CreateTemp(dir, "env")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}

	if _, err := tmp.WriteString(formatEnv(values)); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmp.Name(), envPath)
}

func formatEnv(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(strconv.Quote(values[key]))
		builder.WriteString("\n")
	}
	return builder.String()
}

func requestRestart(ctx context.Context, restartCh chan<- struct{}) error {
	if restartCh == nil {
		return nil
	}
	select {
	case restartCh <- struct{}{}:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-time.After(2 * time.Second):
		return fmt.Errorf("failed to schedule restart: timeout")
	}
}

func stageTLSFiles(certPath, keyPath, destDir string) (string, string, error) {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", "", err
	}

	copy := func(src string) (string, error) {
		base := filepath.Base(src)
		dest := filepath.Join(destDir, base)
		if absSrc, _ := filepath.Abs(src); absSrc != "" {
			if absDest, _ := filepath.Abs(dest); absDest == absSrc {
				return dest, nil
			}
		}

		data, err := os.ReadFile(src)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(dest, data, 0o600); err != nil {
			return "", err
		}
		return dest, nil
	}

	stagedCert, err := copy(certPath)
	if err != nil {
		return "", "", err
	}

	stagedKey, err := copy(keyPath)
	if err != nil {
		return "", "", err
	}

	return stagedCert, stagedKey, nil
}
