package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"bitriver-live/internal/models"
)

// Snapshot captures a complete JSON-serialisable view of the in-memory
// datastore, grouping each model collection by its primary identifier so it can
// be persisted and later replayed into another backing store.
type Snapshot struct {
	Users               map[string]models.User          `json:"users"`
	OAuthAccounts       map[string]models.OAuthAccount  `json:"oauthAccounts"`
	Channels            map[string]models.Channel       `json:"channels"`
	StreamSessions      map[string]models.StreamSession `json:"streamSessions"`
	ChatMessages        map[string]models.ChatMessage   `json:"chatMessages"`
	ChatBans            map[string]map[string]time.Time `json:"chatBans"`
	ChatTimeouts        map[string]map[string]time.Time `json:"chatTimeouts"`
	ChatBanActors       map[string]map[string]string    `json:"chatBanActors"`
	ChatBanReasons      map[string]map[string]string    `json:"chatBanReasons"`
	ChatTimeoutActors   map[string]map[string]string    `json:"chatTimeoutActors"`
	ChatTimeoutReasons  map[string]map[string]string    `json:"chatTimeoutReasons"`
	ChatTimeoutIssuedAt map[string]map[string]time.Time `json:"chatTimeoutIssuedAt"`
	ChatReports         map[string]models.ChatReport    `json:"chatReports"`
	Tips                map[string]models.Tip           `json:"tips"`
	Subscriptions       map[string]models.Subscription  `json:"subscriptions"`
	Profiles            map[string]models.Profile       `json:"profiles"`
	Follows             map[string]map[string]time.Time `json:"follows"`
	Recordings          map[string]models.Recording     `json:"recordings"`
	Uploads             map[string]models.Upload        `json:"uploads"`
	ClipExports         map[string]models.ClipExport    `json:"clipExports"`
}

// SnapshotCounts summarises the size of each collection stored in a Snapshot to
// help operators understand how much data will be serialised and imported.
type SnapshotCounts struct {
	Users                  int
	OAuthAccounts          int
	Channels               int
	StreamSessions         int
	StreamSessionManifests int
	ChatMessages           int
	ChatBans               int
	ChatTimeouts           int
	ChatReports            int
	Tips                   int
	Subscriptions          int
	Profiles               int
	Follows                int
	Recordings             int
	RecordingRenditions    int
	RecordingThumbnails    int
	Uploads                int
	ClipExports            int
}

// LoadSnapshotFromJSON reads a previously exported Snapshot from disk,
// rehydrating the datastore state serialised in JSON so it can be imported or
// inspected.
func LoadSnapshotFromJSON(path string) (*Snapshot, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open snapshot %s: %w", path, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	var snapshot Snapshot
	if err := decoder.Decode(&snapshot); err != nil {
		if err == io.EOF {
			snapshot.ensureInitialized()
			return &snapshot, nil
		}
		return nil, fmt.Errorf("decode snapshot %s: %w", path, err)
	}
	snapshot.ensureInitialized()
	return &snapshot, nil
}

func (s *Snapshot) ensureInitialized() {
	if s.Users == nil {
		s.Users = make(map[string]models.User)
	}
	if s.OAuthAccounts == nil {
		s.OAuthAccounts = make(map[string]models.OAuthAccount)
	}
	if s.Channels == nil {
		s.Channels = make(map[string]models.Channel)
	}
	if s.StreamSessions == nil {
		s.StreamSessions = make(map[string]models.StreamSession)
	}
	if s.ChatMessages == nil {
		s.ChatMessages = make(map[string]models.ChatMessage)
	}
	if s.ChatBans == nil {
		s.ChatBans = make(map[string]map[string]time.Time)
	}
	if s.ChatTimeouts == nil {
		s.ChatTimeouts = make(map[string]map[string]time.Time)
	}
	if s.ChatBanActors == nil {
		s.ChatBanActors = make(map[string]map[string]string)
	}
	if s.ChatBanReasons == nil {
		s.ChatBanReasons = make(map[string]map[string]string)
	}
	if s.ChatTimeoutActors == nil {
		s.ChatTimeoutActors = make(map[string]map[string]string)
	}
	if s.ChatTimeoutReasons == nil {
		s.ChatTimeoutReasons = make(map[string]map[string]string)
	}
	if s.ChatTimeoutIssuedAt == nil {
		s.ChatTimeoutIssuedAt = make(map[string]map[string]time.Time)
	}
	if s.ChatReports == nil {
		s.ChatReports = make(map[string]models.ChatReport)
	}
	if s.Tips == nil {
		s.Tips = make(map[string]models.Tip)
	}
	if s.Subscriptions == nil {
		s.Subscriptions = make(map[string]models.Subscription)
	}
	if s.Profiles == nil {
		s.Profiles = make(map[string]models.Profile)
	}
	if s.Follows == nil {
		s.Follows = make(map[string]map[string]time.Time)
	}
	if s.Recordings == nil {
		s.Recordings = make(map[string]models.Recording)
	}
	if s.Uploads == nil {
		s.Uploads = make(map[string]models.Upload)
	}
	if s.ClipExports == nil {
		s.ClipExports = make(map[string]models.ClipExport)
	}
}

// Counts walks a Snapshot and returns the SnapshotCounts summary reflecting
// how many entities of each type will be serialised for import.
func (s *Snapshot) Counts() SnapshotCounts {
	if s == nil {
		return SnapshotCounts{}
	}
	counts := SnapshotCounts{
		Users:          len(s.Users),
		OAuthAccounts:  len(s.OAuthAccounts),
		Channels:       len(s.Channels),
		StreamSessions: len(s.StreamSessions),
		ChatMessages:   len(s.ChatMessages),
		ChatReports:    len(s.ChatReports),
		Tips:           len(s.Tips),
		Subscriptions:  len(s.Subscriptions),
		Profiles:       len(s.Profiles),
		Recordings:     len(s.Recordings),
		Uploads:        len(s.Uploads),
		ClipExports:    len(s.ClipExports),
	}
	for _, follows := range s.Follows {
		counts.Follows += len(follows)
	}
	for _, bans := range s.ChatBans {
		counts.ChatBans += len(bans)
	}
	for _, timeouts := range s.ChatTimeouts {
		counts.ChatTimeouts += len(timeouts)
	}
	for _, session := range s.StreamSessions {
		counts.StreamSessionManifests += len(session.RenditionManifests)
	}
	for _, recording := range s.Recordings {
		counts.RecordingRenditions += len(recording.Renditions)
		counts.RecordingThumbnails += len(recording.Thumbnails)
	}
	return counts
}

// ImportSnapshotToPostgres hands a Snapshot to the postgresRepository so the
// serialised datastore state can be bulk-loaded into Postgres.
func ImportSnapshotToPostgres(ctx context.Context, repo Repository, snapshot *Snapshot) error {
	if snapshot == nil {
		return fmt.Errorf("snapshot is required")
	}
	pgRepo, ok := repo.(*postgresRepository)
	if !ok {
		return fmt.Errorf("postgres repository required for snapshot import")
	}
	snapshot.ensureInitialized()
	return pgRepo.importSnapshot(ctx, snapshot)
}
