//go:build postgres

package storage

import (
	"errors"
	"testing"
	"time"

	"bitriver-live/internal/ingest"
	"bitriver-live/internal/testsupport/ingeststub"
)

func TestPostgresIngestPipelineEndToEnd(t *testing.T) {
	stub := ingeststub.Start(ingeststub.Options{
		PrimaryIngest:   "rtmp://ingest-primary/live",
		BackupIngest:    "rtmp://ingest-backup/live",
		OriginURL:       "http://origin.internal/live",
		LiveJobIDs:      []string{"job-live-1", "job-live-2"},
		SRSToken:        "srs-secret",
		TranscoderToken: "transcoder-secret",
		OMEAccessToken:  "ome-access-token",
		OMEUser:         "ome-user",
		OMEPassword:     "ome-pass",
	})
	t.Cleanup(stub.Close)

	controllerConfig := ingest.Config{
		SRSBaseURL:         stub.BaseURL(),
		SRSToken:           "srs-secret",
		OMEBaseURL:         stub.BaseURL(),
		OMEPlaybackBaseURL: "https://cdn.example.com/live",
		OMEAccessToken:     "ome-access-token",
		OMEUsername:        "ome-user",
		OMEPassword:        "ome-pass",
		JobBaseURL:         stub.BaseURL(),
		JobToken:           "transcoder-secret",
		MaxBootAttempts:    2,
		RetryInterval:      5 * time.Millisecond,
		HTTPMaxAttempts:    2,
		HTTPRetryInterval:  10 * time.Millisecond,
		HealthTimeout:      time.Second,
		LadderProfiles: []ingest.Rendition{
			{Name: "1080p", ManifestURL: "https://cdn.example.com/hls/1080p.m3u8", Bitrate: 4300},
			{Name: "720p", ManifestURL: "https://cdn.example.com/hls/720p.m3u8", Bitrate: 2400},
		},
	}

	controller, err := controllerConfig.NewHTTPController()
	if err != nil {
		t.Fatalf("NewHTTPController: %v", err)
	}

	store, cleanup, err := postgresRepositoryFactory(t,
		WithIngestController(controller),
		WithIngestRetries(2, 5*time.Millisecond),
		WithIngestTimeout(500*time.Millisecond),
	)
	if errors.Is(err, ErrPostgresUnavailable) {
		t.Skip("postgres repository unavailable in this build")
	}
	if err != nil {
		t.Fatalf("postgresRepositoryFactory: %v", err)
	}
	if cleanup != nil {
		t.Cleanup(cleanup)
	}

	user, err := store.CreateUser(CreateUserParams{
		DisplayName: "Creator",
		Email:       "creator@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	channel, err := store.CreateChannel(user.ID, "Integration Channel", "science", []string{"live"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	session, err := store.StartStream(channel.ID, []string{"1080p", "720p"})
	if err != nil {
		t.Fatalf("StartStream: %v", err)
	}

	if len(session.IngestEndpoints) != 2 {
		t.Fatalf("expected both ingest endpoints, got %v", session.IngestEndpoints)
	}
	if session.PlaybackURL != "https://cdn.example.com/live/"+channel.ID+"/llhls.m3u8" {
		t.Fatalf("unexpected playback URL: %s", session.PlaybackURL)
	}
	if got := len(session.RenditionManifests); got != 2 {
		t.Fatalf("expected two rendition manifests, got %d", got)
	}
	for _, rendition := range session.RenditionManifests {
		if rendition.ManifestURL == "" {
			t.Fatalf("rendition %s missing manifest URL", rendition.Name)
		}
	}

	ended, err := store.StopStream(channel.ID, 99)
	if err != nil {
		t.Fatalf("StopStream: %v", err)
	}

	if ended.EndedAt == nil {
		t.Fatalf("expected session to record end time")
	}

	operations := stub.Operations()
	expectedKinds := []string{
		"channel-create",
		"application-validate",
		"job-start",
		"job-stop",
		"job-stop",
		"channel-delete",
	}
	if len(operations) != len(expectedKinds) {
		t.Fatalf("expected %d operations, got %d", len(expectedKinds), len(operations))
	}
	for i, kind := range expectedKinds {
		if operations[i].Kind != kind {
			t.Fatalf("operation %d: want %s got %s", i, kind, operations[i].Kind)
		}
	}
}
