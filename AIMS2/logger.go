package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const conversationLogPath = "conversation_log.jsonl"

// LogEntry is a single structured NDJSON log record. Every field that is not
// applicable to a given event is omitted from the output.
type LogEntry struct {
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Event     string         `json:"event"`
	Role      string         `json:"role,omitempty"`
	ToolName  string         `json:"tool_name,omitempty"`
	Content   string         `json:"content,omitempty"`
	Model     string         `json:"model,omitempty"`
	Error     string         `json:"error,omitempty"`
	Args      map[string]any `json:"args,omitempty"`
}

type ConversationLogger struct {
	mu   sync.Mutex
	path string
}

func NewConversationLogger(path string) *ConversationLogger {
	if path == "" {
		path = conversationLogPath
	}
	return &ConversationLogger{path: path}
}

// Log writes a single LogEntry as one NDJSON line. It is the core write method
// used by all typed helpers.
func (l *ConversationLogger) Log(entry LogEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if dir := filepath.Dir(l.path); dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create log dir: %w", err)
		}
	}

	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer f.Close()

	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}

	b, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal log entry: %w", err)
	}

	if _, err := f.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("write log entry: %w", err)
	}

	return nil
}

// Append records a conversation message (user / assistant / system / tool).
func (l *ConversationLogger) Append(role, toolName, content string) error {
	return l.LogMessage(role, toolName, content)
}

// LogMessage records a conversation message event.
func (l *ConversationLogger) LogMessage(role, toolName, content string) error {
	return l.Log(LogEntry{
		Level:    "info",
		Event:    "message",
		Role:     role,
		ToolName: toolName,
		Content:  content,
	})
}

// LogToolCall records that the model requested a tool invocation.
func (l *ConversationLogger) LogToolCall(name string, args map[string]any) error {
	return l.Log(LogEntry{
		Level:    "info",
		Event:    "tool_call",
		ToolName: name,
		Args:     args,
	})
}

// LogToolResult records the outcome of a tool execution.
func (l *ConversationLogger) LogToolResult(name, response string, callErr error) error {
	entry := LogEntry{
		Level:    "info",
		Event:    "tool_result",
		ToolName: name,
		Content:  response,
	}
	if callErr != nil {
		entry.Level = "error"
		entry.Error = callErr.Error()
	}
	return l.Log(entry)
}

// LogAPIRequest records bounded request diagnostics without writing the full
// model payload to stdout.
func (l *ConversationLogger) LogAPIRequest(model string, requestBytes, messageCount int) error {
	return l.Log(LogEntry{
		Level: "debug",
		Event: "api_request",
		Model: model,
		Args: map[string]any{
			"request_bytes": requestBytes,
			"message_count": messageCount,
		},
	})
}

// LogAPIResponse records the raw response received from the LLM API.
func (l *ConversationLogger) LogAPIResponse(model string, done bool, content string) error {
	return l.Log(LogEntry{
		Level:   "debug",
		Event:   "api_response",
		Model:   model,
		Content: content,
		Args:    map[string]any{"done": done},
	})
}

// LogError records an error event.
func (l *ConversationLogger) LogError(event, msg string) error {
	return l.Log(LogEntry{
		Level: "error",
		Event: event,
		Error: msg,
	})
}
