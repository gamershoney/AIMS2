package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	apiUrl       = "http://216.119.200.67:8080/api/"
	defMdoel     = "qwen3.5:9b"
	gemmaModel   = "gemma4:12b"
	defKeepAlive = "5m"

	systemPrompt = "You are a purchasing/sales assistant for a welding distributor. Your users will only be outside salespeople and purchasing managers. Frame all of your responses for that demographic. It is important that responses are not given with emojis and are styled professionally. Never answer price, cost, vendor, usage, received dates, sales dates, notes, or full item details from stock_check alone. stock_check is only for candidate discovery and availability. Use item_details for commercial or transaction fields before answering those questions."
)

type Messages struct {
	Role      string     `json:"role"`
	Content   string     `json:"content,omitempty"`
	ToolName  string     `json:"tool_name,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	turn      int
}

type Chat struct {
	Messages      []Messages  `json:"messages"`
	Model         string      `json:"model"`
	Stream        bool        `json:"stream"`
	KeepAlive     string      `json:"keep_alive"`
	Tools         []ModelTool `json:"tools,omitempty"`
	workflowState *WorkflowState
	currentTurn   int
	registry      *Registry
	logger        *ConversationLogger
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
		toolResults = append(toolResults, m.RunRequiredDetailFollowUps(toolResults)...)

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

	fmt.Printf("MODEL REQUEST SIZE: bytes=%d messages=%d\n", len(b), len(m.Messages))
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
		if result.State != nil {
			m.workflowState = result.State
			err := m.ApplyState(result.State)
			if err != nil {
				continue
			}

		}
		results = append(results, result)
	}

	return results
}

func (m *Chat) RunRequiredDetailFollowUps(results []ToolResult) []ToolResult {
	if !asksForItemDetails(m.lastUserMessage()) {
		return nil
	}
	if toolResultsInclude(results, "item_details") {
		return nil
	}

	stockNumber := exactSingleStockCheckResult(results)
	if stockNumber == "" {
		return nil
	}

	return []ToolResult{m.runToolByName("item_details", map[string]any{
		"stock_number": stockNumber,
	})}
}

func (m *Chat) runToolByName(name string, args map[string]any) ToolResult {
	if m.registry == nil || m.registry.tools == nil {
		return ToolResult{Name: name, Error: errors.New("tool registry is not initialized")}
	}

	tool, ok := m.registry.tools[name]
	if !ok {
		return ToolResult{Name: name, Error: fmt.Errorf("tool not found: %s", name)}
	}

	if m.logger != nil {
		_ = m.logger.LogToolCall(name, args)
	}

	tc := ToolCall{}
	tc.Function.Name = name
	tc.Function.Arguments = args

	result := tool.Execute(context.TODO(), tc)
	if result.Name == "" {
		result.Name = name
	}
	result.Response = result.truncate(maxToolResponse)

	if result.State != nil {
		if err := m.ApplyState(result.State); err != nil && m.logger != nil {
			_ = m.logger.LogError("workflow_state", err.Error())
		}
	}

	if m.logger != nil {
		_ = m.logger.LogToolResult(result.Name, result.Response, result.Error)
	}

	return result
}

func (m *Chat) lastUserMessage() string {
	for i := len(m.Messages) - 1; i >= 0; i-- {
		if m.Messages[i].Role == "user" {
			return m.Messages[i].Content
		}
	}
	return ""
}

func asksForItemDetails(text string) bool {
	text = strings.ToLower(text)
	terms := []string{
		"cost",
		"price",
		"retail",
		"selling price",
		"sell price",
		"vendor",
		"usage",
		"received",
		"receive",
		"last sale",
		"last shipment",
		"shipment",
		"note",
		"minimum",
		"target",
		"stocking",
		"full detail",
		"details",
	}
	for _, term := range terms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func toolResultsInclude(results []ToolResult, name string) bool {
	for _, result := range results {
		if result.Name == name && result.Error == nil {
			return true
		}
	}
	return false
}

func exactSingleStockCheckResult(results []ToolResult) string {
	for _, result := range results {
		if result.Name != "stock_check" || result.Error != nil {
			continue
		}

		var response StockCheckResponse
		if err := json.Unmarshal([]byte(result.Response), &response); err != nil {
			continue
		}
		if response.Count != 1 || len(response.Results) != 1 {
			continue
		}
		return response.Results[0].StockNumber
	}
	return ""
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
	m.AppendMessage("system", "workflow_state", "")
}
