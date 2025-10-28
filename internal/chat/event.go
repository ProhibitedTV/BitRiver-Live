package chat

import "time"

// EventType enumerates the supported chat events flowing through the gateway and
// persistence queue.
type EventType string

const (
	// EventTypeMessage represents a chat message authored by a viewer or
	// moderator.
	EventTypeMessage EventType = "message"
	// EventTypeModeration represents a moderation action such as a timeout
	// or ban.
	EventTypeModeration EventType = "moderation"
)

// ModerationAction captures the different moderation operations available to
// channel moderators.
type ModerationAction string

const (
	// ModerationActionTimeout temporarily mutes a user in the room.
	ModerationActionTimeout ModerationAction = "timeout"
	// ModerationActionRemoveTimeout clears a previously scheduled timeout
	// for a user.
	ModerationActionRemoveTimeout ModerationAction = "remove_timeout"
	// ModerationActionBan blocks a user from joining the chat entirely.
	ModerationActionBan ModerationAction = "ban"
	// ModerationActionUnban removes a previously issued ban.
	ModerationActionUnban ModerationAction = "unban"
)

// Event is the wire representation forwarded to the persistence queue.
type Event struct {
	Type       EventType        `json:"type"`
	Message    *MessageEvent    `json:"message,omitempty"`
	Moderation *ModerationEvent `json:"moderation,omitempty"`
	OccurredAt time.Time        `json:"occurredAt"`
}

// MessageEvent transports all information required to persist a chat message.
type MessageEvent struct {
	ID        string    `json:"id"`
	ChannelID string    `json:"channelId"`
	UserID    string    `json:"userId"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

// ModerationEvent describes a moderation action taken by a moderator or
// channel owner.
type ModerationEvent struct {
	Action    ModerationAction `json:"action"`
	ChannelID string           `json:"channelId"`
	ActorID   string           `json:"actorId"`
	TargetID  string           `json:"targetId"`
	ExpiresAt *time.Time       `json:"expiresAt,omitempty"`
	Reason    string           `json:"reason,omitempty"`
}

// RestrictionsSnapshot represents the currently active moderation state for
// each channel. It is primarily used to bootstrap the in-memory gateway view at
// startup.
type RestrictionsSnapshot struct {
	Bans     map[string]map[string]struct{}
	Timeouts map[string]map[string]time.Time
}

// Copy returns a deep copy of the snapshot.
func (r RestrictionsSnapshot) Copy() RestrictionsSnapshot {
	out := RestrictionsSnapshot{
		Bans:     make(map[string]map[string]struct{}, len(r.Bans)),
		Timeouts: make(map[string]map[string]time.Time, len(r.Timeouts)),
	}
	for channel, bans := range r.Bans {
		clone := make(map[string]struct{}, len(bans))
		for user := range bans {
			clone[user] = struct{}{}
		}
		out.Bans[channel] = clone
	}
	for channel, timeouts := range r.Timeouts {
		clone := make(map[string]time.Time, len(timeouts))
		for user, expiry := range timeouts {
			clone[user] = expiry
		}
		out.Timeouts[channel] = clone
	}
	return out
}
