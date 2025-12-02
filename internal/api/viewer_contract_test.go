package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"time"
	"unsafe"

	api "bitriver-live/internal/api"
	"bitriver-live/internal/auth"
	"bitriver-live/internal/ingest"
	"bitriver-live/internal/server"
	"bitriver-live/internal/storage"
	"bitriver-live/internal/testsupport"
)

type directoryContractResponse struct {
	Channels []struct {
		Channel struct {
			ID string `json:"id"`
		} `json:"channel"`
		Live          bool `json:"live"`
		FollowerCount int  `json:"followerCount"`
	} `json:"channels"`
	GeneratedAt string `json:"generatedAt"`
}

type playbackContractResponse struct {
	Channel struct {
		ID string `json:"id"`
	} `json:"channel"`
	Owner struct {
		ID string `json:"id"`
	} `json:"owner"`
	Follow struct {
		Followers int  `json:"followers"`
		Following bool `json:"following"`
	} `json:"follow"`
	Playback *struct {
		SessionID   string `json:"sessionId"`
		PlaybackURL string `json:"playbackUrl"`
		Renditions  []struct {
			ManifestURL string `json:"manifestUrl"`
		} `json:"renditions"`
	} `json:"playback"`
}

type profileContractResponse struct {
	UserID   string `json:"userId"`
	Channels []struct {
		ID string `json:"id"`
	} `json:"channels"`
	LiveChannels []struct {
		ID string `json:"id"`
	} `json:"liveChannels"`
}

type chatMessageContract struct {
	Content string `json:"content"`
}

type ingestStub struct {
	boot ingest.BootResult
}

func (i ingestStub) BootStream(_ context.Context, _ ingest.BootParams) (ingest.BootResult, error) {
	return i.boot, nil
}

func (ingestStub) ShutdownStream(_ context.Context, _ string, _ string, _ []string) error {
	return nil
}

func (ingestStub) HealthChecks(_ context.Context) []ingest.HealthStatus {
	return nil
}

func (ingestStub) TranscodeUpload(_ context.Context, params ingest.UploadTranscodeParams) (ingest.UploadTranscodeResult, error) {
	return ingest.UploadTranscodeResult{PlaybackURL: params.SourceURL}, nil
}

