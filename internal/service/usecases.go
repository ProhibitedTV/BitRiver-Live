package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"bitriver-live/internal/domain"
	"bitriver-live/internal/ingest"
	"bitriver-live/internal/observability/metrics"
)

// AuthUsersUseCase encapsulates user and authentication persistence operations
// needed by the API auth/user handlers.
type AuthUsersUseCase interface {
	CreateUser(params domain.UserCreateParams) (domain.User, error)
	AuthenticateUser(email, password string) (domain.User, error)
	AuthenticateOAuth(params domain.OAuthLoginParams) (domain.User, error)
	GetUser(id string) (domain.User, bool)
	ListUsers() []domain.User
	UpdateUser(id string, update domain.UserUpdate) (domain.User, error)
	DeleteUser(id string) error
	UpsertMFASettings(settings domain.MFASettings) (domain.MFASettings, error)
	GetMFASettings(userID string) (domain.MFASettings, bool, error)
	DeleteMFASettings(userID string) error
}

type ChatModerationUseCase interface {
	GetChannel(id string) (domain.Channel, bool)
	GetUser(id string) (domain.User, bool)
	DeleteChatMessage(channelID, messageID string) error
	ListChatMessages(channelID string, limit int) ([]domain.ChatMessage, error)
	CreateChatMessage(channelID, userID, content string) (domain.ChatMessage, error)
	ListChatRestrictions(channelID string) []domain.ChatRestriction
	CreateChatReport(channelID, reporterID, targetID, reason, messageID, evidenceURL string) (domain.ChatReport, error)
	ListChatReports(channelID string, includeResolved bool) ([]domain.ChatReport, error)
	ResolveChatReport(reportID, resolverID, resolution string) (domain.ChatReport, error)
	CreateAppeal(reportID, reporterID, reason string) (domain.Appeal, error)
	ListAppeals(channelID, requesterID string, includeClosed bool) ([]domain.Appeal, error)
	ResolveAppeal(appealID, resolverID, resolution string) (domain.Appeal, error)
	ReopenAppeal(appealID, actorID, note string) (domain.Appeal, error)
	ListChatFilters(channelID string) ([]domain.ChatFilter, error)
	CreateChatFilter(channelID string, params domain.ChatFilterCreateParams) (domain.ChatFilter, error)
	UpdateChatFilter(id string, update domain.ChatFilterUpdate) (domain.ChatFilter, error)
	DeleteChatFilter(id string) error
	ListChannels(ownerID, query string) []domain.Channel
	ListChatAutoModActions(channelID string, limit int) ([]domain.ChatAutoModAction, error)
}

// ChannelsDirectoryUseCase encapsulates channel directory, follow,
// subscription, and playback-oriented persistence operations used by channel
// handlers.
type ChannelsDirectoryUseCase interface {
	ListChannels(ownerID, query string) []domain.Channel
	ListUsers() []domain.User
	ListProfiles() []domain.Profile
	GetChannel(id string) (domain.Channel, bool)
	GetUser(id string) (domain.User, bool)
	GetProfile(userID string) (domain.Profile, bool)
	CountFollowers(channelID string) int
	ListFollowedChannelIDs(userID string) []string
	ListSubscriptions(channelID string, includeInactive bool) ([]domain.Subscription, error)
	CreateChannel(ownerID, title, category string, tags []string) (domain.Channel, error)
	UpdateChannel(id string, update domain.ChannelUpdate) (domain.Channel, error)
	DeleteChannel(id string) error
	CurrentStreamSession(channelID string) (domain.StreamSession, bool)
	ListStreamSessions(channelID string) ([]domain.StreamSession, error)
	FollowChannel(userID, channelID string) error
	UnfollowChannel(userID, channelID string) error
	IsFollowingChannel(userID, channelID string) bool
	CreateSubscription(params domain.SubscriptionCreateParams) (domain.Subscription, error)
	CancelSubscription(id, cancelledBy, reason string) (domain.Subscription, error)
	ListUploads(channelID string) ([]domain.Upload, error)
	GetRecording(id string) (domain.Recording, bool)
}

