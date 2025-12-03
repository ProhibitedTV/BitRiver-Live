package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"bitriver-live/internal/ingest"
	"bitriver-live/internal/models"
)

type dataset struct {
	Users               map[string]models.User          `json:"users"`
	OAuthAccounts       map[string]models.OAuthAccount  `json:"oauthAccounts"`
	Channels            map[string]models.Channel       `json:"channels"`
	StreamSessions      map[string]models.StreamSession `json:"streamSessions"`
	ChatMessages        map[string]models.ChatMessage   `json:"chatMessages"`
	ChatBans            map[string]map[string]time.Time `json:"chatBans"`
	ChatTimeouts        map[string]map[string]time.Time `json:"chatTimeouts"`
	ChatBanActors       map[string]map[string]string    `json:"chatBanActors"`
	ChatBanReasons      map[string]map[string]string    `json:"chatBanReasons"`
	ChatTimeoutActors   map[string]map[string]string    `json:"chatTimeoutActors"`
	ChatTimeoutReasons  map[string]map[string]string    `json:"chatTimeoutReasons"`
	ChatTimeoutIssuedAt map[string]map[string]time.Time `json:"chatTimeoutIssuedAt"`
	ChatReports         map[string]models.ChatReport    `json:"chatReports"`
	Tips                map[string]models.Tip           `json:"tips"`
	Subscriptions       map[string]models.Subscription  `json:"subscriptions"`
	Profiles            map[string]models.Profile       `json:"profiles"`
	Follows             map[string]map[string]time.Time `json:"follows"`
	Recordings          map[string]models.Recording     `json:"recordings"`
	Uploads             map[string]models.Upload        `json:"uploads"`
	ClipExports         map[string]models.ClipExport    `json:"clipExports"`
}

type Storage struct {
	mu       sync.RWMutex
	filePath string
	data     dataset
	// persistOverride allows tests to intercept persist operations.
	persistOverride     func(dataset) error
	ingestController    ingest.Controller
	ingestMaxAttempts   int
	ingestRetryInterval time.Duration
	ingestTimeout       time.Duration
	ingestHealth        []ingest.HealthStatus
	ingestHealthUpdated time.Time
	recordingRetention  RecordingRetentionPolicy
	objectStorage       ObjectStorageConfig
	objectClient        objectStorageClient
}

// Ping always reports success for the JSON-backed repository.
func (s *Storage) Ping(context.Context) error {
	return nil
}

func applyObjectStorageDefaults(cfg ObjectStorageConfig) ObjectStorageConfig {
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = defaultObjectStorageRequestTimeout
	}
	return cfg
}

func (cfg ObjectStorageConfig) requestTimeout() time.Duration {
	if cfg.RequestTimeout <= 0 {
		return defaultObjectStorageRequestTimeout
	}
	return cfg.RequestTimeout
}

type objectStorageClient interface {
	Enabled() bool
	Upload(ctx context.Context, key, contentType string, body []byte) (objectReference, error)
	Delete(ctx context.Context, key string) error
}

type objectReference struct {
	Key string
	URL string
}

type noopObjectStorageClient struct{}

func (noopObjectStorageClient) Enabled() bool { return false }

func (noopObjectStorageClient) Upload(ctx context.Context, key, contentType string, body []byte) (objectReference, error) {
	return objectReference{}, nil
}

func (noopObjectStorageClient) Delete(ctx context.Context, key string) error {
	return nil
}

func newObjectStorageClient(cfg ObjectStorageConfig) objectStorageClient {
	cfg = applyObjectStorageDefaults(cfg)
	trimmedBucket := strings.TrimSpace(cfg.Bucket)
	trimmedEndpoint := strings.TrimSpace(cfg.Endpoint)
	if trimmedBucket == "" || trimmedEndpoint == "" {
		return noopObjectStorageClient{}
	}
	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}
	endpoint := trimmedEndpoint
	if strings.Contains(endpoint, "://") {
		if parsed, err := url.Parse(endpoint); err == nil {
			endpoint = parsed.Host
		}
	}
	baseURL := &url.URL{Scheme: scheme, Host: endpoint}
	if baseURL.Host == "" {
		return noopObjectStorageClient{}
	}
	sanitized := cfg
	sanitized.Bucket = trimmedBucket
	client := &s3ObjectStorageClient{
		cfg:        sanitized,
		endpoint:   baseURL,
		httpClient: &http.Client{Timeout: sanitized.RequestTimeout},
	}
	return client
}

type s3ObjectStorageClient struct {
	cfg        ObjectStorageConfig
	endpoint   *url.URL
	httpClient *http.Client
}

func (c *s3ObjectStorageClient) Enabled() bool { return true }

func (c *s3ObjectStorageClient) Upload(ctx context.Context, key, contentType string, body []byte) (objectReference, error) {
	finalKey := c.applyPrefix(key)
	target := c.objectURL(finalKey)
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, target.String(), bytes.NewReader(body))
	if err != nil {
		return objectReference{}, fmt.Errorf("create upload request: %w", err)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	hash := hashSHA256Hex(body)
	if err := c.signRequest(request, hash); err != nil {
		return objectReference{}, err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return objectReference{}, fmt.Errorf("upload object %s: %w", finalKey, err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return objectReference{}, fmt.Errorf("upload object %s: unexpected status %d", finalKey, response.StatusCode)
	}
	return objectReference{Key: finalKey, URL: c.publicURL(finalKey)}, nil
}

func (c *s3ObjectStorageClient) Delete(ctx context.Context, key string) error {
	finalKey := c.applyPrefix(key)
	target := c.objectURL(finalKey)
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, target.String(), nil)
	if err != nil {
		return fmt.Errorf("create delete request: %w", err)
	}
	if err := c.signRequest(request, emptyPayloadHash); err != nil {
		return err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("delete object %s: %w", finalKey, err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("delete object %s: unexpected status %d", finalKey, response.StatusCode)
}

func (c *s3ObjectStorageClient) applyPrefix(key string) string {
	trimmed := strings.TrimLeft(strings.TrimSpace(key), "/")
	prefix := strings.Trim(strings.TrimSpace(c.cfg.Prefix), "/")
	if prefix == "" {
		return trimmed
	}
	if trimmed == "" {
		return prefix
	}
	if trimmed == prefix || strings.HasPrefix(trimmed, prefix+"/") {
		return trimmed
	}
	return prefix + "/" + trimmed
}

func (c *s3ObjectStorageClient) objectURL(finalKey string) *url.URL {
	basePath := strings.TrimRight(c.endpoint.Path, "/")
	path := "/" + strings.TrimLeft(c.cfg.Bucket, "/")
	trimmedKey := strings.TrimLeft(finalKey, "/")
	if trimmedKey != "" {
		path += "/" + trimmedKey
	}
	if basePath != "" {
		path = basePath + path
	}
	u := *c.endpoint
	u.Path = path
	return &u
}

func (c *s3ObjectStorageClient) publicURL(key string) string {
	base := strings.TrimSpace(c.cfg.PublicEndpoint)
	if base == "" {
		return ""
	}
	trimmedBase := strings.TrimRight(base, "/")
	trimmedKey := strings.TrimLeft(key, "/")
	if trimmedKey == "" {
		return trimmedBase
	}
	return trimmedBase + "/" + trimmedKey
}

func (c *s3ObjectStorageClient) signRequest(req *http.Request, payloadHash string) error {
	req.Host = req.URL.Host
	req.Header.Set("Host", req.URL.Host)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	accessKey := strings.TrimSpace(c.cfg.AccessKey)
	secretKey := strings.TrimSpace(c.cfg.SecretKey)
	if accessKey == "" || secretKey == "" {
		return nil
	}
	region := strings.TrimSpace(c.cfg.Region)
	if region == "" {
		region = "us-east-1"
	}
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	req.Header.Set("x-amz-date", amzDate)
	canonicalHeaders, signedHeaders := canonicalizeHeaders(req)
	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI(req.URL),
		canonicalQuery(req.URL),
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")
	hash := sha256.Sum256([]byte(canonicalRequest))
	scope := strings.Join([]string{dateStamp, region, "s3", "aws4_request"}, "/")
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		hex.EncodeToString(hash[:]),
	}, "\n")
	signingKey := deriveSigningKey(secretKey, dateStamp, region)
	signature := hmacSHA256Hex(signingKey, stringToSign)
	authorization := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey,
		scope,
		signedHeaders,
		signature,
	)
	req.Header.Set("Authorization", authorization)
	return nil
}

func canonicalizeHeaders(req *http.Request) (string, string) {
	headerMap := make(map[string][]string)
	for key, values := range req.Header {
		lower := strings.ToLower(key)
		if lower == "authorization" {
			continue
		}
		cleaned := make([]string, 0, len(values))
		for _, v := range values {
			cleaned = append(cleaned, strings.TrimSpace(v))
		}
		headerMap[lower] = cleaned
	}
	if _, ok := headerMap["host"]; !ok && req.Host != "" {
		headerMap["host"] = []string{req.Host}
	}
	keys := make([]string, 0, len(headerMap))
	for key := range headerMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	var signed []string
	for _, key := range keys {
		values := headerMap[key]
		builder.WriteString(key)
		builder.WriteByte(':')
		builder.WriteString(strings.Join(values, ","))
		builder.WriteByte('\n')
		signed = append(signed, key)
	}
	return builder.String(), strings.Join(signed, ";")
}

