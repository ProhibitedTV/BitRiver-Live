package scripts_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestHelmBackupUsesManifestBoundCanonicalProducer(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	canonical := readRepoFile(t, repoRoot, filepath.Join("scripts", "backup-postgres.sh"))
	generated := readRepoFile(t, repoRoot, filepath.Join(
		"deploy", "helm", "bitriver-live", "files", "backup-postgres.sh",
	))
	if canonical != generated {
		t.Fatal("generated Helm backup runner differs from scripts/backup-postgres.sh")
	}

	configMap := readRepoFile(t, repoRoot, filepath.Join(
		"deploy", "helm", "bitriver-live", "templates", "configmap-backup-scripts.yaml",
	))
	for _, required := range []string{
		`.Values.backups.enabled`,
		`name: {{ include "bitriver-live.fullname" . }}-backup-scripts`,
		`.Files.Get "files/backup-postgres.sh"`,
	} {
		if !strings.Contains(configMap, required) {
			t.Fatalf("Helm backup ConfigMap missing canonical-producer invariant %q", required)
		}
	}

	cronJob := readRepoFile(t, repoRoot, filepath.Join(
		"deploy", "helm", "bitriver-live", "templates", "cronjob-postgres-backup.yaml",
	))
	for _, required := range []string{
		`backups.objectStorage.enabled must be true when backups.enabled is true`,
		`name: BITRIVER_BACKUP_SOURCE_RELEASE`,
		`name: BITRIVER_BACKUP_SOURCE_COMMIT`,
		`name: BITRIVER_BACKUP_UPLOAD_ENABLED`,
		`value: "1"`,
		`exec /bin/bash /scripts/backup-postgres.sh`,
		`name: backup-scripts`,
		`mountPath: /scripts`,
	} {
		if !strings.Contains(cronJob, required) {
			t.Fatalf("Helm backup CronJob missing manifest-bound producer invariant %q", required)
		}
	}
	for _, retired := range []string{
		`pg_dump -h`,
		`sha256sum "$file"`,
	} {
		if strings.Contains(cronJob, retired) {
			t.Fatalf("Helm backup CronJob retains legacy two-file producer fragment %q", retired)
		}
	}
}
