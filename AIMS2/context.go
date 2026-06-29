package main

import (
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
