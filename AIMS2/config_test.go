package main

import (
	"testing"
	"time"
)

func TestLoadConfigUsesEnvironment(t *testing.T) {
	t.Setenv("AIMS2_MODEL_API_URL", "http://model.example.test/api")
	t.Setenv("AIMS2_MODEL", "test-model")
	t.Setenv("AIMS2_KEEP_ALIVE", "10m")
	t.Setenv("AIMS2_HTTP_TIMEOUT", "45s")
	t.Setenv("AIMS2_WEB_SEARCH_API_URL", "https://search.example.test")
	t.Setenv("OLLAMA_API_KEY", "test-key")
	t.Setenv("AIMS2_CONVERSATION_LOG", "/tmp/aims2-test.jsonl")

	config := LoadConfig()
	if config.ModelAPIURL != "http://model.example.test/api/" {
		t.Fatalf("unexpected model URL: %q", config.ModelAPIURL)
	}
	if config.Model != "test-model" || config.KeepAlive != "10m" {
		t.Fatalf("unexpected model config: %+v", config)
	}
	if config.RequestTimeout != 45*time.Second {
		t.Fatalf("unexpected request timeout: %s", config.RequestTimeout)
	}
	if config.WebSearchAPIURL != "https://search.example.test" || config.WebSearchAPIKey != "test-key" {
		t.Fatalf("unexpected web search config: %+v", config)
	}
	if config.ConversationLog != "/tmp/aims2-test.jsonl" {
		t.Fatalf("unexpected logging config: %+v", config)
	}
}

func TestConversationLoggingIsDisabledByDefault(t *testing.T) {
	t.Setenv("AIMS2_CONVERSATION_LOG", "")
	config := LoadConfig()
	if config.conversationLogger() != nil {
		t.Fatal("expected conversation logging to be disabled without an explicit path")
	}
}

func TestWebSearchRequiresAPIKey(t *testing.T) {
	_, err := runWebSearch(t.Context(), "test query", defaultWebSearchURL, "", nil)
	if err == nil {
		t.Fatal("expected web search without an API key to fail before making a request")
	}
}
