package storage

import (
	"time"

	"bitriver-live/internal/domain"
)

// newDataset executes newDataset.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: no transaction/connection contract applies for this pure helper.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func newDataset() dataset {
	ds := dataset{
		Users:               make(map[string]domain.User),
		MFASettings:         make(map[string]domain.MFASettings),
		OAuthAccounts:       make(map[string]domain.OAuthAccount),
		Channels:            make(map[string]domain.Channel),
		StreamSessions:      make(map[string]domain.StreamSession),
		Tips:                make(map[string]domain.Tip),
		Subscriptions:       make(map[string]domain.Subscription),
		PaymentTransactions: make(map[string]domain.PaymentTransaction),
		Profiles:            make(map[string]domain.Profile),
		Follows:             make(map[string]map[string]time.Time),
		Recordings:          make(map[string]domain.Recording),
		ClipExports:         make(map[string]domain.ClipExport),
	}
	initChatDataset(&ds)
	return ds
}

// ensureDatasetInitializedLocked executes ensureDatasetInitializedLocked.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: no external transaction is required; it coordinates access with
// the in-memory mutex and persists snapshots to disk/object storage as needed.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (s *Storage) ensureDatasetInitializedLocked() {
	if s.data.Users == nil {
		s.data.Users = make(map[string]domain.User)
	}
	if s.data.MFASettings == nil {
		s.data.MFASettings = make(map[string]domain.MFASettings)
	}
	if s.data.OAuthAccounts == nil {
		s.data.OAuthAccounts = make(map[string]domain.OAuthAccount)
	}
	if s.data.Channels == nil {
		s.data.Channels = make(map[string]domain.Channel)
	}
	if s.data.StreamSessions == nil {
		s.data.StreamSessions = make(map[string]domain.StreamSession)
	}
	s.ensureChatDatasetInitializedLocked()
	if s.data.Tips == nil {
		s.data.Tips = make(map[string]domain.Tip)
	}
	if s.data.Subscriptions == nil {
		s.data.Subscriptions = make(map[string]domain.Subscription)
	}
	if s.data.PaymentTransactions == nil {
		s.data.PaymentTransactions = make(map[string]domain.PaymentTransaction)
	}
	if s.data.Profiles == nil {
		s.data.Profiles = make(map[string]domain.Profile)
	}
	if s.data.Follows == nil {
		s.data.Follows = make(map[string]map[string]time.Time)
	}
	if s.data.Recordings == nil {
		s.data.Recordings = make(map[string]domain.Recording)
	}
	if s.data.Uploads == nil {
		s.data.Uploads = make(map[string]domain.Upload)
	}
	if s.data.ClipExports == nil {
		s.data.ClipExports = make(map[string]domain.ClipExport)
	}
	if s.data.DMCACases == nil {
		s.data.DMCACases = make(map[string]domain.DMCACase)
	}
	if s.data.DataSubjectRequests == nil {
		s.data.DataSubjectRequests = make(map[string]domain.DataSubjectRequest)
	}
	if s.data.DataSubjectAudit == nil {
		s.data.DataSubjectAudit = make(map[string][]domain.DataSubjectAuditEvent)
	}
	if s.data.LegalStateHistory == nil {
		s.data.LegalStateHistory = []domain.LegalStateHistory{}
	}
}

// buildObjectKey executes buildObjectKey.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: no transaction/connection contract applies for this pure helper.