func canonicalURI(u *url.URL) string {
	if u == nil {
		return "/"
	}
	path := u.EscapedPath()
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

func canonicalQuery(u *url.URL) string {
	if u == nil {
		return ""
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var parts []string
	for _, key := range keys {
		vals := values[key]
		sort.Strings(vals)
		for _, v := range vals {
			parts = append(parts, url.QueryEscape(key)+"="+url.QueryEscape(v))
		}
	}
	return strings.Join(parts, "&")
}

func deriveSigningKey(secret, dateStamp, region string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte("s3"))
	return hmacSHA256(kService, []byte("aws4_request"))
}

func hmacSHA256(key []byte, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func hmacSHA256Hex(key []byte, data string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

var emptyPayloadHash = hashSHA256Hex(nil)

func hashSHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ClipExportParams captures the request to generate a recording clip.
type ClipExportParams struct {
	Title        string
	StartSeconds int
	EndSeconds   int
}

func newDataset() dataset {
	ds := dataset{
		Users:          make(map[string]models.User),
		OAuthAccounts:  make(map[string]models.OAuthAccount),
		Channels:       make(map[string]models.Channel),
		StreamSessions: make(map[string]models.StreamSession),
		Tips:           make(map[string]models.Tip),
		Subscriptions:  make(map[string]models.Subscription),
		Profiles:       make(map[string]models.Profile),
		Follows:        make(map[string]map[string]time.Time),
		Recordings:     make(map[string]models.Recording),
		ClipExports:    make(map[string]models.ClipExport),
	}
	initChatDataset(&ds)
	return ds
}

func (s *Storage) ensureDatasetInitializedLocked() {
	if s.data.Users == nil {
		s.data.Users = make(map[string]models.User)
	}
	if s.data.OAuthAccounts == nil {
		s.data.OAuthAccounts = make(map[string]models.OAuthAccount)
	}
	if s.data.Channels == nil {
		s.data.Channels = make(map[string]models.Channel)
	}
	if s.data.StreamSessions == nil {
		s.data.StreamSessions = make(map[string]models.StreamSession)
	}
	s.ensureChatDatasetInitializedLocked()
	if s.data.Tips == nil {
		s.data.Tips = make(map[string]models.Tip)
	}
	if s.data.Subscriptions == nil {
		s.data.Subscriptions = make(map[string]models.Subscription)
	}
	if s.data.Profiles == nil {
		s.data.Profiles = make(map[string]models.Profile)
	}
	if s.data.Follows == nil {
		s.data.Follows = make(map[string]map[string]time.Time)
	}
	if s.data.Recordings == nil {
		s.data.Recordings = make(map[string]models.Recording)
	}
	if s.data.Uploads == nil {
		s.data.Uploads = make(map[string]models.Upload)
	}
	if s.data.ClipExports == nil {
		s.data.ClipExports = make(map[string]models.ClipExport)
	}
}

func buildObjectKey(parts ...string) string {
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.Trim(part, "/")
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return strings.Join(normalized, "/")
}

func normalizeObjectComponent(input string) string {
	lowered := strings.ToLower(strings.TrimSpace(input))
	if lowered == "" {
		return "item"
	}
	var builder strings.Builder
	lastDash := false
	for _, r := range lowered {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	normalized := strings.Trim(builder.String(), "-")
	if normalized == "" {
		return "item"
	}
	return normalized
}

func manifestMetadataKey(name string) string {
	return metadataManifestPrefix + normalizeObjectComponent(name)
}

func thumbnailMetadataKey(id string) string {
	return metadataThumbnailPrefix + id
}

// CreateUploadParams captures the information required to store an uploaded asset.
type CreateUploadParams struct {
	ChannelID   string
	Title       string
	Filename    string
	SizeBytes   int64
	Metadata    map[string]string
	PlaybackURL string
}

// UploadUpdate describes the mutable fields of an upload entry.
type UploadUpdate struct {
	Title       *string
	Status      *string
	Progress    *int
	RecordingID *string
	PlaybackURL *string
	Metadata    map[string]string
	Error       *string
	CompletedAt *time.Time
}

func normalizeRoles(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	roles := make([]string, 0, len(input))
	seen := make(map[string]struct{})
	for _, role := range input {
		trimmed := strings.TrimSpace(role)
		if trimmed == "" {
			continue
		}
		normalized := strings.ToLower(trimmed)
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		roles = append(roles, normalized)
	}
	if len(roles) == 0 {
		return nil
	}
	sort.Strings(roles)
	return roles
}

func oauthAccountKey(provider, subject string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	subject = strings.TrimSpace(subject)
	return provider + "|" + subject
}

func fallbackOAuthEmail(provider, subject string) string {
	domain := strings.ToLower(strings.TrimSpace(provider))
	if domain == "" {
		domain = "provider"
	}
	domain = sanitizeOAuthComponent(domain)
	hash := sha256.Sum256([]byte(provider + ":" + subject))
	local := hex.EncodeToString(hash[:8])
	return fmt.Sprintf("%s@%s.oauth", local, domain)
}

func defaultOAuthDisplayName(provider, email, subject string) string {
	trimmedEmail := strings.TrimSpace(email)
	if trimmedEmail != "" {
		local := strings.SplitN(trimmedEmail, "@", 2)[0]
		local = strings.ReplaceAll(local, ".", " ")
		local = strings.TrimSpace(local)
		if local != "" {
			return capitalizeWord(local)
		}
	}
	sanitized := sanitizeOAuthComponent(subject)
	if sanitized != "" {
		return sanitized
	}
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return "Viewer"
	}
	return capitalizeWord(provider) + " user"
}

func sanitizeOAuthComponent(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	if lower == "" {
		return ""
	}
	var builder strings.Builder
	for _, r := range lower {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == ' ':
			builder.WriteRune(' ')
		default:
			builder.WriteRune('-')
		}
	}
	return strings.TrimSpace(builder.String())
}

func capitalizeWord(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	return strings.ToUpper(lower[:1]) + lower[1:]
}

func NewStorage(path string, opts ...Option) (*Storage, error) {
	store := &Storage{
		filePath:            path,
		ingestController:    ingest.NoopController{},
		ingestMaxAttempts:   1,
		ingestTimeout:       defaultIngestOperationTimeout,
		ingestHealth:        []ingest.HealthStatus{{Component: "ingest", Status: "disabled"}},
		ingestHealthUpdated: time.Now().UTC(),
		recordingRetention: RecordingRetentionPolicy{
			Published:   90 * 24 * time.Hour,
			Unpublished: 14 * 24 * time.Hour,
		},
		objectClient: noopObjectStorageClient{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt.applyJSON(store)
		}
	}
	if store.ingestController == nil {
		store.ingestController = ingest.NoopController{}
	}
	if store.ingestMaxAttempts <= 0 {
		store.ingestMaxAttempts = 1
	}
	store.ingestTimeout = normalizeIngestTimeout(store.ingestTimeout)
	if err := store.load(); err != nil {
		return nil, err
	}
	store.objectStorage = applyObjectStorageDefaults(store.objectStorage)
	store.objectClient = newObjectStorageClient(store.objectStorage)
	return store, nil
}

func (s *Storage) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	file, err := os.Open(s.filePath)
	if errors.Is(err, os.ErrNotExist) {
		s.data = newDataset()
		return nil
	} else if err != nil {
		return fmt.Errorf("open store file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&s.data); err != nil {
		if errors.Is(err, io.EOF) {
			s.data = newDataset()
			return nil
		}
		return fmt.Errorf("decode store file: %w", err)
	}

	s.ensureDatasetInitializedLocked()

	return nil
}

func (s *Storage) persist() error {
	return s.persistDataset(s.data)
}

func (s *Storage) persistDataset(data dataset) error {
	if s.persistOverride != nil {
		if err := s.persistOverride(data); err != nil {
			return err
		}
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, "store-*.json")
	if err != nil {
		return fmt.Errorf("create temp store file: %w", err)
	}
	tmpPath := tmpFile.Name()
	success := false
	defer func() {
		if !success {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("encode store file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("flush store file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp store file: %w", err)
	}

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		return fmt.Errorf("replace store file: %w", err)
	}
	success = true
	return nil
}

func cloneDataset(src dataset) dataset {
	clone := dataset{}

	if src.Users != nil {
		clone.Users = make(map[string]models.User, len(src.Users))
		for id, user := range src.Users {
			cloned := user
			if user.Roles != nil {
				cloned.Roles = append([]string(nil), user.Roles...)
			}
			clone.Users[id] = cloned
		}
	}

	if src.OAuthAccounts != nil {
		clone.OAuthAccounts = make(map[string]models.OAuthAccount, len(src.OAuthAccounts))
		for key, account := range src.OAuthAccounts {
			clone.OAuthAccounts[key] = account
		}
	}

	if src.Channels != nil {
		clone.Channels = make(map[string]models.Channel, len(src.Channels))
		for id, channel := range src.Channels {
			cloned := channel
			if channel.Tags != nil {
				cloned.Tags = append([]string(nil), channel.Tags...)
			}
			if channel.CurrentSessionID != nil {
				current := *channel.CurrentSessionID
				cloned.CurrentSessionID = &current
			}
			clone.Channels[id] = cloned
		}
	}

	if src.StreamSessions != nil {
		clone.StreamSessions = make(map[string]models.StreamSession, len(src.StreamSessions))
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
		clone.Tips = make(map[string]models.Tip, len(src.Tips))
		for id, tip := range src.Tips {
			clone.Tips[id] = tip
		}
	}

	if src.Subscriptions != nil {
		clone.Subscriptions = make(map[string]models.Subscription, len(src.Subscriptions))
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
		clone.Recordings = make(map[string]models.Recording, len(src.Recordings))
		for id, recording := range src.Recordings {
			clone.Recordings[id] = cloneRecording(recording)
		}
	}

	if src.Uploads != nil {
		clone.Uploads = make(map[string]models.Upload, len(src.Uploads))
		for id, upload := range src.Uploads {
			clone.Uploads[id] = cloneUpload(upload)
		}
	}

	if src.ClipExports != nil {
		clone.ClipExports = make(map[string]models.ClipExport, len(src.ClipExports))
		for id, clip := range src.ClipExports {
			clone.ClipExports[id] = cloneClipExport(clip)
		}
	}

	if src.Profiles != nil {
		clone.Profiles = make(map[string]models.Profile, len(src.Profiles))
		for id, profile := range src.Profiles {
			cloned := profile
			if profile.SocialLinks != nil {
				cloned.SocialLinks = append([]models.SocialLink(nil), profile.SocialLinks...)
			}
			if profile.TopFriends != nil {
				cloned.TopFriends = append([]string(nil), profile.TopFriends...)
			}
			if profile.DonationAddresses != nil {
				cloned.DonationAddresses = append([]models.CryptoAddress(nil), profile.DonationAddresses...)
			}
			if profile.FeaturedChannelID != nil {
				featured := *profile.FeaturedChannelID
				cloned.FeaturedChannelID = &featured
			}
			clone.Profiles[id] = cloned
		}
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

func cloneRecording(recording models.Recording) models.Recording {
	cloned := recording
	if recording.Renditions != nil {
		cloned.Renditions = append([]models.RecordingRendition(nil), recording.Renditions...)
	}
	if recording.Thumbnails != nil {
		cloned.Thumbnails = append([]models.RecordingThumbnail(nil), recording.Thumbnails...)
	}
	if recording.Metadata != nil {
		meta := make(map[string]string, len(recording.Metadata))
		for k, v := range recording.Metadata {
			meta[k] = v
		}
		cloned.Metadata = meta
	}
	if recording.PublishedAt != nil {
		published := *recording.PublishedAt
		cloned.PublishedAt = &published
	}
	if recording.RetainUntil != nil {
		retain := *recording.RetainUntil
		cloned.RetainUntil = &retain
	}
	if recording.Clips != nil {
		cloned.Clips = append([]models.ClipExportSummary(nil), recording.Clips...)
	}
	return cloned
}

func cloneUpload(upload models.Upload) models.Upload {
	cloned := upload
	if upload.Metadata != nil {
		meta := make(map[string]string, len(upload.Metadata))
		for k, v := range upload.Metadata {
			meta[k] = v
		}
		cloned.Metadata = meta
	}
	if upload.RecordingID != nil {
		recording := *upload.RecordingID
		cloned.RecordingID = &recording
	}
	if upload.CompletedAt != nil {
		completed := *upload.CompletedAt
		cloned.CompletedAt = &completed
	}
	return cloned
}

func cloneClipExport(clip models.ClipExport) models.ClipExport {
	cloned := clip
	if clip.CompletedAt != nil {
		completed := *clip.CompletedAt
		cloned.CompletedAt = &completed
	}
	return cloned
}

func (s *Storage) recordingDeadline(now time.Time, published bool) *time.Time {
	var window time.Duration
	if published {
		window = s.recordingRetention.Published
	} else {
		window = s.recordingRetention.Unpublished
	}
	if window <= 0 {
		return nil
	}
	deadline := now.Add(window)
	return &deadline
}

func (s *Storage) purgeExpiredRecordingsLocked(now time.Time) (bool, dataset, error) {
	if len(s.data.Recordings) == 0 {
		return false, dataset{}, nil
	}
	removed := false
	snapshotTaken := false
	var snapshot dataset
	for id, recording := range s.data.Recordings {
		if recording.RetainUntil == nil || now.Before(*recording.RetainUntil) {
			continue
		}
		if !snapshotTaken {
			snapshot = cloneDataset(s.data)
			snapshotTaken = true
		}
		if err := s.deleteRecordingArtifactsLocked(recording); err != nil {
			if snapshotTaken {
				s.data = snapshot
			}
			return false, dataset{}, err
		}
		for clipID, clip := range s.data.ClipExports {
			if clip.RecordingID != id {
				continue
			}
			if err := s.deleteClipArtifactsLocked(clip); err != nil {
				if snapshotTaken {
					s.data = snapshot
				}
				return false, dataset{}, err
			}
			delete(s.data.ClipExports, clipID)
		}
		delete(s.data.Recordings, id)
		removed = true
	}
	if !removed {
		return false, dataset{}, nil
	}
	return true, snapshot, nil
}

func (s *Storage) recordingWithClipsLocked(recording models.Recording) models.Recording {
	cloned := cloneRecording(recording)
	if len(s.data.ClipExports) == 0 {
		return cloned
	}
	var clips []models.ClipExportSummary
	for _, clip := range s.data.ClipExports {
		if clip.RecordingID != recording.ID {
			continue
		}
		clips = append(clips, models.ClipExportSummary{
			ID:           clip.ID,
			Title:        clip.Title,
			StartSeconds: clip.StartSeconds,
			EndSeconds:   clip.EndSeconds,
			Status:       clip.Status,
		})
	}
	if len(clips) == 0 {
		return cloned
	}
	sort.Slice(clips, func(i, j int) bool {
		if clips[i].StartSeconds == clips[j].StartSeconds {
			return clips[i].ID < clips[j].ID
		}
		return clips[i].StartSeconds < clips[j].StartSeconds
	})
	cloned.Clips = clips
	return cloned
}

// User operations

func (s *Storage) CreateUser(params CreateUserParams) (models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalizedEmail := strings.TrimSpace(strings.ToLower(params.Email))
	if normalizedEmail == "" {
		return models.User{}, errors.New("email is required")
	}
	for _, user := range s.data.Users {
		if user.Email == normalizedEmail {
			return models.User{}, fmt.Errorf("email %s already in use", params.Email)
		}
	}

	displayName := strings.TrimSpace(params.DisplayName)
	if displayName == "" {
		return models.User{}, errors.New("displayName is required")
	}

	roles := normalizeRoles(params.Roles)
	if params.SelfSignup {
		if params.Password == "" {
			return models.User{}, errors.New("password is required for self-service signup")
		}
		if len(roles) == 0 {
			roles = []string{"viewer"}
		}
	}

	id, err := generateID()
	if err != nil {
		return models.User{}, err
	}

	var passwordHash string
	if params.Password != "" {
		hashed, hashErr := hashPassword(params.Password)
		if hashErr != nil {
			return models.User{}, fmt.Errorf("hash password: %w", hashErr)
		}
		passwordHash = hashed
	}

	now := time.Now().UTC()
	user := models.User{
		ID:           id,
		DisplayName:  displayName,
		Email:        normalizedEmail,
		Roles:        roles,
		PasswordHash: passwordHash,
		SelfSignup:   params.SelfSignup,
		CreatedAt:    now,
	}

	s.data.Users[id] = user
	if err := s.persist(); err != nil {
		delete(s.data.Users, id)
		return models.User{}, err
	}

	return user, nil
}

func (s *Storage) ListUsers() []models.User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]models.User, 0, len(s.data.Users))
	for _, user := range s.data.Users {
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].CreatedAt.Before(users[j].CreatedAt)
	})
	return users
}

