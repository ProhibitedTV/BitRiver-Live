package models

import "bitriver-live/internal/domain"

const (
	DMCACaseStatusOpen     = domain.DMCACaseStatusOpen
	DMCACaseStatusActioned = domain.DMCACaseStatusActioned
	DMCACaseStatusRestored = domain.DMCACaseStatusRestored
	DMCACaseStatusRejected = domain.DMCACaseStatusRejected

	DataSubjectRequestTypeExport = domain.DataSubjectRequestTypeExport
	DataSubjectRequestTypeDelete = domain.DataSubjectRequestTypeDelete

	DataSubjectRequestStatusOpen     = domain.DataSubjectRequestStatusOpen
	DataSubjectRequestStatusActioned = domain.DataSubjectRequestStatusActioned
	DataSubjectRequestStatusRejected = domain.DataSubjectRequestStatusRejected

	PaymentStatePending   = domain.PaymentStatePending
	PaymentStateConfirmed = domain.PaymentStateConfirmed
	PaymentStateFailed    = domain.PaymentStateFailed
	PaymentStateRefunded  = domain.PaymentStateRefunded
)

type (
	Money                 = domain.Money
	User                  = domain.User
	MFASettings           = domain.MFASettings
	OAuthAccount          = domain.OAuthAccount
	Channel               = domain.Channel
	StreamSession         = domain.StreamSession
	RenditionManifest     = domain.RenditionManifest
	Recording             = domain.Recording
	RecordingRendition    = domain.RecordingRendition
	RecordingThumbnail    = domain.RecordingThumbnail
	Upload                = domain.Upload
	DMCACase              = domain.DMCACase
	LegalStateHistory     = domain.LegalStateHistory
	DataSubjectRequest    = domain.DataSubjectRequest
	DataSubjectAuditEvent = domain.DataSubjectAuditEvent
	ClipExport            = domain.ClipExport
	ClipExportSummary     = domain.ClipExportSummary
	ChatMessage           = domain.ChatMessage
	ChatReport            = domain.ChatReport
	Appeal                = domain.Appeal
	AppealEvent           = domain.AppealEvent
	ChatRestriction       = domain.ChatRestriction
	ChatFilter            = domain.ChatFilter
	ChatAutoModAction     = domain.ChatAutoModAction
	Tip                   = domain.Tip
	Subscription          = domain.Subscription
	PaymentTransaction    = domain.PaymentTransaction
	CryptoAddress         = domain.CryptoAddress
	SocialLink            = domain.SocialLink
	Profile               = domain.Profile
)

func NewMoneyFromMinorUnits(units int64) Money { return domain.NewMoneyFromMinorUnits(units) }
func ParseMoney(value string) (Money, error)   { return domain.ParseMoney(value) }
func MustParseMoney(value string) Money        { return domain.MustParseMoney(value) }
