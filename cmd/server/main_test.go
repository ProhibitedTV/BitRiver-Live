package main

import (
	"log/slog"
	"testing"

	"bitriver-live/internal/chat"
)

func TestConfigureChatQueueMemory(t *testing.T) {
	queue, err := configureChatQueue("", chat.RedisQueueConfig{}, slog.Default())
	if err != nil {
		t.Fatalf("configureChatQueue returned error: %v", err)
	}
	if queue == nil {
		t.Fatalf("configureChatQueue returned nil queue")
	}
}

func TestConfigureChatQueueRedisMissingAddress(t *testing.T) {
	_, err := configureChatQueue("redis", chat.RedisQueueConfig{}, slog.Default())
	if err == nil {
		t.Fatal("configureChatQueue redis expected error when addr missing")
	}
}
