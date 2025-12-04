package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"bitriver-live/internal/models"
	"bitriver-live/internal/observability/metrics"
	"bitriver-live/internal/storage"
)

type createChannelRequest struct {
	OwnerID  string   `json:"ownerId"`
	Title    string   `json:"title"`
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
}

type updateChannelRequest struct {
	Title    *string   `json:"title"`
	Category *string   `json:"category"`
	Tags     *[]string `json:"tags"`
}

type channelPublicResponse struct {
	ID               string   `json:"id"`
	OwnerID          string   `json:"ownerId"`
	Title            string   `json:"title"`
	Category         string   `json:"category,omitempty"`
	Tags             []string `json:"tags"`
	LiveState        string   `json:"liveState"`
	CurrentSessionID *string  `json:"currentSessionId,omitempty"`
	CreatedAt        string   `json:"createdAt"`
	UpdatedAt        string   `json:"updatedAt"`
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

func (h *Handler) Directory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteMethodNotAllowed(w, r, http.MethodGet)
		return
	}

	query := ""
	if r.URL != nil {
		query = strings.TrimSpace(r.URL.Query().Get("q"))
	}
	channels := h.Store.ListChannels("", query)
	h.writeDirectoryResponse(w, channels)
}

func (h *Handler) DirectoryFeatured(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteMethodNotAllowed(w, r, http.MethodGet)
		return
	}

	profiles := h.Store.ListProfiles()
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

	channels := make([]models.Channel, 0, len(channelIDs))
	for id := range channelIDs {
		if channel, ok := h.Store.GetChannel(id); ok {
			channels = append(channels, channel)
		}
	}

	h.writeDirectoryResponse(w, h.sortChannelsByFollowers(channels, true))
}

func (h *Handler) DirectoryRecommended(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteMethodNotAllowed(w, r, http.MethodGet)
		return
	}

	channels := h.Store.ListChannels("", "")
	h.writeDirectoryResponse(w, h.sortChannelsByFollowers(channels, false))
}

func (h *Handler) DirectoryLive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteMethodNotAllowed(w, r, http.MethodGet)
		return
	}

	channels := h.Store.ListChannels("", "")
	channels = filterLiveChannels(channels)
	h.writeDirectoryResponse(w, h.sortChannelsByFollowers(channels, true))
}

func (h *Handler) DirectoryTrending(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteMethodNotAllowed(w, r, http.MethodGet)
		return
	}

	channels := filterLiveChannels(h.Store.ListChannels("", ""))
	h.writeDirectoryResponse(w, h.sortChannelsByFollowers(channels, true))
}

func (h *Handler) DirectoryCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteMethodNotAllowed(w, r, http.MethodGet)
		return
	}

	channels := filterLiveChannels(h.Store.ListChannels("", ""))
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

func filterLiveChannels(channels []models.Channel) []models.Channel {
	live := make([]models.Channel, 0, len(channels))
	for _, channel := range channels {
		if channel.LiveState == "live" || channel.LiveState == "starting" {
			live = append(live, channel)
		}
	}
	return live
}

func (h *Handler) sortChannelsByFollowers(channels []models.Channel, liveFirst bool) []models.Channel {
	followers := make(map[string]int, len(channels))
	for _, channel := range channels {
		followers[channel.ID] = h.Store.CountFollowers(channel.ID)
	}
	sort.Slice(channels, func(i, j int) bool {
		if liveFirst {
			iLive := channels[i].LiveState == "live" || channels[i].LiveState == "starting"
			jLive := channels[j].LiveState == "live" || channels[j].LiveState == "starting"
			if iLive != jLive {
				return iLive
			}
		}
		if followers[channels[i].ID] == followers[channels[j].ID] {
			return channels[i].CreatedAt.Before(channels[j].CreatedAt)
		}
		return followers[channels[i].ID] > followers[channels[j].ID]
	})
	return channels
}

func (h *Handler) DirectoryFollowing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteMethodNotAllowed(w, r, http.MethodGet)
		return
	}

	viewer, ok := h.requireAuthenticatedUser(w, r)
	if !ok {
		return
	}

	channelIDs := h.Store.ListFollowedChannelIDs(viewer.ID)
	channels := make([]models.Channel, 0, len(channelIDs))
	for _, id := range channelIDs {
		channel, exists := h.Store.GetChannel(id)
		if !exists {
			continue
		}
		if channel.LiveState != "live" && channel.LiveState != "starting" {
			continue
		}
		channels = append(channels, channel)
	}

	h.writeDirectoryResponse(w, channels)
}