func (s *Storage) GetUser(id string) (models.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.data.Users[id]
	return user, ok
}

// FindUserByEmail looks up a user by their normalized email address.
func (s *Storage) FindUserByEmail(email string) (models.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	normalizedEmail := strings.TrimSpace(strings.ToLower(email))
	for _, user := range s.data.Users {
		if user.Email == normalizedEmail {
			return user, true
		}
	}
	return models.User{}, false
}

func (s *Storage) AuthenticateOAuth(params OAuthLoginParams) (models.User, error) {
	provider := strings.ToLower(strings.TrimSpace(params.Provider))
	subject := strings.TrimSpace(params.Subject)
	if provider == "" {
		return models.User{}, errors.New("provider is required")
	}
	if subject == "" {
		return models.User{}, errors.New("subject is required")
	}

	normalizedEmail := strings.TrimSpace(strings.ToLower(params.Email))
	if normalizedEmail == "" {
		normalizedEmail = fallbackOAuthEmail(provider, subject)
	}

	displayName := strings.TrimSpace(params.DisplayName)
	if displayName == "" {
		displayName = defaultOAuthDisplayName(provider, normalizedEmail, subject)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDatasetInitializedLocked()

	if s.data.OAuthAccounts == nil {
		s.data.OAuthAccounts = make(map[string]models.OAuthAccount)
	}

	key := oauthAccountKey(provider, subject)
	if account, ok := s.data.OAuthAccounts[key]; ok {
		if user, ok := s.data.Users[account.UserID]; ok {
			return user, nil
		}
		delete(s.data.OAuthAccounts, key)
	}

	var (
		user   models.User
		exists bool
	)
	for _, existing := range s.data.Users {
		if existing.Email == normalizedEmail {
			user = existing
			exists = true
			break
		}
	}

	now := time.Now().UTC()
	if !exists {
		id, err := generateID()
		if err != nil {
			return models.User{}, err
		}
		user = models.User{
			ID:          id,
			DisplayName: displayName,
			Email:       normalizedEmail,
			Roles:       []string{"viewer"},
			SelfSignup:  true,
			CreatedAt:   now,
		}
	} else {
		if strings.TrimSpace(user.DisplayName) == "" {
			user.DisplayName = displayName
		}
	}

	s.data.Users[user.ID] = user
	s.data.OAuthAccounts[key] = models.OAuthAccount{
		Provider:    provider,
		Subject:     subject,
		UserID:      user.ID,
		Email:       normalizedEmail,
		DisplayName: displayName,
		LinkedAt:    now,
	}

	if err := s.persist(); err != nil {
		if !exists {
			delete(s.data.Users, user.ID)
		} else {
			s.data.Users[user.ID] = user
		}
		delete(s.data.OAuthAccounts, key)
		return models.User{}, err
	}

	return user, nil
}

// UserUpdate represents the fields that can be modified for an existing user.
type UserUpdate struct {
	DisplayName *string
	Email       *string
	Roles       *[]string
}

// UpdateUser mutates user metadata while enforcing uniqueness constraints.
func (s *Storage) UpdateUser(id string, update UserUpdate) (models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updatedData := cloneDataset(s.data)

	user, ok := updatedData.Users[id]
	if !ok {
		return models.User{}, fmt.Errorf("user %s not found", id)
	}

	if update.DisplayName != nil {
		name := strings.TrimSpace(*update.DisplayName)
		if name == "" {
			return models.User{}, errors.New("displayName cannot be empty")
		}
		user.DisplayName = name
	}

	if update.Email != nil {
		email := strings.TrimSpace(strings.ToLower(*update.Email))
		if email == "" {
			return models.User{}, errors.New("email cannot be empty")
		}
		for existingID, existing := range updatedData.Users {
			if existingID == user.ID {
				continue
			}
			if existing.Email == email {
				return models.User{}, fmt.Errorf("email %s already in use", email)
			}
		}
		user.Email = email
	}

	if update.Roles != nil {
		user.Roles = normalizeRoles(*update.Roles)
	}

	updatedData.Users[id] = user
	if err := s.persistDataset(updatedData); err != nil {
		return models.User{}, err
	}

	s.data = updatedData

	return user, nil
}

// SetUserPassword replaces the stored password hash for the provided user.
// DeleteUser removes the user, related profile, and chat history.
func (s *Storage) DeleteUser(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	updatedData := cloneDataset(s.data)

	if _, ok := updatedData.Users[id]; !ok {
		return fmt.Errorf("user %s not found", id)
	}

	for _, channel := range updatedData.Channels {
		if channel.OwnerID == id {
			return fmt.Errorf("user %s owns channel %s; transfer or delete the channel first", id, channel.ID)
		}
	}

	delete(updatedData.Users, id)
	delete(updatedData.Profiles, id)
	delete(updatedData.Follows, id)

	now := time.Now().UTC()
	for profileID, profile := range updatedData.Profiles {
		filtered := make([]string, 0, len(profile.TopFriends))
		for _, friend := range profile.TopFriends {
			if friend == id {
				continue
			}
			filtered = append(filtered, friend)
		}
		if len(filtered) != len(profile.TopFriends) {
			profile.TopFriends = filtered
			profile.UpdatedAt = now
			updatedData.Profiles[profileID] = profile
		}
	}

	for messageID, message := range updatedData.ChatMessages {
		if message.UserID == id {
			delete(updatedData.ChatMessages, messageID)
		}
	}

	if err := s.persistDataset(updatedData); err != nil {
		return err
	}

	s.data = updatedData

	return nil
}

// Profile operations

type ProfileUpdate struct {
	Bio               *string
	AvatarURL         *string
	BannerURL         *string
	SocialLinks       *[]models.SocialLink
	FeaturedChannelID *string
	TopFriends        *[]string
	DonationAddresses *[]models.CryptoAddress
}

func (s *Storage) UpsertProfile(userID string, update ProfileUpdate) (models.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updatedData := cloneDataset(s.data)

	if _, ok := updatedData.Users[userID]; !ok {
		return models.Profile{}, fmt.Errorf("user %s not found", userID)
	}

	profile, exists := updatedData.Profiles[userID]
	now := time.Now().UTC()
	if !exists {
		profile = models.Profile{
			UserID:            userID,
			SocialLinks:       []models.SocialLink{},
			TopFriends:        []string{},
			DonationAddresses: []models.CryptoAddress{},
			CreatedAt:         now,
		}
	}

	if update.Bio != nil {
		profile.Bio = strings.TrimSpace(*update.Bio)
	}
	if update.AvatarURL != nil {
		profile.AvatarURL = strings.TrimSpace(*update.AvatarURL)
	}
	if update.BannerURL != nil {
		profile.BannerURL = strings.TrimSpace(*update.BannerURL)
	}
	if update.SocialLinks != nil {
		normalized, err := NormalizeSocialLinks(*update.SocialLinks)
		if err != nil {
			return models.Profile{}, err
		}
		profile.SocialLinks = normalized
	}
	if update.FeaturedChannelID != nil {
		trimmed := strings.TrimSpace(*update.FeaturedChannelID)
		if trimmed == "" {
			profile.FeaturedChannelID = nil
		} else {
			channel, ok := updatedData.Channels[trimmed]
			if !ok {
				return models.Profile{}, fmt.Errorf("featured channel %s not found", trimmed)
			}
			if channel.OwnerID != userID {
				return models.Profile{}, errors.New("featured channel must belong to profile owner")
			}
			id := channel.ID
			profile.FeaturedChannelID = &id
		}
	}
	if update.TopFriends != nil {
		if len(*update.TopFriends) > 8 {
			return models.Profile{}, errors.New("top friends cannot exceed eight entries")
		}
		seen := make(map[string]struct{})
		ordered := make([]string, 0, len(*update.TopFriends))
		for _, friendID := range *update.TopFriends {
			trimmed := strings.TrimSpace(friendID)
			if trimmed == "" {
				return models.Profile{}, errors.New("top friends must reference valid users")
			}
			if trimmed == userID {
				return models.Profile{}, errors.New("cannot add profile owner as a top friend")
			}
			if _, friendExists := updatedData.Users[trimmed]; !friendExists {
				return models.Profile{}, fmt.Errorf("top friend %s not found", trimmed)
			}
			if _, duplicate := seen[trimmed]; duplicate {
				return models.Profile{}, errors.New("duplicate user in top friends list")
			}
			seen[trimmed] = struct{}{}
			ordered = append(ordered, trimmed)
		}
		profile.TopFriends = ordered
	}
	if update.DonationAddresses != nil {
		addresses := make([]models.CryptoAddress, 0, len(*update.DonationAddresses))
		for _, addr := range *update.DonationAddresses {
			normalized, err := NormalizeDonationAddress(addr)
			if err != nil {
				return models.Profile{}, err
			}
			addresses = append(addresses, normalized)
		}
		profile.DonationAddresses = addresses
	}

	profile.UpdatedAt = now
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = now
	}

	updatedData.Profiles[userID] = profile
	if err := s.persistDataset(updatedData); err != nil {
		return models.Profile{}, err
	}

	s.data = updatedData

	return profile, nil
}