func cloneDataset(src dataset) dataset {
	clone := dataset{}

	if src.Users != nil {
		clone.Users = make(map[string]domain.User, len(src.Users))
		for id, user := range src.Users {
			cloned := user
			if user.Roles != nil {
				cloned.Roles = append([]string(nil), user.Roles...)
			}
			clone.Users[id] = cloned
		}
	}

	if src.MFASettings != nil {
		clone.MFASettings = make(map[string]domain.MFASettings, len(src.MFASettings))
		for userID, settings := range src.MFASettings {
			cloned := settings
			if settings.RecoveryCodes != nil {
				cloned.RecoveryCodes = append([]string(nil), settings.RecoveryCodes...)
			}
			if settings.EnabledAt != nil {
				enabledAt := *settings.EnabledAt
				cloned.EnabledAt = &enabledAt
			}
			if settings.LastUsedAt != nil {
				lastUsed := *settings.LastUsedAt
				cloned.LastUsedAt = &lastUsed
			}
			clone.MFASettings[userID] = cloned
		}
	}

	if src.OAuthAccounts != nil {
		clone.OAuthAccounts = make(map[string]domain.OAuthAccount, len(src.OAuthAccounts))
		for key, account := range src.OAuthAccounts {
			clone.OAuthAccounts[key] = account
		}
	}

	if src.Channels != nil {
		clone.Channels = make(map[string]domain.Channel, len(src.Channels))
		for id, channel := range src.Channels {
			cloned := channel
			if channel.Tags != nil {
				cloned.Tags = append([]string(nil), channel.Tags...)
			}
			cloned.Schedule = cloneChannelSchedule(channel.Schedule)
			if channel.CurrentSessionID != nil {
				current := *channel.CurrentSessionID
				cloned.CurrentSessionID = &current
			}
			clone.Channels[id] = cloned
		}
	}

	if src.StreamSessions != nil {
		clone.StreamSessions = make(map[string]domain.StreamSession, len(src.StreamSessions))
		for id, session := range src.StreamSessions {
			cloned := session
			if session.Renditions != nil {
				cloned.Renditions = append([]string(nil), session.Renditions...)
			}
			if session.EndedAt != nil {
				ended := *session.EndedAt
				cloned.EndedAt = &ended
			}
			clone.StreamSessions[id] = cloned
		}
	}

	cloneChatData(src, &clone)

	if src.Tips != nil {
		clone.Tips = make(map[string]domain.Tip, len(src.Tips))
		for id, tip := range src.Tips {
			clone.Tips[id] = tip
		}
	}

	if src.Subscriptions != nil {
		clone.Subscriptions = make(map[string]domain.Subscription, len(src.Subscriptions))
		for id, subscription := range src.Subscriptions {
			cloned := subscription
			if subscription.CancelledAt != nil {
				cancelled := *subscription.CancelledAt
				cloned.CancelledAt = &cancelled
			}
			clone.Subscriptions[id] = cloned
		}
	}

	if src.Recordings != nil {
		clone.Recordings = make(map[string]domain.Recording, len(src.Recordings))
		for id, recording := range src.Recordings {
			clone.Recordings[id] = cloneRecording(recording)
		}
	}

	if src.Uploads != nil {
		clone.Uploads = make(map[string]domain.Upload, len(src.Uploads))
		for id, upload := range src.Uploads {
			clone.Uploads[id] = cloneUpload(upload)
		}
	}

	if src.ClipExports != nil {
		clone.ClipExports = make(map[string]domain.ClipExport, len(src.ClipExports))
		for id, clip := range src.ClipExports {
			clone.ClipExports[id] = cloneClipExport(clip)
		}
	}

	if src.Profiles != nil {
		clone.Profiles = make(map[string]domain.Profile, len(src.Profiles))
		for id, profile := range src.Profiles {
			cloned := profile
			if profile.SocialLinks != nil {
				cloned.SocialLinks = append([]domain.SocialLink(nil), profile.SocialLinks...)
			}
			if profile.TopFriends != nil {
				cloned.TopFriends = append([]string(nil), profile.TopFriends...)
			}
			if profile.DonationAddresses != nil {
				cloned.DonationAddresses = append([]domain.CryptoAddress(nil), profile.DonationAddresses...)
			}
			if profile.FeaturedChannelID != nil {
				featured := *profile.FeaturedChannelID
				cloned.FeaturedChannelID = &featured
			}
			clone.Profiles[id] = cloned
		}
	}

	if src.DMCACases != nil {
		clone.DMCACases = make(map[string]domain.DMCACase, len(src.DMCACases))
		for id, rec := range src.DMCACases {
			clone.DMCACases[id] = rec
		}
	}

	if src.DataSubjectRequests != nil {
		clone.DataSubjectRequests = make(map[string]domain.DataSubjectRequest, len(src.DataSubjectRequests))
		for id, rec := range src.DataSubjectRequests {
			clone.DataSubjectRequests[id] = rec
		}
	}

	if src.DataSubjectAudit != nil {
		clone.DataSubjectAudit = make(map[string][]domain.DataSubjectAuditEvent, len(src.DataSubjectAudit))
		for requestID, entries := range src.DataSubjectAudit {
			clone.DataSubjectAudit[requestID] = append([]domain.DataSubjectAuditEvent(nil), entries...)
		}
	}

	if src.LegalStateHistory != nil {
		clone.LegalStateHistory = append([]domain.LegalStateHistory(nil), src.LegalStateHistory...)
	}
	if src.Follows != nil {
		clone.Follows = make(map[string]map[string]time.Time, len(src.Follows))
		for userID, channels := range src.Follows {
			if channels == nil {
				clone.Follows[userID] = nil
				continue
			}
			followed := make(map[string]time.Time, len(channels))
			for channelID, followedAt := range channels {
				followed[channelID] = followedAt
			}
			clone.Follows[userID] = followed
		}
	}

	return clone
}

// User operations
