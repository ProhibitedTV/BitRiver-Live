package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"bitriver-live/internal/domain"
	"bitriver-live/internal/observability/metrics"
)

type createChannelRequest struct {
	OwnerID  string                   `json:"ownerId"`
	Title    string                   `json:"title"`
	Category string                   `json:"category"`
	Tags     []string                 `json:"tags"`
	Schedule []channelScheduleRequest `json:"schedule,omitempty"`
}

type updateChannelRequest struct {
	Title    *string                   `json:"title"`
	Category *string                   `json:"category"`
	Tags     *[]string                 `json:"tags"`
	Schedule *[]channelScheduleRequest `json:"schedule"`
}

type channelScheduleRequest struct {
	ID              string `json:"id,omitempty"`
	Title           string `json:"title"`
	StartsAt        string `json:"startsAt"`
	DurationMinutes int    `json:"durationMinutes,omitempty"`
	Description     string `json:"description,omitempty"`
}

type channelPublicResponse struct {
	ID               string                    `json:"id"`
	OwnerID          string                    `json:"ownerId"`
	Title            string                    `json:"title"`
	Category         string                    `json:"category,omitempty"`
	Tags             []string                  `json:"tags"`
	Schedule         []channelScheduleResponse `json:"schedule,omitempty"`
	LiveState        string                    `json:"liveState"`
	CurrentSessionID *string                   `json:"currentSessionId,omitempty"`
	CreatedAt        string                    `json:"createdAt"`
	UpdatedAt        string                    `json:"updatedAt"`
}

