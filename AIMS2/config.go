package main

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultModelAPIURL    = "http://localhost:11434/api/"
	defaultWebSearchURL   = "https://ollama.com/api/web_search"
	defaultModel          = "gemma4:12b"
	defaultKeepAlive      = "5m"
	defaultRequestTimeout = 2 * time.Minute
)

// Config contains runtime settings that may vary by installation. Secrets and
// private service addresses belong in environment variables, never source.
type Config struct {
	ModelAPIURL        string
	Model              string
	KeepAlive          string
	WebSearchAPIURL    string
	WebSearchAPIKey    string
	ConversationLog    string
	RequestTimeout     time.Duration
	DebugModelPayloads bool
}

func LoadConfig() Config {
	return Config{
		ModelAPIURL:        normalizeBaseURL(envOrDefault("AIMS2_MODEL_API_URL", defaultModelAPIURL)),
		Model:              envOrDefault("AIMS2_MODEL", defaultModel),
		KeepAlive:          envOrDefault("AIMS2_KEEP_ALIVE", defaultKeepAlive),
		WebSearchAPIURL:    envOrDefault("AIMS2_WEB_SEARCH_API_URL", defaultWebSearchURL),
		WebSearchAPIKey:    strings.TrimSpace(os.Getenv("OLLAMA_API_KEY")),
		ConversationLog:    strings.TrimSpace(os.Getenv("AIMS2_CONVERSATION_LOG")),
		RequestTimeout:     durationFromEnv("AIMS2_HTTP_TIMEOUT", defaultRequestTimeout),
		DebugModelPayloads: boolFromEnv("AIMS2_DEBUG_MODEL_PAYLOADS"),
	}
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func durationFromEnv(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return fallback
	}
	return duration
}

func boolFromEnv(name string) bool {
	value, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(name)))
	return err == nil && value
}

func normalizeBaseURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/") + "/"
}

func (c Config) httpClient() *http.Client {
	timeout := c.RequestTimeout
	if timeout <= 0 {
		timeout = defaultRequestTimeout
	}
	return &http.Client{Timeout: timeout}
}

func (c Config) conversationLogger() *ConversationLogger {
	if c.ConversationLog == "" {
		return nil
	}
	return NewConversationLogger(c.ConversationLog)
}
