package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

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
	var builder strings.Builder
	for idx, key := range keys {
		if idx > 0 {
			builder.WriteByte('&')
		}
		sort.Strings(values[key])
		for vIdx, value := range values[key] {
			if vIdx > 0 {
				builder.WriteByte('&')
			}
			builder.WriteString(url.QueryEscape(key))
			builder.WriteByte('=')
			builder.WriteString(url.QueryEscape(value))
		}
	}
	return builder.String()
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