func (s *Storage) GetProfile(userID string) (models.Profile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profile, ok := s.data.Profiles[userID]
	if !ok {
		user, userExists := s.data.Users[userID]
		if !userExists {
			return models.Profile{}, false
		}
		profile = models.Profile{
			UserID:            userID,
			SocialLinks:       []models.SocialLink{},
			TopFriends:        []string{},
			DonationAddresses: []models.CryptoAddress{},
			CreatedAt:         user.CreatedAt,
			UpdatedAt:         user.CreatedAt,
		}
		return profile, false
	}

	if profile.SocialLinks == nil {
		profile.SocialLinks = []models.SocialLink{}
	}
	if profile.TopFriends == nil {
		profile.TopFriends = []string{}
	}
	if profile.DonationAddresses == nil {
		profile.DonationAddresses = []models.CryptoAddress{}
	}

	return profile, true
}

func (s *Storage) ListProfiles() []models.Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profiles := make([]models.Profile, 0, len(s.data.Profiles))
	for _, profile := range s.data.Profiles {
		if profile.SocialLinks == nil {
			profile.SocialLinks = []models.SocialLink{}
		}
		if profile.TopFriends == nil {
			profile.TopFriends = []string{}
		}
		if profile.DonationAddresses == nil {
			profile.DonationAddresses = []models.CryptoAddress{}
		}
		profiles = append(profiles, profile)
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].CreatedAt.Before(profiles[j].CreatedAt)
	})
	return profiles
}

// Channel operations

type ChannelUpdate struct {
	Title     *string
	Category  *string
	Tags      *[]string
	LiveState *string
}

func (s *Storage) CreateChannel(ownerID, title, category string, tags []string) (models.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Users[ownerID]; !ok {
		return models.Channel{}, fmt.Errorf("owner %s not found", ownerID)
	}
	if title = strings.TrimSpace(title); title == "" {
		return models.Channel{}, errors.New("title is required")
	}

	id, err := generateID()
	if err != nil {
		return models.Channel{}, err
	}
	streamKey, err := generateStreamKey()
	if err != nil {
		return models.Channel{}, err
	}

	now := time.Now().UTC()
	channel := models.Channel{
		ID:        id,
		OwnerID:   ownerID,
		StreamKey: streamKey,
		Title:     title,
		Category:  strings.TrimSpace(category),
		Tags:      normalizeTags(tags),
		LiveState: "offline",
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.data.Channels[id] = channel
	if err := s.persist(); err != nil {
		delete(s.data.Channels, id)
		return models.Channel{}, err
	}

	return channel, nil
}

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return []string{}
	}
	normalized := make([]string, 0, len(tags))
	seen := make(map[string]struct{})
	for _, tag := range tags {
		trimmed := strings.TrimSpace(strings.ToLower(tag))
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	sort.Strings(normalized)
	return normalized
}

func (s *Storage) UpdateChannel(id string, update ChannelUpdate) (models.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updatedData := cloneDataset(s.data)

	channel, ok := updatedData.Channels[id]
	if !ok {
		return models.Channel{}, fmt.Errorf("channel %s not found", id)
	}

	if update.Title != nil {
		if title := strings.TrimSpace(*update.Title); title != "" {
			channel.Title = title
		} else {
			return models.Channel{}, errors.New("title cannot be empty")
		}
	}
	if update.Category != nil {
		channel.Category = strings.TrimSpace(*update.Category)
	}
	if update.Tags != nil {
		channel.Tags = normalizeTags(*update.Tags)
	}
	if update.LiveState != nil {
		state := strings.ToLower(strings.TrimSpace(*update.LiveState))
		if state != "offline" && state != "live" && state != "starting" && state != "ended" {
			return models.Channel{}, fmt.Errorf("invalid liveState %s", state)
		}
		channel.LiveState = state
	}

	channel.UpdatedAt = time.Now().UTC()
	updatedData.Channels[id] = channel
	if err := s.persistDataset(updatedData); err != nil {
		return models.Channel{}, err
	}

	s.data = updatedData

	return channel, nil
}

