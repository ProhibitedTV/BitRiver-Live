package ingest

import (
	"context"
	"testing"
	"time"

	"bitriver-live/internal/testsupport/ingeststub"
)

func TestHTTPControllerStreamLifecycleIntegration(t *testing.T) {
	stub := ingeststub.Start(ingeststub.Options{
		PrimaryIngest:   "rtmp://primary",
		BackupIngest:    "rtmp://backup",
		OriginURL:       "http://origin",
		PlaybackURL:     "https://playback",
		LiveJobIDs:      []string{"job-live-1", "job-live-2"},
		SRSToken:        "srs-token",
		TranscoderToken: "transcoder-token",
		OMEUser:         "ome-user",
		OMEPassword:     "ome-pass",
	})
	t.Cleanup(stub.Close)

	controller := HTTPController{
		config: Config{
			SRSBaseURL:        stub.BaseURL(),
			SRSToken:          "srs-token",
			OMEBaseURL:        stub.BaseURL(),
			OMEUsername:       "ome-user",
			OMEPassword:       "ome-pass",
			JobBaseURL:        stub.BaseURL(),
			JobToken:          "transcoder-token",
			LadderProfiles:    []Rendition{{Name: "720p", Bitrate: 2400}},
			HTTPRetryInterval: 5 * time.Millisecond,
			HTTPMaxAttempts:   2,
		},
	}

	params := BootParams{
		ChannelID:  "channel-lifecycle",
		StreamKey:  "stream-key",
		SessionID:  "session-1",
		Renditions: []string{"1080p", "720p"},
	}

	result, err := controller.BootStream(context.Background(), params)
	if err != nil {
		t.Fatalf("BootStream: %v", err)
	}

	if result.PrimaryIngest != "rtmp://primary" || result.BackupIngest != "rtmp://backup" {
		t.Fatalf("unexpected ingest endpoints: %+v", result)
	}
	if result.OriginURL != "http://origin" || result.PlaybackURL != "https://playback" {
		t.Fatalf("unexpected playback URLs: %+v", result)
	}
	if len(result.JobIDs) != 2 {
		t.Fatalf("expected two job IDs, got %d", len(result.JobIDs))
	}

	err = controller.ShutdownStream(context.Background(), params.ChannelID, params.SessionID, result.JobIDs)
	if err != nil {
		t.Fatalf("ShutdownStream: %v", err)
	}

	ops := stub.Operations()
	var kinds []string
	for _, op := range ops {
		kinds = append(kinds, op.Kind)
	}

	expectedOrder := []string{
		"channel-create",
		"application-create",
		"job-start",
		"job-stop",
		"job-stop",
		"application-delete",
		"channel-delete",
	}
	if len(kinds) != len(expectedOrder) {
		t.Fatalf("expected %d operations, got %d (%v)", len(expectedOrder), len(kinds), kinds)
	}
	for i, kind := range expectedOrder {
		if kinds[i] != kind {
			t.Fatalf("unexpected operation order at %d: want %s got %s", i, kind, kinds[i])
		}
	}

	var stopped []string
	for _, op := range ops {
		if op.Kind == "job-stop" {
			stopped = append(stopped, op.JobID)
		}
	}
	for _, id := range result.JobIDs {
		if !contains(stopped, id) {
			t.Fatalf("missing stop for job %s", id)
		}
	}
}

func TestHTTPControllerRetriesAndRollsBackOnFailures(t *testing.T) {
	retryInterval := 10 * time.Millisecond
	stub := ingeststub.Start(ingeststub.Options{
		FailChannelCreates: 1,
		FailJobStarts:      2,
		SRSToken:           "srs-token",
		TranscoderToken:    "transcoder-token",
		OMEUser:            "ome-user",
		OMEPassword:        "ome-pass",
	})
	t.Cleanup(stub.Close)

	controller := HTTPController{
		config: Config{
			SRSBaseURL:        stub.BaseURL(),
			SRSToken:          "srs-token",
			OMEBaseURL:        stub.BaseURL(),
			OMEUsername:       "ome-user",
			OMEPassword:       "ome-pass",
			JobBaseURL:        stub.BaseURL(),
			JobToken:          "transcoder-token",
			LadderProfiles:    []Rendition{{Name: "1080p", Bitrate: 4200}},
			HTTPRetryInterval: retryInterval,
			HTTPMaxAttempts:   2,
		},
	}

	params := BootParams{ChannelID: "channel-retries", StreamKey: "stream-key", SessionID: "session-rollback"}

	_, err := controller.BootStream(context.Background(), params)
	if err == nil {
		t.Fatal("expected BootStream failure after exhausted retries")
	}

	ops := stub.Operations()
	channelCreates := filterOps(ops, "channel-create")
	if len(channelCreates) != 2 {
		t.Fatalf("expected 2 channel create attempts, got %d", len(channelCreates))
	}
	if gap := channelCreates[1].Timestamp.Sub(channelCreates[0].Timestamp); gap < retryInterval {
		t.Fatalf("expected retry backoff >= %v, got %v", retryInterval, gap)
	}

	jobStarts := filterOps(ops, "job-start")
	if len(jobStarts) != 2 {
		t.Fatalf("expected 2 job start attempts, got %d", len(jobStarts))
	}
	if gap := jobStarts[1].Timestamp.Sub(jobStarts[0].Timestamp); gap < retryInterval {
		t.Fatalf("expected transcoder retry backoff >= %v, got %v", retryInterval, gap)
	}

	if len(filterOps(ops, "application-create")) != 1 {
		t.Fatalf("expected application create to run once, got %d", len(filterOps(ops, "application-create")))
	}
	if len(filterOps(ops, "application-delete")) != 1 {
		t.Fatalf("expected rollback application delete, got %d", len(filterOps(ops, "application-delete")))
	}
	if len(filterOps(ops, "channel-delete")) != 1 {
		t.Fatalf("expected rollback channel delete, got %d", len(filterOps(ops, "channel-delete")))
	}
}

func contains(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

func filterOps(ops []ingeststub.Operation, kind string) []ingeststub.Operation {
	var filtered []ingeststub.Operation
	for _, op := range ops {
		if op.Kind == kind {
			filtered = append(filtered, op)
		}
	}
	return filtered
}