func (h *Handler) writeDirectoryResponse(w http.ResponseWriter, channels []models.Channel) {
	response := make([]directoryChannelResponse, 0, len(channels))
	for _, channel := range channels {
		owner, exists := h.Store.GetUser(channel.OwnerID)
		if !exists {
			continue
		}
		profile, _ := h.Store.GetProfile(owner.ID)
		followerCount := h.Store.CountFollowers(channel.ID)
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

func buildChannelResponse(channel models.Channel, includeStreamKey bool) channelResponse {
	resp := channelResponse{
		channelPublicResponse: channelPublicResponse{
			ID:        channel.ID,
			OwnerID:   channel.OwnerID,
			Title:     channel.Title,
			Category:  channel.Category,
			Tags:      append([]string{}, channel.Tags...),
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

func newChannelResponse(channel models.Channel) channelResponse {
	return buildChannelResponse(channel, true)
}

func newChannelPublicResponse(channel models.Channel) channelPublicResponse {
	return buildChannelResponse(channel, false).channelPublicResponse
}

func newOwnerResponse(user models.User, profile models.Profile) channelOwnerResponse {
	owner := channelOwnerResponse{ID: user.ID, DisplayName: user.DisplayName}
	if profile.AvatarURL != "" {
		owner.AvatarURL = profile.AvatarURL
	}
	return owner
}

func newProfileSummaryResponse(profile models.Profile) profileSummaryResponse {
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

func (h *Handler) subscriptionState(channelID string, actor *models.User) (subscriptionStateResponse, error) {
	subs, err := h.Store.ListSubscriptions(channelID, false)
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

		channels := h.Store.ListChannels(ownerID, "")
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
		actor, ok := h.requireRole(w, r, roleAdmin, roleCreator)
		if !ok {
			return
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
		channel, err := h.Store.CreateChannel(req.OwnerID, req.Title, req.Category, req.Tags)
		if err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		WriteJSON(w, http.StatusCreated, newChannelResponse(channel))
	default:
		WriteMethodNotAllowed(w, r, http.MethodGet, http.MethodPost)
	}
}

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
			channel, ok := h.Store.GetChannel(channelID)
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
			channel, ok := h.Store.GetChannel(channelID)
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
			update := storage.ChannelUpdate{}
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
			channel, err := h.Store.UpdateChannel(channelID, update)
			if err != nil {
				WriteError(w, http.StatusBadRequest, err)
				return
			}
			WriteJSON(w, http.StatusOK, newChannelResponse(channel))
		case http.MethodDelete:
			channel, ok := h.Store.GetChannel(channelID)
			if !ok {
				WriteError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			if _, ok := h.ensureChannelAccess(w, r, channel); !ok {
				return
			}
			if err := h.Store.DeleteChannel(channelID); err != nil {
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
			channel, ok := h.Store.GetChannel(channelID)
			if !ok {
				WriteError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			if r.Method != http.MethodGet {
				WriteMethodNotAllowed(w, r, http.MethodGet)
				return
			}
			owner, exists := h.Store.GetUser(channel.OwnerID)
			if !exists {
				WriteError(w, http.StatusInternalServerError, fmt.Errorf("channel owner %s not found", channel.OwnerID))
				return
			}
			profile, _ := h.Store.GetProfile(owner.ID)
			follow := followStateResponse{Followers: h.Store.CountFollowers(channel.ID)}
			var viewer *models.User
			if actor, ok := UserFromContext(r.Context()); ok {
				follow.Following = h.Store.IsFollowingChannel(actor.ID, channel.ID)
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
			if session, live := h.Store.CurrentStreamSession(channel.ID); live {
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
			channel, ok := h.Store.GetChannel(channelID)
			if !ok {
				WriteError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			h.handleStreamRoutes(channel, parts[2:], w, r)
			return
		case "sessions":
			channel, ok := h.Store.GetChannel(channelID)
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
			sessions, err := h.Store.ListStreamSessions(channelID)
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
			if len(parts) > 2 {
				WriteError(w, http.StatusNotFound, fmt.Errorf("unknown channel path"))
				return
			}
			if _, ok := h.Store.GetChannel(channelID); !ok {
				WriteError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			actor, ok := h.requireAuthenticatedUser(w, r)
			if !ok {
				return
			}
			switch r.Method {
			case http.MethodPost:
				if err := h.Store.FollowChannel(actor.ID, channelID); err != nil {
					WriteError(w, http.StatusBadRequest, err)
					return
				}
			case http.MethodDelete:
				if err := h.Store.UnfollowChannel(actor.ID, channelID); err != nil {
					WriteError(w, http.StatusBadRequest, err)
					return
				}
			default:
				WriteMethodNotAllowed(w, r, http.MethodPost, http.MethodDelete)
				return
			}
			state := followStateResponse{
				Followers: h.Store.CountFollowers(channelID),
				Following: h.Store.IsFollowingChannel(actor.ID, channelID),
			}
			WriteJSON(w, http.StatusOK, state)
			return
		case "subscribe":
			if len(parts) > 2 {
				WriteError(w, http.StatusNotFound, fmt.Errorf("unknown channel path"))
				return
			}
			channel, ok := h.Store.GetChannel(channelID)
			if !ok {
				WriteError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			switch r.Method {
			case http.MethodGet:
				var viewer *models.User
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
				subs, err := h.Store.ListSubscriptions(channel.ID, false)
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
				if !alreadySubscribed {
					params := storage.CreateSubscriptionParams{
						ChannelID: channel.ID,
						UserID:    actor.ID,
						Tier:      "supporter",
						Provider:  "internal",
						Amount:    models.NewMoneyFromMinorUnits(0),
						Currency:  "USD",
						Duration:  30 * 24 * time.Hour,
						AutoRenew: true,
					}
					sub, err := h.Store.CreateSubscription(params)
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
				subs, err := h.Store.ListSubscriptions(channel.ID, false)
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
					if _, err := h.Store.CancelSubscription(subscriptionID, actor.ID, ""); err != nil {
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
			channel, ok := h.Store.GetChannel(channelID)
			if !ok {
				WriteError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			uploads, err := h.Store.ListUploads(channelID)
			if err != nil {
				WriteError(w, http.StatusBadRequest, err)
				return
			}
			items := make([]vodItemResponse, 0, len(uploads))
			for _, upload := range uploads {
				if upload.RecordingID == nil {
					continue
				}
				recording, ok := h.Store.GetRecording(*upload.RecordingID)
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
			channel, ok := h.Store.GetChannel(channelID)
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