type LegalComplianceUseCase interface {
	CreateDMCACase(params domain.DMCACaseCreateParams) (domain.DMCACase, error)
	UpdateDMCACase(id, status, notes, actorUserID string) (domain.DMCACase, error)
	ListDMCACases() ([]domain.DMCACase, error)
	GetDMCACase(id string) (domain.DMCACase, bool)
	CreateDataSubjectRequest(params domain.DataSubjectRequestCreateParams) (domain.DataSubjectRequest, error)
	UpdateDataSubjectRequest(id, status, notes, actorUserID string) (domain.DataSubjectRequest, error)
	ListDataSubjectRequests() ([]domain.DataSubjectRequest, error)
	GetDataSubjectRequest(id string) (domain.DataSubjectRequest, bool)
	AddDataSubjectAuditEvent(requestID string, params domain.DataSubjectAuditEventCreateParams) (domain.DataSubjectAuditEvent, error)
	ListDataSubjectAuditEvents(requestID string) ([]domain.DataSubjectAuditEvent, error)
	ListLegalStateHistory(entityType, entityID string) ([]domain.LegalStateHistory, error)
}

type StreamsUseCase interface {
	ListChannels(ownerID, query string) []domain.Channel
	CurrentStreamSession(channelID string) (domain.StreamSession, bool)
	StartStream(channelID string, renditions []string) (domain.StreamSession, error)
	StopStream(channelID string, peakConcurrent int) (domain.StreamSession, error)
	UpdateChannel(id string, update domain.ChannelUpdate) (domain.Channel, error)
	RotateChannelStreamKey(id string) (domain.Channel, error)
}

type ProfilesUseCase interface {
	ListProfiles() []domain.Profile
	GetUser(id string) (domain.User, bool)
	GetProfile(userID string) (domain.Profile, bool)
	UpdateUser(id string, update domain.UserUpdate) (domain.User, error)
	UpsertProfile(userID string, update domain.ProfileUpdate) (domain.Profile, error)
	ListChannels(ownerID, query string) []domain.Channel
}

type AnalyticsUseCase interface {
	ComputeAnalyticsOverview(now time.Time) (AnalyticsOverview, error)
}

type AnalyticsSummary struct {
	LiveViewers      int
	StreamsLive      int
	WatchTimeMinutes float64
	ChatMessages     int
}

type AnalyticsChannelOverview struct {
	ChannelID       string
	Title           string
	LiveViewers     int
	Followers       int
	AvgWatchMinutes float64
	ChatMessages    int
}

type AnalyticsOverview struct {
	Summary    *AnalyticsSummary
	PerChannel []AnalyticsChannelOverview
}

type analyticsOverviewStore interface {
	ListChannels(ownerID, query string) []domain.Channel
	CountFollowers(channelID string) int
	CurrentStreamSession(channelID string) (domain.StreamSession, bool)
	ListStreamSessions(channelID string) ([]domain.StreamSession, error)
	ListChatMessages(channelID string, limit int) ([]domain.ChatMessage, error)
}

type SystemHealthUseCase interface {
	Ping(ctx context.Context) error
	IngestHealth(ctx context.Context) []ingest.HealthStatus
	LastIngestHealth() ([]ingest.HealthStatus, time.Time)
}

type MonetizationUseCase interface {
	ListTips(channelID string, limit int) ([]domain.Tip, error)
	GetSubscription(id string) (domain.Subscription, bool)
	CancelSubscription(id, cancelledBy, reason string) (domain.Subscription, error)
	ListSubscriptions(channelID string, includeInactive bool) ([]domain.Subscription, error)
}