func TestViewerContractEndpoints(t *testing.T) {
	repo, boot := newJSONRepository(t)
	sessionStore := testsupport.NewSessionStoreStub()
	sessions := auth.NewSessionManager(time.Hour, auth.WithStore(sessionStore))
	handler := api.NewHandler(repo, sessions)

	srv, err := server.New(handler, server.Config{})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	ts := httptest.NewServer(serverHandler(t, srv))
	defer ts.Close()

	creator, err := repo.CreateUser(storage.CreateUserParams{DisplayName: "Creator", Email: "creator@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("create creator: %v", err)
	}
	viewer, err := repo.CreateUser(storage.CreateUserParams{DisplayName: "Viewer", Email: "viewer@example.com"})
	if err != nil {
		t.Fatalf("create viewer: %v", err)
	}

	channel, err := repo.CreateChannel(creator.ID, "Chill Beats", "music", []string{"lofi", "study"})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}

	avatar := "https://cdn.example.com/avatar.png"
	bio := "Streaming relaxing tracks"
	if _, err := repo.UpsertProfile(creator.ID, storage.ProfileUpdate{Bio: &bio, AvatarURL: &avatar}); err != nil {
		t.Fatalf("upsert profile: %v", err)
	}

	session, err := repo.StartStream(channel.ID, []string{"720p"})
	if err != nil {
		t.Fatalf("start stream: %v", err)
	}

	if err := repo.FollowChannel(viewer.ID, channel.ID); err != nil {
		t.Fatalf("follow channel: %v", err)
	}

	messages := []string{"first", "second", "third"}
	for _, body := range messages {
		if _, err := repo.CreateChatMessage(channel.ID, viewer.ID, body); err != nil {
			t.Fatalf("create chat message: %v", err)
		}
	}

	token, expires, err := sessions.Create(viewer.ID)
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	sessionCookie := &http.Cookie{Name: "bitriver_session", Value: token, Path: "/", Expires: expires}

	client := ts.Client()

	t.Run("directory", func(t *testing.T) {
		var payload directoryContractResponse
		doGet(t, client, ts.URL+"/api/directory", nil, &payload)
		if len(payload.Channels) != 1 {
			t.Fatalf("expected 1 channel, got %d", len(payload.Channels))
		}
		entry := payload.Channels[0]
		if entry.Channel.ID != channel.ID {
			t.Fatalf("expected channel ID %s, got %s", channel.ID, entry.Channel.ID)
		}
		if !entry.Live {
			t.Fatalf("expected channel to be live")
		}
		if entry.FollowerCount != 1 {
			t.Fatalf("expected follower count 1, got %d", entry.FollowerCount)
		}
	})

	t.Run("playback", func(t *testing.T) {
		var payload playbackContractResponse
		doGet(t, client, ts.URL+"/api/channels/"+channel.ID+"/playback", sessionCookie, &payload)
		if payload.Channel.ID != channel.ID {
			t.Fatalf("expected channel ID %s, got %s", channel.ID, payload.Channel.ID)
		}
		if payload.Owner.ID != creator.ID {
			t.Fatalf("expected owner ID %s, got %s", creator.ID, payload.Owner.ID)
		}
		if payload.Follow.Followers != 1 {
			t.Fatalf("expected followers 1, got %d", payload.Follow.Followers)
		}
		if !payload.Follow.Following {
			t.Fatalf("expected viewer to be following")
		}
		if payload.Playback == nil {
			t.Fatalf("expected playback details present")
		}
		if payload.Playback.SessionID != session.ID {
			t.Fatalf("expected session %s, got %s", session.ID, payload.Playback.SessionID)
		}
		if payload.Playback.PlaybackURL != boot.PlaybackURL {
			t.Fatalf("expected playback URL %s, got %s", boot.PlaybackURL, payload.Playback.PlaybackURL)
		}
		if len(payload.Playback.Renditions) != len(boot.Renditions) {
			t.Fatalf("expected %d renditions, got %d", len(boot.Renditions), len(payload.Playback.Renditions))
		}
		if payload.Playback.Renditions[0].ManifestURL != boot.Renditions[0].ManifestURL {
			t.Fatalf("unexpected rendition manifest: %s", payload.Playback.Renditions[0].ManifestURL)
		}
	})

	t.Run("profile", func(t *testing.T) {
		var payload profileContractResponse
		doGet(t, client, ts.URL+"/api/profiles/"+creator.ID, nil, &payload)
		if payload.UserID != creator.ID {
			t.Fatalf("expected profile user %s, got %s", creator.ID, payload.UserID)
		}
		if len(payload.Channels) != 1 || payload.Channels[0].ID != channel.ID {
			t.Fatalf("expected channel %s in profile, got %+v", channel.ID, payload.Channels)
		}
		if len(payload.LiveChannels) != 1 || payload.LiveChannels[0].ID != channel.ID {
			t.Fatalf("expected live channel %s, got %+v", channel.ID, payload.LiveChannels)
		}
	})

	t.Run("following", func(t *testing.T) {
		var payload directoryContractResponse
		doGet(t, client, ts.URL+"/api/directory/following", sessionCookie, &payload)
		if len(payload.Channels) != 1 {
			t.Fatalf("expected 1 followed channel, got %d", len(payload.Channels))
		}
		if payload.Channels[0].Channel.ID != channel.ID {
			t.Fatalf("expected followed channel %s, got %s", channel.ID, payload.Channels[0].Channel.ID)
		}
	})

	t.Run("chat history", func(t *testing.T) {
		var payload []chatMessageContract
		doGet(t, client, ts.URL+"/api/channels/"+channel.ID+"/chat?limit=2", sessionCookie, &payload)
		if len(payload) != 2 {
			t.Fatalf("expected 2 chat messages, got %d", len(payload))
		}
		if payload[0].Content != "third" || payload[1].Content != "second" {
			t.Fatalf("unexpected chat ordering: %+v", payload)
		}
	})
}

func newJSONRepository(t *testing.T) (storage.Repository, ingest.BootResult) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	boot := ingest.BootResult{
		OriginURL:   "rtmp://origin.example.com/live",
		PlaybackURL: "https://cdn.example.com/live/playlist.m3u8",
		Renditions: []ingest.Rendition{{
			Name:        "720p",
			ManifestURL: "https://cdn.example.com/live/720p.m3u8",
			Bitrate:     2000,
		}},
		JobIDs: []string{"job-live"},
	}
	controller := ingestStub{boot: boot}
	repo, err := storage.NewJSONRepository(path, storage.WithIngestController(controller))
	if err != nil {
		t.Fatalf("new json repository: %v", err)
	}
	return repo, boot
}

func doGet[T any](t *testing.T, client *http.Client, url string, cookie *http.Cookie, dest *T) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func serverHandler(t *testing.T, srv *server.Server) http.Handler {
	t.Helper()
	serverValue := reflect.ValueOf(srv).Elem().FieldByName("httpServer")
	if !serverValue.IsValid() || serverValue.IsNil() {
		t.Fatalf("server handler missing")
	}
	httpServer := (*http.Server)(unsafe.Pointer(serverValue.UnsafePointer()))
	if httpServer == nil || httpServer.Handler == nil {
		t.Fatalf("http server handler not configured")
	}
	return httpServer.Handler
}