func (s *Storage) RotateChannelStreamKey(id string) (models.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updatedData := cloneDataset(s.data)

	channel, ok := updatedData.Channels[id]
	if !ok {
		return models.Channel{}, fmt.Errorf("channel %s not found", id)
	}

	streamKey, err := generateStreamKey()
	if err != nil {
		return models.Channel{}, err
	}

	channel.StreamKey = streamKey
	channel.UpdatedAt = time.Now().UTC()
	updatedData.Channels[id] = channel

	if err := s.persistDataset(updatedData); err != nil {
		return models.Channel{}, err
	}

	s.data = updatedData

	return channel, nil
}

func (s *Storage) GetChannel(id string) (models.Channel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	channel, ok := s.data.Channels[id]
	return channel, ok
}

// GetChannelByStreamKey looks up a channel by its stream key.
func (s *Storage) GetChannelByStreamKey(streamKey string) (models.Channel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := strings.TrimSpace(streamKey)
	if key == "" {
		return models.Channel{}, false
	}

	for _, channel := range s.data.Channels {
		if channel.StreamKey == key {
			return channel, true
		}
	}

	return models.Channel{}, false
}

func (s *Storage) ListChannels(ownerID, query string) []models.Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	channels := make([]models.Channel, 0, len(s.data.Channels))
	for _, channel := range s.data.Channels {
		if ownerID != "" && channel.OwnerID != ownerID {
			continue
		}
		if normalizedQuery != "" {
			owner := s.data.Users[channel.OwnerID]
			if !channelMatchesQuery(channel, owner, normalizedQuery) {
				continue
			}
		}
		channels = append(channels, channel)
	}
	sort.Slice(channels, func(i, j int) bool {
		if channels[i].LiveState == channels[j].LiveState {
			return channels[i].CreatedAt.Before(channels[j].CreatedAt)
		}
		return channels[i].LiveState == "live"
	})
	return channels
}

func channelMatchesQuery(channel models.Channel, owner models.User, normalizedQuery string) bool {
	if normalizedQuery == "" {
		return true
	}
	if strings.Contains(strings.ToLower(channel.Title), normalizedQuery) {
		return true
	}
	if owner.DisplayName != "" && strings.Contains(strings.ToLower(owner.DisplayName), normalizedQuery) {
		return true
	}
	for _, tag := range channel.Tags {
		if strings.Contains(strings.ToLower(tag), normalizedQuery) {
			return true
		}
	}
	return false
}

// FollowChannel records that a viewer is following the channel. The operation is idempotent.
func (s *Storage) FollowChannel(userID, channelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	updatedData := cloneDataset(s.data)

	if _, ok := updatedData.Users[userID]; !ok {
		return fmt.Errorf("user %s not found", userID)
	}
	if _, ok := updatedData.Channels[channelID]; !ok {
		return fmt.Errorf("channel %s not found", channelID)
	}

	if updatedData.Follows == nil {
		updatedData.Follows = make(map[string]map[string]time.Time)
	}
	follows := updatedData.Follows[userID]
	if follows == nil {
		follows = make(map[string]time.Time)
	}
	if _, exists := follows[channelID]; !exists {
		follows[channelID] = time.Now().UTC()
	}
	updatedData.Follows[userID] = follows

	if err := s.persistDataset(updatedData); err != nil {
		return err
	}

	s.data = updatedData

	return nil
}

// UnfollowChannel removes the follow association if present. The operation is idempotent.
func (s *Storage) UnfollowChannel(userID, channelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	updatedData := cloneDataset(s.data)

	if _, ok := updatedData.Users[userID]; !ok {
		return fmt.Errorf("user %s not found", userID)
	}
	if _, ok := updatedData.Channels[channelID]; !ok {
		return fmt.Errorf("channel %s not found", channelID)
	}

	if follows, ok := updatedData.Follows[userID]; ok {
		if _, exists := follows[channelID]; exists {
			delete(follows, channelID)
			if len(follows) == 0 {
				delete(updatedData.Follows, userID)
			} else {
				updatedData.Follows[userID] = follows
			}
		}
	}

	if err := s.persistDataset(updatedData); err != nil {
		return err
	}

	s.data = updatedData

	return nil
}

// IsFollowingChannel reports whether the given user follows the channel.
func (s *Storage) IsFollowingChannel(userID, channelID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	follows, ok := s.data.Follows[userID]
	if !ok {
		return false
	}
	_, exists := follows[channelID]
	return exists
}

// CountFollowers returns the number of viewers following the channel.
func (s *Storage) CountFollowers(channelID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, follows := range s.data.Follows {
		if follows == nil {
			continue
		}
		if _, ok := follows[channelID]; ok {
			count++
		}
	}
	return count
}

// ListFollowedChannelIDs returns the identifiers of channels the user follows ordered by recency.
func (s *Storage) ListFollowedChannelIDs(userID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	follows, ok := s.data.Follows[userID]
	if !ok || len(follows) == 0 {
		return nil
	}

	type pair struct {
		id   string
		when time.Time
	}

	pairs := make([]pair, 0, len(follows))
	for channelID, followedAt := range follows {
		pairs = append(pairs, pair{id: channelID, when: followedAt})
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].when.After(pairs[j].when)
	})

	ids := make([]string, 0, len(pairs))
	for _, p := range pairs {
		ids = append(ids, p.id)
	}
	return ids
}

// DeleteChannel removes a channel and its associated sessions and chat transcripts.
func (s *Storage) DeleteChannel(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	updatedData := cloneDataset(s.data)

	channel, ok := updatedData.Channels[id]
	if !ok {
		return fmt.Errorf("channel %s not found", id)
	}
	if channel.CurrentSessionID != nil {
		return errors.New("cannot delete a channel with an active stream")
	}

	delete(updatedData.Channels, id)

	for sessionID, session := range updatedData.StreamSessions {
		if session.ChannelID == id {
			delete(updatedData.StreamSessions, sessionID)
		}
	}
	for messageID, message := range updatedData.ChatMessages {
		if message.ChannelID == id {
			delete(updatedData.ChatMessages, messageID)
		}
	}
	for userID, follows := range updatedData.Follows {
		if follows == nil {
			continue
		}
		if _, exists := follows[id]; exists {
			delete(follows, id)
			if len(follows) == 0 {
				delete(updatedData.Follows, userID)
			} else {
				updatedData.Follows[userID] = follows
			}
		}
	}

	for profileID, profile := range updatedData.Profiles {
		if profile.FeaturedChannelID != nil && *profile.FeaturedChannelID == id {
			profile.FeaturedChannelID = nil
			updatedData.Profiles[profileID] = profile
		}
	}

	if err := s.persistDataset(updatedData); err != nil {
		return err
	}

	s.data = updatedData

	return nil
}

// Streaming operations

func (s *Storage) StartStream(channelID string, renditions []string) (models.StreamSession, error) {
	s.mu.Lock()
	channel, ok := s.data.Channels[channelID]
	if !ok {
		s.mu.Unlock()
		return models.StreamSession{}, fmt.Errorf("channel %s not found", channelID)
	}
	if channel.CurrentSessionID != nil {
		s.mu.Unlock()
		return models.StreamSession{}, errors.New("channel already live")
	}

	sessionID, err := generateID()
	if err != nil {
		s.mu.Unlock()
		return models.StreamSession{}, err
	}

	channel.CurrentSessionID = &sessionID
	channel.LiveState = "starting"
	s.data.Channels[channelID] = channel
	s.mu.Unlock()

	controller := s.ingestController
	if controller == nil {
		s.mu.Lock()
		if updated, exists := s.data.Channels[channelID]; exists {
			updated.CurrentSessionID = nil
			updated.LiveState = "offline"
			s.data.Channels[channelID] = updated
		}
		s.mu.Unlock()
		return models.StreamSession{}, ErrIngestControllerUnavailable
	}

	attempts := s.ingestMaxAttempts
	if attempts <= 0 {
		attempts = 1
	}
	timeout := normalizeIngestTimeout(s.ingestTimeout)
	var boot ingest.BootResult
	var bootErr error
	for attempt := 0; attempt < attempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		boot, bootErr = controller.BootStream(ctx, ingest.BootParams{
			ChannelID:  channelID,
			SessionID:  sessionID,
			StreamKey:  channel.StreamKey,
			Renditions: append([]string{}, renditions...),
		})
		cancel()
		if bootErr == nil {
			break
		}
		if attempt < attempts-1 && s.ingestRetryInterval > 0 {
			time.Sleep(s.ingestRetryInterval)
		}
	}
	if bootErr != nil {
		s.mu.Lock()
		if updated, exists := s.data.Channels[channelID]; exists {
			updated.CurrentSessionID = nil
			updated.LiveState = "offline"
			s.data.Channels[channelID] = updated
		}
		s.mu.Unlock()
		return models.StreamSession{}, fmt.Errorf("boot ingest: %w", bootErr)
	}

	now := time.Now().UTC()
	session := models.StreamSession{
		ID:             sessionID,
		ChannelID:      channelID,
		StartedAt:      now,
		Renditions:     append([]string{}, renditions...),
		PeakConcurrent: 0,
		OriginURL:      boot.OriginURL,
		PlaybackURL:    boot.PlaybackURL,
		IngestJobIDs:   append([]string{}, boot.JobIDs...),
	}
	ingestEndpoints := make([]string, 0, 2)
	if boot.PrimaryIngest != "" {
		ingestEndpoints = append(ingestEndpoints, boot.PrimaryIngest)
	}
	if boot.BackupIngest != "" {
		ingestEndpoints = append(ingestEndpoints, boot.BackupIngest)
	}
	if len(ingestEndpoints) > 0 {
		session.IngestEndpoints = ingestEndpoints
	}
	if len(boot.Renditions) > 0 {
		manifests := make([]models.RenditionManifest, 0, len(boot.Renditions))
		for _, rendition := range boot.Renditions {
			manifests = append(manifests, models.RenditionManifest{
				Name:        rendition.Name,
				ManifestURL: rendition.ManifestURL,
				Bitrate:     rendition.Bitrate,
			})
		}
		session.RenditionManifests = manifests
	}

	s.mu.Lock()
	s.data.StreamSessions[sessionID] = session
	channel = s.data.Channels[channelID]
	channel.CurrentSessionID = &sessionID
	channel.LiveState = "live"
	channel.UpdatedAt = now
	s.data.Channels[channelID] = channel

	if err := s.persist(); err != nil {
		delete(s.data.StreamSessions, sessionID)
		channel.CurrentSessionID = nil
		channel.LiveState = "offline"
		s.data.Channels[channelID] = channel
		jobIDs := append([]string{}, session.IngestJobIDs...)
		s.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		_ = controller.ShutdownStream(ctx, channelID, sessionID, jobIDs)
		cancel()
		return models.StreamSession{}, err
	}
	s.mu.Unlock()

	return session, nil
}