type APIStoreUseCase interface {
	AuthUsersUseCase
	ChannelsDirectoryUseCase
	UploadsUseCase
	RecordingsVODUseCase
	ChatModerationUseCase
	LegalComplianceUseCase
	StreamsUseCase
	ProfilesUseCase
	AnalyticsUseCase
	SystemHealthUseCase
	MonetizationUseCase
}

// UploadsUseCase encapsulates upload lifecycle operations.
type UploadsUseCase interface {
	GetChannel(id string) (domain.Channel, bool)
	ListUploads(channelID string) ([]domain.Upload, error)
	GetUpload(id string) (domain.Upload, bool)
	DeleteUpload(id string) error
	CreateUpload(params domain.UploadCreateParams) (domain.Upload, error)
	UpdateUpload(id string, update domain.UploadUpdate) (domain.Upload, error)
}

// RecordingsVODUseCase encapsulates recording and clip operations.
type RecordingsVODUseCase interface {
	GetChannel(id string) (domain.Channel, bool)
	ListRecordings(channelID string, includeUnpublished bool) ([]domain.Recording, error)
	GetRecording(id string) (domain.Recording, bool)
	PublishRecording(id string) (domain.Recording, error)
	ListClipExports(recordingID string) ([]domain.ClipExport, error)
	CreateClipExport(recordingID string, params domain.ClipExportParams) (domain.ClipExport, error)
	DeleteRecording(id string) error
}

type storeUseCases struct {
	store interface {
		domain.AuthRepository
		domain.ChannelsRepository
		domain.UploadsRepository
		domain.RecordingsRepository
		domain.PaymentsRepository
		domain.LegalRepository
		SystemHealthUseCase
		DeleteChatMessage(channelID, messageID string) error
		ListChatMessages(channelID string, limit int) ([]domain.ChatMessage, error)
		CreateChatMessage(channelID, userID, content string) (domain.ChatMessage, error)
		ListChatRestrictions(channelID string) []domain.ChatRestriction
		CreateChatReport(channelID, reporterID, targetID, reason, messageID, evidenceURL string) (domain.ChatReport, error)
		ListChatReports(channelID string, includeResolved bool) ([]domain.ChatReport, error)
		ResolveChatReport(reportID, resolverID, resolution string) (domain.ChatReport, error)
		CreateAppeal(reportID, reporterID, reason string) (domain.Appeal, error)
		ListAppeals(channelID, requesterID string, includeClosed bool) ([]domain.Appeal, error)
		ResolveAppeal(appealID, resolverID, resolution string) (domain.Appeal, error)
		ReopenAppeal(appealID, actorID, note string) (domain.Appeal, error)
		ListChatFilters(channelID string) ([]domain.ChatFilter, error)
		CreateChatFilter(channelID string, params domain.ChatFilterCreateParams) (domain.ChatFilter, error)
		UpdateChatFilter(id string, update domain.ChatFilterUpdate) (domain.ChatFilter, error)
		DeleteChatFilter(id string) error
		ListChatAutoModActions(channelID string, limit int) ([]domain.ChatAutoModAction, error)
		GetMFASettings(userID string) (domain.MFASettings, bool, error)
		DeleteMFASettings(userID string) error
		StartStream(channelID string, renditions []string) (domain.StreamSession, error)
		StopStream(channelID string, peakConcurrent int) (domain.StreamSession, error)
		RotateChannelStreamKey(id string) (domain.Channel, error)
		UpsertProfile(userID string, update domain.ProfileUpdate) (domain.Profile, error)
		ListTips(channelID string, limit int) ([]domain.Tip, error)
		GetSubscription(id string) (domain.Subscription, bool)
		ListDMCACases() ([]domain.DMCACase, error)
		ListDataSubjectRequests() ([]domain.DataSubjectRequest, error)
		AddDataSubjectAuditEvent(requestID string, params domain.DataSubjectAuditEventCreateParams) (domain.DataSubjectAuditEvent, error)
		ListDataSubjectAuditEvents(requestID string) ([]domain.DataSubjectAuditEvent, error)
		ListLegalStateHistory(entityType, entityID string) ([]domain.LegalStateHistory, error)
	}
}

