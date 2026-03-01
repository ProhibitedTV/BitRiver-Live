package chat

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"bitriver-live/internal/domain"
)

type regexCacheStore struct {
	filters []domain.ChatFilter
}

func (s *regexCacheStore) GetChannel(string) (domain.Channel, bool) { return domain.Channel{}, false }
func (s *regexCacheStore) GetUser(string) (domain.User, bool)       { return domain.User{}, false }
func (s *regexCacheStore) ChatRestrictions() RestrictionsSnapshot   { return RestrictionsSnapshot{} }
func (s *regexCacheStore) IsChatBanned(string, string) bool         { return false }
func (s *regexCacheStore) ChatTimeout(string, string) (time.Time, bool) {
	return time.Time{}, false
}
func (s *regexCacheStore) ListChatFilters(string) ([]domain.ChatFilter, error) { return s.filters, nil }

func TestGatewayMatchChatFilterRegexCacheReusesCompiledPattern(t *testing.T) {
	store := &regexCacheStore{filters: []domain.ChatFilter{{
		ID:      "filter-1",
		Kind:    "regex",
		Pattern: "(?i)spoiler",
		Enabled: true,
	}}}
	gateway := NewGateway(GatewayConfig{Store: store})

	compileCalls := 0
	gateway.regexCompile = func(pattern string) (*regexp.Regexp, error) {
		compileCalls++
		return regexp.Compile(pattern)
	}

	first, err := gateway.matchChatFilter("channel-1", "No Spoilers")
	if err != nil {
		t.Fatalf("matchChatFilter first call: %v", err)
	}
	if first == nil || first.ID != "filter-1" {
		t.Fatalf("expected regex filter match on first call, got %#v", first)
	}

	second, err := gateway.matchChatFilter("channel-1", "spoiler alert")
	if err != nil {
		t.Fatalf("matchChatFilter second call: %v", err)
	}
	if second == nil || second.ID != "filter-1" {
		t.Fatalf("expected regex filter match on second call, got %#v", second)
	}

	if compileCalls != 1 {
		t.Fatalf("expected one regex compilation, got %d", compileCalls)
	}
}

func TestGatewayMatchChatFilterRegexCacheRecompilesWhenPatternChanges(t *testing.T) {
	store := &regexCacheStore{filters: []domain.ChatFilter{{
		ID:      "filter-1",
		Kind:    "regex",
		Pattern: "foo",
		Enabled: true,
	}}}
	gateway := NewGateway(GatewayConfig{Store: store})

	compileCalls := 0
	gateway.regexCompile = func(pattern string) (*regexp.Regexp, error) {
		compileCalls++
		return regexp.Compile(pattern)
	}

	first, err := gateway.matchChatFilter("channel-1", "hello foo")
	if err != nil {
		t.Fatalf("matchChatFilter first call: %v", err)
	}
	if first == nil || first.ID != "filter-1" {
		t.Fatalf("expected first regex filter match, got %#v", first)
	}

	store.filters[0].Pattern = "bar"
	second, err := gateway.matchChatFilter("channel-1", "hello bar")
	if err != nil {
		t.Fatalf("matchChatFilter second call: %v", err)
	}
	if second == nil || second.ID != "filter-1" {
		t.Fatalf("expected second regex filter match after pattern change, got %#v", second)
	}

	if compileCalls != 2 {
		t.Fatalf("expected two regex compilations after pattern change, got %d", compileCalls)
	}
}

func TestGatewayMatchChatFilterRegexSkipsInvalidPattern(t *testing.T) {
	store := &regexCacheStore{filters: []domain.ChatFilter{{
		ID:      "filter-1",
		Kind:    "regex",
		Pattern: "(",
		Enabled: true,
	}}}
	gateway := NewGateway(GatewayConfig{Store: store})

	compileCalls := 0
	gateway.regexCompile = func(pattern string) (*regexp.Regexp, error) {
		compileCalls++
		return nil, errors.New("invalid regex")
	}

	match, err := gateway.matchChatFilter("channel-1", "hello")
	if err != nil {
		t.Fatalf("matchChatFilter: %v", err)
	}
	if match != nil {
		t.Fatalf("expected invalid regex filter to be skipped, got %#v", match)
	}

	match, err = gateway.matchChatFilter("channel-1", "hello")
	if err != nil {
		t.Fatalf("matchChatFilter second call: %v", err)
	}
	if match != nil {
		t.Fatalf("expected invalid regex filter to be skipped on second call, got %#v", match)
	}

	if compileCalls != 2 {
		t.Fatalf("expected invalid regex to compile each call (no cache insert), got %d", compileCalls)
	}
}
