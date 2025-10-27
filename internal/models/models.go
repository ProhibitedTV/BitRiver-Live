package models

import "time"

type User struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"displayName"`
	Email       string    `json:"email"`
	Roles       []string  `json:"roles"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Channel struct {
	ID               string    `json:"id"`
	OwnerID          string    `json:"ownerId"`
	StreamKey        string    `json:"streamKey"`
	Title            string    `json:"title"`
	Category         string    `json:"category,omitempty"`
	Tags             []string  `json:"tags"`
	LiveState        string    `json:"liveState"`
	CurrentSessionID *string   `json:"currentSessionId,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type StreamSession struct {
	ID             string     `json:"id"`
	ChannelID      string     `json:"channelId"`
	StartedAt      time.Time  `json:"startedAt"`
	EndedAt        *time.Time `json:"endedAt,omitempty"`
	Renditions     []string   `json:"renditions"`
	PeakConcurrent int        `json:"peakConcurrent"`
}

type ChatMessage struct {
	ID        string    `json:"id"`
	ChannelID string    `json:"channelId"`
	UserID    string    `json:"userId"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

type CryptoAddress struct {
	Currency string `json:"currency"`
	Address  string `json:"address"`
	Note     string `json:"note,omitempty"`
}

type Profile struct {
	UserID            string          `json:"userId"`
	Bio               string          `json:"bio"`
	AvatarURL         string          `json:"avatarUrl"`
	BannerURL         string          `json:"bannerUrl"`
	FeaturedChannelID *string         `json:"featuredChannelId,omitempty"`
	TopFriends        []string        `json:"topFriends"`
	DonationAddresses []CryptoAddress `json:"donationAddresses"`
	CreatedAt         time.Time       `json:"createdAt"`
	UpdatedAt         time.Time       `json:"updatedAt"`
}