func NewStoreUseCases(store interface {
	domain.AuthRepository
	domain.ChannelsRepository
	domain.UploadsRepository
	domain.RecordingsRepository
	domain.PaymentsRepository
	domain.LegalRepository
	SystemHealthUseCase
	DeleteChatMessage(channelID, messageID string) error
	ListChatMessages(channelID string, limit int) ([]domain.ChatMessage, error)
	CreateChatMessage(channelID, userID, content string) (domain.ChatMessage, error)
	ListChatRestrictions(channelID string) []domain.ChatRestriction
	CreateChatReport(channelID, reporterID, targetID, reason, messageID, evidenceURL string) (domain.ChatReport, error)
	ListChatReports(channelID string, includeResolved bool) ([]domain.ChatReport, error)
	ResolveChatReport(reportID, resolverID, resolution string) (domain.ChatReport, error)
	CreateAppeal(reportID, reporterID, reason string) (domain.Appeal, error)
	ListAppeals(channelID, requesterID string, includeClosed bool) ([]domain.Appeal, error)
	ResolveAppeal(appealID, resolverID, resolution string) (domain.Appeal, error)
	ReopenAppeal(appealID, actorID, note string) (domain.Appeal, error)
	ListChatFilters(channelID string) ([]domain.ChatFilter, error)
	CreateChatFilter(channelID string, params domain.ChatFilterCreateParams) (domain.ChatFilter, error)
	UpdateChatFilter(id string, update domain.ChatFilterUpdate) (domain.ChatFilter, error)
	DeleteChatFilter(id string) error
	ListChatAutoModActions(channelID string, limit int) ([]domain.ChatAutoModAction, error)
	GetMFASettings(userID string) (domain.MFASettings, bool, error)
	DeleteMFASettings(userID string) error
	StartStream(channelID string, renditions []string) (domain.StreamSession, error)
	StopStream(channelID string, peakConcurrent int) (domain.StreamSession, error)
	RotateChannelStreamKey(id string) (domain.Channel, error)
	UpsertProfile(userID string, update domain.ProfileUpdate) (domain.Profile, error)
	ListTips(channelID string, limit int) ([]domain.Tip, error)
	GetSubscription(id string) (domain.Subscription, bool)
	ListDMCACases() ([]domain.DMCACase, error)
	ListDataSubjectRequests() ([]domain.DataSubjectRequest, error)
	AddDataSubjectAuditEvent(requestID string, params domain.DataSubjectAuditEventCreateParams) (domain.DataSubjectAuditEvent, error)
	ListDataSubjectAuditEvents(requestID string) ([]domain.DataSubjectAuditEvent, error)
	ListLegalStateHistory(entityType, entityID string) ([]domain.LegalStateHistory, error)
}) *storeUseCases {
	return &storeUseCases{store: store}
}

func (s *storeUseCases) CreateUser(params domain.UserCreateParams) (domain.User, error) {
	return s.store.CreateUser(params)
}
func (s *storeUseCases) AuthenticateUser(email, password string) (domain.User, error) {
	return s.store.AuthenticateUser(email, password)
}
func (s *storeUseCases) AuthenticateOAuth(params domain.OAuthLoginParams) (domain.User, error) {
	return s.store.AuthenticateOAuth(params)
}
func (s *storeUseCases) GetUser(id string) (domain.User, bool) { return s.store.GetUser(id) }
func (s *storeUseCases) ListUsers() []domain.User              { return s.store.ListUsers() }
func (s *storeUseCases) UpdateUser(id string, update domain.UserUpdate) (domain.User, error) {
	return s.store.UpdateUser(id, update)
}
func (s *storeUseCases) DeleteUser(id string) error { return s.store.DeleteUser(id) }
func (s *storeUseCases) UpsertMFASettings(settings domain.MFASettings) (domain.MFASettings, error) {
	return s.store.UpsertMFASettings(settings)
}
func (s *storeUseCases) GetMFASettings(userID string) (domain.MFASettings, bool, error) {
	return s.store.GetMFASettings(userID)
}
func (s *storeUseCases) DeleteMFASettings(userID string) error {
	return s.store.DeleteMFASettings(userID)
}

