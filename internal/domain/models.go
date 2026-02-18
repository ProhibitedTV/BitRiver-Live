package domain

import "bitriver-live/internal/models"

const (
	DMCACaseStatusOpen     = models.DMCACaseStatusOpen
	DMCACaseStatusActioned = models.DMCACaseStatusActioned
	DMCACaseStatusRestored = models.DMCACaseStatusRestored
	DMCACaseStatusRejected = models.DMCACaseStatusRejected

	DataSubjectRequestTypeExport = models.DataSubjectRequestTypeExport
	DataSubjectRequestTypeDelete = models.DataSubjectRequestTypeDelete

	DataSubjectRequestStatusOpen     = models.DataSubjectRequestStatusOpen
	DataSubjectRequestStatusActioned = models.DataSubjectRequestStatusActioned
	DataSubjectRequestStatusRejected = models.DataSubjectRequestStatusRejected

	PaymentStatePending   = models.PaymentStatePending
	PaymentStateConfirmed = models.PaymentStateConfirmed
	PaymentStateFailed    = models.PaymentStateFailed
	PaymentStateRefunded  = models.PaymentStateRefunded
)

type (
	Money                 = models.Money
	User                  = models.User
	MFASettings           = models.MFASettings
	OAuthAccount          = models.OAuthAccount
	Channel               = models.Channel
	StreamSession         = models.StreamSession
	RenditionManifest     = models.RenditionManifest
	Recording             = models.Recording
	RecordingRendition    = models.RecordingRendition
	RecordingThumbnail    = models.RecordingThumbnail
	Upload                = models.Upload
	DMCACase              = models.DMCACase
	LegalStateHistory     = models.LegalStateHistory
	DataSubjectRequest    = models.DataSubjectRequest
	DataSubjectAuditEvent = models.DataSubjectAuditEvent
	ClipExport            = models.ClipExport
	ClipExportSummary     = models.ClipExportSummary
	ChatMessage           = models.ChatMessage
	ChatReport            = models.ChatReport
	Appeal                = models.Appeal
	AppealEvent           = models.AppealEvent
	ChatRestriction       = models.ChatRestriction
	ChatFilter            = models.ChatFilter
	ChatAutoModAction     = models.ChatAutoModAction
	Tip                   = models.Tip
	Subscription          = models.Subscription
	PaymentTransaction    = models.PaymentTransaction
	CryptoAddress         = models.CryptoAddress
	SocialLink            = models.SocialLink
	Profile               = models.Profile
)

func NewMoneyFromMinorUnits(units int64) Money { return models.NewMoneyFromMinorUnits(units) }
func ParseMoney(value string) (Money, error)   { return models.ParseMoney(value) }
func MustParseMoney(value string) Money        { return models.MustParseMoney(value) }
