package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const (
	apiUrl       = "http://216.119.200.67:8080/api/"
	defMdoel     = "qwen3.5:9b"
	gemmaModel   = "gemma4:12b"
	defKeepAlive = "5m"

	systemPrompt = "You are a purchasing/sales assistant for a welding distributor. Your users will only be outside salespeople and purchasing managers. Frame all of your responses for that demographic. It is important that responses are not given with emojis and are styled professionally."
)

type Messages struct {
	Role      string     `json:"role"`
	Content   string     `json:"content,omitempty"`
	ToolName  string     `json:"tool_name,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	turn      int
}

type Chat struct {
	Messages    []Messages  `json:"messages"`
	Model       string      `json:"model"`
	Stream      bool        `json:"stream"`
	KeepAlive   string      `json:"keep_alive"`
	Tools       []ModelTool `json:"tools,omitempty"`
	currentTurn int
	registry    *Registry
	logger      *ConversationLogger
}

type ToolCall struct {
	ID       string `json:"id"`
	Function struct {
		Index     int64          `json:"index"`
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	} `json:"function,omitempty"`
}

type AIMessage struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type Answer struct {
	Model   string    `json:"model"`
	Message AIMessage `json:"message"`
	Done    bool      `json:"done"`
}

func createMessage(m *Chat) ([]byte, error) {
	return json.Marshal(m)
}

func (m *Chat) SendMessage() (*Answer, error) {
	const maxToolRounds = 5

	for round := 0; round < maxToolRounds; round++ {
		ansr, err := m.sendOnce()
		if err != nil {
			if m.logger != nil {
				_ = m.logger.LogError("send_message", err.Error())
			}
			return nil, err
		}

		fmt.Printf(
			"Response: done:%t, model:%s, ToolCalls: %v\n\n",
			ansr.Done,
			ansr.Model,
			ansr.Message.ToolCalls,
		)

		// This is critical.
		// Save the assistant response WITH tool_calls.
		m.AppendAssistantMessage(ansr.Message)

		// No tool calls means this is the final answer for the user.
		if len(ansr.Message.ToolCalls) == 0 {
			fmt.Println("Finalizing Response to user")
			return ansr, nil
		}

		// Run tools, append tool results, then loop back and ask the model again.
		toolResults := m.RunToolCalls(ansr.Message)

		for _, tr := range toolResults {
			if tr.Error != nil {
				fmt.Println("error on tool results:", tr.Error)

				errResponse := fmt.Sprintf(`{"error":%q}`, tr.Error.Error())
				m.AppendMessage("tool", tr.Name, errResponse)
				continue
			}

			fmt.Println("Tool Result for tool:", tr.Name, tr.Response)
			m.AppendMessage("tool", tr.Name, tr.Response)
		}
	}

	if m.logger != nil {
		_ = m.logger.LogError("send_message", "too many tool call rounds")
	}
	return nil, fmt.Errorf("too many tool call rounds")
}

func (m *Chat) sendOnce() (*Answer, error) {
	b, err := createMessage(m)
	if err != nil {
		return nil, err
	}

	fmt.Println("MESSAGES SENT TO MODEL:")
	fmt.Println(string(b))

	if m.logger != nil {
		_ = m.logger.LogAPIRequest(m.Model)
	}

	br := bytes.NewReader(b)

	resp, err := http.Post(apiUrl+"chat", "application/json", br)
	if err != nil {
		if m.logger != nil {
			_ = m.logger.LogError("api_request", err.Error())
		}
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("error on sending message. StatusCode: %d\nresponse: %s", resp.StatusCode, body)
		if m.logger != nil {
			_ = m.logger.LogError("api_response", errMsg)
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	fmt.Println(string(body))
	fmt.Println("")

	ansr := &Answer{}

	if err := json.Unmarshal(body, ansr); err != nil {
		return nil, fmt.Errorf("unmarshal chat response: %w", err)
	}

	if m.logger != nil {
		_ = m.logger.LogAPIResponse(ansr.Model, ansr.Done, ansr.Message.Content)
	}

	return ansr, nil
}

func (m *Chat) RunToolCalls(ai AIMessage) []ToolResult {
	ch := make(chan ToolResult, len(ai.ToolCalls))
	ctx := context.TODO()

	for _, v := range ai.ToolCalls {
		tool, ok := m.registry.tools[v.Function.Name]
		if !ok {
			if m.logger != nil {
				_ = m.logger.LogError("tool_call", fmt.Sprintf("tool not found: %s", v.Function.Name))
			}
			ch <- ToolResult{
				Name:  v.Function.Name,
				Error: errors.New("tool not found"),
			}
			continue
		}

		if m.logger != nil {
			_ = m.logger.LogToolCall(v.Function.Name, v.Function.Arguments)
		}

		go func(t ToolCall) {
			result := tool.Execute(ctx, t)

			// Make sure the result has the tool name.
			if result.Name == "" {
				result.Name = t.Function.Name
			}

			if m.logger != nil {
				_ = m.logger.LogToolResult(result.Name, result.Response, result.Error)
			}

			ch <- result
		}(v)
	}

	results := make([]ToolResult, 0, len(ai.ToolCalls))

	for range ai.ToolCalls {
		result := <-ch
		result.Response = result.truncate(maxToolResponse)
		results = append(results, result)
	}

	return results
}

func (m *Chat) AppendMessage(role string, toolname string, text string) {
	msg := Messages{
		Role:    role,
		Content: text,
		turn:    m.currentTurn,
	}

	if toolname != "" {
		msg.ToolName = toolname
	}

	m.Messages = append(m.Messages, msg)

	if m.logger != nil {
		_ = m.logger.LogMessage(role, toolname, text)
	}
}

func (m *Chat) AppendAssistantMessage(ai AIMessage) {
	m.Messages = append(m.Messages, Messages{
		Role:      "assistant",
		Content:   ai.Content,
		ToolCalls: ai.ToolCalls,
		turn:      m.currentTurn,
	})
}

func (m *Chat) SetSystemMessage() {
	m.AppendMessage("system", "", systemPrompt)
}