func (s *storeUseCases) ListChannels(ownerID, query string) []domain.Channel {
	return s.store.ListChannels(ownerID, query)
}
func (s *storeUseCases) ListProfiles() []domain.Profile              { return s.store.ListProfiles() }
func (s *storeUseCases) GetChannel(id string) (domain.Channel, bool) { return s.store.GetChannel(id) }
func (s *storeUseCases) GetProfile(userID string) (domain.Profile, bool) {
	return s.store.GetProfile(userID)
}
func (s *storeUseCases) CountFollowers(channelID string) int {
	return s.store.CountFollowers(channelID)
}
func (s *storeUseCases) ListFollowedChannelIDs(userID string) []string {
	return s.store.ListFollowedChannelIDs(userID)
}
func (s *storeUseCases) ListSubscriptions(channelID string, includeInactive bool) ([]domain.Subscription, error) {
	return s.store.ListSubscriptions(channelID, includeInactive)
}
func (s *storeUseCases) CreateChannel(ownerID, title, category string, tags []string) (domain.Channel, error) {
	return s.store.CreateChannel(ownerID, title, category, tags)
}
func (s *storeUseCases) UpdateChannel(id string, update domain.ChannelUpdate) (domain.Channel, error) {
	return s.store.UpdateChannel(id, update)
}
func (s *storeUseCases) DeleteChannel(id string) error { return s.store.DeleteChannel(id) }
func (s *storeUseCases) CurrentStreamSession(channelID string) (domain.StreamSession, bool) {
	return s.store.CurrentStreamSession(channelID)
}
func (s *storeUseCases) ListStreamSessions(channelID string) ([]domain.StreamSession, error) {
	return s.store.ListStreamSessions(channelID)
}
func (s *storeUseCases) ComputeAnalyticsOverview(now time.Time) (AnalyticsOverview, error) {
	return computeAnalyticsOverview(s.store, now)
}

