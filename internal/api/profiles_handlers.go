package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"bitriver-live/internal/models"
	"bitriver-live/internal/storage"
)

type cryptoAddressPayload struct {
	Currency string `json:"currency"`
	Address  string `json:"address"`
	Note     string `json:"note,omitempty"`
}

type socialLinkPayload struct {
	Platform string `json:"platform"`
	URL      string `json:"url"`
}

type upsertProfileRequest struct {
	DisplayName       *string                 `json:"displayName"`
	Email             *string                 `json:"email"`
	Bio               *string                 `json:"bio"`
	AvatarURL         *string                 `json:"avatarUrl"`
	BannerURL         *string                 `json:"bannerUrl"`
	SocialLinks       *[]socialLinkPayload    `json:"socialLinks"`
	FeaturedChannelID *string                 `json:"featuredChannelId"`
	TopFriends        *[]string               `json:"topFriends"`
	DonationAddresses *[]cryptoAddressPayload `json:"donationAddresses"`
}

type cryptoAddressResponse struct {
	Currency string `json:"currency"`
	Address  string `json:"address"`
	Note     string `json:"note,omitempty"`
}

type friendSummaryResponse struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
}

type socialLinkResponse struct {
	Platform string `json:"platform"`
	URL      string `json:"url"`
}

type profileViewResponse struct {
	UserID            string                  `json:"userId"`
	DisplayName       string                  `json:"displayName"`
	Bio               string                  `json:"bio"`
	AvatarURL         string                  `json:"avatarUrl"`
	BannerURL         string                  `json:"bannerUrl"`
	SocialLinks       []socialLinkResponse    `json:"socialLinks"`
	FeaturedChannelID *string                 `json:"featuredChannelId,omitempty"`
	TopFriends        []friendSummaryResponse `json:"topFriends"`
	DonationAddresses []cryptoAddressResponse `json:"donationAddresses"`
	Channels          []channelPublicResponse `json:"channels"`
	LiveChannels      []channelPublicResponse `json:"liveChannels"`
	CreatedAt         string                  `json:"createdAt"`
	UpdatedAt         string                  `json:"updatedAt"`
}

func (h *Handler) Profiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		profiles := h.Store.ListProfiles()
		response := make([]profileViewResponse, 0, len(profiles))
		for _, profile := range profiles {
			user, ok := h.Store.GetUser(profile.UserID)
			if !ok {
				continue
			}
			response = append(response, h.buildProfileViewResponse(user, profile))
		}
		WriteJSON(w, http.StatusOK, response)
	default:
		WriteMethodNotAllowed(w, r, http.MethodGet)
	}
}

func (h *Handler) ProfileByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/profiles/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		WriteError(w, http.StatusNotFound, fmt.Errorf("profile id missing"))
		return
	}
	userID := parts[0]

	switch r.Method {
	case http.MethodGet:
		h.handleGetProfile(userID, w, r)
	case http.MethodPut:
		actor, ok := h.requireAuthenticatedUser(w, r)
		if !ok {
			return
		}
		if actor.ID != userID && !actor.HasRole(roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}
		h.handleUpsertProfile(userID, w, r)
	default:
		WriteMethodNotAllowed(w, r, http.MethodGet, http.MethodPut)
	}
}

func (h *Handler) handleGetProfile(userID string, w http.ResponseWriter, r *http.Request) {
	user, ok := h.Store.GetUser(userID)
	if !ok {
		WriteError(w, http.StatusNotFound, fmt.Errorf("user %s not found", userID))
		return
	}
	profile, _ := h.Store.GetProfile(userID)
	WriteJSON(w, http.StatusOK, h.buildProfileViewResponse(user, profile))
}

