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

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Role      string `json:"role"`
	ToolName  string `json:"tool_name,omitempty"`
	Content   string `json:"content,omitempty"`
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

func (l *ConversationLogger) Append(role, toolName, content string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil && filepath.Dir(l.path) != "." {
		return fmt.Errorf("create log dir: %w", err)
	}

	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer f.Close()

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Role:      role,
		ToolName:  toolName,
		Content:   content,
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