func computeAnalyticsOverview(store analyticsOverviewStore, now time.Time) (AnalyticsOverview, error) {
	channels := store.ListChannels("", "")
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	windowStart := now.Add(-24 * time.Hour)
	summary := AnalyticsSummary{}
	perChannel := make([]AnalyticsChannelOverview, 0, len(channels))
	for _, channel := range channels {
		entry := AnalyticsChannelOverview{
			ChannelID: channel.ID,
			Title:     channel.Title,
			Followers: store.CountFollowers(channel.ID),
		}
		if current, ok := store.CurrentStreamSession(channel.ID); ok {
			entry.LiveViewers = current.PeakConcurrent
		}
		sessions, err := store.ListStreamSessions(channel.ID)
		if err != nil {
			return AnalyticsOverview{}, err
		}
		if len(sessions) > 0 {
			totalMinutes := 0.0
			for _, session := range sessions {
				totalMinutes += sessionDurationMinutes(session, now)
				summary.WatchTimeMinutes += streamWatchOverlapMinutes(session, windowStart, now)
			}
			entry.AvgWatchMinutes = totalMinutes / float64(len(sessions))
		}
		messages, err := store.ListChatMessages(channel.ID, 0)
		if err != nil {
			return AnalyticsOverview{}, err
		}
		today := 0
		for _, message := range messages {
			if message.CreatedAt.Before(startOfDay) {
				break
			}
			today++
		}
		entry.ChatMessages = today
		summary.ChatMessages += today
		summary.LiveViewers += entry.LiveViewers
		perChannel = append(perChannel, entry)
	}
	streamsLive := int(metrics.Default().ActiveStreams())
	if streamsLive <= 0 {
		count := 0
		for _, channel := range channels {
			state := strings.ToLower(strings.TrimSpace(channel.LiveState))
			if state == "live" || state == "starting" {
				count++
			}
		}
		streamsLive = count
	}
	summary.StreamsLive = streamsLive
	sort.Slice(perChannel, func(i, j int) bool {
		if perChannel[i].LiveViewers != perChannel[j].LiveViewers {
			return perChannel[i].LiveViewers > perChannel[j].LiveViewers
		}
		if perChannel[i].Followers != perChannel[j].Followers {
			return perChannel[i].Followers > perChannel[j].Followers
		}
		return perChannel[i].Title < perChannel[j].Title
	})
	resp := AnalyticsOverview{PerChannel: perChannel}
	if len(perChannel) > 0 || summary.LiveViewers > 0 || summary.StreamsLive > 0 || summary.WatchTimeMinutes > 0 || summary.ChatMessages > 0 {
		resp.Summary = &summary
	}
	return resp, nil
}
func (s *storeUseCases) FollowChannel(userID, channelID string) error {
	return s.store.FollowChannel(userID, channelID)
}
func (s *storeUseCases) UnfollowChannel(userID, channelID string) error {
	return s.store.UnfollowChannel(userID, channelID)
}
func (s *storeUseCases) IsFollowingChannel(userID, channelID string) bool {
	return s.store.IsFollowingChannel(userID, channelID)
}
func (s *storeUseCases) CreateSubscription(params domain.SubscriptionCreateParams) (domain.Subscription, error) {
	return s.store.CreateSubscription(params)
}
func (s *storeUseCases) CancelSubscription(id, cancelledBy, reason string) (domain.Subscription, error) {
	return s.store.CancelSubscription(id, cancelledBy, reason)
}
func (s *storeUseCases) ListUploads(channelID string) ([]domain.Upload, error) {
	return s.store.ListUploads(channelID)
}
func (s *storeUseCases) GetRecording(id string) (domain.Recording, bool) {
	return s.store.GetRecording(id)
}
func (s *storeUseCases) GetUpload(id string) (domain.Upload, bool) { return s.store.GetUpload(id) }
func (s *storeUseCases) DeleteUpload(id string) error              { return s.store.DeleteUpload(id) }
func (s *storeUseCases) CreateUpload(params domain.UploadCreateParams) (domain.Upload, error) {
	return s.store.CreateUpload(params)
}
func (s *storeUseCases) UpdateUpload(id string, update domain.UploadUpdate) (domain.Upload, error) {
	return s.store.UpdateUpload(id, update)
}
func (s *storeUseCases) ListRecordings(channelID string, includeUnpublished bool) ([]domain.Recording, error) {
	return s.store.ListRecordings(channelID, includeUnpublished)
}
func (s *storeUseCases) PublishRecording(id string) (domain.Recording, error) {
	return s.store.PublishRecording(id)
}
func (s *storeUseCases) ListClipExports(recordingID string) ([]domain.ClipExport, error) {
	return s.store.ListClipExports(recordingID)
}
func (s *storeUseCases) CreateClipExport(recordingID string, params domain.ClipExportParams) (domain.ClipExport, error) {
	return s.store.CreateClipExport(recordingID, params)
}
func (s *storeUseCases) DeleteRecording(id string) error { return s.store.DeleteRecording(id) }

func (s *storeUseCases) Ping(ctx context.Context) error { return s.store.Ping(ctx) }
func (s *storeUseCases) IngestHealth(ctx context.Context) []ingest.HealthStatus {
	return s.store.IngestHealth(ctx)
}
func (s *storeUseCases) LastIngestHealth() ([]ingest.HealthStatus, time.Time) {
	return s.store.LastIngestHealth()
}

