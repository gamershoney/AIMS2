package main

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