func (s *Storage) StopStream(channelID string, peakConcurrent int) (models.StreamSession, error) {
	s.mu.Lock()
	channel, ok := s.data.Channels[channelID]
	if !ok {
		s.mu.Unlock()
		return models.StreamSession{}, fmt.Errorf("channel %s not found", channelID)
	}
	if channel.CurrentSessionID == nil {
		s.mu.Unlock()
		return models.StreamSession{}, errors.New("channel is not live")
	}

	sessionID := *channel.CurrentSessionID
	session, ok := s.data.StreamSessions[sessionID]
	if !ok {
		s.mu.Unlock()
		return models.StreamSession{}, fmt.Errorf("session %s missing", sessionID)
	}

	originalChannel := channel
	originalSession := session
	jobIDs := append([]string{}, session.IngestJobIDs...)
	s.mu.Unlock()

	controller := s.ingestController
	if controller == nil {
		return models.StreamSession{}, ErrIngestControllerUnavailable
	}

	timeout := normalizeIngestTimeout(s.ingestTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := controller.ShutdownStream(ctx, channelID, sessionID, jobIDs); err != nil {
		return models.StreamSession{}, fmt.Errorf("shutdown ingest: %w", err)
	}

	now := time.Now().UTC()
	session.EndedAt = &now
	if peakConcurrent > session.PeakConcurrent {
		session.PeakConcurrent = peakConcurrent
	}

	s.mu.Lock()
	channel, ok = s.data.Channels[channelID]
	if !ok {
		s.mu.Unlock()
		return models.StreamSession{}, fmt.Errorf("channel %s not found", channelID)
	}
	s.data.StreamSessions[sessionID] = session
	channel.CurrentSessionID = nil
	channel.LiveState = "offline"
	channel.UpdatedAt = now
	s.data.Channels[channelID] = channel

	recording, recErr := s.createRecordingLocked(session, channel, now)
	if recErr != nil {
		s.data.StreamSessions[sessionID] = originalSession
		s.data.Channels[channelID] = originalChannel
		s.mu.Unlock()
		return models.StreamSession{}, recErr
	}
	if recording.ID != "" {
		s.data.Recordings[recording.ID] = recording
	}

	if err := s.persist(); err != nil {
		s.data.StreamSessions[sessionID] = originalSession
		s.data.Channels[channelID] = originalChannel
		if recording.ID != "" {
			delete(s.data.Recordings, recording.ID)
		}
		s.mu.Unlock()
		return models.StreamSession{}, err
	}
	s.mu.Unlock()

	return session, nil
}

func (s *Storage) createRecordingLocked(session models.StreamSession, channel models.Channel, ended time.Time) (models.Recording, error) {
	s.ensureDatasetInitializedLocked()
	id, err := generateID()
	if err != nil {
		return models.Recording{}, err
	}
	duration := int(ended.Sub(session.StartedAt).Round(time.Second).Seconds())
	if duration < 0 {
		duration = 0
	}
	title := channel.Title
	if title == "" {
		title = fmt.Sprintf("Recording %s", session.ID)
	}
	title = strings.TrimSpace(title)
	metadata := map[string]string{
		"channelId":  channel.ID,
		"sessionId":  session.ID,
		"startedAt":  session.StartedAt.UTC().Format(time.RFC3339Nano),
		"endedAt":    ended.UTC().Format(time.RFC3339Nano),
		"renditions": strconv.Itoa(len(session.RenditionManifests)),
	}
	if session.PeakConcurrent > 0 {
		metadata["peakConcurrent"] = strconv.Itoa(session.PeakConcurrent)
	}
	recording := models.Recording{
		ID:              id,
		ChannelID:       channel.ID,
		SessionID:       session.ID,
		Title:           title,
		DurationSeconds: duration,
		PlaybackBaseURL: session.PlaybackURL,
		Metadata:        metadata,
		CreatedAt:       ended,
	}
	if deadline := s.recordingDeadline(ended, false); deadline != nil {
		recording.RetainUntil = deadline
	}
	if len(session.RenditionManifests) > 0 {
		renditions := make([]models.RecordingRendition, 0, len(session.RenditionManifests))
		for _, manifest := range session.RenditionManifests {
			renditions = append(renditions, models.RecordingRendition(manifest))
		}
		recording.Renditions = renditions
	}
	if err := s.populateRecordingArtifactsLocked(&recording, session); err != nil {
		return models.Recording{}, err
	}
	return recording, nil
}

func (s *Storage) populateRecordingArtifactsLocked(recording *models.Recording, session models.StreamSession) error {
	client := s.objectClient
	if client == nil || !client.Enabled() {
		return nil
	}
	if recording.Metadata == nil {
		recording.Metadata = make(map[string]string)
	}

	createdAt := recording.CreatedAt.UTC().Format(time.RFC3339Nano)
	if len(session.RenditionManifests) > 0 {
		for idx, manifest := range session.RenditionManifests {
			key := buildObjectKey("recordings", recording.ID, "manifests", normalizeObjectComponent(manifest.Name)+".json")
			payload := map[string]any{
				"recordingId": recording.ID,
				"sessionId":   recording.SessionID,
				"name":        manifest.Name,
				"source":      manifest.ManifestURL,
				"createdAt":   createdAt,
			}
			if manifest.Bitrate > 0 {
				payload["bitrate"] = manifest.Bitrate
			}
			data, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("encode manifest payload: %w", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), s.objectStorage.requestTimeout())
			ref, err := client.Upload(ctx, key, "application/json", data)
			cancel()
			if err != nil {
				return fmt.Errorf("upload manifest %s: %w", manifest.Name, err)
			}
			if ref.Key != "" {
				recording.Metadata[manifestMetadataKey(manifest.Name)] = ref.Key
			}
			if ref.URL != "" && idx < len(recording.Renditions) {
				recording.Renditions[idx].ManifestURL = ref.URL
			}
		}
	}

	thumbID, err := generateID()
	if err != nil {
		return fmt.Errorf("generate thumbnail id: %w", err)
	}
	thumbKey := buildObjectKey("recordings", recording.ID, "thumbnails", thumbID+".json")
	thumbPayload := map[string]any{
		"recordingId": recording.ID,
		"sessionId":   recording.SessionID,
		"createdAt":   createdAt,
	}
	thumbData, err := json.Marshal(thumbPayload)
	if err != nil {
		return fmt.Errorf("encode thumbnail payload: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.objectStorage.requestTimeout())
	ref, err := client.Upload(ctx, thumbKey, "application/json", thumbData)
	cancel()
	if err != nil {
		return fmt.Errorf("upload thumbnail: %w", err)
	}
	if ref.Key != "" {
		recording.Metadata[thumbnailMetadataKey(thumbID)] = ref.Key
	}
	thumbnail := models.RecordingThumbnail{
		ID:          thumbID,
		RecordingID: recording.ID,
		URL:         ref.URL,
		CreatedAt:   recording.CreatedAt,
	}
	recording.Thumbnails = append(recording.Thumbnails, thumbnail)
	return nil
}

func (s *Storage) deleteRecordingArtifactsLocked(recording models.Recording) error {
	client := s.objectClient
	if client == nil || !client.Enabled() {
		return nil
	}
	if len(recording.Metadata) == 0 {
		return nil
	}
	deleted := make(map[string]struct{})
	for key, objectKey := range recording.Metadata {
		if !strings.HasPrefix(key, metadataManifestPrefix) && !strings.HasPrefix(key, metadataThumbnailPrefix) {
			continue
		}
		trimmed := strings.TrimSpace(objectKey)
		if trimmed == "" {
			continue
		}
		if _, exists := deleted[trimmed]; exists {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), s.objectStorage.requestTimeout())
		err := client.Delete(ctx, trimmed)
		cancel()
		if err != nil {
			return fmt.Errorf("delete object %s: %w", trimmed, err)
		}
		deleted[trimmed] = struct{}{}
	}
	return nil
}

func (s *Storage) deleteClipArtifactsLocked(clip models.ClipExport) error {
	client := s.objectClient
	if client == nil || !client.Enabled() || strings.TrimSpace(clip.StorageObject) == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.objectStorage.requestTimeout())
	err := client.Delete(ctx, clip.StorageObject)
	cancel()
	if err != nil {
		return fmt.Errorf("delete clip object %s: %w", clip.StorageObject, err)
	}
	return nil
}