func (s *storeUseCases) DeleteChatMessage(channelID, messageID string) error {
	return s.store.DeleteChatMessage(channelID, messageID)
}
func (s *storeUseCases) ListChatMessages(channelID string, limit int) ([]domain.ChatMessage, error) {
	return s.store.ListChatMessages(channelID, limit)
}
func (s *storeUseCases) CreateChatMessage(channelID, userID, content string) (domain.ChatMessage, error) {
	return s.store.CreateChatMessage(channelID, userID, content)
}
func (s *storeUseCases) ListChatRestrictions(channelID string) []domain.ChatRestriction {
	return s.store.ListChatRestrictions(channelID)
}
func (s *storeUseCases) CreateChatReport(channelID, reporterID, targetID, reason, messageID, evidenceURL string) (domain.ChatReport, error) {
	return s.store.CreateChatReport(channelID, reporterID, targetID, reason, messageID, evidenceURL)
}
func (s *storeUseCases) ListChatReports(channelID string, includeResolved bool) ([]domain.ChatReport, error) {
	return s.store.ListChatReports(channelID, includeResolved)
}
func (s *storeUseCases) ResolveChatReport(reportID, resolverID, resolution string) (domain.ChatReport, error) {
	return s.store.ResolveChatReport(reportID, resolverID, resolution)
}
func (s *storeUseCases) CreateAppeal(reportID, reporterID, reason string) (domain.Appeal, error) {
	return s.store.CreateAppeal(reportID, reporterID, reason)
}
func (s *storeUseCases) ListAppeals(channelID, requesterID string, includeClosed bool) ([]domain.Appeal, error) {
	return s.store.ListAppeals(channelID, requesterID, includeClosed)
}
func (s *storeUseCases) ResolveAppeal(appealID, resolverID, resolution string) (domain.Appeal, error) {
	return s.store.ResolveAppeal(appealID, resolverID, resolution)
}
func (s *storeUseCases) ReopenAppeal(appealID, actorID, note string) (domain.Appeal, error) {
	return s.store.ReopenAppeal(appealID, actorID, note)
}
func (s *storeUseCases) ListChatFilters(channelID string) ([]domain.ChatFilter, error) {
	return s.store.ListChatFilters(channelID)
}
func (s *storeUseCases) CreateChatFilter(channelID string, params domain.ChatFilterCreateParams) (domain.ChatFilter, error) {
	return s.store.CreateChatFilter(channelID, params)
}
func (s *storeUseCases) UpdateChatFilter(id string, update domain.ChatFilterUpdate) (domain.ChatFilter, error) {
	return s.store.UpdateChatFilter(id, update)
}
func (s *storeUseCases) DeleteChatFilter(id string) error { return s.store.DeleteChatFilter(id) }
func (s *storeUseCases) ListChatAutoModActions(channelID string, limit int) ([]domain.ChatAutoModAction, error) {
	return s.store.ListChatAutoModActions(channelID, limit)
}

func (s *storeUseCases) StartStream(channelID string, renditions []string) (domain.StreamSession, error) {
	return s.store.StartStream(channelID, renditions)
}
func (s *storeUseCases) StopStream(channelID string, peakConcurrent int) (domain.StreamSession, error) {
	return s.store.StopStream(channelID, peakConcurrent)
}
func (s *storeUseCases) RotateChannelStreamKey(id string) (domain.Channel, error) {
	return s.store.RotateChannelStreamKey(id)
}

func (s *storeUseCases) UpsertProfile(userID string, update domain.ProfileUpdate) (domain.Profile, error) {
	return s.store.UpsertProfile(userID, update)
}

func (s *storeUseCases) ListTips(channelID string, limit int) ([]domain.Tip, error) {
	return s.store.ListTips(channelID, limit)
}

