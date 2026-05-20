package storage

import (
	"context"
	"errors"
	"time"

	"bitriver-live/internal/ctxutil"
	"bitriver-live/internal/domain"
	"bitriver-live/internal/ingest"
)

func runIngestBootWithRetry(parent context.Context, controller ingest.Controller, params ingest.BootParams, timeout time.Duration, attempts int, retryInterval time.Duration) (ingest.BootResult, error) {
	parent = ctxutil.Normalize(parent)
	resolvedAttempts := attempts
	if resolvedAttempts <= 0 {
		resolvedAttempts = 1
	}
	deadline := normalizeIngestTimeout(timeout)

	var (
		boot    ingest.BootResult
		bootErr error
	)
	for attempt := 0; attempt < resolvedAttempts; attempt++ {
		if err := parent.Err(); err != nil {
			return boot, err
		}
		ctx, cancel := context.WithTimeout(parent, deadline)
		boot, bootErr = controller.BootStream(ctx, params)
		cancel()
		if bootErr == nil {
			break
		}
		if errors.Is(bootErr, context.Canceled) || errors.Is(bootErr, context.DeadlineExceeded) {
			break
		}
		if attempt < resolvedAttempts-1 && retryInterval > 0 {
			timer := time.NewTimer(retryInterval)
			select {
			case <-parent.Done():
				timer.Stop()
				return boot, parent.Err()
			case <-timer.C:
			}
		}
	}

	return boot, bootErr
}

func applyBootResultToSession(session *domain.StreamSession, boot ingest.BootResult, keepEmptyEndpoints bool) {
	session.OriginURL = boot.OriginURL
	session.PlaybackURL = boot.PlaybackURL
	session.IngestJobIDs = append([]string{}, boot.JobIDs...)

	ingestEndpoints := make([]string, 0, 2)
	if boot.PrimaryIngest != "" {
		ingestEndpoints = append(ingestEndpoints, boot.PrimaryIngest)
	}
	if boot.BackupIngest != "" {
		ingestEndpoints = append(ingestEndpoints, boot.BackupIngest)
	}
	if keepEmptyEndpoints || len(ingestEndpoints) > 0 {
		session.IngestEndpoints = ingestEndpoints
	}

	if len(boot.Renditions) > 0 {
		manifests := make([]domain.RenditionManifest, 0, len(boot.Renditions))
		for _, rendition := range boot.Renditions {
			manifests = append(manifests, domain.RenditionManifest{
				Name:        rendition.Name,
				ManifestURL: rendition.ManifestURL,
				Bitrate:     rendition.Bitrate,
			})
		}
		session.RenditionManifests = manifests
	}
}