func (s *Storage) ListStreamSessions(channelID string) ([]models.StreamSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}

	sessions := make([]models.StreamSession, 0)
	for _, session := range s.data.StreamSessions {
		if session.ChannelID == channelID {
			sessions = append(sessions, session)
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartedAt.After(sessions[j].StartedAt)
	})
	return sessions, nil
}

func (s *Storage) ListRecordings(channelID string, includeUnpublished bool) ([]models.Recording, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}

	now := time.Now().UTC()
	removed, snapshot, err := s.purgeExpiredRecordingsLocked(now)
	if err != nil {
		return nil, err
	}
	if removed {
		if err := s.persist(); err != nil {
			s.data = snapshot
			return nil, err
		}
	}

	recordings := make([]models.Recording, 0)
	for _, recording := range s.data.Recordings {
		if recording.ChannelID != channelID {
			continue
		}
		if !includeUnpublished && recording.PublishedAt == nil {
			continue
		}
		recordings = append(recordings, s.recordingWithClipsLocked(recording))
	}
	sort.Slice(recordings, func(i, j int) bool {
		return recordings[i].CreatedAt.After(recordings[j].CreatedAt)
	})
	return recordings, nil
}

// Upload operations

func (s *Storage) CreateUpload(params CreateUploadParams) (models.Upload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureDatasetInitializedLocked()

	channelID := strings.TrimSpace(params.ChannelID)
	channel, ok := s.data.Channels[channelID]
	if !ok {
		return models.Upload{}, fmt.Errorf("channel %s not found", channelID)
	}

	title := strings.TrimSpace(params.Title)
	if title == "" {
		if channel.Title != "" {
			title = fmt.Sprintf("%s upload", channel.Title)
		} else {
			title = "Uploaded video"
		}
	}

	filename := strings.TrimSpace(params.Filename)
	if filename == "" {
		filename = fmt.Sprintf("upload-%s.mp4", time.Now().UTC().Format("20060102-150405"))
	}

	id, err := generateID()
	if err != nil {
		return models.Upload{}, err
	}

	now := time.Now().UTC()
	metadata := make(map[string]string, len(params.Metadata))
	for k, v := range params.Metadata {
		if strings.TrimSpace(k) == "" {
			continue
		}
		metadata[k] = v
	}

	playbackURL := strings.TrimSpace(params.PlaybackURL)

	upload := models.Upload{
		ID:          id,
		ChannelID:   channelID,
		Title:       title,
		Filename:    filename,
		SizeBytes:   params.SizeBytes,
		Status:      "pending",
		Progress:    0,
		Metadata:    metadata,
		PlaybackURL: playbackURL,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.data.Uploads[id] = upload
	if err := s.persist(); err != nil {
		delete(s.data.Uploads, id)
		return models.Upload{}, err
	}

	return upload, nil
}

func (s *Storage) ListUploads(channelID string) ([]models.Upload, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}

	uploads := make([]models.Upload, 0)
	for _, upload := range s.data.Uploads {
		if upload.ChannelID != channelID {
			continue
		}
		uploads = append(uploads, cloneUpload(upload))
	}
	sort.Slice(uploads, func(i, j int) bool {
		return uploads[i].CreatedAt.After(uploads[j].CreatedAt)
	})
	return uploads, nil
}

func (s *Storage) GetUpload(id string) (models.Upload, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	upload, ok := s.data.Uploads[id]
	if !ok {
		return models.Upload{}, false
	}
	return cloneUpload(upload), true
}

func (s *Storage) UpdateUpload(id string, update UploadUpdate) (models.Upload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	upload, ok := s.data.Uploads[id]
	if !ok {
		return models.Upload{}, fmt.Errorf("upload %s not found", id)
	}

	original := upload

	if update.Title != nil {
		if trimmed := strings.TrimSpace(*update.Title); trimmed != "" {
			upload.Title = trimmed
		}
	}
	if update.Status != nil {
		upload.Status = strings.TrimSpace(*update.Status)
	}
	if update.Progress != nil {
		progress := *update.Progress
		if progress < 0 {
			progress = 0
		}
		if progress > 100 {
			progress = 100
		}
		upload.Progress = progress
	}
	if update.RecordingID != nil {
		trimmed := strings.TrimSpace(*update.RecordingID)
		if trimmed == "" {
			upload.RecordingID = nil
		} else {
			upload.RecordingID = &trimmed
		}
	}
	if update.PlaybackURL != nil {
		upload.PlaybackURL = strings.TrimSpace(*update.PlaybackURL)
	}
	if update.Metadata != nil {
		if upload.Metadata == nil {
			upload.Metadata = make(map[string]string, len(update.Metadata))
		}
		for k, v := range update.Metadata {
			if strings.TrimSpace(k) == "" {
				continue
			}
			if v == "" {
				delete(upload.Metadata, k)
				continue
			}
			upload.Metadata[k] = v
		}
	}
	if update.Error != nil {
		upload.Error = strings.TrimSpace(*update.Error)
	}
	if update.CompletedAt != nil {
		if update.CompletedAt.IsZero() {
			upload.CompletedAt = nil
		} else {
			completed := update.CompletedAt.UTC()
			upload.CompletedAt = &completed
		}
	}

	upload.UpdatedAt = time.Now().UTC()

	s.data.Uploads[id] = upload
	if err := s.persist(); err != nil {
		s.data.Uploads[id] = original
		return models.Upload{}, err
	}
	return cloneUpload(upload), nil
}

func (s *Storage) DeleteUpload(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	upload, ok := s.data.Uploads[id]
	if !ok {
		return fmt.Errorf("upload %s not found", id)
	}

	delete(s.data.Uploads, id)
	if err := s.persist(); err != nil {
		s.data.Uploads[id] = upload
		return err
	}
	return nil
}

func (s *Storage) GetRecording(id string) (models.Recording, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id == "" {
		return models.Recording{}, false
	}
	now := time.Now().UTC()
	removed, snapshot, err := s.purgeExpiredRecordingsLocked(now)
	if err != nil {
		return models.Recording{}, false
	}
	if removed {
		if err := s.persist(); err != nil {
			s.data = snapshot
			return models.Recording{}, false
		}
	}
	recording, ok := s.data.Recordings[id]
	if !ok {
		return models.Recording{}, false
	}
	return s.recordingWithClipsLocked(recording), true
}

func (s *Storage) PublishRecording(id string) (models.Recording, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id == "" {
		return models.Recording{}, fmt.Errorf("recording id is required")
	}

	recording, ok := s.data.Recordings[id]
	if !ok {
		return models.Recording{}, fmt.Errorf("recording %s not found", id)
	}
	if recording.PublishedAt != nil {
		return s.recordingWithClipsLocked(recording), nil
	}

	now := time.Now().UTC()
	updated := cloneRecording(recording)
	updated.PublishedAt = &now
	if deadline := s.recordingDeadline(now, true); deadline != nil {
		updated.RetainUntil = deadline
	} else {
		updated.RetainUntil = nil
	}

	snapshot := cloneDataset(s.data)
	s.data.Recordings[id] = updated
	if err := s.persist(); err != nil {
		s.data = snapshot
		return models.Recording{}, err
	}
	return s.recordingWithClipsLocked(updated), nil
}

func (s *Storage) DeleteRecording(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id == "" {
		return fmt.Errorf("recording id is required")
	}
	recording, ok := s.data.Recordings[id]
	if !ok {
		return fmt.Errorf("recording %s not found", id)
	}
	if err := s.deleteRecordingArtifactsLocked(recording); err != nil {
		return err
	}
	snapshot := cloneDataset(s.data)
	for clipID, clip := range s.data.ClipExports {
		if clip.RecordingID != id {
			continue
		}
		if err := s.deleteClipArtifactsLocked(clip); err != nil {
			s.data = snapshot
			return err
		}
		delete(s.data.ClipExports, clipID)
	}
	delete(s.data.Recordings, id)
	if err := s.persist(); err != nil {
		s.data = snapshot
		return err
	}
	return nil
}

func (s *Storage) CreateClipExport(recordingID string, params ClipExportParams) (models.ClipExport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if recordingID == "" {
		return models.ClipExport{}, fmt.Errorf("recording id is required")
	}
	recording, ok := s.data.Recordings[recordingID]
	if !ok {
		return models.ClipExport{}, fmt.Errorf("recording %s not found", recordingID)
	}
	if params.EndSeconds <= params.StartSeconds {
		return models.ClipExport{}, fmt.Errorf("endSeconds must be greater than startSeconds")
	}
	if params.StartSeconds < 0 {
		return models.ClipExport{}, fmt.Errorf("startSeconds must be non-negative")
	}
	if recording.DurationSeconds > 0 && params.EndSeconds > recording.DurationSeconds {
		return models.ClipExport{}, fmt.Errorf("clip exceeds recording duration")
	}
	id, err := generateID()
	if err != nil {
		return models.ClipExport{}, err
	}
	now := time.Now().UTC()
	clip := models.ClipExport{
		ID:           id,
		RecordingID:  recordingID,
		ChannelID:    recording.ChannelID,
		SessionID:    recording.SessionID,
		Title:        strings.TrimSpace(params.Title),
		StartSeconds: params.StartSeconds,
		EndSeconds:   params.EndSeconds,
		Status:       "pending",
		CreatedAt:    now,
	}
	snapshot := cloneDataset(s.data)
	s.data.ClipExports[id] = clip
	if err := s.persist(); err != nil {
		s.data = snapshot
		return models.ClipExport{}, err
	}
	return clip, nil
}

func (s *Storage) ListClipExports(recordingID string) ([]models.ClipExport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if recordingID == "" {
		return nil, fmt.Errorf("recording id is required")
	}
	if _, ok := s.data.Recordings[recordingID]; !ok {
		return nil, fmt.Errorf("recording %s not found", recordingID)
	}
	clips := make([]models.ClipExport, 0)
	for _, clip := range s.data.ClipExports {
		if clip.RecordingID != recordingID {
			continue
		}
		clips = append(clips, cloneClipExport(clip))
	}
	sort.Slice(clips, func(i, j int) bool {
		return clips[i].CreatedAt.After(clips[j].CreatedAt)
	})
	return clips, nil
}

