# internal/models → internal/domain migration map

This transitional map defines the one-to-one move target for every exported
`internal/models` type and constant. During migration, `internal/domain` re-exports
those symbols so imports can switch incrementally without behaviour changes.

## Value objects

| `internal/models` | `internal/domain` |
| --- | --- |
| `Money` | `Money` |
| `NewMoneyFromMinorUnits` | `NewMoneyFromMinorUnits` |
| `ParseMoney` | `ParseMoney` |
| `MustParseMoney` | `MustParseMoney` |

## Identity, auth, and channel entities

| `internal/models` | `internal/domain` |
| --- | --- |
| `User` | `User` |
| `MFASettings` | `MFASettings` |
| `OAuthAccount` | `OAuthAccount` |
| `Channel` | `Channel` |
| `StreamSession` | `StreamSession` |
| `RenditionManifest` | `RenditionManifest` |

## Content and media entities

| `internal/models` | `internal/domain` |
| --- | --- |
| `Recording` | `Recording` |
| `RecordingRendition` | `RecordingRendition` |
| `RecordingThumbnail` | `RecordingThumbnail` |
| `Upload` | `Upload` |
| `ClipExport` | `ClipExport` |
| `ClipExportSummary` | `ClipExportSummary` |

## Compliance and legal entities

| `internal/models` | `internal/domain` |
| --- | --- |
| `DMCACase` | `DMCACase` |
| `LegalStateHistory` | `LegalStateHistory` |
| `DataSubjectRequest` | `DataSubjectRequest` |
| `DataSubjectAuditEvent` | `DataSubjectAuditEvent` |
| `DMCACaseStatus*` constants | same names |
| `DataSubjectRequestType*` constants | same names |
| `DataSubjectRequestStatus*` constants | same names |

## Chat and moderation entities

| `internal/models` | `internal/domain` |
| --- | --- |
| `ChatMessage` | `ChatMessage` |
| `ChatReport` | `ChatReport` |
| `Appeal` | `Appeal` |
| `AppealEvent` | `AppealEvent` |
| `ChatRestriction` | `ChatRestriction` |
| `ChatFilter` | `ChatFilter` |
| `ChatAutoModAction` | `ChatAutoModAction` |

## Monetization and profile entities

| `internal/models` | `internal/domain` |
| --- | --- |
| `Tip` | `Tip` |
| `Subscription` | `Subscription` |
| `PaymentTransaction` | `PaymentTransaction` |
| `PaymentState*` constants | same names |
| `CryptoAddress` | `CryptoAddress` |
| `SocialLink` | `SocialLink` |
| `Profile` | `Profile` |
