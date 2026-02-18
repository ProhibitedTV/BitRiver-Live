package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"bitriver-live/internal/domain"
)

// Snapshot captures a complete JSON-serialisable view of the in-memory
// datastore, grouping each model collection by its primary identifier so it can
// be persisted and later replayed into another backing store.
type Snapshot struct {
	Users               map[string]domain.User                    `json:"users"`
	MFASettings         map[string]domain.MFASettings             `json:"mfaSettings"`
	OAuthAccounts       map[string]domain.OAuthAccount            `json:"oauthAccounts"`
	Channels            map[string]domain.Channel                 `json:"channels"`
	StreamSessions      map[string]domain.StreamSession           `json:"streamSessions"`
	ChatMessages        map[string]domain.ChatMessage             `json:"chatMessages"`
	ChatBans            map[string]map[string]time.Time           `json:"chatBans"`
	ChatTimeouts        map[string]map[string]time.Time           `json:"chatTimeouts"`
	ChatBanActors       map[string]map[string]string              `json:"chatBanActors"`
	ChatBanReasons      map[string]map[string]string              `json:"chatBanReasons"`
	ChatTimeoutActors   map[string]map[string]string              `json:"chatTimeoutActors"`
	ChatTimeoutReasons  map[string]map[string]string              `json:"chatTimeoutReasons"`
	ChatTimeoutIssuedAt map[string]map[string]time.Time           `json:"chatTimeoutIssuedAt"`
	ChatReports         map[string]domain.ChatReport              `json:"chatReports"`
	Appeals             map[string]domain.Appeal                  `json:"appeals"`
	AppealEvents        map[string][]domain.AppealEvent           `json:"appealEvents"`
	ChatFilters         map[string]domain.ChatFilter              `json:"chatFilters"`
	ChatAutoModActions  map[string]domain.ChatAutoModAction       `json:"chatAutoModActions"`
	Tips                map[string]domain.Tip                     `json:"tips"`
	Subscriptions       map[string]domain.Subscription            `json:"subscriptions"`
	PaymentTransactions map[string]domain.PaymentTransaction      `json:"paymentTransactions"`
	Profiles            map[string]domain.Profile                 `json:"profiles"`
	Follows             map[string]map[string]time.Time           `json:"follows"`
	Recordings          map[string]domain.Recording               `json:"recordings"`
	Uploads             map[string]domain.Upload                  `json:"uploads"`
	ClipExports         map[string]domain.ClipExport              `json:"clipExports"`
	DMCACases           map[string]domain.DMCACase                `json:"dmcaCases"`
	DataSubjectRequests map[string]domain.DataSubjectRequest      `json:"dataSubjectRequests"`
	DataSubjectAudit    map[string][]domain.DataSubjectAuditEvent `json:"dataSubjectAudit"`
	LegalStateHistory   []domain.LegalStateHistory                `json:"legalStateHistory"`
}

// SnapshotCounts summarises the size of each collection stored in a Snapshot to
// help operators understand how much data will be serialised and imported.
type SnapshotCounts struct {
	Users                  int
	MFASettings            int
	OAuthAccounts          int
	Channels               int
	StreamSessions         int
	StreamSessionManifests int
	ChatMessages           int
	ChatBans               int
	ChatTimeouts           int
	ChatReports            int
	Appeals                int
	AppealEvents           int
	ChatFilters            int
	ChatAutoModActions     int
	Tips                   int
	Subscriptions          int
	PaymentTransactions    int
	Profiles               int
	Follows                int
	Recordings             int
	RecordingRenditions    int
	RecordingThumbnails    int
	Uploads                int
	ClipExports            int
	DMCACases              int
	DataSubjectRequests    int
	DataSubjectAuditEvents int
	LegalStateHistory      int
}

// LoadSnapshotFromJSON reads a previously exported Snapshot from disk,
// rehydrating the datastore state serialised in JSON so it can be imported or
// inspected.
func LoadSnapshotFromJSON(path string) (*Snapshot, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open snapshot %s: %w", path, err)
	}
	defer func() {
		_ = file.Close()
	}()

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