// CurrentStreamSession returns the active stream session for the channel if present.
func (s *Storage) CurrentStreamSession(channelID string) (models.StreamSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	channel, ok := s.data.Channels[channelID]
	if !ok || channel.CurrentSessionID == nil {
		return models.StreamSession{}, false
	}
	session, exists := s.data.StreamSessions[*channel.CurrentSessionID]
	if !exists {
		return models.StreamSession{}, false
	}
	return session, true
}

// IngestHealth reports the status of configured ingest dependencies.
func (s *Storage) IngestHealth(ctx context.Context) []ingest.HealthStatus {
	controller := s.ingestController
	if controller == nil {
		status := []ingest.HealthStatus{{Component: "ingest", Status: "disabled"}}
		s.recordIngestHealth(status)
		return status
	}
	checks := controller.HealthChecks(ctx)
	if len(checks) == 0 {
		checks = []ingest.HealthStatus{{Component: "ingest", Status: "unknown"}}
	}
	s.recordIngestHealth(checks)
	return checks
}

func (s *Storage) recordIngestHealth(statuses []ingest.HealthStatus) {
	snapshot := append([]ingest.HealthStatus(nil), statuses...)
	s.mu.Lock()
	s.ingestHealth = snapshot
	s.ingestHealthUpdated = time.Now().UTC()
	s.mu.Unlock()
}

// LastIngestHealth returns the most recently recorded ingest health snapshot.
func (s *Storage) LastIngestHealth() ([]ingest.HealthStatus, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.ingestHealth) == 0 {
		return nil, time.Time{}
	}
	snapshot := append([]ingest.HealthStatus(nil), s.ingestHealth...)
	return snapshot, s.ingestHealthUpdated
}

// CreateTip records a tip event for a channel.
func (s *Storage) CreateTip(params CreateTipParams) (models.Tip, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Channels[params.ChannelID]; !ok {
		return models.Tip{}, fmt.Errorf("channel %s not found", params.ChannelID)
	}
	if _, ok := s.data.Users[params.FromUserID]; !ok {
		return models.Tip{}, fmt.Errorf("user %s not found", params.FromUserID)
	}
	amount := params.Amount
	if amount.MinorUnits() <= 0 {
		return models.Tip{}, fmt.Errorf("amount must be positive")
	}
	currency := strings.ToUpper(strings.TrimSpace(params.Currency))
	if currency == "" {
		return models.Tip{}, fmt.Errorf("currency is required")
	}
	provider := strings.ToLower(strings.TrimSpace(params.Provider))
	if provider == "" {
		return models.Tip{}, fmt.Errorf("provider is required")
	}
	reference := strings.TrimSpace(params.Reference)
	if reference == "" {
		reference = fmt.Sprintf("tip-%d", time.Now().UnixNano())
	}
	if utf8.RuneCountInString(reference) > MaxTipReferenceLength {
		return models.Tip{}, fmt.Errorf("reference exceeds %d characters", MaxTipReferenceLength)
	}
	wallet := strings.TrimSpace(params.WalletAddress)
	if utf8.RuneCountInString(wallet) > MaxTipWalletAddressLength {
		return models.Tip{}, fmt.Errorf("wallet address exceeds %d characters", MaxTipWalletAddressLength)
	}
	message := strings.TrimSpace(params.Message)
	if utf8.RuneCountInString(message) > MaxTipMessageLength {
		return models.Tip{}, fmt.Errorf("message exceeds %d characters", MaxTipMessageLength)
	}
	if s.tipExists(provider, reference) {
		return models.Tip{}, errors.New(duplicateTipReferenceError)
	}
	id, err := generateID()
	if err != nil {
		return models.Tip{}, err
	}
	now := time.Now().UTC()
	tip := models.Tip{
		ID:            id,
		ChannelID:     params.ChannelID,
		FromUserID:    params.FromUserID,
		Amount:        amount,
		Currency:      currency,
		Provider:      provider,
		Reference:     reference,
		WalletAddress: wallet,
		Message:       message,
		CreatedAt:     now,
	}
	if s.data.Tips == nil {
		s.data.Tips = make(map[string]models.Tip)
	}
	s.data.Tips[id] = tip
	if err := s.persist(); err != nil {
		delete(s.data.Tips, id)
		return models.Tip{}, err
	}
	return tip, nil
}

// tipExists reports whether a tip with the given provider/reference pair is
// already persisted. Callers must hold s.mu.
func (s *Storage) tipExists(provider, reference string) bool {
	if len(s.data.Tips) == 0 {
		return false
	}
	for _, tip := range s.data.Tips {
		if tip.Provider == provider && tip.Reference == reference {
			return true
		}
	}
	return false
}

// ListTips returns recent tips for a channel.
func (s *Storage) ListTips(channelID string, limit int) ([]models.Tip, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}
	tips := make([]models.Tip, 0)
	for _, tip := range s.data.Tips {
		if tip.ChannelID == channelID {
			tips = append(tips, tip)
		}
	}
	sort.Slice(tips, func(i, j int) bool {
		return tips[i].CreatedAt.After(tips[j].CreatedAt)
	})
	if limit > 0 && len(tips) > limit {
		tips = tips[:limit]
	}
	return tips, nil
}

// CreateSubscription records a new channel subscription.
func (s *Storage) CreateSubscription(params CreateSubscriptionParams) (models.Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Channels[params.ChannelID]; !ok {
		return models.Subscription{}, fmt.Errorf("channel %s not found", params.ChannelID)
	}
	if _, ok := s.data.Users[params.UserID]; !ok {
		return models.Subscription{}, fmt.Errorf("user %s not found", params.UserID)
	}
	if params.Duration <= 0 {
		return models.Subscription{}, fmt.Errorf("duration must be positive")
	}
	amount := params.Amount
	if amount.MinorUnits() < 0 {
		return models.Subscription{}, fmt.Errorf("amount cannot be negative")
	}
	currency := strings.ToUpper(strings.TrimSpace(params.Currency))
	if currency == "" {
		return models.Subscription{}, fmt.Errorf("currency is required")
	}
	tier := strings.TrimSpace(params.Tier)
	if tier == "" {
		tier = "supporter"
	}
	provider := strings.ToLower(strings.TrimSpace(params.Provider))
	if provider == "" {
		return models.Subscription{}, fmt.Errorf("provider is required")
	}
	reference := strings.TrimSpace(params.Reference)
	if reference == "" {
		reference = fmt.Sprintf("sub-%d", time.Now().UnixNano())
	}
	for _, existing := range s.data.Subscriptions {
		if existing.Provider == provider && existing.Reference == reference {
			return models.Subscription{}, fmt.Errorf("subscription reference %s/%s already exists", provider, reference)
		}
	}
	id, err := generateID()
	if err != nil {
		return models.Subscription{}, err
	}
	started := time.Now().UTC()
	expires := started.Add(params.Duration)
	subscription := models.Subscription{
		ID:                id,
		ChannelID:         params.ChannelID,
		UserID:            params.UserID,
		Tier:              tier,
		Provider:          provider,
		Reference:         reference,
		Amount:            amount,
		Currency:          currency,
		StartedAt:         started,
		ExpiresAt:         expires,
		AutoRenew:         params.AutoRenew,
		Status:            "active",
		ExternalReference: strings.TrimSpace(params.ExternalReference),
	}
	if s.data.Subscriptions == nil {
		s.data.Subscriptions = make(map[string]models.Subscription)
	}
	s.data.Subscriptions[id] = subscription
	if err := s.persist(); err != nil {
		delete(s.data.Subscriptions, id)
		return models.Subscription{}, err
	}
	return subscription, nil
}

// ListSubscriptions lists subscriptions for a channel.
func (s *Storage) ListSubscriptions(channelID string, includeInactive bool) ([]models.Subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}
	subs := make([]models.Subscription, 0)
	for _, sub := range s.data.Subscriptions {
		if sub.ChannelID != channelID {
			continue
		}
		if !includeInactive && !strings.EqualFold(sub.Status, "active") {
			continue
		}
		subs = append(subs, sub)
	}
	sort.Slice(subs, func(i, j int) bool {
		if subs[i].StartedAt.Equal(subs[j].StartedAt) {
			return subs[i].ID < subs[j].ID
		}
		return subs[i].StartedAt.After(subs[j].StartedAt)
	})
	return subs, nil
}

// GetSubscription returns a subscription by id.
func (s *Storage) GetSubscription(id string) (models.Subscription, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sub, ok := s.data.Subscriptions[id]
	return sub, ok
}

// CancelSubscription marks a subscription as cancelled.
func (s *Storage) CancelSubscription(id, cancelledBy, reason string) (models.Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	subscription, ok := s.data.Subscriptions[id]
	if !ok {
		return models.Subscription{}, fmt.Errorf("subscription %s not found", id)
	}
	if subscription.Status == "cancelled" {
		return subscription, nil
	}
	if _, ok := s.data.Users[cancelledBy]; !ok {
		return models.Subscription{}, fmt.Errorf("user %s not found", cancelledBy)
	}
	now := time.Now().UTC()
	subscription.Status = "cancelled"
	subscription.AutoRenew = false
	subscription.CancelledBy = cancelledBy
	subscription.CancelledAt = &now
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		if cancelledBy == subscription.UserID {
			trimmed = "user_cancelled"
		} else {
			trimmed = "cancelled_by_admin"
		}
	}
	subscription.CancelledReason = trimmed
	s.data.Subscriptions[id] = subscription
	if err := s.persist(); err != nil {
		return models.Subscription{}, err
	}
	return subscription, nil
}