func sessionDurationMinutes(session domain.StreamSession, now time.Time) float64 {
	end := now
	if session.EndedAt != nil && session.EndedAt.Before(end) {
		end = *session.EndedAt
	}
	if end.Before(session.StartedAt) {
		return 0
	}
	return end.Sub(session.StartedAt).Minutes()
}

func streamWatchOverlapMinutes(session domain.StreamSession, windowStart, windowEnd time.Time) float64 {
	start := session.StartedAt
	if start.Before(windowStart) {
		start = windowStart
	}
	end := windowEnd
	if session.EndedAt != nil && session.EndedAt.Before(end) {
		end = *session.EndedAt
	}
	if end.Before(windowStart) {
		return 0
	}
	if end.After(windowEnd) {
		end = windowEnd
	}
	if !end.After(start) {
		return 0
	}
	return end.Sub(start).Minutes()
}
func (s *storeUseCases) GetSubscription(id string) (domain.Subscription, bool) {
	return s.store.GetSubscription(id)
}

func (s *storeUseCases) ListDMCACases() ([]domain.DMCACase, error) { return s.store.ListDMCACases() }

func (s *storeUseCases) GetDMCACase(id string) (domain.DMCACase, bool) {
	return s.store.GetDMCACase(id)
}
func (s *storeUseCases) GetDataSubjectRequest(id string) (domain.DataSubjectRequest, bool) {
	return s.store.GetDataSubjectRequest(id)
}
func (s *storeUseCases) ListDataSubjectRequests() ([]domain.DataSubjectRequest, error) {
	return s.store.ListDataSubjectRequests()
}
func (s *storeUseCases) AddDataSubjectAuditEvent(requestID string, params domain.DataSubjectAuditEventCreateParams) (domain.DataSubjectAuditEvent, error) {
	return s.store.AddDataSubjectAuditEvent(requestID, params)
}
func (s *storeUseCases) ListDataSubjectAuditEvents(requestID string) ([]domain.DataSubjectAuditEvent, error) {
	return s.store.ListDataSubjectAuditEvents(requestID)
}
func (s *storeUseCases) ListLegalStateHistory(entityType, entityID string) ([]domain.LegalStateHistory, error) {
	return s.store.ListLegalStateHistory(entityType, entityID)
}

func (s *storeUseCases) CreateDMCACase(params domain.DMCACaseCreateParams) (domain.DMCACase, error) {
	return s.store.CreateDMCACase(params)
}
func (s *storeUseCases) UpdateDMCACase(id, status, notes, actorUserID string) (domain.DMCACase, error) {
	return s.store.UpdateDMCACase(id, domain.DMCACaseUpdate{Status: &status, Notes: &notes}, actorUserID)
}
func (s *storeUseCases) CreateDataSubjectRequest(params domain.DataSubjectRequestCreateParams) (domain.DataSubjectRequest, error) {
	return s.store.CreateDataSubjectRequest(params)
}
func (s *storeUseCases) UpdateDataSubjectRequest(id, status, notes, actorUserID string) (domain.DataSubjectRequest, error) {
	return s.store.UpdateDataSubjectRequest(id, domain.DataSubjectRequestUpdate{Status: &status, Notes: &notes}, actorUserID)
}

var (
	_ AuthUsersUseCase         = (*storeUseCases)(nil)
	_ ChannelsDirectoryUseCase = (*storeUseCases)(nil)
	_ UploadsUseCase           = (*storeUseCases)(nil)
	_ RecordingsVODUseCase     = (*storeUseCases)(nil)
	_ ChatModerationUseCase    = (*storeUseCases)(nil)
	_ StreamsUseCase           = (*storeUseCases)(nil)
	_ ProfilesUseCase          = (*storeUseCases)(nil)
	_ AnalyticsUseCase         = (*storeUseCases)(nil)
	_ SystemHealthUseCase      = (*storeUseCases)(nil)
	_ MonetizationUseCase      = (*storeUseCases)(nil)
	_ APIStoreUseCase          = (*storeUseCases)(nil)
)
