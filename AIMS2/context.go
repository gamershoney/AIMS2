package main

import (
	"context"
	"encoding/json"
	"slices"
)

const (
	systemPromptIndex = 0
	compactTrigger    = 3
)

func (m *Chat) Compact() {
	if m.currentTurn < compactTrigger {
		return
	}

	kept := make([]Messages, 0, len(m.Messages))

	for i, v := range m.Messages {
		if i == systemPromptIndex {
			kept = append(kept, v)
			continue
		}
		if v.turn == m.currentTurn {
			kept = append(kept, v)
			continue
		}
		if len(v.ToolCalls) == 0 && v.Role != "tool" {
			kept = append(kept, v)
			continue
		}

	}
	m.Messages = kept
}

type WorkflowState struct {
	LastToolUsed   string `json:"last_tool_used,omitempty"`
	LastItemSearch string `json:"last_item_searched,omitempty"`
	LastItemFound  string `json:"last_item_found,omitempty"`
	clear          bool
}

func (m *Chat) ApplyState(wf *WorkflowState) error {
	if wf == nil {
		return nil
	}
	state := m.workflowState
	if state == nil {
	} else if state.clear {
		wf.LastToolUsed = ""
		wf.LastItemSearch = ""
		wf.LastItemFound = ""
		wf.clear = false

	}
	m.workflowState = wf

	b, err := json.MarshalIndent(m.workflowState, "", "\t")
	if err != nil {
		return err
	}
	s := string(b)
	msg := Messages{
		Role:     "system",
		ToolName: "",
		turn:     0,
		Content:  "Current Working State:\n" + s,
	}

	if len(m.Messages) > 1 {
		first := m.Messages[1]
		if first.Role == "system" {
			m.Messages[1] = msg
			return nil
		}
		m.Messages = slices.Insert(m.Messages, 1, msg)
	}
	return nil
}

type ToolPermission string

const (
	PermissionNone     ToolPermission = "none"
	PermissionRequired ToolPermission = "required"
)

type SkillTool interface {
	Name() string
	Description() string
	Parameters() Parameters
	Execute(ctx context.Context, tc ToolCall) ToolResult
	Permission() ToolPermission
}

type SkillState struct {
	JobID         string         `json:"job_id"`
	SkillName     string         `json:"skill_name"`
	StateName     string         `json:"state_name"`
	EnteredReason string         `json:"entered_reason"`
	Status        string         `json:"status"`
	Summary       string         `json:"summary,omitempty"`
	Data          map[string]any `json:"data,omitempty"`
	Warnings      []string       `json:"warnings,omitempty"`
}

type SkillSession struct {
	State        SkillState
	Chat         *Chat
	AllowedTools map[string]Tool
}