// ensureInitialized performs ensure initialized and propagates validation or dependency failures to the caller.
func (s *Snapshot) ensureInitialized() {
	if s.Users == nil {
		s.Users = make(map[string]domain.User)
	}
	if s.MFASettings == nil {
		s.MFASettings = make(map[string]domain.MFASettings)
	}
	if s.OAuthAccounts == nil {
		s.OAuthAccounts = make(map[string]domain.OAuthAccount)
	}
	if s.Channels == nil {
		s.Channels = make(map[string]domain.Channel)
	}
	if s.StreamSessions == nil {
		s.StreamSessions = make(map[string]domain.StreamSession)
	}
	if s.ChatMessages == nil {
		s.ChatMessages = make(map[string]domain.ChatMessage)
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
		s.ChatReports = make(map[string]domain.ChatReport)
	}
	if s.Appeals == nil {
		s.Appeals = make(map[string]domain.Appeal)
	}
	if s.AppealEvents == nil {
		s.AppealEvents = make(map[string][]domain.AppealEvent)
	}
	if s.ChatFilters == nil {
		s.ChatFilters = make(map[string]domain.ChatFilter)
	}
	if s.ChatAutoModActions == nil {
		s.ChatAutoModActions = make(map[string]domain.ChatAutoModAction)
	}
	if s.Tips == nil {
		s.Tips = make(map[string]domain.Tip)
	}
	if s.Subscriptions == nil {
		s.Subscriptions = make(map[string]domain.Subscription)
	}
	if s.PaymentTransactions == nil {
		s.PaymentTransactions = make(map[string]domain.PaymentTransaction)
	}
	if s.Profiles == nil {
		s.Profiles = make(map[string]domain.Profile)
	}
	if s.Follows == nil {
		s.Follows = make(map[string]map[string]time.Time)
	}
	if s.Recordings == nil {
		s.Recordings = make(map[string]domain.Recording)
	}
	if s.Uploads == nil {
		s.Uploads = make(map[string]domain.Upload)
	}
	if s.ClipExports == nil {
		s.ClipExports = make(map[string]domain.ClipExport)
	}
	if s.DMCACases == nil {
		s.DMCACases = make(map[string]domain.DMCACase)
	}
	if s.DataSubjectRequests == nil {
		s.DataSubjectRequests = make(map[string]domain.DataSubjectRequest)
	}
	if s.DataSubjectAudit == nil {
		s.DataSubjectAudit = make(map[string][]domain.DataSubjectAuditEvent)
	}
	if s.LegalStateHistory == nil {
		s.LegalStateHistory = []domain.LegalStateHistory{}
	}
}

// Counts walks a Snapshot and returns the SnapshotCounts summary reflecting
// how many entities of each type will be serialised for import.
func (s *Snapshot) Counts() SnapshotCounts {
	if s == nil {
		return SnapshotCounts{}
	}
	counts := SnapshotCounts{
		Users:               len(s.Users),
		MFASettings:         len(s.MFASettings),
		OAuthAccounts:       len(s.OAuthAccounts),
		Channels:            len(s.Channels),
		StreamSessions:      len(s.StreamSessions),
		ChatMessages:        len(s.ChatMessages),
		ChatReports:         len(s.ChatReports),
		Appeals:             len(s.Appeals),
		ChatFilters:         len(s.ChatFilters),
		ChatAutoModActions:  len(s.ChatAutoModActions),
		Tips:                len(s.Tips),
		Subscriptions:       len(s.Subscriptions),
		PaymentTransactions: len(s.PaymentTransactions),
		Profiles:            len(s.Profiles),
		Recordings:          len(s.Recordings),
		Uploads:             len(s.Uploads),
		ClipExports:         len(s.ClipExports),
		DMCACases:           len(s.DMCACases),
		DataSubjectRequests: len(s.DataSubjectRequests),
		LegalStateHistory:   len(s.LegalStateHistory),
	}
	for _, entries := range s.DataSubjectAudit {
		counts.DataSubjectAuditEvents += len(entries)
	}
	for _, follows := range s.Follows {
		counts.Follows += len(follows)
	}
	for _, events := range s.AppealEvents {
		counts.AppealEvents += len(events)
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