type channelScheduleResponse struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	StartsAt        string `json:"startsAt"`
	DurationMinutes int    `json:"durationMinutes,omitempty"`
	Description     string `json:"description,omitempty"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

type channelResponse struct {
	channelPublicResponse
	StreamKey string `json:"streamKey"`
}

type channelOwnerResponse struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
}

type profileSummaryResponse struct {
	Bio         string               `json:"bio,omitempty"`
	AvatarURL   string               `json:"avatarUrl,omitempty"`
	BannerURL   string               `json:"bannerUrl,omitempty"`
	SocialLinks []socialLinkResponse `json:"socialLinks,omitempty"`
}

type directoryChannelResponse struct {
	Channel       channelPublicResponse  `json:"channel"`
	Owner         channelOwnerResponse   `json:"owner"`
	Profile       profileSummaryResponse `json:"profile"`
	Live          bool                   `json:"live"`
	FollowerCount int                    `json:"followerCount"`
}

type directoryResponse struct {
	Channels    []directoryChannelResponse `json:"channels"`
	GeneratedAt string                     `json:"generatedAt"`
}

type categorySummaryResponse struct {
	Name         string `json:"name"`
	ChannelCount int    `json:"channelCount"`
}

type categoryDirectoryResponse struct {
	Categories  []categorySummaryResponse `json:"categories"`
	GeneratedAt string                    `json:"generatedAt"`
}

type followStateResponse struct {
	Followers int  `json:"followers"`
	Following bool `json:"following"`
}

type subscriptionStateResponse struct {
	Subscribers int     `json:"subscribers"`
	Subscribed  bool    `json:"subscribed"`
	Tier        string  `json:"tier,omitempty"`
	RenewsAt    *string `json:"renewsAt,omitempty"`
}

type playbackStreamResponse struct {
	SessionID   string                      `json:"sessionId"`
	StartedAt   string                      `json:"startedAt"`
	PlaybackURL string                      `json:"playbackUrl,omitempty"`
	OriginURL   string                      `json:"originUrl,omitempty"`
	Protocol    string                      `json:"protocol,omitempty"`
	PlayerHint  string                      `json:"playerHint,omitempty"`
	LatencyMode string                      `json:"latencyMode,omitempty"`
	Renditions  []renditionManifestResponse `json:"renditions,omitempty"`
}

type channelPlaybackResponse struct {
	Channel           channelPublicResponse      `json:"channel"`
	Owner             channelOwnerResponse       `json:"owner"`
	Profile           profileSummaryResponse     `json:"profile"`
	DonationAddresses []cryptoAddressResponse    `json:"donationAddresses"`
	Live              bool                       `json:"live"`
	Follow            followStateResponse        `json:"follow"`
	Subscription      *subscriptionStateResponse `json:"subscription,omitempty"`
	Playback          *playbackStreamResponse    `json:"playback,omitempty"`
}

type vodItemResponse struct {
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	DurationSeconds int     `json:"durationSeconds"`
	PublishedAt     *string `json:"publishedAt,omitempty"`
	ThumbnailURL    string  `json:"thumbnailUrl,omitempty"`
	PlaybackURL     string  `json:"playbackUrl,omitempty"`
}

type vodCollectionResponse struct {
	ChannelID string            `json:"channelId"`
	Items     []vodItemResponse `json:"items"`
}

// Directory performs directory and returns an error when dependent systems reject the operation.
func (h *Handler) Directory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteMethodNotAllowed(w, r, http.MethodGet)
		return
	}

	query := ""
	if r.URL != nil {
		query = strings.TrimSpace(r.URL.Query().Get("q"))
	}
	channels := h.channelsService().ListChannels("", query)
	followerCounts := h.followerCountsForChannels(channels)
	h.writeDirectoryResponse(w, channels, followerCounts)
}

// DirectoryFeatured performs directory featured and returns an error when dependent systems reject the operation.
func (h *Handler) DirectoryFeatured(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteMethodNotAllowed(w, r, http.MethodGet)
		return
	}

	profiles := h.channelsService().ListProfiles()
	channelIDs := make(map[string]struct{}, len(profiles))
	for _, profile := range profiles {
		if profile.FeaturedChannelID == nil {
			continue
		}
		id := strings.TrimSpace(*profile.FeaturedChannelID)
		if id == "" {
			continue
		}
		channelIDs[id] = struct{}{}
	}

	channels := make([]domain.Channel, 0, len(channelIDs))
	for id := range channelIDs {
		if channel, ok := h.channelsService().GetChannel(id); ok {
			channels = append(channels, channel)
		}
	}

	followerCounts := h.followerCountsForChannels(channels)
	h.writeDirectoryResponse(w, h.sortChannelsByFollowers(channels, followerCounts, true), followerCounts)
}

// DirectoryRecommended performs directory recommended and returns an error when dependent systems reject the operation.
func (h *Handler) DirectoryRecommended(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteMethodNotAllowed(w, r, http.MethodGet)
		return
	}

	channels := h.channelsService().ListChannels("", "")
	followerCounts := h.followerCountsForChannels(channels)
	h.writeDirectoryResponse(w, h.sortChannelsByFollowers(channels, followerCounts, false), followerCounts)
}

// DirectoryLive performs directory live and returns an error when dependent systems reject the operation.
func (h *Handler) DirectoryLive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteMethodNotAllowed(w, r, http.MethodGet)
		return
	}

	channels := h.channelsService().ListChannels("", "")
	channels = filterLiveChannels(channels)
	followerCounts := h.followerCountsForChannels(channels)
	h.writeDirectoryResponse(w, h.sortChannelsByFollowers(channels, followerCounts, true), followerCounts)
}

// DirectoryTrending performs directory trending and returns an error when dependent systems reject the operation.
func (h *Handler) DirectoryTrending(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteMethodNotAllowed(w, r, http.MethodGet)
		return
	}

	channels := filterLiveChannels(h.channelsService().ListChannels("", ""))
	followerCounts := h.followerCountsForChannels(channels)
	h.writeDirectoryResponse(w, h.sortChannelsByFollowers(channels, followerCounts, true), followerCounts)
}

// DirectoryCategories performs directory categories and returns an error when dependent systems reject the operation.
func (h *Handler) DirectoryCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteMethodNotAllowed(w, r, http.MethodGet)
		return
	}

	channels := filterLiveChannels(h.channelsService().ListChannels("", ""))
	counts := make(map[string]int)
	for _, channel := range channels {
		category := strings.TrimSpace(channel.Category)
		if category == "" {
			continue
		}
		counts[category]++
	}

	summaries := make([]categorySummaryResponse, 0, len(counts))
	for name, count := range counts {
		summaries = append(summaries, categorySummaryResponse{Name: name, ChannelCount: count})
	}
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].ChannelCount == summaries[j].ChannelCount {
			return summaries[i].Name < summaries[j].Name
		}
		return summaries[i].ChannelCount > summaries[j].ChannelCount
	})

	payload := categoryDirectoryResponse{Categories: summaries, GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	WriteJSON(w, http.StatusOK, payload)
}

// filterLiveChannels performs filter live channels and propagates validation or dependency failures to the caller.
func filterLiveChannels(channels []domain.Channel) []domain.Channel {
	live := make([]domain.Channel, 0, len(channels))
	for _, channel := range channels {
		if channel.LiveState == "live" || channel.LiveState == "starting" {
			live = append(live, channel)
		}
	}
	return live
}

// sortChannelsByFollowers performs sort channels by followers and propagates validation or dependency failures to the caller.
func (h *Handler) sortChannelsByFollowers(channels []domain.Channel, followerCounts map[string]int, liveFirst bool) []domain.Channel {
	sort.Slice(channels, func(i, j int) bool {
		if liveFirst {
			iLive := channels[i].LiveState == "live" || channels[i].LiveState == "starting"
			jLive := channels[j].LiveState == "live" || channels[j].LiveState == "starting"
			if iLive != jLive {
				return iLive
			}
		}
		if followerCounts[channels[i].ID] == followerCounts[channels[j].ID] {
			return channels[i].CreatedAt.Before(channels[j].CreatedAt)
		}
		return followerCounts[channels[i].ID] > followerCounts[channels[j].ID]
	})
	return channels
}

func (h *Handler) followerCountsForChannels(channels []domain.Channel) map[string]int {
	followerCounts := make(map[string]int, len(channels))
	for _, channel := range channels {
		followerCounts[channel.ID] = h.channelsService().CountFollowers(channel.ID)
	}
	return followerCounts
}

// DirectoryFollowing performs directory following and returns an error when dependent systems reject the operation.
func (h *Handler) DirectoryFollowing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteMethodNotAllowed(w, r, http.MethodGet)
		return
	}

	viewer, ok := h.requireAuthenticatedUser(w, r)
	if !ok {
		return
	}

	channelIDs := h.channelsService().ListFollowedChannelIDs(viewer.ID)
	channels := make([]domain.Channel, 0, len(channelIDs))
	for _, id := range channelIDs {
		channel, exists := h.channelsService().GetChannel(id)
		if !exists {
			continue
		}
		if channel.LiveState != "live" && channel.LiveState != "starting" {
			continue
		}
		channels = append(channels, channel)
	}

	followerCounts := h.followerCountsForChannels(channels)
	h.writeDirectoryResponse(w, channels, followerCounts)
}

// writeDirectoryResponse writes directory response to the active response or stream and surfaces encode or I/O failures.
func (h *Handler) writeDirectoryResponse(w http.ResponseWriter, channels []domain.Channel, followerCounts map[string]int) {
	users := h.channelsService().ListUsers()
	usersByID := make(map[string]domain.User, len(users))
	for _, user := range users {
		usersByID[user.ID] = user
	}

	profiles := h.channelsService().ListProfiles()
	profilesByUserID := make(map[string]domain.Profile, len(profiles))
	for _, profile := range profiles {
		profilesByUserID[profile.UserID] = profile
	}

	response := make([]directoryChannelResponse, 0, len(channels))
	for _, channel := range channels {
		owner, exists := usersByID[channel.OwnerID]
		if !exists {
			continue
		}
		profile := profilesByUserID[owner.ID]
		followerCount, ok := followerCounts[channel.ID]
		if !ok {
			followerCount = h.channelsService().CountFollowers(channel.ID)
		}
		response = append(response, directoryChannelResponse{
			Channel:       newChannelPublicResponse(channel),
			Owner:         newOwnerResponse(owner, profile),
			Profile:       newProfileSummaryResponse(profile),
			Live:          channel.LiveState == "live" || channel.LiveState == "starting",
			FollowerCount: followerCount,
		})
	}

	payload := directoryResponse{
		Channels:    response,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	WriteJSON(w, http.StatusOK, payload)
}

// buildChannelResponse builds channel response from runtime state used by downstream handlers.
func buildChannelResponse(channel domain.Channel, includeStreamKey bool) channelResponse {
	resp := channelResponse{
		channelPublicResponse: channelPublicResponse{
			ID:        channel.ID,
			OwnerID:   channel.OwnerID,
			Title:     channel.Title,
			Category:  channel.Category,
			Tags:      append([]string{}, channel.Tags...),
			Schedule:  newChannelScheduleResponse(channel.Schedule),
			LiveState: channel.LiveState,
			CreatedAt: channel.CreatedAt.Format(time.RFC3339Nano),
			UpdatedAt: channel.UpdatedAt.Format(time.RFC3339Nano),
		},
	}
	if channel.CurrentSessionID != nil {
		sessionID := *channel.CurrentSessionID
		resp.CurrentSessionID = &sessionID
	}
	if includeStreamKey {
		resp.StreamKey = channel.StreamKey
	}
	return resp
}

// newChannelResponse builds and returns channel response using the supplied dependencies.
func newChannelResponse(channel domain.Channel) channelResponse {
	return buildChannelResponse(channel, true)
}

// newChannelPublicResponse builds and returns channel public response using the supplied dependencies.
func newChannelPublicResponse(channel domain.Channel) channelPublicResponse {
	return buildChannelResponse(channel, false).channelPublicResponse
}

// newChannelScheduleResponse builds a public schedule response preserving storage ordering.
func newChannelScheduleResponse(entries []domain.ChannelScheduleEntry) []channelScheduleResponse {
	if len(entries) == 0 {
		return nil
	}
	response := make([]channelScheduleResponse, 0, len(entries))
	for _, entry := range entries {
		response = append(response, channelScheduleResponse{
			ID:              entry.ID,
			Title:           entry.Title,
			StartsAt:        entry.StartsAt.Format(time.RFC3339Nano),
			DurationMinutes: entry.DurationMinutes,
			Description:     entry.Description,
			CreatedAt:       entry.CreatedAt.Format(time.RFC3339Nano),
			UpdatedAt:       entry.UpdatedAt.Format(time.RFC3339Nano),
		})
	}
	return response
}

// parseChannelScheduleRequest validates wire timestamps before storage-level normalization.
func parseChannelScheduleRequest(requests []channelScheduleRequest) ([]domain.ChannelScheduleEntry, error) {
	entries := make([]domain.ChannelScheduleEntry, 0, len(requests))
	for _, req := range requests {
		var startsAt time.Time
		if rawStartsAt := strings.TrimSpace(req.StartsAt); rawStartsAt != "" {
			parsed, err := time.Parse(time.RFC3339Nano, rawStartsAt)
			if err != nil {
				return nil, fmt.Errorf("invalid schedule startsAt %q", rawStartsAt)
			}
			startsAt = parsed
		}
		entries = append(entries, domain.ChannelScheduleEntry{
			ID:              strings.TrimSpace(req.ID),
			Title:           req.Title,
			StartsAt:        startsAt,
			DurationMinutes: req.DurationMinutes,
			Description:     req.Description,
		})
	}
	return entries, nil
}

// newOwnerResponse builds and returns owner response using the supplied dependencies.
func newOwnerResponse(user domain.User, profile domain.Profile) channelOwnerResponse {
	owner := channelOwnerResponse{ID: user.ID, DisplayName: user.DisplayName}
	if profile.AvatarURL != "" {
		owner.AvatarURL = profile.AvatarURL
	}
	return owner
}

// newProfileSummaryResponse builds and returns profile summary response using the supplied dependencies.
func newProfileSummaryResponse(profile domain.Profile) profileSummaryResponse {
	summary := profileSummaryResponse{}
	if profile.Bio != "" {
		summary.Bio = profile.Bio
	}
	if profile.AvatarURL != "" {
		summary.AvatarURL = profile.AvatarURL
	}
	if profile.BannerURL != "" {
		summary.BannerURL = profile.BannerURL
	}
	if len(profile.SocialLinks) > 0 {
		links := make([]socialLinkResponse, 0, len(profile.SocialLinks))
		for _, link := range profile.SocialLinks {
			links = append(links, socialLinkResponse{Platform: link.Platform, URL: link.URL})
		}
		summary.SocialLinks = links
	}
	return summary
}

// subscriptionState performs subscription state and propagates validation or dependency failures to the caller.
func (h *Handler) subscriptionState(channelID string, actor *domain.User) (subscriptionStateResponse, error) {
	subs, err := h.channelsService().ListSubscriptions(channelID, false)
	if err != nil {
		return subscriptionStateResponse{}, err
	}
	state := subscriptionStateResponse{Subscribers: len(subs)}
	if actor == nil {
		return state, nil
	}
	for _, sub := range subs {
		if sub.UserID != actor.ID {
			continue
		}
		state.Subscribed = true
		state.Tier = sub.Tier
		if sub.ExpiresAt.After(time.Now()) {
			renews := sub.ExpiresAt.Format(time.RFC3339Nano)
			state.RenewsAt = &renews
		}
		break
	}
	return state, nil
}

// Channels performs channels and returns an error when dependent systems reject the operation.
func (h *Handler) Channels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		actor, ok := h.requireAuthenticatedUser(w, r)
		if !ok {
			return
		}
		ownerID := strings.TrimSpace(r.URL.Query().Get("ownerId"))
		if ownerID == "" {
			if !actor.HasRole(roleAdmin) {
				ownerID = actor.ID
			}
		} else if ownerID != actor.ID && !actor.HasRole(roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}

		channels := h.channelsService().ListChannels(ownerID, "")
		if ownerID == actor.ID || actor.HasRole(roleAdmin) {
			response := make([]channelResponse, 0, len(channels))
			for _, channel := range channels {
				response = append(response, newChannelResponse(channel))
			}
			WriteJSON(w, http.StatusOK, response)
			return
		}

		response := make([]channelPublicResponse, 0, len(channels))
		for _, channel := range channels {
			response = append(response, newChannelPublicResponse(channel))
		}
		WriteJSON(w, http.StatusOK, response)
	case http.MethodPost:
		actor, ok := h.requireAuthenticatedUser(w, r)
		if !ok {
			return
		}
		if persistedActor, exists := h.channelsService().GetUser(actor.ID); exists {
			actor = persistedActor
		}
		var req createChannelRequest
		if !DecodeAndValidate(w, r, &req) {
			return
		}
		if req.OwnerID == "" {
			req.OwnerID = actor.ID
		}
		if req.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}
		ownedChannels := h.channelsService().ListChannels(req.OwnerID, "")
		ownerNeedsCreatorBootstrap := req.OwnerID == actor.ID && !actor.HasRole(roleAdmin) && !actor.HasRole(roleCreator)
		if ownerNeedsCreatorBootstrap && len(ownedChannels) > 0 {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}
		var scheduleUpdate *[]domain.ChannelScheduleEntry
		if req.Schedule != nil {
			schedule, err := parseChannelScheduleRequest(req.Schedule)
			if err != nil {
				WriteError(w, http.StatusBadRequest, err)
				return
			}
			scheduleUpdate = &schedule
		}
		channel, err := h.channelsService().CreateChannel(req.OwnerID, req.Title, req.Category, req.Tags)
		if err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		if scheduleUpdate != nil {
			updated, err := h.channelsService().UpdateChannel(channel.ID, domain.ChannelUpdate{Schedule: scheduleUpdate})
			if err != nil {
				if rollbackErr := h.channelsService().DeleteChannel(channel.ID); rollbackErr != nil {
					WriteError(w, http.StatusBadRequest, fmt.Errorf("set channel schedule after creating channel %s: %w (rollback failed: %v)", channel.ID, err, rollbackErr))
					return
				}
				WriteError(w, http.StatusBadRequest, fmt.Errorf("set channel schedule after creating channel %s: %w", channel.ID, err))
				return
			}
			channel = updated
		}
		if owner, exists := h.channelsService().GetUser(req.OwnerID); exists && len(ownedChannels) == 0 && !owner.HasRole(roleCreator) {
			roles := append([]string{}, owner.Roles...)
			roles = append(roles, roleCreator)
			if _, err := h.channelsService().UpdateUser(owner.ID, domain.UserUpdate{Roles: &roles}); err != nil {
				if rollbackErr := h.channelsService().DeleteChannel(channel.ID); rollbackErr != nil {
					WriteError(w, http.StatusInternalServerError, fmt.Errorf("upgrade owner %s to creator after creating channel %s: %w (rollback failed: %v)", owner.ID, channel.ID, err, rollbackErr))
					return
				}
				WriteError(w, http.StatusInternalServerError, fmt.Errorf("upgrade owner %s to creator after creating channel %s: %w", owner.ID, channel.ID, err))
				return
			}
		}
		WriteJSON(w, http.StatusCreated, newChannelResponse(channel))
	default:
		WriteMethodNotAllowed(w, r, http.MethodGet, http.MethodPost)
	}
}

// ChannelByID performs channel by id and returns an error when dependent systems reject the operation.
func (h *Handler) ChannelByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/channels/")
	parts := strings.Split(path, "/")
	for len(parts) > 1 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	if len(parts) == 0 || parts[0] == "" {
		WriteError(w, http.StatusNotFound, fmt.Errorf("channel id missing"))
		return
	}
	channelID := parts[0]

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			channel, ok := h.channelsService().GetChannel(channelID)
			if !ok {
				WriteError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			if actor, ok := UserFromContext(r.Context()); ok && (channel.OwnerID == actor.ID || actor.HasRole(roleAdmin)) {
				WriteJSON(w, http.StatusOK, newChannelResponse(channel))
				return
			}
			WriteJSON(w, http.StatusOK, newChannelPublicResponse(channel))
		case http.MethodPatch:
			channel, ok := h.channelsService().GetChannel(channelID)
			if !ok {
				WriteError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			if _, ok := h.ensureChannelAccess(w, r, channel); !ok {
				return
			}
			var req updateChannelRequest
			if !DecodeAndValidate(w, r, &req) {
				return
			}
			update := domain.ChannelUpdate{}
			if req.Title != nil {
				update.Title = req.Title
			}
			if req.Category != nil {
				update.Category = req.Category
			}
			if req.Tags != nil {
				tagsCopy := append([]string{}, (*req.Tags)...)
				update.Tags = &tagsCopy
			}
			if req.Schedule != nil {
				schedule, err := parseChannelScheduleRequest(*req.Schedule)
				if err != nil {
					WriteError(w, http.StatusBadRequest, err)
					return
				}
				update.Schedule = &schedule
			}
			channel, err := h.channelsService().UpdateChannel(channelID, update)
			if err != nil {
				WriteError(w, http.StatusBadRequest, err)
				return
			}
			WriteJSON(w, http.StatusOK, newChannelResponse(channel))
		case http.MethodDelete:
			channel, ok := h.channelsService().GetChannel(channelID)
			if !ok {
				WriteError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			if _, ok := h.ensureChannelAccess(w, r, channel); !ok {
				return
			}
			if err := h.channelsService().DeleteChannel(channelID); err != nil {
				WriteError(w, http.StatusBadRequest, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			WriteMethodNotAllowed(w, r, http.MethodGet, http.MethodPatch, http.MethodDelete)
		}
		return
	}

	if len(parts) >= 2 {
		switch parts[1] {
		case "playback":
			// Public playback metadata is intentionally readable without channel ownership checks so viewers can bootstrap players from a single response.
			channel, ok := h.channelsService().GetChannel(channelID)
			if !ok {
				WriteError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			if r.Method != http.MethodGet {
				WriteMethodNotAllowed(w, r, http.MethodGet)
				return
			}
			owner, exists := h.channelsService().GetUser(channel.OwnerID)
			if !exists {
				WriteError(w, http.StatusInternalServerError, fmt.Errorf("channel owner %s not found", channel.OwnerID))
				return
			}
			profile, _ := h.channelsService().GetProfile(owner.ID)
			follow := followStateResponse{Followers: h.channelsService().CountFollowers(channel.ID)}
			var viewer *domain.User
			if actor, ok := UserFromContext(r.Context()); ok {
				follow.Following = h.channelsService().IsFollowingChannel(actor.ID, channel.ID)
				viewer = &actor
			}
			donations := make([]cryptoAddressResponse, 0, len(profile.DonationAddresses))
			for _, addr := range profile.DonationAddresses {
				donations = append(donations, cryptoAddressResponse{
					Currency: addr.Currency,
					Address:  addr.Address,
					Note:     addr.Note,
				})
			}

			response := channelPlaybackResponse{
				Channel:           newChannelPublicResponse(channel),
				Owner:             newOwnerResponse(owner, profile),
				Profile:           newProfileSummaryResponse(profile),
				DonationAddresses: donations,
				Live:              channel.LiveState == "live" || channel.LiveState == "starting",
				Follow:            follow,
			}
			if state, err := h.subscriptionState(channel.ID, viewer); err == nil {
				response.Subscription = &state
			} else {
				WriteError(w, http.StatusInternalServerError, err)
				return
			}
			if session, live := h.channelsService().CurrentStreamSession(channel.ID); live {
				playback := playbackStreamResponse{
					SessionID: session.ID,
					StartedAt: session.StartedAt.Format(time.RFC3339Nano),
				}
				if session.PlaybackURL != "" {
					playback.PlaybackURL = session.PlaybackURL
				}
				if session.OriginURL != "" {
					playback.OriginURL = session.OriginURL
				}
				if len(session.RenditionManifests) > 0 {
					manifests := make([]renditionManifestResponse, 0, len(session.RenditionManifests))
					for _, manifest := range session.RenditionManifests {
						manifests = append(manifests, renditionManifestResponse{
							Name:        manifest.Name,
							ManifestURL: manifest.ManifestURL,
							Bitrate:     manifest.Bitrate,
						})
					}
					playback.Renditions = manifests
				}
				protocol := "ll-hls"
				player := "hls.js"
				latency := "low-latency"
				// Prefix-based transport hints keep legacy ingest URLs working while signaling clients to switch between broader-compat LL-HLS and ultra-low-latency WebRTC paths.
				url := strings.ToLower(playback.PlaybackURL)
				if strings.HasPrefix(url, "webrtc") || strings.HasPrefix(url, "wss") {
					protocol = "webrtc"
					player = "ovenplayer"
					latency = "ultra-low"
				}
				playback.Protocol = protocol
				playback.PlayerHint = player
				playback.LatencyMode = latency
				response.Playback = &playback
			}
			WriteJSON(w, http.StatusOK, response)
			return
		case "stream":
			channel, ok := h.channelsService().GetChannel(channelID)
			if !ok {
				WriteError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			h.handleStreamRoutes(channel, parts[2:], w, r)
			return
		case "sessions":
			// Session history is owner-only telemetry; callers must be authorized and receive the full stream timeline for control-plane tooling.
			channel, ok := h.channelsService().GetChannel(channelID)
			if !ok {
				WriteError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			if _, ok := h.ensureChannelAccess(w, r, channel); !ok {
				return
			}
			if r.Method != http.MethodGet {
				WriteMethodNotAllowed(w, r, http.MethodGet)
				return
			}
			sessions, err := h.channelsService().ListStreamSessions(channelID)
			if err != nil {
				WriteError(w, http.StatusBadRequest, err)
				return
			}
			response := make([]sessionResponse, 0, len(sessions))
			for _, session := range sessions {
				response = append(response, newSessionResponse(session))
			}
			WriteJSON(w, http.StatusOK, response)
			return
		case "follow":
			// Follow mutations are per-user state transitions that require auth and always return the post-operation aggregate + viewer-specific follow snapshot.
			if len(parts) > 2 {
				WriteError(w, http.StatusNotFound, fmt.Errorf("unknown channel path"))
				return
			}
			if _, ok := h.channelsService().GetChannel(channelID); !ok {
				WriteError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			actor, ok := h.requireAuthenticatedUser(w, r)
			if !ok {
				return
			}
			switch r.Method {
			case http.MethodPost:
				if err := h.channelsService().FollowChannel(actor.ID, channelID); err != nil {
					WriteError(w, http.StatusBadRequest, err)
					return
				}
			case http.MethodDelete:
				if err := h.channelsService().UnfollowChannel(actor.ID, channelID); err != nil {
					WriteError(w, http.StatusBadRequest, err)
					return
				}
			default:
				WriteMethodNotAllowed(w, r, http.MethodPost, http.MethodDelete)
				return
			}
			state := followStateResponse{
				Followers: h.channelsService().CountFollowers(channelID),
				Following: h.channelsService().IsFollowingChannel(actor.ID, channelID),
			}
			WriteJSON(w, http.StatusOK, state)
			return
		case "subscribe":
			// Subscription state is queryable publicly, but create/cancel remains authenticated and always responds with current entitlement state for UI reconciliation.
			if len(parts) > 2 {
				WriteError(w, http.StatusNotFound, fmt.Errorf("unknown channel path"))
				return
			}
			channel, ok := h.channelsService().GetChannel(channelID)
			if !ok {
				WriteError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			switch r.Method {
			case http.MethodGet:
				var viewer *domain.User
				if actor, ok := UserFromContext(r.Context()); ok {
					viewer = &actor
				}
				state, err := h.subscriptionState(channel.ID, viewer)
				if err != nil {
					WriteError(w, http.StatusBadRequest, err)
					return
				}
				WriteJSON(w, http.StatusOK, state)
			case http.MethodPost:
				actor, ok := h.requireAuthenticatedUser(w, r)
				if !ok {
					return
				}
				subs, err := h.channelsService().ListSubscriptions(channel.ID, false)
				if err != nil {
					WriteError(w, http.StatusBadRequest, err)
					return
				}
				alreadySubscribed := false
				for _, sub := range subs {
					if sub.UserID == actor.ID {
						alreadySubscribed = true
						break
					}
				}
				// Treat POST as idempotent: we reuse an active subscription when present so retried client requests do not duplicate billing records or metrics.
				if !alreadySubscribed {
					params := domain.SubscriptionCreateParams{
						ChannelID: channel.ID,
						UserID:    actor.ID,
						Tier:      "supporter",
						Provider:  "internal",
						Amount:    domain.NewMoneyFromMinorUnits(0),
						Currency:  "USD",
						Duration:  30 * 24 * time.Hour,
						AutoRenew: true,
					}
					sub, err := h.channelsService().CreateSubscription(params)
					if err != nil {
						WriteError(w, http.StatusBadRequest, err)
						return
					}
					metrics.Default().ObserveMonetization("subscription", sub.Amount)
				}
				state, err := h.subscriptionState(channel.ID, &actor)
				if err != nil {
					WriteError(w, http.StatusBadRequest, err)
					return
				}
				WriteJSON(w, http.StatusOK, state)
			case http.MethodDelete:
				actor, ok := h.requireAuthenticatedUser(w, r)
				if !ok {
					return
				}
				subs, err := h.channelsService().ListSubscriptions(channel.ID, false)
				if err != nil {
					WriteError(w, http.StatusBadRequest, err)
					return
				}
				subscriptionID := ""
				for _, sub := range subs {
					if sub.UserID == actor.ID {
						subscriptionID = sub.ID
						break
					}
				}
				if subscriptionID != "" {
					if _, err := h.channelsService().CancelSubscription(subscriptionID, actor.ID, ""); err != nil {
						WriteError(w, http.StatusBadRequest, err)
						return
					}
				}
				state, err := h.subscriptionState(channel.ID, &actor)
				if err != nil {
					WriteError(w, http.StatusBadRequest, err)
					return
				}
				WriteJSON(w, http.StatusOK, state)
			default:
				WriteMethodNotAllowed(w, r, http.MethodGet, http.MethodPost, http.MethodDelete)
			}
			return
		case "vods":
			if len(parts) > 2 {
				WriteError(w, http.StatusNotFound, fmt.Errorf("unknown channel path"))
				return
			}
			if r.Method != http.MethodGet {
				WriteMethodNotAllowed(w, r, http.MethodGet)
				return
			}
			channel, ok := h.channelsService().GetChannel(channelID)
			if !ok {
				WriteError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			uploads, err := h.channelsService().ListUploads(channelID)
			if err != nil {
				WriteError(w, http.StatusBadRequest, err)
				return
			}
			items := make([]vodItemResponse, 0, len(uploads))
			for _, upload := range uploads {
				if upload.RecordingID == nil {
					continue
				}
				recording, ok := h.channelsService().GetRecording(*upload.RecordingID)
				if !ok {
					continue
				}
				if recording.PublishedAt == nil {
					continue
				}
				item := newVodItemResponse(recording)
				if item.PublishedAt == nil {
					continue
				}
				items = append(items, item)
			}
			payload := vodCollectionResponse{ChannelID: channel.ID, Items: items}
			WriteJSON(w, http.StatusOK, payload)
			return
		case "chat":
			h.handleChatRoutes(channelID, parts[2:], w, r)
			return
		case "monetization":
			channel, ok := h.channelsService().GetChannel(channelID)
			if !ok {
				WriteError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			h.handleMonetizationRoutes(channel, parts[2:], w, r)
			return
		}
	}

	WriteError(w, http.StatusNotFound, fmt.Errorf("unknown channel path"))
}