func (h *Handler) handleUpsertProfile(userID string, w http.ResponseWriter, r *http.Request) {
	var req upsertProfileRequest
	if !DecodeAndValidate(w, r, &req) {
		return
	}

	user, ok := h.Store.GetUser(userID)
	if !ok {
		WriteError(w, http.StatusNotFound, fmt.Errorf("user %s not found", userID))
		return
	}

	userUpdate := storage.UserUpdate{}
	if req.DisplayName != nil {
		userUpdate.DisplayName = req.DisplayName
	}
	if req.Email != nil {
		userUpdate.Email = req.Email
	}
	if userUpdate.DisplayName != nil || userUpdate.Email != nil {
		updatedUser, err := h.Store.UpdateUser(userID, userUpdate)
		if err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		user = updatedUser
	}

	update := storage.ProfileUpdate{}
	if req.Bio != nil {
		update.Bio = req.Bio
	}
	if req.AvatarURL != nil {
		update.AvatarURL = req.AvatarURL
	}
	if req.BannerURL != nil {
		update.BannerURL = req.BannerURL
	}
	if req.SocialLinks != nil {
		links := make([]models.SocialLink, 0, len(*req.SocialLinks))
		for _, link := range *req.SocialLinks {
			links = append(links, models.SocialLink{
				Platform: link.Platform,
				URL:      link.URL,
			})
		}
		update.SocialLinks = &links
	}
	if req.FeaturedChannelID != nil {
		update.FeaturedChannelID = req.FeaturedChannelID
	}
	if req.TopFriends != nil {
		friendsCopy := append([]string{}, (*req.TopFriends)...)
		update.TopFriends = &friendsCopy
	}
	if req.DonationAddresses != nil {
		addresses := make([]models.CryptoAddress, 0, len(*req.DonationAddresses))
		for _, addr := range *req.DonationAddresses {
			normalized, err := storage.NormalizeDonationAddress(models.CryptoAddress{
				Currency: addr.Currency,
				Address:  addr.Address,
				Note:     addr.Note,
			})
			if err != nil {
				WriteError(w, http.StatusBadRequest, err)
				return
			}
			addresses = append(addresses, normalized)
		}
		update.DonationAddresses = &addresses
	}

	profile, err := h.Store.UpsertProfile(userID, update)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err)
		return
	}

	WriteJSON(w, http.StatusOK, h.buildProfileViewResponse(user, profile))
}

func (h *Handler) buildProfileViewResponse(user models.User, profile models.Profile) profileViewResponse {
	channels := h.Store.ListChannels(user.ID, "")
	channelResponses := make([]channelPublicResponse, 0, len(channels))
	liveResponses := make([]channelPublicResponse, 0)
	for _, channel := range channels {
		resp := newChannelPublicResponse(channel)
		channelResponses = append(channelResponses, resp)
		if channel.LiveState == "live" {
			liveResponses = append(liveResponses, resp)
		}
	}

	friends := make([]friendSummaryResponse, 0, len(profile.TopFriends))
	for _, friendID := range profile.TopFriends {
		friendUser, ok := h.Store.GetUser(friendID)
		if !ok {
			continue
		}
		friendProfile, _ := h.Store.GetProfile(friendID)
		friends = append(friends, friendSummaryResponse{
			UserID:      friendUser.ID,
			DisplayName: friendUser.DisplayName,
			AvatarURL:   friendProfile.AvatarURL,
		})
	}

	donations := make([]cryptoAddressResponse, 0, len(profile.DonationAddresses))
	for _, addr := range profile.DonationAddresses {
		donations = append(donations, cryptoAddressResponse{
			Currency: addr.Currency,
			Address:  addr.Address,
			Note:     addr.Note,
		})
	}

	socialLinks := make([]socialLinkResponse, 0, len(profile.SocialLinks))
	for _, link := range profile.SocialLinks {
		socialLinks = append(socialLinks, socialLinkResponse{Platform: link.Platform, URL: link.URL})
	}

	response := profileViewResponse{
		UserID:            user.ID,
		DisplayName:       user.DisplayName,
		Bio:               profile.Bio,
		AvatarURL:         profile.AvatarURL,
		BannerURL:         profile.BannerURL,
		SocialLinks:       socialLinks,
		TopFriends:        friends,
		DonationAddresses: donations,
		Channels:          channelResponses,
		LiveChannels:      liveResponses,
		CreatedAt:         profile.CreatedAt.Format(time.RFC3339Nano),
		UpdatedAt:         profile.UpdatedAt.Format(time.RFC3339Nano),
	}
	if profile.FeaturedChannelID != nil {
		id := *profile.FeaturedChannelID
		response.FeaturedChannelID = &id
	}
	return response
}
